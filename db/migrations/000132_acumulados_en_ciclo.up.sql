ALTER TABLE aguinaldos
  ADD COLUMN periodo_planilla_id BIGINT UNSIGNED NULL COMMENT 'Período que lo calculó',
  ADD CONSTRAINT fk_aguinaldo_periodo FOREIGN KEY (periodo_planilla_id) REFERENCES periodos_planilla(id);

ALTER TABLE salarios_escolares
  ADD COLUMN periodo_planilla_id BIGINT UNSIGNED NULL COMMENT 'Período que lo calculó',
  ADD CONSTRAINT fk_salesc_periodo FOREIGN KEY (periodo_planilla_id) REFERENCES periodos_planilla(id);

INSERT INTO tipo_planilla_concepto (tipo_planilla_id, concepto_salarial_id)
SELECT t.id, c.id FROM tipos_planilla t JOIN conceptos_salariales c
  ON (t.clase = 'aguinaldo' AND c.codigo = 'AGUINALDO') OR (t.clase = 'salario_escolar' AND c.codigo = 'SALARIO_ESCOLAR')
WHERE NOT EXISTS (SELECT 1 FROM tipo_planilla_concepto x WHERE x.tipo_planilla_id = t.id AND x.concepto_salarial_id = c.id);

DROP TRIGGER IF EXISTS trg_aud_aguinaldos_ins;
CREATE TRIGGER trg_aud_aguinaldos_ins AFTER INSERT ON aguinaldos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('aguinaldos', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'anio', NEW.anio, 'periodo_desde', NEW.periodo_desde, 'periodo_hasta', NEW.periodo_hasta, 'total_salarios_brutos', NEW.total_salarios_brutos, 'monto_aguinaldo', NEW.monto_aguinaldo, 'estado', NEW.estado, 'calculado_en', NEW.calculado_en, 'procesado_por', NEW.procesado_por, 'procesado_en', NEW.procesado_en, 'periodo_planilla_id', NEW.periodo_planilla_id));
END;

DROP TRIGGER IF EXISTS trg_aud_aguinaldos_upd;
CREATE TRIGGER trg_aud_aguinaldos_upd AFTER UPDATE ON aguinaldos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('aguinaldos', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'anio', OLD.anio, 'periodo_desde', OLD.periodo_desde, 'periodo_hasta', OLD.periodo_hasta, 'total_salarios_brutos', OLD.total_salarios_brutos, 'monto_aguinaldo', OLD.monto_aguinaldo, 'estado', OLD.estado, 'calculado_en', OLD.calculado_en, 'procesado_por', OLD.procesado_por, 'procesado_en', OLD.procesado_en, 'periodo_planilla_id', OLD.periodo_planilla_id), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'anio', NEW.anio, 'periodo_desde', NEW.periodo_desde, 'periodo_hasta', NEW.periodo_hasta, 'total_salarios_brutos', NEW.total_salarios_brutos, 'monto_aguinaldo', NEW.monto_aguinaldo, 'estado', NEW.estado, 'calculado_en', NEW.calculado_en, 'procesado_por', NEW.procesado_por, 'procesado_en', NEW.procesado_en, 'periodo_planilla_id', NEW.periodo_planilla_id));
END;

DROP TRIGGER IF EXISTS trg_aud_aguinaldos_del;
CREATE TRIGGER trg_aud_aguinaldos_del AFTER DELETE ON aguinaldos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('aguinaldos', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'anio', OLD.anio, 'periodo_desde', OLD.periodo_desde, 'periodo_hasta', OLD.periodo_hasta, 'total_salarios_brutos', OLD.total_salarios_brutos, 'monto_aguinaldo', OLD.monto_aguinaldo, 'estado', OLD.estado, 'calculado_en', OLD.calculado_en, 'procesado_por', OLD.procesado_por, 'procesado_en', OLD.procesado_en, 'periodo_planilla_id', OLD.periodo_planilla_id), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_salarios_escolares_ins;
CREATE TRIGGER trg_aud_salarios_escolares_ins AFTER INSERT ON salarios_escolares
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('salarios_escolares', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'anio', NEW.anio, 'periodo_desde', NEW.periodo_desde, 'periodo_hasta', NEW.periodo_hasta, 'porcentaje_aplicado', NEW.porcentaje_aplicado, 'total_salarios_brutos', NEW.total_salarios_brutos, 'monto_salario_escolar', NEW.monto_salario_escolar, 'estado', NEW.estado, 'calculado_en', NEW.calculado_en, 'procesado_por', NEW.procesado_por, 'procesado_en', NEW.procesado_en, 'periodo_planilla_id', NEW.periodo_planilla_id));
END;

DROP TRIGGER IF EXISTS trg_aud_salarios_escolares_upd;
CREATE TRIGGER trg_aud_salarios_escolares_upd AFTER UPDATE ON salarios_escolares
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('salarios_escolares', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'anio', OLD.anio, 'periodo_desde', OLD.periodo_desde, 'periodo_hasta', OLD.periodo_hasta, 'porcentaje_aplicado', OLD.porcentaje_aplicado, 'total_salarios_brutos', OLD.total_salarios_brutos, 'monto_salario_escolar', OLD.monto_salario_escolar, 'estado', OLD.estado, 'calculado_en', OLD.calculado_en, 'procesado_por', OLD.procesado_por, 'procesado_en', OLD.procesado_en, 'periodo_planilla_id', OLD.periodo_planilla_id), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'anio', NEW.anio, 'periodo_desde', NEW.periodo_desde, 'periodo_hasta', NEW.periodo_hasta, 'porcentaje_aplicado', NEW.porcentaje_aplicado, 'total_salarios_brutos', NEW.total_salarios_brutos, 'monto_salario_escolar', NEW.monto_salario_escolar, 'estado', NEW.estado, 'calculado_en', NEW.calculado_en, 'procesado_por', NEW.procesado_por, 'procesado_en', NEW.procesado_en, 'periodo_planilla_id', NEW.periodo_planilla_id));
END;

DROP TRIGGER IF EXISTS trg_aud_salarios_escolares_del;
CREATE TRIGGER trg_aud_salarios_escolares_del AFTER DELETE ON salarios_escolares
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('salarios_escolares', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'anio', OLD.anio, 'periodo_desde', OLD.periodo_desde, 'periodo_hasta', OLD.periodo_hasta, 'porcentaje_aplicado', OLD.porcentaje_aplicado, 'total_salarios_brutos', OLD.total_salarios_brutos, 'monto_salario_escolar', OLD.monto_salario_escolar, 'estado', OLD.estado, 'calculado_en', OLD.calculado_en, 'procesado_por', OLD.procesado_por, 'procesado_en', OLD.procesado_en, 'periodo_planilla_id', OLD.periodo_planilla_id), NULL);
END;

