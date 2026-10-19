# Migraciones de base de datos

Las migraciones viven en `db/migrations/` y se aplican con
[golang-migrate](https://github.com/golang-migrate/migrate) mediante `cmd/migrate`.
La API **nunca** migra al arrancar: se ejecuta a mano (o en el paso de despliegue/CI).

## Convención de nombres

```
NNNNNN_descripcion_en_snake_case.up.sql
NNNNNN_descripcion_en_snake_case.down.sql
```

- Seis dígitos, versión estrictamente creciente. Cada migración tiene **siempre** su `.up.sql` y su `.down.sql`.
- Una migración = una unidad lógica (DDL de una fase, o un cambio de datos). No mezclar DDL con datos de negocio en el mismo archivo.
- Idempotencia razonable en el `up` (`IF NOT EXISTS`), pero **nunca** `DROP TABLE` sobre tablas existentes ni `ON DUPLICATE KEY UPDATE` que pise valores (ver más abajo).
- Migraciones existentes: `000000`–`000006` (base partida desde el antiguo `db/schema.sql`). No se editan una vez aplicadas en un ambiente compartido: se corrige con una migración nueva.

## Regla D10: número reservado por fase

Cada fase tiene un archivo propio con número reservado `0001NN`, donde `NN` es el número de la fase:

| Fase | Versión reservada |
|------|-------------------|
| F02  | `000102`          |
| F12  | `000112`          |
| F17  | `000117`          |
| F18  | `000118`          |

Los huecos son válidos (golang-migrate solo exige orden creciente). Si una fase no necesita cambios de BD, su número queda sin usar.

`000117_rol_empleado` agrega el rol `EMPLEADO` con `planilla.ver_boleta_propia`
y `vacaciones.solicitar`; no vuelve a crear permisos ya sembrados por F12.

**Cambios anuales de parámetros** (renta, salario mínimo, etc.) no pertenecen a una fase y D10 no los cubre. Propuesta, pendiente de confirmar: `AAAANN_...` (año + secuencia, p. ej. `202701_parametros_renta_2027`). Siempre es mayor que `0001NN`, así que el orden se conserva.

## Cambio anual de parámetros

Regla D8: los valores legales viven en BD con vigencia. Un cambio de parámetros es una **nueva migración de datos** que:

1. **Cierra** la vigencia del registro anterior (`fecha_fin_vigencia`).
2. **Inserta** el registro nuevo con `fecha_inicio_vigencia`.

Nunca `ON DUPLICATE KEY UPDATE` ni `UPDATE` de los montos: pisaría el valor sin dejar historial y alteraría recálculos de períodos pasados.

El orden importa: en las tablas con `es_vigente` generado y `UNIQUE (es_vigente)`, hay que cerrar primero y luego insertar. Esquema del `up` (los nombres de tabla/columna son ilustrativos; use los reales de la tabla):

```sql
-- up: renta 2027
UPDATE parametros_xxx
   SET fecha_fin_vigencia = '2026-12-31'
 WHERE fecha_fin_vigencia IS NULL
   AND <clave natural del registro>;

INSERT INTO parametros_xxx (<columnas>, fecha_inicio_vigencia)
VALUES (<valores 2027>, '2027-01-01');
```

```sql
-- down: revierte por clave natural + fecha
DELETE FROM parametros_xxx
 WHERE <clave natural> AND fecha_inicio_vigencia = '2027-01-01';

UPDATE parametros_xxx
   SET fecha_fin_vigencia = NULL
 WHERE <clave natural> AND fecha_fin_vigencia = '2026-12-31';
```

Recuerde: los porcentajes en BD son porcentaje (10.83), salvo `parametros_salario_escolar` (fracción 0.0833) — D3.

## Cargar `SALARIO_MINIMO_INEMBARGABLE` (L4)

Ninguna migración siembra este parámetro: su monto y su base legal los confirma
quien administra los parámetros (D13 del ADR-001: monto global). Hasta que exista
una fila vigente, `CargadorParametros.Cargar` falla con `ErrParametroFaltante` y
`go run ./cmd/jobs verificar-parametros` sale con código 1. Se carga como
migración de datos (`AAAANN_parametro_salario_minimo_inembargable`):

```sql
-- up
-- 1) Cierra la vigencia anterior, si existe (en la primera carga no afecta filas).
UPDATE parametros_generales
   SET fecha_fin_vigencia = DATE_SUB('<FECHA_INICIO>', INTERVAL 1 DAY)
 WHERE clave = 'SALARIO_MINIMO_INEMBARGABLE'
   AND fecha_fin_vigencia IS NULL;

-- 2) Inserta el valor nuevo. fuente_legal es OBLIGATORIA: la norma que fija el monto.
INSERT INTO parametros_generales
    (clave, valor, tipo, descripcion, fuente_legal, fecha_inicio_vigencia, fecha_fin_vigencia, creado_por)
SELECT v.clave, v.valor, 'decimal',
       'Salario mínimo inembargable (monto global) para el cálculo de embargos',
       v.fuente_legal, '<FECHA_INICIO>', NULL, 1
FROM (SELECT 'SALARIO_MINIMO_INEMBARGABLE' AS clave,
             '<MONTO>' AS valor,
             '<FUENTE_LEGAL>' AS fuente_legal) v
WHERE NOT EXISTS (
    SELECT 1 FROM parametros_generales g
     WHERE g.clave = v.clave AND g.es_vigente = 1
);
```

```sql
-- down: no-op intencional.
-- parametros_generales es historial legal inmutable desde L2:
-- no se borran filas y una vigencia cerrada no se reabre.
```

Reglas: sin valor inventado ni por defecto; `<MONTO>`, `<FECHA_INICIO>` y
`<FUENTE_LEGAL>` los aporta quien valida el parámetro. Desde L2
(`trg_pg_vigencia_*`), las filas no se pueden borrar ni una vigencia cerrada
reabrir. Por eso la reversa de una
migración de parámetros generales debe ser no-op; para corregir una carga se
agrega una nueva migración que cierra la vigente e inserta la fila correcta.
No copiar para esta tabla el `DELETE` ni la reapertura del ejemplo genérico de
arriba. Pruebe el `up` y confirme el `down` no-op en una BD efímera antes de
aplicarla.

## Requisitos de conexión

- La URL debe llevar `multiStatements=true`; `cmd/migrate` lo fuerza. Los scripts ya no usan `DELIMITER`: el servidor entiende `CREATE TRIGGER ... BEGIN ... END;` dentro de multi-statement.
- El usuario que migra necesita `CREATE`, `ALTER`, `DROP`, `INDEX`, `REFERENCES`, `TRIGGER`, `INSERT`, `UPDATE`, `DELETE`. Con el binlog activo (por defecto en MySQL 8) crear triggers sin `SUPER` falla con el error 1419, salvo que el servidor tenga `log_bin_trust_function_creators=1`. `docker-compose.yml` ya lo configura para desarrollo; en otros ambientes debe resolverlo el DBA.
- Variables: `DATABASE_URL` o `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_PARAMS` (mismos nombres que `internal/config`). También se lee `.env`.

## Comandos

```bash
go run ./cmd/migrate up          # aplica pendientes
go run ./cmd/migrate down 1      # revierte 1 (N obligatorio)
go run ./cmd/migrate version     # versión actual / dirty
go run ./cmd/migrate force V     # solo tras corregir a mano un estado dirty
```

Atajos de desarrollo (`Makefile`): `make db-up`, `make migrate-up`, `make migrate-down N=2`, `make migrate-version`, `make db-reset` (destruye el volumen local; se niega con `APP_ENV=production|staging`).

Si una migración falla a medias, la versión queda **dirty**: revise qué sentencias alcanzaron a aplicarse, deshaga a mano lo parcial, y recién entonces `force` a la versión anterior y reintente. MySQL no revierte DDL de forma transaccional.

## Cómo probar up / down / up en local

```bash
make db-reset                              # BD limpia + up completo
go run ./cmd/migrate down 7                # baja las 7 migraciones 000000-000006
go run ./cmd/migrate version               # debe decir: ninguna migración aplicada
go run ./cmd/migrate up                    # sube de nuevo
```

Al terminar el `down` la BD debe quedar sin tablas (salvo `schema_migrations`). Comparar conteos tras el segundo `up` con los del primero: 84 tablas y 204 triggers de auditoría (`SELECT COUNT(*) FROM information_schema.triggers WHERE trigger_schema = DATABASE();`).

Nota: las migraciones de solo datos (`000002`, `000005`, `000006`) tienen `down` no-op documentado; el `down` de `000000` elimina todo de todos modos.

## Advertencias

- `000006_rol_admin_y_permisos_base` siembra el usuario `admin` con una contraseña temporal **conocida**. No aplicar en ambientes compartidos ni producción; F12 la reemplaza por `cmd/seedadmin`.
- Migraciones que crean triggers de auditoría dependen de `@auditoria_usuario_id`; en una migración de datos queda `NULL`, como en las semillas actuales.

## Archivo legado

`db/schema.sql` se movió a `db/legacy/schema.sql.OLD` solo como referencia. **Se elimina al cerrar F13.** No debe ejecutarse: incluye `DROP TABLE IF EXISTS` y una contraseña de admin.