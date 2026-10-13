# Plan por fases para dejar listo el backend — Planillas CCDR Liberia

**Base:** `DIAGNOSTICO_BACKEND_PLANILLAS_CCDR.md` (28-sep-2026) y el código de `modulo_planillas_post_fases.zip`.
**Raíz del proyecto en el zip:** `ccdr-planillas-API-sin-migraciones/` (todas las rutas de este documento son relativas a esa carpeta).
**Módulo Go:** `planillas-ccdr` (Go 1.23, Gin, MySQL, `shopspring/decimal`).

---

## 1. Cómo usar este documento

1. Trabaja las fases **en orden numérico**, salvo que la tabla de la sección 3 diga que pueden ir en paralelo.
2. Para cada fase abre **una conversación nueva** con la IA que quieras y pega, en este orden:
   1. El **Bloque común** (sección 4). Es idéntico para todas las fases.
   2. El **Prompt** de la fase (está dentro de un bloque de código, listo para copiar).
   3. Adjunta **solo** los archivos de la sección *"Archivos a pasar"* de esa fase.
3. **Siempre pasa la versión actual de cada archivo**, es decir, la que quedó después de las fases anteriores. Si una fase anterior modificó un archivo, usa el modificado, no el del zip original.
4. Al terminar cada fase, la IA entrega un **informe de traspaso** (máx. 30 líneas). Guárdalo en `docs/fases/FASE-XX.md`. Si una fase posterior lo lista como requisito, adjúntalo también.
5. Si la IA se queda sin créditos a la mitad, no pasa nada: el Bloque común le exige **priorizar por el orden de las tareas** y entregar lo terminado con una lista de pendientes. Repite la fase con la lista de pendientes.
   **Si una fase te parece demasiado grande para tu IA**, córtala por las tareas numeradas (por ejemplo, tareas 1–3 en una conversación y 4–7 en otra), adjuntando el informe de traspaso de la primera mitad. Las fases F20, F22, F23, F30 y F32 traen puntos de corte sugeridos.
6. Ninguna IA pudo compilar hasta ahora (el diagnóstico lo dice). Las fases F02–F10 verifican **solo su paquete**; la **compilación global real** ocurre en la **F11**, con la salida del compilador en la mano.

### Decisión grande que yo tomé por ti (puedes cambiarla)

El diagnóstico (sección 11) recomienda volver a **sqlc**. Este plan usa la **opción B** (mantener `internal/db/dto` escrito a mano, con una sola convención y un **test de contrato contra MySQL real** en la F13). Razones: reescribir ~5 000 líneas de `dto` como código generado exige toolchain y red que las IA de chat normalmente no tienen, y la F13 atrapa el mismo tipo de errores (columnas inexistentes, estados inexistentes) de forma automática. Si prefieres sqlc, avísame y rehago las fases F02, F13 y las de SQL.

---

## 2. Decisiones vinculantes (ya resueltas; todas las fases las obedecen)

| # | Decisión |
|---|---|
| D1 | **Todos los IDs son `int64`** en Go. La conversión desde/hacia HTTP se hace en el borde (handlers). |
| D2 | **Acceso a datos:** `internal/db/dto` manual. Convención: argumentos posicionales hasta 3 parámetros; desde 4, un struct `XxxParams`. Sin tipos `Row` duplicados. Cada consulta debe pasar el test de contrato de la F13. |
| D3 | **Porcentajes:** en BD se guardan como **porcentaje** (10.83) salvo `parametros_salario_escolar` (fracción, 0.0833). El **dominio Go trabaja siempre con fracción** (0.1083). La conversión se hace **en un único punto**: el cargador de parámetros (F18). Nadie más divide ni multiplica por 100. |
| D4 | **Estados del período** (ENUM real): `borrador`, `calculada`, `aprobada`, `pagada`, `contabilizada`. Constantes en `internal/planilla/estados.go`. Dispersión bancaria: `aprobada` o `pagada`. Aguinaldo, salario escolar y liquidación suman períodos `aprobada`, `pagada` o `contabilizada`. "Último período cerrado" del dashboard: `pagada` o `contabilizada`. |
| D5 | **Fuente única del salario:** `contratos.salario_pactado` del contrato vigente. `empleado_datos_laborales.salario_base_nominal` deja de ser fuente de cálculo. |
| D6 | **Cargas sociales en modelo agregado** (`CCSS_OBR`, `CCSS_PAT` incluye los rubros patronales) hasta que Contabilidad/Legal valide el desglose. **Prohibido** capturar a mano `INS_RT`, `BANPOP_PAT`, `FCL_PAT` (doble conteo). |
| D7 | **Auditoría:** toda escritura pasa por `db.WithAuditContext(ctx, sqlDB, usuarioID int64, func(ctx context.Context, q *dto.Queries) error)`. Los servicios reciben `*sql.DB` (no una función inyectada). |
| D8 | **Cero hardcodeo legal:** todo valor legal vive en BD con vigencia (patrón de 2 pasos: cerrar la fila vigente + insertar la nueva). |
| D9 | **Errores:** de dominio y tipados; al cliente nunca se le devuelve `err.Error()` de errores internos. |
| D10 | **Migraciones:** `db/migrations/` con `golang-migrate`. Cada fase que necesite DDL o seeds crea **su propio archivo** con número reservado `0001NN`, donde `NN` es el número de la fase (ej.: F18 → `000118_parametros_generales.up.sql` y su `.down.sql`). Así dos IA nunca chocan. |
| D11 | Idioma: comentarios, mensajes y errores en **español**. Identificadores existentes no se renombran sin necesidad. |
| D12 | Régimen laboral, % de salario escolar, rubros patronales, base mínima, créditos fiscales: **parametrizados**; su validación legal es trabajo humano (sección 7) y **no bloquea** el desarrollo. |

---

## 3. Mapa de fases

| Fase | Nombre | Requisitos | ¿Paralelo con? |
|---|---|---|---|
| **Etapa 0 — Base** | | | |
| F00 | Base y limpieza del repositorio | — | F01 |
| F01 | Migraciones versionadas | — | F00 |
| **Etapa 1 — Que compile** | | | |
| F02 | Núcleo: auditoría, IDs `int64`, estados | F00 | — |
| F03 | Empleados, contratos y organización | F02 | F04–F10 |
| F04 | Autenticación (compilación) | F02 | F03, F05–F10 |
| F05 | Préstamos y embargos | F02 | F03, F04, F06–F10 |
| F06 | Asistencia | F02 | F03–F05, F07–F10 |
| F07 | Vacaciones | F02 | F03–F06, F08–F10 |
| F08 | Liquidaciones y acumulados (SQL + tipos) | F02 | F03–F07, F09, F10 |
| F09 | Paquete `planilla` y exportación | F02 | F03–F08, F10 |
| F10 | Dashboard y contabilidad | F02 | F03–F09 |
| F11 | Integración: `main.go`, `go mod tidy`, bucle de compilación | F03–F10 | — |
| **Etapa 2 — Que corra** | | | |
| F12 | Permisos, roles, admin inicial y test de coherencia | F11, F01 | — |
| F13 | Pruebas de integración con MySQL y test de contrato SQL | F11, F01 | F12 |
| **Etapa 3 — Seguridad** | | | |
| F14 | Autenticación reforzada (S1, S2, S5, S6, S11) | F12, F13 | F16 |
| F15 | Tokens PASETO (S7, S17) | F14 | — |
| F16 | Endurecimiento HTTP, errores y logging (S8, S9, S15, S16, C3) | F11 | F14 |
| F17 | Usuarios/roles API y autorización a nivel de fila (S10) | F14, F16 | F15 |
| **Etapa 4 — Motor de planilla** | | | |
| F18 | Parámetros: `parametros_generales`, cargador, `Porcentaje` | F13 | F14–F17 |
| F19 | Ciclo de vida del período y segregación de funciones | F12, F18 | — |
| F20 | Orquestador de cálculo y persistencia completa | F18, F19 | — |
| F21 | Incapacidades, prorrateo y fuente única de salario | F20 | No (secuencial: toca el motor) |
| F22 | Horas extra tipificadas y con autorización | F20 (y F21) | No (secuencial: toca el motor) |
| F23 | Deducciones: CCSS con base mínima, embargos, topes y préstamos | F20 (y F21) | No (secuencial: toca el motor) |
| F24 | Aguinaldo y salario escolar dentro de la planilla | F20 | No (secuencial) |
| F25 | Vacaciones dentro de la planilla | F20, F21 | No (secuencial) |
| F26 | Liquidación laboral corregida | F24, F25 | — |
| **Etapa 5/6 — Sin hardcodeo, contabilidad, reportes y cumplimiento** | | | |
| F27 | API de mantenimiento de parámetros legales | F18, F17 | F28–F31 |
| F28 | Contabilidad completa | F20, F23 | F27, F29–F31 |
| F29 | Reportes oficiales (CCSS/SICERE, INS, Hacienda, boleta PDF) | F20 | F27, F28, F30, F31 |
| F30 | Ley 8968: cifrado, enmascaramiento y registro de accesos | F17 | F27–F29, F31 |
| F31 | CI/CD, observabilidad y backups | F13 | F27–F30 |
| **Etapa 7 — Cierre** | | | |
| F32 | Pruebas doradas, QA final y checklist de producción | Todas | — |

Las fases marcadas "paralelo" tocan archivos distintos, por eso pueden ir cada una con una IA diferente al mismo tiempo. Si dos fases paralelas terminaran editando el mismo archivo, la de número menor manda y la otra lo hace después.

---

## 4. BLOQUE COMÚN (pegar al inicio de **cada** fase)

```
ROL
Actúas como analista de software, arquitecto y desarrollador senior (Go). Trabajas sobre el backend
"Planillas CCDR Liberia": módulo Go `planillas-ccdr` (Go 1.23, Gin, MySQL, shopspring/decimal,
argon2id, PASETO). Es un sistema de planillas (nómina) del sector público de Costa Rica: los
errores de cálculo y de seguridad tienen consecuencias legales.

REGLAS DE TRABAJO
1. Edita SOLO los archivos que te adjunto y los nuevos que la fase indique. Si necesitas ver otro
   archivo, dilo y detente en ese punto: NO inventes firmas, columnas ni tipos.
2. Entrega los archivos COMPLETOS modificados (no fragmentos sueltos), cada uno con su ruta.
   Para archivos muy largos y cambios pequeños puedes entregar parches por función, indicando
   con precisión dónde van.
3. Si tienes entorno de ejecución con Go, corre `gofmt -l`, `go vet` y `go build` sobre el
   paquete que tocas y pega el resultado. Si NO lo tienes, revisa a mano y declara
   explícitamente "no verificado con compilador". Nunca afirmes que compila si no lo comprobaste.
4. Los errores de compilación que vengan de OTROS paquetes que aún no se corrigieron no son
   tuyos: anótalos en el informe, no los arregles.
5. PRIORIDAD: haz las tareas en el orden numerado. Si vas a quedarte sin espacio o créditos,
   termina la tarea en curso, entrega lo hecho y lista con claridad lo que faltó.
6. No cambies firmas públicas que usan otros paquetes sin declararlo en el informe. Si las
   cambias, lista firma vieja -> firma nueva.
7. Un solo criterio por tema: no dejes código duplicado, muerto ni comentarios obsoletos
   ("Fase 5B", "VERIFICAR:", menciones a sqlc) en los archivos que toques.
8. Los archivos del proyecto vienen con saltos de línea CRLF: conserva el formato que
   `gofmt` produce y no reformatees líneas que no cambias.

DECISIONES VINCULANTES (no las discutas, aplícalas)
D1  Todos los IDs son int64 en Go. Convierte en el borde HTTP.
D2  Acceso a datos: `internal/db/dto` manual. Argumentos posicionales hasta 3; desde 4, struct
    `XxxParams`. Sin tipos `Row` duplicados.
D3  Porcentajes: en BD son porcentaje (10.83), salvo parametros_salario_escolar (fracción
    0.0833). El dominio Go usa siempre FRACCIÓN (0.1083). La conversión se hace en un único
    punto: el cargador de parámetros. Nadie más multiplica ni divide por 100.
D4  Estados del período (ENUM real): borrador, calculada, aprobada, pagada, contabilizada.
    Constantes en internal/planilla/estados.go. Dispersión: aprobada|pagada. Aguinaldo,
    salario escolar y liquidación suman períodos aprobada|pagada|contabilizada. Dashboard
    "último cerrado": pagada|contabilizada.
D5  Fuente única del salario: contratos.salario_pactado del contrato vigente.
D6  Cargas sociales en modelo agregado (CCSS_OBR, CCSS_PAT). Prohibido capturar a mano
    INS_RT, BANPOP_PAT, FCL_PAT (doble conteo).
D7  Toda escritura pasa por
    db.WithAuditContext(ctx, sqlDB, usuarioID int64, func(ctx context.Context, q *dto.Queries) error).
    Los servicios reciben *sql.DB, no una función inyectada.
D8  Cero hardcodeo legal: valores legales en BD con vigencia (cerrar vigente + insertar nueva).
    Si falta un parámetro, error explícito; nunca un valor por defecto silencioso.
D9  Errores de dominio tipados. Al cliente nunca se le devuelve err.Error() de errores internos.
D10 Migraciones en db/migrations/ (golang-migrate), un archivo propio por fase con número
    reservado 0001NN (NN = número de la fase), con su .up.sql y su .down.sql.
D11 Comentarios, mensajes y errores en español.
D12 Régimen laboral, % salario escolar, rubros patronales, base mínima y créditos fiscales están
    parametrizados; su validación legal es humana y no bloquea.

FORMATO DE ENTREGA (obligatorio, al final de tu respuesta)
"INFORME DE TRASPASO" de máximo 30 líneas con:
- Archivos creados / modificados / eliminados (ruta y una línea de por qué).
- Firmas públicas nuevas o cambiadas (vieja -> nueva).
- Migraciones creadas (nombre).
- Qué verificaste y con qué (compilador, tests, lectura manual).
- Pendientes y riesgos que quedaron.
- Decisiones que necesitas de una persona.
```

---

# ETAPA 0 — BASE

## FASE 00 — Base y limpieza del repositorio

**Contexto.** El repositorio arrastra restos de un intento anterior con sqlc, documentación que se contradice (`ADR-000`, `ORGANIZACION.md`, `CICD.md` mencionan archivos que no existen), un `.env.example` que en realidad es código Go copiado, un script de pruebas que apunta a rutas inexistentes y un `go.mod` con una nota larga obsoleta. Esta fase deja el terreno limpio sin tocar código de negocio.

**Requisitos:** ninguno. **Se puede hacer en paralelo con F01.**

**Prompt para la IA**
```
FASE 00 — Base y limpieza del repositorio.

OBJETIVO
Dejar el repositorio limpio y coherente antes de arreglar código: eliminar restos de sqlc,
consolidar la documentación de arquitectura en un solo documento vigente, crear un .env.example
real y arreglar el script de pruebas.

TAREAS (en este orden)
1. ELIMINAR (solo lista de rutas a borrar; no necesitas leer su contenido): sqlc.yaml,
   scripts/gen_sqlc_schema.sh, db/schema.sqlc.sql y los 7 archivos de internal/db/queries/
   (acumulados_liquidaciones.sql, asistencia_vacaciones.sql, dashboard.sql,
   empleados_puestos.sql, planilla_export.sql, prestamos_embargos.sql, security.sql).
   Motivo: sqlc ya no se usa y son una segunda fuente de verdad desalineada.
2. CREAR .env.example REAL (reemplaza el actual, que es código Go). Lee internal/config/config.go
   y lista TODAS las variables de entorno/claves que consume, agrupadas (APP, SERVER, DATABASE,
   AUTH/PASETO, etc.), con valores de ejemplo inofensivos y un comentario de una línea por
   variable (qué es, formato, ejemplo). PASETO_LOCAL_KEY_HEX debe quedar con un placeholder
   claramente falso e indicar cómo generarla (32 bytes aleatorios en hex = 64 caracteres).
   No pongas ningún secreto real.
3. GO.MOD: elimina el comentario largo "NOTA (auditoría de producción...)". Deja la directiva
   `go` coherente con Go 1.23. NO cambies ni quites dependencias (eso ocurre en otras fases).
4. run_full_test_suite.sh: corrígelo para que (a) diga la ruta real (raíz del proyecto), (b) no
   ejecute test/integration/ si esa carpeta no existe (deja un bloque comentado y una variable
   para activarlo cuando exista), (c) exija Go 1.23 y (d) ejecute en orden: gofmt -l (falla si
   lista archivos), go vet ./..., go build ./..., go test ./... -count=1.
5. CONSOLIDAR DOCUMENTACIÓN: crea docs/ADR-001-arquitectura-vigente.md que reemplace a
   ADR-000_decisiones_arquitectura.md, ORGANIZACION.md y CICD.md. Debe reflejar SOLO lo vigente
   y las decisiones D1–D12 del bloque común (IDs int64, dto manual, porcentajes, estados,
   auditoría, migraciones golang-migrate, etc.). Elimina toda mención a sqlc "presente",
   a AUDITORIA.md y plan.md (no existen), a "fases" numeradas antiguas y a migrations/ como
   carpeta inexistente. Después borra los tres documentos antiguos.
   Incluye en el ADR una sección "Estructura de carpetas" real (mírala en el árbol que te paso).
6. Crea docs/fases/README.md explicando la convención: cada fase deja un informe de traspaso en
   docs/fases/FASE-XX.md.
7. Crea .gitignore (binarios, .env, *.log, coverage, /tmp) si no existe.

CRITERIOS DE ACEPTACIÓN
- No queda ninguna referencia a sqlc, AUDITORIA.md ni plan.md en ningún archivo entregado.
- .env.example contiene solo pares CLAVE=valor y comentarios (no código Go).
- Hay un único documento de arquitectura (docs/ADR-001-arquitectura-vigente.md).
No toques ningún archivo .go.
```

