CREATE TABLE empleado_parametros_horas_extra (
  id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  empleado_id            BIGINT UNSIGNED NOT NULL COMMENT 'Empleado al que aplica el factor propio',
  tipo_hora_extra_id     BIGINT UNSIGNED NOT NULL COMMENT 'Tipo de hora extra cuyo factor general se reemplaza para el empleado',
  factor_recargo         DECIMAL(5,4) NOT NULL COMMENT 'Multiplicador total sobre la hora ordinaria (1.5 = 150 %)',
  fuente_legal           VARCHAR(150) NOT NULL COMMENT 'Fundamento del trato individual (contrato, convención, resolución)',
  fecha_inicio_vigencia  DATE NOT NULL COMMENT 'Desde cuándo rige para el empleado',
  fecha_fin_vigencia     DATE NULL COMMENT 'NULL = vigente; al cerrarse el empleado vuelve al factor general',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED
    COMMENT 'Técnica: una vigente por empleado y tipo',
  creado_por             BIGINT UNSIGNED NOT NULL COMMENT 'Usuario que aprobó la propuesta',
  created_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Registro',
  CONSTRAINT fk_ephe_empleado FOREIGN KEY (empleado_id) REFERENCES empleados(id),
  CONSTRAINT fk_ephe_tipo     FOREIGN KEY (tipo_hora_extra_id) REFERENCES tipos_hora_extra(id),
  CONSTRAINT fk_ephe_usuario  FOREIGN KEY (creado_por) REFERENCES usuarios(id),
  CONSTRAINT chk_ephe_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  CONSTRAINT chk_ephe_factor CHECK (factor_recargo >= 1),
  UNIQUE KEY uk_ephe_una_vigente (empleado_id, tipo_hora_extra_id, es_vigente),
  KEY ix_ephe_vigencia (empleado_id, tipo_hora_extra_id, fecha_inicio_vigencia, fecha_fin_vigencia)
) ENGINE=InnoDB COMMENT='Factor de hora extra propio de un empleado; prevalece sobre parametros_horas_extra';

CREATE TABLE empleado_parametros_jornada (
  id                       BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  empleado_id              BIGINT UNSIGNED NOT NULL COMMENT 'Empleado al que aplica el divisor propio',
  jornada_id               BIGINT UNSIGNED NOT NULL COMMENT 'Jornada cuyo divisor general se reemplaza para el empleado',
  divisor_horas_mensuales  DECIMAL(6,2) NOT NULL COMMENT 'Horas mensuales ordinarias con que se valora la hora del empleado',
  fuente_legal             VARCHAR(150) NOT NULL COMMENT 'Fundamento del trato individual',
  fecha_inicio_vigencia    DATE NOT NULL COMMENT 'Desde cuándo rige para el empleado',
  fecha_fin_vigencia       DATE NULL COMMENT 'NULL = vigente; al cerrarse el empleado vuelve al divisor general',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED
    COMMENT 'Técnica: una vigente por empleado y jornada',
  creado_por               BIGINT UNSIGNED NOT NULL COMMENT 'Usuario que aprobó la propuesta',
  created_at               DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Registro',
  CONSTRAINT fk_epj_empleado FOREIGN KEY (empleado_id) REFERENCES empleados(id),
  CONSTRAINT fk_epj_jornada  FOREIGN KEY (jornada_id) REFERENCES jornadas(id),
  CONSTRAINT fk_epj_usuario  FOREIGN KEY (creado_por) REFERENCES usuarios(id),
  CONSTRAINT chk_epj_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  CONSTRAINT chk_epj_divisor CHECK (divisor_horas_mensuales > 0),
  UNIQUE KEY uk_epj_una_vigente (empleado_id, jornada_id, es_vigente),
  KEY ix_epj_vigencia (empleado_id, jornada_id, fecha_inicio_vigencia, fecha_fin_vigencia)
) ENGINE=InnoDB COMMENT='Divisor de la hora ordinaria propio de un empleado; prevalece sobre parametros_jornada';

CREATE TRIGGER trg_empleado_parametros_horas_extra_inmut_bu BEFORE UPDATE ON empleado_parametros_horas_extra
FOR EACH ROW
BEGIN
  IF OLD.fecha_fin_vigencia IS NOT NULL
     AND NEW.fecha_fin_vigencia IS NULL THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: no se puede reabrir una vigencia cerrada';
  END IF;
  IF NOT (NEW.id <=> OLD.id)
     OR NOT (NEW.empleado_id <=> OLD.empleado_id)
     OR NOT (NEW.tipo_hora_extra_id <=> OLD.tipo_hora_extra_id)
     OR NOT (NEW.factor_recargo <=> OLD.factor_recargo)
     OR NOT (NEW.fuente_legal <=> OLD.fuente_legal)
     OR NOT (NEW.fecha_inicio_vigencia <=> OLD.fecha_inicio_vigencia)
     OR NOT (NEW.creado_por <=> OLD.creado_por)
     OR NOT (NEW.created_at <=> OLD.created_at) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: empleado_parametros_horas_extra solo permite cambiar fecha_fin_vigencia';
  END IF;
