ALTER TABLE liquidaciones
  ADD COLUMN periodo_planilla_id BIGINT UNSIGNED NULL COMMENT 'Período mensual de liquidaciones que la paga',
  ADD COLUMN cierre_aplicado_en DATETIME NULL COMMENT 'Cierre de contrato/datos laborales y baja del empleado; NULL = salida programada',
  ADD CONSTRAINT fk_liq_periodo FOREIGN KEY (periodo_planilla_id) REFERENCES periodos_planilla(id),
  ADD KEY ix_liq_periodo (periodo_planilla_id),
  ADD KEY ix_liq_cierre (estado, cierre_aplicado_en, fecha_salida);

UPDATE liquidaciones SET cierre_aplicado_en = COALESCE(procesado_en, calculado_en)
 WHERE estado = 'procesada' AND cierre_aplicado_en IS NULL;

INSERT INTO conceptos_salariales (codigo, nombre, naturaleza, tipo_calculo, afecta_base_ccss, afecta_base_renta,
  afecta_base_aguinaldo, afecta_base_cesantia, afecta_base_vacaciones, afecta_base_embargo, es_deduccion_obligatoria, activo)
SELECT v.codigo, v.nombre, 'ingreso', 'formula', v.ccss, v.renta, 0, 0, 0, v.embargo, 0, 1 FROM (
  SELECT 'LIQ_PREAVISO' AS codigo, 'Preaviso' AS nombre, 0 AS ccss, 0 AS renta, 0 AS embargo
  UNION ALL SELECT 'LIQ_CESANTIA', 'Cesantía', 0, 0, 0
  UNION ALL SELECT 'LIQ_VACACIONES', 'Vacaciones proporcionales', 1, 1, 1
  UNION ALL SELECT 'LIQ_AGUINALDO', 'Aguinaldo proporcional', 0, 0, 0
  UNION ALL SELECT 'LIQ_SAL_ESCOLAR', 'Salario escolar proporcional', 0, 0, 0
) v WHERE NOT EXISTS (SELECT 1 FROM conceptos_salariales c WHERE c.codigo = v.codigo);

INSERT INTO concepto_salarial_formula (concepto_salarial_id, monto_fijo, expresion_formula, fecha_inicio_vigencia, fecha_fin_vigencia)
SELECT cs.id, NULL, JSON_OBJECT('tipo', 'liquidacion', 'rubro', v.rubro), '2026-01-01', NULL
FROM conceptos_salariales cs
JOIN (
  SELECT 'LIQ_PREAVISO' AS codigo, 'preaviso' AS rubro
  UNION ALL SELECT 'LIQ_CESANTIA', 'cesantia'
  UNION ALL SELECT 'LIQ_VACACIONES', 'vacaciones'
  UNION ALL SELECT 'LIQ_AGUINALDO', 'aguinaldo'
  UNION ALL SELECT 'LIQ_SAL_ESCOLAR', 'salario_escolar'
) v ON v.codigo = cs.codigo
WHERE NOT EXISTS (SELECT 1 FROM concepto_salarial_formula f WHERE f.concepto_salarial_id = cs.id AND f.es_vigente = 1);

INSERT INTO orden_calculo_concepto (concepto_salarial_id, orden, fecha_inicio_vigencia, fecha_fin_vigencia)
SELECT cs.id, v.orden, '2026-01-01', NULL
FROM conceptos_salariales cs
JOIN (
  SELECT 'LIQ_PREAVISO' AS codigo, 91 AS orden
  UNION ALL SELECT 'LIQ_CESANTIA', 92
  UNION ALL SELECT 'LIQ_VACACIONES', 93
  UNION ALL SELECT 'LIQ_AGUINALDO', 94
  UNION ALL SELECT 'LIQ_SAL_ESCOLAR', 95
) v ON v.codigo = cs.codigo
WHERE NOT EXISTS (SELECT 1 FROM orden_calculo_concepto o WHERE o.concepto_salarial_id = cs.id AND o.es_vigente = 1);

INSERT INTO regimen_concepto_aplicable (regimen_laboral_id, concepto_salarial_id, fecha_inicio_vigencia, fecha_fin_vigencia)
SELECT rl.id, cs.id, '2026-01-01', NULL
FROM regimenes_laborales rl
JOIN conceptos_salariales cs ON cs.codigo IN ('LIQ_PREAVISO', 'LIQ_CESANTIA', 'LIQ_VACACIONES', 'LIQ_AGUINALDO', 'LIQ_SAL_ESCOLAR')
WHERE rl.codigo = 'CODIGO_TRABAJO'
  AND NOT EXISTS (SELECT 1 FROM regimen_concepto_aplicable r
                  WHERE r.regimen_laboral_id = rl.id AND r.concepto_salarial_id = cs.id AND r.fecha_fin_vigencia IS NULL);

