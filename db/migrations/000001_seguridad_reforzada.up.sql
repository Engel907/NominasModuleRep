-- ============================================================================

-- ============================================================================
-- Origen: migrations/000001_seguridad_reforzada.up.sql
-- ============================================================================
-- ============================================================================
-- 000001_seguridad_reforzada.up.sql
-- CCDR de Liberia — Módulo Seguridad/Auth
-- Refuerza el bloque "6.10 SEGURIDAD Y AUDITORÍA" de planillas_ccdr_v11_completo.sql
-- Se aplica DESPUÉS de correr el archivo v11 completo. No modifica el archivo
-- v11 directamente (mandato del proyecto): todo va como ALTER/CREATE aquí.
--
-- Resuelve los puntos a-i del diagnóstico de seguridad:
--   a) usuarios: created_at / updated_at
--   b) refresh_tokens: sesiones persistidas y revocables
--   c) usuarios: intentos_fallidos / bloqueado_hasta (fuerza bruta)
--   d) tokens_reset_password + requiere_cambio_password
--   e) roles: descripcion / activo
--   f) usuario_roles: versionado por vigencia (patrón ya usado en 17 tablas
--      del esquema base), permite N roles simultáneos vigentes por usuario
--   g) (sin cambio de esquema: password_hash ya existía en v11; el hash con
--      bcrypt/argon2id se aplica en la capa Go de seguridad — Fase 2)
--   h) (sin cambio de esquema: rol_permisos ya modela permiso->rol N:M;
--      la decisión de autorizar por permiso y no por rol se aplica en el
--      middleware Go — Fase 2)
--   i) usuario_roles.creado_por / fecha_asignacion (cubierto en f)
--
-- Motor: InnoDB. Requiere las mismas versiones mínimas que el esquema base
-- (MySQL 8.0.16+ / MariaDB 10.3+), por CHECK constraints, columnas
-- generadas STORED y JSON_OBJECT en los triggers de auditoría.
-- Idempotente: cada DDL usa IF NOT EXISTS / DROP ... IF EXISTS donde MySQL
-- lo permite, para poder re-aplicar sin error en un ambiente ya migrado.
-- ============================================================================


-- ----------------------------------------------------------------------------
-- a) usuarios.created_at / updated_at
-- Rompía la convención que el propio v11 declara para el resto de tablas.
-- ----------------------------------------------------------------------------
ALTER TABLE usuarios
  ADD COLUMN created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
    COMMENT 'Fecha/hora en que se creó el registro' AFTER activo,
  ADD COLUMN updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
    COMMENT 'Fecha/hora de la última modificación del registro' AFTER created_at;

-- ----------------------------------------------------------------------------
-- c) Control de intentos fallidos de login / bloqueo temporal
-- El backend debe: incrementar intentos_fallidos en cada credencial inválida,
-- fijar bloqueado_hasta tras N intentos (política en capa de servicio, no
-- hardcodeada aquí porque no es un parámetro legal sino de seguridad
-- operativa), y resetear ambos campos en un login exitoso.
-- ----------------------------------------------------------------------------
ALTER TABLE usuarios
  ADD COLUMN intentos_fallidos SMALLINT UNSIGNED NOT NULL DEFAULT 0
    COMMENT 'Contador de intentos de login fallidos consecutivos; se resetea a 0 en login exitoso',
  ADD COLUMN bloqueado_hasta DATETIME NULL
    COMMENT 'Si no es NULL y es futuro, la cuenta está temporalmente bloqueada para login; se limpia al expirar o en cambio de contraseña';

-- ----------------------------------------------------------------------------
-- d) Forzar cambio de contraseña / "olvidé mi contraseña"
-- ----------------------------------------------------------------------------
ALTER TABLE usuarios
  ADD COLUMN requiere_cambio_password TINYINT(1) NOT NULL DEFAULT 0
    COMMENT '1 = el usuario debe cambiar su contraseña en el próximo login antes de poder usar el sistema (p.ej. tras un reset administrativo)',
  ADD COLUMN fecha_ultimo_cambio_password DATETIME NULL
    COMMENT 'Fecha/hora del último cambio de contraseña exitoso, para políticas de expiración futura';