**Archivos a pasar**
- `go.mod`
- `ADR-000_decisiones_arquitectura.md`, `ORGANIZACION.md`, `CICD.md`
- `run_full_test_suite.sh`
- `.env.example` (el actual, solo para reemplazarlo)
- `internal/config/config.go` (240 líneas; **solo lectura**, de aquí salen las variables)
- `sqlc.yaml`, `scripts/gen_sqlc_schema.sh` (para eliminarlos)
- Además, pega el **árbol de carpetas** del proyecto (salida de `find . -type f | sort`).

**Archivos que crea:** `docs/ADR-001-arquitectura-vigente.md`, `docs/fases/README.md`, `.gitignore`, nuevo `.env.example`.
**Archivos que elimina:** `sqlc.yaml`, `scripts/gen_sqlc_schema.sh`, `db/schema.sqlc.sql`, `internal/db/queries/*.sql` (7), `ADR-000_decisiones_arquitectura.md`, `ORGANIZACION.md`, `CICD.md`.

---

## FASE 01 — Migraciones versionadas

**Contexto.** `db/schema.sql` (4 379 líneas) mezcla DDL, 204 triggers de auditoría, datos semilla y hasta una contraseña de admin, y se auto-borra las tablas al re-ejecutarse (`DROP TABLE IF EXISTS`). No se puede aplicar un cambio anual de parámetros (ej.: renta 2027) sin riesgo. Hay que partirlo en migraciones versionadas. **La parte mecánica ya está resuelta con un script que te entrego** (`scripts/split_schema_a_migraciones.sh`): corta el archivo por los marcadores `Origen:` que ya trae, quita `CREATE DATABASE`, el guard de versión y el `DROP TABLE`, y convierte `DELIMITER $$` en SQL válido para el driver. Yo lo probé: genera 7 migraciones, 204 triggers (191 + 13) y 84 tablas, que coinciden con el original.

**Requisitos:** ninguno. **Paralelo con F00.**

**Prompt para la IA**
```
FASE 01 — Migraciones versionadas.

CONTEXTO
Ya corrí scripts/split_schema_a_migraciones.sh sobre db/schema.sql y generó en db/migrations/:
000000_esquema_base, 000001_seguridad_reforzada, 000002_seed_catalogos,
000003_calendario_asistencia_vacaciones, 000004_acumulados_liquidaciones,
000005_seed_catalogos_parametricos, 000006_rol_admin_y_permisos_base (todos .up.sql) y el
insumo _drop_tablas_original.txt. NO reescribas esos .up.sql a mano (son miles de líneas).

TAREAS (en este orden)
1. Revisa el script y confirma con lectura que su lógica es correcta (rangos, conversión de
   DELIMITER $$ -> ';'). Señala cualquier riesgo. Recuerda que el driver mysql de
   golang-migrate necesita `multiStatements=true` en la URL; el servidor MySQL sí entiende
   CREATE TRIGGER ... BEGIN ... END; dentro de multi-statement, pero DELIMITER solo lo entiende
   el cliente `mysql`. Si detectas un caso que no funcionaría, propón la corrección al script.
2. Crea el .down.sql de cada migración:
   - 000000: SET FOREIGN_KEY_CHECKS=0; DROP TABLE IF EXISTS <lista de _drop_tablas_original.txt>;
     SET FOREIGN_KEY_CHECKS=1;
   - 000001, 000003, 000004: DROP de las tablas que ese archivo crea (en orden inverso de
     dependencias) y de las columnas que agrega, si agrega.
   - 000002, 000005, 000006 (solo datos): un comentario "migración de datos: no reversible" y
     un DELETE de las filas solo si es seguro por clave natural; si no, déjalo como no-op
     documentado.
3. Crea cmd/migrate/main.go: un comando pequeño que use github.com/golang-migrate/migrate/v4
   (drivers mysql y file) con subcomandos up, down N, version y force V, leyendo la URL de
   conexión de las variables de entorno definidas en internal/config (o DATABASE_URL). Debe
   forzar multiStatements=true, imprimir la versión antes/después y fallar con código != 0 ante
   error. NO debe correr automáticamente al iniciar la API.
4. Crea docker-compose.yml (solo para desarrollo/CI) con MySQL 8.0 (imagen oficial) y volumen,
   más un Makefile (o scripts/) con: migrate-up, migrate-down, db-reset (solo dev).
5. Crea docs/migraciones.md: convención de nombres, regla D10 (número reservado 0001NN por
   fase: 000102 para F02, etc.), cómo hacer el cambio anual de parámetros (nueva migración de
   datos que cierra la vigencia anterior e inserta la nueva; NUNCA `ON DUPLICATE KEY UPDATE` que
   pise valores), y cómo probar up/down/up en local.
6. Lista (sin corregir) los INSERT con `ON DUPLICATE KEY UPDATE` que encuentres en
   000002/000005/000006: el diagnóstico dice que pisan valores sin historial. Solo repórtalos con
   archivo y línea aproximada; los corregirá F12 y F18.
7. NO borres db/schema.sql todavía: renómbralo a db/legacy/schema.sql.OLD (referencia) y agrega
   una nota en docs/migraciones.md de que se elimina al cerrar F13.

CRITERIOS DE ACEPTACIÓN
- `migrate up` sobre una MySQL 8 vacía deja las 84 tablas y 204 triggers; `down` las quita; `up`
  otra vez funciona (si tienes MySQL disponible; si no, declara "no ejecutado").
- Ninguna migración contiene CREATE DATABASE, USE, DROP TABLE de idempotencia ni DELIMITER.
```

**Archivos a pasar**
- `scripts/split_schema_a_migraciones.sh` (te lo entrego junto a este documento).
- **Ejecuta el script tú antes de abrir la conversación** y pasa: `db/migrations/_drop_tablas_original.txt` y las **primeras 60 y últimas 30 líneas** de cada `.up.sql` generado (basta para verificar cortes). No hace falta pasar `schema.sql` completo.
- `internal/config/config.go` (solo lectura, para el nombre de las variables de BD).

**Archivos que crea:** `db/migrations/*.down.sql`, `cmd/migrate/main.go`, `docker-compose.yml`, `Makefile`, `docs/migraciones.md`.
**Archivos que mueve:** `db/schema.sql` → `db/legacy/schema.sql.OLD`.

> **Cómo ejecutar el script:** desde la raíz del proyecto, `bash scripts/split_schema_a_migraciones.sh`. Si tu `schema.sql` ya no es el del zip, aborta solo (verifica marcadores).

---

# ETAPA 1 — QUE COMPILE

> En las fases F03–F10 cada IA verifica **solo su paquete**. Los errores que aparezcan en paquetes ajenos se anotan, no se corrigen. La compilación global es la F11.
AQUIIIIIIIIIII
## FASE 02 — Núcleo: auditoría, IDs `int64` y estados del período

**Contexto.** Es la fase que desbloquea a todas las demás. Tres problemas de raíz: (1) `WithAuditContext` deja `@auditoria_usuario_id` puesto en la conexión del pool (S3 del diagnóstico): la siguiente escritura fuera del wrapper queda atribuida a otro usuario; (2) los IDs están mezclados entre `uint64` e `int64` en `dto/empleados`, `dto/contratos`, `dto/organizacion`, `dto/security` y `middleware`; (3) los estados del período están como strings sueltos y algunos no existen en el ENUM.

**Requisitos:** F00 (recomendado, no bloqueante).

**Prompt para la IA**
```
FASE 02 — Núcleo: auditoría, IDs int64 y estados del período.

TAREAS (en este orden)
1. internal/db/audit.go — endurecer WithAuditContext SIN cambiar su firma
   (ctx, sqlDB *sql.DB, usuarioID int64, fn AuditedFunc) donde
   AuditedFunc = func(ctx context.Context, q *dto.Queries) error:
   a) Tomar una conexión dedicada (sqlDB.Conn(ctx)), abrir la transacción sobre ELLA y ejecutar
      SET @auditoria_usuario_id = ? en esa conexión.
   b) Al terminar (commit, rollback o panic) y ANTES de devolver la conexión al pool, ejecutar
      SET @auditoria_usuario_id = NULL sobre la misma conexión, con un contexto NO cancelado
      (context.WithoutCancel + timeout corto), porque el ctx original puede estar cancelado.
   c) Si ese reset falla, DESCARTAR la conexión del pool (conn.Raw(...) devolviendo
      driver.ErrBadConn) en lugar de reutilizarla.
   d) Mantener el re-lanzamiento de panics tras el rollback y los mensajes de error en español.
   e) Documentar en el comentario del paquete la regla: "toda escritura pasa por aquí".
2. IDs int64 (decisión D1). En internal/db/dto/empleados.go, contratos.go, organizacion.go y
   security.go, cambia TODO ID y FK de uint64 a int64 (campos de structs, parámetros, retornos,
   Scan y LastInsertId). No cambies nombres de funciones ni de columnas.
3. internal/api/middleware/context.go:
   - UsuarioIDDesdeContexto y UsuarioID devuelven int64 (0/false si no hay).
   - Agrega ParseIDParam(c *gin.Context, nombre string) (int64, error): valida que sea entero
     > 0 y NO escribe la respuesta; devuelve un error tipado ErrParametroInvalido (crea el tipo
     si no existe) que ErrorHandler traducirá a 400.
   - Deja ParseUintParam marcado con "// Deprecated: usar ParseIDParam" SIN borrarlo (los
     handlers de otras fases migran uno por uno; la F16 lo elimina).
4. Crea internal/planilla/estados.go con: type EstadoPeriodo string y las constantes
   EstadoBorrador="borrador", EstadoCalculada="calculada", EstadoAprobada="aprobada",
   EstadoPagada="pagada", EstadoContabilizada="contabilizada"; método Valido();
   y funciones EstadosParaDispersion() = [aprobada, pagada],
   EstadosParaAcumulados() = [aprobada, pagada, contabilizada], EstadosCerrados() = [pagada,
   contabilizada]. Añade helper que arme "IN (?,?,...)" con sus argumentos para usar en SQL.
   Nada de lógica de transiciones (eso es F19).

CRITERIOS DE ACEPTACIÓN
- No queda "uint64" en los cuatro dto ni en middleware/context.go, salvo ParseUintParam.
- go vet ./internal/db/... ./internal/api/middleware/... sin errores propios (los de otros
  paquetes se anotan).
- audit.go: el reset a NULL ocurre en todos los caminos (éxito, error, panic).
```

**Archivos a pasar**
- `internal/db/audit.go` (85 líneas)
- `internal/db/dto/db.go` (50)
- `internal/db/dto/empleados.go` (407), `contratos.go` (234), `organizacion.go` (442), `security.go` (419)
- `internal/api/middleware/context.go` (79)

**Archivos que crea:** `internal/planilla/estados.go`.
**Migraciones:** ninguna.

---
ahooraaaaaaa
## FASE 03 — Empleados, contratos y organización

**Contexto.** Los tres servicios (`empleado_service`, `contrato_service`, `organizacion_service`) llaman a `WithAuditContext` con callbacks sin `ctx` y con `usuarioID` `uint64` (23 llamadas en total), usan `services.PaginacionRequest` (duplica `internal/pagination`, con límites distintos 20/200 vs 20/100) y `contrato_service.go:350` tiene un parche `var _ = time.Now`. El diagnóstico **no leyó línea por línea** `contrato_service`, `organizacion_service` ni los handlers: esta fase debe revisarlos.

**Requisitos:** F02.

**Prompt para la IA**
```
FASE 03 — Empleados, contratos y organización.

TAREAS (en este orden)
1. Compilación de los tres servicios (services/empleado_service.go, contrato_service.go,
   organizacion_service.go):
   - Cada llamada a db.WithAuditContext usa la firma real:
     db.WithAuditContext(ctx, s.db, usuarioID, func(ctx context.Context, q *dto.Queries) error {...})
     con usuarioID int64 y s.db *sql.DB (los servicios ya reciben sqlDB).
   - IDs int64 en toda la capa (structs de entrada/salida, parámetros, retornos).
   - Elimina "var _ = time.Now" y el import huérfano de contrato_service.go.
2. Paginación: elimina services.PaginacionRequest y usa el paquete internal/pagination en todos
   los servicios y handlers. Un solo criterio: default 20, máximo 100. Si internal/pagination
   necesita un método más, agrégalo ahí.
3. Handlers (handlers/empleado_handler.go, contrato_handler.go, organizacion_handler.go):
   - Reemplaza ParseUintParam por middleware.ParseIDParam (int64) y NO respondas dos veces.
   - middleware.UsuarioID(c) ya devuelve int64.
   - Los errores salen por middleware.FailFromError; nunca c.JSON con err.Error() interno.
   - NO cambies los códigos de permiso (los reemplaza la F12).
4. REVISIÓN de lo que el diagnóstico no leyó: recorre los tres servicios y los tres handlers
   buscando: errores ignorados, transacciones sin rollback, lecturas fuera de la transacción
   seguidas de escrituras (condiciones de carrera), validaciones que falten (fechas de vigencia
   traslapadas, cédula/identificación duplicada, salario <= 0), y SQL que use columnas que no
   existan (compáralo con el DDL que te adjunto). Corrige lo claro y reporta lo dudoso.
5. Verifica que ValidarSalarioMinimo (o equivalente) esté DEFINIDO y que se use el parámetro
   de salario mínimo vigente; si falta, impleméntalo con el patrón del proyecto y repórtalo. Si
   no existe salario mínimo para la clasificación del puesto, devuelve un error explícito y
   tipado (D8), no un valor por defecto.
6. NO toques la lógica de fuente única de salario (D5): eso es F22. Solo deja un comentario
   "TODO F22" donde el servicio escriba salario_base_nominal.

CRITERIOS DE ACEPTACIÓN
- No queda uint64 ni PaginacionRequest en estos archivos.
- go vet ./internal/services/... ./internal/api/handlers/... : los únicos errores que queden
  deben ser de LiquidacionService/DashboardService (otras fases) y lo declaras en el informe.
```

**Archivos a pasar**
- `internal/services/empleado_service.go` (423), `contrato_service.go` (350), `organizacion_service.go` (352)
- `internal/api/handlers/empleado_handler.go` (258), `contrato_handler.go` (159), `organizacion_handler.go` (389)
- `internal/pagination/pagination.go`
- `internal/db/dto/empleados.go`, `contratos.go`, `organizacion.go` (**versión post-F02**)
- **Solo lectura:** `internal/db/audit.go`, `internal/api/middleware/context.go` (post-F02)
- Fragmento de DDL (en `db/legacy/schema.sql.OLD`, o en `db/migrations/000000_esquema_base.up.sql`): tablas `empleados`, `empleado_datos_personales`, `empleado_datos_laborales`, `empleado_datos_bancarios`, `empleado_datos_familiares`, `contratos`, `unidades_organizativas`, `programas_presupuestarios`, `puestos`, `clasificaciones_puesto`, `parametros_salario_minimo` (en `schema.sql` original: líneas 200–215, 266–310, 325–436, 481–512, 776–797).

---
yaaaaaaaa
## FASE 04 — Autenticación (compilación)

**Contexto.** `auth/service.go` llama a `WithAuditContext` con dos firmas incorrectas (líneas 261 y 277, usa `sqlc.Queries` y le faltan `sqlDB` y `ctx`), el `Servicio` no tiene `*sql.DB`, y los IDs son `uint64`. Aquí **solo se hace compilar**; el endurecimiento de seguridad va en F14/F15 para no mezclar riesgos.

**Requisitos:** F02.

**Prompt para la IA**
```
FASE 04 — Autenticación: que compile (sin cambiar comportamiento de seguridad).

TAREAS (en este orden)
1. auth/service.go: NuevoServicio recibe también *sql.DB (guárdalo en el Servicio). Corrige las
   dos llamadas a WithAuditContext (aprox. líneas 261 y 277): firma real, callback
   func(ctx context.Context, q *dto.Queries) error, tipo dto.Queries (no sqlc), usuarioID int64.
2. IDs int64 en internal/auth/dto.go, service.go, handlers/auth_handler.go,
   internal/security/paseto.go (Payload.UsuarioID y toda su cadena de conversión) y
   middleware/auth.go. Ajusta los claims del token en consecuencia (el valor sigue siendo el
   mismo número; solo cambia el tipo Go).
3. La interfaz Store de auth debe coincidir EXACTAMENTE, nombre y firma, con los métodos de
   *dto.Queries (internal/db/dto/security.go). Si falta un método, agrégalo a dto/security.go
   con el estilo del archivo (D2).
4. Elimina auth.PoliticaPorDefecto() (duplica los defaults de internal/config; la política debe
   venir solo de config). Implementa auth.ErrRolYaVigente: AsignarRol lo devuelve cuando el
   usuario ya tiene ese rol vigente, y el handler lo traduce a 409.
5. En handlers/auth_handler.go usa middleware.ParseIDParam y FailFromError; nunca err.Error()
   interno al cliente. NO cambies los códigos de permiso (F12).
6. NO cambies: hashing, TTLs, bloqueo por intentos, rotación de refresh tokens, reset de
   contraseña. Todo eso es F14/F15. Anota con "// F14:" o "// F15:" los puntos que los
   afecten, sin implementarlos.

CRITERIOS DE ACEPTACIÓN
- go vet ./internal/auth/... ./internal/security/... ./internal/api/middleware/... sin errores
  propios.
- Ninguna llamada a WithAuditContext con firma distinta de la real.
```

**Archivos a pasar**
- `internal/auth/dto.go`, `errors.go`, `service.go` (390)
- `internal/security/paseto.go` (126), `hash.go` (71)
- `internal/api/middleware/auth.go` (81), `context.go` (post-F02)
- `internal/api/handlers/auth_handler.go` (179)
- `internal/db/dto/security.go` (post-F02)
- **Solo lectura:** `internal/db/audit.go` (post-F02), `internal/config/config.go`

---
nowwwwwww

## FASE 05 — Préstamos y embargos

