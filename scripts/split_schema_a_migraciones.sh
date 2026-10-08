#!/usr/bin/env bash
# Parte db/schema.sql en migraciones versionadas (golang-migrate) SIN reescribir nada a mano.
# Uso (desde la raíz del proyecto):  bash scripts/split_schema_a_migraciones.sh
# Genera en db/migrations:  0000NN_*.up.sql  y  0000NN_*.down.sql  (7 migraciones).

set -euo pipefail

SRC="${1:-db/schema.sql}"
OUT="${2:-db/migrations}"

[ -f "$SRC" ] || { echo "No existe $SRC"; exit 1; }
mkdir -p "$OUT"

total=$(wc -l < "$SRC")
[ "$total" -ge 4378 ] || { echo "schema.sql tiene $total líneas (se esperaban ~4379). Abortado."; exit 1; }

chk() { sed -n "${1}p" "$SRC" | tr -d '\r' | grep -q -- "$2" || { echo "Marcador no coincide en la línea $1 (esperaba: $2). Abortado."; exit 1; }; }

chk 16   "000000_planillas_ccdr_v11_completo"
chk 2979 "000001_seguridad_reforzada"
chk 3309 "000002_seed_catalogos"
chk 3658 "000003_calendario_asistencia_vacaciones"
chk 3831 "000004_acumulados_liquidaciones"
chk 3970 "Fase 3B"
chk 4240 "Fase 3C"

limpiar() { tr -d '\r' | sed -e '/^DELIMITER/d' -e '/^USE[[:space:]]/d' -e 's/\$\$[[:space:]]*$/;/'; }

M0="$OUT/000000_esquema_base"
M1="$OUT/000001_seguridad_reforzada"
M2="$OUT/000002_seed_catalogos"
M3="$OUT/000003_calendario_asistencia_vacaciones"
M4="$OUT/000004_acumulados_liquidaciones"
M5="$OUT/000005_seed_catalogos_parametricos"
M6="$OUT/000006_rol_admin_y_permisos_base"

# 000000: cabecera + SET (1-118) y todo desde el primer catálogo (184) hasta antes de 000001.
{ sed -n '1,118p' "$SRC"; sed -n '184,2975p' "$SRC"; } | limpiar > "$M0.up.sql"
sed -n '2976,3307p' "$SRC" | limpiar > "$M1.up.sql"
sed -n '3308,3656p' "$SRC" | limpiar > "$M2.up.sql"
sed -n '3657,3829p' "$SRC" | limpiar > "$M3.up.sql"
sed -n '3830,3968p' "$SRC" | limpiar > "$M4.up.sql"
sed -n '3969,4238p' "$SRC" | limpiar > "$M5.up.sql"
sed -n '4239,$p'     "$SRC" | limpiar > "$M6.up.sql"

# (3) Tipos de FK en 000003 y 000004.
for f in "$M3.up.sql" "$M4.up.sql"; do
  sed -i -E 's/\b(empleado_id|creado_por|aprobado_por|procesado_por|turno_id|tipo_ausencia_id)[[:space:]]*INT\b/\1 BIGINT UNSIGNED/' "$f"
done

# (4) Quitar de 000003 las definiciones muertas de turnos y tipos_ausencia.
quitar_tabla() {
  awk -v t="$2" '
    $0 ~ "^CREATE TABLE IF NOT EXISTS " t " \\(" { skip=1; print "-- [split] definición redundante de `" t "` eliminada: la vigente es la de 000000_esquema_base."; next }
    skip && /^\) ENGINE=InnoDB;/ { skip=0; next }
    !skip { print }
  ' "$1" > "$1.tmp" && mv "$1.tmp" "$1"
}

quitar_tabla "$M3.up.sql" turnos
quitar_tabla "$M3.up.sql" tipos_ausencia

# (5) MariaDB -> MySQL 8.
sed -i 's/ADD COLUMN IF NOT EXISTS /ADD COLUMN /' "$M3.up.sql"

# (6) JSON_OBJECT de EMBARGO.
sed -i "s/'usa','embargos','parametros_tipos_embargo'/'usa','embargos','tipos','parametros_tipos_embargo'/" "$M5.up.sql"

# Avisos en cabecera
prepend() { printf '%s\n' "$2" | cat - "$1" > "$1.tmp" && mv "$1.tmp" "$1"; }
prepend "$M3.up.sql" $'-- [split] AVISO: las FK de empleado_id apuntan a empleados(id) (no a usuarios(id) como dice el texto de abajo).\n-- [split] Las tablas turnos y tipos_ausencia ya existen desde 000000_esquema_base; aquí no se redefinen.'
prepend "$M4.up.sql" "-- [split] AVISO: la asunción sobre la FK de empleado_id ya está resuelta: apunta a empleados(id) (BIGINT UNSIGNED)."
prepend "$M6.up.sql" $'-- [split] AVISO DE SEGURIDAD: esta migración siembra el usuario \'admin\' con contraseña conocida.\n-- [split] NO aplicar en ambientes compartidos ni producción; la fase F12 la reemplaza por cmd/seedadmin.'

# ---------------------------------------------------------------------------
# (7) DOWN migrations.
# ---------------------------------------------------------------------------
tablas_de() { grep -E '^[[:space:]]*CREATE TABLE' "$1" | sed -E 's/.*CREATE TABLE (IF NOT EXISTS )?`?([A-Za-z_0-9]+)`?.*/\2/'; }
lista_inversa() { tablas_de "$1" | awk '{a[NR]=$1} END{for(i=NR;i>=1;i--) printf "%s%s", a[i], (i>1?", ":"")}'; }

extraer_triggers() {
  local f="$1"; shift
  for n in "$@"; do
    awk -v n="$n" '
      $0 == "DROP TRIGGER IF EXISTS " n ";" { p=1 }
      p { print }
      p && /^END;/ { p=0; print "" ; exit }
    ' "$f"
  done
}

