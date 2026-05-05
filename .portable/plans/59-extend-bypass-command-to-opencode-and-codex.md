# Plan: Extend bypass command to OpenCode and Codex (adoptado post-hoc)

Refs #59 · https://github.com/chichex/cvm/pull/59

> **Nota**: este plan fue generado automaticamente por `/portable-recover` a partir del PR existente. No paso por el flujo interactivo de `/portable-plan`. El usuario puede enriquecerlo antes de validar.

## Contexto

El PR extiende `cvm bypass` para operar por harness, manteniendo el formato de Claude, agregando soporte para OpenCode mediante `permission: allow` en `opencode.json`, y soporte para Codex mediante `approval_policy = "never"` y `sandbox_mode = "danger-full-access"` en `~/.codex/config.toml`. El body reporta que `go build ./...`, `go vet ./...` y `go test ./...` pasaron, con nuevas pruebas en `internal/harness/bypass_test.go`.

## Objetivo

Extend bypass command to OpenCode and Codex

## Approach

_(adoptado post-hoc; ver archivos afectados)_

## Pasos

- [x] cmd/bypass.go
- [x] internal/harness/bypass_test.go
- [x] internal/harness/claude.go
- [x] internal/harness/codex.go
- [x] internal/harness/harness.go
- [x] internal/harness/opencode.go
- [x] internal/profile/vanilla_test.go
- [x] profiles/portable/opencode/AGENTS.md

## Archivos afectados

- `cmd/bypass.go`
- `internal/harness/bypass_test.go`
- `internal/harness/claude.go`
- `internal/harness/codex.go`
- `internal/harness/harness.go`
- `internal/harness/opencode.go`
- `internal/profile/vanilla_test.go`
- `profiles/portable/opencode/AGENTS.md`

## Riesgos

_(no relevados; plan adoptado post-hoc)_

## Out of scope

_(no relevado; plan adoptado post-hoc)_

## Asunciones tecnicas validadas

1. Este plan fue generado automaticamente por `/portable-recover`. Los detalles de implementacion se infieren del diff del PR y no fueron validados interactivamente.
