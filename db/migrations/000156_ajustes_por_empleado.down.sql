ALTER TABLE planilla_detalle_concepto
  DROP CHECK chk_pdc_una_formula,
  DROP FOREIGN KEY fk_pdc_formula_empleado;
ALTER TABLE planilla_detalle_concepto DROP COLUMN empleado_concepto_formula_id;

DROP TRIGGER IF EXISTS trg_aud_planilla_detalle_concepto_ins;
CREATE TRIGGER trg_aud_planilla_detalle_concepto_ins AFTER INSERT ON planilla_detalle_concepto
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_concepto', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'planilla_detalle_empleado_id', NEW.planilla_detalle_empleado_id, 'concepto_salarial_id', NEW.concepto_salarial_id, 'concepto_salarial_formula_id', NEW.concepto_salarial_formula_id, 'cantidad', NEW.cantidad, 'monto', NEW.monto, 'concepto_ajuste_de_id', NEW.concepto_ajuste_de_id, 'motivo_ajuste', NEW.motivo_ajuste));
END;

DROP TRIGGER IF EXISTS trg_aud_planilla_detalle_concepto_upd;
CREATE TRIGGER trg_aud_planilla_detalle_concepto_upd AFTER UPDATE ON planilla_detalle_concepto
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_concepto', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'planilla_detalle_empleado_id', OLD.planilla_detalle_empleado_id, 'concepto_salarial_id', OLD.concepto_salarial_id, 'concepto_salarial_formula_id', OLD.concepto_salarial_formula_id, 'cantidad', OLD.cantidad, 'monto', OLD.monto, 'concepto_ajuste_de_id', OLD.concepto_ajuste_de_id, 'motivo_ajuste', OLD.motivo_ajuste), JSON_OBJECT('id', NEW.id, 'planilla_detalle_empleado_id', NEW.planilla_detalle_empleado_id, 'concepto_salarial_id', NEW.concepto_salarial_id, 'concepto_salarial_formula_id', NEW.concepto_salarial_formula_id, 'cantidad', NEW.cantidad, 'monto', NEW.monto, 'concepto_ajuste_de_id', NEW.concepto_ajuste_de_id, 'motivo_ajuste', NEW.motivo_ajuste));
END;

DROP TRIGGER IF EXISTS trg_aud_planilla_detalle_concepto_del;
CREATE TRIGGER trg_aud_planilla_detalle_concepto_del AFTER DELETE ON planilla_detalle_concepto
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_concepto', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'planilla_detalle_empleado_id', OLD.planilla_detalle_empleado_id, 'concepto_salarial_id', OLD.concepto_salarial_id, 'concepto_salarial_formula_id', OLD.concepto_salarial_formula_id, 'cantidad', OLD.cantidad, 'monto', OLD.monto, 'concepto_ajuste_de_id', OLD.concepto_ajuste_de_id, 'motivo_ajuste', OLD.motivo_ajuste), NULL);
END;

DROP TABLE empleado_parametros_generales;
DROP TABLE empleado_concepto_formula;