**Contexto.** `prestamos/service.go` tiene 6 llamadas a `WithAuditContext` con callback sin `ctx`, y `AplicarAbono` y `CambiarEstadoEmbargo` **leen fuera de la transacción y luego escriben** (condición de carrera: dos abonos simultáneos pueden dejar el saldo mal). Tampoco valida coherencia (cuota × cuotas ≈ monto, cuota ≤ monto, empleado activo). El diagnóstico no verificó que `*dto.Queries` cumpla `prestamos.Querier`.

**Requisitos:** F02.

**Prompt para la IA**
```
FASE 05 — Préstamos y embargos.

TAREAS (en este orden)
1. prestamos/service.go: corrige las 6 llamadas a WithAuditContext (callback con ctx, firma real,
   usuarioID int64). El servicio ya recibe *sql.DB.
2. Verifica, método por método, que *dto.Queries satisface la interfaz prestamos.Querier
   (nombres, argumentos, tipos, retornos). Corrige la interfaz o agrega los métodos que falten
   en dto/prestamos_embargos.go (estilo D2). Lista en el informe cada desajuste.
3. Condiciones de carrera: AplicarAbono y CambiarEstadoEmbargo deben hacer la lectura DENTRO de
   la transacción de WithAuditContext con SELECT ... FOR UPDATE (agrega la consulta en dto si
   hace falta), y validar el estado antes de escribir. Revisa el resto de operaciones con el
   mismo patrón leer-luego-escribir.
4. Validaciones de coherencia al crear un préstamo: monto > 0, cuota > 0, cuota <= monto,
   número de cuotas coherente con monto/cuota (tolerancia definida por una constante local con
   comentario, NO valor legal), empleado activo y con contrato vigente, sin otro préstamo
   idéntico duplicado. Errores de dominio tipados.
5. handlers.go y routes.go: ParseIDParam (int64), sin doble respuesta, sin err.Error() interno
   al cliente. NO cambies códigos de permiso (F12).
6. NO implementes aquí: descuento de saldo al cerrar planilla ni embargos acumulados (F23).

CRITERIOS DE ACEPTACIÓN
- go vet ./internal/prestamos/... sin errores propios.
- Ninguna lectura previa a una escritura queda fuera de la transacción.
```

**Archivos a pasar**
- `internal/prestamos/service.go` (311), `handlers.go` (396), `routes.go`
- `internal/db/dto/prestamos_embargos.go` (493)
- **Solo lectura:** `internal/db/audit.go`, `internal/api/middleware/context.go` (post-F02)
- DDL (en `schema.sql` original): `terceros`, `prestamos`, `embargos`, `planilla_detalle_embargo`, `parametros_tipos_embargo` (líneas 643–657, 1060–1152).

---
yyyeyyeyeye
## FASE 06 — Asistencia

**Contexto.** `asistencia.NewService(queries, withAuditCtx)` espera una función de 3 parámetros que `main.go` no puede pasarle (la real tiene 4). Además: `UpsertRegistroAsistencia` y `CrearIncapacidad` escriben **sin auditoría** (S3), el cálculo de horas usa `float64` (`decimal.NewFromFloat`) contra el principio del ADR, no soporta turnos que cruzan la medianoche (L14), guarda `Observaciones` vacío en vez de `NULL`, y `PUT /turnos/:id` responde 501. La tipificación de horas extra (diurna/nocturna/mixta, autorización) **no** es de esta fase: es F22.

**Requisitos:** F02.

**Prompt para la IA**
```
FASE 06 — Asistencia.

TAREAS (en este orden)
1. Constructor: asistencia.NewService recibe (sqlDB *sql.DB, queries Queries, opts Opciones) en
   lugar de la función inyectada withAuditCtx (decisión D7). Toda escritura (incluidas
   UpsertRegistroAsistencia y CrearIncapacidad, que hoy van directo por s.queries) pasa por
   db.WithAuditContext con callback (ctx, q *dto.Queries) y usuarioID int64.
   Opciones contiene ToleranciaTardanzaMin (int) SIN valor por defecto: si es < 0 el
   constructor devuelve error. (Hoy no está parametrizada; cuando exista parametros_generales
   en F18, main.go la leerá de ahí; mientras tanto vendrá de config.)
2. IDs int64 (incluye CreadoPor/AprobadoPor y los campos que hoy reciben middleware.UsuarioID).
3. Cálculo de horas SIN float64: trabaja con minutos enteros (int) y conviértelos a
   decimal.Decimal dividiendo con decimal (Round a 2). Elimina decimal.NewFromFloat.
   Si tras el cambio nadie más usa internal/shared/decimal/decimal.go (ToNumeric), bórralo y
   dilo en el informe; si alguien más lo usa, déjalo y lista quién.
4. Turnos que cruzan la medianoche (salida < entrada): la duración programada y la real deben
   calcularse sumando 24 h cuando corresponda. Agrega tests (table-driven) para: turno diurno,
   nocturno 22:00–06:00, entrada tardía, salida anticipada, exceso sobre jornada.
5. Observaciones: guarda NULL (sql.NullString) cuando viene vacío.
6. Implementa PUT /turnos/:id (handler.actualizarTurno): DTO de actualización con validación,
   servicio con WithAuditContext, 404 si no existe, y sin permitir cambiar un turno con
   asignaciones vigentes de manera que rompa el histórico (si aplica, bloquea con 409).
7. handler.go / filtros.go / calendario.go: ParseIDParam, FailFromError, sin doble respuesta.
   NO cambies códigos de permiso (F12).
8. Deja "horas extra" como CANDIDATAS: el campo calculado debe llamarse/documentarse como
   "exceso sobre jornada, pendiente de autorización y tipificación (F22)". No la conectes a
   tipos de hora extra.

CRITERIOS DE ACEPTACIÓN
- Ninguna escritura sin WithAuditContext en el paquete.
- No queda float64 en internal/asistencia.
- go vet ./internal/asistencia/... sin errores propios (vacaciones_* son de F07: sus errores
  se anotan).
```

**Archivos a pasar**
- `internal/asistencia/service.go` (248), `handler.go` (319), `calendario.go` (134), `filtros.go` (79)
- `internal/db/dto/asistencia.go` (842)
- `internal/shared/decimal/decimal.go`
- **Solo lectura:** `internal/db/audit.go`, `internal/api/middleware/context.go` (post-F02)
- DDL (`schema.sql` original): `turnos`, `empleado_turno`, `registro_asistencia`, `tipos_ausencia`, `ausencias` (líneas 852–920) y `turnos`, `incapacidades`, `registro_asistencia_detalle` del bloque 000003 (líneas 3678–3772).

---
Abakkkkkkkkkkkkkkkkkkkkkkkkkkkkkkjk

## FASE 07 — Vacaciones

**Contexto.** `NewVacacionesService` tiene el mismo problema de firma que asistencia; `AcumularVacaciones` es manual y no protege contra doble acumulación del mismo período; `contarDiasHabiles` (en `vacaciones_service.go`, línea ~414) falla si falta **un solo día** en `calendario_laboral_dia`; y las rutas de vacaciones **nunca se montan** porque nadie construye el servicio (eso lo resuelve F11). El diagnóstico leyó solo partes de este código.

**Requisitos:** F02.

**Prompt para la IA**
```
FASE 07 — Vacaciones.

TAREAS (en este orden)
1. NewVacacionesService(sqlDB *sql.DB, queries VacacionesQueries) (sin función inyectada, D7).
   Toda escritura por db.WithAuditContext con callback (ctx, q *dto.Queries), usuarioID int64.
2. IDs int64 en vacaciones_service.go, vacaciones_handler.go y dto/vacaciones*.go (los 2 usos
   de uint64 de vacaciones_service.go incluidos).
3. Doble acumulación: AcumularVacaciones debe ser idempotente por (empleado, período de
   acumulación): dentro de la transacción, con SELECT ... FOR UPDATE, verifica si ya existe el
   movimiento de acumulación de ese período; si existe devuelve un error de dominio tipado
   (ErrAcumulacionYaRegistrada) y no escribe. Si hace falta un índice único, crea la migración
   db/migrations/000107_vacaciones_acumulacion_unica.up.sql (+ .down.sql) sin borrar datos.
4. contarDiasHabiles: si faltan días en el calendario, NO falles con un error opaco: devuelve un
   error tipado que liste las fechas faltantes (ErrCalendarioIncompleto{Fechas []time.Time})
   para que RR. HH. sepa qué días cargar. No inventes días hábiles por defecto (D8).
5. Revisa el resto de reglas del archivo (solicitud, aprobación, disfrute, saldo): saldo negativo,
   solicitudes traslapadas, aprobar dos veces, aprobar la propia solicitud (deja un
   "TODO F17" para autorización a nivel de fila; no la implementes aquí).
6. vacaciones_handler.go: ParseIDParam, FailFromError, sin doble respuesta. NO cambies códigos
   de permiso (F12). No cambies la ruta ni el JSON público.
7. NO implementes el pago de vacaciones en planilla (F25).

CRITERIOS DE ACEPTACIÓN
- Ninguna escritura sin WithAuditContext.
- Tests table-driven para: acumulación repetida, calendario incompleto, saldo insuficiente.
- go vet ./internal/asistencia/... sin errores propios en vacaciones_*.
```

**Archivos a pasar**
- `internal/asistencia/vacaciones_service.go` (504), `vacaciones_handler.go` (230)
- `internal/db/dto/vacaciones.go` (348), `vacaciones_extra.go` (110)
- **Solo lectura:** `internal/asistencia/calendario.go`, `internal/db/audit.go`, `internal/api/middleware/context.go` (post-F02)
- DDL: `vacaciones_acumulado`, `vacaciones_disfrute` (schema.sql líneas 1036–1059), `vacaciones_solicitudes`, `vacaciones_movimientos`, `calendario_laboral_dia` (líneas 3705–3717, 3773–3829), `parametros_vacaciones_escala` (672–691).

**Migraciones que puede crear:** `000107_vacaciones_acumulacion_unica`.

--- 
awawawwawawawaWA
## FASE 08 — Liquidaciones y acumulados (tipos y SQL)

**Contexto.** `liquidacion_service.go` usa 7 tipos que no existen en `dto` (`SumarSalariosBrutosPeriodoParams`, `ListSalariosBrutosUltimosPeriodosParams`, `ListSalariosBrutosUltimosPeriodosRow`, `ParametrosSalarioEscolar` [el real es `ParametroSalarioEscolar`], `CerrarContratoVigenteParams`, `CerrarDatosLaboralesVigenteParams`, `SetEmpleadoActivoParams`), llama a 5 funciones con estilo de struct `Params` cuando `dto` las define posicionales (líneas 557, 605, 690, 697, 704), y su interfaz `LiquidacionRepository` exige `ListarEscalaCesantiaEnFecha` y `ListarEscalaPreavisoEnFecha`, que no existen (las tablas y datos sí). Además el SQL de `dto/acumulados_liquidaciones.go` usa columnas inexistentes (`pde.periodo_id`, `pp.fecha_fin_periodo`, `pp.fecha_inicio_periodo`) y estados inexistentes (`'calculado'`). **Esta fase arregla compilación y SQL; las reglas de negocio de la liquidación son F26.**

**Requisitos:** F02.

**Prompt para la IA**
```
FASE 08 — Liquidaciones y acumulados: tipos faltantes y SQL correcto.

TAREAS (en este orden)
1. SQL de internal/db/dto/acumulados_liquidaciones.go contra el DDL adjunto:
   - pde.periodo_id -> pde.periodo_planilla_id
   - pp.fecha_fin_periodo / pp.fecha_inicio_periodo -> pp.fecha_fin / pp.fecha_inicio
   - Estados: reemplaza 'calculado' por el conjunto de planilla.EstadosParaAcumulados()
     (D4: aprobada, pagada, contabilizada), construyendo el IN (...) con parámetros con el
     helper de internal/planilla/estados.go (F02). Nada de estados escritos como string suelto.
   Revisa las demás consultas del archivo contra el DDL y corrige lo que no calce.
2. Tipos faltantes: NO los definas dentro de acumulados_liquidaciones.go; créalos en un archivo
   nuevo internal/db/dto/liquidacion_cierre.go con: SumarSalariosBrutosPeriodoParams,
   ListSalariosBrutosUltimosPeriodosParams, ListSalariosBrutosUltimosPeriodosRow,
   CerrarContratoVigenteParams, CerrarDatosLaboralesVigenteParams, SetEmpleadoActivoParams (con
   sus métodos en *Queries: CerrarContratoVigente, CerrarDatosLaboralesVigente,
   SetEmpleadoActivo) y agrega también ListarEscalaCesantiaEnFecha y
   ListarEscalaPreavisoEnFecha (leen parametros_cesantia_escala y parametros_preaviso_escala
   vigentes a la fecha, filtrando por regimen_laboral_id). Respeta D1 (int64) y D2.
   Corrige el nombre ParametrosSalarioEscolar -> ParametroSalarioEscolar en el servicio.
3. services/liquidacion_service.go: ajusta las llamadas (líneas ~557, 605, 690, 697, 704) al
   estilo real de dto (posicional hasta 3 args, struct Params desde 4) y a int64. Verifica que
   *dto.Queries satisface LiquidacionRepository método por método (nombres, argumentos, tipos,
   retornos) y lista los desajustes.
4. handlers/liquidacion_handler.go: ELIMINA el método RegisterRoutes (registra rutas SIN
   permisos; routes.go ya monta las rutas con permiso; dejarlo es una trampa). Usa
   ParseIDParam, FailFromError y middleware.UsuarioID (int64).
5. NO cambies las reglas de cálculo de la liquidación (antigüedad, promedios, cesantía): eso es
   F26. Solo compila y alinea con el esquema.
6. El servicio procesa la liquidación con db.WithAuditContext (ya lo hace en una línea): revisa
   que TODA escritura del proceso (contrato, datos laborales, activo, liquidación) ocurra en
   UNA sola transacción.

CRITERIOS DE ACEPTACIÓN
- Ninguna referencia a pde.periodo_id, fecha_*_periodo ni a 'calculado' queda en el paquete dto.
- go vet ./internal/services/... ./internal/db/dto/... sin errores propios de liquidación.
```

**Archivos a pasar**
- `internal/services/liquidacion_service.go` (717)
- `internal/api/handlers/liquidacion_handler.go` (153)
- `internal/db/dto/acumulados_liquidaciones.go` (475)
- **Solo lectura:** `internal/planilla/estados.go` (F02), `internal/db/dto/contratos.go` y `empleados.go` (post-F02, para ver las funciones existentes), `internal/db/audit.go`, `middleware/context.go`
- DDL: `periodos_planilla`, `planilla_detalle_empleado` (schema.sql 944–996), `parametros_cesantia_escala`, `parametros_preaviso_escala` (692–739), `parametros_salario_escolar`, `aguinaldos`, `salarios_escolares`, `liquidaciones` (3873–3968), `contratos` (481–512).

**Archivos que crea:** `internal/db/dto/liquidacion_cierre.go`.

---
va la de abajoooooo
## FASE 09 — Paquete `planilla` y exportación

**Contexto.** `internal/planilla/service.go` y `export_service.go` declaran **las mismas 16 identidades** (no compila), `periodo.go` tiene 0 bytes (sin `package`: error de sintaxis), y las consultas de exportación (`dto/planilla_export.go`) usan `pde.periodo_id`, `pdc.orden_calculo` (no existe en `planilla_detalle_concepto`), columnas inexistentes en `planilla_detalle_embargo` y estados `('calculado','cerrado')` que no existen, así que la dispersión bancaria nunca devolvería filas. La ruta de exportación no se monta y no deja registro (la tabla `exportaciones_generadas` existe sin uso).

**Requisitos:** F02.

**Prompt para la IA**
```
FASE 09 — Paquete planilla (compilación) y exportación bancaria/boleta.

TAREAS (en este orden)
1. Duplicados: conserva export_service.go y ELIMINA internal/planilla/service.go (es una copia;
   el orquestador real se escribirá en F20 con un archivo nuevo). Crea periodo.go con solo
   "package planilla" y un comentario de paquete: "El ciclo de vida del período se implementa
   en la fase F19".
2. Comprueba con lectura que no queden identificadores duplicados en el paquete planilla
   (lista los archivos que te paso).
3. SQL de internal/db/dto/planilla_export.go contra el DDL adjunto:
   - pde.periodo_id -> pde.periodo_planilla_id.
   - pdc.orden_calculo: no existe; obtén el orden con JOIN a orden_calculo_concepto (ajusta
     el ORDER BY y el struct).
   - ObtenerEmbargosBoletaPago: planilla_detalle_embargo solo tiene orden_prioridad y monto;
     usa esas columnas y elimina monto_calculado/monto_aplicado/saldo_pendiente_insatisfecho
     (F20 agregará esas columnas; deja un comentario "F20").
   - Estados: usa planilla.EstadosParaDispersion() (aprobada|pagada) con IN (...)
     parametrizado. Para la boleta, permite aprobada|pagada|contabilizada.
   - Valida que ExportRepository (planilla) sea satisfecho por el adaptador
     planilla.NewExportRepository(*dto.Queries): nombres, firmas, tipos. Lista desajustes.
4. Trazabilidad de exportación (S14): cada llamada a GenerarArchivoDispersionBancaria debe
   registrar una fila en exportaciones_generadas (quién, cuándo, período, formato, nombre de
   archivo, hash SHA-256 del contenido, cantidad de filas y total). El registro y la
   generación ocurren en la misma transacción auditada (WithAuditContext, usuarioID int64).
   Si la tabla necesita columnas para esto, crea db/migrations/000109_exportaciones_trazabilidad
   (.up/.down) sin borrar datos. La firma del servicio recibe usuarioID int64.
5. planilla_export_handler.go: ELIMINA RegisterRoutes (registra sin permisos; routes.go monta las
   rutas con permiso). Handler con ParseIDParam, FailFromError, sin err.Error() interno.
   Encabezados de descarga seguros: Content-Disposition con nombre saneado, X-Content-Type-Options.
6. domain.go: corrige el comentario falso de estados ('abierto','calculando'...) para que
   refiera a estados.go. Elimina los tipos que sigan sin uso SOLO si no los referencia nadie
   (ResultadoPeriodoOutput y EmpleadoErrorOutput los usará F20: NO los borres).
7. Tests (table-driven) para validarFilaDispersion y sanitizarCampoDispersion con datos límite
   (IBAN vacío, montos con más de 2 decimales, caracteres de control).

CRITERIOS DE ACEPTACIÓN
- El paquete planilla no tiene declaraciones duplicadas y periodo.go tiene package.
- Ningún SQL de exportación usa 'calculado', 'cerrado' ni pde.periodo_id.
```

