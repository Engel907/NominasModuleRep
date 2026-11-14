# Pruebas

## Unitarias (sin base de datos)

```bash
go test ./...
```

Las pruebas de integración llevan el build tag `integration`: sin el tag ni se compilan.

## Integración con MySQL real

Requieren una MySQL 8 y la variable `TEST_DATABASE_URL` con formato go-sql-driver.
El usuario debe poder `CREATE/DROP DATABASE` y crear triggers, por eso se usa `root`.
Cada test crea una base `planillas_test_<aleatorio>`, aplica
`db/migrations` con golang-migrate (`multiStatements=true`) y la elimina al terminar.
Sin `TEST_DATABASE_URL`, `NuevaBD` ejecuta `t.Skip`: nunca falla por falta de BD.

### Opción 1: todo automático

```bash
make test-integration            # = RUN_INTEGRATION=1 ./scripts/pruebas_completas.sh
```

Levanta el servicio `mysql_test` (perfil `test`, puerto 3307, datos en tmpfs).

### Opción 2: a mano

```bash
docker compose --profile test up -d --wait mysql_test
export TEST_DATABASE_URL='root:root_dev@tcp(127.0.0.1:3307)/'
go test -p 1 -tags=integration ./... -count=1 -timeout=90m
```

`-p 1`: un solo MySQL; en paralelo los paquetes lo saturan (`ping BD de prueba: context deadline exceeded`).

### Qué cubren

| Test | Archivo | Qué detecta |
|---|---|---|
| `TestContratoSQL` | `internal/db/contrato_sql_test.go` | Prepara **cada** consulta SQL embebida en Go (extraída con `go/ast`) contra el esquema real: columnas/tablas inexistentes. Reporta archivo y línea de cada una. |
| `TestVariantesDinamicasExport` | ídem | Ejecuta las variantes reales de los `IN (?,?,...)` armados dinámicamente. |
| `TestEnumsEstadoCoincidenConConstantes` | `internal/planilla/estados_enum_test.go` | ENUM de `periodos_planilla.estado`, `prestamos.estado` y `embargos.estado` vs. constantes de Go. |
| `TestHumoAPI` | `internal/app/app_integration_test.go` | login → ruta protegida con/sin token → programa y unidad → empleado → listado; `/healthz`, `/readyz`, `/metrics`. |
| `TestCierreDeMesCompleto` | `internal/e2e/cierre_mes_integration_test.go` | Prueba dorada: 11 empleados (ingreso/cese/aumento a mitad de mes, incapacidad que cruza períodos, horas extra autorizadas, 2 pensiones + embargo judicial, préstamo que se salda, deducciones > remanente, devengos, renta), números calculados a mano; dispersión = Σ neto; liquidación, aguinaldo y salario escolar hasta contabilizar con asientos cuadrados. |
| `TestModalidadesDeSalarioYAjustePorEmpleado` | `internal/planilla/modalidad_y_ajustes_integration_test.go` | Salario diario por días comerciales y por asistencia, salario por hora con horas extra a la tarifa, y un `SALARIO_DIARIO_BASE` propio de un empleado que solo cambia su cálculo. |
| `TestFormulaPropiaDelEmpleado` | ídem | Una fórmula propia reemplaza a la general solo para ese empleado y queda trazada en `planilla_detalle_concepto.empleado_concepto_formula_id`. |
| `TestEmbargoLiquidacionAcumulaConLaRegular` | ídem | La regular aprobada más la liquidación retienen por embargo lo mismo que un cálculo sobre el líquido del mes. |
| `TestMigracionesDownUp` | `internal/db/migraciones_integration_test.go` | Baja y reaplica las últimas seis migraciones. |
| `TestAsignacionesDeConceptos` | `internal/rrhh/asignaciones_integration_test.go` | Asignación, reemplazo con cierre del día previo, traslape, concepto no asignable, cierre e inmutabilidad. |
| `TestHistoricoSalariosCompletaLasBases` | `internal/rrhh/historico_salarios_integration_test.go` | Carga del histórico, duplicados, bases mayores que el bruto y su efecto en las bases de aguinaldo y de liquidación. |

### Consultas dinámicas

`TestContratoSQL` no puede preparar SQL armado con concatenación o `fmt`. Las lista
con `t.Log` y exige que su archivo esté en `listaBlancaDinamicas` (con comentario de
qué test ejecuta sus variantes). Si agregas SQL dinámico en otro archivo, el test falla
hasta que lo registres y le escribas su test.