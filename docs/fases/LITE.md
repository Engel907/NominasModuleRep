# Seguimiento del plan LITE

| Fase | Entrega | Entra (archivos) | Migración | Requiere | Estado |
|---|---|---|---|---|---|
| **A — Base** ||||||
| L0 | Herramientas, lint de migraciones, baseline | 0 | — | F19 | Completada (baseline documentado) |
| L1 | DDL de persistencia del cálculo | DDL | 000120 | L0 | `up/down/up` y contrato SQL verificados en MySQL temporal |
| L2 | Integridad en BD (triggers, función de cuadre, vigencias) | DDL + Go/tests | 000121 | L1 | Verificada en MySQL 8 temporal; un test de integración ajeno a L2 sigue rojo (ver notas L2) |
| L3 | Tipos de planilla (clase) + conceptos por tipo | 2 | 000122 | L1 | Implementada; migración, reversa, seeds y ENUM verificados |
| L4 ⇄ | Parámetros requeridos + verificador + cargador | 2 | — | L3 | Completada; tests, vet y cargador de integración verificados; falta cargar el parámetro legal para habilitar L8 |
| **B — Orquestador** |||||| 
| L5 | Motor → una regla por archivo (sin cambiar comportamiento) | 1 | — | L4 | Completada; tests sin cambios de expectativas |
| L6 ⇄ | Hidratador (lecturas) | 1 + firmas | — | L5 | Completada; unitarios y contrato SQL verificados |
| L7 ⇄ | Escritor de detalle (persistencia) | firmas | — | L5 | Pendiente |
| L8 | Orquestador + cableado + prueba de punta a punta | 3 | — | L6, L7 | Pendiente |
| L9 ⇄ | Devengos manuales (API) | 2 | — | L8 | Pendiente |
| **C — Reglas (una por archivo)** |||||| 
| L10 | Salario: fuente única, días comerciales, vigencias | 3 | 000123 | L8 | Pendiente |
| L11 ⇄ | Incapacidades | 2 | — | L10 | Pendiente |
| L12 | Jornada y divisor de hora extra | 2 | 000124 | L10 | Pendiente |
| L13 ⇄ | Horas extra: autorización | 3 | 000125 | L12 | Pendiente |
| L14 ⇄ | Horas extra: tipificación | 2 | 000126 | L13 | Pendiente |
| L15 ⇄ | CCSS con base mínima + `regimen_ccss_id` | 3 | 000127 | L8 | Pendiente |
| L16 ⇄ | Embargos 100 % desde BD | 2 | 000128 | L8 | Pendiente |
| L17 | Tope de neto (obligatorias vs voluntarias) | 2 | 000129 | L16 | Pendiente |
| L18 | Préstamos al aprobar + gancho múltiple | 3 | 000130 | L17 | Pendiente |
| L19 | Aguinaldo y salario escolar dentro del ciclo | 3 | 000131 | L18 | Pendiente |
| L20 | Vacaciones en planilla + job de acumulación | 3 | 000132 | L18 | Pendiente |
| L21 | Liquidación: parámetros y cálculo puro | 2 | 000133 | L19, L20 | Pendiente |
| L22 | Liquidación por el motor (ciclo completo) | 3 | 000134 | L21 | Pendiente |
| **D — Sistema completo** |||||| 
| L23 | Contabilidad (cuota persistida, pago, segregación) | 3 | 000135 | L22 | Pendiente |
| L24 ⇄ | API de parámetros legales (propuesta/aprobación) | 3 | 000136 | L8 | Pendiente |
| L25 ⇄ | Reportes oficiales + boleta PDF | 3 | 000137 | L22 | Pendiente |
| L26 ⇄ | Ley 8968: cifrado, accesos | 3 | 000138 | F17 | Pendiente |
| L27 ⇄ | CI/CD, observabilidad, backups | 0 | — | L8 | Pendiente |
| L28 | QA final y checklist de producción | 0 | — | todas | Pendiente |

⇄ = puede correrse en paralelo con otras ⇄ de su misma franja de requisitos.

## Baseline de pruebas

`bash ./run_full_test_suite.sh` (Go 1.26.2) falla en `[1/5] gofmt -l` y termina ahí; por `set -e`, `go vet`, `go build` y `go test` no se ejecutaron. Archivos listados por `gofmt`:

