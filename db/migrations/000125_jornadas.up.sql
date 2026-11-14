CREATE TABLE jornadas (
  id     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  codigo VARCHAR(20)  NOT NULL UNIQUE COMMENT 'DIURNA, MIXTA, NOCTURNA',
  nombre VARCHAR(100) NOT NULL COMMENT 'Nombre de la jornada',
  activo TINYINT(1) NOT NULL DEFAULT 1 COMMENT '1 = utilizable'
) ENGINE=InnoDB COMMENT='Catálogo de jornadas';

INSERT INTO jornadas (codigo, nombre)
SELECT v.c, v.n FROM (SELECT 'DIURNA' c, 'Jornada diurna' n UNION ALL SELECT 'MIXTA', 'Jornada mixta' UNION ALL SELECT 'NOCTURNA', 'Jornada nocturna') v
WHERE NOT EXISTS (SELECT 1 FROM jornadas j WHERE j.codigo = v.c);

CREATE TABLE parametros_jornada (
  id                       BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  jornada_id               BIGINT UNSIGNED NOT NULL COMMENT 'Jornada',
  horas_diarias_ordinarias DECIMAL(4,2) NULL COMMENT 'Horas ordinarias por día (NULL = no definido)',
  divisor_horas_mensuales  DECIMAL(6,2) NOT NULL COMMENT 'Divisor del valor hora mensual',
  fuente_legal             VARCHAR(150) NOT NULL COMMENT 'Base legal del valor',
  fecha_inicio_vigencia    DATE NOT NULL COMMENT 'Inicio de vigencia',
  fecha_fin_vigencia       DATE NULL COMMENT 'NULL = vigente',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED COMMENT 'Técnica: una vigente por jornada',
  creado_por               INT NOT NULL COMMENT 'Usuario',
  created_at               DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creación',
  CONSTRAINT fk_pj_jornada FOREIGN KEY (jornada_id) REFERENCES jornadas(id),
  CONSTRAINT chk_pj CHECK ((horas_diarias_ordinarias IS NULL OR (horas_diarias_ordinarias > 0 AND horas_diarias_ordinarias <= 24)) AND divisor_horas_mensuales > 0),
  CONSTRAINT chk_pj_vig CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  UNIQUE KEY uk_pj_una_vigente (jornada_id, es_vigente),
  KEY ix_pj_vigencia (jornada_id, fecha_inicio_vigencia, fecha_fin_vigencia)
) ENGINE=InnoDB COMMENT='Divisor de la hora ordinaria por jornada, versionado por vigencia';

INSERT INTO parametros_jornada (jornada_id, horas_diarias_ordinarias, divisor_horas_mensuales, fuente_legal, fecha_inicio_vigencia, creado_por)
SELECT j.id, NULL, CAST(g.valor AS DECIMAL(6,2)), 'Código de Trabajo, art. 136', g.fecha_inicio_vigencia, 1
FROM jornadas j
JOIN parametros_generales g ON g.clave = 'HORAS_MENSUALES_ORDINARIAS' AND g.fecha_fin_vigencia IS NULL
WHERE j.codigo = 'DIURNA'
  AND NOT EXISTS (SELECT 1 FROM parametros_jornada p WHERE p.jornada_id = j.id AND p.fecha_fin_vigencia IS NULL);

ALTER TABLE contratos
  ADD COLUMN jornada_id BIGINT UNSIGNED NULL COMMENT 'FK a jornadas',
  ADD CONSTRAINT fk_contratos_jornada FOREIGN KEY (jornada_id) REFERENCES jornadas(id);
UPDATE contratos c JOIN jornadas j ON UPPER(j.codigo) = UPPER(c.jornada) SET c.jornada_id = j.id WHERE c.jornada_id IS NULL;

