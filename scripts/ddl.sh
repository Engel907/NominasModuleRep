#!/usr/bin/env bash
# Extrae el DDL vigente de tablas desde db/migrations (CREATE TABLE + ALTER TABLE),
# sin COMMENT, para pegarlo en un prompt con el mínimo de tokens.
# Uso:  scripts/ddl.sh [--con-comentarios] tabla [tabla ...]
#       scripts/ddl.sh --triggers tabla      (solo nombres de triggers de la tabla)
set -euo pipefail
cd "$(dirname "$0")/.."

modo=ddl; comentarios=0
case "${1:-}" in
  --con-comentarios) comentarios=1; shift ;;
  --triggers)        modo=trg; shift ;;
esac
[ $# -ge 1 ] || { echo "uso: $0 [--con-comentarios|--triggers] tabla [tabla ...]" >&2; exit 2; }

limpiar() { if [ "$comentarios" = 1 ]; then cat; else sed -E "s/ COMMENT '([^']|'')*'//g"; fi; }

for t in "$@"; do
  echo "-- ===== $t ====="
  for f in db/migrations/*.up.sql; do
    if [ "$modo" = trg ]; then
      tr -d '\r' < "$f" | grep -E "^CREATE TRIGGER .* ON ${t}( |$)" | sed -E "s/^CREATE TRIGGER ([a-z0-9_]+).*/-- trigger \1  ($(basename "$f"))/" || true
      continue
    fi
    tr -d '\r' < "$f" | limpiar | awk -v t="$t" -v f="$(basename "$f")" '
      BEGIN { st = 0 }
      st == 0 && $0 ~ ("^[[:space:]]*CREATE TABLE (IF NOT EXISTS )?" t "[[:space:]]*\\(") { print "-- origen: " f; st = 1 }
      st == 0 && $0 ~ ("^ALTER TABLE " t "([[:space:]]|$)")                              { print "-- origen: " f; st = 2 }
      st == 1 { print; if ($0 ~ /^\)/) { st = 0; print "" } ; next }
      st == 2 { print; if ($0 ~ /;[[:space:]]*$/) { st = 0; print "" } ; next }
    '
  done
done
