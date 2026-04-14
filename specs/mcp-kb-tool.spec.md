# S-014: MCP KB Tool — KB accesible como MCP server

**Version:** 0.1.0  
**Status:** implemented  
**Validación:** TDD + manual  
**Origen:** Expansión de B-016 (specs/sdd-mem.spec.md). Claude actualmente accede a la KB ejecutando shells a `cvm kb search` y `cvm kb show`. Un MCP server nativo elimina el overhead de process spawning, permite respuestas estructuradas JSON, y habilita a Claude a consultar la KB directamente via tool call sin invocar Bash.

---

## Objetivo

Implementar un MCP server (`cvm-mcp-kb`) que expone dos herramientas — `kb_search` y `kb_get` — permitiendo a Claude Code consultar la Knowledge Base via protocolo MCP en vez de shells. El server lee del mismo almacenamiento que el CLI (misma lógica de `internal/kb`), se registra en `.claude.json` como `mcpServers.cvm-kb`, y se distribuye como binario compilado del módulo `github.com/chichex/cvm`.

---

## Alcance

### Incluido

| ID | Item | Descripción |
|----|------|-------------|
| B-001 | Binario `cvm-mcp-kb` | Nuevo comando/binario que implementa MCP server via stdio transport |
| B-002 | Tool `kb_search` | Expone búsqueda con filtros: query, tags, type, limit, scope |
| B-003 | Tool `kb_get` | Expone lectura de entry por key con scope |
| B-004 | Respuesta JSON estructurada | Los resultados MUST ser JSON, no markdown crudo |
| B-005 | Registro en `.claude.json` | Entrada `mcpServers.cvm-kb` en `profiles/sdd-mem/.claude.json` |
| B-006 | Scope handling | Soporte para global (default) y local (requiere detección de project path) |
| B-007 | Backend compatible | Lee del mismo storage que el CLI; respeta `CVM_KB_BACKEND` si S-013 implementado |

### Excluido

- Herramientas de escritura (`kb_put`, `kb_rm`, etc.) — Claude escribe via CLI, no via MCP
- Autenticación/autorización — el server es local, corre con los permisos del usuario
- Paginación con cursores — se usa `limit` con un máximo razonable
- Hot-reload del storage — el server es stateless, lee del filesystem en cada llamada
- Soporte para múltiples proyectos simultáneos en una sola instancia del server

---

## Contratos

### B-001: Binario `cvm-mcp-kb`

```
Ubicación del source: cmd/mcp-kb/main.go  
Binario compilado: cvm-mcp-kb (en $PATH luego de go install)

Transport: stdio (stdin → JSON-RPC requests, stdout → JSON-RPC responses)
Protocol: MCP (Model Context Protocol) versión 2024-11-05

Startup:
  - El server NO imprime nada a stdout en startup
  - Los logs van a stderr (no a stdout — stdout es del protocolo MCP)
  - Responde a initialize con serverInfo: {name: "cvm-kb", version: "1.0"}

Ciclo de vida:
  - El server corre mientras stdin esté abierto
  - Al recibir EOF en stdin, el server termina limpiamente con exit code 0
  - Si ocurre un error fatal de I/O, termina con exit code 1

Método tools/list:
  Devuelve las 2 tools: kb_search y kb_get (ver B-002, B-003)

Método tools/call:
  Delega a la implementación correspondiente y devuelve el resultado
```

### B-002: Tool `kb_search`

```
Name: "kb_search"
Description: "Search the CVM Knowledge Base for entries matching a query.
              Returns keys, tags, and snippet context for each match."

Input schema:
  {
    "type": "object",
    "properties": {
      "query": {
        "type": "string",
        "description": "Search query (case-insensitive substring match against key and body)"
      },
      "tags": {
        "type": "string",
        "description": "Filter by tag (exact match). Optional."
      },
      "type": {
        "type": "string",
        "enum": ["decision", "learning", "gotcha", "discovery", "session"],
        "description": "Filter by type tag. Optional."
      },
      "limit": {
        "type": "integer",
        "minimum": 1,
        "maximum": 100,
        "default": 20,
        "description": "Maximum number of results to return."
      },
      "scope": {
        "type": "string",
        "enum": ["global", "local"],
        "default": "global",
        "description": "KB scope to search. 'local' requires the server to detect the project path."
      }
    },
    "required": ["query"]
  }

Output (tool result content[0].text): JSON string with structure:
  {
    "results": [
      {
        "key": "my-entry-key",
        "tags": ["decision"],
        "snippet": "...context around the match...",
        "rank": 0,
        "updated_at": "2026-04-13T10:00:00Z"
      }
    ],
    "total": 3,
    "query": "my-entry-key",
    "scope": "global"
  }

When results is empty:
  {
    "results": [],
    "total": 0,
    "query": "notfound",
    "scope": "global"
  }

Rank semantics (from internal/kb.SearchResult):
  0 = exact key match
  1 = key contains query
  2 = body contains query
Results MUST be sorted by rank ascending, then by updated_at descending within same rank.
```

