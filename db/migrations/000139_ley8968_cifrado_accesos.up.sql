ALTER TABLE empleados
  ADD COLUMN numero_identificacion_cifrado VARBINARY(512) NULL COMMENT 'Cifrado AES-256-GCM',
  ADD COLUMN numero_identificacion_hash CHAR(64) NULL COMMENT 'HMAC-SHA256 del valor normalizado (búsqueda exacta)',
  ADD COLUMN numero_identificacion_kid VARCHAR(20) NULL COMMENT 'Llave de cifrado usada',
  ADD KEY ix_empleados_numero_ident_hash (numero_identificacion_hash);

ALTER TABLE empleado_datos_personales
  ADD COLUMN direccion_cifrado VARBINARY(512) NULL COMMENT 'Cifrado AES-256-GCM',
  ADD COLUMN direccion_hash CHAR(64) NULL COMMENT 'HMAC-SHA256 del valor normalizado (búsqueda exacta)',
  ADD COLUMN direccion_kid VARCHAR(20) NULL COMMENT 'Llave de cifrado usada',
  ADD COLUMN telefono_cifrado VARBINARY(512) NULL COMMENT 'Cifrado AES-256-GCM',
  ADD COLUMN telefono_hash CHAR(64) NULL COMMENT 'HMAC-SHA256 del valor normalizado (búsqueda exacta)',
  ADD COLUMN telefono_kid VARCHAR(20) NULL COMMENT 'Llave de cifrado usada',
  ADD COLUMN correo_electronico_cifrado VARBINARY(512) NULL COMMENT 'Cifrado AES-256-GCM',
  ADD COLUMN correo_electronico_hash CHAR(64) NULL COMMENT 'HMAC-SHA256 del valor normalizado (búsqueda exacta)',
  ADD COLUMN correo_electronico_kid VARCHAR(20) NULL COMMENT 'Llave de cifrado usada',
  ADD KEY ix_empleado_dat_direccion_hash (direccion_hash),
  ADD KEY ix_empleado_dat_telefono_hash (telefono_hash),
  ADD KEY ix_empleado_dat_correo_elect_hash (correo_electronico_hash);

ALTER TABLE empleado_datos_bancarios
  ADD COLUMN numero_cuenta_cifrado VARBINARY(512) NULL COMMENT 'Cifrado AES-256-GCM',
  ADD COLUMN numero_cuenta_hash CHAR(64) NULL COMMENT 'HMAC-SHA256 del valor normalizado (búsqueda exacta)',
  ADD COLUMN numero_cuenta_kid VARCHAR(20) NULL COMMENT 'Llave de cifrado usada',
  ADD KEY ix_empleado_dat_numero_cuent_hash (numero_cuenta_hash);

CREATE TABLE registro_accesos_datos (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  usuario_id BIGINT UNSIGNED NOT NULL COMMENT 'Quien accedió',
  empleado_id BIGINT UNSIGNED NULL COMMENT 'Titular de los datos',
  recurso VARCHAR(60) NOT NULL COMMENT 'Endpoint/recurso',
  campos VARCHAR(255) NOT NULL COMMENT 'Campos sensibles revelados',
  ip VARCHAR(45) NULL COMMENT 'IP',
  request_id VARCHAR(64) NULL COMMENT 'Correlación con logs',
  fecha_acceso DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Cuándo',
  CONSTRAINT fk_rad_usuario FOREIGN KEY (usuario_id) REFERENCES usuarios(id),
  KEY ix_rad_emp (empleado_id, fecha_acceso),
  KEY ix_rad_usr (usuario_id, fecha_acceso)
) ENGINE=InnoDB COMMENT='Bitácora de accesos a datos sensibles (Ley 8968), solo inserción';

CREATE TRIGGER trg_rad_no_upd BEFORE UPDATE ON registro_accesos_datos FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'registro_inmutable: la bitácora es solo inserción';
END;

CREATE TRIGGER trg_rad_no_del BEFORE DELETE ON registro_accesos_datos FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'registro_inmutable: la bitácora es solo inserción';
END;

INSERT INTO rol_permisos (rol_id, permiso_id)
SELECT r.id, p.id FROM roles r JOIN permisos p ON p.codigo = 'datos_personales.ver_sensible'
WHERE r.codigo = 'RRHH'
  AND NOT EXISTS (SELECT 1 FROM rol_permisos rp WHERE rp.rol_id = r.id AND rp.permiso_id = p.id);

UPDATE usuarios u
SET u.permisos_version = u.permisos_version + 1
WHERE EXISTS (SELECT 1 FROM usuario_roles ur JOIN roles r ON r.id = ur.rol_id
              WHERE ur.usuario_id = u.id AND r.codigo = 'RRHH' AND ur.fecha_fin_vigencia IS NULL);

