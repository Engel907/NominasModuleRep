DROP TRIGGER trg_parametros_motivos_salida_inmut_bu;
DROP TRIGGER trg_parametros_motivos_salida_inmut_bd;
DROP TRIGGER trg_parametros_cesantia_escala_inmut_bu;
DROP TRIGGER trg_parametros_cesantia_escala_inmut_bd;
DROP TRIGGER trg_parametros_preaviso_escala_inmut_bu;
DROP TRIGGER trg_parametros_preaviso_escala_inmut_bd;
DROP TRIGGER trg_aud_parametros_motivos_salida_ins;
DROP TRIGGER trg_aud_parametros_motivos_salida_upd;
DROP TRIGGER trg_aud_parametros_motivos_salida_del;
DROP TRIGGER trg_aud_parametros_preaviso_escala_ins;
DROP TRIGGER trg_aud_parametros_preaviso_escala_upd;
DROP TRIGGER trg_aud_parametros_preaviso_escala_del;

ALTER TABLE liquidaciones
  DROP COLUMN salario_diario_vacaciones,
  DROP COLUMN periodo_salario_escolar_desde,
  DROP COLUMN total_base_salario_escolar_proporcional,
  DROP COLUMN monto_salario_escolar_proporcional;

DROP TRIGGER trg_parametros_generales_inmut_bd;
DELETE FROM parametros_generales WHERE clave IN ('LIQ_PROMEDIO_MESES', 'VENTANA_PROMEDIO_VACACIONES_SEMANAS');
CREATE TRIGGER trg_parametros_generales_inmut_bd BEFORE DELETE ON parametros_generales
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_generales no se borra; se cierra la vigencia';
END;

DELETE FROM parametros_cesantia_escala WHERE anios_servicio_hasta IS NOT NULL AND anios_servicio_hasta <= 1.00;
ALTER TABLE parametros_cesantia_escala DROP COLUMN fuente_legal, DROP COLUMN creado_por;
ALTER TABLE parametros_preaviso_escala DROP COLUMN fuente_legal;

DROP TRIGGER IF EXISTS trg_aud_parametros_cesantia_escala_ins;
CREATE TRIGGER trg_aud_parametros_cesantia_escala_ins AFTER INSERT ON parametros_cesantia_escala
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_cesantia_escala', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'anios_servicio_desde', NEW.anios_servicio_desde, 'anios_servicio_hasta', NEW.anios_servicio_hasta, 'dias_por_anio', NEW.dias_por_anio, 'tope_anios', NEW.tope_anios, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_cesantia_escala_upd;
CREATE TRIGGER trg_aud_parametros_cesantia_escala_upd AFTER UPDATE ON parametros_cesantia_escala
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_cesantia_escala', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'anios_servicio_desde', OLD.anios_servicio_desde, 'anios_servicio_hasta', OLD.anios_servicio_hasta, 'dias_por_anio', OLD.dias_por_anio, 'tope_anios', OLD.tope_anios, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), JSON_OBJECT('id', NEW.id, 'anios_servicio_desde', NEW.anios_servicio_desde, 'anios_servicio_hasta', NEW.anios_servicio_hasta, 'dias_por_anio', NEW.dias_por_anio, 'tope_anios', NEW.tope_anios, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_cesantia_escala_del;
CREATE TRIGGER trg_aud_parametros_cesantia_escala_del AFTER DELETE ON parametros_cesantia_escala
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_cesantia_escala', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'anios_servicio_desde', OLD.anios_servicio_desde, 'anios_servicio_hasta', OLD.anios_servicio_hasta, 'dias_por_anio', OLD.dias_por_anio, 'tope_anios', OLD.tope_anios, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), NULL);
END;

DROP TABLE parametros_motivos_salida;
