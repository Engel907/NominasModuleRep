ALTER TABLE vacaciones_movimientos
  MODIFY tipo_movimiento ENUM('acumulacion','disfrute','ajuste','compensacion') NOT NULL,
  ADD COLUMN periodo_planilla_id BIGINT UNSIGNED NULL COMMENT 'Período que paga la compensación',
  ADD COLUMN pagado_en DATETIME NULL COMMENT 'Se llena al aprobar el período; NULL = no pagado',
  ADD CONSTRAINT fk_vacmov_periodo FOREIGN KEY (periodo_planilla_id) REFERENCES periodos_planilla(id);

DROP TRIGGER IF EXISTS trg_aud_vacaciones_movimientos_ins;
CREATE TRIGGER trg_aud_vacaciones_movimientos_ins AFTER INSERT ON vacaciones_movimientos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('vacaciones_movimientos', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'tipo_movimiento', NEW.tipo_movimiento, 'dias', NEW.dias, 'fecha_efectiva', NEW.fecha_efectiva, 'periodo_referencia', NEW.periodo_referencia, 'solicitud_id', NEW.solicitud_id, 'observaciones', NEW.observaciones, 'creado_por', NEW.creado_por, 'created_at', NEW.created_at, 'periodo_planilla_id', NEW.periodo_planilla_id, 'pagado_en', NEW.pagado_en));
END;

DROP TRIGGER IF EXISTS trg_aud_vacaciones_movimientos_upd;
CREATE TRIGGER trg_aud_vacaciones_movimientos_upd AFTER UPDATE ON vacaciones_movimientos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('vacaciones_movimientos', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'tipo_movimiento', OLD.tipo_movimiento, 'dias', OLD.dias, 'fecha_efectiva', OLD.fecha_efectiva, 'periodo_referencia', OLD.periodo_referencia, 'solicitud_id', OLD.solicitud_id, 'observaciones', OLD.observaciones, 'creado_por', OLD.creado_por, 'created_at', OLD.created_at, 'periodo_planilla_id', OLD.periodo_planilla_id, 'pagado_en', OLD.pagado_en), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'tipo_movimiento', NEW.tipo_movimiento, 'dias', NEW.dias, 'fecha_efectiva', NEW.fecha_efectiva, 'periodo_referencia', NEW.periodo_referencia, 'solicitud_id', NEW.solicitud_id, 'observaciones', NEW.observaciones, 'creado_por', NEW.creado_por, 'created_at', NEW.created_at, 'periodo_planilla_id', NEW.periodo_planilla_id, 'pagado_en', NEW.pagado_en));
END;

DROP TRIGGER IF EXISTS trg_aud_vacaciones_movimientos_del;
CREATE TRIGGER trg_aud_vacaciones_movimientos_del AFTER DELETE ON vacaciones_movimientos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('vacaciones_movimientos', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'tipo_movimiento', OLD.tipo_movimiento, 'dias', OLD.dias, 'fecha_efectiva', OLD.fecha_efectiva, 'periodo_referencia', OLD.periodo_referencia, 'solicitud_id', OLD.solicitud_id, 'observaciones', OLD.observaciones, 'creado_por', OLD.creado_por, 'created_at', OLD.created_at, 'periodo_planilla_id', OLD.periodo_planilla_id, 'pagado_en', OLD.pagado_en), NULL);
END;

