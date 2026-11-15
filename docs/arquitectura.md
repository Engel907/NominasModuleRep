# Arquitectura

**Proyecto:** backend de planillas y RR. HH. para empleadores de Costa Rica (públicos, privados, asociaciones y organizaciones comunales). Todo lo que distingue a una organización de otra se configura en la base de datos.
**Stack:** Go 1.26, Gin, MySQL 8, `shopspring/decimal`, argon2id, PASETO v4.local (`aidanwoods.dev/go-paseto`). Módulo `planillas-ccdr`.

> Un error de cálculo o de seguridad en este sistema tiene consecuencias legales. Las decisiones de
> la sección 2 se aplican siempre; cambiarlas exige actualizar este documento.

---

## 1. Estructura de carpetas

Todo el código vive bajo `internal/`, salvo los ejecutables de `cmd/`. Así ningún módulo externo
puede importar paquetes internos.

```
planillas-ccdr/
├── cmd/
│   ├── api/                      # servidor HTTP
│   ├── jobs/                     # procesos batch (vacaciones, salidas, cifrado, verificación)
│   ├── migrate/                  # aplica y revierte migraciones
│   └── seedadmin/                # crea el administrador inicial
├── db/migrations/                # migraciones golang-migrate (fuente de verdad del esquema)
├── internal/
│   ├── api/
│   │   ├── handlers/             # handlers Gin y registro de rutas
│   │   └── middleware/           # autenticación, permisos, errores, cabeceras, límites
│   ├── app/                      # construcción del router y de las dependencias
│   ├── asistencia/               # turnos, calendario, asistencia, horas extra, vacaciones
│   ├── auth/                     # login, sesiones, roles, administración de usuarios
│   ├── authz/                    # códigos de permiso
│   ├── catalogos/                # configuración de catálogos de la planilla
│   ├── config/                   # configuración desde variables de entorno
│   ├── contabilidad/             # asientos contables y mapeo concepto-cuenta
│   ├── credenciales/             # hash de contraseñas y tokens PASETO
│   ├── cripto/                   # cifrado de datos personales (Ley 8968)
│   ├── db/                       # conexión y transacción auditada
│   │   └── dto/                  # acceso a datos (database/sql)
│   ├── e2e/                      # prueba de punta a punta del cierre de mes
│   ├── liquidacion/              # cálculo puro de la liquidación laboral
│   ├── observabilidad/           # métricas Prometheus
│   ├── paginacion/               # parámetros ?pagina=&tamano=
│   ├── parametros/               # parámetros legales versionados y propuestas
│   ├── planilla/                 # motor de cálculo, períodos, reportes oficiales y de gestión, exportación
│   │   └── infra/                # repositorios del motor sobre db/dto
│   ├── prestamos/                # préstamos, embargos y terceros
│   ├── rrhh/                     # empleados, contratos, conceptos asignados, histórico de salarios, organización, liquidaciones, dashboard
│   ├── seedadmin/                # alta del administrador inicial
│   ├── shared/porcentaje/        # conversión porcentaje ↔ fracción
│   └── testutil/                 # utilidades de prueba
├── scripts/                      # suite completa, generadores de triggers, respaldos
├── docs/                         # arquitectura, migraciones, operación, pruebas
├── .github/workflows/            # CI
├── Dockerfile, docker-compose.yml, Makefile
└── .env.example                  # plantilla de variables de entorno
```

---

## 2. Decisiones de diseño