**Archivos a pasar**
- `internal/planilla/service.go` (364, **para eliminar**), `export_service.go` (255), `periodo.go` (vacío), `domain.go` (254)
- `internal/api/handlers/planilla_export_handler.go` (113)
- `internal/db/dto/planilla_export.go` (233)
- **Solo lectura:** `internal/planilla/estados.go` (F02), `internal/db/audit.go`
- DDL: `periodos_planilla`, `planilla_detalle_empleado`, `planilla_detalle_concepto` (schema.sql 944–1035), `planilla_detalle_embargo` (1136–1152), `orden_calculo_concepto` (546–563), `formatos_exportacion`, `exportaciones_generadas` (1288–1307).

**Migraciones que puede crear:** `000109_exportaciones_trazabilidad`.
> No hace falta pasar `motor_calculo.go`, `calculator.go` ni `acumulados_periodicos.go`: se validan en F11 con la salida del compilador.

---
abajajajaaaaa

## FASE 10 — Dashboard y contabilidad

**Contexto.** `dashboard.go` usa `pp.calculo_finalizado_en` (columna inexistente; el propio comentario lo admite) y el estado `'cerrado'`. El módulo `contabilidad` está entero sin construir en `main.go` y el diagnóstico solo leyó adaptadores y repositorio, **no** `service.go`, los tests ni el handler. Esta fase lo deja compilable y con SQL correcto; el rediseño contable (asiento de pago, cuota patronal persistida) es F28.

**Requisitos:** F02.

**Prompt para la IA**
```
FASE 10 — Dashboard y contabilidad: que compilen y su SQL calce con el esquema.

TAREAS (en este orden)
1. Dashboard (dto/dashboard.go, services/dashboard_service.go, handlers/dashboard_handler.go):
   - Reemplaza 'cerrado' por planilla.EstadosCerrados() (pagada|contabilizada, D4) con IN(...)
     parametrizado.
   - pp.calculo_finalizado_en NO existe: usa la columna real que corresponda del DDL adjunto
     (por ejemplo la fecha de aprobación/pago) o, si ninguna sirve, elimina esa métrica y
     documéntalo. No agregues columnas aquí.
   - Comprueba el resto de la consulta contra el DDL (alias, tablas, columnas).
   - IDs int64, ParseIDParam/FailFromError en el handler.
2. Contabilidad (internal/contabilidad/*.go y handlers/contabilidad_handler.go): el diagnóstico
   no lo leyó completo. Lee service.go, domain.go, filtro.go, repository.go, sql_adapters.go,
   el handler y los tests, y:
   - Corrige lo que no compile, IDs int64, y toda escritura con db.WithAuditContext.
   - Verifica los SQL de sql_adapters.go/dto/contabilidad.go contra el DDL (el diagnóstico dice
     que pde.periodo_planilla_id y pde.programa_presupuestario_id sí calzan; revisa el resto).
   - Verifica que el cálculo del asiento cuadre (debe = haber, redondeo a 2 decimales con
     decimal) y que el porcentaje patronal se trate según D3 (fracción en dominio; el adaptador
     hoy divide entre 100: déjalo documentado, F18 lo centraliza).
   - Que "generar asiento" solo funcione si el período está en estado pagada (D4) y lo deje en
     contabilizada dentro de la MISMA transacción.
   - Ejecuta/ajusta los tests existentes (service_test.go); agrega casos para asiento
     descuadrado y período en estado incorrecto.
   - NO implementes aquí: cuota patronal persistida, asiento de pago ni de liquidación (F28).
3. Ambos: NO cambies los códigos de permiso (F12).

CRITERIOS DE ACEPTACIÓN
- Ningún estado 'cerrado'/'calculado' ni columnas inexistentes.
- go vet ./internal/contabilidad/... ./internal/services/... (solo dashboard) sin errores propios.
- Los tests de contabilidad pasan o falla solo lo que documentas como pendiente.
```

**Archivos a pasar**
- `internal/services/dashboard_service.go` (186), `internal/db/dto/dashboard.go` (82), `internal/api/handlers/dashboard_handler.go` (91)
- `internal/contabilidad/`: `domain.go`, `filtro.go`, `repository.go`, `service.go` (397), `sql_adapters.go` (296), `service_test.go` (263)
- `internal/db/dto/contabilidad.go` (179), `internal/api/handlers/contabilidad_handler.go` (238)
- **Solo lectura:** `internal/planilla/estados.go` (F02), `internal/db/audit.go`
- DDL: `periodos_planilla`, `planilla_detalle_*` (944–1035), `cuentas_contables`, `tipos_asiento`, `plantillas_asiento_linea`, `mapeo_concepto_cuenta`, `asientos_contables`, `asientos_contables_detalle` (1153–1243), `programas_presupuestarios` (277–286).

---
abajooooo

## FASE 11 — Integración: `main.go`, `go mod tidy` y bucle de compilación

**Contexto.** Cada fase anterior arregló su paquete sin poder compilar el todo. Ahora se conecta todo en `cmd/api/main.go` y se corre el ciclo **compilar → corregir** hasta cero errores. Aquí es donde aparecen los errores en cascada que el diagnóstico avisó ("cuando corra `go build` van a aparecer más"). Esta fase **necesita a una persona con Go instalado y red**: tú ejecutas los comandos y pegas la salida completa a la IA; la IA corrige y repite.

**Requisitos:** F03–F10.

**Prompt para la IA**
```
FASE 11 — Integración y bucle de compilación.

ENTRADA
Te paso main.go, routes.go, go.mod, config.go y la SALIDA COMPLETA de:
go mod tidy ; go build ./... ; go vet ./... ; gofmt -l .
(más los archivos que aparezcan en los errores, cuando me los pidas).

TAREAS (en este orden)
1. Cablea en cmd/api/main.go (buildDeps) TODO lo que hoy está sin construir, usando los
   constructores reales de cada paquete (lee sus firmas en los archivos que te paso):
   - services.NewLiquidacionService(sqlDB, planilla.NewMotorRepository(queries) o el
     repositorio que corresponda, y el RegimenResolver que pida su firma de 3 parámetros).
     Si el RegimenResolver no tiene implementación, créala mínima sobre *dto.Queries.
   - asistencia.NewService(sqlDB, queries, opts) y asistencia.NewVacacionesService(sqlDB, queries)
     -> Deps.Asistencia y Deps.Vacaciones (F06/F07).
   - planilla.NewExportService(planilla.NewExportRepository(queries)) -> Deps.PlanillaExport.
   - contabilidad.NewService/NewDetallesSQL/NewLectorSQL -> Deps.Contabilidad.
   - planilla.NewMotorEmpleado y planilla.NewAcumuladosService: constrúyelos y déjalos en Deps
     con un comentario "se usan desde F20/F24" (solo si compilan sin uso muerto; si generan
     "unused", pospón su construcción).
   Todo servicio recibe *sql.DB; ninguno recibe funciones de auditoría (D7).
2. Elimina de main.go y routes.go los comentarios "VERIFICAR:", "Fase N" y las notas obsoletas.
   Mantén /healthz y /health como están.
3. routes.go: monta los handlers con los constructores reales. Quita los `if deps.X != nil`
   de PlanillaExport, Vacaciones y Contabilidad solo si ya siempre existen (si aún pueden
   faltar por configuración, deja el if).
4. go.mod: ejecuta el ciclo `go mod tidy` -> `go build ./...` -> corregir. Deja go.sum
   generado. NO cambies versiones de dependencias salvo que tidy lo haga solo (la
   sustitución de o1egl/paseto y de aead/chacha20poly1305 es F15).
5. BUCLE: por cada tanda de errores del compilador, corrige y pide la siguiente salida. Agrupa
   errores por paquete. Prioriza errores de firma/tipo sobre los de "declared and not used".
   No "arregles" un error borrando funcionalidad: si algo no calza, dilo.
6. Cuando `go build ./...` y `go vet ./...` den cero errores, corre `go test ./... -count=1`
   y reporta cada test que falle con causa probable (no ocultes fallos: si un test falla
   porque la expectativa es vieja, explica por qué antes de cambiarla).
7. gofmt -l . debe quedar vacío.

CRITERIOS DE ACEPTACIÓN
- go build ./..., go vet ./... y gofmt -l . sin salida de error.
- go test ./... ejecuta; los tests que fallen quedan listados con causa.
- Todos los módulos del diagnóstico (3.1) tienen su constructor invocado en main.go.
```

**Archivos a pasar**
- `cmd/api/main.go` (~190), `internal/api/handlers/routes.go` (159), `go.mod`, `internal/config/config.go`
- **La salida completa** de: `go mod tidy`, `go build ./...`, `go vet ./...`, `gofmt -l .`
- En las rondas siguientes, **solo los archivos que nombre el compilador** en sus errores (con la versión actual).
- **Solo lectura, para ver constructores:** `internal/auth/service.go` (firma), `internal/asistencia/service.go` y `vacaciones_service.go` (constructores), `internal/planilla/export_service.go` (constructores), `internal/planilla/motor_calculo.go` (solo `NewMotorRepository`, `NewMotorEmpleado`), `internal/services/liquidacion_service.go` (constructor y `RegimenResolver`), `internal/contabilidad/service.go` y `sql_adapters.go` (constructores).

**Al terminar esta fase, el backend compila.** Guarda el commit: es el punto de retorno.

---

# ETAPA 2 — QUE CORRA

## FASE 12 — Permisos, roles, admin inicial y test de coherencia

**Contexto.** Las rutas exigen 27 códigos de permiso que **no existen** en la BD, y el rol `ADMINISTRADOR` solo recibe 10 permisos "gestionar" que ninguna ruta usa: hoy **todo devuelve 403**, ni el admin puede operar. Además, la migración `000006` trae un usuario `admin` con contraseña conocida (`CambiarEsta123!`) y su hash versionado en el repositorio (S1/S18), y los seeds usan `ON DUPLICATE KEY UPDATE`, que pisa valores. Esta fase alinea permisos, define los roles reales con **segregación de funciones** (preparar ≠ aprobar ≠ pagar) y crea el admin **sin contraseña en el repositorio**.

**Requisitos:** F01 y F11.

**Prompt para la IA**
```
FASE 12 — Permisos, roles y admin inicial.

TAREAS (en este orden)
1. Crea internal/authz/permisos.go con UNA CONSTANTE por código de permiso (const
   PermEmpleadosCrear = "empleados.crear", etc.) y una función Todos() []Permiso con
   {Codigo, Nombre, Modulo}. Incluye los 27 que hoy usan las rutas y no existen:
   contratos.{crear,leer,actualizar} empleados.{crear,leer,actualizar}
   organizacion.{crear,leer,actualizar} prestamos.{crear,ver,abonar,cancelar}
   embargos.{crear,ver,cambiar_estado} terceros.{crear,ver}
   liquidaciones.{proyectar,procesar} planilla.{aprobar,exportar_bancario,ver_boleta}
   dashboard.ver usuarios.{asignar_rol,gestionar_sesiones}
   más los que ya están sembrados y sí usan las rutas: asistencia.{crear,ver}
   calendario.administrar incapacidades.gestionar vacaciones.{solicitar,aprobar,
   registrar_disfrute,ver} asientos_contables.{generar,ver}
   y estos RESERVADOS para fases futuras (definirlos ya evita choques entre IA):
   planilla.{crear,calcular,ver,pagar,ver_boleta_propia} aguinaldo.ejecutar
   salario_escolar.ejecutar parametros.{ver,proponer,aprobar}
   usuarios.{crear,ver,actualizar,reset_password} auditoria.ver
   reportes.{ccss,ins,hacienda} vacaciones.{solicitar_terceros,ver_terceros}
   datos_personales.ver_sensible.
2. Reemplaza en TODOS los archivos de rutas que te paso cada literal de RequirePermission("...")
   por la constante de authz. Elimina el patrón literal "contratos.*" del comentario de
   routes.go. No cambies rutas ni handlers.
3. middleware.RequirePermission: registra cada código exigido en un registro interno
   (func PermisosExigidos() []string) al construirse el middleware, para poder testearlo.
4. Migración db/migrations/000112_permisos_y_roles.up.sql (+ .down.sql):
   - INSERT de TODOS los permisos de Todos() con INSERT ... SELECT ... WHERE NOT EXISTS (no
     ON DUPLICATE KEY UPDATE ni INSERT IGNORE que oculte fallos).
   - Roles (por codigo, NUNCA ids fijos): ADMINISTRADOR_SISTEMA (renombra el actual
     ADMINISTRADOR), RRHH, PLANILLAS_PREPARA, PLANILLAS_APRUEBA, TESORERIA, CONTABILIDAD,
     AUDITOR.
   - Asignación inicial propuesta (ajustable después, pero con SEGREGACIÓN):
     ADMINISTRADOR_SISTEMA: usuarios.*, auditoria.ver, dashboard.ver, parametros.ver
       (NUNCA planilla.aprobar/pagar/calcular).
     RRHH: empleados.*, contratos.*, organizacion.*, asistencia.*, calendario.administrar,
       incapacidades.gestionar, vacaciones.* (incluye *_terceros), prestamos.*, embargos.*,
       terceros.*, liquidaciones.proyectar, dashboard.ver.
     PLANILLAS_PREPARA: planilla.crear/calcular/ver/ver_boleta, aguinaldo.ejecutar,
       salario_escolar.ejecutar, liquidaciones.proyectar/procesar, parametros.proponer,
       reportes.*.
     PLANILLAS_APRUEBA: planilla.ver/aprobar/ver_boleta, parametros.aprobar, dashboard.ver.
     TESORERIA: planilla.ver, planilla.exportar_bancario, planilla.pagar.
     CONTABILIDAD: asientos_contables.*, planilla.ver, reportes.*.
     AUDITOR: todos los permisos de lectura (*.ver, *.leer, auditoria.ver, dashboard.ver).
   - Elimina de rol_permisos y luego de permisos los 10 códigos "gestionar/ejecutar" obsoletos
     (empleados.gestionar, planilla.ejecutar, usuarios_roles.administrar, ...) SOLO si ninguna
     ruta ni constante los usa; si el borrado es riesgoso, deja una nota y no los borres.
5. Edita db/migrations/000006_rol_admin_y_permisos_base.up.sql: elimina el INSERT del usuario
   admin (con su hash y el comentario de la contraseña) y su asignación de rol, y reemplaza
   `ON DUPLICATE KEY UPDATE` por inserciones que no pisen datos. Nada de credenciales en el
   repositorio.
6. Crea cmd/seedadmin/main.go: crea el usuario inicial (nombre de usuario y correo por flags o
   variables de entorno), genera una contraseña ALEATORIA (crypto/rand, >= 20 caracteres),
   la hashea con internal/security (argon2id), inserta el usuario con
   requiere_cambio_password=1 y rol ADMINISTRADOR_SISTEMA, IMPRIME la contraseña UNA sola vez
   por stdout y no la escribe en ningún archivo ni log. Debe ser idempotente: si ya existe un
   administrador, se niega. Usa @auditoria_usuario_id = NULL explícito (es un alta inicial sin
   autor previo) en su propia transacción.
7. TESTS (paquete de tests en internal/authz o en handlers):
   a) Construye el router real con Deps vacías y verifica que middleware.PermisosExigidos() ⊆
      authz.Todos().
   b) Parsea db/migrations/*.up.sql con expresión regular y verifica que cada código de
      authz.Todos() esté sembrado y que ninguno sembrado quede fuera de authz.
   c) Para cada ruta de /api/v1 que NO esté en una lista blanca explícita (login, refresh,
      logout, reset de contraseña, cambio de contraseña, /me), lanza una petición con un token
      válido SIN permisos y espera 403. Si una ruta protegida responde algo distinto, el test
      falla (detecta rutas sin RequirePermission).

CRITERIOS DE ACEPTACIÓN
- No queda ningún literal de permiso fuera de internal/authz.
- Ningún archivo del repositorio contiene una contraseña ni un hash de contraseña.
- Los tres tests pasan.
```

**Archivos a pasar**
- `internal/api/handlers/routes.go`, `auth_handler.go`, `contrato_handler.go`, `dashboard_handler.go`, `empleado_handler.go`, `organizacion_handler.go`
- `internal/asistencia/handler.go`, `vacaciones_handler.go`, `internal/prestamos/routes.go`
- `internal/api/middleware/require_permission.go` (83), `require_permission_test.go` (122), `auth.go`
- `db/migrations/000006_rol_admin_y_permisos_base.up.sql` (141 líneas) y `000001_seguridad_reforzada.up.sql` (solo el DDL de `usuarios`, `roles`, `usuario_roles`)
- `internal/security/hash.go`, `internal/config/config.go`, `internal/db/audit.go`
- DDL de `usuarios`, `roles`, `permisos`, `rol_permisos`, `usuario_roles` (schema.sql 1312–1351 y 3110–3125).

**Archivos que crea:** `internal/authz/permisos.go`, `cmd/seedadmin/main.go`, tests, `db/migrations/000112_permisos_y_roles.{up,down}.sql`.

---

## FASE 13 — Pruebas de integración con MySQL y test de contrato SQL

