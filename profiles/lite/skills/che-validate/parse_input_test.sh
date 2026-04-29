#!/usr/bin/env bash
# Spec test for /che-validate input parsing.
#
# SCOPE: This validates the *documented specification* in SKILL.md — not the
# runtime behavior of Claude executing the skill. The parse_input() below is a
# parallel reference implementation of the rules in SKILL.md "Paso 1". If the
# skill text changes shape and Claude parses inputs differently at runtime,
# this harness would still pass. Treat it as a regression guard for the spec
# (shape of NUM/FORCED_KIND/OWNER_REPO/REPO_FLAG and the gh api endpoint),
# not proof that /che-validate produces the right values end-to-end.
#
# For behavioral coverage, only a live invocation of /che-validate against a real PR
# in a sandbox repo would do. That remains the user's responsibility.
#
# Run: bash profiles/lite/skills/che-validate/parse_input_test.sh
# Exit code 0 = all cases pass, 1 = at least one failure.

set -u

# parse_input emits four lines: NUM, FORCED_KIND, OWNER_REPO, REPO_FLAG.
# Mirrors what the skill instructs Claude to extract in Step 1.
parse_input() {
  local input="$1"
  local num=""
  local kind=""
  local owner_repo=""
  local repo_flag=""

  if [[ "$input" =~ ^https://github\.com/([^/]+)/([^/]+)/(pull|issues)/([0-9]+)/?$ ]]; then
    owner_repo="${BASH_REMATCH[1]}/${BASH_REMATCH[2]}"
    case "${BASH_REMATCH[3]}" in
      pull) kind="pr" ;;
      issues) kind="issue" ;;
    esac
    num="${BASH_REMATCH[4]}"
  elif [[ "$input" =~ ^[Pp][Rr][[:space:]]+#?([0-9]+)$ ]]; then
    kind="pr"
    num="${BASH_REMATCH[1]}"
  elif [[ "$input" =~ ^[Ii]ssue[[:space:]]+#?([0-9]+)$ ]]; then
    kind="issue"
    num="${BASH_REMATCH[1]}"
  elif [[ "$input" =~ ^#?([0-9]+)$ ]]; then
    num="${BASH_REMATCH[1]}"
  fi

  if [[ -n "$owner_repo" ]]; then
    repo_flag="--repo $owner_repo"
  fi

  printf '%s\n%s\n%s\n%s\n' "$num" "$kind" "$owner_repo" "$repo_flag"
}

assert_parse() {
  local desc="$1" input="$2"
  local want_num="$3" want_kind="$4" want_owner_repo="$5" want_repo_flag="$6"

  local out
  out=$(parse_input "$input")
  local got_num got_kind got_owner_repo got_repo_flag
  { read -r got_num
    read -r got_kind
    read -r got_owner_repo
    read -r got_repo_flag
  } <<<"$out"

  local fail=0
  [[ "$got_num" == "$want_num" ]] || fail=1
  [[ "$got_kind" == "$want_kind" ]] || fail=1
  [[ "$got_owner_repo" == "$want_owner_repo" ]] || fail=1
  [[ "$got_repo_flag" == "$want_repo_flag" ]] || fail=1

  if [[ $fail -eq 0 ]]; then
    printf 'PASS  %s\n' "$desc"
  else
    printf 'FAIL  %s\n' "$desc"
    printf '      input=%q\n' "$input"
    printf '      want NUM=%q KIND=%q OWNER_REPO=%q REPO_FLAG=%q\n' \
      "$want_num" "$want_kind" "$want_owner_repo" "$want_repo_flag"
    printf '      got  NUM=%q KIND=%q OWNER_REPO=%q REPO_FLAG=%q\n' \
      "$got_num" "$got_kind" "$got_owner_repo" "$got_repo_flag"
    FAILURES=$((FAILURES + 1))
  fi
}

assert_invalid() {
  local desc="$1" input="$2"
  local out
  out=$(parse_input "$input")
  local got_num
  read -r got_num <<<"$out"
  if [[ -z "$got_num" ]]; then
    printf 'PASS  %s\n' "$desc"
  else
    printf 'FAIL  %s (expected empty NUM, got %q)\n' "$desc" "$got_num"
    FAILURES=$((FAILURES + 1))
  fi
}

FAILURES=0

# numero pelado
assert_parse "bare number"          "24"         "24" ""      ""              ""
assert_parse "hashed number"        "#24"        "24" ""      ""              ""

# pr / issue
assert_parse "lowercase pr N"       "pr 24"      "24" "pr"    ""              ""
assert_parse "uppercase PR N"       "PR 24"      "24" "pr"    ""              ""
assert_parse "issue N"              "issue 7"    "7"  "issue" ""              ""
assert_parse "Issue N"              "Issue 7"    "7"  "issue" ""              ""
assert_parse "pr #N"                "pr #99"     "99" "pr"    ""              ""

# URLs
assert_parse "PR URL same repo"     "https://github.com/chichex/cvm/pull/24" \
  "24" "pr"    "chichex/cvm" "--repo chichex/cvm"
assert_parse "issue URL same repo"  "https://github.com/chichex/cvm/issues/24" \
  "24" "issue" "chichex/cvm" "--repo chichex/cvm"
assert_parse "PR URL other repo"    "https://github.com/anthropics/claude-code/pull/123" \
  "123" "pr"   "anthropics/claude-code" "--repo anthropics/claude-code"
assert_parse "PR URL trailing slash" "https://github.com/foo/bar/pull/9/" \
  "9" "pr"     "foo/bar"     "--repo foo/bar"

# invalid -> NUM empty -> skill aborts
assert_invalid "non numeric"        "abc"
assert_invalid "empty"              ""
assert_invalid "non-github URL"     "https://gitlab.com/foo/bar/pull/24"

# Sanity check: the gh api endpoint constructed in Step 7 is the same for PR
# and issue (both use /repos/.../issues/<num>/comments). This locks in the
# decision documented in the SKILL so future edits cannot silently regress it.
build_post_endpoint() {
  local owner_repo="$1" num="$2"
  printf 'repos/%s/issues/%s/comments\n' "$owner_repo" "$num"
}
endpoint_pr=$(build_post_endpoint "chichex/cvm" "24")
endpoint_issue=$(build_post_endpoint "chichex/cvm" "24")
if [[ "$endpoint_pr" == "$endpoint_issue" && "$endpoint_pr" == "repos/chichex/cvm/issues/24/comments" ]]; then
  printf 'PASS  gh api endpoint identical for PR and issue\n'
else
  printf 'FAIL  gh api endpoint divergence: pr=%q issue=%q\n' "$endpoint_pr" "$endpoint_issue"
  FAILURES=$((FAILURES + 1))
fi

if [[ $FAILURES -eq 0 ]]; then
  echo "OK: all parse cases passed"
  exit 0
else
  echo "FAIL: $FAILURES case(s) failed"
  exit 1
fi
