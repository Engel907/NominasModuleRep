-- L14 — Tipificación de horas extra (diurna, nocturna, mixta, doble).
--
-- Hallazgo: la API de asistencia escribe registro_asistencia_detalle (un solo
-- "exceso" sin tipo) y la planilla leía las columnas tipificadas de
-- registro_asistencia, que ninguna API llena. Desde aquí la asistencia
-- tipifica el exceso al registrarse y lo guarda en
-- asistencia_hora_extra_tipificada, que es lo que suma la planilla (con el
-- tope de autorización de L13).
--
-- parametros_franja_nocturna NO se siembra (valor legal): sin franja vigente,
-- registrar un día con horas extra que no sea feriado ni descanso da error
-- explícito. Cárguese con una migración de datos (cierre + inserción, fuente_legal).
ALTER TABLE tipos_hora_extra
  ADD COLUMN clase ENUM('diurna','nocturna','mixta','doble') NULL COMMENT 'Regla de tipificación; Go no usa codigo',
  ADD UNIQUE KEY uk_the_clase (clase);
UPDATE tipos_hora_extra SET clase = LOWER(codigo) WHERE LOWER(codigo) IN ('diurna','nocturna','mixta','doble') AND clase IS NULL;

CREATE TABLE parametros_franja_nocturna (
  id                    BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  hora_inicio           TIME NOT NULL COMMENT 'Inicio de la franja nocturna',
  hora_fin              TIME NOT NULL COMMENT 'Fin de la franja nocturna (puede cruzar medianoche)',
  fuente_legal          VARCHAR(150) NOT NULL COMMENT 'Base legal o PENDIENTE VALIDAR',
  fecha_inicio_vigencia DATE NOT NULL COMMENT 'Inicio',
  fecha_fin_vigencia    DATE NULL COMMENT 'NULL = vigente',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED COMMENT 'Técnica',
  creado_por            INT NOT NULL COMMENT 'Usuario',
  created_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creación',
  CONSTRAINT chk_pfn_horas CHECK (hora_inicio <> hora_fin),
  CONSTRAINT chk_pfn_vig CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  UNIQUE KEY uk_pfn_una_vigente (es_vigente)
) ENGINE=InnoDB COMMENT='Franja horaria nocturna vigente (NO se siembra: valor legal)';

CREATE TABLE asistencia_hora_extra_tipificada (
  id                 BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  empleado_id        BIGINT UNSIGNED NOT NULL COMMENT 'Empleado',
  fecha              DATE NOT NULL COMMENT 'Día',
  tipo_hora_extra_id BIGINT UNSIGNED NOT NULL COMMENT 'Tipo resultante',
  horas              DECIMAL(5,2) NOT NULL COMMENT 'Horas de ese tipo',
  CONSTRAINT fk_ahet_emp  FOREIGN KEY (empleado_id) REFERENCES empleados(id),
  CONSTRAINT fk_ahet_tipo FOREIGN KEY (tipo_hora_extra_id) REFERENCES tipos_hora_extra(id),
  CONSTRAINT chk_ahet CHECK (horas > 0),
  UNIQUE KEY uk_ahet (empleado_id, fecha, tipo_hora_extra_id)
) ENGINE=InnoDB COMMENT='Horas extra tipificadas por día';

-- Inmutabilidad de la franja (scripts/gen_inmutabilidad_parametros.sh).
-- parametros_franja_nocturna: solo se puede cerrar la vigencia; nunca borrar
CREATE TRIGGER trg_parametros_franja_nocturna_inmut_bu BEFORE UPDATE ON parametros_franja_nocturna
FOR EACH ROW
BEGIN
  IF OLD.fecha_fin_vigencia IS NOT NULL
     AND NEW.fecha_fin_vigencia IS NULL THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: no se puede reabrir una vigencia cerrada';
  END IF;
  IF NOT (NEW.id <=> OLD.id)
     OR NOT (NEW.hora_inicio <=> OLD.hora_inicio)
     OR NOT (NEW.hora_fin <=> OLD.hora_fin)
     OR NOT (NEW.fuente_legal <=> OLD.fuente_legal)
     OR NOT (NEW.fecha_inicio_vigencia <=> OLD.fecha_inicio_vigencia)
     OR NOT (NEW.creado_por <=> OLD.creado_por)
     OR NOT (NEW.created_at <=> OLD.created_at) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_franja_nocturna solo permite cambiar fecha_fin_vigencia';
  END IF;
