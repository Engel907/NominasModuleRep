-- L13 — Autorización previa de horas extra.
--
-- Una hora extra solo llega a planilla si el día tiene una autorización en
-- estado 'autorizada': se paga min(horas extra registradas, horas_autorizadas).
-- Quien solicita no puede resolver (CHECK en BD + 403 en Go).
CREATE TABLE horas_extra_autorizadas (
  id                 BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  empleado_id        BIGINT UNSIGNED NOT NULL COMMENT 'Empleado',
  fecha              DATE NOT NULL COMMENT 'Día de la hora extra',
  horas_autorizadas  DECIMAL(5,2) NOT NULL COMMENT 'Tope autorizado',
  tipo_hora_extra_id BIGINT UNSIGNED NULL COMMENT 'Tipo, si se fija de antemano',
  motivo             VARCHAR(255) NOT NULL COMMENT 'Justificación',
  estado             ENUM('pendiente','autorizada','rechazada') NOT NULL DEFAULT 'pendiente' COMMENT 'Estado',
  solicitado_por     BIGINT UNSIGNED NOT NULL COMMENT 'Quien solicita',
  fecha_solicitud    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Solicitud',
  resuelto_por       BIGINT UNSIGNED NULL COMMENT 'Quien autoriza/rechaza',
  fecha_resolucion   DATETIME NULL COMMENT 'Resolución',
  motivo_resolucion  VARCHAR(255) NULL COMMENT 'Motivo del rechazo',
  CONSTRAINT fk_hea_emp   FOREIGN KEY (empleado_id)        REFERENCES empleados(id),
  CONSTRAINT fk_hea_tipo  FOREIGN KEY (tipo_hora_extra_id) REFERENCES tipos_hora_extra(id),
  CONSTRAINT fk_hea_sol   FOREIGN KEY (solicitado_por)     REFERENCES usuarios(id),
  CONSTRAINT fk_hea_res   FOREIGN KEY (resuelto_por)       REFERENCES usuarios(id),
  CONSTRAINT chk_hea_horas CHECK (horas_autorizadas > 0 AND horas_autorizadas <= 24),
  CONSTRAINT chk_hea_segregacion CHECK (resuelto_por IS NULL OR resuelto_por <> solicitado_por),
  CONSTRAINT chk_hea_estado CHECK ((estado = 'pendiente' AND resuelto_por IS NULL) OR (estado <> 'pendiente' AND resuelto_por IS NOT NULL)),
  UNIQUE KEY uk_hea (empleado_id, fecha),
  KEY ix_hea_estado (estado, fecha)
) ENGINE=InnoDB COMMENT='Autorización previa de horas extra; quien solicita no puede resolver (CHECK)';

INSERT INTO permisos (codigo, nombre, modulo)
SELECT 'horas_extra.autorizar', 'Autorizar o rechazar horas extra', 'asistencia'
WHERE NOT EXISTS (SELECT 1 FROM permisos WHERE codigo = 'horas_extra.autorizar');

INSERT INTO rol_permisos (rol_id, permiso_id)
SELECT r.id, p.id
FROM roles r
JOIN permisos p ON p.codigo = 'horas_extra.autorizar'
WHERE r.codigo = 'RRHH'
  AND NOT EXISTS (SELECT 1 FROM rol_permisos rp WHERE rp.rol_id = r.id AND rp.permiso_id = p.id);

-- Los tokens ya emitidos a usuarios RRHH no traen el permiso nuevo (F15).
UPDATE usuarios u
SET u.permisos_version = u.permisos_version + 1
WHERE EXISTS (SELECT 1 FROM usuario_roles ur JOIN roles r ON r.id = ur.rol_id
              WHERE ur.usuario_id = u.id AND r.codigo = 'RRHH' AND ur.fecha_fin_vigencia IS NULL);

-- Auditoría (scripts/gen_auditoria.sh).
DROP TRIGGER IF EXISTS trg_aud_horas_extra_autorizadas_ins;
CREATE TRIGGER trg_aud_horas_extra_autorizadas_ins AFTER INSERT ON horas_extra_autorizadas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('horas_extra_autorizadas', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'fecha', NEW.fecha, 'horas_autorizadas', NEW.horas_autorizadas, 'tipo_hora_extra_id', NEW.tipo_hora_extra_id, 'motivo', NEW.motivo, 'estado', NEW.estado, 'solicitado_por', NEW.solicitado_por, 'fecha_solicitud', NEW.fecha_solicitud, 'resuelto_por', NEW.resuelto_por, 'fecha_resolucion', NEW.fecha_resolucion, 'motivo_resolucion', NEW.motivo_resolucion));
END;

DROP TRIGGER IF EXISTS trg_aud_horas_extra_autorizadas_upd;
CREATE TRIGGER trg_aud_horas_extra_autorizadas_upd AFTER UPDATE ON horas_extra_autorizadas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('horas_extra_autorizadas', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'fecha', OLD.fecha, 'horas_autorizadas', OLD.horas_autorizadas, 'tipo_hora_extra_id', OLD.tipo_hora_extra_id, 'motivo', OLD.motivo, 'estado', OLD.estado, 'solicitado_por', OLD.solicitado_por, 'fecha_solicitud', OLD.fecha_solicitud, 'resuelto_por', OLD.resuelto_por, 'fecha_resolucion', OLD.fecha_resolucion, 'motivo_resolucion', OLD.motivo_resolucion), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'fecha', NEW.fecha, 'horas_autorizadas', NEW.horas_autorizadas, 'tipo_hora_extra_id', NEW.tipo_hora_extra_id, 'motivo', NEW.motivo, 'estado', NEW.estado, 'solicitado_por', NEW.solicitado_por, 'fecha_solicitud', NEW.fecha_solicitud, 'resuelto_por', NEW.resuelto_por, 'fecha_resolucion', NEW.fecha_resolucion, 'motivo_resolucion', NEW.motivo_resolucion));
END;

DROP TRIGGER IF EXISTS trg_aud_horas_extra_autorizadas_del;
CREATE TRIGGER trg_aud_horas_extra_autorizadas_del AFTER DELETE ON horas_extra_autorizadas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('horas_extra_autorizadas', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'fecha', OLD.fecha, 'horas_autorizadas', OLD.horas_autorizadas, 'tipo_hora_extra_id', OLD.tipo_hora_extra_id, 'motivo', OLD.motivo, 'estado', OLD.estado, 'solicitado_por', OLD.solicitado_por, 'fecha_solicitud', OLD.fecha_solicitud, 'resuelto_por', OLD.resuelto_por, 'fecha_resolucion', OLD.fecha_resolucion, 'motivo_resolucion', OLD.motivo_resolucion), NULL);
END;