```text
cmd\api\main.go
cmd\seedadmin\main.go
cmd\seedadmin\main_test.go
internal\api\handlers\dashboard_handler.go
internal\api\handlers\empleado_handler.go
internal\api\handlers\organizacion_handler.go
internal\api\handlers\planilla_periodo_handler.go
internal\api\middleware\auth.go
internal\api\middleware\bodylimit.go
internal\api\middleware\cors.go
internal\api\middleware\errors.go
internal\api\middleware\exigir_cambio_password.go
internal\api\middleware\exigir_cambio_password_test.go
internal\api\middleware\headers.go
internal\api\middleware\logging.go
internal\api\middleware\ratelimit.go
internal\api\middleware\require_permission.go
internal\api\middleware\sesion.go
internal\api\middleware\sesion_test.go
internal\app\app_integration_test.go
internal\asistencia\handler.go
internal\auth\notificador.go
internal\auth\politica.go
internal\auth\politica_test.go
internal\authz\migraciones_test.go
internal\config\config.go
internal\contabilidad\sql_adapters.go
internal\db\contrato_sql_test.go
internal\db\dto\planilla_ciclo.go
internal\db\dto\security_integration_test.go
internal\parametros\generales.go
internal\parametros\generales_test.go
internal\parametros\repositorio_sql.go
internal\planilla\estados_enum_test.go
internal\planilla\periodo.go
internal\planilla\periodo_integracion_test.go
internal\planilla\periodo_service.go
internal\planilla\periodo_test.go
internal\prestamos\estados.go
internal\prestamos\routes.go
internal\prestamos\service.go
internal\security\paseto.go
internal\security\paseto_test.go
internal\seedadmin\seedadmin.go
internal\shared\porcentaje\porcentaje.go
internal\shared\porcentaje\porcentaje_test.go
internal\testutil\db.go
```

Validación específica L0: `go test ./internal/db -run '^TestMigracionesLint$' -count=1` pasa.

## Notas L1 (000120)

- Archivos: `db/migrations/000120_motor_persistencia.up.sql` y `.down.sql`.
- Firmas Go nuevas: ninguna (solo DDL).
- Migración: tablas/columnas del DDL L1 y auditoría de `planilla_devengos_manuales`; regenera auditoría existente de `periodos_planilla` y `planilla_detalle_empleado`, restaurándola en el `.down`.
- `planilla_detalle_embargo` no tiene auditoría previa; no se crea ahora. Su inmutabilidad queda para L2.
- Verificado en MySQL 8 temporal: migraciones `up` hasta 120, `down 1` hasta 119 y `up` otra vez hasta 120.
- `TestContratoSQL` prepara 189 consultas estáticas y valida rutas dinámicas por sus métodos de integración; fragmento `usuarioAdminSelect` se valida al ejecutarse en sus consultas completas.
- `TestVariantesDinamicasSQL` ejercita acumulados, dashboard, ciclo, bases de aguinaldo/salario escolar, administración de usuarios/auditoría y carga de líneas de asientos.
- Chequeo de solo lectura en la única BD Docker disponible (`planillas_ccdr`, MySQL 8.0.46): `planilla_detalle_empleado` tiene 0 filas; 0 incumplen `chk_pde_neto`.
- `chk_pdemb_calc` se prepara igualando `monto_calculado` con `monto`. El chequeo solo cubre esta BD Docker vacía; repetirlo si se migra/restaura una BD con datos.


## Notas L2 (000121)

