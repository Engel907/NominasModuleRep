ALTER TABLE parametros_cargas_sociales
  MODIFY rubro VARCHAR(60) NOT NULL COMMENT 'Etiqueta legible del componente (SEM, IVM, Banco Popular, INA, …); no se usa para calcular',
  MODIFY porcentaje_patronal DECIMAL(6,4) NOT NULL COMMENT 'Porcentaje que aporta el patrono sobre la base CCSS',
  MODIFY porcentaje_obrero DECIMAL(6,4) NOT NULL COMMENT 'Porcentaje que se deduce al trabajador sobre la base CCSS',
  ADD COLUMN regimen_ccss_id BIGINT UNSIGNED NULL COMMENT 'Régimen CCSS al que aplica la tasa; NULL = todo régimen sin fila propia' AFTER concepto_salarial_id,
  ADD COLUMN regimen_ccss_clave BIGINT UNSIGNED GENERATED ALWAYS AS (IFNULL(regimen_ccss_id, 0)) STORED COMMENT 'Columna técnica: soporta uk_pcs_una_vigente con régimen nulo',
  ADD CONSTRAINT fk_pcs_regimen_ccss FOREIGN KEY (regimen_ccss_id) REFERENCES regimenes_ccss(id),
  DROP INDEX uk_pcs_una_vigente,
  ADD UNIQUE KEY uk_pcs_una_vigente (concepto_salarial_id, regimen_ccss_clave, es_vigente),
  COMMENT = 'Tasas obrera y patronal de cada concepto de carga social, por régimen CCSS y con vigencia';

DROP TRIGGER trg_parametros_cargas_sociales_inmut_bu;
DROP TRIGGER trg_parametros_cargas_sociales_inmut_bd;

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
     OR NOT (NEW.regimen_ccss_id <=> OLD.regimen_ccss_id)
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
  VALUES ('parametros_cargas_sociales', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'rubro', NEW.rubro, 'concepto_salarial_id', NEW.concepto_salarial_id, 'regimen_ccss_id', NEW.regimen_ccss_id, 'porcentaje_patronal', NEW.porcentaje_patronal, 'porcentaje_obrero', NEW.porcentaje_obrero, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'fuente_legal', NEW.fuente_legal, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_cargas_sociales_upd;
CREATE TRIGGER trg_aud_parametros_cargas_sociales_upd AFTER UPDATE ON parametros_cargas_sociales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_cargas_sociales', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'rubro', OLD.rubro, 'concepto_salarial_id', OLD.concepto_salarial_id, 'regimen_ccss_id', OLD.regimen_ccss_id, 'porcentaje_patronal', OLD.porcentaje_patronal, 'porcentaje_obrero', OLD.porcentaje_obrero, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'fuente_legal', OLD.fuente_legal, 'es_vigente', OLD.es_vigente), JSON_OBJECT('id', NEW.id, 'rubro', NEW.rubro, 'concepto_salarial_id', NEW.concepto_salarial_id, 'regimen_ccss_id', NEW.regimen_ccss_id, 'porcentaje_patronal', NEW.porcentaje_patronal, 'porcentaje_obrero', NEW.porcentaje_obrero, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'fuente_legal', NEW.fuente_legal, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_cargas_sociales_del;
CREATE TRIGGER trg_aud_parametros_cargas_sociales_del AFTER DELETE ON parametros_cargas_sociales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_cargas_sociales', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'rubro', OLD.rubro, 'concepto_salarial_id', OLD.concepto_salarial_id, 'regimen_ccss_id', OLD.regimen_ccss_id, 'porcentaje_patronal', OLD.porcentaje_patronal, 'porcentaje_obrero', OLD.porcentaje_obrero, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'fuente_legal', OLD.fuente_legal, 'es_vigente', OLD.es_vigente), NULL);
END;

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

UPDATE conceptos_salariales
   SET tipo_calculo = 'formula'
 WHERE codigo IN ('INS_RT', 'BANPOP_PAT', 'FCL_PAT') AND tipo_calculo = 'captura_manual';

UPDATE conceptos_salariales
   SET activo = 0
 WHERE codigo IN ('BANPOP_PAT', 'FCL_PAT');

INSERT INTO concepto_salarial_formula (concepto_salarial_id, monto_fijo, expresion_formula, fecha_inicio_vigencia, fecha_fin_vigencia)
SELECT cs.id, NULL, JSON_OBJECT('tipo', 'carga_social', 'lado', 'patronal', 'usa', 'parametros_cargas_sociales'), '2026-01-01', NULL
FROM conceptos_salariales cs
WHERE cs.codigo IN ('INS_RT', 'BANPOP_PAT', 'FCL_PAT')
  AND NOT EXISTS (SELECT 1 FROM concepto_salarial_formula f WHERE f.concepto_salarial_id = cs.id);
