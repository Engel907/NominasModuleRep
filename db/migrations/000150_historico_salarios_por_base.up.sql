ALTER TABLE historico_salarios_pre_migracion
  DROP FOREIGN KEY fk_hsm_concepto,
  DROP FOREIGN KEY fk_hsm_usuario;

ALTER TABLE historico_salarios_pre_migracion
  DROP INDEX uk_hsm,
  DROP INDEX fk_hsm_concepto,
  DROP COLUMN concepto_salarial_id,
  CHANGE monto monto_bruto DECIMAL(14,2) NOT NULL COMMENT 'Total devengado en el mes',
  ADD COLUMN base_aguinaldo DECIMAL(14,2) NOT NULL COMMENT 'Parte del mes que integra la base de aguinaldo y salario escolar' AFTER monto_bruto,
  ADD COLUMN base_cesantia DECIMAL(14,2) NOT NULL COMMENT 'Parte del mes que integra el promedio de preaviso y cesantía' AFTER base_aguinaldo,
  ADD COLUMN base_vacaciones DECIMAL(14,2) NOT NULL COMMENT 'Parte del mes que integra el promedio de vacaciones' AFTER base_cesantia,
  MODIFY fuente VARCHAR(100) NOT NULL COMMENT 'Origen del dato (sistema anterior, planilla en hoja de cálculo…)',
  MODIFY cargado_por_usuario_id BIGINT UNSIGNED NOT NULL COMMENT 'Usuario que cargó el mes',
  ADD CONSTRAINT chk_hsm_montos CHECK (monto_bruto >= 0 AND base_aguinaldo >= 0 AND base_cesantia >= 0 AND base_vacaciones >= 0
                                       AND base_aguinaldo <= monto_bruto AND base_cesantia <= monto_bruto AND base_vacaciones <= monto_bruto),
  ADD UNIQUE KEY uk_hsm (empleado_id, anio, mes),
  ADD CONSTRAINT fk_hsm_usuario FOREIGN KEY (cargado_por_usuario_id) REFERENCES usuarios(id),
  COMMENT = 'Salarios de meses anteriores a la puesta en marcha: completan las bases de aguinaldo, salario escolar y promedios de liquidación';

DROP TRIGGER IF EXISTS trg_aud_historico_salarios_pre_migracion_ins;
CREATE TRIGGER trg_aud_historico_salarios_pre_migracion_ins AFTER INSERT ON historico_salarios_pre_migracion
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('historico_salarios_pre_migracion', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'anio', NEW.anio, 'mes', NEW.mes, 'monto_bruto', NEW.monto_bruto, 'base_aguinaldo', NEW.base_aguinaldo, 'base_cesantia', NEW.base_cesantia, 'base_vacaciones', NEW.base_vacaciones, 'fuente', NEW.fuente, 'cargado_por_usuario_id', NEW.cargado_por_usuario_id, 'fecha_carga', NEW.fecha_carga));
END;

DROP TRIGGER IF EXISTS trg_aud_historico_salarios_pre_migracion_upd;
CREATE TRIGGER trg_aud_historico_salarios_pre_migracion_upd AFTER UPDATE ON historico_salarios_pre_migracion
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('historico_salarios_pre_migracion', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'anio', OLD.anio, 'mes', OLD.mes, 'monto_bruto', OLD.monto_bruto, 'base_aguinaldo', OLD.base_aguinaldo, 'base_cesantia', OLD.base_cesantia, 'base_vacaciones', OLD.base_vacaciones, 'fuente', OLD.fuente, 'cargado_por_usuario_id', OLD.cargado_por_usuario_id, 'fecha_carga', OLD.fecha_carga), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'anio', NEW.anio, 'mes', NEW.mes, 'monto_bruto', NEW.monto_bruto, 'base_aguinaldo', NEW.base_aguinaldo, 'base_cesantia', NEW.base_cesantia, 'base_vacaciones', NEW.base_vacaciones, 'fuente', NEW.fuente, 'cargado_por_usuario_id', NEW.cargado_por_usuario_id, 'fecha_carga', NEW.fecha_carga));
END;

DROP TRIGGER IF EXISTS trg_aud_historico_salarios_pre_migracion_del;
CREATE TRIGGER trg_aud_historico_salarios_pre_migracion_del AFTER DELETE ON historico_salarios_pre_migracion
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('historico_salarios_pre_migracion', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'anio', OLD.anio, 'mes', OLD.mes, 'monto_bruto', OLD.monto_bruto, 'base_aguinaldo', OLD.base_aguinaldo, 'base_cesantia', OLD.base_cesantia, 'base_vacaciones', OLD.base_vacaciones, 'fuente', OLD.fuente, 'cargado_por_usuario_id', OLD.cargado_por_usuario_id, 'fecha_carga', OLD.fecha_carga), NULL);
END;

