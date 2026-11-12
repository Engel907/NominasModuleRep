-- L10 — Salario: fuente única (D5) y tipo de contrato por catálogo.
--
-- 1) contratos.tipo_contrato_id (FK a tipos_contrato) reemplaza al texto
--    contratos.tipo_contrato, que queda DEPRECADO (no se elimina). Backfill por
--    coincidencia de código o nombre. Informe de los que no calzan (no borra):
--      SELECT id, tipo_contrato FROM contratos WHERE tipo_contrato_id IS NULL;
-- 2) empleado_datos_laborales.salario_base_nominal pasa a ser COPIA DERIVADA
--    de contratos.salario_pactado del contrato vigente, mantenida por la BD.
--    El cálculo de planilla nunca la lee (L10: segmentos de contrato).
-- 3) contratos no tenía auditoría: se crea con todas sus columnas.
ALTER TABLE contratos
  ADD COLUMN tipo_contrato_id BIGINT UNSIGNED NULL COMMENT 'FK a tipos_contrato; reemplaza tipo_contrato (texto, deprecado)',
  ADD CONSTRAINT fk_contratos_tipo FOREIGN KEY (tipo_contrato_id) REFERENCES tipos_contrato(id);

UPDATE contratos c
JOIN tipos_contrato t ON UPPER(t.codigo) = UPPER(c.tipo_contrato) OR UPPER(t.nombre) = UPPER(c.tipo_contrato)
   SET c.tipo_contrato_id = t.id
 WHERE c.tipo_contrato_id IS NULL;

-- Copia derivada al crear datos laborales: toma el salario del contrato vigente.
CREATE TRIGGER trg_edl_salario_derivado_bi BEFORE INSERT ON empleado_datos_laborales
FOR EACH ROW
BEGIN
  DECLARE v_sal DECIMAL(14,2) DEFAULT NULL;
  SELECT salario_pactado INTO v_sal FROM contratos
   WHERE empleado_id = NEW.empleado_id AND fecha_fin_vigencia IS NULL
   ORDER BY fecha_inicio_vigencia DESC LIMIT 1;
  IF v_sal IS NOT NULL THEN
    SET NEW.salario_base_nominal = v_sal;
  END IF;
END;

-- Copia derivada al abrir una versión de contrato vigente.
CREATE TRIGGER trg_contratos_sync_salario_ai AFTER INSERT ON contratos
FOR EACH ROW
BEGIN
  IF NEW.fecha_fin_vigencia IS NULL THEN
    UPDATE empleado_datos_laborales SET salario_base_nominal = NEW.salario_pactado
     WHERE empleado_id = NEW.empleado_id AND fecha_fin_vigencia IS NULL;
  END IF;
END;

-- Auditoría de contratos (scripts/gen_auditoria.sh).
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