- **Identificadores.** Todos los IDs son `int64` en Go. La conversión desde/hacia texto ocurre solo en el borde HTTP.
- **Acceso a datos.** En `internal/db/dto`, escrito a mano. Hasta 3 argumentos: posicionales. Desde 4: un struct `XxxParams`. Sin tipos `Row` duplicados.
- **Porcentajes.** En BD son porcentaje (10.83), salvo `parametros_salario_escolar` (fracción 0.0833). El dominio Go usa siempre **fracción** (0.1083). La conversión se hace al cargar (cargador de parámetros e hidratador). Nadie más multiplica ni divide por 100.
- **Estados del período** (ENUM real): `borrador`, `calculada`, `aprobada`, `pagada`, `contabilizada`. Constantes en `internal/planilla/estados.go`. Dispersión: `aprobada` o `pagada`. Aguinaldo, salario escolar, liquidación, renta acumulada del mes y reportes suman períodos `aprobada`, `pagada` o `contabilizada`.
- **Fuente única del salario.** `contratos.salario_pactado` del contrato vigente, en la unidad de `contratos.modalidad_salario` (`mensual`, `quincenal` o `semanal`); `empleado_datos_laborales.salario_base_nominal` es una copia que mantiene un trigger.
- **Contratos sin texto duplicado.** El tipo de contrato y la jornada se guardan solo como `tipo_contrato_id` y `jornada_id` (FK a sus catálogos); la API devuelve además sus códigos, leídos del catálogo.
- **Regla general y regla de un empleado.** La fórmula de cada concepto (`concepto_salarial_formula`) y los parámetros generales (`parametros_generales`) rigen para todos; los factores de hora extra (`parametros_horas_extra`) y los divisores de jornada (`parametros_jornada`) también; `empleado_concepto_formula`, `empleado_parametros_generales`, `empleado_parametros_horas_extra` y `empleado_parametros_jornada` los reemplazan solo para un empleado. Todas tienen vigencia y se cambian por propuesta con aprobación.
- **Retroactivos.** Un período regular paga la diferencia de períodos ya aprobados recalculándolos con los datos de hoy (`retroactivos`); la diferencia va como líneas de sus conceptos ligadas al período de origen (`planilla_detalle_concepto.retroactivo_periodo_origen_id`), así que nunca se paga dos veces.
- **Cargas sociales por concepto.** Cada concepto con fórmula `carga_social` toma su tasa de `parametros_cargas_sociales` (fila general o fila de su régimen CCSS). Se puede trabajar con un concepto obrero y uno patronal agregados (como se siembra) o desglosar componentes (SEM, IVM, Banco Popular, INA, riesgos del trabajo…); nunca las dos cosas a la vez, porque se contaría doble. `BANPOP_PAT` y `FCL_PAT` se siembran inactivos porque están dentro de `CCSS_PAT`.
- **Auditoría.** Toda escritura pasa por `db.WithAuditContext(ctx, sqlDB, usuarioID int64, func(ctx context.Context, q *dto.Queries) error)`. Los servicios reciben `*sql.DB`, no una función inyectada.
- **Cero hardcodeo legal.** Los valores legales y las reglas de negocio viven en BD con vigencia (cerrar la vigente e insertar la nueva). Si falta un parámetro, error explícito; nunca un valor por defecto silencioso. Una regla con más de una interpretación válida es un parámetro sin valor sembrado.
- **Errores.** De dominio y tipados. Al cliente nunca se le devuelve `err.Error()` de errores internos.
- **Migraciones.** En `db/migrations/` con golang-migrate. Numeración creciente de seis dígitos, cada una con su `.up.sql` y su `.down.sql`.
- **Idioma.** Comentarios, mensajes y errores en español.

---

## 3. Reglas de diseño que se derivan

### 3.1 Patrón de vigencia
Las tablas de parámetros y datos versionados usan `fecha_inicio_vigencia`, `fecha_fin_vigencia` y
`es_vigente`, con un `UNIQUE KEY ..._una_vigente` como última defensa contra dos filas vigentes.
La lista exacta de tablas se consulta en las migraciones, no se copia aquí.

**Toda escritura sobre estas tablas es una sola transacción auditada de dos pasos:**
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
  El detalle está en la sección 3.9.

### 3.3 Dinero y cantidades
Toda columna `DECIMAL` se mapea a `decimal.Decimal`, nunca a `float64` ni a `string` suelto. Incluye
las no monetarias (días, horas) por las fracciones de 0.5 días o 0.25 horas.

