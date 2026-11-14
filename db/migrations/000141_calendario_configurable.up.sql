CREATE TABLE feriados_catalogo (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  nombre VARCHAR(100) NOT NULL COMMENT 'Nombre del feriado',
  tipo ENUM('fijo','movil') NOT NULL COMMENT 'fijo = mes/día; movil = días respecto al Domingo de Pascua',
  mes TINYINT UNSIGNED NULL COMMENT 'Mes (solo fijo)',
  dia TINYINT UNSIGNED NULL COMMENT 'Día del mes (solo fijo)',
  desplazamiento_pascua SMALLINT NULL COMMENT 'Días respecto al Domingo de Pascua (solo movil)',
  pago_obligatorio TINYINT(1) NOT NULL COMMENT '1 = pago obligatorio (CT art. 148-149); 0 = pago no obligatorio',
  fuente_legal VARCHAR(255) NOT NULL COMMENT 'Norma que lo establece',
  anio_desde SMALLINT UNSIGNED NOT NULL COMMENT 'Primer año en que aplica',
  anio_hasta SMALLINT UNSIGNED NULL COMMENT 'Último año en que aplica (NULL = sin fin)',
  activo TINYINT(1) NOT NULL DEFAULT 1 COMMENT 'Se considera al generar el calendario',
  creado_por BIGINT UNSIGNED NULL COMMENT 'Usuario que lo registró',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Registro',
  CONSTRAINT fk_feriados_catalogo_usuario FOREIGN KEY (creado_por) REFERENCES usuarios(id),
  CONSTRAINT chk_feriados_catalogo_forma CHECK (
    (tipo = 'fijo' AND mes BETWEEN 1 AND 12 AND dia BETWEEN 1 AND 31 AND desplazamiento_pascua IS NULL)
    OR (tipo = 'movil' AND desplazamiento_pascua BETWEEN -70 AND 70 AND mes IS NULL AND dia IS NULL)),
  CONSTRAINT chk_feriados_catalogo_anios CHECK (anio_hasta IS NULL OR anio_hasta >= anio_desde)
) ENGINE=InnoDB COMMENT='Catálogo configurable de feriados de ley';

INSERT INTO feriados_catalogo (nombre, tipo, mes, dia, desplazamiento_pascua, pago_obligatorio, fuente_legal, anio_desde, anio_hasta) VALUES
  ('Año Nuevo', 'fijo', 1, 1, NULL, 1, 'Código de Trabajo, art. 148', 2000, NULL),
  ('Jueves Santo', 'movil', NULL, NULL, -3, 1, 'Código de Trabajo, art. 148', 2000, NULL),
  ('Viernes Santo', 'movil', NULL, NULL, -2, 1, 'Código de Trabajo, art. 148', 2000, NULL),
  ('Día de Juan Santamaría', 'fijo', 4, 11, NULL, 1, 'Código de Trabajo, art. 148', 2000, NULL),
  ('Día Internacional del Trabajo', 'fijo', 5, 1, NULL, 1, 'Código de Trabajo, art. 148', 2000, NULL),
  ('Anexión del Partido de Nicoya', 'fijo', 7, 25, NULL, 1, 'Código de Trabajo, art. 148', 2000, NULL),
  ('Virgen de los Ángeles', 'fijo', 8, 2, NULL, 0, 'Código de Trabajo, art. 148', 2000, NULL),
  ('Día de la Madre', 'fijo', 8, 15, NULL, 1, 'Código de Trabajo, art. 148', 2000, NULL),
  ('Día de la Persona Negra y la Cultura Afrocostarricense', 'fijo', 8, 31, NULL, 0, 'Código de Trabajo, art. 148', 2022, NULL),
  ('Independencia de Costa Rica', 'fijo', 9, 15, NULL, 1, 'Código de Trabajo, art. 148', 2000, NULL),
  ('Día de las Culturas', 'fijo', 10, 12, NULL, 0, 'Código de Trabajo, art. 148', 2000, 2019),
  ('Abolición del Ejército', 'fijo', 12, 1, NULL, 0, 'Código de Trabajo, art. 148', 2020, NULL),
  ('Navidad', 'fijo', 12, 25, NULL, 1, 'Código de Trabajo, art. 148', 2000, NULL);

