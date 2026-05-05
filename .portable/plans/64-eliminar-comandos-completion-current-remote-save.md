# Plan: Eliminar comandos completion, current, remote, save, status, nuke de cvm CLI

Refs #64 · https://github.com/chichex/cvm/issues/64

## Contexto

Reducir la superficie de la CLI `cvm` eliminando 6 comandos (`completion`, `current`, `remote`, `save`, `status`, `nuke`) cuya funcionalidad esta cubierta por otros (`status` y `remote ls` ya estan en `ls`; `nuke` esta cubierto por `restore`) o queda fuera del scope core (`completion`, `current`, `save`, `remote add`, `remote rm`). El comando `rm` se preserva (refinamiento del spec).

## Objetivo

Que la CLI `cvm` quede con `add`, `use`, `ls`, `rm`, `pull`, `restore`, `bypass`, `harness`. Los comandos absorbidos quedan cubiertos sin perder funcionalidad observable; los eliminados sin reemplazo desaparecen del binario y del help.

## Approach

1. Borrar archivos `cmd/save.go`, `cmd/status.go`, `cmd/nuke.go`, `cmd/remote.go`. (Notar: `currentCmd` y `saveCmd` viven ambos en `save.go`.)
2. Desregistrar comandos del `init()` de `cmd/root.go` y desactivar el `completion` builtin de Cobra con `rootCmd.CompletionOptions.DisableDefaultCmd = true`.
3. Ajustar el `Long` description del root command (que hoy menciona "nuke everything").
4. Verificar via grep que `internal/remote` y `internal/profile.Current()` siguen siendo usados por comandos preservados (`pull`, `ls`); si no, borrar tambien.
5. Actualizar `README.md` y `CHANGELOG.md` (si existe).
6. Correr `go vet`, `go build`, `go test` para validar.

## Pasos

- [ ] Borrar `cmd/save.go` (incluye `currentCmd` y `saveCmd`)
- [ ] Borrar `cmd/status.go`
- [ ] Borrar `cmd/nuke.go`
- [ ] Borrar `cmd/remote.go`
- [ ] Editar `cmd/root.go`: remover `AddCommand` de los 5 comandos, agregar `rootCmd.CompletionOptions.DisableDefaultCmd = true`, ajustar `Long` description
- [ ] Verificar via `grep -r "internal/remote"` que el paquete sigue usado por `pull` y/o `ls`
- [ ] Verificar via `grep -r "profile.Current"` que el helper sigue usado por `ls`
- [ ] Actualizar `README.md` removiendo referencias a comandos eliminados
- [ ] Actualizar `CHANGELOG.md` (si existe) con entradas `Removed`
- [ ] Verificar que `cmd/add_test.go` no referencia comandos eliminados; ajustar si lo hace
- [ ] Correr `go vet ./... && go build ./... && go test ./...`

## Archivos afectados

- `cmd/save.go` — borrar — define `currentCmd` y `saveCmd`
- `cmd/status.go` — borrar — define `statusCmd`
- `cmd/nuke.go` — borrar — define `nukeCmd`
- `cmd/remote.go` — borrar — define `remoteCmd` + `remoteAddCmd` + `remoteLsCmd` + `remoteRmCmd`
- `cmd/root.go` — modificar — desregistrar comandos en `init()`, desactivar completion builtin, ajustar `Long`
- `README.md` — modificar — remover referencias a comandos eliminados
- `CHANGELOG.md` — modificar — agregar entradas `Removed` (solo si el archivo existe)
- `cmd/add_test.go` — verificar; modificar solo si referencia comandos eliminados

## Riesgos

- `remote add` cae sin reemplazo: usuarios pierden la capacidad de linkear nuevos profiles a repos remotos desde la CLI.
- Si `internal/remote` o `profile.Current()` tienen referencias adicionales no detectadas, el build se rompe (mitigado por `go vet` + `go build`).
- Si tests E2E externos a `cmd/` invocan comandos eliminados, fallaran.
- El output de `ls` (que ya muestra `IN USE`/`idle` y `source`) puede no satisfacer al usuario que esperaba algo distinto al absorber `status`/`remote`.

## Out of scope

- Agregar nuevos comandos o flags (`--json`, `--active-only`, `--remote-only`, etc.).
- Agregar tests nuevos de regresion para `ls`.
- Cortar la release con tag `vX.0.0` (post-merge segun release process actual).
- Modificar comportamiento interno de `ls`, `restore`, `pull` o cualquier comando preservado.
- Refactor del paquete `internal/remote`.

## Asunciones tecnicas validadas

1. Stack: el cambio se hace en Go sobre el codigo Cobra-based actual, sin migrar a otro framework ni reescritura.
2. `completion` es builtin de Cobra: no hay archivo `cmd/completion.go`. Se desactiva con `rootCmd.CompletionOptions.DisableDefaultCmd = true` en `init()` de `cmd/root.go`.
3. `remote add` cae sin reemplazo: junto con el bloque `remote`, el subcomando `remote add` (linkear profile a repo GitHub) tambien desaparece. Se pierde de la CLI la funcionalidad de agregar profiles remotos.
4. `internal/remote` package se conserva: lo siguen usando `pull` y la columna `source` de `ls`. Solo se borran las referencias desde `cmd/remote.go`.
5. `internal/profile.Current()` helper se conserva: lo usa `ls` para detectar el activo. Solo se borra `cmd/save.go` (envoltorio del comando), no el helper interno.
6. `cvm ls` no requiere cambios funcionales: el output actual ya muestra el marker de activo (`IN USE`/`idle`) y la columna `source` con info de remotos. "Absorber `status`/`remote`" significa eliminar los comandos redundantes, no agregar nada nuevo a `ls`.
7. Flag `--harness` de `status` no se migra a `ls`: la vista por-harness queda fuera de la CLI.
8. `restore` cubre `nuke` sin cambios: si la inspeccion revela una diferencia de scope, `restore` se ajusta en este mismo PR (no se abre uno separado).
9. PR monolitico, un solo commit consolidado: un unico PR con todas las eliminaciones agrupadas en un commit.
10. Major version bump post-merge: el bump se hace via `git tag -a vX.0.0 + git push` despues de mergear (release.yml + goreleaser). NO se incluye en este PR.
11. CHANGELOG: si existe `CHANGELOG.md` se agrega entrada `Removed`. Si no existe, se documenta solo en el body de la GitHub release. No se crea archivo nuevo.
12. README + `Long` description del root se actualizan: `README.md` removiendo referencias; el `Long` del root command (que menciona "nuke everything") se ajusta para no mencionarlo.
13. Tests: `cmd/add_test.go` se conserva si no referencia comandos eliminados (verificado via grep). No se agregan tests nuevos.
14. Verificacion: antes de pedir review, se corre `go vet ./... && go build ./... && go test ./...` y se requiere 0 errores.

---

_Plan generado por `/portable-plan` a partir de #64._
