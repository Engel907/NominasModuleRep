#!/usr/bin/env bash
# run_full_test_suite.sh  (raíz del proyecto)
#
# Verificación completa de planillas-ccdr. Sirve igual en CI que en la
# máquina de un desarrollador antes de abrir un PR.
#
# Uso (desde la raíz del proyecto):
#   ./run_full_test_suite.sh
#   RUN_INTEGRATION=1 ./run_full_test_suite.sh
#
# Integración (F13): con RUN_INTEGRATION=1 corre `go test -tags=integration`.
#   - Si TEST_DATABASE_URL ya está definida, la usa tal cual (MySQL 8 con un
#     usuario que pueda CREATE/DROP DATABASE y crear triggers).
#   - Si no, levanta el servicio mysql_test de docker-compose.yml (puerto 3307,
#     datos en tmpfs) y exporta TEST_DATABASE_URL apuntando a él.
#
# Requiere: Go 1.23 o superior (y Docker para la integración sin URL propia).

set -euo pipefail

# Se ejecuta siempre desde la raíz del proyecto, sin importar desde dónde se llame.
cd "$(dirname "${BASH_SOURCE[0]}")"

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

echo "==> [2/5] go vet ./... (y con -tags=integration, para compilar esas pruebas)"
go vet ./...
go vet -tags=integration ./...

echo "==> [3/5] go build ./..."
go build ./...

echo "==> [4/5] go test ./... -count=1  (las de integración se saltan sin el tag)"
go test ./... -count=1

echo "==> [5/5] pruebas de integración (MySQL real)"
if [[ "$RUN_INTEGRATION" == "1" ]]; then
  if [[ -z "${TEST_DATABASE_URL:-}" ]]; then
    command -v docker >/dev/null 2>&1 || { echo "Error: se necesita docker (o define TEST_DATABASE_URL)." >&2; exit 1; }
    docker compose --profile test up -d --wait mysql_test
    export TEST_DATABASE_URL="root:${DB_ROOT_PASSWORD:-root_dev}@tcp(127.0.0.1:${TEST_DB_PORT:-3307})/"
    echo "    TEST_DATABASE_URL apunta al servicio mysql_test de docker-compose"
  else
    echo "    usando TEST_DATABASE_URL del entorno"
  fi
  go test -tags=integration ./... -count=1 -timeout=10m
else
  echo "    omitidas (usa RUN_INTEGRATION=1; ver docs/pruebas.md)"
fi

echo ""
echo "Suite completa OK."