# 000000
{
  echo "-- down 000000_esquema_base: elimina las tablas que crea el up (y sus triggers)."
  echo "SET FOREIGN_KEY_CHECKS = 0;"
  echo "DROP TABLE IF EXISTS $(lista_inversa "$M0.up.sql");"
  echo "SET FOREIGN_KEY_CHECKS = 1;"
} > "$M0.down.sql"

# 000001
TRG9="trg_aud_usuarios_ins trg_aud_usuarios_upd trg_aud_usuarios_del trg_aud_roles_ins trg_aud_roles_upd trg_aud_roles_del trg_aud_usuario_roles_ins trg_aud_usuario_roles_upd trg_aud_usuario_roles_del"
{
  echo "-- down 000001_seguridad_reforzada: deja usuarios/roles/usuario_roles y sus triggers como en 000000."
  echo "DROP TRIGGER IF EXISTS trg_aud_usuario_roles_ins;"
  echo "DROP TRIGGER IF EXISTS trg_aud_usuario_roles_upd;"
  echo "DROP TRIGGER IF EXISTS trg_aud_usuario_roles_del;"
  echo "DROP TABLE IF EXISTS usuario_roles;"
  echo "DROP TABLE IF EXISTS refresh_tokens;"
  echo "DROP TABLE IF EXISTS tokens_reset_password;"
  echo "ALTER TABLE roles DROP COLUMN activo, DROP COLUMN descripcion;"
  echo "ALTER TABLE usuarios"
  echo "  DROP COLUMN fecha_ultimo_cambio_password, DROP COLUMN requiere_cambio_password,"
  echo "  DROP COLUMN bloqueado_hasta, DROP COLUMN intentos_fallidos,"
  echo "  DROP COLUMN updated_at, DROP COLUMN created_at;"
  echo
  echo "-- usuario_roles como la define 000000 (PK compuesta)"
  awk '/^CREATE TABLE usuario_roles \(/{p=1} p{print} p && /^\) ENGINE=InnoDB/{exit}' "$M0.up.sql"
  echo
  echo "-- triggers de auditoría tal como los define 000000"
  # shellcheck disable=SC2086
  extraer_triggers "$M0.up.sql" $TRG9
} > "$M1.down.sql"

# 000002 y 000005
for m in "$M2:000002_seed_catalogos:catálogos base" "$M5:000005_seed_catalogos_parametricos:parámetros"; do
  IFS=: read -r f nom desc <<<"$m"
  cat > "$f.down.sql" <<DOWN
-- down $nom: migración de datos (${desc}); NO reversible.
SELECT 1;
DOWN
done

# 000003
{
  echo "-- down 000003_calendario_asistencia_vacaciones"
  echo "-- (turnos y tipos_ausencia pertenecen a 000000; NO se tocan aquí)"
  echo "ALTER TABLE vacaciones_acumulado DROP COLUMN dias_disponibles;"
  echo "DROP TABLE IF EXISTS $(lista_inversa "$M3.up.sql");"
  PERMS=$(grep -oE "^[[:space:]]*\('[a-z_]+\.[a-z_]+'" "$M3.up.sql" | sed -E "s/^[[:space:]]*\('//; s/'$//" | sed "s/.*/'&'/" | paste -sd, -)
  echo "DELETE rp FROM rol_permisos rp JOIN permisos p ON p.id = rp.permiso_id WHERE p.codigo IN ($PERMS);"
  echo "DELETE FROM permisos WHERE codigo IN ($PERMS);"
} > "$M3.down.sql"

# 000004
{
  echo "-- down 000004_acumulados_liquidaciones"
  echo "DROP TABLE IF EXISTS $(lista_inversa "$M4.up.sql");"
} > "$M4.down.sql"

# 000006
cat > "$M6.down.sql" <<'DOWN'
-- down 000006_rol_admin_y_permisos_base: migración de datos; NO reversible de forma segura.
SELECT 1;
DOWN

echo "Listo. Archivos en $OUT:"; ls -1 "$OUT"

# Verificaciones estáticas
fallo=0
grep -l 'DELIMITER\|\$\$' "$OUT"/*.sql 2>/dev/null && { echo "ADVERTENCIA: quedaron DELIMITER/\$\$"; fallo=1; } || true
grep -n '^USE[[:space:]]' "$OUT"/*.sql 2>/dev/null && { echo "ADVERTENCIA: quedaron sentencias USE"; fallo=1; } || true
grep -n -E '\b(empleado_id|creado_por|aprobado_por|procesado_por|turno_id|tipo_ausencia_id)[[:space:]]+INT\b' "$M3.up.sql" "$M4.up.sql" 2>/dev/null && { echo "ADVERTENCIA: columnas FK como INT"; fallo=1; } || true
grep -n -E '(ADD|DROP)[[:space:]]+(COLUMN[[:space:]]+)?IF[[:space:]]+(NOT[[:space:]]+)?EXISTS' "$OUT"/*.sql 2>/dev/null && { echo "ADVERTENCIA: sintaxis MariaDB"; fallo=1; } || true
grep -n "'embargos','parametros_tipos_embargo'" "$M5.up.sql" 2>/dev/null && { echo "ADVERTENCIA: JSON_OBJECT de EMBARGO impar"; fallo=1; } || true
grep -n -E 'CREATE TABLE IF NOT EXISTS (turnos|tipos_ausencia) ' "$M3.up.sql" 2>/dev/null && { echo "ADVERTENCIA: siguen las definiciones muertas de turnos/tipos_ausencia"; fallo=1; } || true

[ "$fallo" -eq 0 ] || exit 1
echo "Verificación estática OK."