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
| L7 ⇄ | Escritor de detalle (persistencia) | firmas | — | L5 | Completada; integración verificada en MySQL |
| L8 | Orquestador + cableado + prueba de punta a punta | 3 | 000123 | L6, L7 | Completada; ciclo completo verificado en MySQL |
| L9 ⇄ | Devengos manuales (API) | 2 | — | L8 | Completada; integración verificada en MySQL |
| **C — Reglas (una por archivo)** |||||| 
| L10 | Salario: fuente única, días comerciales, vigencias | 3 | 000124 | L8 | Completada; golden + integración en MySQL |
| L11 ⇄ | Incapacidades | 2 | — | L10 | Completada; golden + integración en MySQL |
| L12 | Jornada y divisor de hora extra | 2 | 000125 | L10 | Completada; unit + integración en MySQL |
| L13 ⇄ | Horas extra: autorización | 3 | 000126 | L12 | Completada; integración en MySQL |
| L14 ⇄ | Horas extra: tipificación | 2 | 000127 | L13 | Completada; golden + integración en MySQL |
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

## Notas L7

- Archivos: `internal/planilla/escritor_detalle.go`, `internal/db/dto/planilla_detalle.go`, tests `escritor_detalle_integration_test.go` y `fixture_calculo_integration_test.go` (fixture compartido con L8: 3 empleados con contrato, datos laborales, embargo, préstamo y horas extra).
- Firmas nuevas: `LineaPatronal`, `LineaPrestamo`, `LineaIncapacidad`, `ResultadoPersistible`, `EscritorDetalle{Escribir, EliminarDetalle, GuardarSnapshot}` (implementa `EliminadorDetalle`), `ErrDetalleNoCuadra{EmpleadoID, Motivo}`; dto `InsertarDetalles{Empleado,Concepto,Patronal,Embargo,Incapacidad,Prestamo}`, `EliminarDetallePeriodo`, `GuardarSnapshotPeriodo`.
- Motor (solo salida nueva, sin cambio de cálculo): `ConceptoCalculadoOutput.Naturaleza`; `CalculoEmpleadoDetalle.{Patronal,Prestamos,Incapacidades}`; `regla_ccss` emite la `LineaPatronal` del concepto de lado patronal; `regla_prestamo` emite una `LineaPrestamo` por préstamo.
- `Escribir` replica `fn_detalle_cuadra` y los CHECK hijos en memoria antes de insertar. Los ids de `planilla_detalle_empleado` se leen de vuelta (no se asumen consecutivos). Lotes de 500 filas.
- `planilla_detalle.go` agregado a `listaBlancaDinamicas` de `TestContratoSQL`.
- Verificado: `TestL7*` (integración), `TestContratoSQL`, `go test ./...`.

## Notas L8