INSERT INTO tipo_planilla_concepto (tipo_planilla_id, concepto_salarial_id)
SELECT t.id, c.id FROM tipos_planilla t JOIN conceptos_salariales c
  ON c.codigo IN ('LIQ_PREAVISO', 'LIQ_CESANTIA', 'LIQ_VACACIONES', 'LIQ_AGUINALDO', 'LIQ_SAL_ESCOLAR',
                  'CCSS_OBR', 'CCSS_PAT', 'IMP_RENTA', 'EMBARGO', 'PRESTAMO')
WHERE t.clase = 'liquidacion'
  AND NOT EXISTS (SELECT 1 FROM tipo_planilla_concepto x WHERE x.tipo_planilla_id = t.id AND x.concepto_salarial_id = c.id);

DROP TRIGGER IF EXISTS trg_aud_liquidaciones_ins;
CREATE TRIGGER trg_aud_liquidaciones_ins AFTER INSERT ON liquidaciones
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('liquidaciones', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'fecha_salida', NEW.fecha_salida, 'motivo_salida', NEW.motivo_salida, 'aplica_responsabilidad_patronal', NEW.aplica_responsabilidad_patronal, 'fecha_ingreso', NEW.fecha_ingreso, 'antiguedad_meses', NEW.antiguedad_meses, 'salario_promedio_6meses', NEW.salario_promedio_6meses, 'salario_diario_promedio', NEW.salario_diario_promedio, 'dias_preaviso', NEW.dias_preaviso, 'monto_preaviso', NEW.monto_preaviso, 'dias_cesantia', NEW.dias_cesantia, 'monto_cesantia', NEW.monto_cesantia, 'dias_vacaciones_pendientes', NEW.dias_vacaciones_pendientes, 'monto_vacaciones_proporcionales', NEW.monto_vacaciones_proporcionales, 'periodo_aguinaldo_desde', NEW.periodo_aguinaldo_desde, 'total_salarios_brutos_aguinaldo_proporcional', NEW.total_salarios_brutos_aguinaldo_proporcional, 'monto_aguinaldo_proporcional', NEW.monto_aguinaldo_proporcional, 'salario_diario_vacaciones', NEW.salario_diario_vacaciones, 'periodo_salario_escolar_desde', NEW.periodo_salario_escolar_desde, 'total_base_salario_escolar_proporcional', NEW.total_base_salario_escolar_proporcional, 'monto_salario_escolar_proporcional', NEW.monto_salario_escolar_proporcional, 'total_liquidacion', NEW.total_liquidacion, 'estado', NEW.estado, 'calculado_en', NEW.calculado_en, 'procesado_por', NEW.procesado_por, 'procesado_en', NEW.procesado_en, 'periodo_planilla_id', NEW.periodo_planilla_id, 'cierre_aplicado_en', NEW.cierre_aplicado_en));
END;

