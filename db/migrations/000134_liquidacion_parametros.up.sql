SET @fecha_vigencia_2026 = COALESCE(@fecha_vigencia_2026, '2026-01-01');

CREATE TABLE parametros_motivos_salida (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  motivo_salida ENUM('despido_sin_responsabilidad','despido_con_responsabilidad','despido_indirecto','renuncia','vencimiento_contrato','mutuo_acuerdo','fallecimiento') NOT NULL COMMENT 'Mismo ENUM que liquidaciones.motivo_salida',
  paga_preaviso TINYINT(1) NOT NULL COMMENT 'Corresponde preaviso',
  paga_cesantia TINYINT(1) NOT NULL COMMENT 'Corresponde cesantía',
  paga_vacaciones TINYINT(1) NOT NULL COMMENT 'Vacaciones proporcionales',
  paga_aguinaldo TINYINT(1) NOT NULL COMMENT 'Aguinaldo proporcional',
  paga_salario_escolar TINYINT(1) NOT NULL COMMENT 'Salario escolar proporcional',
  observacion VARCHAR(255) NULL COMMENT 'Notas',
  fuente_legal VARCHAR(150) NOT NULL COMMENT 'Base legal del valor',
  fecha_inicio_vigencia DATE NOT NULL COMMENT 'Inicio',
  fecha_fin_vigencia DATE NULL COMMENT 'NULL = vigente',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED COMMENT 'Técnica',
  creado_por INT NOT NULL COMMENT 'Usuario',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creación',
  CONSTRAINT chk_pms_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  UNIQUE KEY uk_pms_una_vigente (motivo_salida, es_vigente),
  KEY ix_pms_vigencia (motivo_salida, fecha_inicio_vigencia, fecha_fin_vigencia)
) ENGINE=InnoDB COMMENT='Qué rubros paga cada motivo de salida';

INSERT INTO parametros_motivos_salida
  (motivo_salida, paga_preaviso, paga_cesantia, paga_vacaciones, paga_aguinaldo, paga_salario_escolar, observacion, fuente_legal, fecha_inicio_vigencia, creado_por)
SELECT v.motivo, v.pre, v.ces, 1, 1, 1, v.obs, 'Código de Trabajo, arts. 28, 29, 31, 83 y 85', @fecha_vigencia_2026, 1
FROM (
  SELECT 'despido_sin_responsabilidad' AS motivo, 0 AS pre, 0 AS ces, 'Art. 81 CT: sin preaviso ni cesantía' AS obs
  UNION ALL SELECT 'despido_con_responsabilidad', 1, 1, 'Arts. 28 y 29 CT'
  UNION ALL SELECT 'despido_indirecto', 1, 1, 'Art. 83 CT: se equipara al despido con responsabilidad'
  UNION ALL SELECT 'renuncia', 0, 0, 'Renuncia: sin preaviso ni cesantía a cargo del patrono'
  UNION ALL SELECT 'vencimiento_contrato', 0, 0, 'Vencimiento de contrato a plazo (art. 31 CT)'
  UNION ALL SELECT 'mutuo_acuerdo', 0, 0, 'Según lo pactado'
  UNION ALL SELECT 'fallecimiento', 0, 0, 'Fallecimiento (art. 85 CT)'
) v
WHERE NOT EXISTS (
  SELECT 1 FROM parametros_motivos_salida p WHERE p.motivo_salida = v.motivo AND p.es_vigente = 1
);

ALTER TABLE parametros_cesantia_escala
  ADD COLUMN fuente_legal VARCHAR(150) NULL COMMENT 'Base legal del valor',
  ADD COLUMN creado_por INT NULL COMMENT 'Usuario que cargó el tramo (1 = carga inicial)';
UPDATE parametros_cesantia_escala
   SET fuente_legal = 'Código de Trabajo, art. 29', creado_por = 1
 WHERE fuente_legal IS NULL;
ALTER TABLE parametros_cesantia_escala
  MODIFY fuente_legal VARCHAR(150) NOT NULL COMMENT 'Base legal del valor',
  MODIFY creado_por INT NOT NULL COMMENT 'Usuario que cargó el tramo (1 = carga inicial)';

ALTER TABLE parametros_preaviso_escala
  ADD COLUMN fuente_legal VARCHAR(150) NULL COMMENT 'Base legal del valor';
UPDATE parametros_preaviso_escala
   SET fuente_legal = 'Código de Trabajo, art. 28'
 WHERE fuente_legal IS NULL;
ALTER TABLE parametros_preaviso_escala
  MODIFY fuente_legal VARCHAR(150) NOT NULL COMMENT 'Base legal del valor';

