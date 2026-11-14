ALTER TABLE formatos_exportacion
  ADD COLUMN reporte_oficial ENUM('ccss','ins','hacienda') NULL COMMENT 'Reporte oficial que genera el formato (NULL = no es reporte oficial)',
  ADD COLUMN columna_total VARCHAR(64) NULL COMMENT 'Columna cuya suma queda en exportaciones_generadas.total_monto';

CREATE TABLE formatos_exportacion_columnas (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  formato_exportacion_id BIGINT UNSIGNED NOT NULL COMMENT 'Formato',
  orden INT NOT NULL COMMENT 'Posición de la columna en el archivo',
  columna_origen VARCHAR(64) NOT NULL COMMENT 'Columna de la vista del reporte',
  encabezado VARCHAR(100) NOT NULL COMMENT 'Encabezado oficial',
  CONSTRAINT fk_fec_formato FOREIGN KEY (formato_exportacion_id) REFERENCES formatos_exportacion(id),
  CONSTRAINT chk_fec_orden CHECK (orden > 0),
  UNIQUE KEY uk_fec_orden (formato_exportacion_id, orden),
  UNIQUE KEY uk_fec_columna (formato_exportacion_id, columna_origen)
) ENGINE=InnoDB COMMENT='Layout configurable de cada formato de exportación (lo entrega RR. HH./Contabilidad)';

INSERT INTO formatos_exportacion (codigo, nombre, tipo_destino, reporte_oficial, columna_total)
SELECT v.codigo, v.nombre, 'autoridad', v.rep, v.total FROM (
  SELECT 'REPORTE_CCSS' AS codigo, 'Planilla CCSS (SICERE)' AS nombre, 'ccss' AS rep, 'base_ccss' AS total
  UNION ALL SELECT 'REPORTE_INS', 'Planilla INS (riesgos del trabajo)', 'ins', 'salario_bruto'
  UNION ALL SELECT 'REPORTE_HACIENDA', 'Retenciones de renta (Hacienda)', 'hacienda', 'renta_neta'
) v WHERE NOT EXISTS (SELECT 1 FROM formatos_exportacion f WHERE f.codigo = v.codigo);

CREATE VIEW v_reporte_ccss AS
SELECT pde.periodo_planilla_id, pde.empleado_id,
       e.numero_identificacion AS identificacion, e.nombre_completo,
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
       e.numero_identificacion AS identificacion, e.nombre_completo,
       pde.total_ingresos AS salario_bruto, pde.base_ccss
FROM planilla_detalle_empleado pde
JOIN empleados e ON e.id = pde.empleado_id;

CREATE VIEW v_reporte_renta AS
SELECT pde.periodo_planilla_id, pde.empleado_id,
       e.numero_identificacion AS identificacion, e.nombre_completo,
       pde.base_renta, pde.renta_bruta, pde.creditos_fiscales_aplicados, pde.renta_neta
FROM planilla_detalle_empleado pde
JOIN empleados e ON e.id = pde.empleado_id;

DROP TRIGGER IF EXISTS trg_aud_formatos_exportacion_ins;
CREATE TRIGGER trg_aud_formatos_exportacion_ins AFTER INSERT ON formatos_exportacion
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('formatos_exportacion', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'tipo_destino', NEW.tipo_destino, 'reporte_oficial', NEW.reporte_oficial, 'columna_total', NEW.columna_total));
END;

DROP TRIGGER IF EXISTS trg_aud_formatos_exportacion_upd;
CREATE TRIGGER trg_aud_formatos_exportacion_upd AFTER UPDATE ON formatos_exportacion
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('formatos_exportacion', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'tipo_destino', OLD.tipo_destino, 'reporte_oficial', OLD.reporte_oficial, 'columna_total', OLD.columna_total), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'tipo_destino', NEW.tipo_destino, 'reporte_oficial', NEW.reporte_oficial, 'columna_total', NEW.columna_total));
END;

DROP TRIGGER IF EXISTS trg_aud_formatos_exportacion_del;
CREATE TRIGGER trg_aud_formatos_exportacion_del AFTER DELETE ON formatos_exportacion
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('formatos_exportacion', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'tipo_destino', OLD.tipo_destino, 'reporte_oficial', OLD.reporte_oficial, 'columna_total', OLD.columna_total), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_formatos_exportacion_columnas_ins;
CREATE TRIGGER trg_aud_formatos_exportacion_columnas_ins AFTER INSERT ON formatos_exportacion_columnas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('formatos_exportacion_columnas', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'formato_exportacion_id', NEW.formato_exportacion_id, 'orden', NEW.orden, 'columna_origen', NEW.columna_origen, 'encabezado', NEW.encabezado));
END;

DROP TRIGGER IF EXISTS trg_aud_formatos_exportacion_columnas_upd;
CREATE TRIGGER trg_aud_formatos_exportacion_columnas_upd AFTER UPDATE ON formatos_exportacion_columnas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('formatos_exportacion_columnas', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'formato_exportacion_id', OLD.formato_exportacion_id, 'orden', OLD.orden, 'columna_origen', OLD.columna_origen, 'encabezado', OLD.encabezado), JSON_OBJECT('id', NEW.id, 'formato_exportacion_id', NEW.formato_exportacion_id, 'orden', NEW.orden, 'columna_origen', NEW.columna_origen, 'encabezado', NEW.encabezado));
END;

DROP TRIGGER IF EXISTS trg_aud_formatos_exportacion_columnas_del;
CREATE TRIGGER trg_aud_formatos_exportacion_columnas_del AFTER DELETE ON formatos_exportacion_columnas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('formatos_exportacion_columnas', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'formato_exportacion_id', OLD.formato_exportacion_id, 'orden', OLD.orden, 'columna_origen', OLD.columna_origen, 'encabezado', OLD.encabezado), NULL);
END;

