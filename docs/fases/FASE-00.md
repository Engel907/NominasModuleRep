# FASE 00 — Base y limpieza del repositorio

**Verificación:** no verificado con compilador (no se tocó ningún `.go`). `run_full_test_suite.sh` pasó `bash -n`.
**Errores de otros paquetes:** ninguno evaluado.

## Creados
- `.env.example` — plantilla real con las 27 variables de `internal/config/config.go`.
- `.gitignore` — binarios, `.env`, `*.log`, cobertura y `/tmp`.
- `docs/ADR-001-arquitectura-vigente.md` — único documento de arquitectura, con D1–D12.
- `docs/fases/README.md` y este informe — convención de informes de traspaso.

## Modificados
- `go.mod` — sin la nota obsoleta; `go 1.23`; dependencias intactas.
- `run_full_test_suite.sh` — raíz del proyecto, Go 1.23+, orden gofmt, vet, build, test; integración opcional.

## Eliminados
- Configuración y scripts del generador de código anterior: 3 archivos (raíz, `scripts/`, `db/`).
- `internal/db/queries/`: los 7 `.sql` (segunda fuente de verdad desalineada).
- `ADR-000_decisiones_arquitectura.md`, `ORGANIZACION.md`, `CICD.md` — fusionados en el ADR-001.

## Firmas públicas cambiadas
Ninguna.

## Pendientes
1. `internal/config/config.go`: dos comentarios obsoletos (cita `ORGANIZACION.md`; dice que la clave del `.env.example` es todo ceros).
2. Falta `go.sum`: `go mod tidy` con red.
3. Decidir retiro de `db/schema.sql`, `scripts/split_schema_a_migraciones.sh` y `db/migrations/_drop_tablas_original.txt`.
4. `internal/asistencia` e `internal/prestamos` sin contrato único de error.
5. Sin `.gitattributes`: recomendable `*.sh text eol=lf`.