ALTER TABLE turnos
  ADD COLUMN jornada_id BIGINT UNSIGNED NULL COMMENT 'Jornada a la que pertenece el turno',
  ADD CONSTRAINT fk_turnos_jornada FOREIGN KEY (jornada_id) REFERENCES jornadas(id);

CREATE TRIGGER trg_contratos_catalogos_bi BEFORE INSERT ON contratos
FOR EACH ROW
BEGIN
  IF NEW.jornada_id IS NULL THEN
    SET NEW.jornada_id = (SELECT j.id FROM jornadas j WHERE UPPER(j.codigo) = UPPER(NEW.jornada));
  END IF;
  IF NEW.tipo_contrato_id IS NULL THEN
    SET NEW.tipo_contrato_id = (SELECT t.id FROM tipos_contrato t
                                 WHERE UPPER(t.codigo) = UPPER(NEW.tipo_contrato) OR UPPER(t.nombre) = UPPER(NEW.tipo_contrato)
                                 ORDER BY t.id LIMIT 1);
  END IF;
END;

CREATE TRIGGER trg_parametros_jornada_inmut_bu BEFORE UPDATE ON parametros_jornada
FOR EACH ROW
BEGIN
  IF OLD.fecha_fin_vigencia IS NOT NULL
     AND NEW.fecha_fin_vigencia IS NULL THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: no se puede reabrir una vigencia cerrada';
  END IF;
  IF NOT (NEW.id <=> OLD.id)
     OR NOT (NEW.jornada_id <=> OLD.jornada_id)
     OR NOT (NEW.horas_diarias_ordinarias <=> OLD.horas_diarias_ordinarias)
     OR NOT (NEW.divisor_horas_mensuales <=> OLD.divisor_horas_mensuales)
     OR NOT (NEW.fuente_legal <=> OLD.fuente_legal)
     OR NOT (NEW.fecha_inicio_vigencia <=> OLD.fecha_inicio_vigencia)
     OR NOT (NEW.creado_por <=> OLD.creado_por)
     OR NOT (NEW.created_at <=> OLD.created_at) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_jornada solo permite cambiar fecha_fin_vigencia';
  END IF;
END;

CREATE TRIGGER trg_parametros_jornada_inmut_bd BEFORE DELETE ON parametros_jornada
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_jornada no se borra; se cierra la vigencia';
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_jornada_ins;
CREATE TRIGGER trg_aud_parametros_jornada_ins AFTER INSERT ON parametros_jornada
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_jornada', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'jornada_id', NEW.jornada_id, 'horas_diarias_ordinarias', NEW.horas_diarias_ordinarias, 'divisor_horas_mensuales', NEW.divisor_horas_mensuales, 'fuente_legal', NEW.fuente_legal, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'creado_por', NEW.creado_por, 'created_at', NEW.created_at));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_jornada_upd;
CREATE TRIGGER trg_aud_parametros_jornada_upd AFTER UPDATE ON parametros_jornada
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_jornada', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'jornada_id', OLD.jornada_id, 'horas_diarias_ordinarias', OLD.horas_diarias_ordinarias, 'divisor_horas_mensuales', OLD.divisor_horas_mensuales, 'fuente_legal', OLD.fuente_legal, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'creado_por', OLD.creado_por, 'created_at', OLD.created_at), JSON_OBJECT('id', NEW.id, 'jornada_id', NEW.jornada_id, 'horas_diarias_ordinarias', NEW.horas_diarias_ordinarias, 'divisor_horas_mensuales', NEW.divisor_horas_mensuales, 'fuente_legal', NEW.fuente_legal, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'creado_por', NEW.creado_por, 'created_at', NEW.created_at));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_jornada_del;
CREATE TRIGGER trg_aud_parametros_jornada_del AFTER DELETE ON parametros_jornada
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_jornada', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'jornada_id', OLD.jornada_id, 'horas_diarias_ordinarias', OLD.horas_diarias_ordinarias, 'divisor_horas_mensuales', OLD.divisor_horas_mensuales, 'fuente_legal', OLD.fuente_legal, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'creado_por', OLD.creado_por, 'created_at', OLD.created_at), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_contratos_ins;
CREATE TRIGGER trg_aud_contratos_ins AFTER INSERT ON contratos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('contratos', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'puesto_id', NEW.puesto_id, 'regimen_laboral_id', NEW.regimen_laboral_id, 'tipo_contrato', NEW.tipo_contrato, 'tipo_contrato_id', NEW.tipo_contrato_id, 'salario_pactado', NEW.salario_pactado, 'jornada', NEW.jornada, 'jornada_id', NEW.jornada_id, 'fecha_inicio_contrato', NEW.fecha_inicio_contrato, 'fecha_fin_contrato', NEW.fecha_fin_contrato, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'creado_por', NEW.creado_por));
END;

