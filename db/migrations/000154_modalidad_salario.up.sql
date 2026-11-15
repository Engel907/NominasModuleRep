ALTER TABLE contratos
  ADD COLUMN modalidad_salario ENUM('mensual','quincenal','semanal') NOT NULL DEFAULT 'mensual'
    COMMENT 'Período que remunera salario_pactado: mensual = monto del mes; quincenal = monto de la quincena; semanal = monto de la semana' AFTER salario_pactado;

ALTER TABLE contratos ALTER COLUMN modalidad_salario DROP DEFAULT;

ALTER TABLE contratos
  MODIFY salario_pactado DECIMAL(14,2) NOT NULL COMMENT 'Salario pactado en la unidad de modalidad_salario (mes, quincena o semana)';

DROP TRIGGER IF EXISTS trg_aud_contratos_ins;
CREATE TRIGGER trg_aud_contratos_ins AFTER INSERT ON contratos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('contratos', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'puesto_id', NEW.puesto_id, 'regimen_laboral_id', NEW.regimen_laboral_id, 'tipo_contrato_id', NEW.tipo_contrato_id, 'salario_pactado', NEW.salario_pactado, 'modalidad_salario', NEW.modalidad_salario, 'jornada_id', NEW.jornada_id, 'fecha_inicio_contrato', NEW.fecha_inicio_contrato, 'fecha_fin_contrato', NEW.fecha_fin_contrato, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'creado_por', NEW.creado_por));
END;

DROP TRIGGER IF EXISTS trg_aud_contratos_upd;
CREATE TRIGGER trg_aud_contratos_upd AFTER UPDATE ON contratos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('contratos', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'puesto_id', OLD.puesto_id, 'regimen_laboral_id', OLD.regimen_laboral_id, 'tipo_contrato_id', OLD.tipo_contrato_id, 'salario_pactado', OLD.salario_pactado, 'modalidad_salario', OLD.modalidad_salario, 'jornada_id', OLD.jornada_id, 'fecha_inicio_contrato', OLD.fecha_inicio_contrato, 'fecha_fin_contrato', OLD.fecha_fin_contrato, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'creado_por', OLD.creado_por), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'puesto_id', NEW.puesto_id, 'regimen_laboral_id', NEW.regimen_laboral_id, 'tipo_contrato_id', NEW.tipo_contrato_id, 'salario_pactado', NEW.salario_pactado, 'modalidad_salario', NEW.modalidad_salario, 'jornada_id', NEW.jornada_id, 'fecha_inicio_contrato', NEW.fecha_inicio_contrato, 'fecha_fin_contrato', NEW.fecha_fin_contrato, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'creado_por', NEW.creado_por));
END;

DROP TRIGGER IF EXISTS trg_aud_contratos_del;
CREATE TRIGGER trg_aud_contratos_del AFTER DELETE ON contratos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('contratos', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'puesto_id', OLD.puesto_id, 'regimen_laboral_id', OLD.regimen_laboral_id, 'tipo_contrato_id', OLD.tipo_contrato_id, 'salario_pactado', OLD.salario_pactado, 'modalidad_salario', OLD.modalidad_salario, 'jornada_id', OLD.jornada_id, 'fecha_inicio_contrato', OLD.fecha_inicio_contrato, 'fecha_fin_contrato', OLD.fecha_fin_contrato, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'creado_por', OLD.creado_por), NULL);
END;

