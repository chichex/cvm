# Plan: Upgrade CVM al ultimo release estable

Refs #66 · https://github.com/chichex/cvm/issues/66

## Contexto

Hoy `cvm` se instala via `install.sh` (curl + GitHub Releases) o Homebrew tap, pero no hay forma de actualizar el binario desde el propio CLI. Hace falta un comando `cvm upgrade` que traiga la herramienta al ultimo release estable de forma atomica, sin sudo, sin reiniciar nada y sin tocar configuraciones del usuario.

## Objetivo

Permitir que el usuario corra `cvm upgrade` y obtenga la ultima version estable publicada en GitHub Releases, mostrando version actual vs target, con rollback implicito si algo falla, sin tocar configs ni requerir privilegios elevados.

## Approach

Nuevo subcomando cobra `upgrade` que:

1. Lee `cmd.Version` y consulta `https://api.github.com/repos/chichex/cvm/releases/latest`.
2. Resuelve el path del binario actual con `os.Executable()` y detecta si fue instalado via Homebrew (en cuyo caso aborta sugiriendo `brew upgrade chichex/tap/cvm`).
3. Si las versiones coinciden, sale con `Already on latest version (vX.Y.Z)`.
4. Si difieren, descarga el tarball del release (`cvm_<v>_<os>_<arch>.tar.gz`) con stdlib, lo extrae a `<bin>.new`, codesigna en macOS, y hace `os.Rename` sobre el binario original (rename atomico same-fs).
5. Si falla cualquier paso previo al rename, borra `.new` y el original queda intacto.

No se toca `install.sh` ni ninguna config en `~/.claude`, `~/.config/opencode`, `~/.codex`.

## Pasos

- [ ] Crear `cmd/upgrade.go` con cobra command `upgrade`.
- [ ] Registrar `upgradeCmd` en `cmd/root.go`.
- [ ] Implementar `internal/upgrade/release.go`: cliente GitHub Releases API (`/releases/latest`), parseo de `tag_name`.
- [ ] Implementar `internal/upgrade/install.go`: detect install path (`os.Executable()`), detect Homebrew prefix, write-permission check.
- [ ] Implementar `internal/upgrade/download.go`: HTTP GET tarball, extract con `archive/tar` + `compress/gzip`, codesign en darwin, atomic `os.Rename`.
- [ ] Implementar comparacion de versiones (`vX.Y.Z` string-compare normalizado).
- [ ] Politica para `Version == "dev"`: permitir upgrade con warning explicito.
- [ ] UX output: header con `current` vs `latest`, `Downloading vX.Y.Z...`, `Upgraded to vX.Y.Z` o `Already on latest version (vX.Y.Z)`.
- [ ] Unit tests: version compare, parseo de release JSON, deteccion de Homebrew prefix.
- [ ] Integration test: `httptest.Server` mockeando releases API + tarball generado en memoria; verifica el rename atomico contra un binario dummy.
- [ ] Update `README.md` con seccion "Upgrading".

## Archivos afectados

- `cmd/upgrade.go` — crear — comando cobra `upgrade`.
- `cmd/root.go` — modificar — registrar `upgradeCmd` en `init()`.
- `internal/upgrade/release.go` — crear — cliente GitHub Releases API.
- `internal/upgrade/install.go` — crear — deteccion de install path / Homebrew / permisos.
- `internal/upgrade/download.go` — crear — descarga, extraccion, codesign, rename atomico.
- `internal/upgrade/upgrade_test.go` — crear — unit + integration tests.
- `README.md` — modificar — agregar seccion "Upgrading".

## Riesgos

- Binario instalado via Homebrew: si auto-upgrade pisa el binario gestionado por brew, rompe `brew upgrade` futuro. Mitigacion: deteccion de prefix Homebrew y abort con mensaje claro.
- Path no escribible (ej. `/usr/local/bin` instalado con sudo): el rename va a fallar. Mitigacion: preflight de write-permission y mensaje claro pidiendo reinstall via `install.sh` en `~/.local/bin`.
- macOS Gatekeeper: binario sin codesign queda muerto post-rename. Mitigacion: reusar `codesign --sign - --force` (mismo criterio que `install.sh`).
- API rate-limit de GitHub sin token: 60 req/h por IP. Aceptable para uso esporadico, pero hay que devolver un error legible si se pega.
- `Version == "dev"` (builds locales sin ldflags): el compare daria "siempre upgrade". Mitigacion: politica explicita que permite upgrade con warning.
- Race con el binario corriendo: en macOS/linux el rename sobre un binario en uso es seguro (inode swap), pero conviene documentarlo en el codigo.

## Out of scope

- Flags `--check` (dry-run), `--force` y `--version=X.Y.Z` (selector de version).
- Auto-check periodico o notificaciones de nuevas versiones.
- Canales pre-release / beta.
- Soporte Windows (no esta en goreleaser actual).
- Modificar `install.sh`.
- Auto-upgrade Homebrew (se delega al usuario).

## Asunciones tecnicas validadas

1. Implementacion como nuevo cobra command en `cmd/upgrade.go`, registrado en `cmd/root.go` (mismo patron que `addCmd`/`useCmd`).
2. Fuente de releases: GitHub Releases API publica (`api.github.com/repos/chichex/cvm/releases/latest`), sin token.
3. Path del binario instalado: resuelto con `os.Executable()` en runtime; no hardcodear `~/.local/bin/cvm`.
4. Si el binario vive bajo prefix Homebrew (`/opt/homebrew/...`, `/usr/local/Cellar/...`, `/home/linuxbrew/...`), abortar con mensaje sugiriendo `brew upgrade chichex/tap/cvm` — no auto-upgrade.
5. Si `cmd.Version == "dev"` (build local sin ldflags), permitir upgrade con warning "running dev build, upgrading to vX.Y.Z".
6. Comparacion de versiones: string compare de semver normalizado (`vX.Y.Z`), sin libreria externa de semver.
7. Descarga del tarball: stdlib `net/http`, sin barra de progreso (output simple `Downloading vX.Y.Z...`).
8. Descompresion: stdlib `archive/tar` + `compress/gzip`, sin shell-out a `tar`.
9. Atomicidad: descargar+extraer a `<bin>.new`, codesign (macOS), `os.Rename` sobre el original. Si falla cualquier paso previo al rename, borrar `.new` y el original queda intacto. No hay backup explicito.
10. Codesign en macOS: `codesign --sign - --force` sobre `.new` antes del rename; tolerar fallo si `codesign` no esta disponible (mismo criterio que `install.sh`).
11. Permisos: si el binario esta en un path no escribible (ej. `/usr/local/bin` instalado con sudo), abortar con error claro. No escalar a sudo.
12. Plataformas: darwin/linux x amd64/arm64, detectadas via `runtime.GOOS`/`runtime.GOARCH`. Otras plataformas: error explicito "unsupported platform".
13. Nombre del binario dentro del tarball: `cvm` exacto (matchea goreleaser actual); extraer buscando esa entrada.
14. Sin flags `--force`/`--check`/`--version=X.Y.Z` en este plan — solo el comando base `cvm upgrade`.
15. Sin retries automaticos ante network errors — mensaje claro + exit 1.
16. Testing: unit tests para version compare + parseo de release JSON + deteccion de Homebrew. Integration test con `httptest.Server` y tarball en memoria. Sin e2e real (no se baja binario en CI).
17. `install.sh` no se modifica — sigue siendo la via de bootstrap inicial; `cvm upgrade` reutiliza el mismo formato de URL del tarball.

---

_Plan generado por `/portable-plan` a partir de #66._
