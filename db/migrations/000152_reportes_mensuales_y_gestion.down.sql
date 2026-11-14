DELETE FROM rol_permisos WHERE permiso_id IN (SELECT id FROM permisos WHERE codigo = 'reportes.gestion');
DELETE FROM permisos WHERE codigo = 'reportes.gestion';

DROP VIEW v_reporte_ccss;
CREATE VIEW v_reporte_ccss AS
SELECT pde.periodo_planilla_id, pde.empleado_id,
       HEX(e.numero_identificacion_cifrado) AS identificacion_cifrado_hex, e.numero_identificacion_kid AS identificacion_kid,
       e.nombre_completo,
       pde.base_ccss, pde.cuota_obrera_ccss, pde.cuota_patronal_ccss,
       cs.nombre AS rubro_patronal, pat.porcentaje AS porcentaje_patronal, pat.monto AS monto_patronal
FROM planilla_detalle_empleado pde
JOIN empleados e ON e.id = pde.empleado_id
LEFT JOIN planilla_detalle_patronal pat ON pat.planilla_detalle_empleado_id = pde.id
LEFT JOIN conceptos_salariales cs ON cs.id = pat.concepto_salarial_id;

DELETE FROM exportaciones_generadas WHERE periodo_planilla_id IS NULL;

ALTER TABLE exportaciones_generadas DROP FOREIGN KEY fk_eg_periodo;

ALTER TABLE exportaciones_generadas
  DROP CHECK chk_eg_alcance,
  DROP COLUMN anio,
  DROP COLUMN mes,
  MODIFY periodo_planilla_id BIGINT UNSIGNED NOT NULL COMMENT 'Corrida de planilla que originó esta exportación',
  ADD CONSTRAINT fk_eg_periodo FOREIGN KEY (periodo_planilla_id) REFERENCES periodos_planilla(id);

DROP TRIGGER IF EXISTS trg_aud_exportaciones_generadas_ins;
CREATE TRIGGER trg_aud_exportaciones_generadas_ins AFTER INSERT ON exportaciones_generadas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('exportaciones_generadas', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'periodo_planilla_id', NEW.periodo_planilla_id, 'formato_exportacion_id', NEW.formato_exportacion_id, 'fecha_generacion', NEW.fecha_generacion, 'ruta_archivo', NEW.ruta_archivo, 'hash_verificacion', NEW.hash_verificacion, 'generado_por_usuario_id', NEW.generado_por_usuario_id, 'nombre_archivo', NEW.nombre_archivo, 'cantidad_filas', NEW.cantidad_filas, 'total_monto', NEW.total_monto));
END;

DROP TRIGGER IF EXISTS trg_aud_exportaciones_generadas_upd;
CREATE TRIGGER trg_aud_exportaciones_generadas_upd AFTER UPDATE ON exportaciones_generadas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('exportaciones_generadas', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'periodo_planilla_id', OLD.periodo_planilla_id, 'formato_exportacion_id', OLD.formato_exportacion_id, 'fecha_generacion', OLD.fecha_generacion, 'ruta_archivo', OLD.ruta_archivo, 'hash_verificacion', OLD.hash_verificacion, 'generado_por_usuario_id', OLD.generado_por_usuario_id, 'nombre_archivo', OLD.nombre_archivo, 'cantidad_filas', OLD.cantidad_filas, 'total_monto', OLD.total_monto), JSON_OBJECT('id', NEW.id, 'periodo_planilla_id', NEW.periodo_planilla_id, 'formato_exportacion_id', NEW.formato_exportacion_id, 'fecha_generacion', NEW.fecha_generacion, 'ruta_archivo', NEW.ruta_archivo, 'hash_verificacion', NEW.hash_verificacion, 'generado_por_usuario_id', NEW.generado_por_usuario_id, 'nombre_archivo', NEW.nombre_archivo, 'cantidad_filas', NEW.cantidad_filas, 'total_monto', NEW.total_monto));
END;

DROP TRIGGER IF EXISTS trg_aud_exportaciones_generadas_del;
CREATE TRIGGER trg_aud_exportaciones_generadas_del AFTER DELETE ON exportaciones_generadas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('exportaciones_generadas', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'periodo_planilla_id', OLD.periodo_planilla_id, 'formato_exportacion_id', OLD.formato_exportacion_id, 'fecha_generacion', OLD.fecha_generacion, 'ruta_archivo', OLD.ruta_archivo, 'hash_verificacion', OLD.hash_verificacion, 'generado_por_usuario_id', OLD.generado_por_usuario_id, 'nombre_archivo', OLD.nombre_archivo, 'cantidad_filas', OLD.cantidad_filas, 'total_monto', OLD.total_monto), NULL);
END;

