#!/usr/bin/env bash
#
# bootstrap-herdr-override.sh
#
# Prepara el override layer de cvm para que la integracion `herdr` <-> claude
# sobreviva a los `cvm pull` / `cvm use` (que borran ~/.claude/hooks y
# ~/.claude/settings.json en CleanManagedItems antes de re-copiar el profile).
#
# Que hace, en orden:
#   1) Verifica binarios: cvm, herdr.
#   2) `herdr integration install claude` para instalar el script de hook en
#      ~/.claude/hooks/herdr-agent-state.sh.
#   3) Copia ese hook a ~/.cvm/global/overrides/<PROFILE>/hooks/ (default
#      PROFILE=harness) para que ApplyOverrides lo restituya en cada re-apply.
#   4) Mergea el bloque `hooks` en ~/.cvm/global/overrides/<PROFILE>/settings.json
#      preservando lo que ya este ahi (ej `permissions`). Los `command` usan
#      "$HOME/.claude/hooks/herdr-agent-state.sh" para ser portables entre
#      maquinas — el shell expande la env var al ejecutar el hook.
#
# Uso:
#   bash scripts/bootstrap-herdr-override.sh [<profile-name>]
#
# Idempotente: se puede correr N veces sin daño.

set -euo pipefail

PROFILE="${1:-harness}"
CVM_HOME="${HOME}/.cvm"
OVERRIDE_DIR="${CVM_HOME}/global/overrides/${PROFILE}"
OVERRIDE_HOOKS_DIR="${OVERRIDE_DIR}/hooks"
OVERRIDE_SETTINGS="${OVERRIDE_DIR}/settings.json"
LIVE_HOOK="${HOME}/.claude/hooks/herdr-agent-state.sh"

log() { printf '[bootstrap] %s\n' "$*" >&2; }
die() { printf '[bootstrap] error: %s\n' "$*" >&2; exit 1; }

# --- 1) Preflight ---

command -v cvm   >/dev/null 2>&1 || die "cvm no esta en PATH"
command -v herdr >/dev/null 2>&1 || die "herdr no esta en PATH (ver https://herdr.dev)"
command -v python3 >/dev/null 2>&1 || die "python3 requerido para mergear settings.json"

if [ ! -d "${CVM_HOME}/global/profiles/${PROFILE}" ] && [ ! -d "${CVM_HOME}/global/profiles" ]; then
  die "no encuentro ~/.cvm/global/profiles — instalaste cvm?"
fi

# --- 2) Instalar / actualizar la integracion claude en herdr ---

log "instalando integracion herdr <-> claude"
herdr integration install claude >/dev/null

[ -x "${LIVE_HOOK}" ] || die "herdr no creo ${LIVE_HOOK} (revisar 'herdr integration status')"

# --- 3) Copiar hook al override layer ---

log "copiando hook al override layer (${OVERRIDE_HOOKS_DIR})"
mkdir -p "${OVERRIDE_HOOKS_DIR}"
cp "${LIVE_HOOK}" "${OVERRIDE_HOOKS_DIR}/herdr-agent-state.sh"
chmod +x "${OVERRIDE_HOOKS_DIR}/herdr-agent-state.sh"

# --- 4) Mergear bloque hooks en settings.json del override ---

log "mergeando bloque hooks en ${OVERRIDE_SETTINGS}"
mkdir -p "${OVERRIDE_DIR}"
[ -f "${OVERRIDE_SETTINGS}" ] || printf '{}\n' > "${OVERRIDE_SETTINGS}"

python3 - "${OVERRIDE_SETTINGS}" <<'PY'
import json, sys

path = sys.argv[1]
with open(path) as f:
    cfg = json.load(f)

cmd = lambda action: f'bash "$HOME/.claude/hooks/herdr-agent-state.sh" {action}'

hook_entry = lambda action: [{
    "matcher": "*",
    "hooks": [{
        "type": "command",
        "command": cmd(action),
        "timeout": 10,
    }],
}]

events = {
    "PermissionRequest": "blocked",
    "PreToolUse":        "working",
    "UserPromptSubmit":  "working",
    "Stop":              "idle",
    "SessionEnd":        "release",
}

cfg.setdefault("hooks", {})
for event, action in events.items():
    cfg["hooks"][event] = hook_entry(action)

with open(path, "w") as f:
    json.dump(cfg, f, indent=2)
    f.write("\n")
PY

# --- 5) Re-aplicar el profile para que el override aterrice en ~/.claude/ ---

log "re-aplicando profile '${PROFILE}' (cvm use)"
cvm use "${PROFILE}" >/dev/null

# --- 6) Verificacion ---

if [ ! -x "${LIVE_HOOK}" ]; then
  die "post-apply: ${LIVE_HOOK} no esta presente"
fi

LIVE_HOOKS_EVENTS="$(python3 -c "import json; print(','.join(json.load(open('${HOME}/.claude/settings.json')).get('hooks', {}).keys()))")"
log "live hook eventos registrados: ${LIVE_HOOKS_EVENTS}"

HERDR_STATUS="$(herdr integration status 2>/dev/null | awk '/^claude:/ {print $2}')"
log "herdr integration status claude: ${HERDR_STATUS}"

log "listo. override en ${OVERRIDE_DIR}"
