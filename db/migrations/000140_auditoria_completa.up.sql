CREATE TRIGGER trg_aud_calendario_laboral_dia_ins AFTER INSERT ON calendario_laboral_dia
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('calendario_laboral_dia', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'fecha', NEW.fecha, 'anio', NEW.anio, 'es_habil', NEW.es_habil, 'es_feriado', NEW.es_feriado, 'feriado_id', NEW.feriado_id, 'dia_semana', NEW.dia_semana, 'generado_en', NEW.generado_en));
END;

CREATE TRIGGER trg_aud_calendario_laboral_dia_upd AFTER UPDATE ON calendario_laboral_dia
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('calendario_laboral_dia', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'fecha', OLD.fecha, 'anio', OLD.anio, 'es_habil', OLD.es_habil, 'es_feriado', OLD.es_feriado, 'feriado_id', OLD.feriado_id, 'dia_semana', OLD.dia_semana, 'generado_en', OLD.generado_en), JSON_OBJECT('id', NEW.id, 'fecha', NEW.fecha, 'anio', NEW.anio, 'es_habil', NEW.es_habil, 'es_feriado', NEW.es_feriado, 'feriado_id', NEW.feriado_id, 'dia_semana', NEW.dia_semana, 'generado_en', NEW.generado_en));
END;

CREATE TRIGGER trg_aud_calendario_laboral_dia_del AFTER DELETE ON calendario_laboral_dia
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('calendario_laboral_dia', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'fecha', OLD.fecha, 'anio', OLD.anio, 'es_habil', OLD.es_habil, 'es_feriado', OLD.es_feriado, 'feriado_id', OLD.feriado_id, 'dia_semana', OLD.dia_semana, 'generado_en', OLD.generado_en), NULL);
END;

CREATE TRIGGER trg_aud_feriados_ins AFTER INSERT ON feriados
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('feriados', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'fecha', NEW.fecha, 'nombre', NEW.nombre, 'tipo', NEW.tipo, 'pago_obligatorio', NEW.pago_obligatorio, 'anio', NEW.anio, 'created_at', NEW.created_at));
END;

CREATE TRIGGER trg_aud_feriados_upd AFTER UPDATE ON feriados
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('feriados', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'fecha', OLD.fecha, 'nombre', OLD.nombre, 'tipo', OLD.tipo, 'pago_obligatorio', OLD.pago_obligatorio, 'anio', OLD.anio, 'created_at', OLD.created_at), JSON_OBJECT('id', NEW.id, 'fecha', NEW.fecha, 'nombre', NEW.nombre, 'tipo', NEW.tipo, 'pago_obligatorio', NEW.pago_obligatorio, 'anio', NEW.anio, 'created_at', NEW.created_at));
END;

CREATE TRIGGER trg_aud_feriados_del AFTER DELETE ON feriados
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('feriados', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'fecha', OLD.fecha, 'nombre', OLD.nombre, 'tipo', OLD.tipo, 'pago_obligatorio', OLD.pago_obligatorio, 'anio', OLD.anio, 'created_at', OLD.created_at), NULL);
END;

CREATE TRIGGER trg_aud_incapacidades_ins AFTER INSERT ON incapacidades
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('incapacidades', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'entidad_emisora', NEW.entidad_emisora, 'numero_boleta', NEW.numero_boleta, 'fecha_inicio', NEW.fecha_inicio, 'fecha_fin', NEW.fecha_fin, 'dias', NEW.dias, 'diagnostico_codigo', IF(NEW.diagnostico_codigo IS NULL, NULL, '***'), 'observaciones', NEW.observaciones, 'creado_por', NEW.creado_por, 'created_at', NEW.created_at, 'updated_at', NEW.updated_at));
END;

CREATE TRIGGER trg_aud_incapacidades_upd AFTER UPDATE ON incapacidades
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('incapacidades', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'entidad_emisora', OLD.entidad_emisora, 'numero_boleta', OLD.numero_boleta, 'fecha_inicio', OLD.fecha_inicio, 'fecha_fin', OLD.fecha_fin, 'dias', OLD.dias, 'diagnostico_codigo', IF(OLD.diagnostico_codigo IS NULL, NULL, '***'), 'observaciones', OLD.observaciones, 'creado_por', OLD.creado_por, 'created_at', OLD.created_at, 'updated_at', OLD.updated_at), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'entidad_emisora', NEW.entidad_emisora, 'numero_boleta', NEW.numero_boleta, 'fecha_inicio', NEW.fecha_inicio, 'fecha_fin', NEW.fecha_fin, 'dias', NEW.dias, 'diagnostico_codigo', IF(NEW.diagnostico_codigo IS NULL, NULL, '***'), 'observaciones', NEW.observaciones, 'creado_por', NEW.creado_por, 'created_at', NEW.created_at, 'updated_at', NEW.updated_at));
END;