CREATE TABLE IF NOT EXISTS tokens_reset_password (
  id                BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del token de restablecimiento',
  usuario_id        BIGINT UNSIGNED NOT NULL COMMENT 'Usuario que solicitó (o para quien se generó) el restablecimiento',
  token_hash        CHAR(64) NOT NULL COMMENT 'SHA-256 hex del token enviado por correo; el token en claro nunca se persiste',
  fecha_creacion    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Fecha/hora de generación del token',
  fecha_expiracion  DATETIME NOT NULL COMMENT 'A partir de esta fecha/hora el token deja de ser válido, incluso si no se usó',
  usado             TINYINT(1) NOT NULL DEFAULT 0 COMMENT '1 = el token ya se consumió; un token usado nunca vuelve a aceptarse (de un solo uso)',
  fecha_uso         DATETIME NULL COMMENT 'Fecha/hora en que se consumió el token, si aplica',
  ip_origen         VARCHAR(45) NULL COMMENT 'IP desde la que se solicitó el restablecimiento (IPv4 o IPv6)',
  CONSTRAINT fk_trp_usuario FOREIGN KEY (usuario_id) REFERENCES usuarios(id),
  UNIQUE KEY uk_trp_token_hash (token_hash),
  KEY ix_trp_usuario_vigentes (usuario_id, usado, fecha_expiracion)
) ENGINE=InnoDB COMMENT='Tokens de un solo uso, expirables, para el flujo de "olvidé mi contraseña"';

-- ----------------------------------------------------------------------------
-- b) Refresh tokens / sesiones activas, revocables
-- El access token PASETO sigue viviendo 15-30 min (capa Go, Fase 2); esta
-- tabla sostiene el refresh token de vida larga y el logout real / "cerrar
-- sesión en todos los dispositivos" (UPDATE masivo de revocado=1 por
-- usuario_id).
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS refresh_tokens (
  id                BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno de la sesión/refresh token',
  usuario_id        BIGINT UNSIGNED NOT NULL COMMENT 'Usuario dueño de esta sesión',
  token_hash        CHAR(64) NOT NULL COMMENT 'SHA-256 hex del refresh token; el token en claro solo existe en el cliente, nunca en la base de datos',
  fecha_emision     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Fecha/hora en que se emitió esta sesión (login o refresh)',
  fecha_expiracion  DATETIME NOT NULL COMMENT 'A partir de esta fecha/hora el refresh token deja de ser utilizable',
  revocado          TINYINT(1) NOT NULL DEFAULT 0 COMMENT '1 = sesión cerrada manualmente (logout) o invalidada administrativamente antes de expirar',
  fecha_revocacion  DATETIME NULL COMMENT 'Fecha/hora en que se revocó, si aplica',
  ip_origen         VARCHAR(45) NULL COMMENT 'IP desde la que se emitió/usó la sesión (IPv4 o IPv6)',
  user_agent        VARCHAR(255) NULL COMMENT 'User-Agent del cliente que solicitó la sesión, para que el usuario reconozca sus dispositivos activos',
  CONSTRAINT fk_rt_usuario FOREIGN KEY (usuario_id) REFERENCES usuarios(id),
  UNIQUE KEY uk_rt_token_hash (token_hash),
  KEY ix_rt_usuario_vigentes (usuario_id, revocado, fecha_expiracion)
) ENGINE=InnoDB COMMENT='Refresh tokens persistidos y revocables; base de logout real y "cerrar sesión en todos los dispositivos"';

-- ----------------------------------------------------------------------------
-- e) roles.descripcion / roles.activo
-- ----------------------------------------------------------------------------
ALTER TABLE roles
  ADD COLUMN descripcion VARCHAR(255) NULL
    COMMENT 'Descripción funcional del rol, para administración' AFTER nombre,
  ADD COLUMN activo TINYINT(1) NOT NULL DEFAULT 1
    COMMENT '1 = rol asignable a nuevos usuarios, 0 = descontinuado (no se borra para no perder histórico en usuario_roles)' AFTER descripcion;

