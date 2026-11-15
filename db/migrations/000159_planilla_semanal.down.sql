-- Se niega (FK) si ya hay períodos semanales.
DELETE tpc FROM tipo_planilla_concepto tpc JOIN tipos_planilla t ON t.id = tpc.tipo_planilla_id
 WHERE t.codigo = 'semanal' AND NOT EXISTS (SELECT 1 FROM periodos_planilla p WHERE p.tipo_planilla_id = t.id);
DELETE FROM tipos_planilla WHERE codigo = 'semanal';
