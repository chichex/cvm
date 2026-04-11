Detectar patrones repetidos en la conversacion o KB y proponer nuevos skills automaticamente.

## Proceso

### Paso 1: Detectar patrones
Buscar en dos fuentes:

**A. Conversacion actual:**
- Secuencias de acciones que se repitieron 2+ veces
- Workflows que el usuario pidio de forma similar multiples veces
- Tareas que requirieron multiples pasos manuales y podrian automatizarse

**B. Knowledge Base:**
- Buscar entries con tags similares: `cvm kb ls` y `cvm kb ls --local`
- Agrupar entries relacionadas que sugieran un patron comun
- Identificar gotchas recurrentes que un skill podria prevenir

### Paso 2: Evaluar candidatos
Para cada patron detectado, evaluar:
- **Frecuencia**: se repitio 2+ veces o se espera que se repita?
- **Complejidad**: tiene 3+ pasos que se beneficiarian de automatizacion?
- **Valor**: ahorra tiempo o previene errores significativos?

Descartar patrones que no cumplan al menos 2 de 3 criterios.

### Paso 3: Disenar skill
Para cada candidato viable:
1. Definir nombre en kebab-case
2. Escribir descripcion de una linea
3. Definir pasos del workflow
4. Definir MUST DO / MUST NOT DO
5. Mantener el skill bajo 100 lineas

### Paso 4: Presentar propuesta
Mostrar al usuario:
```
Patron detectado: [descripcion del patron]
Frecuencia: [cuantas veces se repitio]
Skill propuesto: /[nombre]
Descripcion: [una linea]
```

Incluir el contenido completo del SKILL.md propuesto.

### Paso 5: Crear (si aprobado)
Determinar donde guardar:
- Si el skill es **especifico del proyecto**: `cvm override add skill [nombre] --local`
- Si el skill es **global** (aplica a cualquier proyecto): `cvm override add skill [nombre]`

Registrar la creacion en KB:
```bash
cvm kb put "skill-created-[nombre]" --body "Skill /[nombre] creado. Proposito: [descripcion]. Patron origen: [de donde salio]" --tag "evolve,skill" [--local]
```

## MUST DO
- Basarse en evidencia concreta (repeticion real, no especulacion)
- Mantener skills enfocados — un workflow por skill
- Seguir la estructura existente de skills del profile

## MUST NOT DO
- No crear skills especulativos ("por si acaso")
- No crear skills que dupliquen funcionalidad de skills existentes
- No crear sin aprobacion del usuario