- **Renumeración de migraciones:** L8 necesitó una migración de datos y golang-migrate no aplica versiones menores a la última aplicada. Desde aquí: `000123` L8 · `000124` L10 · `000125` L12 · `000126` L13 · `000127` L14 · `000128` L15 · `000129` L16 · `000130` L17 · … (cada reserva del plan §3 corre +1).
- Migración `000123_regimen_conceptos_codigo_trabajo`: **hallazgo** — `regimen_concepto_aplicable` estaba vacía y ningún empleado se podía calcular. Siembra todos los conceptos activos para `CODIGO_TRABAJO` excepto `INS_RT/BANPOP_PAT/FCL_PAT` (D6). `SUPUESTO A VALIDAR` con RR. HH.
- Archivos: `internal/planilla/service.go` (Orquestador), `app.go` (ganchos `Calculador`/`Eliminador`), `planilla_periodo_handler.go` (`ErrCalculoConErrores` → 422 con `empleados[]`), `routes.go` (sale `Deps.MotorEmpleado`).
- Firmas: `VersionMotor`, `NewOrquestador`, `(*Orquestador).Calcular`, `TipoPlanillaLector`, `ErrCalculoConErrores`, `ErrClaseNoSoportada`, `ErrPeriodoSinEmpleados`, `ErrParametrosIncompletos` (422 con la clave faltante). **Cambiadas:** `ListarConceptosAplicablesEnFecha(ctx, regimen, fecha)` → `(ctx, regimen, tipoPlanillaID, fecha)` (JOIN `tipo_planilla_concepto`, D18) en dto, `MotorRepository` e infra; `evaluarConcepto` recibe el tipo; `CalculoEmpleadoInput.TipoPlanillaID`; `EmpleadoErrorOutput{EmpleadoID, Error}` → `{EmpleadoID, Codigo, Mensaje}` (acumulados actualizado).
- Errores por empleado (códigos `empleado_sin_contrato`, `empleado_sin_datos_laborales`, `datos_incompletos`, `calculo_invalido`, `detalle_no_cuadra`) se acumulan todos; errores de BD/contexto abortan con 500 sin exponer el texto (D9). Solo clase `regular` (otras → 422 hasta L19/L22).
- Lecturas de hidratación/parámetros van por el pool (`*sql.DB`), no por la transacción: son lecturas consistentes sin bloqueo; las escrituras sí van en la transacción.
- Verificado (MySQL): `TestL8CicloCompletoTresEmpleados` (crear→calcular→recalcular sin duplicar→aprobar→pagar, 3 usuarios, cuadre, snapshot con % en fracción, cuota patronal persistida), `TestL8EmpleadoConErrorNoPersisteNada`, `TestL8SinSalarioMinimoInembargable…`; unit `TestFallarCalculoConErrores`.
- Suite de integración: arreglados tests rotos previos (fixture L2 sin `fraccion_mensual`, `security_integration_test` sin BD, `TestHumoAPI` sin issuer/rate-limit/body-limit y sin rol de negocio). Quedan rojos y **preexistentes**: `TestHumoAPI/crear_y_listar_empleado` (cuerpo de alta de empleado desactualizado) y `TestEnumsEstado…/prestamos.estado` (`pagado`, lo resuelve L18).
- Sigue faltando cargar `SALARIO_MINIMO_INEMBARGABLE` real: sin él, calcular responde 422 `parametros_incompletos`.

## Notas L9

- Archivos: `internal/planilla/devengos_service.go`, `internal/db/dto/planilla_devengos.go`, `internal/api/handlers/planilla_devengos_handler.go`, `routes.go` (`Deps.PlanillaDevengos`), `app.go`; test `devengos_integration_test.go`.
- Rutas: `POST /planillas/:id/devengos` y `DELETE /planillas/:id/devengos/:devengo_id` (`planilla.crear`), `GET /planillas/:id/devengos?empleado_id=` (`planilla.ver`, paginado). `monto` se recibe como **texto decimal** (nunca float).
- Reglas: período bloqueado `FOR UPDATE` y en `borrador` (si no, 409); concepto activo, `captura_manual`, `ingreso` y presente en `tipo_planilla_concepto` del tipo del período (422); empleado activo (422); monto > 0, ≤ 2 decimales, cabe en DECIMAL(14,2) (400).
- Firmas: `NewDevengosService`, `(*DevengosService).{Crear,Listar,Eliminar}`, `NuevoDevengo`, errores `ErrDevengoNoEncontrado`, `ErrConceptoNoCapturable`, `ErrEmpleadoNoActivo`, `ErrMontoDevengoInvalido`, `ErrObservacionMuyLarga`.
- Verificado (MySQL): `TestL9DevengosManuales` (el motor consume el devengo; 409 en período calculado), `TestContratoSQL`.

## Notas L10 (000124)

