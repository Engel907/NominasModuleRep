# Planillas

API REST en Go para calcular, aprobar, pagar y contabilizar planillas conforme a la
legislación laboral de Costa Rica: CCSS, impuesto al salario, embargos, horas extra,
incapacidades, vacaciones, aguinaldo, salario escolar y liquidaciones. Sirve a
cualquier empleador (público, privado, asociación u organización comunal): lo que
cambia de uno a otro se configura, no se programa.

Todo valor legal o de negocio (porcentajes, tramos, escalas, conceptos, fórmulas,
cargas sociales por componente y régimen, deducciones recurrentes por empleado,
feriados, cuentas contables, layouts de reportes) vive en la base de datos con vigencia
y se administra desde la propia API, con doble aprobación y auditoría de cada cambio.
Genera los reportes oficiales (CCSS, INS, Hacienda) por período o por mes y reportes de
gestión en JSON o CSV.

## Requisitos

- Go 1.26
- Docker (MySQL 8 local y de pruebas)

## Puesta en marcha

```bash
cp .env.example .env              # completar las variables marcadas [REQUERIDA]
make db-up                        # MySQL local
make migrate-up                   # esquema y catálogos
go run ./cmd/seedadmin -usuario admin -correo admin@ejemplo.cr
go run ./cmd/api
```

## Comandos

| Comando | Uso |
|---|---|
| `go run ./cmd/api` | servidor HTTP |
| `go run ./cmd/migrate up\|down N\|goto V\|force V\|version` | migraciones |
| `go run ./cmd/jobs <subcomando>` | `verificar-parametros`, `acumular-vacaciones`, `cerrar-salidas`, `cifrar-existentes` |
| `go run ./cmd/seedadmin` | administrador inicial |
| `go test ./...` | pruebas unitarias |
| `make test-integration` | pruebas de integración contra MySQL |
| `./scripts/pruebas_completas.sh` | verificación completa (la misma de la CI) |

## Documentación

- [Arquitectura](docs/arquitectura.md): estructura, decisiones y contratos.
- [Operación](docs/operacion.md): configuración desde la API, jobs, respaldos y checklist de producción.
- [Migraciones](docs/migraciones.md): convención y cambios de parámetros.
- [Pruebas](docs/pruebas.md): unitarias, integración y prueba de cierre de mes.