DROP TRIGGER IF EXISTS trg_aud_contratos_upd;
CREATE TRIGGER trg_aud_contratos_upd AFTER UPDATE ON contratos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('contratos', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'puesto_id', OLD.puesto_id, 'regimen_laboral_id', OLD.regimen_laboral_id, 'tipo_contrato', OLD.tipo_contrato, 'tipo_contrato_id', OLD.tipo_contrato_id, 'salario_pactado', OLD.salario_pactado, 'jornada', OLD.jornada, 'jornada_id', OLD.jornada_id, 'fecha_inicio_contrato', OLD.fecha_inicio_contrato, 'fecha_fin_contrato', OLD.fecha_fin_contrato, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'creado_por', OLD.creado_por), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'puesto_id', NEW.puesto_id, 'regimen_laboral_id', NEW.regimen_laboral_id, 'tipo_contrato', NEW.tipo_contrato, 'tipo_contrato_id', NEW.tipo_contrato_id, 'salario_pactado', NEW.salario_pactado, 'jornada', NEW.jornada, 'jornada_id', NEW.jornada_id, 'fecha_inicio_contrato', NEW.fecha_inicio_contrato, 'fecha_fin_contrato', NEW.fecha_fin_contrato, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'creado_por', NEW.creado_por));
END;

DROP TRIGGER IF EXISTS trg_aud_contratos_del;
CREATE TRIGGER trg_aud_contratos_del AFTER DELETE ON contratos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('contratos', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'puesto_id', OLD.puesto_id, 'regimen_laboral_id', OLD.regimen_laboral_id, 'tipo_contrato', OLD.tipo_contrato, 'tipo_contrato_id', OLD.tipo_contrato_id, 'salario_pactado', OLD.salario_pactado, 'jornada', OLD.jornada, 'jornada_id', OLD.jornada_id, 'fecha_inicio_contrato', OLD.fecha_inicio_contrato, 'fecha_fin_contrato', OLD.fecha_fin_contrato, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'creado_por', OLD.creado_por), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_turnos_ins;
CREATE TRIGGER trg_aud_turnos_ins AFTER INSERT ON turnos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('turnos', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'hora_inicio', NEW.hora_inicio, 'hora_fin', NEW.hora_fin, 'jornada_id', NEW.jornada_id));
END;

DROP TRIGGER IF EXISTS trg_aud_turnos_upd;
CREATE TRIGGER trg_aud_turnos_upd AFTER UPDATE ON turnos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('turnos', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'hora_inicio', OLD.hora_inicio, 'hora_fin', OLD.hora_fin, 'jornada_id', OLD.jornada_id), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'hora_inicio', NEW.hora_inicio, 'hora_fin', NEW.hora_fin, 'jornada_id', NEW.jornada_id));
END;

DROP TRIGGER IF EXISTS trg_aud_turnos_del;
CREATE TRIGGER trg_aud_turnos_del AFTER DELETE ON turnos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('turnos', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'hora_inicio', OLD.hora_inicio, 'hora_fin', OLD.hora_fin, 'jornada_id', OLD.jornada_id), NULL);
END;