**Contexto.** El diagnóstico lo dice claro: los errores de la sección 4 (26 consultas con columnas o estados inexistentes) **los habría atrapado el primer test contra MySQL real**. Esta fase crea esa red de seguridad: un test que toma **cada** consulta SQL embebida en Go y la prepara contra el esquema real (es el reemplazo de sqlc para la opción B), un test que compara los estados en Go con el ENUM de la BD, y un recorrido de humo de la API.

**Requisitos:** F11 y F01. **Paralelo con F12** (si F12 termina antes, incluye su test en la suite).

**Prompt para la IA**
```
FASE 13 — Integración con MySQL real y test de contrato SQL.

REGLAS DEL ENTORNO DE PRUEBA
Los tests de integración llevan el build tag `integration` y usan la variable de entorno
TEST_DATABASE_URL (una MySQL 8 disponible, p. ej. la de docker-compose.yml). Si no está
definida, se SALTAN con t.Skip (nunca fallan por falta de BD). Cada ejecución crea una base
temporal con nombre aleatorio, aplica las migraciones de db/migrations con golang-migrate
(en código, usando multiStatements=true) y la elimina al terminar.

TAREAS (en este orden)
1. internal/testutil/db.go: helper NuevaBD(t) *sql.DB con lo descrito arriba.
2. internal/db/contrato_sql_test.go — TEST DE CONTRATO: con go/ast recorre TODOS los .go no de
   test bajo internal/ y extrae cada literal de cadena cuyo primer token (tras trim) sea
   SELECT, INSERT, UPDATE, DELETE o WITH. Para cada una hace db.PrepareContext contra la BD de
   prueba: un error de columna/tabla inexistente falla el test con archivo, línea y mensaje.
   Las consultas construidas dinámicamente (fmt/concatenación, %s, IN (...) armados) no se
   pueden preparar tal cual: el test las lista en un t.Log aparte y exige que estén en una
   lista blanca explícita con comentario; para cada una añade un test específico que ejecute
   sus variantes reales (p. ej. los estados de estados.go).
3. internal/planilla/estados_enum_test.go: lee information_schema el ENUM real de
   periodos_planilla.estado y compara, sin importar el orden, con el conjunto de constantes de
   estados.go. Igual con los ENUM de estado de prestamos y embargos contra sus constantes (si
   las constantes no existen, créalas en el paquete que corresponda y úsalas en el servicio).
4. Extrae el armado del router de main.go a internal/app/app.go: func New(cfg *config.Config,
   sqlDB *sql.DB, tokenMaker security.TokenMaker) (*gin.Engine, error). main.go solo carga
   configuración, conecta, llama a app.New y arranca el servidor.
5. Prueba de humo (integration): con el admin creado por el mismo código de cmd/seedadmin (o su
   función interna) y requiere_cambio_password = 0 actualizado directo en la BD de prueba:
   login -> GET protegido con y sin token -> crear una unidad organizativa/programa ->
   crear un empleado con los datos mínimos que exijan los handlers (léelos) -> listarlo.
   Deja un test con t.Skip("F19") para crear período de planilla.
6. Actualiza run_full_test_suite.sh y docker-compose.yml: paso opcional "integración" que
   levante MySQL y exporte TEST_DATABASE_URL; documenta en docs/pruebas.md cómo correrlo.
7. Elimina db/legacy/schema.sql.OLD (ya no se necesita) y la referencia en docs/migraciones.md,
   SOLO si el test de contrato pasa con las migraciones.

CRITERIOS DE ACEPTACIÓN
- `go test ./...` (sin TEST_DATABASE_URL) pasa saltando lo de integración.
- `go test -tags=integration ./...` con MySQL: el test de contrato falla listando CADA
  consulta con problema (en este momento se esperan aún fallas reales: repórtalas todas sin
  ocultarlas; corrige solo las obvias y de una línea, y lista el resto para quien corresponda).
```

**Archivos a pasar**
- `cmd/api/main.go`, `internal/api/handlers/routes.go`, `internal/config/config.go` (post-F11)
- `internal/db/dto/db.go` y **una muestra** del estilo de consulta: `internal/db/dto/planilla_export.go`
- `internal/planilla/estados.go`, `internal/prestamos/service.go` (solo constantes/estados)
- `docker-compose.yml`, `Makefile`, `cmd/migrate/main.go`, `run_full_test_suite.sh` (F00/F01)
- **Nombres** de los archivos de `db/migrations/` (no hace falta el contenido)
- `internal/security/hash.go`, `cmd/seedadmin/main.go` (si F12 ya terminó)

**Archivos que crea:** `internal/testutil/db.go`, `internal/db/contrato_sql_test.go`, `internal/planilla/estados_enum_test.go`, `internal/app/app.go`, `docs/pruebas.md`.
> Después de esta fase, si el test de contrato lista consultas rotas que ninguna fase asumió, ábrelas como **hallazgos** y arréglalas antes de seguir (pásale cada una a una IA con su archivo y el DDL de sus tablas).

---

# ETAPA 3 — SEGURIDAD

> **Nada de esta etapa va a producción sin cambiar la contraseña del admin, arreglar la auditoría y agregar rate limiting** (F02, F12, F14, F16).

## FASE 14 — Autenticación reforzada (S1, S2, S5, S6, S11)

**Contexto.** Hoy: la contraseña temporal del admin no se puede cambiar (no hay endpoint autenticado ni middleware que obligue a cambiarla); el reset de contraseña genera el token y **lo descarta**; el login distingue "inactivo/bloqueado" de "credenciales inválidas" (enumeración de usuarios) y responde sin calcular hash cuando el usuario no existe (enumeración por tiempo); el contador de intentos fallidos se calcula con un valor leído antes (no atómico); la rotación de refresh tokens es leer→validar→revocar sin transacción y sin detectar reutilización; la política de contraseñas es solo `min=8`.

**Requisitos:** F12 y F13.

**Prompt para la IA**
```
FASE 14 — Autenticación reforzada.

TAREAS (en este orden)
1. CAMBIO DE CONTRASEÑA: POST /auth/password/cambiar (autenticado). Recibe contraseña actual y
   nueva; valida la actual; aplica la política (tarea 6); guarda hash argon2id; limpia
   requiere_cambio_password; actualiza fecha_ultimo_cambio_password; REVOCA todas las demás
   sesiones (refresh tokens) del usuario; todo en una transacción con WithAuditContext.
2. OBLIGAR EL CAMBIO: agrega al token un claim "rcp" (requiere cambio de contraseña) al emitirlo
   en login/refresh. Crea middleware.ExigirCambioPassword: mientras rcp=true, TODA ruta
   protegida devuelve 403 con código "cambio_password_requerido", salvo: cambiar contraseña,
   logout y /auth/me. Móntalo en el grupo `protegido` de routes.go.
3. RESET POR CORREO (S2): define la interfaz auth.Notificador
   {EnviarResetPassword(ctx, correo, nombre, token string, expira time.Time) error} con dos
   implementaciones: SMTP (net/smtp con TLS, configurada por variables SMTP_HOST, SMTP_PORT,
   SMTP_USER, SMTP_PASS, SMTP_FROM, URL_BASE_RESET en config y .env.example) y LogNotificador
   que SOLO se puede activar con APP_ENV=development (registra el token en el log de
   desarrollo). SolicitarResetPassword debe usar el notificador y SIEMPRE responder 202 igual
   exista o no el correo (sin revelar existencia). El handler no descarta el token.
   Completa el flujo: POST /auth/password/reset/confirmar (token + nueva contraseña, token de
   un solo uso, con expiración, guardado solo como hash; revoca sesiones).
4. LOGIN (S5):
   a) Respuesta única 401 "credenciales inválidas" para: usuario inexistente, contraseña mala,
      usuario inactivo y usuario bloqueado (el motivo real solo se registra en el log interno).
   b) Cuando el usuario NO existe, calcula igual un hash argon2id contra un hash "señuelo"
      precalculado con los mismos parámetros para igualar el tiempo.
   c) Contador de intentos ATÓMICO en SQL: UPDATE usuarios SET intentos_fallidos =
      intentos_fallidos + 1, bloqueado_hasta = IF(intentos_fallidos + 1 >= ?, NOW() + INTERVAL ?
      MINUTE, bloqueado_hasta) WHERE id = ? y luego lectura del estado; reinicio a 0 en éxito.
      Ajusta los nombres a las columnas reales del DDL. Agrega la consulta en dto/security.go.
   d) Registra ip_origen y user_agent de los intentos fallidos si el esquema lo permite.
5. REFRESH TOKENS (S6):
   a) Rotación atómica dentro de UNA transacción: UPDATE refresh_tokens SET revocado = 1 WHERE
      id = ? AND revocado = 0 y verifica RowsAffected == 1; si es 0, es un intento de
      reutilización.
   b) Detección de reutilización: un token ya revocado que reaparece revoca TODA la familia de
      sesiones de esa cadena (agrega familia_id al refresh token) y registra el evento; responde
      401 genérico.
   c) Verifica que fila.UsuarioID == payload.UsuarioID y que el usuario siga activo.
   d) Migración db/migrations/000114_refresh_familia_y_login.{up,down}.sql con lo que haga
      falta (familia_id, ip/user_agent, etc.), sin borrar datos ni bloquear tablas largas.
6. POLÍTICA DE CONTRASEÑAS (S11): paquete internal/auth/politica.go: longitud mínima 12,
   máxima 128, no puede contener el nombre de usuario ni el correo (parte local), no puede estar
   en una lista de contraseñas comunes (archivo embebido con go:embed de al menos 300 entradas
   comunes en español e inglés). Errores tipados con mensajes útiles. Úsala en cambio y reset.
7. Tests: política de contraseñas (table-driven), atomicidad del contador (con dos goroutines
   sobre BD de prueba si hay TEST_DATABASE_URL), reutilización de refresh token y respuestas
   uniformes de login.

CRITERIOS DE ACEPTACIÓN
- Un admin recién sembrado NO puede usar ninguna ruta hasta cambiar su contraseña.
- Login inexistente/inactivo/bloqueado/mala contraseña son indistinguibles en cuerpo y código.
- El reset de contraseña funciona de punta a punta con LogNotificador en desarrollo.
```

**Archivos a pasar**
- `internal/auth/service.go` (390), `dto.go`, `errors.go`, `internal/api/handlers/auth_handler.go` (179)
- `internal/security/hash.go`, `paseto.go`, `internal/api/middleware/auth.go`, `context.go`
- `internal/db/dto/security.go` (419), `internal/config/config.go`, `.env.example`, `internal/api/handlers/routes.go`
- DDL de `usuarios`, `tokens_reset_password`, `refresh_tokens`, `usuario_roles` (schema.sql 1312–1322, 3044–3125)

**Migraciones que crea:** `000114_refresh_familia_y_login`.

---

## FASE 15 — Tokens PASETO (S7, S17)

**Contexto.** Los permisos viajan dentro del access token (20 min): desactivar a un usuario o quitarle un rol **no surte efecto hasta que expire**. Además `PASETO_ISSUER`/`PASETO_AUDIENCE` se leen y no se usan (los tokens no llevan `iss`/`aud`/`nbf`), no hay rotación de claves (`kid`), la librería `o1egl/paseto` lleva años sin mantenimiento (su versión `v1.1.1` **[VERIFICAR]** que exista) y `aead/chacha20poly1305` se importa solo por la constante `KeySize`.

**Requisitos:** F14.

**Prompt para la IA**
```
FASE 15 — Tokens PASETO: revocación efectiva, claims y rotación de claves.

TAREAS (en este orden)
1. Reemplaza github.com/o1egl/paseto por aidanwoods.dev/go-paseto (mantenida, con reglas de
   validación integradas: iss/aud/nbf/exp) y elimina github.com/aead/chacha20poly1305 (usa la
   constante 32 o golang.org/x/crypto/chacha20poly1305.KeySize). Mantén la interfaz
   security.TokenMaker; ajusta go.mod (indica cuáles comandos debo correr: go mod tidy).
2. Los tokens deben llevar y VALIDAR: iss (PASETO_ISSUER), aud (PASETO_AUDIENCE), iat, nbf, exp,
   jti, el claim "rcp" (de F14), "pv" (versión de permisos) y un "kid" en el footer. Si iss o
   aud no coinciden, el token se rechaza. Ya no se aceptan tokens sin esos claims.
3. ROTACIÓN DE CLAVES: la configuración acepta PASETO_LOCAL_KEYS_HEX="kid1:hex,kid2:hex" y
   PASETO_ACTIVE_KID. Se firma solo con la activa y se valida con cualquiera de la lista.
   Mantén compatibilidad temporal con PASETO_LOCAL_KEY_HEX (una sola clave, kid="k1"). Valida
   largo de claves en config.Validate() y falla en el arranque si algo está mal.
4. REVOCACIÓN EFECTIVA (S7): migración db/migrations/000115_permisos_version.{up,down}.sql que
   agrega usuarios.permisos_version INT UNSIGNED NOT NULL DEFAULT 1. El servicio de auth la
   incrementa (en la misma transacción) cada vez que se asigna/revoca un rol, se desactiva/
   activa un usuario o se cambia su contraseña. Emite "pv" en el token.
5. Middleware AuthPASETO recibe un SesionVerificador (interfaz) que devuelve
   (activo bool, permisosVersion uint32, err) por usuario, con caché en memoria de TTL
   configurable (por defecto 5 s, variable AUTH_CACHE_TTL) y singleflight para no saturar la BD.
   Si el usuario está inactivo o pv difiere: 401 con código "sesion_invalida"; el cliente debe
   refrescar el token. Los permisos vigentes del usuario se leen de BD al refrescar, no de un
   token viejo.
6. Tests: token sin aud, con aud/iss distintos, expirado, con nbf futuro, con kid desconocido,
   rotación (token firmado con kid viejo sigue válido hasta retirarlo), usuario desactivado
   (con verificador falso) y cambio de rol (pv distinto).
7. Actualiza .env.example y docs/ADR-001 (sección "Tokens").

CRITERIOS DE ACEPTACIÓN
- Un usuario desactivado o con rol revocado pierde el acceso en <= AUTH_CACHE_TTL segundos.
- No queda import de o1egl/paseto ni de aead/chacha20poly1305.
```

**Archivos a pasar**
- `internal/security/paseto.go`, `hash.go`, `internal/api/middleware/auth.go`, `context.go`
- `internal/auth/service.go`, `dto.go`, `internal/api/handlers/auth_handler.go`
- `internal/db/dto/security.go`, `internal/config/config.go`, `.env.example`, `go.mod`
- `cmd/api/main.go` o `internal/app/app.go` (cableado del verificador), `internal/api/handlers/routes.go`
- **Informe de traspaso de F14** (nombres de claims y consultas nuevas).

**Migraciones que crea:** `000115_permisos_version`.

---

## FASE 16 — Endurecimiento HTTP, errores y logging (S8, S9, S15, S16, C3)

**Contexto.** No hay rate limiting, CORS, cabeceras de seguridad, límite de tamaño de cuerpo ni `SetTrustedProxies`; `gin.Logger()` registra la URL completa; el DSN de MySQL no fuerza TLS; `isBindingError` es un stub que siempre devuelve `false` (todo JSON inválido responde 500); `ParseUintParam` escribe la respuesta 400 y luego se intenta escribir otra; y `respond.go` devuelve `err.Error()` al cliente.

**Requisitos:** F11. **Paralelo con F14.**

**Prompt para la IA**
```
FASE 16 — Endurecimiento HTTP, manejo de errores y logging estructurado.

TAREAS (en este orden)
1. respond.go (C3): implementa isBindingError con errors.As sobre validator.ValidationErrors,
   *json.UnmarshalTypeError, *json.SyntaxError e io.EOF/io.ErrUnexpectedEOF. Respuesta 400 con
   {codigo:"datos_invalidos", campos:[{campo, mensaje}]} en español. Elimina el stub y el "_ ="
   de syntaxErr. Los errores de dominio salen con su código y mensaje seguro; los internos como
   500 genérico con request_id (S15); ErrSalarioInferiorAlMinimo devuelve un mensaje fijo, no
   err.Error().
2. Elimina middleware.ParseUintParam (migró todo a ParseIDParam en F03–F10; corrige cualquier
   uso restante) y confirma que ningún handler escribe dos respuestas.
3. RATE LIMITING (S8): middleware con golang.org/x/time/rate, bucket por IP para
   /auth/login, /auth/refresh y reset de contraseña (límites por variable de entorno, sin valor
   por defecto inseguro) y un límite general más laxo por usuario autenticado. 429 con
   Retry-After. Documenta que es en memoria (una réplica) y qué cambiar para varias réplicas.
4. CABECERAS Y CORS: middleware de cabeceras (X-Content-Type-Options: nosniff,
   X-Frame-Options: DENY, Referrer-Policy: no-referrer, Strict-Transport-Security solo en
   producción, Cache-Control: no-store en respuestas de API). CORS con lista blanca de orígenes
   desde CORS_ALLOWED_ORIGINS; jamás "*" junto con credenciales.
5. CUERPO Y PROXIES: http.MaxBytesReader con límite configurable (MAX_BODY_BYTES);
   router.SetTrustedProxies con lista de TRUSTED_PROXIES (vacía = ninguno); IP real del cliente
   coherente para el rate limiter y los logs.
6. LOGGING (S16): reemplaza gin.Logger y todo log.Printf por log/slog en JSON, con request_id
   (X-Request-ID recibido o generado, devuelto en la respuesta y propagado en el contexto),
   método, ruta (SIN query string ni parámetros sensibles), estado, latencia, usuario_id
   cuando exista. Recovery también con slog. NUNCA registrar cuerpos, tokens ni contraseñas.
7. TLS A LA BD (S9): DatabaseConfig agrega DB_TLS_MODE (disabled|required|verify-full) y
   DB_TLS_CA_FILE; en APP_ENV=production, "disabled" hace fallar el arranque. Registra la
   config TLS con mysql.RegisterTLSConfig y arma el DSN con tls=<nombre>.
8. Actualiza .env.example y docs/ADR-001. Tests: binding inválido -> 400, límite de cuerpo -> 413,
   rate limit -> 429, CORS origen no permitido, cabeceras presentes.

CRITERIOS DE ACEPTACIÓN
- Un JSON malformado responde 400 (no 500). Ningún err.Error() interno llega al cliente.
- Ningún log contiene contraseñas, tokens ni query strings.
```