UPDATE auditoria_cambios
   SET valor_anterior = JSON_SET(valor_anterior, '$.numero_identificacion', '***')
 WHERE tabla_afectada = 'empleados'
   AND JSON_TYPE(JSON_EXTRACT(valor_anterior, '$.numero_identificacion')) NOT IN ('NULL')
   AND JSON_UNQUOTE(JSON_EXTRACT(valor_anterior, '$.numero_identificacion')) <> '***';

UPDATE auditoria_cambios
   SET valor_nuevo = JSON_SET(valor_nuevo, '$.numero_identificacion', '***')
 WHERE tabla_afectada = 'empleados'
   AND JSON_TYPE(JSON_EXTRACT(valor_nuevo, '$.numero_identificacion')) NOT IN ('NULL')
   AND JSON_UNQUOTE(JSON_EXTRACT(valor_nuevo, '$.numero_identificacion')) <> '***';

UPDATE auditoria_cambios
   SET valor_anterior = JSON_SET(valor_anterior, '$.direccion', '***')
 WHERE tabla_afectada = 'empleado_datos_personales'
   AND JSON_TYPE(JSON_EXTRACT(valor_anterior, '$.direccion')) NOT IN ('NULL')
   AND JSON_UNQUOTE(JSON_EXTRACT(valor_anterior, '$.direccion')) <> '***';

UPDATE auditoria_cambios
   SET valor_nuevo = JSON_SET(valor_nuevo, '$.direccion', '***')
 WHERE tabla_afectada = 'empleado_datos_personales'
   AND JSON_TYPE(JSON_EXTRACT(valor_nuevo, '$.direccion')) NOT IN ('NULL')
   AND JSON_UNQUOTE(JSON_EXTRACT(valor_nuevo, '$.direccion')) <> '***';

UPDATE auditoria_cambios
   SET valor_anterior = JSON_SET(valor_anterior, '$.telefono', '***')
 WHERE tabla_afectada = 'empleado_datos_personales'
   AND JSON_TYPE(JSON_EXTRACT(valor_anterior, '$.telefono')) NOT IN ('NULL')
   AND JSON_UNQUOTE(JSON_EXTRACT(valor_anterior, '$.telefono')) <> '***';

UPDATE auditoria_cambios
   SET valor_nuevo = JSON_SET(valor_nuevo, '$.telefono', '***')
 WHERE tabla_afectada = 'empleado_datos_personales'
   AND JSON_TYPE(JSON_EXTRACT(valor_nuevo, '$.telefono')) NOT IN ('NULL')
   AND JSON_UNQUOTE(JSON_EXTRACT(valor_nuevo, '$.telefono')) <> '***';

UPDATE auditoria_cambios
   SET valor_anterior = JSON_SET(valor_anterior, '$.correo_electronico', '***')
 WHERE tabla_afectada = 'empleado_datos_personales'
   AND JSON_TYPE(JSON_EXTRACT(valor_anterior, '$.correo_electronico')) NOT IN ('NULL')
   AND JSON_UNQUOTE(JSON_EXTRACT(valor_anterior, '$.correo_electronico')) <> '***';

UPDATE auditoria_cambios
   SET valor_nuevo = JSON_SET(valor_nuevo, '$.correo_electronico', '***')
 WHERE tabla_afectada = 'empleado_datos_personales'
   AND JSON_TYPE(JSON_EXTRACT(valor_nuevo, '$.correo_electronico')) NOT IN ('NULL')
   AND JSON_UNQUOTE(JSON_EXTRACT(valor_nuevo, '$.correo_electronico')) <> '***';

UPDATE auditoria_cambios
   SET valor_anterior = JSON_SET(valor_anterior, '$.numero_cuenta', '***')
 WHERE tabla_afectada = 'empleado_datos_bancarios'
   AND JSON_TYPE(JSON_EXTRACT(valor_anterior, '$.numero_cuenta')) NOT IN ('NULL')
   AND JSON_UNQUOTE(JSON_EXTRACT(valor_anterior, '$.numero_cuenta')) <> '***';

UPDATE auditoria_cambios
   SET valor_nuevo = JSON_SET(valor_nuevo, '$.numero_cuenta', '***')
 WHERE tabla_afectada = 'empleado_datos_bancarios'
   AND JSON_TYPE(JSON_EXTRACT(valor_nuevo, '$.numero_cuenta')) NOT IN ('NULL')
   AND JSON_UNQUOTE(JSON_EXTRACT(valor_nuevo, '$.numero_cuenta')) <> '***';

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
