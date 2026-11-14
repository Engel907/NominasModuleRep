DROP VIEW v_reporte_renta;
DROP VIEW v_reporte_ins;
DROP VIEW v_reporte_ccss;
DROP TRIGGER trg_aud_formatos_exportacion_columnas_ins;
DROP TRIGGER trg_aud_formatos_exportacion_columnas_upd;
DROP TRIGGER trg_aud_formatos_exportacion_columnas_del;
DROP TABLE formatos_exportacion_columnas;
DELETE FROM formatos_exportacion WHERE codigo IN ('REPORTE_CCSS', 'REPORTE_INS', 'REPORTE_HACIENDA');
ALTER TABLE formatos_exportacion DROP COLUMN reporte_oficial, DROP COLUMN columna_total;

DROP TRIGGER IF EXISTS trg_aud_formatos_exportacion_ins;
CREATE TRIGGER trg_aud_formatos_exportacion_ins AFTER INSERT ON formatos_exportacion
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('formatos_exportacion', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'tipo_destino', NEW.tipo_destino));
END;

DROP TRIGGER IF EXISTS trg_aud_formatos_exportacion_upd;
CREATE TRIGGER trg_aud_formatos_exportacion_upd AFTER UPDATE ON formatos_exportacion
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('formatos_exportacion', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'tipo_destino', OLD.tipo_destino), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'tipo_destino', NEW.tipo_destino));
END;

DROP TRIGGER IF EXISTS trg_aud_formatos_exportacion_del;
CREATE TRIGGER trg_aud_formatos_exportacion_del AFTER DELETE ON formatos_exportacion
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('formatos_exportacion', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'tipo_destino', OLD.tipo_destino), NULL);
END;

