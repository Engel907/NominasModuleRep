ALTER TABLE exportaciones_generadas DROP FOREIGN KEY fk_eg_periodo;

ALTER TABLE exportaciones_generadas
  MODIFY periodo_planilla_id BIGINT UNSIGNED NULL COMMENT 'Corrida de planilla del archivo; NULL si el archivo consolida un mes',
  ADD COLUMN anio SMALLINT UNSIGNED NULL COMMENT 'Año del archivo mensual' AFTER periodo_planilla_id,
  ADD COLUMN mes TINYINT UNSIGNED NULL COMMENT 'Mes (1-12) del archivo mensual' AFTER anio,
  ADD CONSTRAINT fk_eg_periodo FOREIGN KEY (periodo_planilla_id) REFERENCES periodos_planilla(id),
  ADD CONSTRAINT chk_eg_alcance CHECK ((periodo_planilla_id IS NOT NULL AND anio IS NULL AND mes IS NULL)
                                       OR (periodo_planilla_id IS NULL AND anio IS NOT NULL AND mes BETWEEN 1 AND 12));

DROP TRIGGER IF EXISTS trg_aud_exportaciones_generadas_ins;
CREATE TRIGGER trg_aud_exportaciones_generadas_ins AFTER INSERT ON exportaciones_generadas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('exportaciones_generadas', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'periodo_planilla_id', NEW.periodo_planilla_id, 'anio', NEW.anio, 'mes', NEW.mes, 'formato_exportacion_id', NEW.formato_exportacion_id, 'fecha_generacion', NEW.fecha_generacion, 'ruta_archivo', NEW.ruta_archivo, 'hash_verificacion', NEW.hash_verificacion, 'generado_por_usuario_id', NEW.generado_por_usuario_id, 'nombre_archivo', NEW.nombre_archivo, 'cantidad_filas', NEW.cantidad_filas, 'total_monto', NEW.total_monto));
END;

DROP TRIGGER IF EXISTS trg_aud_exportaciones_generadas_upd;
CREATE TRIGGER trg_aud_exportaciones_generadas_upd AFTER UPDATE ON exportaciones_generadas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('exportaciones_generadas', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'periodo_planilla_id', OLD.periodo_planilla_id, 'anio', OLD.anio, 'mes', OLD.mes, 'formato_exportacion_id', OLD.formato_exportacion_id, 'fecha_generacion', OLD.fecha_generacion, 'ruta_archivo', OLD.ruta_archivo, 'hash_verificacion', OLD.hash_verificacion, 'generado_por_usuario_id', OLD.generado_por_usuario_id, 'nombre_archivo', OLD.nombre_archivo, 'cantidad_filas', OLD.cantidad_filas, 'total_monto', OLD.total_monto), JSON_OBJECT('id', NEW.id, 'periodo_planilla_id', NEW.periodo_planilla_id, 'anio', NEW.anio, 'mes', NEW.mes, 'formato_exportacion_id', NEW.formato_exportacion_id, 'fecha_generacion', NEW.fecha_generacion, 'ruta_archivo', NEW.ruta_archivo, 'hash_verificacion', NEW.hash_verificacion, 'generado_por_usuario_id', NEW.generado_por_usuario_id, 'nombre_archivo', NEW.nombre_archivo, 'cantidad_filas', NEW.cantidad_filas, 'total_monto', NEW.total_monto));
END;

DROP TRIGGER IF EXISTS trg_aud_exportaciones_generadas_del;
CREATE TRIGGER trg_aud_exportaciones_generadas_del AFTER DELETE ON exportaciones_generadas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('exportaciones_generadas', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'periodo_planilla_id', OLD.periodo_planilla_id, 'anio', OLD.anio, 'mes', OLD.mes, 'formato_exportacion_id', OLD.formato_exportacion_id, 'fecha_generacion', OLD.fecha_generacion, 'ruta_archivo', OLD.ruta_archivo, 'hash_verificacion', OLD.hash_verificacion, 'generado_por_usuario_id', OLD.generado_por_usuario_id, 'nombre_archivo', OLD.nombre_archivo, 'cantidad_filas', OLD.cantidad_filas, 'total_monto', OLD.total_monto), NULL);
END;

DROP VIEW v_reporte_ccss;
CREATE VIEW v_reporte_ccss AS
SELECT pde.periodo_planilla_id, pde.empleado_id,
       HEX(e.numero_identificacion_cifrado) AS identificacion_cifrado_hex, e.numero_identificacion_kid AS identificacion_kid,
       e.nombre_completo,
       pde.total_ingresos AS salario_bruto, pde.base_ccss, pde.cuota_obrera_ccss, pde.cuota_patronal_ccss
FROM planilla_detalle_empleado pde
JOIN empleados e ON e.id = pde.empleado_id;

INSERT INTO permisos (codigo, nombre, modulo)
SELECT 'reportes.gestion', 'Consultar los reportes de gestión de la planilla', 'integraciones'
WHERE NOT EXISTS (SELECT 1 FROM permisos p WHERE p.codigo = 'reportes.gestion');

INSERT INTO rol_permisos (rol_id, permiso_id)
SELECT r.id, p.id
FROM roles r
JOIN permisos p ON p.codigo = 'reportes.gestion'
WHERE r.codigo IN ('RRHH', 'PLANILLAS_PREPARA', 'PLANILLAS_APRUEBA', 'CONTABILIDAD', 'TESORERIA', 'AUDITOR')
  AND NOT EXISTS (SELECT 1 FROM rol_permisos rp WHERE rp.rol_id = r.id AND rp.permiso_id = p.id);

UPDATE usuarios u
SET u.permisos_version = u.permisos_version + 1
WHERE EXISTS (SELECT 1 FROM usuario_roles ur JOIN roles r ON r.id = ur.rol_id
              WHERE ur.usuario_id = u.id AND ur.fecha_fin_vigencia IS NULL
                AND r.codigo IN ('RRHH', 'PLANILLAS_PREPARA', 'PLANILLAS_APRUEBA', 'CONTABILIDAD', 'TESORERIA', 'AUDITOR'));