### B-003: Tool `kb_get`

```
Name: "kb_get"
Description: "Retrieve the full content of a Knowledge Base entry by key."

Input schema:
  {
    "type": "object",
    "properties": {
      "key": {
        "type": "string",
        "description": "The exact key of the KB entry to retrieve."
      },
      "scope": {
        "type": "string",
        "enum": ["global", "local"],
        "default": "global",
        "description": "KB scope. 'local' requires project path detection."
      }
    },
    "required": ["key"]
  }

Output (tool result content[0].text): JSON string with structure:
  {
    "key": "my-entry-key",
    "tags": ["decision"],
    "body": "Full markdown body of the entry (frontmatter stripped)",
    "created_at": "2026-04-01T09:00:00Z",
    "updated_at": "2026-04-13T10:00:00Z",
    "scope": "global"
  }

On key not found:
  The tool call MUST return isError: true with content[0].text:
  {
    "error": "key_not_found",
    "key": "nonexistent-key",
    "scope": "global"
  }
```

### B-004: Respuesta JSON estructurada

```
Todo output de tools MUST ser un JSON string válido en content[0].text.
content[0].type MUST ser "text".
El JSON MUST ser parseable (json.Unmarshal sin error).
NO incluir markdown wrappers (sin ```json ... ```).
Campos de tiempo MUST usar RFC3339 (time.RFC3339).
```

### B-005: Registro en `.claude.json`

```
Archivo: profiles/sdd-mem/.claude.json

Entrada a agregar:
  "cvm-kb": {
    "command": "cvm-mcp-kb",
    "args": []
  }

El binario MUST estar en $PATH cuando Claude Code corre (requiere go install o PATH config).
No se incluyen env vars forzadas en la config — el server hereda el entorno del proceso padre.

Resultado final de mcpServers:
  {
    "playwright": { "command": "npx", "args": ["-y", "@playwright/mcp@latest"] },
    "context7":   { "command": "npx", "args": ["-y", "@upstash/context7-mcp@latest"] },
    "cvm-kb":     { "command": "cvm-mcp-kb", "args": [] }
  }
```

### B-006: Scope y project path detection

```
Scope "global":
  - Usa config.GlobalKBDir() — directamente disponible, sin detección
  - Siempre disponible

Scope "local":
  - El server MUST detectar el project path buscando el directorio de trabajo
    actual del proceso padre (env var PWD o os.Getwd())
  - Luego busca .cvm/ subiendo el árbol de directorios (igual que config.LocalKBDir)
  - Si no encuentra .cvm/ local, devuelve error "local_kb_not_found"

Cuando scope no se especifica en la llamada al tool: usar "global" como default.
```

### B-007: Compatibilidad con backend storage

```
El server MUST usar el mismo paquete internal/kb que el CLI.
Las funciones a invocar:
  - kb_search → kb.SearchWithOptions(scope, projectPath, query, opts)
  - kb_get    → kb.Show(scope, projectPath, key)

Si la variable de entorno CVM_KB_BACKEND está definida (futura S-013):
  El server MUST respetarla (la respetará automáticamente si kb.go la respeta).

El server MUST ser stateless: cada tool call hace sus propias operaciones de filesystem.
No se cachea el índice entre llamadas.
```

---

## Behaviors (Given/When/Then)

### B-002-happy: Search con resultados

