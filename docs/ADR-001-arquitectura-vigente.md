# ADR-001 — Arquitectura vigente

**Proyecto:** Planillas CCDR Liberia (backend de planillas del sector público de Costa Rica).
**Estado:** vigente. Reemplaza a `ADR-000_decisiones_arquitectura.md`, `ORGANIZACION.md` y `CICD.md`.
**Stack:** Go 1.25.11, Gin, MySQL 8, `shopspring/decimal`, argon2id, PASETO v4.local (`aidanwoods.dev/go-paseto`). Módulo `planillas-ccdr`.

> Un error de cálculo o de seguridad en este sistema tiene consecuencias legales. Las decisiones de
> la sección 2 son vinculantes: se aplican, no se discuten caso por caso.

---

## 1. Estructura de carpetas

Todo el código vive bajo `internal/`, salvo `cmd/`. Así ningún módulo externo puede importar
paquetes internos por error.

```
planillas-ccdr/
├── cmd/api/main.go               # entrypoint: config, base de datos, router, wiring
├── db/
│   ├── migrations/               # migraciones golang-migrate (fuente de verdad del esquema)
│   └── schema.sql                # heredado; NO es fuente de verdad (ver sección 4)
├── internal/
│   ├── api/
│   │   ├── handlers/             # handlers Gin (auth, empleados, contratos, organización,
│   │   │                         #   liquidación, dashboard, contabilidad, exportación) y routes.go
│   │   └── middleware/           # auth PASETO, RequirePermission, contexto, contrato único de error
│   ├── asistencia/               # turnos, calendario, asistencia, vacaciones, incapacidades
│   ├── auth/                     # login, refresh, logout, roles, reset de contraseña
│   ├── config/                   # configuración (viper) desde variables de entorno o .env
│   ├── contabilidad/             # asientos contables y mapeo concepto-cuenta
│   ├── db/
│   │   ├── audit.go              # WithAuditContext: envoltorio transaccional de toda escritura
│   │   └── dto/                  # acceso a datos escrito a mano (database/sql)
│   ├── pagination/               # parseo de ?pagina=&tamano=
│   ├── planilla/                 # motor de cálculo, períodos, acumulados, exportación
│   ├── prestamos/                # préstamos, embargos y terceros
│   ├── security/                 # hashing de contraseñas y tokens PASETO
│   ├── services/                 # empleados, contratos, organización, dashboard, liquidación
│   └── shared/decimal/           # utilidades sobre shopspring/decimal
├── scripts/                      # utilidades de desarrollo (rehacer BD dev, verificaciones)
├── docs/
│   ├── ADR-001-arquitectura-vigente.md
│   └── fases/                    # un informe de traspaso por fase (FASE-XX.md)
├── .env.example                  # plantilla de variables de entorno
├── run_full_test_suite.sh        # verificación completa local y de CI
└── go.mod
```

---

## 2. Decisiones vinculantes (D1–D12)

