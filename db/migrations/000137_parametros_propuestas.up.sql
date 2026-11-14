CREATE TABLE parametros_propuestas (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  tabla VARCHAR(60) NOT NULL COMMENT 'Tabla paramétrica destino',
  clave_natural VARCHAR(120) NOT NULL COMMENT 'Serie afectada (p. ej. clave=DIVISOR_DIAS_MES, jornada_id=2, * para escala única)',
  payload JSON NOT NULL COMMENT 'Valores propuestos, normalizados: {"filas":[{...}]}',
  fecha_inicio_vigencia DATE NOT NULL COMMENT 'Desde cuándo rige',
  fuente_legal VARCHAR(150) NOT NULL COMMENT 'Base legal (obligatoria)',
  estado ENUM('pendiente','aprobada','rechazada','aplicada') NOT NULL DEFAULT 'pendiente' COMMENT 'Estado',
  propuesto_por BIGINT UNSIGNED NOT NULL COMMENT 'Quien propone',
  fecha_propuesta DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Propuesta',
  resuelto_por BIGINT UNSIGNED NULL COMMENT 'Quien aprueba/rechaza',
  fecha_resolucion DATETIME NULL COMMENT 'Resolución',
  motivo_rechazo VARCHAR(255) NULL COMMENT 'Obligatorio al rechazar',
  CONSTRAINT fk_ppr_prop FOREIGN KEY (propuesto_por) REFERENCES usuarios(id),
  CONSTRAINT fk_ppr_res  FOREIGN KEY (resuelto_por)  REFERENCES usuarios(id),
  CONSTRAINT chk_ppr_cuatro_ojos CHECK (resuelto_por IS NULL OR resuelto_por <> propuesto_por),
  CONSTRAINT chk_ppr_estado CHECK ((estado = 'pendiente' AND resuelto_por IS NULL) OR (estado <> 'pendiente' AND resuelto_por IS NOT NULL)),
  CONSTRAINT chk_ppr_rechazo CHECK (estado <> 'rechazada' OR motivo_rechazo IS NOT NULL),
  KEY ix_ppr_estado (estado, tabla)
) ENGINE=InnoDB COMMENT='Cambios de parámetros legales con doble control';

INSERT INTO rol_permisos (rol_id, permiso_id)
SELECT r.id, p.id
FROM roles r
JOIN permisos p ON p.codigo = 'parametros.ver'
WHERE r.codigo IN ('PLANILLAS_PREPARA', 'PLANILLAS_APRUEBA', 'AUDITOR')
  AND NOT EXISTS (SELECT 1 FROM rol_permisos rp WHERE rp.rol_id = r.id AND rp.permiso_id = p.id);

UPDATE usuarios u
SET u.permisos_version = u.permisos_version + 1
WHERE EXISTS (SELECT 1 FROM usuario_roles ur JOIN roles r ON r.id = ur.rol_id
              WHERE ur.usuario_id = u.id AND r.codigo IN ('PLANILLAS_PREPARA', 'PLANILLAS_APRUEBA', 'AUDITOR')
                AND ur.fecha_fin_vigencia IS NULL);

DROP TRIGGER IF EXISTS trg_aud_parametros_propuestas_ins;
CREATE TRIGGER trg_aud_parametros_propuestas_ins AFTER INSERT ON parametros_propuestas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_propuestas', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'tabla', NEW.tabla, 'clave_natural', NEW.clave_natural, 'payload', NEW.payload, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fuente_legal', NEW.fuente_legal, 'estado', NEW.estado, 'propuesto_por', NEW.propuesto_por, 'fecha_propuesta', NEW.fecha_propuesta, 'resuelto_por', NEW.resuelto_por, 'fecha_resolucion', NEW.fecha_resolucion, 'motivo_rechazo', NEW.motivo_rechazo));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_propuestas_upd;
CREATE TRIGGER trg_aud_parametros_propuestas_upd AFTER UPDATE ON parametros_propuestas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_propuestas', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'tabla', OLD.tabla, 'clave_natural', OLD.clave_natural, 'payload', OLD.payload, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fuente_legal', OLD.fuente_legal, 'estado', OLD.estado, 'propuesto_por', OLD.propuesto_por, 'fecha_propuesta', OLD.fecha_propuesta, 'resuelto_por', OLD.resuelto_por, 'fecha_resolucion', OLD.fecha_resolucion, 'motivo_rechazo', OLD.motivo_rechazo), JSON_OBJECT('id', NEW.id, 'tabla', NEW.tabla, 'clave_natural', NEW.clave_natural, 'payload', NEW.payload, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fuente_legal', NEW.fuente_legal, 'estado', NEW.estado, 'propuesto_por', NEW.propuesto_por, 'fecha_propuesta', NEW.fecha_propuesta, 'resuelto_por', NEW.resuelto_por, 'fecha_resolucion', NEW.fecha_resolucion, 'motivo_rechazo', NEW.motivo_rechazo));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_propuestas_del;
CREATE TRIGGER trg_aud_parametros_propuestas_del AFTER DELETE ON parametros_propuestas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_propuestas', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'tabla', OLD.tabla, 'clave_natural', OLD.clave_natural, 'payload', OLD.payload, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fuente_legal', OLD.fuente_legal, 'estado', OLD.estado, 'propuesto_por', OLD.propuesto_por, 'fecha_propuesta', OLD.fecha_propuesta, 'resuelto_por', OLD.resuelto_por, 'fecha_resolucion', OLD.fecha_resolucion, 'motivo_rechazo', OLD.motivo_rechazo), NULL);
END;

