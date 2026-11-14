ALTER TABLE contratos
  ADD COLUMN tipo_contrato VARCHAR(30) NULL COMMENT 'Modalidad de contratación (p.ej. indefinido, plazo_fijo, interino); string libre' AFTER regimen_laboral_id,
  ADD COLUMN jornada VARCHAR(30) NULL COMMENT 'Jornada laboral pactada en el contrato (p.ej. completa, media_jornada)' AFTER salario_pactado;

UPDATE contratos c JOIN tipos_contrato t ON t.id = c.tipo_contrato_id SET c.tipo_contrato = t.codigo;
UPDATE contratos c JOIN jornadas j ON j.id = c.jornada_id SET c.jornada = j.codigo;

ALTER TABLE contratos
  DROP FOREIGN KEY fk_contratos_tipo,
  DROP FOREIGN KEY fk_contratos_jornada;

ALTER TABLE contratos
  MODIFY tipo_contrato VARCHAR(30) NOT NULL COMMENT 'Modalidad de contratación (p.ej. indefinido, plazo_fijo, interino); string libre',
  MODIFY jornada VARCHAR(30) NOT NULL COMMENT 'Jornada laboral pactada en el contrato (p.ej. completa, media_jornada)',
  MODIFY tipo_contrato_id BIGINT UNSIGNED NULL COMMENT 'FK a tipos_contrato',
  MODIFY jornada_id BIGINT UNSIGNED NULL COMMENT 'FK a jornadas',
  ADD KEY ix_contrato_tipo (tipo_contrato),
  ADD CONSTRAINT fk_contratos_tipo FOREIGN KEY (tipo_contrato_id) REFERENCES tipos_contrato(id),
  ADD CONSTRAINT fk_contratos_jornada FOREIGN KEY (jornada_id) REFERENCES jornadas(id);

CREATE TRIGGER trg_contratos_catalogos_bi BEFORE INSERT ON contratos
FOR EACH ROW
BEGIN
  IF NEW.jornada_id IS NULL THEN
    SET NEW.jornada_id = (SELECT j.id FROM jornadas j WHERE UPPER(j.codigo) = UPPER(NEW.jornada));
  END IF;
  IF NEW.tipo_contrato_id IS NULL THEN
    SET NEW.tipo_contrato_id = (SELECT t.id FROM tipos_contrato t
                                 WHERE UPPER(t.codigo) = UPPER(NEW.tipo_contrato) OR UPPER(t.nombre) = UPPER(NEW.tipo_contrato)
                                 ORDER BY t.id LIMIT 1);
  END IF;
END;

DROP TRIGGER IF EXISTS trg_aud_contratos_ins;
CREATE TRIGGER trg_aud_contratos_ins AFTER INSERT ON contratos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('contratos', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'puesto_id', NEW.puesto_id, 'regimen_laboral_id', NEW.regimen_laboral_id, 'tipo_contrato', NEW.tipo_contrato, 'tipo_contrato_id', NEW.tipo_contrato_id, 'salario_pactado', NEW.salario_pactado, 'jornada', NEW.jornada, 'jornada_id', NEW.jornada_id, 'fecha_inicio_contrato', NEW.fecha_inicio_contrato, 'fecha_fin_contrato', NEW.fecha_fin_contrato, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'creado_por', NEW.creado_por));
END;

DROP TRIGGER IF EXISTS trg_aud_contratos_upd;
CREATE TRIGGER trg_aud_contratos_upd AFTER UPDATE ON contratos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('contratos', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'puesto_id', OLD.puesto_id, 'regimen_laboral_id', OLD.regimen_laboral_id, 'tipo_contrato', OLD.tipo_contrato, 'tipo_contrato_id', OLD.tipo_contrato_id, 'salario_pactado', OLD.salario_pactado, 'jornada', OLD.jornada, 'jornada_id', OLD.jornada_id, 'fecha_inicio_contrato', OLD.fecha_inicio_contrato, 'fecha_fin_contrato', OLD.fecha_fin_contrato, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'creado_por', OLD.creado_por), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'puesto_id', NEW.puesto_id, 'regimen_laboral_id', NEW.regimen_laboral_id, 'tipo_contrato', NEW.tipo_contrato, 'tipo_contrato_id', NEW.tipo_contrato_id, 'salario_pactado', NEW.salario_pactado, 'jornada', NEW.jornada, 'jornada_id', NEW.jornada_id, 'fecha_inicio_contrato', NEW.fecha_inicio_contrato, 'fecha_fin_contrato', NEW.fecha_fin_contrato, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'creado_por', NEW.creado_por));
END;

DROP TRIGGER IF EXISTS trg_aud_contratos_del;
CREATE TRIGGER trg_aud_contratos_del AFTER DELETE ON contratos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('contratos', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'puesto_id', OLD.puesto_id, 'regimen_laboral_id', OLD.regimen_laboral_id, 'tipo_contrato', OLD.tipo_contrato, 'tipo_contrato_id', OLD.tipo_contrato_id, 'salario_pactado', OLD.salario_pactado, 'jornada', OLD.jornada, 'jornada_id', OLD.jornada_id, 'fecha_inicio_contrato', OLD.fecha_inicio_contrato, 'fecha_fin_contrato', OLD.fecha_fin_contrato, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'creado_por', OLD.creado_por), NULL);
END;

