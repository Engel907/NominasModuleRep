ALTER TABLE empleado_datos_laborales
  ADD COLUMN regimen_ccss_id BIGINT UNSIGNED NULL COMMENT 'Régimen CCSS del empleado; sin valor = el cálculo falla' AFTER sindicato_id,
  ADD CONSTRAINT fk_edl_regimen_ccss FOREIGN KEY (regimen_ccss_id) REFERENCES regimenes_ccss(id);

DROP TRIGGER IF EXISTS trg_aud_empleado_datos_laborales_ins;
CREATE TRIGGER trg_aud_empleado_datos_laborales_ins AFTER INSERT ON empleado_datos_laborales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_datos_laborales', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'puesto_id', NEW.puesto_id, 'salario_base_nominal', NEW.salario_base_nominal, 'tipo_contrato_id', NEW.tipo_contrato_id, 'programa_presupuestario_id', NEW.programa_presupuestario_id, 'sindicato_id', NEW.sindicato_id, 'regimen_ccss_id', NEW.regimen_ccss_id, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_datos_laborales_upd;
CREATE TRIGGER trg_aud_empleado_datos_laborales_upd AFTER UPDATE ON empleado_datos_laborales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_datos_laborales', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'puesto_id', OLD.puesto_id, 'salario_base_nominal', OLD.salario_base_nominal, 'tipo_contrato_id', OLD.tipo_contrato_id, 'programa_presupuestario_id', OLD.programa_presupuestario_id, 'sindicato_id', OLD.sindicato_id, 'regimen_ccss_id', OLD.regimen_ccss_id, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'puesto_id', NEW.puesto_id, 'salario_base_nominal', NEW.salario_base_nominal, 'tipo_contrato_id', NEW.tipo_contrato_id, 'programa_presupuestario_id', NEW.programa_presupuestario_id, 'sindicato_id', NEW.sindicato_id, 'regimen_ccss_id', NEW.regimen_ccss_id, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_datos_laborales_del;
CREATE TRIGGER trg_aud_empleado_datos_laborales_del AFTER DELETE ON empleado_datos_laborales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_datos_laborales', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'puesto_id', OLD.puesto_id, 'salario_base_nominal', OLD.salario_base_nominal, 'tipo_contrato_id', OLD.tipo_contrato_id, 'programa_presupuestario_id', OLD.programa_presupuestario_id, 'sindicato_id', OLD.sindicato_id, 'regimen_ccss_id', OLD.regimen_ccss_id, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), NULL);
END;

