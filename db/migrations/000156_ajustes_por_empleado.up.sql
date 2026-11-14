CREATE TABLE empleado_concepto_formula (
  id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  empleado_id            BIGINT UNSIGNED NOT NULL COMMENT 'Empleado al que aplica la fórmula propia',
  concepto_salarial_id   BIGINT UNSIGNED NOT NULL COMMENT 'Concepto cuya fórmula general se reemplaza para este empleado',
  monto_fijo             DECIMAL(14,4) NULL COMMENT 'Monto fijo propio (conceptos de tipo_calculo = fijo)',
  expresion_formula      TEXT NULL COMMENT 'Fórmula propia (conceptos de tipo_calculo = formula), mismo formato que concepto_salarial_formula',
  fuente_legal           VARCHAR(150) NOT NULL COMMENT 'Fundamento del trato individual (contrato, convención, resolución)',
  fecha_inicio_vigencia  DATE NOT NULL COMMENT 'Desde cuándo rige para el empleado',
  fecha_fin_vigencia     DATE NULL COMMENT 'NULL = vigente; al cerrarse el empleado vuelve a la fórmula general',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED
    COMMENT 'Técnica: una vigente por empleado y concepto',
  creado_por             BIGINT UNSIGNED NOT NULL COMMENT 'Usuario que aprobó la propuesta',
  created_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Registro',
  CONSTRAINT fk_ecf_empleado FOREIGN KEY (empleado_id) REFERENCES empleados(id),
  CONSTRAINT fk_ecf_concepto FOREIGN KEY (concepto_salarial_id) REFERENCES conceptos_salariales(id),
  CONSTRAINT fk_ecf_usuario  FOREIGN KEY (creado_por) REFERENCES usuarios(id),
  CONSTRAINT chk_ecf_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  CONSTRAINT chk_ecf_valor CHECK ((monto_fijo IS NULL) <> (expresion_formula IS NULL)),
  UNIQUE KEY uk_ecf_una_vigente (empleado_id, concepto_salarial_id, es_vigente),
  KEY ix_ecf_vigencia (empleado_id, concepto_salarial_id, fecha_inicio_vigencia, fecha_fin_vigencia)
) ENGINE=InnoDB COMMENT='Fórmula o monto de un concepto propio de un empleado; prevalece sobre concepto_salarial_formula';

CREATE TABLE empleado_parametros_generales (
  id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  empleado_id            BIGINT UNSIGNED NOT NULL COMMENT 'Empleado al que aplica el valor propio',
  clave                  VARCHAR(80)  NOT NULL COMMENT 'Clave de parametros_generales que se reemplaza para el empleado',
  valor                  VARCHAR(255) NOT NULL COMMENT 'Valor serializado como texto; se interpreta según tipo',
  tipo                   ENUM('entero','decimal','porcentaje','texto','fecha','booleano') NOT NULL COMMENT 'Tipo con el que se interpreta valor',
  fuente_legal           VARCHAR(150) NOT NULL COMMENT 'Fundamento del trato individual',
  fecha_inicio_vigencia  DATE NOT NULL COMMENT 'Desde cuándo rige para el empleado',
  fecha_fin_vigencia     DATE NULL COMMENT 'NULL = vigente; al cerrarse el empleado vuelve al valor general',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED
    COMMENT 'Técnica: una vigente por empleado y clave',
  creado_por             BIGINT UNSIGNED NOT NULL COMMENT 'Usuario que aprobó la propuesta',
  created_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Registro',
  CONSTRAINT fk_epg_empleado FOREIGN KEY (empleado_id) REFERENCES empleados(id),
  CONSTRAINT fk_epg_usuario  FOREIGN KEY (creado_por) REFERENCES usuarios(id),
  CONSTRAINT chk_epg_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  UNIQUE KEY uk_epg_una_vigente (empleado_id, clave, es_vigente),
  KEY ix_epg_vigencia (empleado_id, clave, fecha_inicio_vigencia, fecha_fin_vigencia)
) ENGINE=InnoDB COMMENT='Valor de un parámetro general propio de un empleado; prevalece sobre parametros_generales';

ALTER TABLE planilla_detalle_concepto
  ADD COLUMN empleado_concepto_formula_id BIGINT UNSIGNED NULL
    COMMENT 'Fórmula propia del empleado usada en la línea (excluye concepto_salarial_formula_id)' AFTER concepto_salarial_formula_id,
  ADD CONSTRAINT fk_pdc_formula_empleado FOREIGN KEY (empleado_concepto_formula_id) REFERENCES empleado_concepto_formula(id),
  ADD CONSTRAINT chk_pdc_una_formula CHECK (concepto_salarial_formula_id IS NULL OR empleado_concepto_formula_id IS NULL);

