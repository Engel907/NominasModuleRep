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
- Una migración = una unidad lógica (un cambio de esquema, con los datos que ese cambio necesita).
- Idempotencia razonable en el `up` (`IF NOT EXISTS`), pero **nunca** `DROP TABLE` sobre tablas existentes ni `ON DUPLICATE KEY UPDATE` que pise valores (ver más abajo).
- Migraciones existentes: `000000`–`000156`, con huecos. No se editan una vez aplicadas en un ambiente compartido: se corrige con una migración nueva (p. ej. `000149` corrige valores sembrados, cerrando y reponiendo el trigger de inmutabilidad en la misma migración).

## Numeración

Cada migración usa el siguiente número libre de seis dígitos. Los huecos son
válidos: golang-migrate solo exige orden creciente.

## Cambios de parámetros

Los valores legales y de negocio (renta, cargas sociales, salario mínimo
inembargable, escalas, conceptos, feriados) **no se cambian con migraciones**:
se proponen y aprueban por la API (`POST /api/v1/parametros/propuestas`), que
cierra la vigencia anterior, inserta la nueva y lo audita. Ver
[operacion.md](operacion.md#subsistema-de-configuración-todo-desde-la-api).

Una migración solo cambia el esquema o siembra catálogos iniciales. Las tablas de
parámetros son inmutables por triggers: no admiten `DELETE` ni reabrir una
vigencia cerrada, así que la reversa de una migración que siembre parámetros es
un no-op.

## Antes de aplicar 000153

`000153` elimina `contratos.tipo_contrato` y `contratos.jornada` (texto) y deja
`tipo_contrato_id` y `jornada_id` obligatorios. Si algún contrato no tiene esos ids (su
texto no coincidía con el catálogo), la primera sentencia falla y nada se modifica.
Complételos antes de migrar:

```sql
SELECT id, empleado_id, tipo_contrato, jornada FROM contratos
 WHERE tipo_contrato_id IS NULL OR jornada_id IS NULL;
```

`000154` agrega `contratos.modalidad_salario`; los contratos existentes quedan `mensual`.
Su `down` se niega (CHECK) si hay contratos diarios o por hora.

## Requisitos de conexión

- La URL debe llevar `multiStatements=true`; `cmd/migrate` lo fuerza. Los scripts no usan `DELIMITER`: el servidor entiende `CREATE TRIGGER ... BEGIN ... END;` dentro de multi-statement.
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
go run ./cmd/migrate down "$(ls db/migrations/*.up.sql | wc -l)"   # baja todas
go run ./cmd/migrate version               # debe decir: ninguna migración aplicada
go run ./cmd/migrate up                    # sube de nuevo
```

Al terminar el `down` la BD debe quedar sin tablas (salvo `schema_migrations`). Tras el segundo `up`, los conteos de tablas, vistas y triggers (`information_schema.tables`, `.views`, `.triggers`) deben ser los del primero. `TestMigracionesDownUp` hace lo mismo con las últimas seis migraciones.

Nota: las migraciones de solo datos (`000002`, `000005`, `000006`) tienen `down` no-op; el `down` de `000000` elimina todo de todos modos. Los triggers de auditoría e inmutabilidad se generan con `scripts/gen_auditoria.sh` y `scripts/gen_inmutabilidad_parametros.sh`: al alterar una tabla, regenérelos con todas sus columnas.

## Advertencias

- Ninguna migración crea usuarios: el administrador inicial se crea con `cmd/seedadmin` (contraseña aleatoria mostrada una sola vez y cambio obligatorio).
- Migraciones que crean triggers de auditoría dependen de `@auditoria_usuario_id`; en una migración de datos queda `NULL`, como en las semillas actuales.