| # | Decisión |
|---|---|
| D1 | Todos los IDs son `int64` en Go. La conversión desde/hacia texto ocurre solo en el borde HTTP. |
| D2 | Acceso a datos en `internal/db/dto`, escrito a mano. Hasta 3 argumentos: posicionales. Desde 4: un struct `XxxParams`. Sin tipos `Row` duplicados. |
| D3 | Porcentajes: en BD son porcentaje (10.83), salvo `parametros_salario_escolar` (fracción 0.0833). El dominio Go usa siempre **fracción** (0.1083). La conversión se hace en un único punto: el cargador de parámetros. Nadie más multiplica ni divide por 100. |
| D4 | Estados del período (ENUM real): `borrador`, `calculada`, `aprobada`, `pagada`, `contabilizada`. Constantes en `internal/planilla/estados.go`. Dispersión: `aprobada` o `pagada`. Aguinaldo, salario escolar y liquidación suman períodos `aprobada`, `pagada` o `contabilizada`. Dashboard "último cerrado": `pagada` o `contabilizada`. |
| D5 | Fuente única del salario: `contratos.salario_pactado` del contrato vigente. |
| D6 | Cargas sociales en modelo agregado (`CCSS_OBR`, `CCSS_PAT`). Prohibido capturar a mano `INS_RT`, `BANPOP_PAT` y `FCL_PAT` (doble conteo). |
| D7 | Toda escritura pasa por `db.WithAuditContext(ctx, sqlDB, usuarioID int64, func(ctx context.Context, q *dto.Queries) error)`. Los servicios reciben `*sql.DB`, no una función inyectada. |
| D8 | Cero hardcodeo legal: los valores legales viven en BD con vigencia (cerrar la vigente e insertar la nueva). Si falta un parámetro, error explícito; nunca un valor por defecto silencioso. |
| D9 | Errores de dominio tipados. Al cliente nunca se le devuelve `err.Error()` de errores internos. |
| D10 | Migraciones en `db/migrations/` con golang-migrate. Un archivo propio por fase, con número reservado `0001NN` (NN = número de la fase), con su `.up.sql` y su `.down.sql`. |
| D11 | Comentarios, mensajes y errores en español. |
| D12 | Régimen laboral, % de salario escolar, rubros patronales, base mínima y créditos fiscales están parametrizados. Su validación legal es humana y no bloquea. |

---

## 3. Reglas de diseño que se derivan

### 3.1 Patrón de vigencia
Las tablas de parámetros y datos versionados usan `fecha_inicio_vigencia`, `fecha_fin_vigencia` y
`es_vigente`, con un `UNIQUE KEY ..._una_vigente` como última defensa contra dos filas vigentes.
La lista exacta de tablas se consulta en las migraciones, no se copia aquí.

**Toda escritura sobre estas tablas es una sola transacción de dos pasos** (D7 y D8):
1. Cerrar la fila vigente: `UPDATE ... SET fecha_fin_vigencia = <fecha_efectiva> WHERE <clave> = ? AND es_vigente = 1`.
2. Insertar la nueva fila vigente con `fecha_fin_vigencia = NULL`.

No se confía solo en el índice único para la consistencia. Excepción: `embargos` tiene fechas de
vigencia pero **no** usa `es_vigente`; su estado vigente lo controla la columna `estado` (ENUM).

### 3.2 Autorización por permiso, no por rol
- `permisos` es el catálogo atómico (`codigo` único; `modulo` solo agrupa para menús).
- `rol_permisos` asigna permisos a roles. Es comodidad administrativa, no lo que se consulta al autorizar.
- `usuario_roles` es versionada y admite varios roles vigentes por usuario.
- El middleware `RequirePermission` autoriza contra el conjunto de `permisos.codigo` vigentes del
  usuario (`usuarios` → `usuario_roles` vigentes → `roles` activos → `rol_permisos` → `permisos`),
  nunca contra `roles.codigo`.
- Los permisos viajan dentro del access token. La revocación efectiva se implementa con la versión
  `pv` del usuario: `AuthPASETO` compara el valor firmado con el de la BD mediante una caché corta.
  El detalle está en la decisión F15 de la sección 3.7.

### 3.3 Dinero y cantidades
Toda columna `DECIMAL` se mapea a `decimal.Decimal`, nunca a `float64` ni a `string` suelto. Incluye
las no monetarias (días, horas) por las fracciones de 0.5 días o 0.25 horas.

### 3.4 Códigos estables
Los códigos de `conceptos_salariales.codigo` que el motor referencia por texto (`CCSS_OBR`,
`CCSS_PAT`, etc.) se siembran por migración de datos, no se asumen ni se escriben en Go. La columna
`rubro` es solo una etiqueta legible: no se usa para lógica.

### 3.5 Contrato único de error HTTP
Los handlers responden errores mediante `internal/api/middleware` (`ErrorResponse`, `Fail`,
`FailFromError`), que traduce los errores de dominio tipados (D9) a código HTTP. Ningún paquete
define su propio formato de error (`gin.H{"error": ...}` ad hoc).

