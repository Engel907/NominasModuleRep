#!/usr/bin/env bash
# Genera los 3 triggers de auditoría (ins/upd/del) con el MISMO patrón de 000000/000001.
# Uso: scripts/gen_auditoria.sh <tabla> <col1> <col2> ...   (la PK debe llamarse id)
# Si ALTERas una tabla que ya tiene triggers de auditoría, regénerelos con TODAS sus columnas.
set -euo pipefail
t="${1:?tabla}"; shift
[ $# -ge 1 ] || { echo "faltan columnas" >&2; exit 2; }
json() { local pref="$1" s="" c; shift; for c in "$@"; do s+="'$c', $pref.$c, "; done; printf 'JSON_OBJECT(%s)' "${s%, }"; }
nuevo="$(json NEW "$@")"; viejo="$(json OLD "$@")"
for a in ins:INSERT upd:UPDATE del:DELETE; do
  k="${a%%:*}"; ev="${a##*:}"
  case "$k" in
    ins) reg=NEW; acc=insert; ant=NULL;     nvo="$nuevo" ;;
    upd) reg=NEW; acc=update; ant="$viejo"; nvo="$nuevo" ;;
    del) reg=OLD; acc=delete; ant="$viejo"; nvo=NULL ;;
  esac
  cat <<SQL
DROP TRIGGER IF EXISTS trg_aud_${t}_${k};
CREATE TRIGGER trg_aud_${t}_${k} AFTER ${ev} ON ${t}
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('${t}', ${reg}.id, @auditoria_usuario_id, '${acc}', ${ant}, ${nvo});
END;

SQL
done
