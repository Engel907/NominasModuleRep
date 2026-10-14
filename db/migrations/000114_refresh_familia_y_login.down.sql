-- 000114_refresh_familia_y_login.down.sql
-- Revierte solo la estructura agregada por el .up. No hay datos que restaurar:
-- familia_id es información derivada de la rotación.

ALTER TABLE refresh_tokens
  DROP KEY ix_rt_familia,
  ALGORITHM=INPLACE, LOCK=NONE;

ALTER TABLE refresh_tokens
  DROP COLUMN familia_id,
  ALGORITHM=INPLACE, LOCK=NONE;