INSERT INTO parametros_cesantia_escala
  (regimen_laboral_id, anios_servicio_desde, anios_servicio_hasta, dias_por_anio, tope_anios, fecha_inicio_vigencia, fecha_fin_vigencia, fuente_legal, creado_por)
SELECT e.regimen_laboral_id, v.desde, v.hasta, v.dias, e.tope_anios, e.fecha_inicio_vigencia, NULL,
       'Código de Trabajo, art. 29', 1
FROM parametros_cesantia_escala e
JOIN (
  SELECT 0.00 AS desde, 0.25 AS hasta, 0.00 AS dias
  UNION ALL SELECT 0.25, 0.50, 7.00
  UNION ALL SELECT 0.50, 1.00, 14.00
) v ON 1 = 1
WHERE e.es_vigente = 1
  AND e.anios_servicio_desde = 1.00
  AND NOT EXISTS (
    SELECT 1 FROM (SELECT regimen_laboral_id, anios_servicio_desde FROM parametros_cesantia_escala WHERE es_vigente = 1) x
    WHERE x.regimen_laboral_id = e.regimen_laboral_id AND x.anios_servicio_desde = v.desde
  );

INSERT INTO parametros_generales
    (clave, valor, tipo, descripcion, fuente_legal, fecha_inicio_vigencia, fecha_fin_vigencia, creado_por)
SELECT v.clave, v.valor, 'entero', v.descripcion, v.fuente, @fecha_vigencia_2026, NULL, 1
FROM (
  SELECT 'LIQ_PROMEDIO_MESES' AS clave, '6' AS valor,
         'Liquidación: meses hacia atrás desde la salida para el salario promedio de preaviso y cesantía (base_cesantia)' AS descripcion,
         'Código de Trabajo, art. 30' AS fuente
  UNION ALL SELECT 'VENTANA_PROMEDIO_VACACIONES_SEMANAS', '26',
         'Liquidación: semanas hacia atrás desde la salida para el salario promedio de vacaciones (base_vacaciones)',
         'Configuración inicial'
) v
WHERE NOT EXISTS (
  SELECT 1 FROM parametros_generales g WHERE g.clave = v.clave AND g.es_vigente = 1
);

ALTER TABLE liquidaciones
  ADD COLUMN salario_diario_vacaciones DECIMAL(14,4) NOT NULL DEFAULT 0 COMMENT 'Salario diario promedio de la ventana de vacaciones',
  ADD COLUMN periodo_salario_escolar_desde DATE NULL COMMENT 'Inicio del período de salario escolar proporcional',
  ADD COLUMN total_base_salario_escolar_proporcional DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Base acumulada del salario escolar proporcional',
  ADD COLUMN monto_salario_escolar_proporcional DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Salario escolar proporcional';

CREATE TRIGGER trg_parametros_motivos_salida_inmut_bu BEFORE UPDATE ON parametros_motivos_salida
FOR EACH ROW
BEGIN
  IF OLD.fecha_fin_vigencia IS NOT NULL
     AND NEW.fecha_fin_vigencia IS NULL THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: no se puede reabrir una vigencia cerrada';
  END IF;
  IF NOT (NEW.id <=> OLD.id)
     OR NOT (NEW.motivo_salida <=> OLD.motivo_salida)
     OR NOT (NEW.paga_preaviso <=> OLD.paga_preaviso)
     OR NOT (NEW.paga_cesantia <=> OLD.paga_cesantia)
     OR NOT (NEW.paga_vacaciones <=> OLD.paga_vacaciones)
     OR NOT (NEW.paga_aguinaldo <=> OLD.paga_aguinaldo)
     OR NOT (NEW.paga_salario_escolar <=> OLD.paga_salario_escolar)
     OR NOT (NEW.observacion <=> OLD.observacion)
     OR NOT (NEW.fuente_legal <=> OLD.fuente_legal)
     OR NOT (NEW.fecha_inicio_vigencia <=> OLD.fecha_inicio_vigencia)
     OR NOT (NEW.creado_por <=> OLD.creado_por)
     OR NOT (NEW.created_at <=> OLD.created_at) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_motivos_salida solo permite cambiar fecha_fin_vigencia';
  END IF;
END;

CREATE TRIGGER trg_parametros_motivos_salida_inmut_bd BEFORE DELETE ON parametros_motivos_salida
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_motivos_salida no se borra; se cierra la vigencia';
END;