- Archivos: `db/migrations/000121_integridad_bd.{up,down}.sql`, `scripts/gen_inmutabilidad_parametros.sh <tabla> <cols…>`; Go: `internal/planilla/{periodo.go,periodo_service.go,periodo_test.go}`.
- Firmas nuevas: `fn_detalle_cuadra(BIGINT UNSIGNED) -> TINYINT(1)`, vista `v_planilla_resumen_periodo`; Go `ErrDetalleDescuadrado` (`*ErrorCiclo`, código `detalle_descuadrado`, 409) y su caso en `traducirErrorBD`.
- Migración (31 triggers): `trg_pp_cuadre_bu` (FOLLOWS `trg_pp_ciclo_bu`); `trg_{pdemb,pdpat,pdinc,pdpre}_inmutable_{bi,bu,bd}` (préstamo: solo `aplicado_en` NULL→valor); `trg_pg_vigencia_{bi,bu}`; `trg_<tabla>_inmut_{bu,bd}` en las 8 tablas paramétricas. Cubre la inmutabilidad de `planilla_detalle_embargo` anunciada en L1.
- Las vigencias generales rechazan traslapes tanto al insertar como al cerrar/extender; un cierre sin traslape está permitido y una vigencia cerrada no se puede reabrir. La misma regla de no reapertura aplica a las 8 tablas paramétricas versionadas.
- `Aprobar` conserva `ErrPeriodoSinDetalle` (422) si no hay líneas; el trigger devuelve `ErrDetalleDescuadrado` (409) para líneas cuyos importes no cuadran.
- Verificado: `TestMigracionL2DownUp` baja/reaplica 000121 en una BD efímera; pruebas L2 de cuadre/vacío, embargo aprobado, `porcentaje_obrero`, cierre, traslape y reapertura pasan con `go test -tags=integration ./internal/planilla -run '^TestL2' -count=1`.
- `TestMigracionesLint`, `TestContratoSQL`, `go test ./internal/planilla`, `go vet ./internal/db ./internal/planilla` y `git diff --check` pasan. La suite completa de integración (`go test -tags=integration ./internal/db ./internal/planilla -count=1`) conserva un fallo ajeno a L2: `TestEnumsEstadoCoincidenConConstantes/prestamos.estado` encuentra `pagado` solo en Go, no en el ENUM de la BD.
- Con binlog activo, crear la función exige `log_bin_trust_function_creators=1` o que el usuario de migraciones tenga SUPER. Verificado en el contenedor MySQL temporal: con binlog activo, trust en 0 y usuario sin SUPER, `CREATE FUNCTION` falla con el error 1419 esperado; trust se restauró a 1 y se limpió el usuario/base temporales.

## Notas L3 (000122)

- Archivos: `db/migrations/000122_tipos_planilla_clase.{up,down}.sql`, `internal/planilla/estados.go`, `internal/planilla/estados_enum_test.go`, `internal/db/dto/planilla_tipo.go`.
- `go vet ./internal/db ./internal/planilla` y `go test ./internal/db ./internal/planilla -count=1` pasan; `shopspring/decimal` ya está en `go.mod`.
- Verificado en MySQL 8 temporal: la última migración baja y vuelve a subir sin dirty; seed crea 5 tipos y exactamente 28 relaciones de conceptos; el ENUM `tipos_planilla.clase` coincide con las constantes Go.
- El endpoint API POST de creación mensual/quincenal no se probó de punta a punta. La migración habilita los tipos; el flujo de `Crear` existente no fue modificado en L3.
- La lista de 14 conceptos regulares se mantiene sin `INS_RT`, `BANPOP_PAT` ni `FCL_PAT`, conforme a D6 del ADR-001: las cargas se modelan de forma agregada en `CCSS_OBR`/`CCSS_PAT` y no se capturan esos rubros por separado para evitar doble conteo. Sus filas heredadas en `orden_calculo_concepto` no forman parte de `tipo_planilla_concepto` y no requieren cambios en 000122.

## Notas L4

- Archivos: `internal/parametros/requeridos{,_test}.go`, `internal/planilla/cargador_parametros.go`, `motor_calculo.go` (campo `DivisorDiasMes`), `cargador_parametros_integration_test.go`, `cmd/jobs/main.go`, `internal/config/{config,jobs}.go`, `docs/migraciones.md`.
- Firmas nuevas: `parametros.Requerido`, `Requeridos()`, `(*Generales).Faltantes(ctx, fecha)`, `config.LoadJobsConfig()`; `ParametrosCalculo.DivisorDiasMes`; `cargarGenerales` (privada).
- `Cargar` ahora exige `DIVISOR_DIAS_MES` y `SALARIO_MINIMO_INEMBARGABLE` (D13, global, > 0); si faltan: `ErrParametroFaltante` + `ErrParametroNoConfigurado` con la clave.
- Migración: ninguna. `SALARIO_MINIMO_INEMBARGABLE` no está sembrado; plantilla en `docs/migraciones.md`. Hasta cargarlo, `Cargar` falla en cualquier ambiente.
- `cmd/jobs verificar-parametros [--fecha]`: salida 0 ok, 1 faltantes, 2 error operativo.
- `HORAS_MENSUALES_ORDINARIAS` sigue en `Cargar`, fuera de `Requeridos()`, hasta L12.
- Verificación: `go vet ./cmd/jobs ./internal/parametros ./internal/planilla ./internal/config`, tests de esos paquetes, `TestMigracionesLint` e integración `Cargador` pasan. Los archivos Go de L4 quedaron formateados; `gofmt -l` aún lista archivos preexistentes de otras fases (`generales.go`, `generales_test.go`, `repositorio_sql.go`, `periodo.go`, `periodo_test.go` y `periodo_integracion_test.go`).
- `cmd/jobs verificar-parametros` en BD temporal reportó únicamente `SALARIO_MINIMO_INEMBARGABLE` y salió con código 1, como corresponde. Esto no bloquea el cierre técnico de L4, pero sí el cálculo que requiera ese parámetro y L8 hasta cargar monto, fecha y fuente legal confirmados.
- `git diff --check` pasa. `connectDB` duplicado en `cmd/api` y `cmd/jobs` (extraer en L27).

