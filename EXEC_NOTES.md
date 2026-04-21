# Notas de ejecucion

## Dual location: solo se commitea el del repo

El paso 2 del plan pide replicar `SKILL.md` en `~/.claude/skills/idea/SKILL.md` ademas de `profiles/lite/skills/idea/SKILL.md`. La memory `lite_profile_dual_location` y el paso 8 del plan aclaran que **solo el del repo se versiona**; el de `~/.claude/skills/` se mantiene sincronizado manualmente fuera de git.

Adicionalmente, el sandbox de este worktree no permite escribir fuera del repo (intento de `mkdir -p ~/.claude/skills/idea` fue denegado), asi que la copia a `~/.claude/skills/idea/SKILL.md` queda como **paso manual post-merge**:

```bash
mkdir -p ~/.claude/skills/idea
cp profiles/lite/skills/idea/SKILL.md ~/.claude/skills/idea/SKILL.md
```

Esto es consistente con como vive `/issue`, `/r`, `/pr`, etc — el repo es la fuente de verdad y la copia user-side se sincroniza a mano.

## Smoke tests (mental, sin invocar el skill real)

Validados contra el flujo descrito en `SKILL.md`:

- **Input concreto** `/idea agregar dark mode al dashboard de retros`:
  Glob `**/dashboard*` y Grep `retro` (case-insensitive) detectan `cmd/dashboard.go`, `specs/dashboard.spec.md`, `internal/dashboard/api.go`, `specs/retro-summary.spec.md`, `internal/session/session.go`. Type `feature` (matchea "agregar"), size `m` (≤6 archivos detectados). Body queda con secciones completas.

- **Input vago** `/idea hay algo raro en el flujo`:
  Glob/Grep no detectan archivos relevantes. Type `feature` (default — "raro" no esta en la lista bug). Size `xs` (input corto, 0 archivos). La seccion Notas/warnings recibe el mensaje explicito "input muy vago, no se detectaron archivos — completar en fase de plan".

Smoke test end-to-end real contra GitHub queda como tarea de validacion post-merge (requiere ejecutar el skill desde una sesion con Skill tool disponible; este harness es solo de implementacion).
