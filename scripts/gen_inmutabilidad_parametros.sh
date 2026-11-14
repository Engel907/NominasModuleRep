#!/usr/bin/env bash
# Genera (a stdout) los triggers de inmutabilidad de una tabla paramétrica versionada por vigencia:
#   - BU: rechaza cambiar cualquier columna de valor; solo fecha_fin_vigencia (cierre de vigencia) puede cambiar.
#         Una vigencia cerrada no se puede reabrir.
#   - BD: rechaza todo borrado (se cierra la vigencia, no se borra).
# Uso: scripts/gen_inmutabilidad_parametros.sh <tabla> <col1> [col2 ...]
#   Las columnas son TODAS las de la tabla salvo fecha_fin_vigencia y es_vigente (columna generada a partir de aquella).
# Mensajes con prefijo `parametro_inmutable:` (límite de MESSAGE_TEXT: 128 caracteres).
set -euo pipefail

if [ "$#" -lt 2 ]; then
  echo "uso: $0 <tabla> <col1> [col2 ...]" >&2
  exit 2
fi

tabla="$1"; shift
cols=("$@")

if ! [[ "$tabla" =~ ^[a-z][a-z0-9_]*$ ]]; then
  echo "tabla invalida: $tabla" >&2; exit 2
fi
for c in "${cols[@]}"; do
  if ! [[ "$c" =~ ^[a-z][a-z0-9_]*$ ]]; then
    echo "columna invalida: $c" >&2; exit 2
  fi
  if [ "$c" = "fecha_fin_vigencia" ] || [ "$c" = "es_vigente" ]; then
    echo "columna no permitida (es la unica modificable o es generada): $c" >&2; exit 2
  fi
done

# Los nombres de trigger de MySQL admiten 64 caracteres.
nombre_bu="trg_${tabla}_inmut_bu"
nombre_bd="trg_${tabla}_inmut_bd"
if [ "${#nombre_bu}" -gt 64 ]; then
  echo "nombre de trigger demasiado largo: $nombre_bu" >&2; exit 2
fi

msg_bu="parametro_inmutable: ${tabla} solo permite cambiar fecha_fin_vigencia"
msg_bd="parametro_inmutable: ${tabla} no se borra; se cierra la vigencia"
if [ "${#msg_bu}" -gt 128 ] || [ "${#msg_bd}" -gt 128 ]; then
  echo "mensaje supera 128 caracteres para $tabla" >&2; exit 2
fi

cond=""
for c in "${cols[@]}"; do
  if [ -z "$cond" ]; then
    cond="NOT (NEW.${c} <=> OLD.${c})"
  else
    cond="${cond}
     OR NOT (NEW.${c} <=> OLD.${c})"
  fi
done

cat <<SQL
CREATE TRIGGER ${nombre_bu} BEFORE UPDATE ON ${tabla}
FOR EACH ROW
BEGIN
  IF OLD.fecha_fin_vigencia IS NOT NULL
     AND NEW.fecha_fin_vigencia IS NULL THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: no se puede reabrir una vigencia cerrada';
  END IF;
  IF ${cond} THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = '${msg_bu}';
  END IF;
END;

CREATE TRIGGER ${nombre_bd} BEFORE DELETE ON ${tabla}
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = '${msg_bd}';
END;

SQL