#!/usr/bin/env bash
# Respaldo cifrado de la BD de planillas.
#
#   DB_HOST=… DB_PORT=3306 DB_USER=respaldo DB_NAME=planillas_ccdr \
#   MYSQL_PWD=… AGE_RECIPIENTS_FILE=/etc/planillas/backup.pub \
#   BACKUP_DIR=/var/backups/planillas RETENCION_DIAS=35 scripts/respaldar_bd.sh
#
# - mysqldump --single-transaction (consistente sin bloquear, InnoDB),
#   --routines --triggers --events (fn_detalle_cuadra, ~230 triggers),
#   --hex-blob (columnas cifradas VARBINARY).
# - Cifrado con age (https://age-encryption.org) a las llaves públicas de
#   AGE_RECIPIENTS_FILE. El respaldo NUNCA se escribe en claro a disco.
# - La contraseña va por MYSQL_PWD (nunca en la línea de comandos: `ps` la vería).
# - Escribe <archivo>.sha256 y borra respaldos con más de RETENCION_DIAS días.
# Salida: 0 ok; 1 error.
set -euo pipefail

: "${DB_HOST:?DB_HOST requerido}"
: "${DB_USER:?DB_USER requerido}"
: "${DB_NAME:?DB_NAME requerido}"
: "${MYSQL_PWD:?MYSQL_PWD requerido (no se acepta contraseña en argumentos)}"
: "${AGE_RECIPIENTS_FILE:?AGE_RECIPIENTS_FILE requerido (llaves públicas age)}"
: "${BACKUP_DIR:?BACKUP_DIR requerido}"
DB_PORT="${DB_PORT:-3306}"
RETENCION_DIAS="${RETENCION_DIAS:-35}"
export MYSQL_PWD

for b in mysqldump age gzip sha256sum; do
  command -v "$b" >/dev/null 2>&1 || { echo "backup: falta $b en el PATH" >&2; exit 1; }
done
[[ -r "$AGE_RECIPIENTS_FILE" ]] || { echo "backup: no se puede leer $AGE_RECIPIENTS_FILE" >&2; exit 1; }
[[ "$RETENCION_DIAS" =~ ^[0-9]+$ && "$RETENCION_DIAS" -ge 1 ]] || { echo "backup: RETENCION_DIAS inválido" >&2; exit 1; }

umask 077
mkdir -p "$BACKUP_DIR"
marca="$(date -u +%Y%m%dT%H%M%SZ)"
destino="$BACKUP_DIR/${DB_NAME}_${marca}.sql.gz.age"
parcial="$destino.parcial"
trap 'rm -f "$parcial"' EXIT

# Solo el mysqldump de MySQL 8: el de MariaDB vuelca los valores de las
# columnas generadas (es_vigente) y MySQL rechaza ese respaldo al restaurar
# (error 3105).
if mysqldump --version 2>/dev/null | grep -qi mariadb; then
  echo "backup: se requiere el mysqldump de MySQL 8 (el de MariaDB genera respaldos no restaurables)" >&2
  exit 1
fi

mysqldump --host="$DB_HOST" --port="$DB_PORT" --user="$DB_USER" \
  --single-transaction --quick --routines --triggers --events --hex-blob \
  --set-gtid-purged=OFF --no-tablespaces --default-character-set=utf8mb4 \
  "$DB_NAME" \
  | gzip -9 \
  | age --encrypt --recipients-file "$AGE_RECIPIENTS_FILE" > "$parcial"

# Un dump vacío o truncado no se da por bueno.
[[ -s "$parcial" ]] || { echo "backup: el respaldo quedó vacío" >&2; exit 1; }
mv "$parcial" "$destino"
( cd "$BACKUP_DIR" && sha256sum "$(basename "$destino")" > "$(basename "$destino").sha256" )
echo "backup: $destino ($(wc -c < "$destino") bytes)"

# Retención: solo archivos de este script y de esta BD.
find "$BACKUP_DIR" -maxdepth 1 -type f \( -name "${DB_NAME}_*.sql.gz.age" -o -name "${DB_NAME}_*.sql.gz.age.sha256" \) \
  -mtime +"$RETENCION_DIAS" -print -delete