**Archivos a pasar**
- `internal/api/middleware/respond.go` (112), `errors.go` (114), `context.go` (79), `auth.go`
- `internal/config/config.go` (240), `.env.example`, `go.mod`
- `cmd/api/main.go` o `internal/app/app.go` y `internal/api/handlers/routes.go`
- Lista de archivos que aún usan `ParseUintParam` (`grep -rn ParseUintParam internal`) y esos archivos si aparecen.

---

## FASE 17 — Usuarios/roles (API) y autorización a nivel de fila (S10)

**Contexto.** Solo existe asignar/revocar un rol ya existente: no se pueden crear, listar ni desactivar usuarios, ni resetear contraseñas administrativamente, ni consultar la auditoría (`auditoria_cambios`). Además, `vacaciones.solicitar` recibe `empleado_id` en el body (cualquiera solicita por cualquiera) y `planilla.ver_boleta` permite ver la boleta de **cualquier** empleado.

**Requisitos:** F14 y F16. **Paralelo con F15.**

**Prompt para la IA**
```
FASE 17 — Gestión de usuarios/roles y autorización a nivel de fila.

TAREAS (en este orden)
1. API de usuarios (permisos de F12: usuarios.crear/ver/actualizar/reset_password/asignar_rol/
   gestionar_sesiones):
   POST /usuarios (nombre de usuario, nombre completo, correo, empleado_id opcional; genera
     contraseña temporal aleatoria mostrada UNA vez en la respuesta, requiere_cambio_password=1),
   GET /usuarios (paginada con internal/pagination, filtros por texto/activo/rol),
   GET /usuarios/:id, PATCH /usuarios/:id (nombre, correo, empleado_id),
   POST /usuarios/:id/activar y /desactivar (desactivar revoca todas las sesiones y sube
     permisos_version si F15 ya existe; si no, deja "TODO F15"),
   POST /usuarios/:id/password/reset (temporal aleatoria mostrada una vez, revoca sesiones),
   GET /roles y GET /roles/:id/permisos (solo lectura).
   Toda escritura con WithAuditContext. Nadie puede desactivarse a sí mismo ni quitarse el último
   rol de administración.
2. Consulta de auditoría: GET /auditoria (permiso auditoria.ver): filtros por tabla, registro_id,
   usuario, rango de fechas; paginada; devuelve JSON anterior/nuevo tal cual (las contraseñas ya
   vienen redactadas por los triggers; verifica que passwords, hashes y tokens salgan
   siempre redactados).
3. Autorización a nivel de fila (S10):
   a) Migración db/migrations/000117_rol_empleado.{up,down}.sql: rol EMPLEADO con
      planilla.ver_boleta_propia y vacaciones.solicitar. (Los permisos ya existen desde F12.)
   b) middleware.EmpleadoIDDelUsuario(ctx) resuelve usuarios.empleado_id del usuario
      autenticado (error explícito si no tiene empleado vinculado).
   c) POST de solicitud de vacaciones: si el usuario NO tiene vacaciones.solicitar_terceros, el
      empleado_id se toma del usuario y se IGNORA/RECHAZA (403) el del body si difiere.
      Aprobar/rechazar: el aprobador no puede ser el solicitante.
   d) Boleta de pago: con planilla.ver_boleta se ve cualquiera; con planilla.ver_boleta_propia
      solo la del empleado vinculado (403 en otro caso). Ambas rutas usan el mismo handler con
      la regla en el servicio, no en el handler.
   e) Datos sensibles: cédula/IBAN/salario ya salen enmascarados para quien no tenga
      datos_personales.ver_sensible (deja la función de enmascarado y su uso; el cifrado en
      reposo es F30).
4. Tests: crear usuario, doble desactivación, auto-desactivación (rechazada), reset admin,
   empleado A intentando ver boleta de B (403), solicitud de vacaciones por tercero sin permiso.

CRITERIOS DE ACEPTACIÓN
- Ningún endpoint devuelve contraseñas o hashes; la temporal solo aparece en la respuesta de
  creación/reset.
- Un usuario con rol EMPLEADO no puede leer ni solicitar nada de otro empleado.
```

**Archivos a pasar**
- `internal/auth/service.go`, `dto.go`, `errors.go`, `internal/api/handlers/auth_handler.go`, `internal/api/handlers/routes.go`
- `internal/db/dto/security.go`, `internal/authz/permisos.go` (F12), `internal/auth/politica.go` (F14)
- `internal/asistencia/vacaciones_service.go`, `vacaciones_handler.go`
- `internal/planilla/export_service.go`, `internal/api/handlers/planilla_export_handler.go`
- `internal/api/middleware/context.go`, `internal/pagination/pagination.go`
- DDL: `usuarios`, `roles`, `permisos`, `rol_permisos`, `usuario_roles`, `auditoria_cambios` (schema.sql 1312–1366, 3110–3125).
- Informes de traspaso de F12, F14 y (si existe) F15.

**Migraciones que crea:** `000117_rol_empleado`.

---

# ETAPA 4 — MOTOR DE PLANILLA

> Aquí se construye lo que hoy **no existe**: el motor de cálculo no tiene ningún llamador, no hay forma de crear/calcular/aprobar/pagar un período y varias reglas legales están vacías o incompletas. Las fases van en cadena (F18 → F19 → F20 → reglas). Las reglas F21–F25 pueden repartirse entre IA distintas **una vez que exista F20**, pero como tocan `motor_calculo.go`, **aplícalas una por una** (la de número menor primero) para no pisar cambios.

## FASE 18 — Parámetros: `parametros_generales`, cargador único y tipo `Porcentaje`

**Contexto.** Hoy hay **tres convenciones de porcentaje** (CCSS y renta en BD como 10.83 pero el motor espera 0.1083; embargo `DECIMAL(5,2)` en porcentaje pero se multiplica directo; salario escolar en fracción; contabilidad divide entre 100) y **no existe el cargador de parámetros**: si se conectara el motor tal cual, la CCSS y la renta saldrían **100 veces mayores**. Además faltan `parametros_generales` y la escala de embargo, y hay valores legales escritos dentro del código Go (`0.125`, `0.25`, `0.5`, múltiplo `3`, `DivisorDiasMes = 30`, ventanas del aguinaldo, meses de cesantía…).

**Requisitos:** F13. **Paralelo con F14–F17.**

**Prompt para la IA**
```
FASE 18 — Parámetros y cargador único de vigencias.

TAREAS (en este orden)
1. Paquete internal/shared/porcentaje: type Porcentaje (envuelve decimal.Decimal) con
   DesdePorcentaje(d) (10.83 -> 0.1083 internamente), DesdeFraccion(d), Fraccion() y
   Porcentaje() (para mostrar). Validación 0 <= p <= 100. Tests table-driven de conversión y
   redondeos. Regla D3: es el ÚNICO lugar donde se convierte.
2. Migración db/migrations/000118_parametros_generales.{up,down}.sql:
   a) parametros_generales(id, clave VARCHAR(80), valor VARCHAR(255), tipo ENUM('entero',
      'decimal','porcentaje','texto','fecha','booleano'), descripcion, fuente_legal (NOT NULL),
      fecha_inicio_vigencia, fecha_fin_vigencia NULL, es_vigente (columna generada como en las
      demás tablas paramétricas), creado_por, UNIQUE (clave, es_vigente)). Mismo patrón de
      vigencia de 2 pasos de parametros_cargas_sociales.
   b) parametros_embargo_escala (tramos de embargo sobre el excedente del salario embargable,
      por múltiplo del salario mínimo inembargable: desde_multiplo, hasta_multiplo NULL,
      porcentaje_maximo, vigencia). Deriva los valores actuales de internal/planilla/calculator.go
      (0.125, 0.25, 0.5, múltiplo 3) y siémbralos con fuente_legal =
      'PENDIENTE VALIDAR (Art. 172 Código de Trabajo)'.
   c) parametros_tipos_embargo: columna es_pension_alimentaria BOOLEAN (reemplaza identificar la
      pensión por PrioridadPensionAlimentaria = 1) y tope_porcentaje_liquido NULL (el 50%).
   d) Siembra en parametros_generales (con fuente_legal 'VALOR HISTÓRICO DEL CÓDIGO - PENDIENTE
      VALIDAR') TODOS los valores legales que hoy están dentro del código Go. Encuéntralos con el
      listado de grep que te paso (DivisorDiasMes=30, mesesAguinaldo=12, ventana de aguinaldo y
      pago, mes del salario escolar, meses/días de cesantía y preaviso que estén fijos, tolerancia
      de tardanza, etc.), una clave por valor (ej.: DIVISOR_DIAS_MES, AGUINALDO_MESES,
      AGUINALDO_VENTANA_INICIO_MES_DIA...). No inventes valores: solo los que ya están en el código.
   Los seeds usan INSERT ... SELECT ... WHERE NOT EXISTS, nunca ON DUPLICATE KEY UPDATE.
3. Paquete internal/parametros: servicio Generales con Entero(ctx, clave, fecha) (int64, error),
   Decimal, Porcentaje, Texto, Fecha, Booleano; si la clave no está vigente a esa fecha ->
   ErrParametroNoConfigurado{Clave, Fecha} (D8: nunca un valor por defecto). Caché en memoria
   con TTL corto (PARAMETROS_CACHE_TTL) y método Invalidar() (lo usará F27).
4. internal/planilla/cargador_parametros.go: CargadorParametros.Cargar(ctx, fechaInicio,
   fechaFin) (*ParametrosCalculo, error) que resuelve TODO "vigente a la fecha X" en un solo
   lugar: cargas sociales, tramos de renta, créditos fiscales, horas extra, subsidios de
   incapacidad, base mínima contributiva y generales, CONVIRTIENDO porcentajes con el tipo
   Porcentaje (el dominio recibe fracciones). Falla con error explícito si falta cualquier
   parámetro obligatorio o si hay tramos de renta con huecos/solapes. Ajusta ParametrosCalculo
   (motor_calculo.go) y los tipos de domain.go solo lo necesario, sin cambiar reglas de cálculo.
5. Contabilidad: sustituye la división por `cien` de sql_adapters.go por el tipo Porcentaje.
6. Test de integración (build tag integration, BD de F13): carga los seeds reales y verifica que
   CCSS obrero = 0.1083 (fracción), que los tramos de renta son fracciones (0.10, 0.15...),
   que el salario escolar sigue en fracción y que se rechazan tramos con hueco.
7. NO cambies aún los usos de las constantes hardcodeadas en los servicios (cada fase de
   reglas las sustituirá por parametros.Generales). Deja una lista en el informe de cada
   constante y la fase que la consumirá.

CRITERIOS DE ACEPTACIÓN
- Existe una sola función de conversión de porcentajes y nadie más divide/multiplica por 100.
- El cargador falla con mensaje claro si falta un parámetro.
- Migración up/down/up sin errores.
```

**Archivos a pasar**
- `internal/planilla/domain.go` (254), `motor_calculo.go` (608), `calculator.go` (340)
- `internal/db/dto/planilla_periodo.go` (501), `planilla_motor.go` (158)
- `internal/contabilidad/sql_adapters.go` (296)
- La salida de: `grep -rn "DivisorDiasMes\|mesesAguinaldo\|0\.125\|0\.25\|0\.5\|PrioridadPensionAlimentaria\|== 12\|> 6\|fechaUTC" internal --include=*.go` y los archivos `acumulados_periodicos.go` (296) y los fragmentos de `services/liquidacion_service.go` que salgan en ese listado.
- DDL: tablas `parametros_*` (schema.sql 587–795), `parametros_salario_escolar` (3873–3883), `embargos` (1088–1135), `conceptos_salariales` y `concepto_salarial_formula` (514–545).
- Solo los `INSERT` de `parametros_*` de las migraciones `000002` y `000005`.
- `internal/config/config.go`, `.env.example`.

**Archivos que crea:** `internal/shared/porcentaje/`, `internal/parametros/`, `internal/planilla/cargador_parametros.go`.
**Migraciones que crea:** `000118_parametros_generales`.

---

## FASE 19 — Ciclo de vida del período y segregación de funciones

**Contexto.** No existe ningún endpoint para crear, listar, calcular, aprobar o pagar un período (`periodo.go` es un placeholder). La tabla `periodos_planilla` tiene `preparado_por_usuario_id`, `revisado_por_usuario_id` y `aprobado_por_usuario_id`, pero **nada impide que la misma persona prepare y apruebe** (S10): la segregación de funciones es el corazón del control interno. Ojo: `planilla_detalle_empleado.estado` tiene su **propio** ENUM (`calculado`, `aprobado`, `pagado`) que debe mantenerse en sincronía con el del período.

**Requisitos:** F12 y F18.

**Prompt para la IA**
```
FASE 19 — Ciclo de vida del período de planilla.

TAREAS (en este orden)
1. Migración db/migrations/000119_periodo_ciclo.{up,down}.sql: agrega a periodos_planilla
   calculado_por_usuario_id, fecha_calculo, pagado_por_usuario_id, fecha_pago,
   contabilizado_por_usuario_id, fecha_contabilizacion, version_calculo INT NOT NULL DEFAULT 0
   (cuántas veces se calculó). Agrega un trigger BEFORE UPDATE en periodos_planilla que rechace
   (SIGNAL SQLSTATE '45000') el cambio de estado si violaría la segregación de funciones (ver
   tarea 3) o una transición inválida: defensa en profundidad, además de la validación en Go.
2. internal/planilla/periodo.go (reemplaza el placeholder) y periodo_service.go:
   - TransicionPermitida(desde, hasta EstadoPeriodo) con la tabla: borrador->calculada,
     calculada->borrador (descartar/recalcular), calculada->aprobada, aprobada->pagada,
     pagada->contabilizada. Desde aprobada en adelante NO hay retroceso (inmutable).
   - CrearPeriodo(tipo_planilla_id, fecha_inicio, fecha_fin): valida fecha_fin >= fecha_inicio,
     que el tipo exista y NO haya TRASLAPE de fechas con otro período del mismo tipo (no solo
     igualdad exacta como el UNIQUE actual). Estado inicial borrador.
   - Transiciones: Aprobar, Pagar. Cada una, dentro de db.WithAuditContext, con
     SELECT ... FOR UPDATE del período, verificación del estado esperado (409 si es otro, lo que
     las hace seguras ante doble clic), registro de usuario/fecha y actualización de
     planilla_detalle_empleado.estado en la misma transacción.
   - Descartar/recalcular (calculada->borrador): solo con planilla.calcular y solo si el período
     está calculada; deja un ganchos EliminarDetalle(ctx, q, periodoID) (interfaz que
     implementará F20).
   - Ganchos por interfaz (para que otras fases se conecten sin editar este archivo):
     Calculador (F20), AlAprobar(ctx, q, periodo) (F23) y AlPagar(ctx, q, periodo) (F28).
3. SEGREGACIÓN (regla de negocio, en Go y en el trigger): quien calcula/prepara != quien aprueba
   != quien paga; quien contabiliza != quien pagó. Error de dominio
   ErrSegregacionFunciones con mensaje seguro. El usuario que preparó queda en
   preparado_por_usuario_id al calcular.
4. Endpoints bajo /planillas (permisos de F12): POST /planillas (planilla.crear), GET /planillas
   (planilla.ver; paginada con filtros tipo/estado/rango), GET /planillas/:id (planilla.ver),
   POST /planillas/:id/calcular (planilla.calcular; hasta F20 devuelve 501 con código
   "calculo_no_disponible"), POST /planillas/:id/recalcular (planilla.calcular),
   POST /planillas/:id/aprobar (planilla.aprobar), POST /planillas/:id/pagar (planilla.pagar).
   Respuestas con el estado, quién y cuándo de cada paso. NO toques /planillas/:id/contabilizar
   (F28) ni las rutas de exportación (F09), pero verifica que no choquen.
5. Inmutabilidad: comprueba si existen triggers que impidan modificar planilla_detalle_* cuando el
   período está aprobada/pagada/contabilizada (el comentario de la tabla lo promete). Si no
   existen, créalos en la migración (permitiendo solo el cambio de la columna estado de la
   línea hecho por las transiciones). Si no puedes hacerlo con seguridad, repórtalo.
6. Tests (unitarios sobre la tabla de transiciones y de segregación; integración con BD):
   traslape de fechas, doble aprobación concurrente (una gana, otra 409), aprobar con el mismo
   usuario que preparó (rechazado), retroceso desde aprobada (rechazado).

CRITERIOS DE ACEPTACIÓN
- Ninguna combinación de usuarios puede preparar y aprobar (ni aprobar y pagar) el mismo período.
- Dos aprobaciones simultáneas: exactamente una tiene éxito.
```

**Archivos a pasar**
- `internal/planilla/periodo.go`, `estados.go`, `domain.go`
- `internal/db/dto/planilla_periodo.go` (501)
- `internal/authz/permisos.go`, `internal/api/handlers/routes.go`, `internal/api/middleware/context.go`, `internal/db/audit.go`, `internal/pagination/pagination.go`
- DDL: `periodos_planilla`, `planilla_detalle_empleado`, `planilla_detalle_concepto`, `tipos_planilla` (schema.sql 921–1035) y los triggers de `planilla_detalle_*` y `periodos_planilla` (búscalos con `grep -n "trg_.*planilla" db/migrations/000000*`).
- `internal/planilla/export_service.go` (solo la parte de estados) para no chocar.

**Archivos que crea:** `internal/planilla/periodo_service.go`, `internal/db/dto/planilla_ciclo.go`, `internal/api/handlers/planilla_periodo_handler.go`, tests.
**Migraciones que crea:** `000119_periodo_ciclo`.

