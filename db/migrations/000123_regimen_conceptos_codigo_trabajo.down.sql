-- Revierte solo las filas sembradas por 000123 (vigencia 2026-01-01 abierta).
DELETE r FROM regimen_concepto_aplicable r
JOIN regimenes_laborales rl ON rl.id = r.regimen_laboral_id
WHERE rl.codigo = 'CODIGO_TRABAJO'
  AND r.fecha_inicio_vigencia = '2026-01-01'
  AND r.fecha_fin_vigencia IS NULL;
