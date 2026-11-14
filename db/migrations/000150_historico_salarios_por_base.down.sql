ALTER TABLE historico_salarios_pre_migracion
  DROP FOREIGN KEY fk_hsm_usuario;

ALTER TABLE historico_salarios_pre_migracion
  DROP CHECK chk_hsm_montos,
  DROP INDEX uk_hsm,
  DROP COLUMN base_aguinaldo,
  DROP COLUMN base_cesantia,
  DROP COLUMN base_vacaciones,
  CHANGE monto_bruto monto DECIMAL(14,2) NOT NULL COMMENT 'Monto devengado (ingreso computable) ese mes',
  MODIFY fuente VARCHAR(100) NULL COMMENT 'Origen del dato migrado',
  MODIFY cargado_por_usuario_id BIGINT UNSIGNED NULL COMMENT 'Usuario que realizó la carga histórica',
  ADD COLUMN concepto_salarial_id BIGINT UNSIGNED NULL COMMENT 'Concepto salarial, si se tiene el desglose' AFTER mes,
  ADD CONSTRAINT fk_hsm_concepto FOREIGN KEY (concepto_salarial_id) REFERENCES conceptos_salariales(id),
  ADD UNIQUE KEY uk_hsm (empleado_id, anio, mes, concepto_salarial_id),
  ADD CONSTRAINT fk_hsm_usuario FOREIGN KEY (cargado_por_usuario_id) REFERENCES usuarios(id);

DROP TRIGGER IF EXISTS trg_aud_historico_salarios_pre_migracion_ins;
CREATE TRIGGER trg_aud_historico_salarios_pre_migracion_ins AFTER INSERT ON historico_salarios_pre_migracion
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('historico_salarios_pre_migracion', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'anio', NEW.anio, 'mes', NEW.mes, 'concepto_salarial_id', NEW.concepto_salarial_id, 'monto', NEW.monto, 'fuente', NEW.fuente, 'cargado_por_usuario_id', NEW.cargado_por_usuario_id, 'fecha_carga', NEW.fecha_carga));
END;

DROP TRIGGER IF EXISTS trg_aud_historico_salarios_pre_migracion_upd;
CREATE TRIGGER trg_aud_historico_salarios_pre_migracion_upd AFTER UPDATE ON historico_salarios_pre_migracion
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('historico_salarios_pre_migracion', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'anio', OLD.anio, 'mes', OLD.mes, 'concepto_salarial_id', OLD.concepto_salarial_id, 'monto', OLD.monto, 'fuente', OLD.fuente, 'cargado_por_usuario_id', OLD.cargado_por_usuario_id, 'fecha_carga', OLD.fecha_carga), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'anio', NEW.anio, 'mes', NEW.mes, 'concepto_salarial_id', NEW.concepto_salarial_id, 'monto', NEW.monto, 'fuente', NEW.fuente, 'cargado_por_usuario_id', NEW.cargado_por_usuario_id, 'fecha_carga', NEW.fecha_carga));
END;

DROP TRIGGER IF EXISTS trg_aud_historico_salarios_pre_migracion_del;
CREATE TRIGGER trg_aud_historico_salarios_pre_migracion_del AFTER DELETE ON historico_salarios_pre_migracion
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('historico_salarios_pre_migracion', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'anio', OLD.anio, 'mes', OLD.mes, 'concepto_salarial_id', OLD.concepto_salarial_id, 'monto', OLD.monto, 'fuente', OLD.fuente, 'cargado_por_usuario_id', OLD.cargado_por_usuario_id, 'fecha_carga', OLD.fecha_carga), NULL);
END;