---

## FASE 20 — Orquestador de cálculo y persistencia completa

**Contexto.** Nadie hidrata `CalculoEmpleadoInput`, arma los parámetros, recorre a los empleados ni persiste `planilla_detalle_empleado`, `planilla_detalle_concepto` y `planilla_detalle_embargo`. Además la **cuota patronal no se persiste** (la contabilidad la recalcula con el porcentaje vigente al fin del período y aborta si no cuadra: frágil, L7), `planilla_detalle_embargo` no guarda calculado/aplicado/insatisfecho (F9 dejó "F20" ahí), y no se guarda con qué parámetros se calculó (trazabilidad legal: qué porcentaje se aplicó en esa planilla).

**Decisión de diseño (ya tomada):** el cálculo de un período es **todo o nada**. Se calcula a todos los empleados dentro de **una** transacción; si alguno falla, se **reportan todos los errores** (`EmpleadoErrorOutput`) y **no se persiste nada**. Así nunca queda una planilla parcial.

**Requisitos:** F18 y F19.

**Prompt para la IA**
```
FASE 20 — Orquestador de planilla.

TAREAS (en este orden)
1. Migración db/migrations/000120_planilla_detalle_completo.{up,down}.sql:
   a) planilla_detalle_patronal (planilla_detalle_empleado_id, concepto_salarial_id, base
      DECIMAL(14,2), porcentaje DECIMAL(9,6) [FRACCIÓN], monto DECIMAL(14,2), UNIQUE por
      detalle+concepto) para guardar la cuota patronal calculada.
   b) planilla_detalle_embargo: agrega monto_calculado, monto_aplicado, monto_insatisfecho
      (DECIMAL(14,2), NOT NULL DEFAULT 0; el existente `monto` queda = monto_aplicado, agrega un
      CHECK monto = monto_aplicado o elimínalo con backfill; documenta).
   c) periodos_planilla: parametros_snapshot JSON NULL y version_motor VARCHAR(20) NULL.
2. internal/planilla/service.go (archivo NUEVO; el duplicado se eliminó en F09): tipo
   Orquestador con Calcular(ctx, periodoID, usuarioID) (*ResultadoPeriodoOutput, error) que
   implementa la interfaz Calculador de F19. Pasos:
   a) Dentro de db.WithAuditContext: SELECT ... FOR UPDATE del período; exige estado borrador
      (si hay detalle previo de un intento fallido, bórralo primero).
   b) Cargar ParametrosCalculo UNA vez con el CargadorParametros (F18) para las fechas del período.
   c) ListarEmpleadosActivosEnRango; por cada empleado hidratar CalculoEmpleadoInput con:
      contrato vigente y salario (contratos.salario_pactado, D5), días del período en que el
      contrato estuvo vigente, horas extra (SumarHorasExtraPeriodo), incapacidades
      (ListarIncapacidadesSolapadas), préstamos y embargos activos ordenados por prioridad,
      créditos fiscales (ObtenerDatosFamiliaresEnFecha -> CreditoFiscalInput), devengos manuales
      del período (ver tarea 4). Reutiliza los métodos ya existentes de dto/planilla_periodo.go.
   d) Llamar a MotorEmpleado.CalcularEmpleado por empleado. Si hay errores, acumula TODOS
      (EmpleadoErrorOutput: empleado, código, mensaje) y al final aborta la transacción sin
      persistir, devolviendo la lista completa.
   e) Persistir (inserts por lotes) planilla_detalle_empleado (con programa_presupuestario_id
      copiado del expediente), planilla_detalle_concepto (respetando concepto_ajuste_de_id),
      planilla_detalle_patronal y planilla_detalle_embargo (calculado/aplicado/insatisfecho).
   f) Guardar en periodos_planilla el parametros_snapshot (JSON con cada parámetro usado: tabla,
      id de fila, vigencia, valor en fracción o unidad) y version_motor (constante
      "planilla-motor-<semver>" en el código).
   g) Pasar el período a calculada con fecha_calculo, calculado_por_usuario_id y
      version_calculo+1 (usa el servicio de F19, dentro de la misma transacción).
3. Recalcular: implementa EliminarDetalle (borra detalle de los 4 tipos del período) para el
   descarte de F19 (calculada->borrador), dentro de la transacción del descarte.
4. Devengos manuales (comisiones, bonos, viáticos) — hoy no hay dónde registrarlos:
   migración 000120 también crea planilla_devengos_manuales (periodo_planilla_id NULL o
   empleado_id + fecha, concepto_salarial_id, monto, observación, creado_por) y endpoints
   POST/GET/DELETE /planillas/:id/devengos (permiso planilla.crear, solo con período en
   borrador). El orquestador los consume.
5. Conecta POST /planillas/:id/calcular y /recalcular (F19) con el Orquestador. Respuesta:
   resumen (cantidad de empleados, totales de ingresos, deducciones, neto, cuota patronal) o, si
   falló, 422 con la lista completa de errores por empleado.
6. Conecta en main.go/app.go: construye Orquestador, MotorEmpleado, CargadorParametros.
7. Tests: (unitarios) hidratación con repositorio falso; (integración, BD de F13) período con 3
   empleados (uno con embargo, uno con préstamo, uno con hora extra) calculado de punta a punta;
   error de un empleado => no se persiste nada; recalcular no duplica detalle.
8. NO implementes aquí: incapacidades (F21), prorrateo por vigencias (F21), tipificación de
   horas extra (F22), topes de neto/embargos acumulados/préstamos (F23). Si el motor devuelve
   "no implementado" para un empleado con incapacidad, se reporta como error de ese empleado.

CRITERIOS DE ACEPTACIÓN
- Calcular un período con empleados válidos persiste detalle completo, cuota patronal y snapshot.
- Si un empleado falla, la BD queda idéntica a antes del intento.
- El snapshot permite reconstruir qué porcentaje se aplicó en esa planilla.
```

**Archivos a pasar**
- `internal/planilla/domain.go` (254), `motor_calculo.go` (608), `calculator.go` (340), `estados.go`, `periodo_service.go` (F19), `cargador_parametros.go` (F18)
- `internal/db/dto/planilla_periodo.go` (501), `planilla_motor.go` (158), `planilla_ciclo.go` (F19)
- `internal/api/handlers/planilla_periodo_handler.go` (F19), `routes.go`, `internal/authz/permisos.go`
- `cmd/api/main.go` o `internal/app/app.go`, `internal/db/audit.go`
- DDL: `planilla_detalle_empleado`, `planilla_detalle_concepto`, `planilla_detalle_embargo`, `periodos_planilla` (schema.sql 944–1035, 1136–1152), `embargos`, `prestamos` (1066–1135), `conceptos_salariales` (514–586)
- **Informes de traspaso de F18 y F19.**

**Migraciones que crea:** `000120_planilla_detalle_completo`.
> Es una fase grande. Si tu IA no alcanza, corta así: **20A** = tareas 1, 2 (a–d) y 6; **20B** = tareas 2 (e–g), 3, 4, 5 y 7, adjuntando el informe de 20A.

---

## FASE 21 — Incapacidades, prorrateo por vigencias y fuente única de salario

**Contexto.** Tres huecos que dan planillas incorrectas: (L1) con **cualquier** incapacidad el motor devuelve error "aún no implementado" y, además, los días de incapacidad no se descuentan del salario base (riesgo de **pagar dos veces**); `parametros_subsidios_incapacidad` no se usa; (L4) el salario se toma del contrato vigente a `fecha_fin` y solo se prorratea por `DiasTrabajados`, así que un aumento, ingreso o cese a mitad de período no se parte por vigencias; (L3) hay **dos fuentes de salario** (`contratos.salario_pactado` y `empleado_datos_laborales.salario_base_nominal`) y un aumento en datos laborales no llega a la planilla; `contratos.tipo_contrato` (texto) duplica el catálogo `tipos_contrato`.

**Requisitos:** F20.

**Prompt para la IA**
```
FASE 21 — Base salarial del período: fuente única, prorrateo e incapacidades.

TAREAS (en este orden)
1. FUENTE ÚNICA (D5): el motor y el orquestador usan SOLO contratos.salario_pactado del contrato
   vigente. En services/empleado_service.go y contrato_service.go: salario_base_nominal de
   empleado_datos_laborales deja de ser editable por la API; se actualiza como COPIA DERIVADA en
   la misma transacción cuando cambia el salario del contrato (cambio de salario = cerrar
   contrato vigente + nueva vigencia, patrón de 2 pasos existente). Quita la advertencia "los
   salarios difieren" del motor. Comenta la columna como DEPRECADA para cálculo (migración
   000121).
2. tipo_contrato: migración 000121 agrega contratos.tipo_contrato_id (FK a tipos_contrato) con
   backfill por coincidencia de nombre; los que no calcen quedan en un informe (SELECT
   documentado en el .up.sql, sin borrar datos); el servicio escribe el FK. La columna de texto
   queda deprecada (no se elimina).
3. PRORRATEO POR VIGENCIAS: función pura planilla.SegmentarPeriodo(desde, hasta,
   vigenciasContrato) que divide el período en segmentos [inicio, fin, salario, contrato] según
   los contratos/salarios vigentes dentro del rango (ingreso, cese, aumento, cambio de
   contrato). SalarioBasePeriodo = suma de (días del segmento × salario del segmento ÷
   divisor de días), con el divisor y la base de días tomados de parametros_generales
   (DIVISOR_DIAS_MES, F18) y redondeo a 2 decimales una sola vez al final. Reemplaza el uso de
   "salario a fecha_fin". Necesita una consulta nueva en dto (ListarContratosEnRango).
4. INCAPACIDADES: implementa formulaIncapacidad del motor con parametros_subsidios_incapacidad
   (tramos por número de día de incapacidad, con quién paga cada tramo y qué porcentaje del
   salario diario). La numeración de días es por incapacidad CONTINUA (usa la fecha de inicio de la
   incapacidad, que puede ser de un período anterior). Por cada incapacidad solapada:
   a) descuenta del salario base los días de incapacidad (DiasTrabajados efectivos) para no pagar
      dos veces;
   b) calcula el subsidio a cargo del patrono según los tramos y lo agrega como devengo con su
      concepto salarial;
   c) los tramos que paga CCSS/INS NO se pagan por planilla: se registran en el detalle como
      información (concepto informativo o columna) para reportes, sin sumarlos al neto;
   d) el efecto del subsidio en la base de cotización CCSS/renta lo determinan las banderas del
      concepto salarial (afecta_base_...), NO el código.
   Si el modelo de columnas de parametros_subsidios_incapacidad no permite expresar algo de lo
   anterior, NO lo inventes: repórtalo como decisión pendiente (es criterio legal, sección 7).
5. Debes leer el DDL de incapacidades y del motor para decidir los nombres exactos; documenta en
   el código los supuestos de cada regla con "SUPUESTO A VALIDAR:" y lista todos en el informe.
6. Elimina el error "aún no implementado" y los case vacíos que correspondan.
7. Tests golden (table-driven, con decimal): (i) mes completo sin novedades; (ii) ingreso el día
   10; (iii) aumento el día 16 (dos segmentos); (iv) cese el día 20; (v) incapacidad de 2 días
   dentro del período; (vi) incapacidad de 10 días que cruza dos períodos (numeración continua);
   (vii) incapacidad que cubre todo el período. Calcula los esperados a mano en el propio test
   con comentarios.

CRITERIOS DE ACEPTACIÓN
- Un empleado con incapacidad ya no produce error y no se le paga dos veces.
- Un aumento a mitad de período se refleja proporcionalmente.
- Ninguna referencia a salario_base_nominal como fuente de cálculo.
```

**Archivos a pasar**
- `internal/planilla/motor_calculo.go` (608), `domain.go`, `service.go` (orquestador F20), `cargador_parametros.go` (F18)
- `internal/db/dto/planilla_periodo.go`, `planilla_motor.go`, `contratos.go`
- `internal/services/empleado_service.go` (423), `contrato_service.go` (350)
- DDL: `contratos`, `empleado_datos_laborales`, `tipos_contrato` (schema.sql 298–304, 359–380, 481–512), `incapacidades` y `entidades_emisoras_incapacidad` (3729–3747, 238–248), `parametros_subsidios_incapacidad` (755–775), `conceptos_salariales`/`concepto_salarial_formula` (514–545).
- **Informes de F18 y F20.**

**Migraciones que crea:** `000121_salario_fuente_unica_tipo_contrato`.

---

## FASE 22 — Horas extra tipificadas y con autorización

**Contexto.** (L2) `asistencia` calcula "extra" como cualquier exceso sobre el turno, **sin autorización previa y sin tipo** (diurna/nocturna/mixta/doble por feriado o descanso), por eso no puede producir `HoraExtraInput.TipoHoraExtraID`. Y el motor usa **un solo divisor** `HorasMensualesOrdinarias` para toda la corrida: debe depender de la **jornada** (diurna, mixta, nocturna).

**Requisitos:** F20. **Puede ir en paralelo con F21 solo si aceptas resolver el choque en `motor_calculo.go` a mano** (recomendado: después de F21).

**Prompt para la IA**
```
FASE 22 — Horas extra: autorización, tipificación y divisor por jornada.

PRIORIDAD (si no alcanzas, respeta este orden): 1) divisor por jornada, 2) autorización,
3) tipificación automática.

TAREAS
1. Divisor por jornada: migración db/migrations/000122_horas_extra.{up,down}.sql crea
   parametros_jornada (jornada: diurna|mixta|nocturna, horas_diarias, divisor_horas_mensuales,
   vigencia de 2 pasos, fuente_legal NOT NULL) y siembra con fuente_legal =
   'PENDIENTE VALIDAR' los valores que ya usa el código (lee HorasMensualesOrdinarias y el
   cálculo actual; si el código solo tiene uno, siembra ese para la jornada que corresponda y
   deja las otras dos SIN inventar, reportándolo). Cada empleado tiene una jornada por su turno
   asignado (lee turnos/empleado_turno; si no hay campo de jornada, agrégalo a turnos con
   backfill documentado). El cargador (F18) entrega los divisores; CalcularHorasExtra usa el de
   la jornada de cada empleado. Elimina el divisor único.
2. Autorización previa: tabla horas_extra_autorizadas (empleado_id, fecha, horas_autorizadas,
   tipo_hora_extra_id NULL, motivo, estado ENUM('pendiente','autorizada','rechazada'),
   solicitado_por, autorizado_por, fechas) en la misma migración; endpoints (permiso
   asistencia.crear para solicitar; agrega horas_extra.autorizar a authz/permisos.go y a la
   migración de permisos como MIGRACIÓN NUEVA 000122b si hace falta): POST solicitud,
   POST autorizar/rechazar (autorizador != solicitante), GET listado. SumarHorasExtraPeriodo (dto)
   suma SOLO horas autorizadas y nunca más que las trabajadas.
3. Tipificación automática: función pura en internal/asistencia (o planilla)
   TipificarHorasExtra(registro, turno, esFeriado, esDiaDescanso) []HoraExtraTipificada que
   reparte el exceso en tramos diurno/mixto/nocturno según los límites de horario del turno y
   marca doble si cae en feriado o día de descanso (usa calendario_laboral_dia y feriados; los
   límites horarios de jornada y los factores vienen de parametros_horas_extra/parametros_jornada,
   no del código). El registro de asistencia guarda el detalle tipificado en
   registro_asistencia_detalle (revisa su DDL; agrega columnas solo si hace falta).
4. Elimina float64 y cualquier NewFromFloat restante; decimal en todo el camino.
5. Tests golden: extra diurna simple, extra que cruza a nocturna, feriado, día de descanso, hora
   extra sin autorización (no se paga), autorizada por menos horas que las trabajadas.

CRITERIOS DE ACEPTACIÓN
- Horas extra sin autorización no llegan a la planilla.
- El divisor depende de la jornada del empleado.
- No hay factores ni límites horarios escritos en el código.
```

**Archivos a pasar**
- `internal/planilla/motor_calculo.go` (funciones `CalcularHorasExtra` y `HorasMensualesOrdinarias`), `domain.go` (`HoraExtraInput`, `ParametroHoraExtra`), `cargador_parametros.go`
- `internal/asistencia/service.go`, `handler.go`, `calendario.go`, `internal/db/dto/asistencia.go`, `planilla_periodo.go`
- `internal/authz/permisos.go`, `internal/api/handlers/routes.go`
- DDL: `tipos_hora_extra`, `parametros_horas_extra` (249–265, 658–671), `turnos`, `empleado_turno`, `registro_asistencia`, `feriados_patron` (811–890), `calendario_laboral_dia`, `registro_asistencia_detalle` (3705–3717, 3748–3772).
- **Informe de F20** (y F21 si ya terminó).

**Migraciones que crea:** `000122_horas_extra` (y `000122b` si necesitas permisos nuevos).

---

## FASE 23 — Deducciones: CCSS con base mínima, embargos acumulados, topes y préstamos

**Contexto.** Cinco fallas de deducciones: (L6) la CCSS no aplica la **base mínima contributiva** (la tabla existe y está sembrada) ni maneja topes; (L5) si las deducciones superan los ingresos el motor **aborta el cálculo de ese empleado** en vez de topar las deducciones voluntarias al remanente respetando prioridades; (L8) el tope de pensión alimentaria (50 %) se aplica **por embargo** (dos pensiones pueden llevarse el 100 %), `TopeMensual` siempre llega en 0, el "líquido" incluye ingresos no salariales, `SalarioMinimoInembargable` no lo carga nadie, y `planilla_detalle_embargo` pierde el rastro legal; (L9) **el saldo del préstamo y `cuotas_pagadas` nunca se actualizan** al cerrar la planilla (se cobraría de por vida).

**Requisitos:** F20 (y F21 recomendada).

