DROP VIEW v_reporte_ccss;
DROP VIEW v_reporte_ins;
DROP VIEW v_reporte_renta;

ALTER TABLE empleados
  DROP INDEX uk_empleado_identificacion,
  ADD COLUMN numero_identificacion VARCHAR(20) NULL COMMENT 'cédula (9 díg.), DIMEX (12 díg.), pasaporte o cédula de residencia' AFTER tipo_identificacion_id,
  MODIFY numero_identificacion_cifrado VARBINARY(512) NULL COMMENT 'Cifrado AES-256-GCM',
  MODIFY numero_identificacion_hash CHAR(64) NULL COMMENT 'HMAC-SHA256 del valor normalizado (búsqueda exacta)',
  MODIFY numero_identificacion_kid VARCHAR(20) NULL COMMENT 'Llave de cifrado usada',
  ADD UNIQUE KEY uk_empleado_identificacion (tipo_identificacion_id, numero_identificacion);

ALTER TABLE empleado_datos_personales
  ADD COLUMN direccion VARCHAR(255) NULL COMMENT 'Dirección de residencia del empleado' AFTER empleado_id,
  ADD COLUMN telefono VARCHAR(30) NULL COMMENT 'Teléfono de contacto del empleado' AFTER direccion,
  ADD COLUMN correo_electronico VARCHAR(150) NULL COMMENT 'Correo electrónico del empleado' AFTER telefono;

ALTER TABLE empleado_datos_bancarios
  ADD COLUMN numero_cuenta VARCHAR(40) NULL COMMENT 'Número de cuenta o IBAN de depósito' AFTER tipo_cuenta,
  MODIFY numero_cuenta_cifrado VARBINARY(512) NULL COMMENT 'Cifrado AES-256-GCM',
  MODIFY numero_cuenta_hash CHAR(64) NULL COMMENT 'HMAC-SHA256 del valor normalizado (búsqueda exacta)',
  MODIFY numero_cuenta_kid VARCHAR(20) NULL COMMENT 'Llave de cifrado usada';

CREATE VIEW v_reporte_ccss AS
SELECT pde.periodo_planilla_id, pde.empleado_id,
       e.numero_identificacion AS identificacion, e.nombre_completo,
       pde.base_ccss, pde.cuota_obrera_ccss, pde.cuota_patronal_ccss,
       pcs.rubro AS rubro_patronal, pat.porcentaje AS porcentaje_patronal, pat.monto AS monto_patronal
FROM planilla_detalle_empleado pde
JOIN periodos_planilla pp ON pp.id = pde.periodo_planilla_id
JOIN empleados e ON e.id = pde.empleado_id
LEFT JOIN planilla_detalle_patronal pat ON pat.planilla_detalle_empleado_id = pde.id
LEFT JOIN parametros_cargas_sociales pcs
       ON pcs.concepto_salarial_id = pat.concepto_salarial_id
      AND pcs.fecha_inicio_vigencia <= pp.fecha_fin
      AND (pcs.fecha_fin_vigencia IS NULL OR pcs.fecha_fin_vigencia >= pp.fecha_fin);

CREATE VIEW v_reporte_ins AS
SELECT pde.periodo_planilla_id, pde.empleado_id,
       e.numero_identificacion AS identificacion, e.nombre_completo,
       pde.total_ingresos AS salario_bruto, pde.base_ccss
FROM planilla_detalle_empleado pde
JOIN empleados e ON e.id = pde.empleado_id;

CREATE VIEW v_reporte_renta AS
SELECT pde.periodo_planilla_id, pde.empleado_id,
       e.numero_identificacion AS identificacion, e.nombre_completo,
       pde.base_renta, pde.renta_bruta, pde.creditos_fiscales_aplicados, pde.renta_neta
FROM planilla_detalle_empleado pde
JOIN empleados e ON e.id = pde.empleado_id;

DROP TRIGGER IF EXISTS trg_aud_empleados_ins;
CREATE TRIGGER trg_aud_empleados_ins AFTER INSERT ON empleados
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleados', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo_interno', NEW.codigo_interno, 'tipo_identificacion_id', NEW.tipo_identificacion_id, 'numero_identificacion', IF(NEW.numero_identificacion IS NULL, NULL, '***'), 'nombre_completo', NEW.nombre_completo, 'fecha_ingreso', NEW.fecha_ingreso, 'activo', NEW.activo, 'created_at', NEW.created_at, 'updated_at', NEW.updated_at, 'numero_identificacion_hash', NEW.numero_identificacion_hash, 'numero_identificacion_kid', NEW.numero_identificacion_kid));
END;

DROP TRIGGER IF EXISTS trg_aud_empleados_upd;
CREATE TRIGGER trg_aud_empleados_upd AFTER UPDATE ON empleados
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleados', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo_interno', OLD.codigo_interno, 'tipo_identificacion_id', OLD.tipo_identificacion_id, 'numero_identificacion', IF(OLD.numero_identificacion IS NULL, NULL, '***'), 'nombre_completo', OLD.nombre_completo, 'fecha_ingreso', OLD.fecha_ingreso, 'activo', OLD.activo, 'created_at', OLD.created_at, 'updated_at', OLD.updated_at, 'numero_identificacion_hash', OLD.numero_identificacion_hash, 'numero_identificacion_kid', OLD.numero_identificacion_kid), JSON_OBJECT('id', NEW.id, 'codigo_interno', NEW.codigo_interno, 'tipo_identificacion_id', NEW.tipo_identificacion_id, 'numero_identificacion', IF(NEW.numero_identificacion IS NULL, NULL, '***'), 'nombre_completo', NEW.nombre_completo, 'fecha_ingreso', NEW.fecha_ingreso, 'activo', NEW.activo, 'created_at', NEW.created_at, 'updated_at', NEW.updated_at, 'numero_identificacion_hash', NEW.numero_identificacion_hash, 'numero_identificacion_kid', NEW.numero_identificacion_kid));
END;

