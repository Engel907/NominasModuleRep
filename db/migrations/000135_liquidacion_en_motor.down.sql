DROP TRIGGER trg_aud_liquidaciones_ins;
DROP TRIGGER trg_aud_liquidaciones_upd;
DROP TRIGGER trg_aud_liquidaciones_del;

DELETE x FROM tipo_planilla_concepto x JOIN conceptos_salariales c ON c.id = x.concepto_salarial_id
 WHERE c.codigo IN ('LIQ_PREAVISO', 'LIQ_CESANTIA', 'LIQ_VACACIONES', 'LIQ_AGUINALDO', 'LIQ_SAL_ESCOLAR');
DELETE x FROM tipo_planilla_concepto x JOIN tipos_planilla t ON t.id = x.tipo_planilla_id
 WHERE t.clase = 'liquidacion';
DELETE x FROM regimen_concepto_aplicable x JOIN conceptos_salariales c ON c.id = x.concepto_salarial_id
 WHERE c.codigo IN ('LIQ_PREAVISO', 'LIQ_CESANTIA', 'LIQ_VACACIONES', 'LIQ_AGUINALDO', 'LIQ_SAL_ESCOLAR');
DELETE x FROM orden_calculo_concepto x JOIN conceptos_salariales c ON c.id = x.concepto_salarial_id
 WHERE c.codigo IN ('LIQ_PREAVISO', 'LIQ_CESANTIA', 'LIQ_VACACIONES', 'LIQ_AGUINALDO', 'LIQ_SAL_ESCOLAR');
DELETE x FROM concepto_salarial_formula x JOIN conceptos_salariales c ON c.id = x.concepto_salarial_id
 WHERE c.codigo IN ('LIQ_PREAVISO', 'LIQ_CESANTIA', 'LIQ_VACACIONES', 'LIQ_AGUINALDO', 'LIQ_SAL_ESCOLAR');
DELETE FROM conceptos_salariales
 WHERE codigo IN ('LIQ_PREAVISO', 'LIQ_CESANTIA', 'LIQ_VACACIONES', 'LIQ_AGUINALDO', 'LIQ_SAL_ESCOLAR');

ALTER TABLE liquidaciones
  DROP FOREIGN KEY fk_liq_periodo,
  DROP KEY ix_liq_periodo,
  DROP KEY ix_liq_cierre,
  DROP COLUMN periodo_planilla_id,
  DROP COLUMN cierre_aplicado_en;
