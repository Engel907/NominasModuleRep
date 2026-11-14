ALTER TABLE tipos_planilla
  ADD COLUMN clase ENUM('regular','aguinaldo','salario_escolar','liquidacion') NOT NULL DEFAULT 'regular' COMMENT 'Comportamiento del tipo',
  ADD COLUMN fraccion_mensual DECIMAL(6,4) NULL COMMENT 'Fracción del mes que cubre un período (mensual 1.0000, quincenal 0.5000); NULL si no es regular',
  ADD CONSTRAINT chk_tp_fraccion CHECK ((clase = 'regular' AND fraccion_mensual IS NOT NULL AND fraccion_mensual > 0 AND fraccion_mensual <= 1) OR (clase <> 'regular' AND fraccion_mensual IS NULL));

DROP TRIGGER IF EXISTS trg_aud_tipos_planilla_ins;
CREATE TRIGGER trg_aud_tipos_planilla_ins AFTER INSERT ON tipos_planilla
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_planilla', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'clase', NEW.clase, 'fraccion_mensual', NEW.fraccion_mensual));
END;

DROP TRIGGER IF EXISTS trg_aud_tipos_planilla_upd;
CREATE TRIGGER trg_aud_tipos_planilla_upd AFTER UPDATE ON tipos_planilla
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_planilla', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'clase', OLD.clase, 'fraccion_mensual', OLD.fraccion_mensual), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'clase', NEW.clase, 'fraccion_mensual', NEW.fraccion_mensual));
END;

DROP TRIGGER IF EXISTS trg_aud_tipos_planilla_del;
CREATE TRIGGER trg_aud_tipos_planilla_del AFTER DELETE ON tipos_planilla
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_planilla', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'clase', OLD.clase, 'fraccion_mensual', OLD.fraccion_mensual), NULL);
END;

INSERT INTO tipos_planilla (codigo, nombre, clase, fraccion_mensual)
SELECT v.codigo, v.nombre, v.clase, v.fraccion FROM (
  SELECT 'mensual' codigo, 'Planilla mensual' nombre, 'regular' clase, 1.0000 fraccion
  UNION ALL SELECT 'quincenal', 'Planilla quincenal', 'regular', 0.5000
  UNION ALL SELECT 'aguinaldo', 'Aguinaldo', 'aguinaldo', NULL
  UNION ALL SELECT 'salario_escolar', 'Salario escolar', 'salario_escolar', NULL
  UNION ALL SELECT 'liquidacion', 'Liquidación laboral', 'liquidacion', NULL
) v WHERE NOT EXISTS (SELECT 1 FROM tipos_planilla t WHERE t.codigo = v.codigo);

CREATE TABLE tipo_planilla_concepto (
  id                   BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  tipo_planilla_id     BIGINT UNSIGNED NOT NULL COMMENT 'Tipo de planilla',
  concepto_salarial_id BIGINT UNSIGNED NOT NULL COMMENT 'Concepto que ese tipo calcula',
  CONSTRAINT fk_tpc_tipo     FOREIGN KEY (tipo_planilla_id)     REFERENCES tipos_planilla(id),
  CONSTRAINT fk_tpc_concepto FOREIGN KEY (concepto_salarial_id) REFERENCES conceptos_salariales(id),
  UNIQUE KEY uk_tpc (tipo_planilla_id, concepto_salarial_id)
) ENGINE=InnoDB COMMENT='Qué conceptos calcula cada tipo de planilla';

DROP TRIGGER IF EXISTS trg_aud_tipo_planilla_concepto_ins;
CREATE TRIGGER trg_aud_tipo_planilla_concepto_ins AFTER INSERT ON tipo_planilla_concepto
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipo_planilla_concepto', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'tipo_planilla_id', NEW.tipo_planilla_id, 'concepto_salarial_id', NEW.concepto_salarial_id));
END;

DROP TRIGGER IF EXISTS trg_aud_tipo_planilla_concepto_upd;
CREATE TRIGGER trg_aud_tipo_planilla_concepto_upd AFTER UPDATE ON tipo_planilla_concepto
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipo_planilla_concepto', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'tipo_planilla_id', OLD.tipo_planilla_id, 'concepto_salarial_id', OLD.concepto_salarial_id), JSON_OBJECT('id', NEW.id, 'tipo_planilla_id', NEW.tipo_planilla_id, 'concepto_salarial_id', NEW.concepto_salarial_id));
END;

DROP TRIGGER IF EXISTS trg_aud_tipo_planilla_concepto_del;
CREATE TRIGGER trg_aud_tipo_planilla_concepto_del AFTER DELETE ON tipo_planilla_concepto
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipo_planilla_concepto', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'tipo_planilla_id', OLD.tipo_planilla_id, 'concepto_salarial_id', OLD.concepto_salarial_id), NULL);
END;

INSERT INTO tipo_planilla_concepto (tipo_planilla_id, concepto_salarial_id)
SELECT t.id, c.id FROM tipos_planilla t JOIN conceptos_salariales c
 ON c.codigo IN ('SAL_BASE','HRS_EXTRA','COMISIONES','BONO_DISCRECIONAL','VIATICOS','SUB_INCAP','PAGO_VACACIONES',
                 'CCSS_OBR','IMP_RENTA','EMBARGO','PRESTAMO','ASOC_SOLIDARISTA','SINDICATO','CCSS_PAT')
WHERE t.clase = 'regular'
  AND NOT EXISTS (SELECT 1 FROM tipo_planilla_concepto x WHERE x.tipo_planilla_id = t.id AND x.concepto_salarial_id = c.id);