-- ----------------------------------------------------------------------------
-- f) + i) usuario_roles: versionado por vigencia + trazabilidad de otorgamiento
--
-- El diseño original tenía PK compuesta (usuario_id, rol_id) sin histórico:
-- una revocación era un DELETE que borraba para siempre quién lo asignó y
-- cuándo. Se rediseña con el mismo patrón fecha_inicio_vigencia /
-- fecha_fin_vigencia / es_vigente que ya usan ~17 tablas del esquema base,
-- adaptado para permitir MÚLTIPLES roles simultáneos vigentes por usuario
-- (a diferencia del patrón 1-fila-vigente de otras tablas, aquí la unicidad
-- es por el PAR usuario+rol, no solo por usuario): un usuario puede tener
-- "Encargada RRHH" y "Contabilidad" vigentes al mismo tiempo, pero no puede
-- tener el MISMO rol vigente dos veces.
--
-- No hay ninguna FK externa hacia usuario_roles en el esquema v11 (se
-- verificó contra el archivo completo), así que es seguro recrearla.
-- ----------------------------------------------------------------------------
DROP TRIGGER IF EXISTS trg_aud_usuario_roles_ins;
DROP TRIGGER IF EXISTS trg_aud_usuario_roles_upd;
DROP TRIGGER IF EXISTS trg_aud_usuario_roles_del;
DROP TABLE IF EXISTS usuario_roles;

CREATE TABLE usuario_roles (
  id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno de esta asignación de rol',
  usuario_id             BIGINT UNSIGNED NOT NULL COMMENT 'Usuario al que se le asigna el rol',
  rol_id                 BIGINT UNSIGNED NOT NULL COMMENT 'Rol asignado al usuario',
  creado_por             BIGINT UNSIGNED NULL COMMENT 'Usuario administrador que otorgó este rol; NULL solo en cargas de migración inicial sin operador identificable',
  fecha_asignacion       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Fecha/hora exacta en que se otorgó el rol (distinta de fecha_inicio_vigencia, que es la fecha efectiva de negocio)',
  fecha_inicio_vigencia  DATE NOT NULL COMMENT 'Fecha desde la cual el usuario cuenta efectivamente con este rol',
  fecha_fin_vigencia     DATE NULL COMMENT 'NULL = rol vigente actualmente; fecha de revocación si ya no aplica',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED
    COMMENT 'Columna técnica: soporta uk_ur_una_vigente',
  CONSTRAINT fk_ur_usuario FOREIGN KEY (usuario_id) REFERENCES usuarios(id),
  CONSTRAINT fk_ur_rol FOREIGN KEY (rol_id) REFERENCES roles(id),
  CONSTRAINT fk_ur_creador FOREIGN KEY (creado_por) REFERENCES usuarios(id),
  CONSTRAINT chk_ur_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  UNIQUE KEY uk_ur_una_vigente (usuario_id, rol_id, es_vigente),
  KEY ix_ur_usuario_vigencia (usuario_id, fecha_inicio_vigencia, fecha_fin_vigencia)
) ENGINE=InnoDB COMMENT='Asignación de roles a usuarios, versionada; permite varios roles simultáneos por usuario y conserva histórico completo de otorgamiento/revocación (Ley 8292)';

-- ============================================================================
-- Extensión de los triggers de auditoría automática (sección 6.11 del v11)
-- para que capturen las columnas nuevas de usuarios, roles y usuario_roles.
-- Sin esto, la auditoría automática del propio v11 quedaría ciega a los
-- cambios de seguridad que introduce esta migración (contradice el punto 1
-- de los principios de diseño del esquema: trazabilidad completa).
-- password_hash se mantiene redactado, igual que en el v11 original.
-- ============================================================================

DROP TRIGGER IF EXISTS trg_aud_usuarios_ins;
CREATE TRIGGER trg_aud_usuarios_ins AFTER INSERT ON usuarios
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('usuarios', NEW.id, @auditoria_usuario_id, 'insert', NULL,
    JSON_OBJECT('id', NEW.id, 'nombre_usuario', NEW.nombre_usuario, 'nombre_completo', NEW.nombre_completo,
      'correo', NEW.correo, 'password_hash', '***REDACTED***', 'empleado_id', NEW.empleado_id, 'activo', NEW.activo,
      'intentos_fallidos', NEW.intentos_fallidos, 'bloqueado_hasta', NEW.bloqueado_hasta,
      'requiere_cambio_password', NEW.requiere_cambio_password,
      'fecha_ultimo_cambio_password', NEW.fecha_ultimo_cambio_password));
END;

