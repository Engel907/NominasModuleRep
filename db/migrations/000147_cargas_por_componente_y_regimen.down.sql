DROP VIEW v_reporte_ccss;
CREATE VIEW v_reporte_ccss AS
SELECT pde.periodo_planilla_id, pde.empleado_id,
       HEX(e.numero_identificacion_cifrado) AS identificacion_cifrado_hex, e.numero_identificacion_kid AS identificacion_kid,
       e.nombre_completo,
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

DROP TRIGGER trg_parametros_cargas_sociales_inmut_bu;
DROP TRIGGER trg_parametros_cargas_sociales_inmut_bd;

ALTER TABLE parametros_cargas_sociales
  DROP INDEX uk_pcs_una_vigente,
  DROP FOREIGN KEY fk_pcs_regimen_ccss,
  DROP COLUMN regimen_ccss_clave,
  DROP COLUMN regimen_ccss_id,
  ADD UNIQUE KEY uk_pcs_una_vigente (concepto_salarial_id, es_vigente);

CREATE TRIGGER trg_parametros_cargas_sociales_inmut_bu BEFORE UPDATE ON parametros_cargas_sociales
FOR EACH ROW
BEGIN
  IF OLD.fecha_fin_vigencia IS NOT NULL
     AND NEW.fecha_fin_vigencia IS NULL THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: no se puede reabrir una vigencia cerrada';
  END IF;
  IF NOT (NEW.id <=> OLD.id)
     OR NOT (NEW.rubro <=> OLD.rubro)
     OR NOT (NEW.concepto_salarial_id <=> OLD.concepto_salarial_id)
     OR NOT (NEW.porcentaje_patronal <=> OLD.porcentaje_patronal)
     OR NOT (NEW.porcentaje_obrero <=> OLD.porcentaje_obrero)
     OR NOT (NEW.fecha_inicio_vigencia <=> OLD.fecha_inicio_vigencia)
     OR NOT (NEW.fuente_legal <=> OLD.fuente_legal) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_cargas_sociales solo permite cambiar fecha_fin_vigencia';
  END IF;
END;

CREATE TRIGGER trg_parametros_cargas_sociales_inmut_bd BEFORE DELETE ON parametros_cargas_sociales
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_cargas_sociales no se borra; se cierra la vigencia';
END;


DROP TRIGGER IF EXISTS trg_aud_parametros_cargas_sociales_ins;
CREATE TRIGGER trg_aud_parametros_cargas_sociales_ins AFTER INSERT ON parametros_cargas_sociales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_cargas_sociales', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'rubro', NEW.rubro, 'concepto_salarial_id', NEW.concepto_salarial_id, 'porcentaje_patronal', NEW.porcentaje_patronal, 'porcentaje_obrero', NEW.porcentaje_obrero, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'fuente_legal', NEW.fuente_legal, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_cargas_sociales_upd;
CREATE TRIGGER trg_aud_parametros_cargas_sociales_upd AFTER UPDATE ON parametros_cargas_sociales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_cargas_sociales', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'rubro', OLD.rubro, 'concepto_salarial_id', OLD.concepto_salarial_id, 'porcentaje_patronal', OLD.porcentaje_patronal, 'porcentaje_obrero', OLD.porcentaje_obrero, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'fuente_legal', OLD.fuente_legal, 'es_vigente', OLD.es_vigente), JSON_OBJECT('id', NEW.id, 'rubro', NEW.rubro, 'concepto_salarial_id', NEW.concepto_salarial_id, 'porcentaje_patronal', NEW.porcentaje_patronal, 'porcentaje_obrero', NEW.porcentaje_obrero, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'fuente_legal', NEW.fuente_legal, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_cargas_sociales_del;
CREATE TRIGGER trg_aud_parametros_cargas_sociales_del AFTER DELETE ON parametros_cargas_sociales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_cargas_sociales', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'rubro', OLD.rubro, 'concepto_salarial_id', OLD.concepto_salarial_id, 'porcentaje_patronal', OLD.porcentaje_patronal, 'porcentaje_obrero', OLD.porcentaje_obrero, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'fuente_legal', OLD.fuente_legal, 'es_vigente', OLD.es_vigente), NULL);
END;

UPDATE conceptos_salariales SET activo = 1 WHERE codigo IN ('BANPOP_PAT', 'FCL_PAT');