```
GIVEN KB global contiene entries: "arch-decision-api" (tags: [decision])
  y "api-gateway-notes" (tags: [learning])
AND la query es "api"
WHEN kb_search es llamado con {"query": "api"}
THEN el resultado MUST ser JSON con "total": 2
AND "results" MUST tener 2 elementos con keys "arch-decision-api" y "api-gateway-notes"
AND cada resultado MUST tener "key", "tags", "snippet", "rank", "updated_at"
AND resultados MUST estar ordenados por rank ASC (exacto < contiene < body)
```

### B-002-filter: Search con filtro type

```
GIVEN KB global contiene: "bug-fix-2026" (tags: [gotcha]), "arch-v2" (tags: [decision])
WHEN kb_search es llamado con {"query": "a", "type": "gotcha"}
THEN el resultado MUST incluir SOLO "bug-fix-2026"
AND "total" MUST ser 1
```

### B-002-limit: Search respeta limit

```
GIVEN KB global contiene 50 entries que contienen "common"
WHEN kb_search es llamado con {"query": "common", "limit": 5}
THEN "results" MUST tener exactamente 5 elementos
AND "total" MUST ser 5 (total de resultados devueltos, no del universo)
```

### B-002-empty: Search sin resultados

```
GIVEN KB global no contiene ninguna entry con "xyznotfound"
WHEN kb_search es llamado con {"query": "xyznotfound"}
THEN el resultado MUST ser JSON válido con "results": [] y "total": 0
AND isError MUST ser false
```

### B-003-happy: Get entry existente

```
GIVEN KB global contiene entry "my-decision" con body "Usamos flat files por simplicidad"
  y tags ["decision"]
WHEN kb_get es llamado con {"key": "my-decision"}
THEN el resultado MUST ser JSON con "key": "my-decision"
AND "body" MUST ser "Usamos flat files por simplicidad" (sin frontmatter)
AND "tags" MUST ser ["decision"]
AND "scope" MUST ser "global"
```

### B-003-notfound: Get key inexistente

```
GIVEN KB global no contiene entry "does-not-exist"
WHEN kb_get es llamado con {"key": "does-not-exist"}
THEN isError MUST ser true
AND content[0].text MUST ser JSON con "error": "key_not_found"
AND "key" MUST ser "does-not-exist"
```

### B-006-local: Scope local con proyecto

```
GIVEN el servidor corre en directorio /Users/user/myproject
AND /Users/user/myproject/.cvm/kb/ contiene entry "local-note"
WHEN kb_search es llamado con {"query": "local-note", "scope": "local"}
THEN la búsqueda MUST ocurrir en el KB local del proyecto
AND la entry "local-note" MUST aparecer en los resultados
```

### B-006-local-notfound: Scope local sin .cvm/

```
GIVEN el servidor corre en /tmp/random-dir (sin .cvm/)
WHEN kb_get es llamado con {"key": "anything", "scope": "local"}
THEN isError MUST ser true
AND "error" MUST ser "local_kb_not_found"
```

### E-001: KB global no inicializada

```
GIVEN ~/.cvm/kb/ no existe (KB nunca inicializada)
WHEN kb_search es llamado con cualquier query
THEN el resultado MUST ser JSON con "results": [] y "total": 0
AND NO MUST retornar error (ausencia de KB = KB vacía)
```

### E-002: type inválido en kb_search

```
GIVEN un tool call kb_search con {"query": "x", "type": "invalid-type"}
WHEN el server procesa la llamada
THEN isError MUST ser true
AND "error" MUST ser "invalid_type"
AND "valid_types" MUST listar los tipos válidos: ["decision","learning","gotcha","discovery","session"]
```

### E-003: limit fuera de rango

```
GIVEN un tool call kb_search con {"query": "x", "limit": 200}
WHEN el server valida el input
THEN isError MUST ser true
AND "error" MUST ser "invalid_input"
AND el mensaje MUST mencionar el máximo permitido (100)
```

---

## Invariantes

