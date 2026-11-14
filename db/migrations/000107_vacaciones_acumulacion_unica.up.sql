ALTER TABLE vacaciones_movimientos
  ADD COLUMN acumulacion_periodo_key VARCHAR(20)
    GENERATED ALWAYS AS (CASE WHEN tipo_movimiento = 'acumulacion' THEN periodo_referencia ELSE NULL END) STORED
    COMMENT 'Columna técnica: periodo de acumulación; NULL para disfrute/ajuste. Soporta uk_vacmov_acumulacion_periodo',
  ADD UNIQUE KEY uk_vacmov_acumulacion_periodo (empleado_id, acumulacion_periodo_key);