## Notas L5

- Archivos nuevos: `internal/planilla/reglas.go` (contrato `ContextoRegla`, `Regla`, `reglasPorDefecto()`, constantes `formula*`), `regla_{salario,horas_extra,incapacidad,ccss,renta,embargo,prestamo,diferida}.go`, `reglas_test.go`.
- `motor_calculo.go` queda con tipos de acceso a datos, `evaluarConcepto`, `CalcularEmpleado`, `calcularConcepto` (captura manual/fijo + despacho a `m.reglas`) y totales. `CalcularHorasExtra` vive en `regla_horas_extra.go`; `resolverCCSS` y `ladoObrero/ladoPatronal` en `regla_ccss.go`.
- Firmas: `MotorEmpleado` gana el campo privado `reglas map[string]Regla`; `NewMotorEmpleado(repo)` sin cambios. Tipo de fórmula desconocido sigue dando error.
- Migración: ninguna.
- Verificado: `go vet ./internal/planilla`, `go test ./internal/planilla/... -count=1`, `go build ./...`; archivos de L5 sin salida en `gofmt -l`. Ningún test existente cambió.
- Pendiente: L6 (hidratador) y L7 (escritor de detalle) pueden ir en paralelo.

## Notas L6

- Archivos: `internal/planilla/hidratador{,_test}.go`, `internal/db/dto/planilla_hidratacion.go`.
- Firmas nuevas: `PeriodoParaCalculo`, `EntradaEmpleado`, `FuenteHidratacion` (los 7 métodos del plan + `ListarCreditosFiscalesEnFecha` para traducir CONYUGE/HIJO a `tipo_credito_fiscal_id`), `NewHidratador`, `(*Hidratador).EmpleadosDelPeriodo`, `(*Hidratador).Hidratar`, `ErrEmpleadoSinContrato{EmpleadoID}`, `ErrEmpleadoSinDatosLaborales{EmpleadoID}`; dto `ListarDevengosManualesEmpleado`, `ObtenerContextoLaboral` → `ContextoLaboralRow{ProgramaPresupuestarioID; ClasificacionPuestoID, RegimenCCSSID *int64}` (`RegimenCCSSID` siempre nil hasta L15).
- **Ampliación del contrato:** `PeriodoParaCalculo.DiasPeriodoCompleto` (= `DivisorDiasMes × FraccionMensual`, lo llena L8). El hidratador no conoce el divisor y no hardcodea 30; `DiasTrabajados` = ese valor de forma provisional (`TODO L10`). Sin valor → error.
- Embargo: `porcentaje` de BD (0–100) pasa a fracción con `porcentaje.DesdePorcentaje`; sin porcentaje ni monto fijo → error. `TopeMensual` queda en cero (la tabla no lo trae; L16).
- Incapacidad (`SUPUESTO A VALIDAR`, L11): días naturales de la boleta dentro del período; `DiaDesde` numerado de forma continua desde el inicio de la boleta.
- Contrato y datos laborales se resuelven a `fecha_fin`: un cese a mitad de período da `ErrEmpleadoSinContrato` hasta L10.
- Verificado: `go vet`, `go test ./internal/planilla/... ./internal/db/...`, `gofmt -l` limpio en archivos de L6. `TestContratoSQL` y `TestVariantesDinamicasSQL` pasan contra MySQL 8 (contenedor de desarrollo, puerto 3307).
