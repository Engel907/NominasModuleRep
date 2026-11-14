#!/usr/bin/env bash
# Prueba de restauración de un respaldo. NUNCA toca la BD de producción:
# restaura en una base temporal nueva, la verifica y la borra.
#
#   DB_HOST=… DB_PORT=3306 DB_USER=restaurador MYSQL_PWD=… \
#   AGE_IDENTITY_FILE=/ruta/segura/backup.key \
#   scripts/probar_restauracion.sh /var/backups/planillas/planillas_ccdr_20261112T020000Z.sql.gz.age
#
# Verifica:
#   1. El .sha256 del respaldo (si existe) coincide.
#   2. schema_migrations sin "dirty".
#   3. fn_detalle_cuadra(último período aprobado|pagado|contabilizado) = 1.
#   4. Cada período contabilizado tiene asientos con debe = haber.
# Variables: KEEP=1 conserva la base temporal (para inspección).
# Salida: 0 restauración verificada; 1 error.
set -euo pipefail

respaldo="${1:?uso: probar_restauracion.sh <respaldo.sql.gz.age>}"
: "${DB_HOST:?DB_HOST requerido}"
: "${DB_USER:?DB_USER requerido (debe poder CREATE/DROP DATABASE y crear triggers)}"
: "${MYSQL_PWD:?MYSQL_PWD requerido}"
: "${AGE_IDENTITY_FILE:?AGE_IDENTITY_FILE requerido (llave privada age)}"
DB_PORT="${DB_PORT:-3306}"
export MYSQL_PWD

for b in mysql age gunzip sha256sum sed; do
  command -v "$b" >/dev/null 2>&1 || { echo "restore: falta $b en el PATH" >&2; exit 1; }
done
[[ -r "$respaldo" ]] || { echo "restore: no se puede leer $respaldo" >&2; exit 1; }

if [[ -f "$respaldo.sha256" ]]; then
  ( cd "$(dirname "$respaldo")" && sha256sum --check --status "$(basename "$respaldo").sha256" ) \
    || { echo "restore: el sha256 del respaldo NO coincide" >&2; exit 1; }
  echo "restore: sha256 verificado"
fi

base="restore_test_$(date -u +%Y%m%d%H%M%S)_$$"
sql() { mysql --host="$DB_HOST" --port="$DB_PORT" --user="$DB_USER" --batch --skip-column-names "$@"; }
limpiar() {
  if [[ "${KEEP:-0}" != "1" ]]; then sql -e "DROP DATABASE IF EXISTS \`$base\`" || true; fi
}
trap limpiar EXIT

sql -e "CREATE DATABASE \`$base\` CHARACTER SET utf8mb4"
# DEFINER se quita para no exigir SUPER al restaurar los triggers/función.
age --decrypt --identity "$AGE_IDENTITY_FILE" "$respaldo" \
  | gunzip \
  | sed -E 's/DEFINER=`[^`]+`@`[^`]+`//g' \
  | sql "$base"
echo "restore: restaurado en $base"

dirty="$(sql "$base" -e "SELECT COALESCE(MAX(dirty), 0) FROM schema_migrations")"
[[ "$dirty" == "0" ]] || { echo "restore: schema_migrations quedó dirty" >&2; exit 1; }
version="$(sql "$base" -e "SELECT MAX(version) FROM schema_migrations")"
echo "restore: versión de esquema $version"

ultimo="$(sql "$base" -e "SELECT id FROM periodos_planilla WHERE estado IN ('aprobada','pagada','contabilizada') ORDER BY fecha_fin DESC, id DESC LIMIT 1")"
if [[ -n "$ultimo" ]]; then
  cuadra="$(sql "$base" -e "SELECT fn_detalle_cuadra($ultimo)")"
  [[ "$cuadra" == "1" ]] || { echo "restore: fn_detalle_cuadra($ultimo) = $cuadra (esperado 1)" >&2; exit 1; }
  echo "restore: período $ultimo cuadra"
else
  echo "restore: no hay períodos aprobados todavía (sin verificación de cuadre)"
fi

descuadrados="$(sql "$base" -e "
SELECT COUNT(*) FROM (
  SELECT a.periodo_planilla_id
  FROM asientos_contables a JOIN asientos_contables_detalle d ON d.asiento_contable_id = a.id
  JOIN periodos_planilla p ON p.id = a.periodo_planilla_id
  WHERE p.estado = 'contabilizada'
  GROUP BY a.periodo_planilla_id
  HAVING SUM(d.monto_debe) <> SUM(d.monto_haber)) x")"
[[ "$descuadrados" == "0" ]] || { echo "restore: $descuadrados período(s) contabilizado(s) con asientos descuadrados" >&2; exit 1; }

echo "restore: OK"
