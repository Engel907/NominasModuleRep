-- 000114_refresh_familia_y_login.up.sql
-- F14 (S6): familia de sesiones para detectar reutilización de refresh tokens.
--
-- Cada login abre una familia (UUID); cada rotación hereda la familia de la
-- fila que reemplaza. Si un token ya revocado reaparece, el servicio revoca
-- TODA la familia (ver internal/auth Refresh / manejarReutilizacion).
--
-- Diseño para no bloquear tablas largas:
--   * La columna es NULL-able y SIN backfill: las filas existentes quedan con
--     familia_id NULL (el código las trata como "familia desconocida" y, ante
--     una reutilización, revoca todas las sesiones del usuario). Viven como
--     máximo AUTH_REFRESH_TOKEN_HOURS (7 días por defecto).
--   * ADD COLUMN (nullable, sin DEFAULT) y ADD KEY van con ALGORITHM=INPLACE y
--     LOCK=NONE: si MySQL no puede cumplirlo, falla en vez de bloquear.
--   * No se borra ni se modifica ningún dato.
--
-- ip_origen y user_agent YA existen en refresh_tokens (000001_seguridad_reforzada):
-- F14 solo empieza a poblarlos desde CrearRefreshTokenConFamilia.

ALTER TABLE refresh_tokens
  ADD COLUMN familia_id CHAR(36) NULL
    COMMENT 'UUID de la cadena de rotación (una por login); NULL en sesiones anteriores a F14'
    AFTER usuario_id,
  ALGORITHM=INPLACE, LOCK=NONE;

ALTER TABLE refresh_tokens
  ADD KEY ix_rt_familia (familia_id, revocado),
  ALGORITHM=INPLACE, LOCK=NONE;