DROP TRIGGER IF EXISTS trg_aud_liquidaciones_upd;
CREATE TRIGGER trg_aud_liquidaciones_upd AFTER UPDATE ON liquidaciones
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('liquidaciones', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'fecha_salida', OLD.fecha_salida, 'motivo_salida', OLD.motivo_salida, 'aplica_responsabilidad_patronal', OLD.aplica_responsabilidad_patronal, 'fecha_ingreso', OLD.fecha_ingreso, 'antiguedad_meses', OLD.antiguedad_meses, 'salario_promedio_6meses', OLD.salario_promedio_6meses, 'salario_diario_promedio', OLD.salario_diario_promedio, 'dias_preaviso', OLD.dias_preaviso, 'monto_preaviso', OLD.monto_preaviso, 'dias_cesantia', OLD.dias_cesantia, 'monto_cesantia', OLD.monto_cesantia, 'dias_vacaciones_pendientes', OLD.dias_vacaciones_pendientes, 'monto_vacaciones_proporcionales', OLD.monto_vacaciones_proporcionales, 'periodo_aguinaldo_desde', OLD.periodo_aguinaldo_desde, 'total_salarios_brutos_aguinaldo_proporcional', OLD.total_salarios_brutos_aguinaldo_proporcional, 'monto_aguinaldo_proporcional', OLD.monto_aguinaldo_proporcional, 'salario_diario_vacaciones', OLD.salario_diario_vacaciones, 'periodo_salario_escolar_desde', OLD.periodo_salario_escolar_desde, 'total_base_salario_escolar_proporcional', OLD.total_base_salario_escolar_proporcional, 'monto_salario_escolar_proporcional', OLD.monto_salario_escolar_proporcional, 'total_liquidacion', OLD.total_liquidacion, 'estado', OLD.estado, 'calculado_en', OLD.calculado_en, 'procesado_por', OLD.procesado_por, 'procesado_en', OLD.procesado_en, 'periodo_planilla_id', OLD.periodo_planilla_id, 'cierre_aplicado_en', OLD.cierre_aplicado_en), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'fecha_salida', NEW.fecha_salida, 'motivo_salida', NEW.motivo_salida, 'aplica_responsabilidad_patronal', NEW.aplica_responsabilidad_patronal, 'fecha_ingreso', NEW.fecha_ingreso, 'antiguedad_meses', NEW.antiguedad_meses, 'salario_promedio_6meses', NEW.salario_promedio_6meses, 'salario_diario_promedio', NEW.salario_diario_promedio, 'dias_preaviso', NEW.dias_preaviso, 'monto_preaviso', NEW.monto_preaviso, 'dias_cesantia', NEW.dias_cesantia, 'monto_cesantia', NEW.monto_cesantia, 'dias_vacaciones_pendientes', NEW.dias_vacaciones_pendientes, 'monto_vacaciones_proporcionales', NEW.monto_vacaciones_proporcionales, 'periodo_aguinaldo_desde', NEW.periodo_aguinaldo_desde, 'total_salarios_brutos_aguinaldo_proporcional', NEW.total_salarios_brutos_aguinaldo_proporcional, 'monto_aguinaldo_proporcional', NEW.monto_aguinaldo_proporcional, 'salario_diario_vacaciones', NEW.salario_diario_vacaciones, 'periodo_salario_escolar_desde', NEW.periodo_salario_escolar_desde, 'total_base_salario_escolar_proporcional', NEW.total_base_salario_escolar_proporcional, 'monto_salario_escolar_proporcional', NEW.monto_salario_escolar_proporcional, 'total_liquidacion', NEW.total_liquidacion, 'estado', NEW.estado, 'calculado_en', NEW.calculado_en, 'procesado_por', NEW.procesado_por, 'procesado_en', NEW.procesado_en, 'periodo_planilla_id', NEW.periodo_planilla_id, 'cierre_aplicado_en', NEW.cierre_aplicado_en));
END;

DROP TRIGGER IF EXISTS trg_aud_liquidaciones_del;
CREATE TRIGGER trg_aud_liquidaciones_del AFTER DELETE ON liquidaciones
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('liquidaciones', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'fecha_salida', OLD.fecha_salida, 'motivo_salida', OLD.motivo_salida, 'aplica_responsabilidad_patronal', OLD.aplica_responsabilidad_patronal, 'fecha_ingreso', OLD.fecha_ingreso, 'antiguedad_meses', OLD.antiguedad_meses, 'salario_promedio_6meses', OLD.salario_promedio_6meses, 'salario_diario_promedio', OLD.salario_diario_promedio, 'dias_preaviso', OLD.dias_preaviso, 'monto_preaviso', OLD.monto_preaviso, 'dias_cesantia', OLD.dias_cesantia, 'monto_cesantia', OLD.monto_cesantia, 'dias_vacaciones_pendientes', OLD.dias_vacaciones_pendientes, 'monto_vacaciones_proporcionales', OLD.monto_vacaciones_proporcionales, 'periodo_aguinaldo_desde', OLD.periodo_aguinaldo_desde, 'total_salarios_brutos_aguinaldo_proporcional', OLD.total_salarios_brutos_aguinaldo_proporcional, 'monto_aguinaldo_proporcional', OLD.monto_aguinaldo_proporcional, 'salario_diario_vacaciones', OLD.salario_diario_vacaciones, 'periodo_salario_escolar_desde', OLD.periodo_salario_escolar_desde, 'total_base_salario_escolar_proporcional', OLD.total_base_salario_escolar_proporcional, 'monto_salario_escolar_proporcional', OLD.monto_salario_escolar_proporcional, 'total_liquidacion', OLD.total_liquidacion, 'estado', OLD.estado, 'calculado_en', OLD.calculado_en, 'procesado_por', OLD.procesado_por, 'procesado_en', OLD.procesado_en, 'periodo_planilla_id', OLD.periodo_planilla_id, 'cierre_aplicado_en', OLD.cierre_aplicado_en), NULL);
END;