CREATE TRIGGER trg_parametros_cesantia_escala_inmut_bu BEFORE UPDATE ON parametros_cesantia_escala
FOR EACH ROW
BEGIN
  IF OLD.fecha_fin_vigencia IS NOT NULL
     AND NEW.fecha_fin_vigencia IS NULL THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: no se puede reabrir una vigencia cerrada';
  END IF;
  IF NOT (NEW.id <=> OLD.id)
     OR NOT (NEW.regimen_laboral_id <=> OLD.regimen_laboral_id)
     OR NOT (NEW.anios_servicio_desde <=> OLD.anios_servicio_desde)
     OR NOT (NEW.anios_servicio_hasta <=> OLD.anios_servicio_hasta)
     OR NOT (NEW.dias_por_anio <=> OLD.dias_por_anio)
     OR NOT (NEW.tope_anios <=> OLD.tope_anios)
     OR NOT (NEW.fecha_inicio_vigencia <=> OLD.fecha_inicio_vigencia)
     OR NOT (NEW.fuente_legal <=> OLD.fuente_legal)
     OR NOT (NEW.creado_por <=> OLD.creado_por) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_cesantia_escala solo permite cambiar fecha_fin_vigencia';
  END IF;
END;

CREATE TRIGGER trg_parametros_cesantia_escala_inmut_bd BEFORE DELETE ON parametros_cesantia_escala
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_cesantia_escala no se borra; se cierra la vigencia';
END;

CREATE TRIGGER trg_parametros_preaviso_escala_inmut_bu BEFORE UPDATE ON parametros_preaviso_escala
FOR EACH ROW
BEGIN
  IF OLD.fecha_fin_vigencia IS NOT NULL
     AND NEW.fecha_fin_vigencia IS NULL THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: no se puede reabrir una vigencia cerrada';
  END IF;
  IF NOT (NEW.id <=> OLD.id)
     OR NOT (NEW.regimen_laboral_id <=> OLD.regimen_laboral_id)
     OR NOT (NEW.antiguedad_meses_desde <=> OLD.antiguedad_meses_desde)
     OR NOT (NEW.antiguedad_meses_hasta <=> OLD.antiguedad_meses_hasta)
     OR NOT (NEW.dias_preaviso <=> OLD.dias_preaviso)
     OR NOT (NEW.fecha_inicio_vigencia <=> OLD.fecha_inicio_vigencia)
     OR NOT (NEW.fuente_legal <=> OLD.fuente_legal)
     OR NOT (NEW.creado_por <=> OLD.creado_por)
     OR NOT (NEW.created_at <=> OLD.created_at) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_preaviso_escala solo permite cambiar fecha_fin_vigencia';
  END IF;
END;

