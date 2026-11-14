ALTER TABLE refresh_tokens
  ADD COLUMN familia_id CHAR(36) NULL
    COMMENT 'UUID de la cadena de rotación (una por login)'
    AFTER usuario_id,
  ALGORITHM=INPLACE, LOCK=NONE;

ALTER TABLE refresh_tokens
  ADD KEY ix_rt_familia (familia_id, revocado),
  ALGORITHM=INPLACE, LOCK=NONE;
