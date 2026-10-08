#!/usr/bin/env bash
# run_full_test_suite.sh  (raíz del proyecto)
#
# Verificación completa de planillas-ccdr. Sirve igual en CI que en la
# máquina de un desarrollador antes de abrir un PR.
#
# Uso (desde la raíz del proyecto):
#   ./run_full_test_suite.sh
#   RUN_INTEGRATION=1 ./run_full_test_suite.sh   # activa integración (ver abajo)
#
# Requiere: Go 1.23 o superior.
# Las pruebas de integración necesitarán además MySQL accesible con las
# variables de internal/config (DB_HOST, DB_PORT, DB_USER, DB_PASSWORD,
# DB_NAME) cuando exista la carpeta test/integration/.

set -euo pipefail

# Se ejecuta siempre desde la raíz del proyecto, sin importar desde dónde se llame.
cd "$(dirname "${BASH_SOURCE[0]}")"

# Poner en 1 (o exportar RUN_INTEGRATION=1) cuando exista test/integration/.
RUN_INTEGRATION="${RUN_INTEGRATION:-0}"

echo "==> [0/5] verificando versión de Go (se exige 1.23+)"
if ! command -v go >/dev/null 2>&1; then
  echo "Error: no se encontró 'go' en el PATH." >&2
  exit 1
fi
GO_VERSION="$(go env GOVERSION)"   # ej. go1.23.4
GO_MINOR="$(echo "$GO_VERSION" | sed -E 's/^go[0-9]+\.([0-9]+).*/\1/')"
GO_MAJOR="$(echo "$GO_VERSION" | sed -E 's/^go([0-9]+)\..*/\1/')"
if [[ "$GO_MAJOR" -lt 1 || ( "$GO_MAJOR" -eq 1 && "$GO_MINOR" -lt 23 ) ]]; then
  echo "Error: se requiere Go 1.23 o superior; se encontró $GO_VERSION." >&2
  exit 1
fi
echo "    $GO_VERSION"

echo "==> [1/5] gofmt -l (falla si lista archivos sin formatear)"
UNFORMATTED="$(gofmt -l .)"
if [[ -n "$UNFORMATTED" ]]; then
  echo "Archivos sin formatear:"
  echo "$UNFORMATTED"
  exit 1
fi

echo "==> [2/5] go vet ./..."
go vet ./...

echo "==> [3/5] go build ./..."
go build ./...

echo "==> [4/5] go test ./... -count=1"
go test ./... -count=1

echo "==> [5/5] pruebas de integración"
if [[ "$RUN_INTEGRATION" == "1" ]]; then
  if [[ -d test/integration ]]; then
    go test ./test/integration/... -count=1 -tags=integration -timeout=5m
  else
    echo "Error: RUN_INTEGRATION=1 pero test/integration/ no existe." >&2
    exit 1
  fi
else
  echo "    omitidas (la carpeta test/integration/ aún no existe; use RUN_INTEGRATION=1 cuando exista)"
fi
# Bloque para activar cuando exista test/integration/ (reemplaza el bloque de arriba):
#
#   go test ./test/integration/... -count=1 -tags=integration -timeout=5m

echo ""
echo "Suite completa OK."