**Prompt para la IA**
```
FASE 23 — Deducciones: CCSS, embargos, tope de neto y préstamos.

TAREAS (en este orden)
1. CCSS con base mínima (L6): en resolverCCSS/motor, la base de cotización = max(base real,
   base mínima contributiva vigente) si la regla de la tabla parametros_base_minima_contributiva
   lo indica (lee sus columnas; no supongas). Sin dato vigente -> error explícito (D8).
   Persiste la base usada en detalle patronal y en la línea obrera. Documenta topes: si la
   tabla o el DDL no los soportan, repórtalo (no inventes). Cuota patronal SIEMPRE persistida
   (F20). Respeta D6: nada de captura manual de INS_RT/BANPOP_PAT/FCL_PAT.
2. Embargos (L8), usando parametros_embargo_escala y la bandera es_pension_alimentaria de F18:
   a) escala por múltiplo del salario mínimo inembargable, sin constantes en el código;
   b) el tope de pensión (tope_porcentaje_liquido) es ACUMULADO sobre todas las pensiones del
      empleado, no por embargo;
   c) el "líquido embargable" se calcula solo con ingresos marcados como salariales (usa la
      bandera del concepto; si no existe una bandera adecuada, propón la columna en una
      migración 000123 con backfill conservador y repórtalo);
   d) SalarioMinimoInembargable sale de parametros_generales (clave
      SALARIO_MINIMO_INEMBARGABLE, agrégala en la migración 000123 con fuente_legal =
      'PENDIENTE VALIDAR' y sin inventar valor: si no hay dato vigente, error explícito);
   e) TopeMensual: si el embargo tiene tope mensual en su registro úsalo; si la tabla no lo tiene,
      agrega la columna (nullable) en 000123;
   f) el orden judicial > Hacienda > CCSS lo define parametros_tipos_embargo.prioridad_legal
      (dato, no código);
   g) persiste calculado / aplicado / insatisfecho por embargo (columnas de F20) y arrastra el
      insatisfecho al período siguiente como deuda pendiente del embargo.
3. Tope de neto (L5): reemplaza el aborto por esta regla: primero se aplican deducciones
   obligatorias (CCSS, renta, embargos según su prioridad legal); luego las voluntarias
   (préstamos, asociación, sindicato) se aplican por orden de prioridad hasta agotar el
   remanente disponible; lo que no cabe queda como cuota pendiente (migración 000123: tabla
   planilla_deduccion_pendiente con empleado, concepto/origen, período, monto) y se cobra en
   el siguiente período. El neto NUNCA es negativo y el empleado NUNCA se descarta por esto.
   Un neto < 0 solo puede ocurrir por ingresos negativos de ajuste: en ese caso, error explícito.
4. Préstamos (L9): al APROBAR el período (gancho AlAprobar de F19, dentro de la misma
   transacción) se aplica cada cuota efectivamente retenida: baja saldo, sube cuotas_pagadas y
   marca 'saldado' cuando corresponda; lo retenido a embargos actualiza su acumulado. Recalcular
   antes de aprobar NO toca saldos. Idempotente: guarda en el detalle qué cuota ya se aplicó para
   que un reintento no duplique. Usa las funciones de prestamos con su bloqueo FOR UPDATE (F05).
5. Tests golden: (i) base bajo la mínima -> se usa la mínima; (ii) dos pensiones que juntas
   pasarían el 50 % -> se topan acumuladas; (iii) deducciones > remanente -> neto 0 y pendiente;
   (iv) préstamo que se salda en la última cuota; (v) aprobar dos veces no duplica el abono.
6. Sustituye de calculator.go las constantes 0.125/0.25/0.5/3 y PrioridadPensionAlimentaria por
   lectura de parámetros (D8). Sin hardcodeo legal.

CRITERIOS DE ACEPTACIÓN
- Ningún porcentaje legal de embargo queda en código Go.
- Un neto negativo no se produce nunca por deducciones.
- Después de aprobar el período, el saldo de cada préstamo baja exactamente una vez.
```

**Archivos a pasar**
- `internal/planilla/calculator.go` (340), `calculator_test.go` (291), `motor_calculo.go` (608), `motor_calculo_test.go`, `domain.go`, `cargador_parametros.go`, `service.go` (F20), `periodo_service.go` (F19)
- `internal/prestamos/service.go`, `internal/db/dto/prestamos_embargos.go`, `planilla_periodo.go`, `planilla_motor.go`
- DDL: `embargos`, `prestamos`, `parametros_tipos_embargo`, `planilla_detalle_embargo`, `parametros_base_minima_contributiva`, `parametros_cargas_sociales` (587–603, 643–657, 740–754, 1066–1152) y las migraciones 000118/000120.
- **Informes de F18, F19, F20** (y F21 si existe).

**Migraciones que crea:** `000123_deducciones_embargos_pendientes`.

---

## FASE 24 — Aguinaldo y salario escolar dentro de la planilla

**Contexto.** (L10, L11) El aguinaldo y el salario escolar son una corrida separada que guarda en tablas propias y **no entra a planilla, dispersión ni asiento**; `CalcularAguinaldo` está **duplicado** (en `planilla` con `DivRound(…,6)` y en `services` con `DivRound(…,2)`); la corrida suma por la bandera `afecta_base_aguinaldo`, pero la liquidación suma `total_ingresos` completo (dos bases distintas); no usa `WithAuditContext`; y las ventanas de fechas y `mesesAguinaldo = 12` están en el código. El seed usa 8.33 % (1/12) para el salario escolar; lo comúnmente aplicado en el sector público es 8.19 % **[VERIFICAR con Contabilidad/Legal del CCDR]** — se deja **parametrizado**.

**Requisitos:** F20. Las cifras dependen de F08 (estados correctos).

**Prompt para la IA**
```
FASE 24 — Aguinaldo y salario escolar como planillas del sistema.

TAREAS (en este orden)
1. Una sola implementación: deja CalcularAguinaldo y CalcularSalarioEscolar únicamente en
   internal/planilla y elimina las copias de internal/services. Redondeo definido y documentado:
   se calcula sin redondear intermedios y se redondea a 2 decimales UNA vez al resultado final.
   Un solo test que fije el criterio.
2. Ventanas y constantes desde parametros_generales (F18): inicio/fin de la ventana del
   aguinaldo, fecha/rango de pago, meses del aguinaldo, mes de pago del salario escolar. Elimina
   fechaUTC(...) con fechas escritas y mesesAguinaldo. Ninguna fecha legal en código.
3. Integración al ciclo de vida: EjecutarAguinaldo(anio) y EjecutarSalarioEscolar(anio)
   crean un PERÍODO de planilla en borrador de un tipo_planilla especial (AGUINALDO /
   SALARIO_ESCOLAR; si no existen esos tipos, siémbralos en la migración 000124) con las
   fechas de la ventana, calculan a cada empleado dentro de una transacción auditada (todo o
   nada, como F20) y dejan las líneas en planilla_detalle_empleado/concepto para que sigan el
   MISMO ciclo: aprobar -> pagar -> dispersión bancaria -> contabilizar. Las tablas aguinaldos y
   salarios_escolares pasan a ser el registro por empleado/año con periodo_planilla_id (FK,
   migración 000124) y UNIQUE por empleado+año para evitar dobles corridas.
4. Base única: la base de aguinaldo y de salario escolar se suma por la bandera
   afecta_base_aguinaldo / afecta_base_salario_escolar del concepto, sobre períodos
   aprobados/pagados/contabilizados (D4). La liquidación (F26) deberá usar esta misma función:
   exponla como planilla.BaseAcumulada(ctx, empleado, desde, hasta, bandera).
5. Tratamiento de cargas: el aguinaldo y el salario escolar no llevan CCSS ni renta SOLO si los
   conceptos salariales están definidos así (banderas); no lo dejes fijo en código. Verifica las
   banderas sembradas y repórtalas como SUPUESTO A VALIDAR.
6. El porcentaje del salario escolar sale de parametros_salario_escolar (fracción, D3) vía el
   cargador. No cambies el 8.33 %: repórtalo como pendiente de validación (8.19 %).
7. Rutas: POST /aguinaldo/:anio/ejecutar (aguinaldo.ejecutar) y POST /salario-escolar/:anio/
   ejecutar (salario_escolar.ejecutar) que devuelven el período creado; cada ejecución solo
   si no existe ya para ese año.
8. Tests golden: base de 12 meses, empleado que ingresó a mitad de año, empleado con incapacidad
   (según banderas), doble ejecución (rechazada).

CRITERIOS DE ACEPTACIÓN
- Aguinaldo y salario escolar pueden aprobarse, pagarse por dispersión y contabilizarse.
- No queda ninguna implementación duplicada ni fecha legal escrita en Go.
```

**Archivos a pasar**
- `internal/planilla/acumulados_periodicos.go` (296), `domain.go`, `service.go` (F20), `periodo_service.go` (F19), `cargador_parametros.go`
- `internal/db/dto/acumulados_liquidaciones.go` (475), `planilla_periodo.go`
- `internal/services/liquidacion_service.go` (solo las funciones `CalcularAguinaldo`/`CalcularSalarioEscolar` y su uso; sugerido: fragmento con `grep -n "Aguinaldo\|SalarioEscolar"`), `internal/authz/permisos.go`, `routes.go`
- DDL: `aguinaldos`, `salarios_escolares`, `parametros_salario_escolar`, `tipos_planilla`, `conceptos_salariales` (3873–3928, 921–943, 514–545)
- **Informes de F18, F19, F20.**

**Migraciones que crea:** `000124_aguinaldo_escolar_planilla`.

---

## FASE 25 — Vacaciones dentro de la planilla y acumulación automática

**Contexto.** (L13) `formulaVacaciones` es un `case` vacío: `PAGO_VACACIONES` **nunca genera monto**; el disfrute de vacaciones no se refleja en la planilla; `AcumularVacaciones` es manual y sin job periódico. **Ojo:** en general el salario mensual no se descuenta cuando hay vacaciones (son días pagados); lo que puede generar un monto extra es el **pago en dinero** de vacaciones (compensación). Qué hace exactamente el concepto `PAGO_VACACIONES` lo dicen sus metadatos (`conceptos_salariales`, `concepto_salarial_formula`) y las tablas de vacaciones: la IA debe leerlos, no adivinar.

**Requisitos:** F20 y F21 (para no contar vacaciones como ausencia).

**Prompt para la IA**
```
FASE 25 — Vacaciones en planilla y acumulación automática.

TAREAS (en este orden)
1. LEE conceptos_salariales/concepto_salarial_formula/regimen_concepto_aplicable para
   PAGO_VACACIONES y los tipos de movimiento de vacaciones_movimientos. En el informe, ANTES de
   programar, describe qué entendiste que debe pagarse (días disfrutados pagados como salario
   normal vs compensación en dinero). Si los datos no permiten decidirlo con certeza, PARA y
   lista la pregunta exacta para una persona (Recursos Humanos/Legal); no implementes por
   suposición.
2. Implementa formulaVacaciones según lo que digan los metadatos: monto = días aplicables ×
   valor del día (salario ordinario del período ÷ DIVISOR_DIAS_MES de parametros_generales),
   con días tomados de movimientos de compensación o disfrute del período, dentro de la
   transacción de F20. El orquestador hidrata VacacionesPeriodoInput (nuevo en domain.go).
3. Los días de vacaciones disfrutadas NO cuentan como ausencia ni reducen DiasTrabajados (F21):
   verifica el punto y agrega test.
4. El movimiento de vacaciones queda ligado al período (vacaciones_movimientos.
   periodo_planilla_id, migración 000125 si falta) para que no se pague dos veces; al APROBAR
   el período (gancho de F19/F23) se marcan como pagados; idempotente.
5. Acumulación automática: comando cmd/jobs con subcomando acumular-vacaciones [--fecha] que
   recorre empleados activos y llama a AcumularVacaciones (idempotente, F07); documenta en
   docs/operacion.md cómo programarlo (cron/Kubernetes CronJob) y que es seguro re-ejecutarlo.
   La escala de días sale de parametros_vacaciones_escala (dato).
6. Tests: acumulación mensual doble (no duplica), disfrute dentro del período (no descuenta),
   compensación (genera monto y se marca pagada al aprobar).

CRITERIOS DE ACEPTACIÓN
- PAGO_VACACIONES genera monto cuando corresponde y nunca dos veces.
- La acumulación se puede correr todos los días sin duplicar.
```

**Archivos a pasar**
- `internal/planilla/motor_calculo.go`, `domain.go`, `service.go` (F20), `periodo_service.go`
- `internal/asistencia/vacaciones_service.go`, `internal/db/dto/vacaciones.go`, `vacaciones_extra.go`
- DDL: `vacaciones_acumulado`, `vacaciones_disfrute`, `vacaciones_solicitudes`, `vacaciones_movimientos`, `parametros_vacaciones_escala`, `conceptos_salariales`, `concepto_salarial_formula`, `regimen_concepto_aplicable` (schema.sql 514–586, 672–691, 1036–1059, 3773–3829).
- **Informes de F07, F20 y F21.**

**Migraciones que crea:** `000125_vacaciones_periodo` (si falta la columna).

---

## FASE 26 — Liquidación laboral corregida

**Contexto.** (L12) La antigüedad se toma del **último contrato** (`contrato.FechaInicioContrato`), no de `empleados.fecha_ingreso`: con renovaciones se pierde la continuidad (afecta cesantía y preaviso); las vacaciones proporcionales se valoran con el promedio de 6 meses cuando el Art. 156 del Código de Trabajo usa el de las últimas 50 semanas **[VERIFICAR]**; no incluye salario escolar proporcional; **no aplica deducciones** (CCSS, renta, embargos); `SetEmpleadoActivo(false)` ocurre de inmediato aunque `fecha_salida` sea futura; no genera asiento ni pago. Y hay hardcodeo: los meses `3`, `6`, `12`, los días `7` y `14` de cesantía, la fracción `> 6` meses, el año tope y `DivisorDiasMes = 30`.

**Requisitos:** F24 y F25 (usan la misma base acumulada y el mismo motor).

**Prompt para la IA**
```
FASE 26 — Liquidación laboral.

TAREAS (en este orden)
1. Antigüedad: usa empleados.fecha_ingreso (continuidad entre contratos). Si hubo interrupciones
   reales entre contratos, la regla debe venir de un dato (agrega el campo/tabla en la
   migración 000126 solo si el modelo no lo cubre) y reportarse como SUPUESTO A VALIDAR.
2. Escalas de cesantía y preaviso 100% desde BD (D8): extiende parametros_cesantia_escala
   /parametros_preaviso_escala (migración 000126) para cubrir tramos menores a 12 meses (3, 6, 12
   meses, 7 y 14 días, "fracción > 6 meses", año tope) con vigencia; siembra los valores que hoy
   están en el código con fuente_legal = 'VALOR HISTÓRICO DEL CÓDIGO - PENDIENTE VALIDAR'. Elimina
   del servicio todas esas constantes y DivisorDiasMes (usa parametros_generales).
3. Motivo de salida: tabla parametros_motivos_salida (código, paga_cesantia, paga_preaviso,
   paga_vacaciones, observación, vigencia) que reemplaza cualquier if por motivo en el código
   (renuncia, despido con y sin responsabilidad, pensión, fallecimiento...). Siembra con
   'PENDIENTE VALIDAR' sin inventar tratamientos: donde no estés seguro, deja el flag en el valor
   más conservador y repórtalo.
4. Vacaciones proporcionales: la ventana del promedio se parametriza
   (VENTANA_PROMEDIO_VACACIONES_SEMANAS, sembrada con el valor actual equivalente a 6 meses y
   marcada 'PENDIENTE VALIDAR: Art. 156 usa 50 semanas'); el saldo sale de F07.
5. Salario escolar proporcional: usa planilla.BaseAcumulada (F24) desde el último pago hasta la
   fecha de salida y el porcentaje vigente. Aguinaldo proporcional con la misma función de F24.
6. Deducciones: la liquidación se calcula con el MISMO motor (MotorEmpleado) como una planilla
   de tipo LIQUIDACION de un solo empleado (tipo sembrado en la migración), de modo que se
   apliquen CCSS, renta y embargos según los conceptos aplicables; los rubros indemnizatorios
   (cesantía, preaviso) respetan las banderas de sus conceptos, no reglas en código.
   Reutiliza el ciclo de F19 (aprobar/pagar/contabilizar) y la dispersión.
7. Salida del expediente: SetEmpleadoActivo(false) y el cierre de contrato/datos laborales se
   ejecutan cuando fecha_salida <= hoy; si es futura, quedan programados (comando
   cmd/jobs cerrar-salidas, idempotente). Todo el proceso, en una sola transacción auditada.
8. Proyectar (simulación) y Procesar comparten el mismo cálculo; proyectar no escribe nada.
9. Tests golden: empleado con 2 contratos consecutivos (antigüedad continua), renuncia con 5
   meses, despido con 3 años, salida futura (no se desactiva hoy), embargo activo aplicado.

CRITERIOS DE ACEPTACIÓN
- Cambiar la escala de cesantía es una migración de datos, no un cambio de código.
- La liquidación aplica deducciones y puede pagarse por dispersión.
```

**Archivos a pasar**
- `internal/services/liquidacion_service.go` (717), `internal/api/handlers/liquidacion_handler.go`
- `internal/db/dto/liquidacion_cierre.go` (F08), `acumulados_liquidaciones.go`, `contratos.go`, `empleados.go`
- `internal/planilla/acumulados_periodicos.go` (post-F24), `service.go`, `motor_calculo.go`, `periodo_service.go`
- `internal/asistencia/vacaciones_service.go` (solo cálculo de saldo)
- DDL: `liquidaciones`, `parametros_cesantia_escala`, `parametros_preaviso_escala`, `regimenes_laborales`, `contratos`, `empleados` (325–342, 438–480, 481–512, 692–739, 3929–3968).
- **Informes de F08, F24 y F25.**

**Migraciones que crea:** `000126_liquidacion_parametros`.
