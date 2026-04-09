Validacion post-implementacion: lint, tests, slop check, imports no usados. Correr despues de implementar cambios.

## Proceso

### Paso 1: Identificar archivos cambiados
```bash
git diff --name-only HEAD
git diff --name-only --cached
git ls-files --others --exclude-standard  # archivos nuevos no trackeados
```

Si no hay cambios en git, pedir al usuario que indique que archivos validar.

### Paso 2: Lint y type-check
Detectar y correr el linter del proyecto:
- Node: `npm run lint` o `npx eslint <archivos>`
- Go: `go vet ./...` y `golangci-lint run`
- Python: `ruff check` o `flake8`
- Rust: `cargo clippy`

Detectar y correr type-check:
- TypeScript: `npx tsc --noEmit`
- Python: `mypy` o `pyright`

Reportar errores encontrados.

### Paso 3: Tests
Correr tests relacionados con los archivos cambiados:
- Buscar archivos de test correspondientes
- Correr test suite si es rapida (<2 min)
- Si la suite es lenta, correr solo tests afectados

### Paso 4: Slop check
Revisar archivos cambiados buscando:
- Comentarios obvios que restatan el codigo ("set the variable", "return the result")
- Comentarios narrativos ("First, we need to...", "This function...")
- Lineas de separacion inutiles (// -----------)
- `console.log` / `print` de debug olvidados
- `TODO` o `FIXME` introducidos sin intencion

### Paso 5: Imports no usados
Verificar que no quedaron imports sin usar en los archivos cambiados.
- El linter del paso 2 generalmente los detecta
- Si no hay linter, revisar manualmente

### Paso 6: Lectura final
Leer cada archivo cambiado y verificar:
- La logica matchea la intencion
- No hay codigo duplicado introducido
- No hay side effects no intencionados

### Paso 7: Reporte
```
Quality gate:
- Lint: PASS/FAIL [detalles si fallo]
- Type-check: PASS/FAIL/N/A
- Tests: PASS/FAIL [X passing, Y failing]
- Slop: PASS/FAIL [N items encontrados]
- Imports: PASS/FAIL
- Review: PASS/FAIL [observaciones]

Veredicto: PASS / FAIL (N issues)
```

Si hay FAILs, listar los issues especificos y proponer fixes.

## MUST DO
- Correr TODOS los checks, no saltar ninguno
- Reportar resultados honestos aunque haya failures
- Proponer fixes concretos para cada failure

## MUST NOT DO
- No skipear tests que fallan
- No deshabilitar reglas de lint para que pase
- No marcar PASS si hay issues pendientes