### 3.6 Seguridad
- Contraseñas con argon2id. Tokens PASETO v4.local con claves aleatorias de 32 bytes en hex
  (64 caracteres), cargadas desde `PASETO_LOCAL_KEYS_HEX` y `PASETO_ACTIVE_KID`. Se acepta
  `PASETO_LOCAL_KEY_HEX` como configuración legada de una sola clave (kid `k1`).
- Sesiones persistidas y revocables (`refresh_tokens`), bloqueo por intentos fallidos y reset de
  contraseña de un solo uso. La política (umbral y minutos de bloqueo) va en configuración.
- `config.LoadConfig` rechaza claves ausentes, mal formadas o de ejemplo, y en producción exige
  `DB_PASSWORD`.

### 3.7 Tokens PASETO y revocación efectiva (F15)

- **Formato y biblioteca:** PASETO v4.local mediante `aidanwoods.dev/go-paseto`. Se reemplazó
  `o1egl/paseto` v2.local; la aplicación ya no depende de `github.com/aead/chacha20poly1305`.
- **Claims obligatorios:** `iss` (emisor), `aud` (audiencia), `iat`, `nbf`, `exp`, `jti`, `tipo`,
  `uid`, `usr`, `rcp` y `pv`; `perm` se incluye solo en access tokens. Todos se validan al leer
  el token. El footer contiene `{"kid":"..."}` para identificar la clave. Se toleran 5 segundos
  de diferencia de reloj para `iat` y `nbf`.
- **Rotación de claves:** `PASETO_LOCAL_KEYS_HEX="kid1:hex,kid2:hex"` contiene el llavero y
  `PASETO_ACTIVE_KID` el identificador usado al firmar. Cada clave debe generarse con
  `openssl rand -hex 32`. Para rotar: agregar `k2` junto a `k1` y desplegar; activar `k2`;
  retirar `k1` tras la vigencia máxima de refresh tokens (7 días por defecto). La variable
  legada `PASETO_LOCAL_KEY_HEX` equivale a una sola clave `k1`.
- **Invalidación de sesiones:** `usuarios.permisos_version` (migración `000115`) aumenta dentro
  de la misma transacción que asigna/revoca roles, cambia o restablece contraseñas, cambia el
  estado de usuario o modifica/desactiva un rol. Desactivar una cuenta también revoca sus refresh
  tokens. Los cambios de `rol_permisos` y de estado de rol incrementan `pv` para sus usuarios.
- **Administración de accesos:** `PATCH /usuarios/:id/estado` requiere `usuarios.actualizar`;
  `PATCH /roles/:id/estado` y `PUT /roles/:id/permisos` requieren el permiso separado
  `roles.administrar` (migración `000116`). El último reemplaza el conjunto completo y recibe
  `{"codigos":["empleados.leer"]}`; una lista vacía explícita elimina todos los permisos.
- **Verificación en cada sesión:** `AuthPASETO` consulta si el usuario sigue activo y su `pv`
  mediante `SesionVerificador`, con caché por instancia controlada por `AUTH_CACHE_TTL` (5 s por
  defecto) y singleflight. Usuario inactivo o `pv` diferente responde `401` con código
  `sesion_invalida`; el cliente debe llamar `/auth/refresh` y reintentar la solicitud una vez.
  Si no se puede verificar contra la BD, se responde `503 sesion_no_verificable` (falla cerrado).
- **Consecuencia del despliegue:** tokens v2 anteriores (access y refresh) dejan de ser válidos;
  todos los usuarios deberán iniciar sesión nuevamente. Los refresh tokens nuevos se emiten con
  permisos y `pv` leídos de la BD.

---

## 4. Base de datos y migraciones

- Las migraciones de `db/migrations/` (`NNNNNN_nombre.up.sql` y `.down.sql`, con punto y no guion
  bajo) son la **única fuente de verdad** del esquema. `db/schema.sql` no lo es y está pendiente de
  retiro.