DROP TRIGGER IF EXISTS trg_aud_usuarios_upd;
CREATE TRIGGER trg_aud_usuarios_upd AFTER UPDATE ON usuarios
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('usuarios', NEW.id, @auditoria_usuario_id, 'update',
    JSON_OBJECT('id', OLD.id, 'nombre_usuario', OLD.nombre_usuario, 'nombre_completo', OLD.nombre_completo,
      'correo', OLD.correo, 'password_hash', '***REDACTED***', 'empleado_id', OLD.empleado_id, 'activo', OLD.activo,
      'intentos_fallidos', OLD.intentos_fallidos, 'bloqueado_hasta', OLD.bloqueado_hasta,
      'requiere_cambio_password', OLD.requiere_cambio_password,
      'fecha_ultimo_cambio_password', OLD.fecha_ultimo_cambio_password),
    JSON_OBJECT('id', NEW.id, 'nombre_usuario', NEW.nombre_usuario, 'nombre_completo', NEW.nombre_completo,
      'correo', NEW.correo, 'password_hash', '***REDACTED***', 'empleado_id', NEW.empleado_id, 'activo', NEW.activo,
      'intentos_fallidos', NEW.intentos_fallidos, 'bloqueado_hasta', NEW.bloqueado_hasta,
      'requiere_cambio_password', NEW.requiere_cambio_password,
      'fecha_ultimo_cambio_password', NEW.fecha_ultimo_cambio_password));
END;

DROP TRIGGER IF EXISTS trg_aud_usuarios_del;
CREATE TRIGGER trg_aud_usuarios_del AFTER DELETE ON usuarios
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('usuarios', OLD.id, @auditoria_usuario_id, 'delete',
    JSON_OBJECT('id', OLD.id, 'nombre_usuario', OLD.nombre_usuario, 'nombre_completo', OLD.nombre_completo,
      'correo', OLD.correo, 'password_hash', '***REDACTED***', 'empleado_id', OLD.empleado_id, 'activo', OLD.activo,
      'intentos_fallidos', OLD.intentos_fallidos, 'bloqueado_hasta', OLD.bloqueado_hasta,
      'requiere_cambio_password', OLD.requiere_cambio_password,
      'fecha_ultimo_cambio_password', OLD.fecha_ultimo_cambio_password),
    NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_roles_ins;
CREATE TRIGGER trg_aud_roles_ins AFTER INSERT ON roles
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('roles', NEW.id, @auditoria_usuario_id, 'insert', NULL,
    JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'descripcion', NEW.descripcion, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_roles_upd;
CREATE TRIGGER trg_aud_roles_upd AFTER UPDATE ON roles
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('roles', NEW.id, @auditoria_usuario_id, 'update',
    JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'descripcion', OLD.descripcion, 'activo', OLD.activo),
    JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'descripcion', NEW.descripcion, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_roles_del;
CREATE TRIGGER trg_aud_roles_del AFTER DELETE ON roles
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('roles', OLD.id, @auditoria_usuario_id, 'delete',
    JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'descripcion', OLD.descripcion, 'activo', OLD.activo),
    NULL);
END;

-- usuario_roles ahora tiene id propio: se corrige además el defecto heredado
-- del v11 que logueaba registro_id = 0 (hardcodeado) para esta tabla.
DROP TRIGGER IF EXISTS trg_aud_usuario_roles_ins;
CREATE TRIGGER trg_aud_usuario_roles_ins AFTER INSERT ON usuario_roles
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('usuario_roles', NEW.id, @auditoria_usuario_id, 'insert', NULL,
    JSON_OBJECT('id', NEW.id, 'usuario_id', NEW.usuario_id, 'rol_id', NEW.rol_id, 'creado_por', NEW.creado_por,
      'fecha_asignacion', NEW.fecha_asignacion, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia,
      'fecha_fin_vigencia', NEW.fecha_fin_vigencia));
END;

DROP TRIGGER IF EXISTS trg_aud_usuario_roles_upd;
CREATE TRIGGER trg_aud_usuario_roles_upd AFTER UPDATE ON usuario_roles
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('usuario_roles', NEW.id, @auditoria_usuario_id, 'update',
    JSON_OBJECT('id', OLD.id, 'usuario_id', OLD.usuario_id, 'rol_id', OLD.rol_id, 'creado_por', OLD.creado_por,
      'fecha_asignacion', OLD.fecha_asignacion, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia,
      'fecha_fin_vigencia', OLD.fecha_fin_vigencia),
    JSON_OBJECT('id', NEW.id, 'usuario_id', NEW.usuario_id, 'rol_id', NEW.rol_id, 'creado_por', NEW.creado_por,
      'fecha_asignacion', NEW.fecha_asignacion, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia,
      'fecha_fin_vigencia', NEW.fecha_fin_vigencia));
END;

DROP TRIGGER IF EXISTS trg_aud_usuario_roles_del;
CREATE TRIGGER trg_aud_usuario_roles_del AFTER DELETE ON usuario_roles
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('usuario_roles', OLD.id, @auditoria_usuario_id, 'delete',
    JSON_OBJECT('id', OLD.id, 'usuario_id', OLD.usuario_id, 'rol_id', OLD.rol_id, 'creado_por', OLD.creado_por,
      'fecha_asignacion', OLD.fecha_asignacion, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia,
      'fecha_fin_vigencia', OLD.fecha_fin_vigencia),
    NULL);
END;

-- Trigger de auditoría para tokens_reset_password (tabla nueva); se excluye
-- token_hash del JSON por el mismo criterio que password_hash: aunque es un
-- hash y no el token en claro, no debe circular ni siquiera en la bitácora.
DROP TRIGGER IF EXISTS trg_aud_tokens_reset_password_ins;
CREATE TRIGGER trg_aud_tokens_reset_password_ins AFTER INSERT ON tokens_reset_password
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tokens_reset_password', NEW.id, @auditoria_usuario_id, 'insert', NULL,
    JSON_OBJECT('id', NEW.id, 'usuario_id', NEW.usuario_id, 'token_hash', '***REDACTED***',
      'fecha_creacion', NEW.fecha_creacion, 'fecha_expiracion', NEW.fecha_expiracion, 'usado', NEW.usado,
      'fecha_uso', NEW.fecha_uso, 'ip_origen', NEW.ip_origen));