CREATE TRIGGER trg_aud_incapacidades_del AFTER DELETE ON incapacidades
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('incapacidades', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'entidad_emisora', OLD.entidad_emisora, 'numero_boleta', OLD.numero_boleta, 'fecha_inicio', OLD.fecha_inicio, 'fecha_fin', OLD.fecha_fin, 'dias', OLD.dias, 'diagnostico_codigo', IF(OLD.diagnostico_codigo IS NULL, NULL, '***'), 'observaciones', OLD.observaciones, 'creado_por', OLD.creado_por, 'created_at', OLD.created_at, 'updated_at', OLD.updated_at), NULL);
END;

CREATE TRIGGER trg_aud_jornadas_ins AFTER INSERT ON jornadas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('jornadas', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'activo', NEW.activo));
END;

CREATE TRIGGER trg_aud_jornadas_upd AFTER UPDATE ON jornadas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('jornadas', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'activo', OLD.activo), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'activo', NEW.activo));
END;

CREATE TRIGGER trg_aud_jornadas_del AFTER DELETE ON jornadas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('jornadas', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'activo', OLD.activo), NULL);
END;

CREATE TRIGGER trg_aud_parametros_embargo_escala_ins AFTER INSERT ON parametros_embargo_escala
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_embargo_escala', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'desde_multiplo', NEW.desde_multiplo, 'hasta_multiplo', NEW.hasta_multiplo, 'porcentaje_maximo', NEW.porcentaje_maximo, 'fuente_legal', NEW.fuente_legal, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente, 'creado_por', NEW.creado_por, 'created_at', NEW.created_at));
END;

CREATE TRIGGER trg_aud_parametros_embargo_escala_upd AFTER UPDATE ON parametros_embargo_escala
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_embargo_escala', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'desde_multiplo', OLD.desde_multiplo, 'hasta_multiplo', OLD.hasta_multiplo, 'porcentaje_maximo', OLD.porcentaje_maximo, 'fuente_legal', OLD.fuente_legal, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente, 'creado_por', OLD.creado_por, 'created_at', OLD.created_at), JSON_OBJECT('id', NEW.id, 'desde_multiplo', NEW.desde_multiplo, 'hasta_multiplo', NEW.hasta_multiplo, 'porcentaje_maximo', NEW.porcentaje_maximo, 'fuente_legal', NEW.fuente_legal, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente, 'creado_por', NEW.creado_por, 'created_at', NEW.created_at));
END;

CREATE TRIGGER trg_aud_parametros_embargo_escala_del AFTER DELETE ON parametros_embargo_escala
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_embargo_escala', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'desde_multiplo', OLD.desde_multiplo, 'hasta_multiplo', OLD.hasta_multiplo, 'porcentaje_maximo', OLD.porcentaje_maximo, 'fuente_legal', OLD.fuente_legal, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente, 'creado_por', OLD.creado_por, 'created_at', OLD.created_at), NULL);
END;

CREATE TRIGGER trg_aud_parametros_generales_ins AFTER INSERT ON parametros_generales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_generales', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'clave', NEW.clave, 'valor', NEW.valor, 'tipo', NEW.tipo, 'descripcion', NEW.descripcion, 'fuente_legal', NEW.fuente_legal, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente, 'creado_por', NEW.creado_por, 'created_at', NEW.created_at));
END;

CREATE TRIGGER trg_aud_parametros_generales_upd AFTER UPDATE ON parametros_generales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_generales', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'clave', OLD.clave, 'valor', OLD.valor, 'tipo', OLD.tipo, 'descripcion', OLD.descripcion, 'fuente_legal', OLD.fuente_legal, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente, 'creado_por', OLD.creado_por, 'created_at', OLD.created_at), JSON_OBJECT('id', NEW.id, 'clave', NEW.clave, 'valor', NEW.valor, 'tipo', NEW.tipo, 'descripcion', NEW.descripcion, 'fuente_legal', NEW.fuente_legal, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente, 'creado_por', NEW.creado_por, 'created_at', NEW.created_at));
END;