- Migración `000124_salario_fuente_unica`: `contratos.tipo_contrato_id` (FK, backfill por código/nombre; texto deprecado), triggers `trg_edl_salario_derivado_bi` y `trg_contratos_sync_salario_ai` (`salario_base_nominal` = copia derivada de `salario_pactado`), y **auditoría de `contratos`** (no tenía). up/down/up verificado.
- Archivos: `internal/planilla/salario_periodo{,_test}.go`, `regla_salario.go`, `hidratador.go`, `service.go`, `domain.go`, `dto/planilla_hidratacion.go` (`ListarContratosEnRango`), `dto/contratos.go` (el INSERT resuelve `tipo_contrato_id`), `services/empleado_service.go` y `contrato_service.go` (sin `TODO F22`).
- Firmas: `DiasComerciales(desde, hasta, divisor int) int`, `VigenciaContrato`, `SegmentoSalario`, **`SegmentarPeriodo(desde, hasta, v, divisor int)`** (el plan no traía divisor: cada segmento necesita sus días), `SalarioBasePeriodo(s, divisor)`, `DiasSegmentos`; `CalculoEmpleadoInput.Segmentos`; `PeriodoParaCalculo.DivisorDiasMes`; `FuenteHidratacion` cambia `ObtenerContratoVigenteEnFecha` → `ListarContratosEnRango`. **Quitados:** `CalculoEmpleadoInput.SalarioBaseNominal`, `DatosLaboralesInput.SalarioBaseNominal` (la API ya no lo acepta) y la advertencia "salarios difieren".
- Días comerciales (D15, `SUPUESTO A VALIDAR`): posición(fin) − posición(día previo) por mes ⇒ cualquier partición del mes suma `DIVISOR_DIAS_MES`. Un aumento el 31 aporta 0 días; ingreso el último día de febrero aporta 3.
- Segmento = vigencia de la versión ∩ [fecha_inicio_contrato, fecha_fin_contrato]. Convención existente de `CerrarContratoVigente`: la versión vieja cierra el mismo día en que abre la nueva → ese día es de la nueva; otro solape = error.
- Fecha efectiva del empleado = último día con contrato (fin del período o cese). `DiasTrabajados` = Σ días de segmentos (L11 descuenta incapacidad).
- Verificado: golden (mes completo, ingreso 10, aumento 16 con y sin frontera compartida, cese 20, febrero, quincena 16–31, redondeo único), `TestL10AumentoAMitadDePeriodoYSalarioDerivado`, L7–L9 en verde, `TestContratoSQL`.

## Notas L11

- Archivos: `internal/planilla/incapacidad{,_test,_integration_test}.go`, `regla_incapacidad.go`, `regla_salario.go`, `hidratador.go`, `cargador_parametros.go`, `domain.go`, `motor_calculo.go`.
- Firmas: `IncapacidadInput{IncapacidadID, EntidadEmisoraID; Desde, Hasta; DiaSerieDesde}` (reemplaza `{EntidadEmisoraID, DiaDesde, DiasIncapacidad}`), `TramoSubsidio{…DiaHasta *int32; PctPatronal, PctEntidad}` (reemplaza `TramoSubsidioIncapacidad`; `ParametrosCalculo.SubsidiosIncapacidad` cambia de tipo), `SubsidioIncapacidad`, `CalcularSubsidioIncapacidad(salarioDiario, inc, tramos, divisor)`, `SegmentosNetos`.
- Cargador: trae `dia_hasta` y `porcentaje_entidad` (NULL → 0), ambos convertidos a fracción con `porcentaje`.
- `regla_salario` descuenta los días comerciales incapacitados por segmento (no se paga dos veces); `DiasTrabajados` = días con contrato − incapacidad y debe coincidir. `SUB_INCAP` = solo la parte patronal; la de CCSS/INS va a `planilla_detalle_incapacidad.monto_entidad_informativo`. Se quitó el error "aún no implementado". Boletas solapadas → error; incapacidad sin `SUB_INCAP` aplicable → error.
- `SUPUESTOS A VALIDAR` (en `incapacidad.go`): serie por boleta (una prórroga en otra boleta reinicia la serie); cada día vale sus días comerciales; salario diario = salario del segmento ÷ divisor; cotización del subsidio según banderas de `SUB_INCAP` (hoy no afecta CCSS ni renta).
- Verificado: golden (2 días, boleta que cruza dos períodos con serie continua, todo el período, tramo sin `dia_hasta`, sin tramo → error), motor completo, `TestL11IncapacidadEnElCalculo` (MySQL: líneas por tramo, SUB_INCAP 11 666.67, cuadre).