END;

DROP TRIGGER IF EXISTS trg_aud_tokens_reset_password_upd;
CREATE TRIGGER trg_aud_tokens_reset_password_upd AFTER UPDATE ON tokens_reset_password
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tokens_reset_password', NEW.id, @auditoria_usuario_id, 'update',
    JSON_OBJECT('id', OLD.id, 'usuario_id', OLD.usuario_id, 'token_hash', '***REDACTED***',
      'fecha_creacion', OLD.fecha_creacion, 'fecha_expiracion', OLD.fecha_expiracion, 'usado', OLD.usado,
      'fecha_uso', OLD.fecha_uso, 'ip_origen', OLD.ip_origen),
    JSON_OBJECT('id', NEW.id, 'usuario_id', NEW.usuario_id, 'token_hash', '***REDACTED***',
      'fecha_creacion', NEW.fecha_creacion, 'fecha_expiracion', NEW.fecha_expiracion, 'usado', NEW.usado,
      'fecha_uso', NEW.fecha_uso, 'ip_origen', NEW.ip_origen));
END;

-- Trigger de auditoría para refresh_tokens (tabla nueva); mismo criterio de
-- redacción para token_hash.
DROP TRIGGER IF EXISTS trg_aud_refresh_tokens_ins;
CREATE TRIGGER trg_aud_refresh_tokens_ins AFTER INSERT ON refresh_tokens
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('refresh_tokens', NEW.id, @auditoria_usuario_id, 'insert', NULL,
    JSON_OBJECT('id', NEW.id, 'usuario_id', NEW.usuario_id, 'token_hash', '***REDACTED***',
      'fecha_emision', NEW.fecha_emision, 'fecha_expiracion', NEW.fecha_expiracion, 'revocado', NEW.revocado,
      'fecha_revocacion', NEW.fecha_revocacion, 'ip_origen', NEW.ip_origen, 'user_agent', NEW.user_agent));
END;

DROP TRIGGER IF EXISTS trg_aud_refresh_tokens_upd;
CREATE TRIGGER trg_aud_refresh_tokens_upd AFTER UPDATE ON refresh_tokens
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('refresh_tokens', NEW.id, @auditoria_usuario_id, 'update',
    JSON_OBJECT('id', OLD.id, 'usuario_id', OLD.usuario_id, 'token_hash', '***REDACTED***',
      'fecha_emision', OLD.fecha_emision, 'fecha_expiracion', OLD.fecha_expiracion, 'revocado', OLD.revocado,
      'fecha_revocacion', OLD.fecha_revocacion, 'ip_origen', OLD.ip_origen, 'user_agent', OLD.user_agent),
    JSON_OBJECT('id', NEW.id, 'usuario_id', NEW.usuario_id, 'token_hash', '***REDACTED***',
      'fecha_emision', NEW.fecha_emision, 'fecha_expiracion', NEW.fecha_expiracion, 'revocado', NEW.revocado,
      'fecha_revocacion', NEW.fecha_revocacion, 'ip_origen', NEW.ip_origen, 'user_agent', NEW.user_agent));
END;


