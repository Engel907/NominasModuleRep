ALTER TABLE jornadas
  ADD COLUMN clase ENUM('diurna','mixta','nocturna') NULL COMMENT 'Clase legal de la jornada' AFTER nombre;

UPDATE jornadas SET clase = LOWER(codigo) WHERE codigo IN ('DIURNA', 'MIXTA', 'NOCTURNA');
UPDATE jornadas SET clase = 'diurna' WHERE clase IS NULL;

ALTER TABLE jornadas MODIFY clase ENUM('diurna','mixta','nocturna') NOT NULL COMMENT 'Clase legal de la jornada';

DROP TRIGGER trg_aud_jornadas_ins;
CREATE TRIGGER trg_aud_jornadas_ins AFTER INSERT ON jornadas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('jornadas', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'clase', NEW.clase, 'activo', NEW.activo));
END;

DROP TRIGGER trg_aud_jornadas_upd;
CREATE TRIGGER trg_aud_jornadas_upd AFTER UPDATE ON jornadas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('jornadas', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'clase', OLD.clase, 'activo', OLD.activo), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'clase', NEW.clase, 'activo', NEW.activo));
END;

DROP TRIGGER trg_aud_jornadas_del;
CREATE TRIGGER trg_aud_jornadas_del AFTER DELETE ON jornadas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('jornadas', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'clase', OLD.clase, 'activo', OLD.activo), NULL);
END;