CREATE TRIGGER trg_aud_feriados_catalogo_ins AFTER INSERT ON feriados_catalogo
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('feriados_catalogo', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'nombre', NEW.nombre, 'tipo', NEW.tipo, 'mes', NEW.mes, 'dia', NEW.dia, 'desplazamiento_pascua', NEW.desplazamiento_pascua, 'pago_obligatorio', NEW.pago_obligatorio, 'fuente_legal', NEW.fuente_legal, 'anio_desde', NEW.anio_desde, 'anio_hasta', NEW.anio_hasta, 'activo', NEW.activo, 'creado_por', NEW.creado_por, 'created_at', NEW.created_at));
END;

CREATE TRIGGER trg_aud_feriados_catalogo_upd AFTER UPDATE ON feriados_catalogo
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('feriados_catalogo', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'nombre', OLD.nombre, 'tipo', OLD.tipo, 'mes', OLD.mes, 'dia', OLD.dia, 'desplazamiento_pascua', OLD.desplazamiento_pascua, 'pago_obligatorio', OLD.pago_obligatorio, 'fuente_legal', OLD.fuente_legal, 'anio_desde', OLD.anio_desde, 'anio_hasta', OLD.anio_hasta, 'activo', OLD.activo, 'creado_por', OLD.creado_por, 'created_at', OLD.created_at), JSON_OBJECT('id', NEW.id, 'nombre', NEW.nombre, 'tipo', NEW.tipo, 'mes', NEW.mes, 'dia', NEW.dia, 'desplazamiento_pascua', NEW.desplazamiento_pascua, 'pago_obligatorio', NEW.pago_obligatorio, 'fuente_legal', NEW.fuente_legal, 'anio_desde', NEW.anio_desde, 'anio_hasta', NEW.anio_hasta, 'activo', NEW.activo, 'creado_por', NEW.creado_por, 'created_at', NEW.created_at));
END;

CREATE TRIGGER trg_aud_feriados_catalogo_del AFTER DELETE ON feriados_catalogo
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('feriados_catalogo', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'nombre', OLD.nombre, 'tipo', OLD.tipo, 'mes', OLD.mes, 'dia', OLD.dia, 'desplazamiento_pascua', OLD.desplazamiento_pascua, 'pago_obligatorio', OLD.pago_obligatorio, 'fuente_legal', OLD.fuente_legal, 'anio_desde', OLD.anio_desde, 'anio_hasta', OLD.anio_hasta, 'activo', OLD.activo, 'creado_por', OLD.creado_por, 'created_at', OLD.created_at), NULL);
END;

ALTER TABLE turnos
  ADD COLUMN tolerancia_tardanza_minutos SMALLINT UNSIGNED NOT NULL DEFAULT 0
    COMMENT 'Minutos de gracia en la entrada antes de contar tardanza' AFTER hora_fin,
  ADD CONSTRAINT chk_turnos_tolerancia CHECK (tolerancia_tardanza_minutos <= 240);

DROP TRIGGER trg_aud_turnos_ins;
CREATE TRIGGER trg_aud_turnos_ins AFTER INSERT ON turnos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('turnos', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'hora_inicio', NEW.hora_inicio, 'hora_fin', NEW.hora_fin, 'tolerancia_tardanza_minutos', NEW.tolerancia_tardanza_minutos, 'jornada_id', NEW.jornada_id));
END;

DROP TRIGGER trg_aud_turnos_upd;
CREATE TRIGGER trg_aud_turnos_upd AFTER UPDATE ON turnos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('turnos', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'hora_inicio', OLD.hora_inicio, 'hora_fin', OLD.hora_fin, 'tolerancia_tardanza_minutos', OLD.tolerancia_tardanza_minutos, 'jornada_id', OLD.jornada_id), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'hora_inicio', NEW.hora_inicio, 'hora_fin', NEW.hora_fin, 'tolerancia_tardanza_minutos', NEW.tolerancia_tardanza_minutos, 'jornada_id', NEW.jornada_id));
END;

DROP TRIGGER trg_aud_turnos_del;
CREATE TRIGGER trg_aud_turnos_del AFTER DELETE ON turnos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('turnos', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'hora_inicio', OLD.hora_inicio, 'hora_fin', OLD.hora_fin, 'tolerancia_tardanza_minutos', OLD.tolerancia_tardanza_minutos, 'jornada_id', OLD.jornada_id), NULL);
END;

