DROP PROCEDURE IF EXISTS verificar_datos_cifrados;

CREATE PROCEDURE verificar_datos_cifrados()
BEGIN
  IF EXISTS (SELECT 1 FROM empleados WHERE numero_identificacion_cifrado IS NULL)
     OR EXISTS (SELECT 1 FROM empleado_datos_bancarios WHERE numero_cuenta_cifrado IS NULL)
     OR EXISTS (SELECT 1 FROM empleado_datos_personales
                WHERE (direccion IS NOT NULL AND direccion <> '' AND direccion_cifrado IS NULL)
                   OR (telefono IS NOT NULL AND telefono <> '' AND telefono_cifrado IS NULL)
                   OR (correo_electronico IS NOT NULL AND correo_electronico <> '' AND correo_electronico_cifrado IS NULL)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'datos_sin_cifrar: ejecute migrate goto 145 y jobs cifrar-existentes antes de continuar';
  END IF;
END;

CALL verificar_datos_cifrados();
DROP PROCEDURE verificar_datos_cifrados;

DROP VIEW v_reporte_ccss;
DROP VIEW v_reporte_ins;
DROP VIEW v_reporte_renta;

ALTER TABLE empleados
  DROP INDEX uk_empleado_identificacion,
  DROP COLUMN numero_identificacion,
  MODIFY numero_identificacion_cifrado VARBINARY(512) NOT NULL COMMENT 'Identificación cifrada AES-256-GCM',
  MODIFY numero_identificacion_hash CHAR(64) NOT NULL COMMENT 'HMAC-SHA256 del valor normalizado (búsqueda exacta y unicidad)',
  MODIFY numero_identificacion_kid VARCHAR(20) NOT NULL COMMENT 'Llave de cifrado usada',
  ADD UNIQUE KEY uk_empleado_identificacion (tipo_identificacion_id, numero_identificacion_hash);

ALTER TABLE empleado_datos_personales
  DROP COLUMN direccion,
  DROP COLUMN telefono,
  DROP COLUMN correo_electronico;

ALTER TABLE empleado_datos_bancarios
  DROP COLUMN numero_cuenta,
  MODIFY numero_cuenta_cifrado VARBINARY(512) NOT NULL COMMENT 'Número de cuenta o IBAN cifrado AES-256-GCM',
  MODIFY numero_cuenta_hash CHAR(64) NOT NULL COMMENT 'HMAC-SHA256 del valor normalizado (búsqueda exacta)',
  MODIFY numero_cuenta_kid VARCHAR(20) NOT NULL COMMENT 'Llave de cifrado usada';

CREATE VIEW v_reporte_ccss AS
SELECT pde.periodo_planilla_id, pde.empleado_id,
       HEX(e.numero_identificacion_cifrado) AS identificacion_cifrado_hex, e.numero_identificacion_kid AS identificacion_kid,
       e.nombre_completo,
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
       HEX(e.numero_identificacion_cifrado) AS identificacion_cifrado_hex, e.numero_identificacion_kid AS identificacion_kid,
       e.nombre_completo,
       pde.total_ingresos AS salario_bruto, pde.base_ccss
FROM planilla_detalle_empleado pde
JOIN empleados e ON e.id = pde.empleado_id;

CREATE VIEW v_reporte_renta AS
SELECT pde.periodo_planilla_id, pde.empleado_id,
       HEX(e.numero_identificacion_cifrado) AS identificacion_cifrado_hex, e.numero_identificacion_kid AS identificacion_kid,
       e.nombre_completo,
       pde.base_renta, pde.renta_bruta, pde.creditos_fiscales_aplicados, pde.renta_neta
FROM planilla_detalle_empleado pde
JOIN empleados e ON e.id = pde.empleado_id;

DROP TRIGGER trg_aud_empleados_ins;
CREATE TRIGGER trg_aud_empleados_ins AFTER INSERT ON empleados
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleados', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo_interno', NEW.codigo_interno, 'tipo_identificacion_id', NEW.tipo_identificacion_id, 'nombre_completo', NEW.nombre_completo, 'fecha_ingreso', NEW.fecha_ingreso, 'activo', NEW.activo, 'created_at', NEW.created_at, 'updated_at', NEW.updated_at, 'numero_identificacion_hash', NEW.numero_identificacion_hash, 'numero_identificacion_kid', NEW.numero_identificacion_kid));
END;