CREATE TRIGGER trg_empleado_concepto_formula_inmut_bu BEFORE UPDATE ON empleado_concepto_formula
FOR EACH ROW
BEGIN
  IF OLD.fecha_fin_vigencia IS NOT NULL
     AND NEW.fecha_fin_vigencia IS NULL THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: no se puede reabrir una vigencia cerrada';
  END IF;
  IF NOT (NEW.id <=> OLD.id)
     OR NOT (NEW.empleado_id <=> OLD.empleado_id)
     OR NOT (NEW.concepto_salarial_id <=> OLD.concepto_salarial_id)
     OR NOT (NEW.monto_fijo <=> OLD.monto_fijo)
     OR NOT (NEW.expresion_formula <=> OLD.expresion_formula)
     OR NOT (NEW.fuente_legal <=> OLD.fuente_legal)
     OR NOT (NEW.fecha_inicio_vigencia <=> OLD.fecha_inicio_vigencia)
     OR NOT (NEW.creado_por <=> OLD.creado_por)
     OR NOT (NEW.created_at <=> OLD.created_at) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: empleado_concepto_formula solo permite cambiar fecha_fin_vigencia';
  END IF;
END;

CREATE TRIGGER trg_empleado_concepto_formula_inmut_bd BEFORE DELETE ON empleado_concepto_formula
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: empleado_concepto_formula no se borra; se cierra la vigencia';
END;

CREATE TRIGGER trg_empleado_parametros_generales_inmut_bu BEFORE UPDATE ON empleado_parametros_generales
FOR EACH ROW
BEGIN
  IF OLD.fecha_fin_vigencia IS NOT NULL
     AND NEW.fecha_fin_vigencia IS NULL THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: no se puede reabrir una vigencia cerrada';
  END IF;
  IF NOT (NEW.id <=> OLD.id)
     OR NOT (NEW.empleado_id <=> OLD.empleado_id)
     OR NOT (NEW.clave <=> OLD.clave)
     OR NOT (NEW.valor <=> OLD.valor)
     OR NOT (NEW.tipo <=> OLD.tipo)
     OR NOT (NEW.fuente_legal <=> OLD.fuente_legal)
     OR NOT (NEW.fecha_inicio_vigencia <=> OLD.fecha_inicio_vigencia)
     OR NOT (NEW.creado_por <=> OLD.creado_por)
     OR NOT (NEW.created_at <=> OLD.created_at) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: empleado_parametros_generales solo permite cambiar fecha_fin_vigencia';
  END IF;
END;

