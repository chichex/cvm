# Token Awareness

Gestionar el presupuesto de tokens del contexto de KB:

- El hook `context-inject.sh` inyecta maximo `CVM_CONTEXT_MAX_TOKENS` tokens (default: 2000) al inicio de sesion
- No inyectar contenido de KB adicional que exceda el budget restante
- Usar `cvm kb search` (snippets) antes que `cvm kb show` (contenido completo)
- Si una entry de KB es muy grande (>500 tokens), resumir en vez de citar completa
- Estimar tokens como chars/4 de forma consistente (I-004)
- Al referenciar KB entries en respuestas, usar key + snippet, no el body completo