CREATE TRIGGER trg_aud_parametros_generales_del AFTER DELETE ON parametros_generales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_generales', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'clave', OLD.clave, 'valor', OLD.valor, 'tipo', OLD.tipo, 'descripcion', OLD.descripcion, 'fuente_legal', OLD.fuente_legal, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente, 'creado_por', OLD.creado_por, 'created_at', OLD.created_at), NULL);
END;

CREATE TRIGGER trg_aud_parametros_salario_escolar_ins AFTER INSERT ON parametros_salario_escolar
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_salario_escolar', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'porcentaje', NEW.porcentaje, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente, 'creado_por', NEW.creado_por, 'created_at', NEW.created_at));
END;

CREATE TRIGGER trg_aud_parametros_salario_escolar_upd AFTER UPDATE ON parametros_salario_escolar
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_salario_escolar', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'porcentaje', OLD.porcentaje, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente, 'creado_por', OLD.creado_por, 'created_at', OLD.created_at), JSON_OBJECT('id', NEW.id, 'porcentaje', NEW.porcentaje, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente, 'creado_por', NEW.creado_por, 'created_at', NEW.created_at));
END;

CREATE TRIGGER trg_aud_parametros_salario_escolar_del AFTER DELETE ON parametros_salario_escolar
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_salario_escolar', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'porcentaje', OLD.porcentaje, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente, 'creado_por', OLD.creado_por, 'created_at', OLD.created_at), NULL);
END;

CREATE TRIGGER trg_aud_planilla_detalle_embargo_ins AFTER INSERT ON planilla_detalle_embargo
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_embargo', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'planilla_detalle_empleado_id', NEW.planilla_detalle_empleado_id, 'embargo_id', NEW.embargo_id, 'orden_prioridad', NEW.orden_prioridad, 'monto', NEW.monto, 'monto_calculado', NEW.monto_calculado, 'monto_insatisfecho', NEW.monto_insatisfecho));
END;

CREATE TRIGGER trg_aud_planilla_detalle_embargo_upd AFTER UPDATE ON planilla_detalle_embargo
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_embargo', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'planilla_detalle_empleado_id', OLD.planilla_detalle_empleado_id, 'embargo_id', OLD.embargo_id, 'orden_prioridad', OLD.orden_prioridad, 'monto', OLD.monto, 'monto_calculado', OLD.monto_calculado, 'monto_insatisfecho', OLD.monto_insatisfecho), JSON_OBJECT('id', NEW.id, 'planilla_detalle_empleado_id', NEW.planilla_detalle_empleado_id, 'embargo_id', NEW.embargo_id, 'orden_prioridad', NEW.orden_prioridad, 'monto', NEW.monto, 'monto_calculado', NEW.monto_calculado, 'monto_insatisfecho', NEW.monto_insatisfecho));
END;

CREATE TRIGGER trg_aud_planilla_detalle_embargo_del AFTER DELETE ON planilla_detalle_embargo
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_embargo', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'planilla_detalle_empleado_id', OLD.planilla_detalle_empleado_id, 'embargo_id', OLD.embargo_id, 'orden_prioridad', OLD.orden_prioridad, 'monto', OLD.monto, 'monto_calculado', OLD.monto_calculado, 'monto_insatisfecho', OLD.monto_insatisfecho), NULL);
END;

CREATE TRIGGER trg_aud_planilla_detalle_incapacidad_ins AFTER INSERT ON planilla_detalle_incapacidad
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_incapacidad', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'planilla_detalle_empleado_id', NEW.planilla_detalle_empleado_id, 'incapacidad_id', NEW.incapacidad_id, 'dia_serie_desde', NEW.dia_serie_desde, 'dia_serie_hasta', NEW.dia_serie_hasta, 'dias', NEW.dias, 'monto_patronal', NEW.monto_patronal, 'monto_entidad_informativo', NEW.monto_entidad_informativo));
END;

