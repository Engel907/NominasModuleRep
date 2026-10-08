-- down 000001_seguridad_reforzada: deja usuarios/roles/usuario_roles y sus triggers como en 000000.
DROP TRIGGER IF EXISTS trg_aud_usuario_roles_ins;
DROP TRIGGER IF EXISTS trg_aud_usuario_roles_upd;
DROP TRIGGER IF EXISTS trg_aud_usuario_roles_del;
DROP TABLE IF EXISTS usuario_roles;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS tokens_reset_password;
ALTER TABLE roles DROP COLUMN activo, DROP COLUMN descripcion;
ALTER TABLE usuarios
  DROP COLUMN fecha_ultimo_cambio_password, DROP COLUMN requiere_cambio_password,
  DROP COLUMN bloqueado_hasta, DROP COLUMN intentos_fallidos,
  DROP COLUMN updated_at, DROP COLUMN created_at;

-- usuario_roles como la define 000000 (PK compuesta)
CREATE TABLE usuario_roles (
  usuario_id  BIGINT UNSIGNED NOT NULL COMMENT 'Usuario al que se le asigna el rol',
  rol_id      BIGINT UNSIGNED NOT NULL COMMENT 'Rol asignado al usuario',
  PRIMARY KEY (usuario_id, rol_id),
  CONSTRAINT fk_ur_usuario FOREIGN KEY (usuario_id) REFERENCES usuarios(id),
  CONSTRAINT fk_ur_rol FOREIGN KEY (rol_id) REFERENCES roles(id)
) ENGINE=InnoDB COMMENT='Asignación de roles a cada usuario del sistema';

-- triggers de auditoría tal como los define 000000
DROP TRIGGER IF EXISTS trg_aud_usuarios_ins;
CREATE TRIGGER trg_aud_usuarios_ins AFTER INSERT ON usuarios
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('usuarios', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'nombre_usuario', NEW.nombre_usuario, 'nombre_completo', NEW.nombre_completo, 'correo', NEW.correo, 'password_hash', '***REDACTED***', 'empleado_id', NEW.empleado_id, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_usuarios_upd;
CREATE TRIGGER trg_aud_usuarios_upd AFTER UPDATE ON usuarios
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('usuarios', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'nombre_usuario', OLD.nombre_usuario, 'nombre_completo', OLD.nombre_completo, 'correo', OLD.correo, 'password_hash', '***REDACTED***', 'empleado_id', OLD.empleado_id, 'activo', OLD.activo), JSON_OBJECT('id', NEW.id, 'nombre_usuario', NEW.nombre_usuario, 'nombre_completo', NEW.nombre_completo, 'correo', NEW.correo, 'password_hash', '***REDACTED***', 'empleado_id', NEW.empleado_id, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_usuarios_del;
CREATE TRIGGER trg_aud_usuarios_del AFTER DELETE ON usuarios
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('usuarios', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'nombre_usuario', OLD.nombre_usuario, 'nombre_completo', OLD.nombre_completo, 'correo', OLD.correo, 'password_hash', '***REDACTED***', 'empleado_id', OLD.empleado_id, 'activo', OLD.activo), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_roles_ins;
CREATE TRIGGER trg_aud_roles_ins AFTER INSERT ON roles
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('roles', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre));
END;

DROP TRIGGER IF EXISTS trg_aud_roles_upd;
CREATE TRIGGER trg_aud_roles_upd AFTER UPDATE ON roles
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('roles', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre));
END;

DROP TRIGGER IF EXISTS trg_aud_roles_del;
CREATE TRIGGER trg_aud_roles_del AFTER DELETE ON roles
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('roles', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_usuario_roles_ins;
CREATE TRIGGER trg_aud_usuario_roles_ins AFTER INSERT ON usuario_roles
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('usuario_roles', 0, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('usuario_id', NEW.usuario_id, 'rol_id', NEW.rol_id));
END;

DROP TRIGGER IF EXISTS trg_aud_usuario_roles_upd;
CREATE TRIGGER trg_aud_usuario_roles_upd AFTER UPDATE ON usuario_roles
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('usuario_roles', 0, @auditoria_usuario_id, 'update', JSON_OBJECT('usuario_id', OLD.usuario_id, 'rol_id', OLD.rol_id), JSON_OBJECT('usuario_id', NEW.usuario_id, 'rol_id', NEW.rol_id));
END;

DROP TRIGGER IF EXISTS trg_aud_usuario_roles_del;
CREATE TRIGGER trg_aud_usuario_roles_del AFTER DELETE ON usuario_roles
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('usuario_roles', 0, @auditoria_usuario_id, 'delete', JSON_OBJECT('usuario_id', OLD.usuario_id, 'rol_id', OLD.rol_id), NULL);
END;

