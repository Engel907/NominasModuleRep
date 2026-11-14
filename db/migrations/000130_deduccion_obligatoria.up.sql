ALTER TABLE conceptos_salariales
  ADD COLUMN es_deduccion_obligatoria TINYINT(1) NOT NULL DEFAULT 0 COMMENT '1 = se aplica antes que las voluntarias y no se topa por neto' AFTER afecta_base_embargo;

UPDATE conceptos_salariales SET es_deduccion_obligatoria = 1 WHERE codigo IN ('CCSS_OBR', 'IMP_RENTA', 'EMBARGO');

DROP TRIGGER IF EXISTS trg_aud_conceptos_salariales_ins;
CREATE TRIGGER trg_aud_conceptos_salariales_ins AFTER INSERT ON conceptos_salariales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('conceptos_salariales', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'naturaleza', NEW.naturaleza, 'tipo_calculo', NEW.tipo_calculo, 'afecta_base_ccss', NEW.afecta_base_ccss, 'afecta_base_renta', NEW.afecta_base_renta, 'afecta_base_aguinaldo', NEW.afecta_base_aguinaldo, 'afecta_base_cesantia', NEW.afecta_base_cesantia, 'afecta_base_vacaciones', NEW.afecta_base_vacaciones, 'afecta_base_embargo', NEW.afecta_base_embargo, 'es_deduccion_obligatoria', NEW.es_deduccion_obligatoria, 'activo', NEW.activo, 'created_at', NEW.created_at));
END;

DROP TRIGGER IF EXISTS trg_aud_conceptos_salariales_upd;
CREATE TRIGGER trg_aud_conceptos_salariales_upd AFTER UPDATE ON conceptos_salariales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('conceptos_salariales', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'naturaleza', OLD.naturaleza, 'tipo_calculo', OLD.tipo_calculo, 'afecta_base_ccss', OLD.afecta_base_ccss, 'afecta_base_renta', OLD.afecta_base_renta, 'afecta_base_aguinaldo', OLD.afecta_base_aguinaldo, 'afecta_base_cesantia', OLD.afecta_base_cesantia, 'afecta_base_vacaciones', OLD.afecta_base_vacaciones, 'afecta_base_embargo', OLD.afecta_base_embargo, 'es_deduccion_obligatoria', OLD.es_deduccion_obligatoria, 'activo', OLD.activo, 'created_at', OLD.created_at), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'naturaleza', NEW.naturaleza, 'tipo_calculo', NEW.tipo_calculo, 'afecta_base_ccss', NEW.afecta_base_ccss, 'afecta_base_renta', NEW.afecta_base_renta, 'afecta_base_aguinaldo', NEW.afecta_base_aguinaldo, 'afecta_base_cesantia', NEW.afecta_base_cesantia, 'afecta_base_vacaciones', NEW.afecta_base_vacaciones, 'afecta_base_embargo', NEW.afecta_base_embargo, 'es_deduccion_obligatoria', NEW.es_deduccion_obligatoria, 'activo', NEW.activo, 'created_at', NEW.created_at));
END;

DROP TRIGGER IF EXISTS trg_aud_conceptos_salariales_del;
CREATE TRIGGER trg_aud_conceptos_salariales_del AFTER DELETE ON conceptos_salariales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('conceptos_salariales', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'naturaleza', OLD.naturaleza, 'tipo_calculo', OLD.tipo_calculo, 'afecta_base_ccss', OLD.afecta_base_ccss, 'afecta_base_renta', OLD.afecta_base_renta, 'afecta_base_aguinaldo', OLD.afecta_base_aguinaldo, 'afecta_base_cesantia', OLD.afecta_base_cesantia, 'afecta_base_vacaciones', OLD.afecta_base_vacaciones, 'afecta_base_embargo', OLD.afecta_base_embargo, 'es_deduccion_obligatoria', OLD.es_deduccion_obligatoria, 'activo', OLD.activo, 'created_at', OLD.created_at), NULL);
END;

