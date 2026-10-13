-- 000107_vacaciones_acumulacion_unica.up.sql
-- F07: red de seguridad a nivel de BD contra la doble acumulación del mismo
-- período. El servicio ya lo verifica bajo lock del empleado; este índice
-- cubre cualquier escritura que se salte el servicio.
--
-- Una columna generada es NULL salvo para movimientos 'acumulacion' con
-- periodo_referencia; los NULL no chocan en un índice UNIQUE de MySQL, así que
-- disfrutes y ajustes quedan fuera de la restricción.
--
-- ATENCIÓN: si ya existen acumulaciones duplicadas, el ALTER fallará. Revisar
-- primero con:
--   SELECT empleado_id, periodo_referencia, COUNT(*)
--   FROM vacaciones_movimientos
--   WHERE tipo_movimiento = 'acumulacion' AND periodo_referencia IS NOT NULL
--   GROUP BY empleado_id, periodo_referencia HAVING COUNT(*) > 1;
-- Esta migración NO borra ni modifica datos existentes.

ALTER TABLE vacaciones_movimientos
  ADD COLUMN acumulacion_periodo_key VARCHAR(20)
    GENERATED ALWAYS AS (CASE WHEN tipo_movimiento = 'acumulacion' THEN periodo_referencia ELSE NULL END) STORED
    COMMENT 'Columna técnica: periodo de acumulación; NULL para disfrute/ajuste. Soporta uk_vacmov_acumulacion_periodo',
  ADD UNIQUE KEY uk_vacmov_acumulacion_periodo (empleado_id, acumulacion_periodo_key);