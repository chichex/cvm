package kb

// Spec: S-010 | Type: e2e
// Integration tests covering multi-operation workflows (B-008 through B-014).
// Individual feature unit tests live in kb_test.go.

import (
	"strings"
	"testing"
	"time"

	"github.com/chichex/cvm/internal/config"
)

// TestE2E_FullLifecycle exercises the complete happy path:
// Put → Search → Show (updates LastReferenced) → List with tag filter → Timeline → StatsDetailed → Remove → verify gone.
func TestE2E_FullLifecycle(t *testing.T) {
	projectPath := setupTestKB(t)

	// 1. Put entries with explicit types
	if err := PutWithOptions(config.ScopeLocal, projectPath, "db-decision", "Use PostgreSQL for persistence", []string{"infra"}, "decision"); err != nil {
		t.Fatalf("PutWithOptions db-decision: %v", err)
	}
	if err := PutWithOptions(config.ScopeLocal, projectPath, "auth-learning", "JWT expiry must be validated server-side", []string{"security"}, "learning"); err != nil {
		t.Fatalf("PutWithOptions auth-learning: %v", err)
	}
	seedEntry(t, projectPath, "readme-note", "Project readme needs updating", []string{"docs"})

	// 2. Search finds entries and ranks them properly
	results, err := SearchWithOptions(config.ScopeLocal, projectPath, "auth-learning", SearchOptions{})
	if err != nil {
		t.Fatalf("SearchWithOptions: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results, got none")
	}
	if results[0].Entry.Key != "auth-learning" {
		t.Errorf("expected exact key match first, got %q", results[0].Entry.Key)
	}
	if results[0].Rank != 0 {
		t.Errorf("expected rank 0 for exact match, got %d", results[0].Rank)
	}

	// 3. Show updates LastReferenced
	before := time.Now().Add(-time.Millisecond)
	content, err := Show(config.ScopeLocal, projectPath, "auth-learning")
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if !strings.Contains(content, "JWT") {
		t.Errorf("Show: expected body content, got %q", content)
	}

	// Verify LastReferenced via Backend.Get (works for both flat and SQLite)
	// Spec: S-013 | Fix: Backend wiring — use Backend instead of loadIndex
	{
		b, bErr := NewBackend(config.ScopeLocal, projectPath)
		if bErr != nil {
			t.Fatalf("NewBackend for LastReferenced check: %v", bErr)
		}
		doc, gErr := b.Get("auth-learning")
		b.Close()
		if gErr != nil {
			t.Fatalf("Backend.Get auth-learning: %v", gErr)
		}
		lr := doc.Entry.LastReferenced
		if lr.IsZero() || lr.Before(before) {
			t.Errorf("Show should update LastReferenced; got %v, want after %v", lr, before)
		}
	}

	// 4. List with tag filter returns only matching entries
	entries, err := List(config.ScopeLocal, projectPath, "security")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 || entries[0].Key != "auth-learning" {
		t.Errorf("List(security): expected [auth-learning], got %v", entries)
	}

	// 5. Timeline captures today's entries
	days, err := Timeline(config.ScopeLocal, projectPath, 7)
	if err != nil {
		t.Fatalf("Timeline: %v", err)
	}
	if len(days) == 0 {
		t.Fatal("Timeline: expected at least 1 day")
	}
	totalInTimeline := 0
	for _, d := range days {
		totalInTimeline += len(d.Entries)
	}
	if totalInTimeline != 3 {
		t.Errorf("Timeline: expected 3 entries total, got %d", totalInTimeline)
	}

	// 6. StatsDetailed reflects all 3 entries, all enabled
	stats, err := StatsDetailed(config.ScopeLocal, projectPath)
	if err != nil {
		t.Fatalf("StatsDetailed: %v", err)
	}
	if stats.Total != 3 {
		t.Errorf("StatsDetailed: expected total=3, got %d", stats.Total)
	}
	if stats.Enabled != 3 {
		t.Errorf("StatsDetailed: expected enabled=3, got %d", stats.Enabled)
	}
	if _, ok := stats.PerEntry["auth-learning"]; !ok {
		t.Error("StatsDetailed: expected auth-learning in PerEntry")
	}

	// 7. Remove one entry and verify it is gone
	if err := Remove(config.ScopeLocal, projectPath, "readme-note"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	all, err := List(config.ScopeLocal, projectPath, "")
	if err != nil {
		t.Fatalf("List after Remove: %v", err)
	}
	for _, e := range all {
		if e.Key == "readme-note" {
			t.Error("readme-note should have been removed but still appears in List")
		}
	}
	if len(all) != 2 {
		t.Errorf("expected 2 entries after Remove, got %d", len(all))
	}

	// 8. Show on removed entry returns error
	if _, err := Show(config.ScopeLocal, projectPath, "readme-note"); err == nil {
		t.Error("Show on removed entry should return error")
	}
}

// TestE2E_DedupWorkflow exercises the full dedup lifecycle:
// identical content → skipped; same body different tags → updated; different key same body → warning but written.
func TestE2E_DedupWorkflow(t *testing.T) {
	projectPath := setupTestKB(t)

	body := "content that will be duplicated across test scenarios"

	// 1. First put — never skipped
	skipped, err := PutWithDedup(config.ScopeLocal, projectPath, "orig", body, []string{"tag1"})
	if err != nil {
		t.Fatalf("first PutWithDedup: %v", err)
	}
	if skipped {
		t.Error("first put must not be skipped")
	}

	// 2. Same key, same body, same tags — skipped
	skipped, err = PutWithDedup(config.ScopeLocal, projectPath, "orig", body, []string{"tag1"})
	if err != nil {
		t.Fatalf("second PutWithDedup (identical): %v", err)
	}
	if !skipped {
		t.Error("identical put must be skipped")
	}

	// Verify exactly 1 entry still exists
	entries, _ := List(config.ScopeLocal, projectPath, "")
	if len(entries) != 1 {
		t.Errorf("after skip: expected 1 entry, got %d", len(entries))
	}

	// 3. Same key, same body, different tags — not skipped; tags updated
	skipped, err = PutWithDedup(config.ScopeLocal, projectPath, "orig", body, []string{"tag2", "extra"})
	if err != nil {
		t.Fatalf("PutWithDedup (tag update): %v", err)
	}
	if skipped {
		t.Error("tag-change put must not be skipped")
	}

	entries, _ = List(config.ScopeLocal, projectPath, "")
	if len(entries) != 1 {
		t.Fatalf("after tag update: expected 1 entry, got %d", len(entries))
	}
	// Verify new tags are present
	found := false
	for _, tag := range entries[0].Tags {
		if tag == "tag2" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected tag 'tag2' after update, got %v", entries[0].Tags)
	}

	// 4. Different key, same body — warns to stderr but STILL writes
	skipped, err = PutWithDedup(config.ScopeLocal, projectPath, "clone", body, []string{"clone-tag"})
	if err != nil {
		t.Fatalf("PutWithDedup (different key): %v", err)
	}
	if skipped {
		t.Error("different-key put must not be skipped even with duplicate body")
	}

	entries, _ = List(config.ScopeLocal, projectPath, "")
	if len(entries) != 2 {
		t.Errorf("after different-key put: expected 2 entries, got %d", len(entries))
	}

	// 5. Verify 'clone' is independently retrievable
	content, err := Show(config.ScopeLocal, projectPath, "clone")
	if err != nil {
		t.Fatalf("Show clone: %v", err)
	}
	if !strings.Contains(content, "duplicated") {
		t.Errorf("clone body should contain original content, got: %q", content)
	}
}

// TestE2E_SearchRankingWithFilters seeds 6 entries with various tags and types, then
// verifies that SearchWithOptions correctly applies combined filters and returns ranked results.
func TestE2E_SearchRankingWithFilters(t *testing.T) {
	projectPath := setupTestKB(t)

	// Seed entries: mix of types, tags, and matching terms
	if err := PutWithOptions(config.ScopeLocal, projectPath, "cache", "Cache invalidation strategy document", []string{"infra"}, "decision"); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if err := PutWithOptions(config.ScopeLocal, projectPath, "cache-gotcha", "Cache must be invalidated on write", []string{"infra"}, "gotcha"); err != nil {
		t.Fatalf("seed cache-gotcha: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	seedEntry(t, projectPath, "network-infra", "Network infra config for cache layer", []string{"infra"})
	time.Sleep(5 * time.Millisecond)
	seedEntry(t, projectPath, "db-schema", "Database schema has no cache mention", []string{"db"})
	time.Sleep(5 * time.Millisecond)
	if err := PutWithOptions(config.ScopeLocal, projectPath, "session-log", "Session log about cache usage patterns", []string{"infra"}, "learning"); err != nil {
		t.Fatalf("seed session-log: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	seedEntry(t, projectPath, "unrelated", "This entry has nothing to do with the query", []string{"misc"})

	// --- Filter 1: tag=infra — should exclude db-schema and unrelated
	results, err := SearchWithOptions(config.ScopeLocal, projectPath, "cache", SearchOptions{Tag: "infra"})
	if err != nil {
		t.Fatalf("SearchWithOptions tag=infra: %v", err)
	}
	for _, r := range results {
		hasInfra := false
		for _, tg := range r.Entry.Tags {
			if tg == "infra" {
				hasInfra = true
			}
		}
		if !hasInfra {
			t.Errorf("tag filter violated: entry %q lacks tag 'infra'", r.Entry.Key)
		}
	}
	// "cache" and "cache-gotcha" are exact/contains, "network-infra" and "session-log" are body matches
	if len(results) < 3 {
		t.Errorf("expected at least 3 infra results for 'cache', got %d", len(results))
	}

	// --- Filter 2: type=gotcha — only entries with tag type:gotcha
	results, err = SearchWithOptions(config.ScopeLocal, projectPath, "cache", SearchOptions{TypeTag: "gotcha"})
	if err != nil {
		t.Fatalf("SearchWithOptions type=gotcha: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for type=gotcha, got %d", len(results))
	}
	if results[0].Entry.Key != "cache-gotcha" {
		t.Errorf("expected 'cache-gotcha', got %q", results[0].Entry.Key)
	}

	// --- Filter 3: tag=infra + type=learning — intersection
	results, err = SearchWithOptions(config.ScopeLocal, projectPath, "cache", SearchOptions{Tag: "infra", TypeTag: "learning"})
	if err != nil {
		t.Fatalf("SearchWithOptions tag+type: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for infra+learning, got %d", len(results))
	}
	if results[0].Entry.Key != "session-log" {
		t.Errorf("expected 'session-log', got %q", results[0].Entry.Key)
	}

	// --- Filter 4: since=1h — all entries were just created, all should be included
	results, err = SearchWithOptions(config.ScopeLocal, projectPath, "cache", SearchOptions{Since: time.Hour})
	if err != nil {
		t.Fatalf("SearchWithOptions since=1h: %v", err)
	}
	// db-schema and unrelated have no "cache" match, so they won't appear
	if len(results) < 3 {
		t.Errorf("expected >=3 recent results, got %d", len(results))
	}

	// --- Ranking: without filters, exact key match ranks first
	results, err = SearchWithOptions(config.ScopeLocal, projectPath, "cache", SearchOptions{})
	if err != nil {
		t.Fatalf("SearchWithOptions ranking: %v", err)
	}
	if results[0].Rank != 0 || results[0].Entry.Key != "cache" {
		t.Errorf("ranking: expected rank-0 exact match 'cache' first, got rank=%d key=%q", results[0].Rank, results[0].Entry.Key)
	}
	if results[1].Rank != 1 {
		t.Errorf("ranking: expected rank-1 (key-contains) second, got rank=%d", results[1].Rank)
	}
}

// TestE2E_CompactWorkflow seeds multi-line entries, calls Compact, verifies first-line extraction
// and descending token-sorted order, then adds more entries and calls Compact again to verify update.
func TestE2E_CompactWorkflow(t *testing.T) {
	projectPath := setupTestKB(t)

	// Seed entries with multi-line bodies
	seedEntry(t, projectPath, "alpha", "Alpha first line summary\nSecond line details\nThird line more details", []string{"a"})
	time.Sleep(5 * time.Millisecond)
	seedEntry(t, projectPath, "beta", "Beta summary line\nBeta details here", []string{"b"})
	time.Sleep(5 * time.Millisecond)
	seedEntry(t, projectPath, "gamma", strings.Repeat("x", 100), []string{"c"}) // single line, long — will be truncated

	// 1. First Compact call
	compact, err := Compact(config.ScopeLocal, projectPath)
	if err != nil {
		t.Fatalf("Compact: %v", err)
	}
	if len(compact) != 3 {
		t.Fatalf("expected 3 compact entries, got %d", len(compact))
	}

	// 2. Verify descending UpdatedAt order (gamma is newest)
	if compact[0].Key != "gamma" {
		t.Errorf("first compact entry should be most recent 'gamma', got %q", compact[0].Key)
	}
	if compact[2].Key != "alpha" {
		t.Errorf("last compact entry should be oldest 'alpha', got %q", compact[2].Key)
	}

	// 3. Verify first-line extraction
	for _, ce := range compact {
		switch ce.Key {
		case "alpha":
			if ce.FirstLine != "Alpha first line summary" {
				t.Errorf("alpha: expected 'Alpha first line summary', got %q", ce.FirstLine)
			}
		case "beta":
			if ce.FirstLine != "Beta summary line" {
				t.Errorf("beta: expected 'Beta summary line', got %q", ce.FirstLine)
			}
		case "gamma":
			// body is 100 x's — exceeds 80 char limit, should be truncated with "..."
			if !strings.HasSuffix(ce.FirstLine, "...") {
				t.Errorf("gamma: expected truncated first line ending in '...', got %q", ce.FirstLine)
			}
			if len(ce.FirstLine) != 83 { // 80 chars + "..."
				t.Errorf("gamma: expected length 83, got %d", len(ce.FirstLine))
			}
		}
	}

	// 4. Add more entries, call Compact again — new entries appear at the top
	time.Sleep(5 * time.Millisecond)
	seedEntry(t, projectPath, "delta", "Delta is the newest entry\nWith extra lines", []string{"d"})

	compact2, err := Compact(config.ScopeLocal, projectPath)
	if err != nil {
		t.Fatalf("second Compact: %v", err)
	}
	if len(compact2) != 4 {
		t.Fatalf("after adding delta: expected 4 compact entries, got %d", len(compact2))
	}
	if compact2[0].Key != "delta" {
		t.Errorf("after add: delta should be first (newest), got %q", compact2[0].Key)
	}
	if compact2[0].FirstLine != "Delta is the newest entry" {
		t.Errorf("delta first line: expected 'Delta is the newest entry', got %q", compact2[0].FirstLine)
	}
}

// TestE2E_EnableDisableWorkflow verifies that SetEnabled changes are reflected in StatsDetailed,
// that disabled entries remain in List, and that re-enabling restores the count.
func TestE2E_EnableDisableWorkflow(t *testing.T) {
	projectPath := setupTestKB(t)

	// 1. Seed 4 entries, all enabled by default
	seedEntry(t, projectPath, "e1", "Entry one body", []string{"group-a"})
	seedEntry(t, projectPath, "e2", "Entry two body", []string{"group-a"})
	seedEntry(t, projectPath, "e3", "Entry three body", []string{"group-b"})
	seedEntry(t, projectPath, "e4", "Entry four body", []string{"group-b"})

	stats, err := StatsDetailed(config.ScopeLocal, projectPath)
	if err != nil {
		t.Fatalf("StatsDetailed (baseline): %v", err)
	}
	if stats.Total != 4 || stats.Enabled != 4 {
		t.Errorf("baseline: expected total=4 enabled=4, got total=%d enabled=%d", stats.Total, stats.Enabled)
	}

	// 2. Disable two entries
	if err := SetEnabled(config.ScopeLocal, projectPath, "e2", false); err != nil {
		t.Fatalf("SetEnabled e2 false: %v", err)
	}
	if err := SetEnabled(config.ScopeLocal, projectPath, "e4", false); err != nil {
		t.Fatalf("SetEnabled e4 false: %v", err)
	}

	stats, err = StatsDetailed(config.ScopeLocal, projectPath)
	if err != nil {
		t.Fatalf("StatsDetailed after disable: %v", err)
	}
	if stats.Total != 4 {
		t.Errorf("after disable: total should remain 4, got %d", stats.Total)
	}
	if stats.Enabled != 2 {
		t.Errorf("after disable: expected enabled=2, got %d", stats.Enabled)
	}

	// 3. Disabled entries still appear in List (disable is not removal)
	all, err := List(config.ScopeLocal, projectPath, "")
	if err != nil {
		t.Fatalf("List after disable: %v", err)
	}
	if len(all) != 4 {
		t.Errorf("List should still return all 4 entries including disabled, got %d", len(all))
	}

	// 4. Disabled entries still appear in Search
	results, err := SearchWithOptions(config.ScopeLocal, projectPath, "Entry", SearchOptions{})
	if err != nil {
		t.Fatalf("SearchWithOptions after disable: %v", err)
	}
	if len(results) != 4 {
		t.Errorf("Search should find all 4 entries regardless of enabled state, got %d", len(results))
	}

	// 5. Re-enable one entry — Enabled count goes back up
	if err := SetEnabled(config.ScopeLocal, projectPath, "e2", true); err != nil {
		t.Fatalf("SetEnabled e2 true: %v", err)
	}

	stats, err = StatsDetailed(config.ScopeLocal, projectPath)
	if err != nil {
		t.Fatalf("StatsDetailed after re-enable: %v", err)
	}
	if stats.Enabled != 3 {
		t.Errorf("after re-enable: expected enabled=3, got %d", stats.Enabled)
	}

	// 6. SetEnabled on non-existent key returns error
	if err := SetEnabled(config.ScopeLocal, projectPath, "ghost", false); err == nil {
		t.Error("SetEnabled on missing key should return error")
	}

	// 7. Verify Enabled flag is persisted correctly via List (works for both flat and SQLite)
	// Spec: S-013 | Fix: Backend wiring — use List instead of loadIndex
	allFinal, err := List(config.ScopeLocal, projectPath, "")
	if err != nil {
		t.Fatalf("List for enabled check: %v", err)
	}
	for _, e := range allFinal {
		switch e.Key {
		case "e1", "e2":
			if !e.Enabled {
				t.Errorf("%s should be enabled", e.Key)
			}
		case "e4":
			if e.Enabled {
				t.Errorf("e4 should still be disabled")
			}
		}
	}
}
