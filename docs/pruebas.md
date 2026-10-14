# Pruebas

## Unitarias (sin base de datos)

```bash
go test ./...
```

Las pruebas de integración llevan el build tag `integration`: sin el tag ni se compilan.

## Integración con MySQL real (F13)

Requieren una MySQL 8 y la variable `TEST_DATABASE_URL` con formato go-sql-driver.
El usuario debe poder `CREATE/DROP DATABASE` y crear triggers (el esquema tiene 204),
por eso se usa `root`. Cada test crea una base `ccdr_test_<aleatorio>`, aplica
`db/migrations` con golang-migrate (`multiStatements=true`) y la elimina al terminar.
Sin `TEST_DATABASE_URL`, `NuevaBD` ejecuta `t.Skip`: nunca falla por falta de BD.

### Opción 1: todo automático

```bash
make test-integration            # = RUN_INTEGRATION=1 ./run_full_test_suite.sh
```

Levanta el servicio `mysql_test` (perfil `test`, puerto 3307, datos en tmpfs).

### Opción 2: a mano

```bash
docker compose --profile test up -d --wait mysql_test
export TEST_DATABASE_URL='root:root_dev@tcp(127.0.0.1:3307)/'
go test -tags=integration ./... -count=1 -timeout=10m
```

### Qué cubren

| Test | Archivo | Qué detecta |
|---|---|---|
| `TestContratoSQL` | `internal/db/contrato_sql_test.go` | Prepara **cada** consulta SQL embebida en Go (extraída con `go/ast`) contra el esquema real: columnas/tablas inexistentes. Reporta archivo y línea de cada una. |
| `TestVariantesDinamicasExport` | ídem | Ejecuta las variantes reales de los `IN (?,?,...)` armados dinámicamente. |
| `TestEnumsEstadoCoincidenConConstantes` | `internal/planilla/estados_enum_test.go` | ENUM de `periodos_planilla.estado`, `prestamos.estado` y `embargos.estado` vs. constantes de Go. |
| `TestHumoAPI` | `internal/app/app_integration_test.go` | login → ruta protegida con/sin token → programa y unidad → empleado → listado. |

### Consultas dinámicas

`TestContratoSQL` no puede preparar SQL armado con concatenación o `fmt`. Las lista
con `t.Log` y exige que su archivo esté en `listaBlancaDinamicas` (con comentario de
qué test ejecuta sus variantes). Si agregas SQL dinámico en otro archivo, el test falla
hasta que lo registres y le escribas su test.