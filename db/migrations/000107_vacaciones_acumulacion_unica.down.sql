ALTER TABLE vacaciones_movimientos
  DROP INDEX uk_vacmov_acumulacion_periodo,
  DROP COLUMN acumulacion_periodo_key;