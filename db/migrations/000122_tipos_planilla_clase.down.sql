-- Down de 000122. Orden: auditoría sin columnas nuevas -> tabla puente -> seed -> columnas.
DROP TRIGGER IF EXISTS trg_aud_tipos_planilla_ins;
CREATE TRIGGER trg_aud_tipos_planilla_ins AFTER INSERT ON tipos_planilla
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_planilla', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre));
END;

DROP TRIGGER IF EXISTS trg_aud_tipos_planilla_upd;
CREATE TRIGGER trg_aud_tipos_planilla_upd AFTER UPDATE ON tipos_planilla
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_planilla', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre));
END;

DROP TRIGGER IF EXISTS trg_aud_tipos_planilla_del;
CREATE TRIGGER trg_aud_tipos_planilla_del AFTER DELETE ON tipos_planilla
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_planilla', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre), NULL);
END;

DROP TABLE IF EXISTS tipo_planilla_concepto;

-- Solo borra los tipos sembrados que ningún período usa (FK de periodos_planilla).
DELETE FROM tipos_planilla
WHERE codigo IN ('mensual','quincenal','aguinaldo','salario_escolar','liquidacion')
  AND NOT EXISTS (SELECT 1 FROM periodos_planilla p WHERE p.tipo_planilla_id = tipos_planilla.id);

ALTER TABLE tipos_planilla DROP CHECK chk_tp_fraccion;
ALTER TABLE tipos_planilla
  DROP COLUMN fraccion_mensual,
  DROP COLUMN clase;