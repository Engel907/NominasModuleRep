DROP TRIGGER trg_contratos_catalogos_bi;
DROP TRIGGER IF EXISTS trg_aud_contratos_ins;
CREATE TRIGGER trg_aud_contratos_ins AFTER INSERT ON contratos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('contratos', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'puesto_id', NEW.puesto_id, 'regimen_laboral_id', NEW.regimen_laboral_id, 'tipo_contrato', NEW.tipo_contrato, 'tipo_contrato_id', NEW.tipo_contrato_id, 'salario_pactado', NEW.salario_pactado, 'jornada', NEW.jornada, 'fecha_inicio_contrato', NEW.fecha_inicio_contrato, 'fecha_fin_contrato', NEW.fecha_fin_contrato, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'creado_por', NEW.creado_por));
END;

DROP TRIGGER IF EXISTS trg_aud_contratos_upd;
CREATE TRIGGER trg_aud_contratos_upd AFTER UPDATE ON contratos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('contratos', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'puesto_id', OLD.puesto_id, 'regimen_laboral_id', OLD.regimen_laboral_id, 'tipo_contrato', OLD.tipo_contrato, 'tipo_contrato_id', OLD.tipo_contrato_id, 'salario_pactado', OLD.salario_pactado, 'jornada', OLD.jornada, 'fecha_inicio_contrato', OLD.fecha_inicio_contrato, 'fecha_fin_contrato', OLD.fecha_fin_contrato, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'creado_por', OLD.creado_por), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'puesto_id', NEW.puesto_id, 'regimen_laboral_id', NEW.regimen_laboral_id, 'tipo_contrato', NEW.tipo_contrato, 'tipo_contrato_id', NEW.tipo_contrato_id, 'salario_pactado', NEW.salario_pactado, 'jornada', NEW.jornada, 'fecha_inicio_contrato', NEW.fecha_inicio_contrato, 'fecha_fin_contrato', NEW.fecha_fin_contrato, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'creado_por', NEW.creado_por));
END;

DROP TRIGGER IF EXISTS trg_aud_contratos_del;
CREATE TRIGGER trg_aud_contratos_del AFTER DELETE ON contratos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('contratos', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'puesto_id', OLD.puesto_id, 'regimen_laboral_id', OLD.regimen_laboral_id, 'tipo_contrato', OLD.tipo_contrato, 'tipo_contrato_id', OLD.tipo_contrato_id, 'salario_pactado', OLD.salario_pactado, 'jornada', OLD.jornada, 'fecha_inicio_contrato', OLD.fecha_inicio_contrato, 'fecha_fin_contrato', OLD.fecha_fin_contrato, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'creado_por', OLD.creado_por), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_turnos_ins;
CREATE TRIGGER trg_aud_turnos_ins AFTER INSERT ON turnos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('turnos', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'hora_inicio', NEW.hora_inicio, 'hora_fin', NEW.hora_fin));
END;

DROP TRIGGER IF EXISTS trg_aud_turnos_upd;
CREATE TRIGGER trg_aud_turnos_upd AFTER UPDATE ON turnos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('turnos', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'hora_inicio', OLD.hora_inicio, 'hora_fin', OLD.hora_fin), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'hora_inicio', NEW.hora_inicio, 'hora_fin', NEW.hora_fin));
END;

DROP TRIGGER IF EXISTS trg_aud_turnos_del;
CREATE TRIGGER trg_aud_turnos_del AFTER DELETE ON turnos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('turnos', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'hora_inicio', OLD.hora_inicio, 'hora_fin', OLD.hora_fin), NULL);
END;

ALTER TABLE turnos DROP FOREIGN KEY fk_turnos_jornada;
ALTER TABLE turnos DROP COLUMN jornada_id;
ALTER TABLE contratos DROP FOREIGN KEY fk_contratos_jornada;
ALTER TABLE contratos DROP COLUMN jornada_id;
DROP TABLE parametros_jornada;
DROP TABLE jornadas;