DROP TRIGGER IF EXISTS trg_aud_empleados_del;
CREATE TRIGGER trg_aud_empleados_del AFTER DELETE ON empleados
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleados', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo_interno', OLD.codigo_interno, 'tipo_identificacion_id', OLD.tipo_identificacion_id, 'numero_identificacion', IF(OLD.numero_identificacion IS NULL, NULL, '***'), 'nombre_completo', OLD.nombre_completo, 'fecha_ingreso', OLD.fecha_ingreso, 'activo', OLD.activo, 'created_at', OLD.created_at, 'updated_at', OLD.updated_at, 'numero_identificacion_hash', OLD.numero_identificacion_hash, 'numero_identificacion_kid', OLD.numero_identificacion_kid), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_datos_personales_ins;
CREATE TRIGGER trg_aud_empleado_datos_personales_ins AFTER INSERT ON empleado_datos_personales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_datos_personales', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'direccion', IF(NEW.direccion IS NULL, NULL, '***'), 'telefono', IF(NEW.telefono IS NULL, NULL, '***'), 'correo_electronico', IF(NEW.correo_electronico IS NULL, NULL, '***'), 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente, 'direccion_hash', NEW.direccion_hash, 'direccion_kid', NEW.direccion_kid, 'telefono_hash', NEW.telefono_hash, 'telefono_kid', NEW.telefono_kid, 'correo_electronico_hash', NEW.correo_electronico_hash, 'correo_electronico_kid', NEW.correo_electronico_kid));
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_datos_personales_upd;
CREATE TRIGGER trg_aud_empleado_datos_personales_upd AFTER UPDATE ON empleado_datos_personales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_datos_personales', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'direccion', IF(OLD.direccion IS NULL, NULL, '***'), 'telefono', IF(OLD.telefono IS NULL, NULL, '***'), 'correo_electronico', IF(OLD.correo_electronico IS NULL, NULL, '***'), 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente, 'direccion_hash', OLD.direccion_hash, 'direccion_kid', OLD.direccion_kid, 'telefono_hash', OLD.telefono_hash, 'telefono_kid', OLD.telefono_kid, 'correo_electronico_hash', OLD.correo_electronico_hash, 'correo_electronico_kid', OLD.correo_electronico_kid), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'direccion', IF(NEW.direccion IS NULL, NULL, '***'), 'telefono', IF(NEW.telefono IS NULL, NULL, '***'), 'correo_electronico', IF(NEW.correo_electronico IS NULL, NULL, '***'), 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente, 'direccion_hash', NEW.direccion_hash, 'direccion_kid', NEW.direccion_kid, 'telefono_hash', NEW.telefono_hash, 'telefono_kid', NEW.telefono_kid, 'correo_electronico_hash', NEW.correo_electronico_hash, 'correo_electronico_kid', NEW.correo_electronico_kid));
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_datos_personales_del;
CREATE TRIGGER trg_aud_empleado_datos_personales_del AFTER DELETE ON empleado_datos_personales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_datos_personales', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'direccion', IF(OLD.direccion IS NULL, NULL, '***'), 'telefono', IF(OLD.telefono IS NULL, NULL, '***'), 'correo_electronico', IF(OLD.correo_electronico IS NULL, NULL, '***'), 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente, 'direccion_hash', OLD.direccion_hash, 'direccion_kid', OLD.direccion_kid, 'telefono_hash', OLD.telefono_hash, 'telefono_kid', OLD.telefono_kid, 'correo_electronico_hash', OLD.correo_electronico_hash, 'correo_electronico_kid', OLD.correo_electronico_kid), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_datos_bancarios_ins;
CREATE TRIGGER trg_aud_empleado_datos_bancarios_ins AFTER INSERT ON empleado_datos_bancarios
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_datos_bancarios', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'banco_id', NEW.banco_id, 'tipo_cuenta', NEW.tipo_cuenta, 'numero_cuenta', IF(NEW.numero_cuenta IS NULL, NULL, '***'), 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente, 'numero_cuenta_hash', NEW.numero_cuenta_hash, 'numero_cuenta_kid', NEW.numero_cuenta_kid));
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_datos_bancarios_upd;
CREATE TRIGGER trg_aud_empleado_datos_bancarios_upd AFTER UPDATE ON empleado_datos_bancarios
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_datos_bancarios', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'banco_id', OLD.banco_id, 'tipo_cuenta', OLD.tipo_cuenta, 'numero_cuenta', IF(OLD.numero_cuenta IS NULL, NULL, '***'), 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente, 'numero_cuenta_hash', OLD.numero_cuenta_hash, 'numero_cuenta_kid', OLD.numero_cuenta_kid), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'banco_id', NEW.banco_id, 'tipo_cuenta', NEW.tipo_cuenta, 'numero_cuenta', IF(NEW.numero_cuenta IS NULL, NULL, '***'), 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente, 'numero_cuenta_hash', NEW.numero_cuenta_hash, 'numero_cuenta_kid', NEW.numero_cuenta_kid));
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_datos_bancarios_del;
CREATE TRIGGER trg_aud_empleado_datos_bancarios_del AFTER DELETE ON empleado_datos_bancarios
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_datos_bancarios', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'banco_id', OLD.banco_id, 'tipo_cuenta', OLD.tipo_cuenta, 'numero_cuenta', IF(OLD.numero_cuenta IS NULL, NULL, '***'), 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente, 'numero_cuenta_hash', OLD.numero_cuenta_hash, 'numero_cuenta_kid', OLD.numero_cuenta_kid), NULL);
END;