## Notas L12 (000125)

- Migración `000125_jornadas`: catálogo `jornadas` (DIURNA, MIXTA, NOCTURNA), `parametros_jornada` con inmutabilidad (`gen_inmutabilidad_parametros.sh`) y auditoría; **solo DIURNA = 240**, copiado de `HORAS_MENSUALES_ORDINARIAS` (MIXTA/NOCTURNA no se siembran: valor legal). `contratos.jornada_id` y `turnos.jornada_id` (FK) con backfill; `trg_contratos_catalogos_bi` resuelve `jornada_id` y `tipo_contrato_id` desde el texto para todo escritor; auditoría de `contratos` y `turnos` regenerada. up/down/up verificado.
- **Desvío del plan:** `horas_diarias_ordinarias` es NULL-able (el código no tenía ese valor; no se inventa).
- Go: `ContratoVigente.JornadaID` (y `dto.ContratoVigenteEnFechaRow`), `dto.ListarParametrosJornadaEnFecha`, `dto.ExisteJornada`, `FuenteParametros.ListarParametrosJornadaEnFecha` (infra actualizado), `ParametrosCalculo.DivisorPorJornada map[int64]decimal.Decimal`. **Quitados:** `ParametrosCalculo.HorasMensualesOrdinarias` y `ClaveHorasMensualesOrdinarias` (la clave queda en BD, deprecada, nadie la lee). El subselect de `tipo_contrato_id` en `CrearVersionContrato` (L10) se sustituye por el trigger (un solo lugar).
- `regla_horas_extra`: divisor = `DivisorPorJornada[contrato.JornadaID]`; contrato sin jornada del catálogo o jornada sin parámetro vigente → error (solo si hay horas extra). `contrato_service` rechaza (400) una jornada que no esté en el catálogo.
- Verificado: `TestHorasExtraPorJornada` (diurna 1.5 → 50 000; nocturna sin divisor y contrato sin jornada → error), `TestL12JornadaYDivisor` (MySQL), L7–L11, `TestCargador*`, `TestContratoSQL`.

## Notas L13 (000126)

- Migración `000126_horas_extra_autorizadas`: tabla con `chk_hea_segregacion` (quien solicita ≠ quien resuelve) y `chk_hea_estado`, `UNIQUE (empleado_id, fecha)`, auditoría; permiso `horas_extra.autorizar` asignado a `RRHH` por código; sube `permisos_version` de usuarios RRHH (sus tokens se renuevan). up/down/up verificado.
- Rutas (`internal/asistencia/horas_extra_{service,handler}.go`): `POST /horas-extra/autorizaciones` (`asistencia.crear`), `GET /horas-extra/autorizaciones?empleado_id&estado&desde&hasta` (`asistencia.ver`, paginado), `POST /horas-extra/autorizaciones/:id/autorizar|rechazar` (`horas_extra.autorizar`; rechazar exige motivo), `GET /horas-extra/candidatas?desde&hasta[&empleado_id]` (días con horas extra y sin autorización). Errores tipados `ErrorHorasExtra` (nunca `err.Error()` interno). `horas_autorizadas` como texto decimal.
- `dto.SumarHorasExtraPeriodo` (misma firma) suma solo días `autorizada`, con tope `horas_autorizadas` por día; si la autorización fija el tipo, solo ese tipo. `SUPUESTO A VALIDAR`: sin tipo fijado, el tope se reparte por orden de `tipo_hora_extra_id` (lo reemplaza L14).
- Firmas: `authz.PermHorasExtraAutorizar`; dto `InsertarHoraExtraAutorizacion`, `ObtenerHoraExtraAutorizacion[ParaActualizar]`, `ResolverHoraExtraAutorizacion`, `Listar/ContarHorasExtraAutorizaciones`, `ListarHorasExtraCandidatas`, `ExisteTipoHoraExtraActivo`; `asistencia.NewHorasExtraService`, `NewHorasExtraHandler`; `Deps.HorasExtra`.
- Verificado (MySQL): `TestL13HorasExtraSoloAutorizadas` (6/6 → 26 250; 4 de 6 → 17 500; sin autorización o pendiente → sin línea), `TestL13SegregacionAutorizacion` (403 en Go, CHECK 3819 en BD, doble resolución 409, duplicado 409, candidatas). El fixture de cálculo ahora autoriza sus 6 horas.

