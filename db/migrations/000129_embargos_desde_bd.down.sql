DROP TRIGGER IF EXISTS trg_aud_embargos_ins;
CREATE TRIGGER trg_aud_embargos_ins AFTER INSERT ON embargos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('embargos', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'tercero_id', NEW.tercero_id, 'tipo_embargo_id', NEW.tipo_embargo_id, 'numero_expediente', NEW.numero_expediente, 'porcentaje', NEW.porcentaje, 'monto_fijo', NEW.monto_fijo, 'prioridad_aplicacion', NEW.prioridad_aplicacion, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'estado', NEW.estado));
END;

DROP TRIGGER IF EXISTS trg_aud_embargos_upd;
CREATE TRIGGER trg_aud_embargos_upd AFTER UPDATE ON embargos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('embargos', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'tercero_id', OLD.tercero_id, 'tipo_embargo_id', OLD.tipo_embargo_id, 'numero_expediente', OLD.numero_expediente, 'porcentaje', OLD.porcentaje, 'monto_fijo', OLD.monto_fijo, 'prioridad_aplicacion', OLD.prioridad_aplicacion, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'estado', OLD.estado), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'tercero_id', NEW.tercero_id, 'tipo_embargo_id', NEW.tipo_embargo_id, 'numero_expediente', NEW.numero_expediente, 'porcentaje', NEW.porcentaje, 'monto_fijo', NEW.monto_fijo, 'prioridad_aplicacion', NEW.prioridad_aplicacion, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'estado', NEW.estado));
END;

DROP TRIGGER IF EXISTS trg_aud_embargos_del;
CREATE TRIGGER trg_aud_embargos_del AFTER DELETE ON embargos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('embargos', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'tercero_id', OLD.tercero_id, 'tipo_embargo_id', OLD.tipo_embargo_id, 'numero_expediente', OLD.numero_expediente, 'porcentaje', OLD.porcentaje, 'monto_fijo', OLD.monto_fijo, 'prioridad_aplicacion', OLD.prioridad_aplicacion, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'estado', OLD.estado), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_conceptos_salariales_ins;
CREATE TRIGGER trg_aud_conceptos_salariales_ins AFTER INSERT ON conceptos_salariales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('conceptos_salariales', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'naturaleza', NEW.naturaleza, 'tipo_calculo', NEW.tipo_calculo, 'afecta_base_ccss', NEW.afecta_base_ccss, 'afecta_base_renta', NEW.afecta_base_renta, 'afecta_base_aguinaldo', NEW.afecta_base_aguinaldo, 'afecta_base_cesantia', NEW.afecta_base_cesantia, 'afecta_base_vacaciones', NEW.afecta_base_vacaciones, 'activo', NEW.activo, 'created_at', NEW.created_at));
END;

DROP TRIGGER IF EXISTS trg_aud_conceptos_salariales_upd;
CREATE TRIGGER trg_aud_conceptos_salariales_upd AFTER UPDATE ON conceptos_salariales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('conceptos_salariales', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'naturaleza', OLD.naturaleza, 'tipo_calculo', OLD.tipo_calculo, 'afecta_base_ccss', OLD.afecta_base_ccss, 'afecta_base_renta', OLD.afecta_base_renta, 'afecta_base_aguinaldo', OLD.afecta_base_aguinaldo, 'afecta_base_cesantia', OLD.afecta_base_cesantia, 'afecta_base_vacaciones', OLD.afecta_base_vacaciones, 'activo', OLD.activo, 'created_at', OLD.created_at), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'naturaleza', NEW.naturaleza, 'tipo_calculo', NEW.tipo_calculo, 'afecta_base_ccss', NEW.afecta_base_ccss, 'afecta_base_renta', NEW.afecta_base_renta, 'afecta_base_aguinaldo', NEW.afecta_base_aguinaldo, 'afecta_base_cesantia', NEW.afecta_base_cesantia, 'afecta_base_vacaciones', NEW.afecta_base_vacaciones, 'activo', NEW.activo, 'created_at', NEW.created_at));
END;

DROP TRIGGER IF EXISTS trg_aud_conceptos_salariales_del;
CREATE TRIGGER trg_aud_conceptos_salariales_del AFTER DELETE ON conceptos_salariales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('conceptos_salariales', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'naturaleza', OLD.naturaleza, 'tipo_calculo', OLD.tipo_calculo, 'afecta_base_ccss', OLD.afecta_base_ccss, 'afecta_base_renta', OLD.afecta_base_renta, 'afecta_base_aguinaldo', OLD.afecta_base_aguinaldo, 'afecta_base_cesantia', OLD.afecta_base_cesantia, 'afecta_base_vacaciones', OLD.afecta_base_vacaciones, 'activo', OLD.activo, 'created_at', OLD.created_at), NULL);
END;

ALTER TABLE conceptos_salariales DROP COLUMN afecta_base_embargo;

ALTER TABLE embargos
  DROP CHECK chk_emb_tope_mensual,
  DROP COLUMN tope_mensual;