### 3.4 El motor no conoce códigos de concepto
Qué hace cada concepto lo dice su fórmula vigente (`concepto_salarial_formula.expresion_formula`,
clave `tipo`), no su código: una organización puede crear sus propios conceptos (pluses,
deducciones, cargas desglosadas) sin tocar Go. La columna `rubro` de las cargas es solo una
etiqueta legible. Únicas excepciones, en `internal/app`: el concepto cuyo mapeo `haber` es la
cuenta del neto por pagar (`SAL_BASE`) y el tipo de asiento del pago (`PAGO_PLANILLA`).

### 3.5 Tipos de fórmula

| `tipo` | Qué calcula | Claves propias |
|---|---|---|
| `referencia_directa` | salario del período por segmentos de contrato: mensual y quincenal por días comerciales, semanal por días naturales | |
| `horas_extra`, `feriado_laborado` | recargos sobre la hora ordinaria (salario llevado al mes ÷ horas mensuales de la jornada) | |
| `incapacidad` | parte patronal del subsidio por tramos, con el salario vigente o el promedio de los meses anteriores a la boleta (`INCAPACIDAD_SALARIO_BASE`) | |
| `aguinaldo`, `salario_escolar`, `vacaciones` | pagos diferidos y vacaciones (día vigente, promedio de las semanas anteriores o el mayor: `VACACIONES_SALARIO_BASE`) | |
| `carga_social` | cuota obrera o patronal sobre la base CCSS | `lado`: `obrero` o `patronal` |
| `renta` | impuesto al salario del mes, menos créditos | |
| `embargo`, `prestamo` | retenciones a terceros con límites legales | |
| `asignacion_empleado` | monto mensual o porcentaje asignado a cada empleado | `base`: `bruto`, `salario_base`, `ccss`, `renta` o `liquido` |
| `liquidacion` | rubros de la liquidación ya calculados | `rubro` |

Además, `tipo_calculo = fijo` (monto fijo del concepto) y `captura_manual` (ingresos o
deducciones digitados por período, sujetos al tope de neto si son voluntarios).

### 3.6 Reglas mensuales en planillas de fracción de mes
- **Renta:** el impuesto al salario es mensual. Una planilla mensual o de liquidación grava el
  total del mes (este período más los ya aprobados del mismo mes) y retiene la diferencia con lo
  ya retenido. En una planilla quincenal o semanal decide `RENTA_METODO_PERIODO`:
  `acumulado_mes` (lo mismo, exacto al cierre del mes) o `proyeccion` (tramos y créditos ×
  fracción del mes). Sin valor vigente, el cálculo de ese tipo de planilla falla.
- **Embargo:** el salario mínimo inembargable y el tope mensual de cada embargo se prorratean
  por la fracción del mes. La planilla de liquidación, en cambio, calcula los embargos sobre el
  mes: suma su líquido embargable al de los períodos aprobados del mes
  (`planilla_detalle_empleado.base_embargo`), aplica el mínimo y los topes mensuales completos y
  retiene, por embargo, la diferencia con lo ya retenido. Un embargo de monto fijo pide en la
  liquidación lo que diga `LIQ_EMBARGO_MONTO_FIJO`.
- **Base mínima CCSS** y **asignaciones en monto**: se prorratean por la fracción del mes.

### 3.7 Contrato único de error HTTP
Los handlers responden errores mediante `internal/api/middleware` (`ErrorResponse`, `Fail`,
`FailFromError`), que traduce los errores de dominio tipados a código HTTP. Ningún paquete
define su propio formato de error (`gin.H{"error": ...}` ad hoc).

### 3.8 Seguridad
- Contraseñas con argon2id. Tokens PASETO v4.local con claves aleatorias de 32 bytes en hex
  (64 caracteres), cargadas desde `PASETO_LOCAL_KEYS_HEX` y `PASETO_ACTIVE_KID`. Se acepta
  `PASETO_LOCAL_KEY_HEX` como configuración legada de una sola clave (kid `k1`).
- Sesiones persistidas y revocables (`refresh_tokens`), bloqueo por intentos fallidos y reset de
  contraseña de un solo uso. La política (umbral y minutos de bloqueo) va en configuración.
- `config.LoadConfig` rechaza claves ausentes, mal formadas o de ejemplo, y en producción exige
  `DB_PASSWORD`.

