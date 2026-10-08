#!/usr/bin/env bash
# Busca llamadas JSON_OBJECT(...) con un número IMPAR de argumentos (deben ser pares clave,valor).
# MySQL solo lo detecta al DISPARARSE el trigger (error 1582), no al crearlo; por eso 204 triggers
# "creados" no prueban que todos funcionen.
# Uso (desde la raíz del proyecto):  bash scripts/verificar_json_object.sh [archivos.sql ...]
# Por defecto revisa db/migrations/*.up.sql. Sale con código 1 si encuentra alguno.
set -uo pipefail

if [ "$#" -gt 0 ]; then files=("$@"); else files=(db/migrations/*.up.sql); fi

bad=0
for f in "${files[@]}"; do
  [ -f "$f" ] || continue
  out=$(tr -d '\r' < "$f" | awk -v F="$(basename "$f")" '
    BEGIN { RS = "\001" }
    {
      txt = $0; n = length(txt)

      # posición y nombre de cada CREATE TRIGGER
      nt = 0; p = 1
      while ((i = index(substr(txt, p), "CREATE TRIGGER ")) > 0) {
        abs = p + i - 1
        rest = substr(txt, abs + 15, 80)
        match(rest, /^[A-Za-z_0-9]+/)
        nt++; tpos[nt] = abs; tname[nt] = substr(rest, 1, RLENGTH)
        p = abs + 15
      }

      checked = 0; p = 1
      while ((i = index(substr(txt, p), "JSON_OBJECT(")) > 0) {
        abs = p + i - 1
        j = abs + 12
        depth = 1; inq = 0; commas = 0; nonempty = 0
        while (j <= n && depth > 0) {
          c = substr(txt, j, 1)
          if (inq) {
            if (c == "\\") { j++ }
            else if (c == "\047") {
              if (substr(txt, j + 1, 1) == "\047") j++
              else inq = 0
            }
          } else {
            if (c == "\047") { inq = 1; nonempty = 1 }
            else if (c == "(") { depth++; nonempty = 1 }
            else if (c == ")") { depth-- }
            else if (c == "," && depth == 1) { commas++ }
            else if (c != " " && c != "\t" && c != "\n") { nonempty = 1 }
          }
          j++
        }
        args = nonempty ? commas + 1 : 0
        checked++
        if (args % 2 == 1) {
          name = "(fuera de un trigger)"
          for (k = 1; k <= nt; k++) if (tpos[k] < abs) name = tname[k]
          tmp = substr(txt, 1, abs); ln = gsub(/\n/, "\n", tmp) + 1
          printf "IMPAR: %s:%d  trigger=%s  argumentos=%d\n", F, ln, name, args
        }
        p = abs + 12
      }
      printf "OK_COUNT %s %d\n", F, checked
    }')
  echo "$out" | grep '^IMPAR' && bad=1
  echo "$out" | grep '^OK_COUNT' | awk '{ printf "revisados %d JSON_OBJECT en %s\n", $3, $2 }'
done

if [ "$bad" -eq 1 ]; then
  echo "RESULTADO: hay JSON_OBJECT con argumentos impares (arriba). Esos triggers fallarán al dispararse."
  exit 1
fi
echo "RESULTADO: todos los JSON_OBJECT tienen argumentos pares."