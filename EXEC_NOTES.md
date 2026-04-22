# EXEC_NOTES — Issue #24

## Cambios aplicados
- Nuevo skill `profiles/lite/skills/check/SKILL.md` — review multi-agente de PR/issue con posteo como comments separados.
- Tabla de skills actualizada en `profiles/lite/CLAUDE.md`.
- Aclaracion en `profiles/lite/CLAUDE.md` sobre la diferencia entre la copia runtime de `~/.claude/CLAUDE.md` (no se edita en sesion) y la fuente del profile en este repo (si se edita, via PR).
- Test harness `profiles/lite/skills/check/parse_input_test.sh` — valida el parsing de los formatos de input documentados (`24`, `#24`, `pr N`, `issue N`, URLs PR/issue de cualquier repo) y bloquea regresiones del endpoint `gh api ... /issues/<num>/comments`.

## Iteracion 1 (cambios pedidos por validador codex)
- Conservar `OWNER/REPO` cuando viene URL y propagarlo a todos los `gh pr|issue view|diff` via `--repo`. Antes, una URL de otro repo habria operado sobre el repo actual.
- Reescribir el snippet de deteccion de tipo: ahora redirige stdout a `/dev/null` y asigna `KIND` solo segun exit code, en vez de mezclar JSON y `echo "pr"`/`echo "issue"`.
- Postear comments con `gh api --method POST repos/<owner>/<repo>/issues/<num>/comments -F body=@<file> --jq .html_url`. Asi la URL viene de la respuesta JSON de la API (contrato estable) y no del stdout no documentado de `gh pr|issue comment`. Como bonus, el endpoint es el mismo para PR y para issue.
- Endurecer `MUST DO` / `MUST NOT DO` con esos tres puntos para que validadores futuros tengan checklist explicito.

## No aplicado (intencional)
- **No modifique `~/.claude/CLAUDE.md`** (el CLAUDE.md global del usuario). Razon: es la copia desplegada del profile y la regla del propio profile prohibe editarla en runtime; ademas vive fuera del worktree y no formaria parte del diff. Tras mergear, redesplegar el profile (via `cvm`) sincroniza `/check` en la copia global.

## Validacion pendiente
- **Validacion end-to-end con un PR real** sigue sin ejecutarse: requiere invocar el skill interactivamente (Claude Code lanzando agentes y posteando comments) y eso lo dispara el usuario. Mitigacion en este iter: el harness `parse_input_test.sh` cubre el parsing y el endpoint del POST, que eran los puntos mas frecuentes de regresion silenciosa.
