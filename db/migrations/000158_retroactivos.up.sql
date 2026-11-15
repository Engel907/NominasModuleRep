CREATE TABLE retroactivos (
  id                   BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  periodo_planilla_id  BIGINT UNSIGNED NOT NULL COMMENT 'Período regular en que se paga la diferencia',
  empleado_id          BIGINT UNSIGNED NOT NULL COMMENT 'Empleado al que se le recalculan los períodos aprobados',
  desde                DATE NOT NULL COMMENT 'Se recalculan los períodos aprobados con fecha_fin desde esta fecha',
  hasta                DATE NOT NULL COMMENT 'hasta esta fecha (anterior al inicio del período de pago)',
  motivo               VARCHAR(255) NOT NULL COMMENT 'Causa del retroactivo (aumento, reclasificación, resolución)',
  creado_por           BIGINT UNSIGNED NOT NULL COMMENT 'Usuario que lo registró',
  created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Registro',
  CONSTRAINT fk_retro_periodo  FOREIGN KEY (periodo_planilla_id) REFERENCES periodos_planilla(id),
  CONSTRAINT fk_retro_empleado FOREIGN KEY (empleado_id) REFERENCES empleados(id),
  CONSTRAINT fk_retro_usuario  FOREIGN KEY (creado_por) REFERENCES usuarios(id),
  CONSTRAINT chk_retro_rango CHECK (hasta >= desde),
  UNIQUE KEY uk_retro_periodo_empleado (periodo_planilla_id, empleado_id)
) ENGINE=InnoDB COMMENT='Recálculo de períodos aprobados de un empleado cuya diferencia paga un período regular';

ALTER TABLE planilla_detalle_concepto
  ADD COLUMN retroactivo_periodo_origen_id BIGINT UNSIGNED NULL
    COMMENT 'Período aprobado cuya diferencia paga la línea (retroactivo); NULL en una línea propia del período' AFTER motivo_ajuste,
  ADD CONSTRAINT fk_pdc_retro_origen FOREIGN KEY (retroactivo_periodo_origen_id) REFERENCES periodos_planilla(id),
  ADD KEY ix_pdc_retro_origen (retroactivo_periodo_origen_id);

DROP TRIGGER IF EXISTS trg_aud_retroactivos_ins;
CREATE TRIGGER trg_aud_retroactivos_ins AFTER INSERT ON retroactivos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('retroactivos', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'periodo_planilla_id', NEW.periodo_planilla_id, 'empleado_id', NEW.empleado_id, 'desde', NEW.desde, 'hasta', NEW.hasta, 'motivo', NEW.motivo, 'creado_por', NEW.creado_por));
END;

DROP TRIGGER IF EXISTS trg_aud_retroactivos_upd;
CREATE TRIGGER trg_aud_retroactivos_upd AFTER UPDATE ON retroactivos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('retroactivos', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'periodo_planilla_id', OLD.periodo_planilla_id, 'empleado_id', OLD.empleado_id, 'desde', OLD.desde, 'hasta', OLD.hasta, 'motivo', OLD.motivo, 'creado_por', OLD.creado_por), JSON_OBJECT('id', NEW.id, 'periodo_planilla_id', NEW.periodo_planilla_id, 'empleado_id', NEW.empleado_id, 'desde', NEW.desde, 'hasta', NEW.hasta, 'motivo', NEW.motivo, 'creado_por', NEW.creado_por));
END;

DROP TRIGGER IF EXISTS trg_aud_retroactivos_del;
CREATE TRIGGER trg_aud_retroactivos_del AFTER DELETE ON retroactivos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('retroactivos', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'periodo_planilla_id', OLD.periodo_planilla_id, 'empleado_id', OLD.empleado_id, 'desde', OLD.desde, 'hasta', OLD.hasta, 'motivo', OLD.motivo, 'creado_por', OLD.creado_por), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_planilla_detalle_concepto_ins;
CREATE TRIGGER trg_aud_planilla_detalle_concepto_ins AFTER INSERT ON planilla_detalle_concepto
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_concepto', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'planilla_detalle_empleado_id', NEW.planilla_detalle_empleado_id, 'concepto_salarial_id', NEW.concepto_salarial_id, 'concepto_salarial_formula_id', NEW.concepto_salarial_formula_id, 'empleado_concepto_formula_id', NEW.empleado_concepto_formula_id, 'cantidad', NEW.cantidad, 'monto', NEW.monto, 'concepto_ajuste_de_id', NEW.concepto_ajuste_de_id, 'motivo_ajuste', NEW.motivo_ajuste, 'retroactivo_periodo_origen_id', NEW.retroactivo_periodo_origen_id));
END;

DROP TRIGGER IF EXISTS trg_aud_planilla_detalle_concepto_upd;
CREATE TRIGGER trg_aud_planilla_detalle_concepto_upd AFTER UPDATE ON planilla_detalle_concepto
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_concepto', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'planilla_detalle_empleado_id', OLD.planilla_detalle_empleado_id, 'concepto_salarial_id', OLD.concepto_salarial_id, 'concepto_salarial_formula_id', OLD.concepto_salarial_formula_id, 'empleado_concepto_formula_id', OLD.empleado_concepto_formula_id, 'cantidad', OLD.cantidad, 'monto', OLD.monto, 'concepto_ajuste_de_id', OLD.concepto_ajuste_de_id, 'motivo_ajuste', OLD.motivo_ajuste, 'retroactivo_periodo_origen_id', OLD.retroactivo_periodo_origen_id), JSON_OBJECT('id', NEW.id, 'planilla_detalle_empleado_id', NEW.planilla_detalle_empleado_id, 'concepto_salarial_id', NEW.concepto_salarial_id, 'concepto_salarial_formula_id', NEW.concepto_salarial_formula_id, 'empleado_concepto_formula_id', NEW.empleado_concepto_formula_id, 'cantidad', NEW.cantidad, 'monto', NEW.monto, 'concepto_ajuste_de_id', NEW.concepto_ajuste_de_id, 'motivo_ajuste', NEW.motivo_ajuste, 'retroactivo_periodo_origen_id', NEW.retroactivo_periodo_origen_id));
END;

DROP TRIGGER IF EXISTS trg_aud_planilla_detalle_concepto_del;
CREATE TRIGGER trg_aud_planilla_detalle_concepto_del AFTER DELETE ON planilla_detalle_concepto
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_concepto', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'planilla_detalle_empleado_id', OLD.planilla_detalle_empleado_id, 'concepto_salarial_id', OLD.concepto_salarial_id, 'concepto_salarial_formula_id', OLD.concepto_salarial_formula_id, 'empleado_concepto_formula_id', OLD.empleado_concepto_formula_id, 'cantidad', OLD.cantidad, 'monto', OLD.monto, 'concepto_ajuste_de_id', OLD.concepto_ajuste_de_id, 'motivo_ajuste', OLD.motivo_ajuste, 'retroactivo_periodo_origen_id', OLD.retroactivo_periodo_origen_id), NULL);
END;