- Migraciones existentes: `000000` a `000006` y `000107`, `000109`, `000112`, `000114`, `000115`
  y `000116`. La `000115` agrega `usuarios.permisos_version`; la `000116` siembra el permiso
  `roles.administrar`. Las nuevas usan el número reservado `0001NN` de su fase (D10).
- Toda migración lleva su `.down.sql`. Advertencia heredada: el reverso de `usuario_roles` colapsa el
  histórico a las filas vigentes, así que no debe usarse en producción con historial real que conservar.
- Antes de cualquier cambio de esquema en producción: respaldo de la BD y reversión ensayada en
  staging.

---

## 5. Verificación y CI

Un solo comando, desde la raíz: `./run_full_test_suite.sh`. Usa la versión de Go declarada en
`go.mod` y corre, en orden,
`gofmt -l` (falla si lista archivos), `go vet ./...`, `go build ./...` y `go test ./... -count=1`.
Las pruebas de integración se activan con `RUN_INTEGRATION=1` cuando exista `test/integration/`.

Ejemplo de pipeline (GitHub Actions):

```yaml
# .github/workflows/ci.yml
name: CI
on:
  pull_request:
  push:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    services:
      mysql:
        image: mysql:8.0
        env:
          MYSQL_ROOT_PASSWORD: root
          MYSQL_DATABASE: planillas_ccdr_test
        ports: ["3306:3306"]
        options: >-
          --health-cmd="mysqladmin ping" --health-interval=5s
          --health-timeout=3s --health-retries=10
    env:
      DB_HOST: 127.0.0.1
      DB_PORT: 3306
      DB_USER: root
      DB_PASSWORD: root
      DB_NAME: planillas_ccdr_test
      PASETO_LOCAL_KEY_HEX: ${{ secrets.CI_PASETO_TEST_KEY }} # 64 caracteres hex
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.25.11"
      - name: Instalar golang-migrate
        run: go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
      - name: Aplicar migraciones
        run: >
          migrate -path db/migrations
          -database "mysql://root:root@tcp(127.0.0.1:3306)/planillas_ccdr_test" up
      - name: Suite completa
        run: ./run_full_test_suite.sh
```

---

## 6. Checklist antes de producción

- [ ] `./run_full_test_suite.sh` pasa en verde.
- [ ] Migraciones aplicadas y verificadas en staging con datos representativos, incluida `000115`
      (`usuarios.permisos_version`), no solo en una BD vacía.
- [ ] Variables de producción presentes y validadas por `config.LoadConfig`. En particular
      `PASETO_LOCAL_KEYS_HEX` + `PASETO_ACTIVE_KID` (claves de 64 hex, distintas a las de staging
      y CI; o la variable legada `PASETO_LOCAL_KEY_HEX`) y las `DB_*`.
- [ ] Comunicado el cierre de sesión: tras el despliegue de v4.local todos deberán iniciar sesión
      nuevamente. El frontend debe tratar `401 sesion_invalida` intentando `/auth/refresh` y
      reintentando la solicitud, no enviando inmediatamente al usuario al login.
- [ ] `GET /health` responde 200 contra la BD de producción antes de registrar la instancia en el
      balanceador.
- [ ] Permiso `dashboard.ver` sembrado en `permisos` y `rol_permisos` para los roles gerenciales; sin
      él, `GET /api/v1/dashboard/metricas` responde 403.
- [ ] Confirmados contra el esquema real los tres supuestos de las consultas del dashboard (costo
      patronal, tabla de solicitudes de vacaciones, tabla de incapacidades). No desplegar con
      supuestos sin confirmar.
- [ ] Respaldo de la BD previo a migrar y plan de reversión ensayado en staging.
- [ ] Logs y métricas de la ruta del dashboard visibles en observabilidad (latencia, tasa de error)
      antes de anunciar la función a usuarios finales.

---

## 7. Pendientes conocidos de arquitectura

- `internal/asistencia` e `internal/prestamos` aún no usan el contrato único de error (3.5).
- `db/schema.sql` y `scripts/split_schema_a_migraciones.sh` son restos de la migración al esquema
  dividido; decidir su retiro.