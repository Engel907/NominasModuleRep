ALTER TABLE terceros
  MODIFY tipo ENUM('juzgado','entidad_financiera','asociacion_solidarista','sindicato','aseguradora','cooperativa','otro') NOT NULL COMMENT 'Categoría del tercero',
  COMMENT = 'Terceros externos: juzgados, entidades financieras, asociaciones, sindicatos, aseguradoras, cooperativas';

CREATE TABLE empleado_conceptos_asignados (
  id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador de la asignación',
  empleado_id            BIGINT UNSIGNED NOT NULL COMMENT 'Empleado al que se aplica el concepto en cada planilla',
  concepto_salarial_id   BIGINT UNSIGNED NOT NULL COMMENT 'Concepto con fórmula asignacion_empleado (ingreso o deducción)',
  modo                   ENUM('monto','porcentaje') NOT NULL COMMENT 'monto = valor mensual (se prorratea en períodos de fracción de mes); porcentaje = valor % sobre la base que define la fórmula del concepto',
  valor                  DECIMAL(14,4) NOT NULL COMMENT 'Monto mensual o porcentaje (5.0000 = 5 %)',
  tercero_id             BIGINT UNSIGNED NULL COMMENT 'Destinatario de la deducción (asociación, sindicato, aseguradora…)',
  referencia             VARCHAR(60) NULL COMMENT 'Número de afiliación, póliza u operación del tercero',
  observacion            VARCHAR(255) NULL COMMENT 'Nota libre',
  fecha_inicio_vigencia  DATE NOT NULL COMMENT 'Desde cuándo se aplica',
  fecha_fin_vigencia     DATE NULL COMMENT 'NULL = vigente',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED COMMENT 'Columna técnica: soporta uk_eca_una_vigente',
  creado_por             BIGINT UNSIGNED NOT NULL COMMENT 'Usuario que registró la asignación',
  created_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Fecha/hora de registro',
  CONSTRAINT fk_eca_empleado FOREIGN KEY (empleado_id) REFERENCES empleados(id),
  CONSTRAINT fk_eca_concepto FOREIGN KEY (concepto_salarial_id) REFERENCES conceptos_salariales(id),
  CONSTRAINT fk_eca_tercero FOREIGN KEY (tercero_id) REFERENCES terceros(id),
  CONSTRAINT fk_eca_creador FOREIGN KEY (creado_por) REFERENCES usuarios(id),
  CONSTRAINT chk_eca_valor CHECK (valor > 0 AND (modo <> 'porcentaje' OR valor <= 100)),
  CONSTRAINT chk_eca_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  UNIQUE KEY uk_eca_una_vigente (empleado_id, concepto_salarial_id, es_vigente),
  KEY ix_eca_vigencia (empleado_id, fecha_inicio_vigencia, fecha_fin_vigencia)
) ENGINE=InnoDB COMMENT='Conceptos recurrentes por empleado (asociación solidarista, sindicato, pólizas, ahorro, pluses), con vigencia';

CREATE TRIGGER trg_empleado_conceptos_asignados_inmut_bu BEFORE UPDATE ON empleado_conceptos_asignados
FOR EACH ROW
BEGIN
  IF OLD.fecha_fin_vigencia IS NOT NULL
     AND NEW.fecha_fin_vigencia IS NULL THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: no se puede reabrir una vigencia cerrada';
  END IF;
  IF NOT (NEW.id <=> OLD.id)
     OR NOT (NEW.empleado_id <=> OLD.empleado_id)
     OR NOT (NEW.concepto_salarial_id <=> OLD.concepto_salarial_id)
     OR NOT (NEW.modo <=> OLD.modo)
     OR NOT (NEW.valor <=> OLD.valor)
     OR NOT (NEW.tercero_id <=> OLD.tercero_id)
     OR NOT (NEW.referencia <=> OLD.referencia)
     OR NOT (NEW.observacion <=> OLD.observacion)
     OR NOT (NEW.fecha_inicio_vigencia <=> OLD.fecha_inicio_vigencia)
     OR NOT (NEW.creado_por <=> OLD.creado_por)
     OR NOT (NEW.created_at <=> OLD.created_at) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'registro_inmutable: empleado_conceptos_asignados solo permite cambiar fecha_fin_vigencia';
  END IF;
END;

CREATE TRIGGER trg_empleado_conceptos_asignados_inmut_bd BEFORE DELETE ON empleado_conceptos_asignados
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'registro_inmutable: empleado_conceptos_asignados no se borra; se cierra la vigencia';
END;


DROP TRIGGER IF EXISTS trg_aud_empleado_conceptos_asignados_ins;
CREATE TRIGGER trg_aud_empleado_conceptos_asignados_ins AFTER INSERT ON empleado_conceptos_asignados
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_conceptos_asignados', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'concepto_salarial_id', NEW.concepto_salarial_id, 'modo', NEW.modo, 'valor', NEW.valor, 'tercero_id', NEW.tercero_id, 'referencia', NEW.referencia, 'observacion', NEW.observacion, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente, 'creado_por', NEW.creado_por, 'created_at', NEW.created_at));
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_conceptos_asignados_upd;
CREATE TRIGGER trg_aud_empleado_conceptos_asignados_upd AFTER UPDATE ON empleado_conceptos_asignados
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_conceptos_asignados', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'concepto_salarial_id', OLD.concepto_salarial_id, 'modo', OLD.modo, 'valor', OLD.valor, 'tercero_id', OLD.tercero_id, 'referencia', OLD.referencia, 'observacion', OLD.observacion, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente, 'creado_por', OLD.creado_por, 'created_at', OLD.created_at), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'concepto_salarial_id', NEW.concepto_salarial_id, 'modo', NEW.modo, 'valor', NEW.valor, 'tercero_id', NEW.tercero_id, 'referencia', NEW.referencia, 'observacion', NEW.observacion, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente, 'creado_por', NEW.creado_por, 'created_at', NEW.created_at));
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_conceptos_asignados_del;
CREATE TRIGGER trg_aud_empleado_conceptos_asignados_del AFTER DELETE ON empleado_conceptos_asignados
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_conceptos_asignados', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'concepto_salarial_id', OLD.concepto_salarial_id, 'modo', OLD.modo, 'valor', OLD.valor, 'tercero_id', OLD.tercero_id, 'referencia', OLD.referencia, 'observacion', OLD.observacion, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente, 'creado_por', OLD.creado_por, 'created_at', OLD.created_at), NULL);
END;

UPDATE conceptos_salariales
   SET tipo_calculo = 'formula'
 WHERE codigo IN ('ASOC_SOLIDARISTA', 'SINDICATO') AND tipo_calculo = 'captura_manual';

INSERT INTO concepto_salarial_formula (concepto_salarial_id, monto_fijo, expresion_formula, fecha_inicio_vigencia, fecha_fin_vigencia)
SELECT cs.id, NULL, JSON_OBJECT('tipo', 'asignacion_empleado', 'base', 'bruto', 'usa', 'empleado_conceptos_asignados'), '2026-01-01', NULL
FROM conceptos_salariales cs
WHERE cs.codigo IN ('ASOC_SOLIDARISTA', 'SINDICATO')
  AND NOT EXISTS (SELECT 1 FROM concepto_salarial_formula f WHERE f.concepto_salarial_id = cs.id);