## Notas L14 (000127)

- **Hallazgo:** la API de asistencia escribe `registro_asistencia_detalle` (un solo exceso sin tipo) y la planilla leía las columnas tipificadas de `registro_asistencia`, que ninguna API llena: ninguna hora extra registrada por API llegaba a planilla. Ahora `RegistrarAsistencia` tipifica el exceso y lo guarda en `asistencia_hora_extra_tipificada` (reemplaza el día en cada upsert); `dto.SumarHorasExtraPeriodo` y `ListarHorasExtraCandidatas` leen de esa tabla (con el tope de autorización de L13). `registro_asistencia` ya no se usa para horas extra.
- Migración `000127_horas_extra_tipificadas`: `tipos_hora_extra.clase` (ENUM único, backfill desde el código), `parametros_franja_nocturna` (**no sembrada**, inmutable, auditada), `asistencia_hora_extra_tipificada` (auditada); auditoría de `tipos_hora_extra` regenerada. up/down/up verificado.
- Firmas: `asistencia.TipificarHorasExtra(entrada, salida, turnoIni, turnoFin, franja FranjaNocturna, esFeriado, esDescanso, jornadaMixta) ([]HoraExtraTipificada, error)`, `FranjaNocturna{Vigente; Inicio, Fin}`, `HoraExtraTipificada{Clase, Horas}`, `ClaseHE*`, errores `ErrFranjaNocturnaNoVigente`, `ErrCalendarioNoGenerado`, `ErrSinJornadaContrato` (422 en el handler); dto `ObtenerFranjaNocturnaEnFecha`, `ObtenerJornadaContratoEnFecha`, `ReemplazarHorasExtraTipificadas`.
- `SUPUESTOS A VALIDAR` (en `tipificacion.go`): horas extra = los últimos N minutos trabajados (N = trabajado − jornada del turno; coincide con `ExcesoJornada` y llegar/salir tarde no genera extra — el plan decía "minutos fuera del turno"); feriado o descanso (`calendario_laboral_dia`: `es_feriado`, o `es_habil = 0`) → todo doble; franja nocturna → nocturna, o mixta si la jornada del contrato es `MIXTA`.
- **Bloqueo operativo:** sin `parametros_franja_nocturna` vigente, registrar un día hábil con exceso responde 422 y no guarda nada (D8). Feriados, descansos y días sin exceso no lo necesitan. Cargar la franja con migración de datos (cierre + inserción, `fuente_legal`).
- Verificado: golden (diurna, cruza a nocturna, mitad/mitad, feriado, descanso, turno que cruza medianoche, exceso nocturno tras medianoche, jornada mixta, llegar y salir tarde), `TestL14RegistrarAsistenciaTipificaHorasExtra` (MySQL), L13 con la nueva fuente.
- Suite de integración completa tras L5–L14 (`go test -p 1 -tags=integration ./...`): verde salvo los 2 rojos **preexistentes** (`TestHumoAPI/crear_y_listar_empleado`, `TestEnumsEstado…/prestamos.estado` → L18). Correr con `-p 1`: en paralelo los paquetes saturan el único MySQL (`ping BD de prueba: context deadline exceeded`).
