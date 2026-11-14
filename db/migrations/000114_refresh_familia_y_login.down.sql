ALTER TABLE refresh_tokens
  DROP KEY ix_rt_familia,
  ALGORITHM=INPLACE, LOCK=NONE;

ALTER TABLE refresh_tokens
  DROP COLUMN familia_id,
  ALGORITHM=INPLACE, LOCK=NONE;