END;

CREATE TRIGGER trg_empleado_parametros_horas_extra_inmut_bd BEFORE DELETE ON empleado_parametros_horas_extra
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: empleado_parametros_horas_extra no se borra; se cierra la vigencia';
END;


CREATE TRIGGER trg_empleado_parametros_jornada_inmut_bu BEFORE UPDATE ON empleado_parametros_jornada
FOR EACH ROW
BEGIN
  IF OLD.fecha_fin_vigencia IS NOT NULL
     AND NEW.fecha_fin_vigencia IS NULL THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: no se puede reabrir una vigencia cerrada';
  END IF;
  IF NOT (NEW.id <=> OLD.id)
     OR NOT (NEW.empleado_id <=> OLD.empleado_id)
     OR NOT (NEW.jornada_id <=> OLD.jornada_id)
     OR NOT (NEW.divisor_horas_mensuales <=> OLD.divisor_horas_mensuales)
     OR NOT (NEW.fuente_legal <=> OLD.fuente_legal)
     OR NOT (NEW.fecha_inicio_vigencia <=> OLD.fecha_inicio_vigencia)
     OR NOT (NEW.creado_por <=> OLD.creado_por)
     OR NOT (NEW.created_at <=> OLD.created_at) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: empleado_parametros_jornada solo permite cambiar fecha_fin_vigencia';
  END IF;
END;

CREATE TRIGGER trg_empleado_parametros_jornada_inmut_bd BEFORE DELETE ON empleado_parametros_jornada
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: empleado_parametros_jornada no se borra; se cierra la vigencia';
END;


DROP TRIGGER IF EXISTS trg_aud_empleado_parametros_horas_extra_ins;
CREATE TRIGGER trg_aud_empleado_parametros_horas_extra_ins AFTER INSERT ON empleado_parametros_horas_extra
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_parametros_horas_extra', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'tipo_hora_extra_id', NEW.tipo_hora_extra_id, 'factor_recargo', NEW.factor_recargo, 'fuente_legal', NEW.fuente_legal, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'creado_por', NEW.creado_por));
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_parametros_horas_extra_upd;
CREATE TRIGGER trg_aud_empleado_parametros_horas_extra_upd AFTER UPDATE ON empleado_parametros_horas_extra
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_parametros_horas_extra', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'tipo_hora_extra_id', OLD.tipo_hora_extra_id, 'factor_recargo', OLD.factor_recargo, 'fuente_legal', OLD.fuente_legal, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'creado_por', OLD.creado_por), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'tipo_hora_extra_id', NEW.tipo_hora_extra_id, 'factor_recargo', NEW.factor_recargo, 'fuente_legal', NEW.fuente_legal, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'creado_por', NEW.creado_por));
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_parametros_horas_extra_del;
CREATE TRIGGER trg_aud_empleado_parametros_horas_extra_del AFTER DELETE ON empleado_parametros_horas_extra
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_parametros_horas_extra', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'tipo_hora_extra_id', OLD.tipo_hora_extra_id, 'factor_recargo', OLD.factor_recargo, 'fuente_legal', OLD.fuente_legal, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'creado_por', OLD.creado_por), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_parametros_jornada_ins;
CREATE TRIGGER trg_aud_empleado_parametros_jornada_ins AFTER INSERT ON empleado_parametros_jornada
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_parametros_jornada', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'jornada_id', NEW.jornada_id, 'divisor_horas_mensuales', NEW.divisor_horas_mensuales, 'fuente_legal', NEW.fuente_legal, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'creado_por', NEW.creado_por));
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_parametros_jornada_upd;
CREATE TRIGGER trg_aud_empleado_parametros_jornada_upd AFTER UPDATE ON empleado_parametros_jornada
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_parametros_jornada', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'jornada_id', OLD.jornada_id, 'divisor_horas_mensuales', OLD.divisor_horas_mensuales, 'fuente_legal', OLD.fuente_legal, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'creado_por', OLD.creado_por), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'jornada_id', NEW.jornada_id, 'divisor_horas_mensuales', NEW.divisor_horas_mensuales, 'fuente_legal', NEW.fuente_legal, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'creado_por', NEW.creado_por));
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_parametros_jornada_del;
CREATE TRIGGER trg_aud_empleado_parametros_jornada_del AFTER DELETE ON empleado_parametros_jornada
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_parametros_jornada', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'jornada_id', OLD.jornada_id, 'divisor_horas_mensuales', OLD.divisor_horas_mensuales, 'fuente_legal', OLD.fuente_legal, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'creado_por', OLD.creado_por), NULL);
END;

