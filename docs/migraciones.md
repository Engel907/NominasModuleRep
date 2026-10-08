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
| F18  | `000118`          |

Los huecos son válidos (golang-migrate solo exige orden creciente). Si una fase no necesita cambios de BD, su número queda sin usar.

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