END;

CREATE TRIGGER trg_parametros_franja_nocturna_inmut_bd BEFORE DELETE ON parametros_franja_nocturna
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_franja_nocturna no se borra; se cierra la vigencia';
END;


-- Auditoría (scripts/gen_auditoria.sh).
DROP TRIGGER IF EXISTS trg_aud_parametros_franja_nocturna_ins;
CREATE TRIGGER trg_aud_parametros_franja_nocturna_ins AFTER INSERT ON parametros_franja_nocturna
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_franja_nocturna', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'hora_inicio', NEW.hora_inicio, 'hora_fin', NEW.hora_fin, 'fuente_legal', NEW.fuente_legal, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'creado_por', NEW.creado_por, 'created_at', NEW.created_at));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_franja_nocturna_upd;
CREATE TRIGGER trg_aud_parametros_franja_nocturna_upd AFTER UPDATE ON parametros_franja_nocturna
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_franja_nocturna', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'hora_inicio', OLD.hora_inicio, 'hora_fin', OLD.hora_fin, 'fuente_legal', OLD.fuente_legal, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'creado_por', OLD.creado_por, 'created_at', OLD.created_at), JSON_OBJECT('id', NEW.id, 'hora_inicio', NEW.hora_inicio, 'hora_fin', NEW.hora_fin, 'fuente_legal', NEW.fuente_legal, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'creado_por', NEW.creado_por, 'created_at', NEW.created_at));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_franja_nocturna_del;
CREATE TRIGGER trg_aud_parametros_franja_nocturna_del AFTER DELETE ON parametros_franja_nocturna
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_franja_nocturna', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'hora_inicio', OLD.hora_inicio, 'hora_fin', OLD.hora_fin, 'fuente_legal', OLD.fuente_legal, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'creado_por', OLD.creado_por, 'created_at', OLD.created_at), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_asistencia_hora_extra_tipificada_ins;
CREATE TRIGGER trg_aud_asistencia_hora_extra_tipificada_ins AFTER INSERT ON asistencia_hora_extra_tipificada
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('asistencia_hora_extra_tipificada', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'fecha', NEW.fecha, 'tipo_hora_extra_id', NEW.tipo_hora_extra_id, 'horas', NEW.horas));
END;

DROP TRIGGER IF EXISTS trg_aud_asistencia_hora_extra_tipificada_upd;
CREATE TRIGGER trg_aud_asistencia_hora_extra_tipificada_upd AFTER UPDATE ON asistencia_hora_extra_tipificada
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('asistencia_hora_extra_tipificada', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'fecha', OLD.fecha, 'tipo_hora_extra_id', OLD.tipo_hora_extra_id, 'horas', OLD.horas), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'fecha', NEW.fecha, 'tipo_hora_extra_id', NEW.tipo_hora_extra_id, 'horas', NEW.horas));
END;

DROP TRIGGER IF EXISTS trg_aud_asistencia_hora_extra_tipificada_del;
CREATE TRIGGER trg_aud_asistencia_hora_extra_tipificada_del AFTER DELETE ON asistencia_hora_extra_tipificada
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('asistencia_hora_extra_tipificada', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'fecha', OLD.fecha, 'tipo_hora_extra_id', OLD.tipo_hora_extra_id, 'horas', OLD.horas), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_tipos_hora_extra_ins;
CREATE TRIGGER trg_aud_tipos_hora_extra_ins AFTER INSERT ON tipos_hora_extra
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_hora_extra', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'activo', NEW.activo, 'clase', NEW.clase));
END;

DROP TRIGGER IF EXISTS trg_aud_tipos_hora_extra_upd;
CREATE TRIGGER trg_aud_tipos_hora_extra_upd AFTER UPDATE ON tipos_hora_extra
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_hora_extra', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'activo', OLD.activo, 'clase', OLD.clase), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'activo', NEW.activo, 'clase', NEW.clase));
END;

DROP TRIGGER IF EXISTS trg_aud_tipos_hora_extra_del;
CREATE TRIGGER trg_aud_tipos_hora_extra_del AFTER DELETE ON tipos_hora_extra
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_hora_extra', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'activo', OLD.activo, 'clase', OLD.clase), NULL);
END;

