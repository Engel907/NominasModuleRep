DROP TRIGGER trg_aud_jornadas_ins;
CREATE TRIGGER trg_aud_jornadas_ins AFTER INSERT ON jornadas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('jornadas', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'activo', NEW.activo));
END;

DROP TRIGGER trg_aud_jornadas_upd;
CREATE TRIGGER trg_aud_jornadas_upd AFTER UPDATE ON jornadas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('jornadas', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'activo', OLD.activo), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'activo', NEW.activo));
END;

DROP TRIGGER trg_aud_jornadas_del;
CREATE TRIGGER trg_aud_jornadas_del AFTER DELETE ON jornadas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('jornadas', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'activo', OLD.activo), NULL);
END;

ALTER TABLE jornadas DROP COLUMN clase;
