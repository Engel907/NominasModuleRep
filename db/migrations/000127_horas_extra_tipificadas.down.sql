-- Revierte 000127 (auditoría de tipos_hora_extra sin clase, como en 000000).
DROP TRIGGER IF EXISTS trg_aud_tipos_hora_extra_ins;
CREATE TRIGGER trg_aud_tipos_hora_extra_ins AFTER INSERT ON tipos_hora_extra
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_hora_extra', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_tipos_hora_extra_upd;
CREATE TRIGGER trg_aud_tipos_hora_extra_upd AFTER UPDATE ON tipos_hora_extra
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_hora_extra', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'activo', OLD.activo), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_tipos_hora_extra_del;
CREATE TRIGGER trg_aud_tipos_hora_extra_del AFTER DELETE ON tipos_hora_extra
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_hora_extra', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'activo', OLD.activo), NULL);
END;

DROP TABLE asistencia_hora_extra_tipificada;
DROP TABLE parametros_franja_nocturna;
ALTER TABLE tipos_hora_extra DROP INDEX uk_the_clase;
ALTER TABLE tipos_hora_extra DROP COLUMN clase;