CREATE TRIGGER trg_parametros_preaviso_escala_inmut_bd BEFORE DELETE ON parametros_preaviso_escala
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_preaviso_escala no se borra; se cierra la vigencia';
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_motivos_salida_ins;
CREATE TRIGGER trg_aud_parametros_motivos_salida_ins AFTER INSERT ON parametros_motivos_salida
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_motivos_salida', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'motivo_salida', NEW.motivo_salida, 'paga_preaviso', NEW.paga_preaviso, 'paga_cesantia', NEW.paga_cesantia, 'paga_vacaciones', NEW.paga_vacaciones, 'paga_aguinaldo', NEW.paga_aguinaldo, 'paga_salario_escolar', NEW.paga_salario_escolar, 'observacion', NEW.observacion, 'fuente_legal', NEW.fuente_legal, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'creado_por', NEW.creado_por, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'created_at', NEW.created_at));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_motivos_salida_upd;
CREATE TRIGGER trg_aud_parametros_motivos_salida_upd AFTER UPDATE ON parametros_motivos_salida
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_motivos_salida', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'motivo_salida', OLD.motivo_salida, 'paga_preaviso', OLD.paga_preaviso, 'paga_cesantia', OLD.paga_cesantia, 'paga_vacaciones', OLD.paga_vacaciones, 'paga_aguinaldo', OLD.paga_aguinaldo, 'paga_salario_escolar', OLD.paga_salario_escolar, 'observacion', OLD.observacion, 'fuente_legal', OLD.fuente_legal, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'creado_por', OLD.creado_por, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'created_at', OLD.created_at), JSON_OBJECT('id', NEW.id, 'motivo_salida', NEW.motivo_salida, 'paga_preaviso', NEW.paga_preaviso, 'paga_cesantia', NEW.paga_cesantia, 'paga_vacaciones', NEW.paga_vacaciones, 'paga_aguinaldo', NEW.paga_aguinaldo, 'paga_salario_escolar', NEW.paga_salario_escolar, 'observacion', NEW.observacion, 'fuente_legal', NEW.fuente_legal, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'creado_por', NEW.creado_por, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'created_at', NEW.created_at));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_motivos_salida_del;
CREATE TRIGGER trg_aud_parametros_motivos_salida_del AFTER DELETE ON parametros_motivos_salida
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_motivos_salida', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'motivo_salida', OLD.motivo_salida, 'paga_preaviso', OLD.paga_preaviso, 'paga_cesantia', OLD.paga_cesantia, 'paga_vacaciones', OLD.paga_vacaciones, 'paga_aguinaldo', OLD.paga_aguinaldo, 'paga_salario_escolar', OLD.paga_salario_escolar, 'observacion', OLD.observacion, 'fuente_legal', OLD.fuente_legal, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'creado_por', OLD.creado_por, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'created_at', OLD.created_at), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_cesantia_escala_ins;
CREATE TRIGGER trg_aud_parametros_cesantia_escala_ins AFTER INSERT ON parametros_cesantia_escala
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_cesantia_escala', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'regimen_laboral_id', NEW.regimen_laboral_id, 'anios_servicio_desde', NEW.anios_servicio_desde, 'anios_servicio_hasta', NEW.anios_servicio_hasta, 'dias_por_anio', NEW.dias_por_anio, 'tope_anios', NEW.tope_anios, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'fuente_legal', NEW.fuente_legal, 'creado_por', NEW.creado_por));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_cesantia_escala_upd;
CREATE TRIGGER trg_aud_parametros_cesantia_escala_upd AFTER UPDATE ON parametros_cesantia_escala
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_cesantia_escala', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'regimen_laboral_id', OLD.regimen_laboral_id, 'anios_servicio_desde', OLD.anios_servicio_desde, 'anios_servicio_hasta', OLD.anios_servicio_hasta, 'dias_por_anio', OLD.dias_por_anio, 'tope_anios', OLD.tope_anios, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'fuente_legal', OLD.fuente_legal, 'creado_por', OLD.creado_por), JSON_OBJECT('id', NEW.id, 'regimen_laboral_id', NEW.regimen_laboral_id, 'anios_servicio_desde', NEW.anios_servicio_desde, 'anios_servicio_hasta', NEW.anios_servicio_hasta, 'dias_por_anio', NEW.dias_por_anio, 'tope_anios', NEW.tope_anios, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'fuente_legal', NEW.fuente_legal, 'creado_por', NEW.creado_por));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_cesantia_escala_del;
CREATE TRIGGER trg_aud_parametros_cesantia_escala_del AFTER DELETE ON parametros_cesantia_escala
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_cesantia_escala', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'regimen_laboral_id', OLD.regimen_laboral_id, 'anios_servicio_desde', OLD.anios_servicio_desde, 'anios_servicio_hasta', OLD.anios_servicio_hasta, 'dias_por_anio', OLD.dias_por_anio, 'tope_anios', OLD.tope_anios, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'fuente_legal', OLD.fuente_legal, 'creado_por', OLD.creado_por), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_preaviso_escala_ins;
CREATE TRIGGER trg_aud_parametros_preaviso_escala_ins AFTER INSERT ON parametros_preaviso_escala
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_preaviso_escala', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'regimen_laboral_id', NEW.regimen_laboral_id, 'antiguedad_meses_desde', NEW.antiguedad_meses_desde, 'antiguedad_meses_hasta', NEW.antiguedad_meses_hasta, 'dias_preaviso', NEW.dias_preaviso, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'fuente_legal', NEW.fuente_legal, 'creado_por', NEW.creado_por, 'created_at', NEW.created_at));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_preaviso_escala_upd;
CREATE TRIGGER trg_aud_parametros_preaviso_escala_upd AFTER UPDATE ON parametros_preaviso_escala
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_preaviso_escala', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'regimen_laboral_id', OLD.regimen_laboral_id, 'antiguedad_meses_desde', OLD.antiguedad_meses_desde, 'antiguedad_meses_hasta', OLD.antiguedad_meses_hasta, 'dias_preaviso', OLD.dias_preaviso, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'fuente_legal', OLD.fuente_legal, 'creado_por', OLD.creado_por, 'created_at', OLD.created_at), JSON_OBJECT('id', NEW.id, 'regimen_laboral_id', NEW.regimen_laboral_id, 'antiguedad_meses_desde', NEW.antiguedad_meses_desde, 'antiguedad_meses_hasta', NEW.antiguedad_meses_hasta, 'dias_preaviso', NEW.dias_preaviso, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'fuente_legal', NEW.fuente_legal, 'creado_por', NEW.creado_por, 'created_at', NEW.created_at));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_preaviso_escala_del;
CREATE TRIGGER trg_aud_parametros_preaviso_escala_del AFTER DELETE ON parametros_preaviso_escala
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_preaviso_escala', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'regimen_laboral_id', OLD.regimen_laboral_id, 'antiguedad_meses_desde', OLD.antiguedad_meses_desde, 'antiguedad_meses_hasta', OLD.antiguedad_meses_hasta, 'dias_preaviso', OLD.dias_preaviso, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'fuente_legal', OLD.fuente_legal, 'creado_por', OLD.creado_por, 'created_at', OLD.created_at), NULL);
END;

