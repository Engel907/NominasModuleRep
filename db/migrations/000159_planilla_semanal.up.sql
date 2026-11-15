INSERT INTO tipos_planilla (codigo, nombre, clase, fraccion_mensual)
SELECT 'semanal', 'Planilla semanal', 'regular', 0.2308
WHERE NOT EXISTS (SELECT 1 FROM tipos_planilla WHERE codigo = 'semanal');

INSERT INTO tipo_planilla_concepto (tipo_planilla_id, concepto_salarial_id)
SELECT s.id, tpc.concepto_salarial_id
FROM tipos_planilla s
JOIN tipos_planilla q ON q.codigo = 'quincenal'
JOIN tipo_planilla_concepto tpc ON tpc.tipo_planilla_id = q.id
WHERE s.codigo = 'semanal'
  AND NOT EXISTS (SELECT 1 FROM tipo_planilla_concepto x WHERE x.tipo_planilla_id = s.id AND x.concepto_salarial_id = tpc.concepto_salarial_id);
