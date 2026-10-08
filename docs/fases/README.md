# Informes de fase

Este proyecto se modifica por fases ordenadas. Cada fase deja un **informe de
traspaso** en `docs/fases/FASE-XX.md` (XX = número de la fase con dos dígitos:
`FASE-00.md`, `FASE-01.md`, ...).

## Convención

- **Un informe por fase**, creado al terminar la fase, máximo 30 líneas.
- **Contenido mínimo:**
  - Archivos creados, modificados y eliminados (ruta y una línea de por qué).
  - Firmas públicas cambiadas (firma vieja -> firma nueva).
  - Migraciones añadidas (número reservado `0001NN`, con `.up.sql` y `.down.sql`).
  - Verificación realizada (`gofmt -l`, `go vet`, `go build`, `go test`) o la
    declaración explícita "no verificado con compilador".
  - Errores de compilación que vienen de otros paquetes aún no corregidos.
  - Pendientes y decisiones que quedan abiertas para la fase siguiente.
- **Los informes no se reescriben** después de cerrada la fase: si algo cambia,
  se anota en el informe de la fase que lo cambia.
- La arquitectura vigente y las decisiones vinculantes (D1-D12) viven en
  `docs/ADR-001-arquitectura-vigente.md`; los informes de fase solo registran
  lo que cada fase hizo.