| ID | Invariante |
|----|-----------|
| I-001 | El server MUST ser stateless: ningún estado persiste entre tool calls |
| I-002 | El server MUST NO escribir en stdout salvo para respuestas JSON-RPC del protocolo MCP |
| I-003 | El server MUST NO modificar el KB (solo lectura): no llama a Put, Remove, SetEnabled |
| I-004 | El server MUST usar el mismo `internal/kb` package que el CLI, sin código de storage duplicado |
| I-005 | Todos los errores de tool MUST retornar isError: true con JSON en content[0].text |
| I-006 | El server MUST terminar limpiamente (exit 0) al recibir EOF en stdin |
| I-007 | El body devuelto por kb_get MUST tener el frontmatter (---\nkey: ...\n---) stripeado |

---

## Errores

| Condición | Comportamiento |
|-----------|---------------|
| KB global no inicializada | Retornar resultados vacíos (no error) — misma semántica que `cvm kb search` con KB vacía |
| Key no encontrada | isError: true, error: "key_not_found" |
| Scope local sin .cvm/ | isError: true, error: "local_kb_not_found" |
| Type inválido | isError: true, error: "invalid_type", valid_types: [...] |
| Limit fuera de rango [1,100] | isError: true, error: "invalid_input" |
| Error de filesystem (permisos) | isError: true, error: "storage_error", message: "..." |
| JSON parse error en stdin | Log a stderr, responder con JSON-RPC error y continuar |

---

## Restricciones no funcionales

| ID | Restricción |
|----|------------|
| NF-001 | El server MUST responder a cada tool call en < 500ms para KBs con hasta 500 entries |
| NF-002 | El binario compilado MUST ser < 15MB |
| NF-003 | No introducir dependencias externas para el MCP transport si se implementa minimal (solo stdlib + internal/kb) |
| NF-004 | Si se usa un MCP Go SDK oficial, la dependencia MUST ser pinnable con go.mod |

---

## Specs relacionadas

| Spec | Relación |
|------|---------|
| S-010 (sdd-mem.spec.md) | Define la KB, el storage format, y las funciones Go que este server consume. B-016 es el origen de este spec. |
| S-013 (SQLite backend, si existe) | Si S-013 introduce CVM_KB_BACKEND, este server lo respeta automáticamente vía internal/kb |

---

## Dependencias

- `github.com/chichex/cvm/internal/kb` — Storage layer (ya implementado)
- `github.com/chichex/cvm/internal/config` — Scope, GlobalKBDir, LocalKBDir (ya implementado)
- MCP Go SDK OR implementación minimal de stdio JSON-RPC (a decidir en Wave 1)

---

## Plan de implementación

### Wave 1 — MCP transport + scaffolding

1. Investigar si existe un MCP Go SDK oficial (`github.com/modelcontextprotocol/go-sdk` o similar)
   - Si existe y está maduro: importarlo como dependencia
   - Si no: implementar minimal stdio transport (initialize + tools/list + tools/call) con `encoding/json`
2. Crear `cmd/mcp-kb/main.go` con el skeleton del server
3. Implementar handlers vacíos para `tools/list`, `tools/call`
4. Test: `echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}' | cvm-mcp-kb` → respuesta válida

### Wave 2 — Tool `kb_search`

1. Implementar `handleKbSearch(params)` usando `kb.SearchWithOptions`
2. Implementar scope detection (global default, local via os.Getwd + subir árbol)
3. Serializar resultado a JSON estructurado (B-004)
4. Tests unitarios para B-002-happy, B-002-filter, B-002-limit, B-002-empty, E-001, E-002, E-003

### Wave 3 — Tool `kb_get`

1. Implementar `handleKbGet(params)` usando `kb.Show`
2. Stripear frontmatter del body (ya disponible via `readBody` — reusar si se exporta, o reimplementar la lógica de parseo)
3. Manejo de errores estructurado (B-003-notfound, B-006-local-notfound)
4. Tests unitarios para B-003-happy, B-003-notfound, I-007

### Wave 4 — Registro e integración

1. Actualizar `profiles/sdd-mem/.claude.json` con entrada `cvm-kb` (B-005)
2. Agregar `cvm-mcp-kb` al `go build` / `go install` del proyecto (Makefile o README)
3. Test manual: `cvm use sdd-mem` → Claude puede llamar `kb_search` y `kb_get` via MCP tool panel
4. Test end-to-end: Claude consulta KB con una query real y recibe resultados JSON

---

## Changelog

| Version | Fecha | Cambios |
|---------|-------|---------|
| 0.1.0 | 2026-04-13 | Draft inicial |
