#!/usr/bin/env bash
# SOLO DESARROLLO. Regenera las migraciones, BORRA y recrea la base en el contenedor Docker,
# aplica las migraciones y verifica los conteos esperados (85 tablas, 204 triggers).
# Uso (desde la raíz del proyecto):  bash scripts/rehacer_bd_dev.sh
# Variables opcionales: CONT (contenedor), DB (base), PASS (clave root), PORT (puerto del host).
set -uo pipefail

CONT="${CONT:-mysql-planillas}"
DB="${DB:-planillas}"
PASS="${PASS:-root}"
PORT="${PORT:-3307}"
URL="mysql://root:${PASS}@tcp(localhost:${PORT})/${DB}?multiStatements=true"

echo "== 1. Comprobando que el script de split tiene la corrección de tipos =="
n=$(grep -c "turno_id" scripts/split_schema_a_migraciones.sh || true)
echo "líneas con turno_id en split_schema_a_migraciones.sh: $n (deben ser 2 o más; si es 0, falta la corrección)"

echo "== 2. Regenerando migraciones =="
bash scripts/split_schema_a_migraciones.sh | tail -n 3 || { echo "FALLÓ el split"; exit 1; }

echo "== 3. Tipo de turno_id / tipo_ausencia_id en 000003 (deben ser BIGINT UNSIGNED) =="
grep -n -E "(turno_id|tipo_ausencia_id)[[:space:]]" db/migrations/000003*.up.sql || true

echo "== 4. Recreando la base '$DB' vacía =="
docker exec "$CONT" mysql -uroot -p"$PASS" \
  -e "DROP DATABASE IF EXISTS $DB; CREATE DATABASE $DB CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;" 2>&1 \
  | grep -v "Using a password" || true

echo "== 5. Aplicando migraciones (solo las últimas líneas de salida) =="
migrate -path db/migrations -database "$URL" up 2>&1 | tail -n 6
echo "estado: $(migrate -path db/migrations -database "$URL" version 2>&1 | tail -n 1)"

echo "== 6. Conteos (esperado: 85 tablas y 204 triggers) =="
docker exec "$CONT" mysql -uroot -p"$PASS" "$DB" -N -e \
  "SELECT CONCAT('tablas: ', COUNT(*)) FROM information_schema.tables WHERE table_schema='$DB' AND table_type='BASE TABLE'; \
   SELECT CONCAT('triggers: ', COUNT(*)) FROM information_schema.triggers WHERE trigger_schema='$DB';" 2>&1 \
  | grep -v "Using a password" || true