CREATE TRIGGER trg_aud_planilla_detalle_incapacidad_upd AFTER UPDATE ON planilla_detalle_incapacidad
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_incapacidad', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'planilla_detalle_empleado_id', OLD.planilla_detalle_empleado_id, 'incapacidad_id', OLD.incapacidad_id, 'dia_serie_desde', OLD.dia_serie_desde, 'dia_serie_hasta', OLD.dia_serie_hasta, 'dias', OLD.dias, 'monto_patronal', OLD.monto_patronal, 'monto_entidad_informativo', OLD.monto_entidad_informativo), JSON_OBJECT('id', NEW.id, 'planilla_detalle_empleado_id', NEW.planilla_detalle_empleado_id, 'incapacidad_id', NEW.incapacidad_id, 'dia_serie_desde', NEW.dia_serie_desde, 'dia_serie_hasta', NEW.dia_serie_hasta, 'dias', NEW.dias, 'monto_patronal', NEW.monto_patronal, 'monto_entidad_informativo', NEW.monto_entidad_informativo));
END;

CREATE TRIGGER trg_aud_planilla_detalle_incapacidad_del AFTER DELETE ON planilla_detalle_incapacidad
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_incapacidad', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'planilla_detalle_empleado_id', OLD.planilla_detalle_empleado_id, 'incapacidad_id', OLD.incapacidad_id, 'dia_serie_desde', OLD.dia_serie_desde, 'dia_serie_hasta', OLD.dia_serie_hasta, 'dias', OLD.dias, 'monto_patronal', OLD.monto_patronal, 'monto_entidad_informativo', OLD.monto_entidad_informativo), NULL);
END;

CREATE TRIGGER trg_aud_planilla_detalle_patronal_ins AFTER INSERT ON planilla_detalle_patronal
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_patronal', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'planilla_detalle_empleado_id', NEW.planilla_detalle_empleado_id, 'concepto_salarial_id', NEW.concepto_salarial_id, 'base', NEW.base, 'porcentaje', NEW.porcentaje, 'monto', NEW.monto));
END;

CREATE TRIGGER trg_aud_planilla_detalle_patronal_upd AFTER UPDATE ON planilla_detalle_patronal
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_patronal', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'planilla_detalle_empleado_id', OLD.planilla_detalle_empleado_id, 'concepto_salarial_id', OLD.concepto_salarial_id, 'base', OLD.base, 'porcentaje', OLD.porcentaje, 'monto', OLD.monto), JSON_OBJECT('id', NEW.id, 'planilla_detalle_empleado_id', NEW.planilla_detalle_empleado_id, 'concepto_salarial_id', NEW.concepto_salarial_id, 'base', NEW.base, 'porcentaje', NEW.porcentaje, 'monto', NEW.monto));
END;

CREATE TRIGGER trg_aud_planilla_detalle_patronal_del AFTER DELETE ON planilla_detalle_patronal
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_patronal', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'planilla_detalle_empleado_id', OLD.planilla_detalle_empleado_id, 'concepto_salarial_id', OLD.concepto_salarial_id, 'base', OLD.base, 'porcentaje', OLD.porcentaje, 'monto', OLD.monto), NULL);
END;

CREATE TRIGGER trg_aud_planilla_detalle_prestamo_ins AFTER INSERT ON planilla_detalle_prestamo
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_prestamo', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'planilla_detalle_empleado_id', NEW.planilla_detalle_empleado_id, 'prestamo_id', NEW.prestamo_id, 'cuota_calculada', NEW.cuota_calculada, 'monto_retenido', NEW.monto_retenido, 'aplicado_en', NEW.aplicado_en));
END;

CREATE TRIGGER trg_aud_planilla_detalle_prestamo_upd AFTER UPDATE ON planilla_detalle_prestamo
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_prestamo', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'planilla_detalle_empleado_id', OLD.planilla_detalle_empleado_id, 'prestamo_id', OLD.prestamo_id, 'cuota_calculada', OLD.cuota_calculada, 'monto_retenido', OLD.monto_retenido, 'aplicado_en', OLD.aplicado_en), JSON_OBJECT('id', NEW.id, 'planilla_detalle_empleado_id', NEW.planilla_detalle_empleado_id, 'prestamo_id', NEW.prestamo_id, 'cuota_calculada', NEW.cuota_calculada, 'monto_retenido', NEW.monto_retenido, 'aplicado_en', NEW.aplicado_en));
END;

