#!/usr/bin/env bash
# Imprime solo las firmas públicas (func/type/const/var de primer nivel) de un paquete Go,
# sin cuerpos, para dar contexto sin pasar archivos completos.
# Uso: scripts/firmas.sh internal/planilla [patrón_archivo]
set -euo pipefail
cd "$(dirname "$0")/.."
dir="${1:?uso: $0 <dir> [patron]}"; pat="${2:-.}"
for f in $(ls "$dir"/*.go | grep -v '_test.go' | grep -E "$pat"); do
  echo "// $f"
  tr -d '\r' < "$f" | grep -nE '^(func|type) ' | sed -E 's/ \{$//'
done