### 3.9 Tokens PASETO y revocación efectiva

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

### 3.10 Endurecimiento HTTP, errores y observabilidad

- **Contrato de error:** se conserva `{"error":{code,message,details,request_id}}`. La traducción
  del error a status, código y mensaje se centraliza; los mensajes de binding usan nombres de
  campos JSON y nunca reflejan valores recibidos. Los errores internos no exponen `err.Error()`.
  `ParseIDParam` devuelve un error tipado sin escribir la respuesta; el handler lo propaga una sola
  vez al manejador global.
- **Rate limiting:** limitador en memoria por IP+ruta para endpoints públicos de autenticación y
  por `usuario_id` para rutas protegidas. Se devuelve 429 con `Retry-After`. Las variables
  `RATE_LIMIT_AUTH_PER_MIN`, `RATE_LIMIT_AUTH_BURST`, `RATE_LIMIT_USER_PER_MIN` y
  `RATE_LIMIT_USER_BURST` son obligatorias y deben ser mayores que cero. La limitación es por
  proceso; con varias réplicas se requiere un límite en el gateway/WAF o un almacén compartido.
- **Proxy confiable:** `TRUSTED_PROXIES` define explícitamente los proxies confiables; vacío
  significa ninguno. En producción se rechazan las redes abiertas `0.0.0.0/0` y `::/0`.
- **Respuestas HTTP:** se envían cabeceras `nosniff`, `X-Frame-Options: DENY`,
  `Referrer-Policy: no-referrer`, `Cache-Control: no-store` y CSP `default-src 'none'`;
  HSTS se habilita solo en producción. CORS usa la lista exacta `CORS_ALLOWED_ORIGINS`, rechaza
  `*` y no habilita credenciales. `MAX_BODY_BYTES` limita el cuerpo de cada petición.
- **Logging:** `log/slog` en JSON; el log de petición incluye `request_id`, método, plantilla de
  ruta sin query string ni valores de parámetros, estado, latencia, bytes, IP y `usuario_id`.
  No se registran cuerpos ni cabeceras; atributos que sugieran secretos se redactan. Un
  `X-Request-ID` entrante se acepta solo si cumple `^[A-Za-z0-9._-]{8,64}$`.
- **TLS MySQL:** `DB_TLS_MODE` acepta `disabled`, `required` o `verify-full`, con
  `DB_TLS_CA_FILE` para la CA. `disabled` hace fallar el arranque en producción; `required`
  cifra el tráfico pero no valida el certificado, por lo que se recomienda `verify-full`.
- **Orden de middleware global:** `RequestID → SecurityHeaders → RequestLogger → Recovery →
  ErrorHandler → CORS → LimitarCuerpo → rutas`. Los middlewares que usan `Fail` deben ir después
  de `ErrorHandler`.

---

## 4. Base de datos y migraciones

- Las migraciones de `db/migrations/` (`NNNNNN_nombre.up.sql` y `.down.sql`, con punto y no guion
  bajo) son la **única fuente de verdad** del esquema.
- Migraciones existentes: de `000000` a `000152`, con huecos. Una nueva usa el siguiente número
  libre.
- Toda migración lleva su `.down.sql`. Advertencia heredada: el reverso de `usuario_roles` colapsa el
  histórico a las filas vigentes, así que no debe usarse en producción con historial real que conservar.
- Antes de cualquier cambio de esquema en producción: respaldo de la BD y reversión ensayada en
  staging.

---

## 5. Verificación y CI

Un solo comando, desde la raíz: `./scripts/pruebas_completas.sh`. Corre, en orden, `gofmt -l`
(falla si lista archivos), `go vet ./...` (también con `-tags=integration`), `go build ./...` y
`go test ./... -count=1`. Con `RUN_INTEGRATION=1` corre además las pruebas de integración contra
MySQL 8 (ver [pruebas.md](pruebas.md)). El pipeline de GitHub Actions está en
`.github/workflows/ci.yml`.

## 6. Checklist antes de producción

La lista completa, con parámetros, seguridad y operación, está en
[operacion.md](operacion.md#checklist-de-producción).