CREATE TRIGGER trg_aud_planilla_detalle_prestamo_del AFTER DELETE ON planilla_detalle_prestamo
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_prestamo', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'planilla_detalle_empleado_id', OLD.planilla_detalle_empleado_id, 'prestamo_id', OLD.prestamo_id, 'cuota_calculada', OLD.cuota_calculada, 'monto_retenido', OLD.monto_retenido, 'aplicado_en', OLD.aplicado_en), NULL);
END;

CREATE TRIGGER trg_aud_regimen_concepto_aplicable_ins AFTER INSERT ON regimen_concepto_aplicable
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('regimen_concepto_aplicable', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'regimen_laboral_id', NEW.regimen_laboral_id, 'concepto_salarial_id', NEW.concepto_salarial_id, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

CREATE TRIGGER trg_aud_regimen_concepto_aplicable_upd AFTER UPDATE ON regimen_concepto_aplicable
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('regimen_concepto_aplicable', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'regimen_laboral_id', OLD.regimen_laboral_id, 'concepto_salarial_id', OLD.concepto_salarial_id, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), JSON_OBJECT('id', NEW.id, 'regimen_laboral_id', NEW.regimen_laboral_id, 'concepto_salarial_id', NEW.concepto_salarial_id, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

CREATE TRIGGER trg_aud_regimen_concepto_aplicable_del AFTER DELETE ON regimen_concepto_aplicable
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('regimen_concepto_aplicable', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'regimen_laboral_id', OLD.regimen_laboral_id, 'concepto_salarial_id', OLD.concepto_salarial_id, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), NULL);
END;

CREATE TRIGGER trg_aud_regimenes_laborales_ins AFTER INSERT ON regimenes_laborales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('regimenes_laborales', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'descripcion', NEW.descripcion, 'activo', NEW.activo));
END;

CREATE TRIGGER trg_aud_regimenes_laborales_upd AFTER UPDATE ON regimenes_laborales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('regimenes_laborales', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'descripcion', OLD.descripcion, 'activo', OLD.activo), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'descripcion', NEW.descripcion, 'activo', NEW.activo));
END;

CREATE TRIGGER trg_aud_regimenes_laborales_del AFTER DELETE ON regimenes_laborales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('regimenes_laborales', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'descripcion', OLD.descripcion, 'activo', OLD.activo), NULL);
END;

CREATE TRIGGER trg_aud_registro_asistencia_detalle_ins AFTER INSERT ON registro_asistencia_detalle
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('registro_asistencia_detalle', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'turno_id', NEW.turno_id, 'fecha', NEW.fecha, 'hora_entrada_real', NEW.hora_entrada_real, 'hora_salida_real', NEW.hora_salida_real, 'horas_ordinarias', NEW.horas_ordinarias, 'horas_extra', NEW.horas_extra, 'horas_tardanza', NEW.horas_tardanza, 'minutos_tardanza', NEW.minutos_tardanza, 'estado', NEW.estado, 'tipo_ausencia_id', NEW.tipo_ausencia_id, 'incapacidad_id', NEW.incapacidad_id, 'observaciones', NEW.observaciones, 'creado_por', NEW.creado_por, 'created_at', NEW.created_at, 'updated_at', NEW.updated_at));
END;

CREATE TRIGGER trg_aud_registro_asistencia_detalle_upd AFTER UPDATE ON registro_asistencia_detalle
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('registro_asistencia_detalle', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'turno_id', OLD.turno_id, 'fecha', OLD.fecha, 'hora_entrada_real', OLD.hora_entrada_real, 'hora_salida_real', OLD.hora_salida_real, 'horas_ordinarias', OLD.horas_ordinarias, 'horas_extra', OLD.horas_extra, 'horas_tardanza', OLD.horas_tardanza, 'minutos_tardanza', OLD.minutos_tardanza, 'estado', OLD.estado, 'tipo_ausencia_id', OLD.tipo_ausencia_id, 'incapacidad_id', OLD.incapacidad_id, 'observaciones', OLD.observaciones, 'creado_por', OLD.creado_por, 'created_at', OLD.created_at, 'updated_at', OLD.updated_at), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'turno_id', NEW.turno_id, 'fecha', NEW.fecha, 'hora_entrada_real', NEW.hora_entrada_real, 'hora_salida_real', NEW.hora_salida_real, 'horas_ordinarias', NEW.horas_ordinarias, 'horas_extra', NEW.horas_extra, 'horas_tardanza', NEW.horas_tardanza, 'minutos_tardanza', NEW.minutos_tardanza, 'estado', NEW.estado, 'tipo_ausencia_id', NEW.tipo_ausencia_id, 'incapacidad_id', NEW.incapacidad_id, 'observaciones', NEW.observaciones, 'creado_por', NEW.creado_por, 'created_at', NEW.created_at, 'updated_at', NEW.updated_at));
END;