CREATE TRIGGER trg_empleado_parametros_generales_inmut_bd BEFORE DELETE ON empleado_parametros_generales
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: empleado_parametros_generales no se borra; se cierra la vigencia';
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_concepto_formula_ins;
CREATE TRIGGER trg_aud_empleado_concepto_formula_ins AFTER INSERT ON empleado_concepto_formula
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_concepto_formula', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'concepto_salarial_id', NEW.concepto_salarial_id, 'monto_fijo', NEW.monto_fijo, 'expresion_formula', NEW.expresion_formula, 'fuente_legal', NEW.fuente_legal, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'creado_por', NEW.creado_por, 'created_at', NEW.created_at));
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_concepto_formula_upd;
CREATE TRIGGER trg_aud_empleado_concepto_formula_upd AFTER UPDATE ON empleado_concepto_formula
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_concepto_formula', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'concepto_salarial_id', OLD.concepto_salarial_id, 'monto_fijo', OLD.monto_fijo, 'expresion_formula', OLD.expresion_formula, 'fuente_legal', OLD.fuente_legal, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'creado_por', OLD.creado_por, 'created_at', OLD.created_at), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'concepto_salarial_id', NEW.concepto_salarial_id, 'monto_fijo', NEW.monto_fijo, 'expresion_formula', NEW.expresion_formula, 'fuente_legal', NEW.fuente_legal, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'creado_por', NEW.creado_por, 'created_at', NEW.created_at));
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_concepto_formula_del;
CREATE TRIGGER trg_aud_empleado_concepto_formula_del AFTER DELETE ON empleado_concepto_formula
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_concepto_formula', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'concepto_salarial_id', OLD.concepto_salarial_id, 'monto_fijo', OLD.monto_fijo, 'expresion_formula', OLD.expresion_formula, 'fuente_legal', OLD.fuente_legal, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'creado_por', OLD.creado_por, 'created_at', OLD.created_at), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_parametros_generales_ins;
CREATE TRIGGER trg_aud_empleado_parametros_generales_ins AFTER INSERT ON empleado_parametros_generales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_parametros_generales', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'clave', NEW.clave, 'valor', NEW.valor, 'tipo', NEW.tipo, 'fuente_legal', NEW.fuente_legal, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'creado_por', NEW.creado_por, 'created_at', NEW.created_at));
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_parametros_generales_upd;
CREATE TRIGGER trg_aud_empleado_parametros_generales_upd AFTER UPDATE ON empleado_parametros_generales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_parametros_generales', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'clave', OLD.clave, 'valor', OLD.valor, 'tipo', OLD.tipo, 'fuente_legal', OLD.fuente_legal, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'creado_por', OLD.creado_por, 'created_at', OLD.created_at), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'clave', NEW.clave, 'valor', NEW.valor, 'tipo', NEW.tipo, 'fuente_legal', NEW.fuente_legal, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'creado_por', NEW.creado_por, 'created_at', NEW.created_at));
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_parametros_generales_del;
CREATE TRIGGER trg_aud_empleado_parametros_generales_del AFTER DELETE ON empleado_parametros_generales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_parametros_generales', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'clave', OLD.clave, 'valor', OLD.valor, 'tipo', OLD.tipo, 'fuente_legal', OLD.fuente_legal, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'creado_por', OLD.creado_por, 'created_at', OLD.created_at), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_planilla_detalle_concepto_ins;
CREATE TRIGGER trg_aud_planilla_detalle_concepto_ins AFTER INSERT ON planilla_detalle_concepto
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_concepto', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'planilla_detalle_empleado_id', NEW.planilla_detalle_empleado_id, 'concepto_salarial_id', NEW.concepto_salarial_id, 'concepto_salarial_formula_id', NEW.concepto_salarial_formula_id, 'empleado_concepto_formula_id', NEW.empleado_concepto_formula_id, 'cantidad', NEW.cantidad, 'monto', NEW.monto, 'concepto_ajuste_de_id', NEW.concepto_ajuste_de_id, 'motivo_ajuste', NEW.motivo_ajuste));
END;

DROP TRIGGER IF EXISTS trg_aud_planilla_detalle_concepto_upd;
CREATE TRIGGER trg_aud_planilla_detalle_concepto_upd AFTER UPDATE ON planilla_detalle_concepto
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_concepto', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'planilla_detalle_empleado_id', OLD.planilla_detalle_empleado_id, 'concepto_salarial_id', OLD.concepto_salarial_id, 'concepto_salarial_formula_id', OLD.concepto_salarial_formula_id, 'empleado_concepto_formula_id', OLD.empleado_concepto_formula_id, 'cantidad', OLD.cantidad, 'monto', OLD.monto, 'concepto_ajuste_de_id', OLD.concepto_ajuste_de_id, 'motivo_ajuste', OLD.motivo_ajuste), JSON_OBJECT('id', NEW.id, 'planilla_detalle_empleado_id', NEW.planilla_detalle_empleado_id, 'concepto_salarial_id', NEW.concepto_salarial_id, 'concepto_salarial_formula_id', NEW.concepto_salarial_formula_id, 'empleado_concepto_formula_id', NEW.empleado_concepto_formula_id, 'cantidad', NEW.cantidad, 'monto', NEW.monto, 'concepto_ajuste_de_id', NEW.concepto_ajuste_de_id, 'motivo_ajuste', NEW.motivo_ajuste));
END;

DROP TRIGGER IF EXISTS trg_aud_planilla_detalle_concepto_del;
CREATE TRIGGER trg_aud_planilla_detalle_concepto_del AFTER DELETE ON planilla_detalle_concepto
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_concepto', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'planilla_detalle_empleado_id', OLD.planilla_detalle_empleado_id, 'concepto_salarial_id', OLD.concepto_salarial_id, 'concepto_salarial_formula_id', OLD.concepto_salarial_formula_id, 'empleado_concepto_formula_id', OLD.empleado_concepto_formula_id, 'cantidad', OLD.cantidad, 'monto', OLD.monto, 'concepto_ajuste_de_id', OLD.concepto_ajuste_de_id, 'motivo_ajuste', OLD.motivo_ajuste), NULL);
END;

