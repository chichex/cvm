# Plan: Reducir CLI de cvm al core portable

Refs #62 - https://github.com/chichex/cvm/issues/62

## Contexto

La spec busca reducir la superficie del CLI de `cvm` para hacerlo más portable y simple, preservando los flujos core existentes. Hace falta un plan que separe claramente los comandos core de las funcionalidades accesorias y que evite que comandos no-core sigan expuestos por accidente.

## Objetivo

Implementar una reducción explícita de la superficie del CLI: mantener los comandos core, eliminar las funcionalidades accesorias del CLI principal y su código asociado, actualizar ayuda/documentación, y verificar que los flujos esenciales sigan funcionando.

## Approach

La implementación debe hacer explícita la superficie core del CLI en el registro Cobra y eliminar en la misma pasada el código de comandos no-core. `cmd/root.go` debe quedar como el punto principal para auditar comandos visibles, y cualquier comando que hoy se registre desde su propio `init()` debe revisarse para que no se agregue por fuera del registro core. No se cambia el framework CLI, el formato de perfiles, el estado persistido, ni la resolución de harnesses.

## Pasos

- [ ] Inventariar los comandos actualmente expuestos por `rootCmd`, incluidos los registrados desde `init()` en archivos secundarios.
- [ ] Definir en `cmd/root.go` una superficie core explícita y fácil de auditar.
- [ ] Eliminar del CLI principal los comandos no-core y el código interno asociado a esos comandos.
- [ ] Revisar comandos con auto-registro propio, especialmente `profile` y `upgrade`, para que no se agreguen por fuera del registro core.
- [ ] Mantener funcionando los comandos core de gestión de perfiles, activación, estado, actualización remota, limpieza y restauración.
- [ ] Actualizar `README.md` para documentar solo la superficie core resultante.
- [ ] Ajustar o agregar tests acotados de comandos/help para evitar regresiones en la superficie expuesta.
- [ ] Ejecutar `go test ./...` para validar que el CLI reducido compila y mantiene los tests existentes.

## Archivos afectados

- `cmd/root.go` - modificar - centralizar y auditar la superficie core registrada.
- `cmd/profile.go` - borrar|modificar - eliminar comando no-core o conservar solo partes reutilizables si siguen siendo requeridas por core.
- `cmd/upgrade.go` - borrar|modificar - eliminar auto-registro y código de upgrade si queda fuera del core.
- `cmd/save.go` - borrar|modificar - eliminar `save`/`edit` si quedan fuera del core visible.
- `cmd/override.go` - borrar|modificar - eliminar comando de overrides si queda fuera del core visible.
- `cmd/bypass.go` - borrar|modificar - eliminar comando bypass si queda fuera del core visible.
- `cmd/*_test.go` - crear|modificar - cubrir help/superficie expuesta y ajustar tests afectados.
- `README.md` - modificar - alinear la documentación con el CLI reducido.

## Riesgos

- Comandos registrados desde `init()` pueden seguir apareciendo aunque se limpie `cmd/root.go`.
- Tests existentes pueden depender de comandos que dejen de estar expuestos.
- Eliminar código asociado a comandos no-core puede romper funciones compartidas si no se identifica bien qué internals siguen siendo reutilizados por comandos core.
- Cambios en help/usage pueden afectar scripts que parseen salida del CLI.
- Eliminar comandos en la misma pasada reduce más superficie, pero aumenta el riesgo frente a una estrategia solo de desexposición.

## Out of scope

- Reescribir internals de `profile`, `remote`, `harness` o `state` que no estén ligados a comandos removidos.
- Cambiar el formato de perfiles, manifests, overrides o estado persistido.
- Agregar un sistema de plugins o feature flags.
- Optimizar tamaño binario como objetivo principal.
- Cambiar distribución, release, Homebrew o `install.sh`.
- Regenerar artefactos bajo `dist/`.

## Asunciones tecnicas validadas

1. El proyecto es un CLI Go basado en Cobra, con comandos definidos bajo `cmd/`.
2. La reducción del CLI se implementará principalmente controlando qué comandos se registran en Cobra.
3. La implementación eliminará también el código interno asociado a comandos no-core en la misma pasada.
4. `cmd/root.go` será el punto principal para auditar la superficie visible del CLI.
5. Los comandos que se agregan desde `init()` fuera de `root.go` deben revisarse porque pueden saltarse el registro core.
6. `profile` y `upgrade` requieren atención específica porque actualmente se registran desde sus propios archivos.
7. La implementación mantendrá `github.com/spf13/cobra`; no se migrará a otro framework CLI.
8. No se introducirán dependencias nuevas para resolver esta reducción.
9. Los tests se mantendrán en Go usando `go test`, sin agregar framework externo.
10. La validación mínima incluirá `go test ./...`.
11. El README es la documentación principal que debe reflejar la superficie del CLI.
12. No se cambiará el módulo Go ni la versión declarada en `go.mod`.
13. No se cambiarán los paths de configuración de usuario administrados por `internal/config`, `internal/profile` o `internal/harness`.
14. No se cambiará el modelo de estado persistido en `~/.cvm/state.json`.
15. No se migrarán datos existentes ni se agregarán migraciones.
16. No se cambiará el formato de `cvm.profile.toml`.
17. No se cambiará la resolución de harnesses `claude`, `opencode` y `codex` en esta pasada.
18. La compatibilidad se preservará a nivel de comandos core expuestos, no necesariamente para comandos removidos de la ayuda.
19. Si una parte interna de un comando no-core sigue siendo reutilizada por un flujo core, puede conservarse separada del comando removido.
20. Si un comando no-core tiene tests dedicados, se ajustarán para probar comportamiento interno solo si sigue siendo necesario.
21. La implementación no agregará aliases ocultos salvo que un test o flujo core existente lo justifique.
22. La salida de `cvm --help` será parte de la superficie a verificar.
23. La salida de `cvm help` o comandos equivalentes de Cobra debe quedar alineada con el README.
24. La implementación no tocará perfiles incluidos bajo `profiles/` salvo que la documentación del CLI dentro de esos perfiles quede claramente incorrecta.
25. Los artefactos de release bajo `dist/` no se regenerarán en este plan.
26. El plan asume que el PR de implementación puede modificar varios archivos, pero el PR de plan solo agregará el markdown en `.portable/plans/`.

---

_Plan generado por `/portable-plan` a partir de #62._