CREATE TRIGGER trg_aud_registro_asistencia_detalle_del AFTER DELETE ON registro_asistencia_detalle
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('registro_asistencia_detalle', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'turno_id', OLD.turno_id, 'fecha', OLD.fecha, 'hora_entrada_real', OLD.hora_entrada_real, 'hora_salida_real', OLD.hora_salida_real, 'horas_ordinarias', OLD.horas_ordinarias, 'horas_extra', OLD.horas_extra, 'horas_tardanza', OLD.horas_tardanza, 'minutos_tardanza', OLD.minutos_tardanza, 'estado', OLD.estado, 'tipo_ausencia_id', OLD.tipo_ausencia_id, 'incapacidad_id', OLD.incapacidad_id, 'observaciones', OLD.observaciones, 'creado_por', OLD.creado_por, 'created_at', OLD.created_at, 'updated_at', OLD.updated_at), NULL);
END;

CREATE TRIGGER trg_aud_vacaciones_solicitudes_ins AFTER INSERT ON vacaciones_solicitudes
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('vacaciones_solicitudes', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'fecha_inicio', NEW.fecha_inicio, 'fecha_fin', NEW.fecha_fin, 'dias_solicitados', NEW.dias_solicitados, 'estado', NEW.estado, 'aprobado_por', NEW.aprobado_por, 'fecha_aprobacion', NEW.fecha_aprobacion, 'motivo_rechazo', NEW.motivo_rechazo, 'creado_por', NEW.creado_por, 'created_at', NEW.created_at, 'updated_at', NEW.updated_at));
END;

CREATE TRIGGER trg_aud_vacaciones_solicitudes_upd AFTER UPDATE ON vacaciones_solicitudes
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('vacaciones_solicitudes', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'fecha_inicio', OLD.fecha_inicio, 'fecha_fin', OLD.fecha_fin, 'dias_solicitados', OLD.dias_solicitados, 'estado', OLD.estado, 'aprobado_por', OLD.aprobado_por, 'fecha_aprobacion', OLD.fecha_aprobacion, 'motivo_rechazo', OLD.motivo_rechazo, 'creado_por', OLD.creado_por, 'created_at', OLD.created_at, 'updated_at', OLD.updated_at), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'fecha_inicio', NEW.fecha_inicio, 'fecha_fin', NEW.fecha_fin, 'dias_solicitados', NEW.dias_solicitados, 'estado', NEW.estado, 'aprobado_por', NEW.aprobado_por, 'fecha_aprobacion', NEW.fecha_aprobacion, 'motivo_rechazo', NEW.motivo_rechazo, 'creado_por', NEW.creado_por, 'created_at', NEW.created_at, 'updated_at', NEW.updated_at));
END;

CREATE TRIGGER trg_aud_vacaciones_solicitudes_del AFTER DELETE ON vacaciones_solicitudes
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('vacaciones_solicitudes', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'fecha_inicio', OLD.fecha_inicio, 'fecha_fin', OLD.fecha_fin, 'dias_solicitados', OLD.dias_solicitados, 'estado', OLD.estado, 'aprobado_por', OLD.aprobado_por, 'fecha_aprobacion', OLD.fecha_aprobacion, 'motivo_rechazo', OLD.motivo_rechazo, 'creado_por', OLD.creado_por, 'created_at', OLD.created_at, 'updated_at', OLD.updated_at), NULL);
END;

CREATE TRIGGER trg_auditoria_cambios_no_upd BEFORE UPDATE ON auditoria_cambios
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'registro_inmutable: la auditoría es solo inserción';
END;

CREATE TRIGGER trg_auditoria_cambios_no_del BEFORE DELETE ON auditoria_cambios
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'registro_inmutable: la auditoría es solo inserción';
END;