DROP TRIGGER trg_aud_empleados_upd;
CREATE TRIGGER trg_aud_empleados_upd AFTER UPDATE ON empleados
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleados', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo_interno', OLD.codigo_interno, 'tipo_identificacion_id', OLD.tipo_identificacion_id, 'nombre_completo', OLD.nombre_completo, 'fecha_ingreso', OLD.fecha_ingreso, 'activo', OLD.activo, 'created_at', OLD.created_at, 'updated_at', OLD.updated_at, 'numero_identificacion_hash', OLD.numero_identificacion_hash, 'numero_identificacion_kid', OLD.numero_identificacion_kid), JSON_OBJECT('id', NEW.id, 'codigo_interno', NEW.codigo_interno, 'tipo_identificacion_id', NEW.tipo_identificacion_id, 'nombre_completo', NEW.nombre_completo, 'fecha_ingreso', NEW.fecha_ingreso, 'activo', NEW.activo, 'created_at', NEW.created_at, 'updated_at', NEW.updated_at, 'numero_identificacion_hash', NEW.numero_identificacion_hash, 'numero_identificacion_kid', NEW.numero_identificacion_kid));
END;

DROP TRIGGER trg_aud_empleados_del;
CREATE TRIGGER trg_aud_empleados_del AFTER DELETE ON empleados
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleados', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo_interno', OLD.codigo_interno, 'tipo_identificacion_id', OLD.tipo_identificacion_id, 'nombre_completo', OLD.nombre_completo, 'fecha_ingreso', OLD.fecha_ingreso, 'activo', OLD.activo, 'created_at', OLD.created_at, 'updated_at', OLD.updated_at, 'numero_identificacion_hash', OLD.numero_identificacion_hash, 'numero_identificacion_kid', OLD.numero_identificacion_kid), NULL);
END;

DROP TRIGGER trg_aud_empleado_datos_personales_ins;
CREATE TRIGGER trg_aud_empleado_datos_personales_ins AFTER INSERT ON empleado_datos_personales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_datos_personales', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'direccion_hash', NEW.direccion_hash, 'direccion_kid', NEW.direccion_kid, 'telefono_hash', NEW.telefono_hash, 'telefono_kid', NEW.telefono_kid, 'correo_electronico_hash', NEW.correo_electronico_hash, 'correo_electronico_kid', NEW.correo_electronico_kid));
END;

DROP TRIGGER trg_aud_empleado_datos_personales_upd;
CREATE TRIGGER trg_aud_empleado_datos_personales_upd AFTER UPDATE ON empleado_datos_personales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_datos_personales', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'direccion_hash', OLD.direccion_hash, 'direccion_kid', OLD.direccion_kid, 'telefono_hash', OLD.telefono_hash, 'telefono_kid', OLD.telefono_kid, 'correo_electronico_hash', OLD.correo_electronico_hash, 'correo_electronico_kid', OLD.correo_electronico_kid), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'direccion_hash', NEW.direccion_hash, 'direccion_kid', NEW.direccion_kid, 'telefono_hash', NEW.telefono_hash, 'telefono_kid', NEW.telefono_kid, 'correo_electronico_hash', NEW.correo_electronico_hash, 'correo_electronico_kid', NEW.correo_electronico_kid));
END;

DROP TRIGGER trg_aud_empleado_datos_personales_del;
CREATE TRIGGER trg_aud_empleado_datos_personales_del AFTER DELETE ON empleado_datos_personales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_datos_personales', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'direccion_hash', OLD.direccion_hash, 'direccion_kid', OLD.direccion_kid, 'telefono_hash', OLD.telefono_hash, 'telefono_kid', OLD.telefono_kid, 'correo_electronico_hash', OLD.correo_electronico_hash, 'correo_electronico_kid', OLD.correo_electronico_kid), NULL);
END;

DROP TRIGGER trg_aud_empleado_datos_bancarios_ins;
CREATE TRIGGER trg_aud_empleado_datos_bancarios_ins AFTER INSERT ON empleado_datos_bancarios
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_datos_bancarios', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'banco_id', NEW.banco_id, 'tipo_cuenta', NEW.tipo_cuenta, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'numero_cuenta_hash', NEW.numero_cuenta_hash, 'numero_cuenta_kid', NEW.numero_cuenta_kid));
END;

DROP TRIGGER trg_aud_empleado_datos_bancarios_upd;
CREATE TRIGGER trg_aud_empleado_datos_bancarios_upd AFTER UPDATE ON empleado_datos_bancarios
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_datos_bancarios', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'banco_id', OLD.banco_id, 'tipo_cuenta', OLD.tipo_cuenta, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'numero_cuenta_hash', OLD.numero_cuenta_hash, 'numero_cuenta_kid', OLD.numero_cuenta_kid), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'banco_id', NEW.banco_id, 'tipo_cuenta', NEW.tipo_cuenta, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'numero_cuenta_hash', NEW.numero_cuenta_hash, 'numero_cuenta_kid', NEW.numero_cuenta_kid));
END;

DROP TRIGGER trg_aud_empleado_datos_bancarios_del;
CREATE TRIGGER trg_aud_empleado_datos_bancarios_del AFTER DELETE ON empleado_datos_bancarios
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_datos_bancarios', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'banco_id', OLD.banco_id, 'tipo_cuenta', OLD.tipo_cuenta, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'numero_cuenta_hash', OLD.numero_cuenta_hash, 'numero_cuenta_kid', OLD.numero_cuenta_kid), NULL);
END;

