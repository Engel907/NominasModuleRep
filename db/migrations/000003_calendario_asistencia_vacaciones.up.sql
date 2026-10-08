-- [split] AVISO: las FK de empleado_id apuntan a empleados(id) (no a usuarios(id) como dice el texto de abajo).
-- [split] Las tablas turnos y tipos_ausencia ya existen desde 000000_esquema_base; aquí no se redefinen.
-- ============================================================================
-- Origen: migrations/000003_calendario_asistencia_vacaciones.up.sql
-- ============================================================================
-- 000003_calendario_asistencia_vacaciones.up.sql
-- Fase 4: Calendario Laboral, Asistencia, Ausencias y Vacaciones
-- Convención heredada de 000001/000002: idempotente (IF NOT EXISTS), snake_case,
-- FKs contra usuarios(id) para empleado_id (igual que el resto del esquema v11).
--
-- NOTA DE ALCANCE (ver ADR-000 secciones 2 y 5):
--   - `empleado_turno`, `vacaciones_acumulado`, `parametros_vacaciones_escala` y
--     `registro_asistencia` (con columnas horas_*) YA EXISTEN en el v11 base y en la
--     lista de 19 tablas con patrón de vigencia. Esta migración NO las recrea.
--   - Las tablas nuevas de abajo (turnos, feriados, calendario_laboral_dia,
--     tipos_ausencia, incapacidades, registro_asistencia_detalle,
--     vacaciones_solicitudes, vacaciones_movimientos) son necesarias para Fase 4 y
--     no estaban en el material fuente entregado — nombres y columnas a confirmar
--     con el equipo antes de correr en un ambiente compartido.
--   - `registro_asistencia_detalle` es la tabla de marca/día por empleado que usa
--     esta fase; si `registro_asistencia` del v11 ya cumple ese rol, fusionar antes
--     de aplicar esta migración en producción para evitar duplicidad.

-- [split] definición redundante de `turnos` eliminada: la vigente es la de 000000_esquema_base.

CREATE TABLE IF NOT EXISTS feriados (
    id INT AUTO_INCREMENT PRIMARY KEY,
    fecha DATE NOT NULL,
    nombre VARCHAR(100) NOT NULL,
    tipo ENUM('fijo','movil') NOT NULL,
    pago_obligatorio TINYINT(1) NOT NULL COMMENT '1=obligatorio (Código de Trabajo Art.148), 0=facultativo',
    anio SMALLINT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY feriados_fecha_unique (fecha)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS calendario_laboral_dia (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    fecha DATE NOT NULL,
    anio SMALLINT NOT NULL,
    es_habil TINYINT(1) NOT NULL DEFAULT 1,
    es_feriado TINYINT(1) NOT NULL DEFAULT 0,
    feriado_id INT NULL,
    dia_semana TINYINT NOT NULL COMMENT '1=lunes...7=domingo',
    generado_en DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY calendario_fecha_unique (fecha),
    CONSTRAINT fk_calendario_feriado FOREIGN KEY (feriado_id) REFERENCES feriados(id)
) ENGINE=InnoDB;

-- [split] definición redundante de `tipos_ausencia` eliminada: la vigente es la de 000000_esquema_base.

CREATE TABLE IF NOT EXISTS incapacidades (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    empleado_id BIGINT UNSIGNED NOT NULL,
    entidad_emisora ENUM('CCSS','INS') NOT NULL,
    numero_boleta VARCHAR(50) NOT NULL,
    fecha_inicio DATE NOT NULL,
    fecha_fin DATE NOT NULL,
    dias SMALLINT NOT NULL,
    diagnostico_codigo VARCHAR(20) NULL,
    observaciones VARCHAR(255) NULL,
    creado_por BIGINT UNSIGNED NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    KEY incapacidades_empleado_idx (empleado_id, fecha_inicio),
    CONSTRAINT fk_incapacidad_empleado FOREIGN KEY (empleado_id) REFERENCES empleados(id)
) ENGINE=InnoDB;

-- Detalle diario de asistencia. `estado` tipifica presencia/ausencia/incapacidad/
-- vacaciones/feriado en un solo lugar para simplificar reportes de planilla.
CREATE TABLE IF NOT EXISTS registro_asistencia_detalle (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    empleado_id BIGINT UNSIGNED NOT NULL,
    turno_id BIGINT UNSIGNED NULL,
    fecha DATE NOT NULL,
    hora_entrada_real TIME NULL,
    hora_salida_real TIME NULL,
    horas_ordinarias DECIMAL(5,2) NOT NULL DEFAULT 0,
    horas_extra DECIMAL(5,2) NOT NULL DEFAULT 0,
    horas_tardanza DECIMAL(5,2) NOT NULL DEFAULT 0,
    minutos_tardanza SMALLINT NOT NULL DEFAULT 0,
    estado ENUM('presente','ausente_justificada','ausente_injustificada','incapacidad','vacaciones','feriado','dia_libre') NOT NULL,
    tipo_ausencia_id BIGINT UNSIGNED NULL,
    incapacidad_id BIGINT NULL,
    observaciones VARCHAR(255) NULL,
    creado_por BIGINT UNSIGNED NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY registro_asistencia_empleado_fecha_unique (empleado_id, fecha),
    CONSTRAINT fk_asistencia_empleado FOREIGN KEY (empleado_id) REFERENCES empleados(id),
    CONSTRAINT fk_asistencia_turno FOREIGN KEY (turno_id) REFERENCES turnos(id),
    CONSTRAINT fk_asistencia_tipo_ausencia FOREIGN KEY (tipo_ausencia_id) REFERENCES tipos_ausencia(id),
    CONSTRAINT fk_asistencia_incapacidad FOREIGN KEY (incapacidad_id) REFERENCES incapacidades(id)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS vacaciones_solicitudes (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    empleado_id BIGINT UNSIGNED NOT NULL,
    fecha_inicio DATE NOT NULL,
    fecha_fin DATE NOT NULL,
    dias_solicitados DECIMAL(5,2) NOT NULL,
    estado ENUM('pendiente','aprobada','rechazada','cancelada') NOT NULL DEFAULT 'pendiente',
    aprobado_por BIGINT UNSIGNED NULL,
    fecha_aprobacion DATETIME NULL,
    motivo_rechazo VARCHAR(255) NULL,
    creado_por BIGINT UNSIGNED NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    KEY vacaciones_solicitudes_empleado_idx (empleado_id, estado),
    CONSTRAINT fk_vacsol_empleado FOREIGN KEY (empleado_id) REFERENCES empleados(id)
) ENGINE=InnoDB;

-- Ledger de movimientos (append-only) que alimenta el saldo materializado en
-- vacaciones_acumulado. Nunca se actualiza/borra una fila existente aquí: los
-- ajustes se registran como nuevo movimiento, para trazabilidad de auditoría.
CREATE TABLE IF NOT EXISTS vacaciones_movimientos (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    empleado_id BIGINT UNSIGNED NOT NULL,
    tipo_movimiento ENUM('acumulacion','disfrute','ajuste') NOT NULL,
    dias DECIMAL(5,2) NOT NULL COMMENT 'positivo=acumulación/ajuste+, negativo=disfrute/ajuste-',
    fecha_efectiva DATE NOT NULL,
    periodo_referencia VARCHAR(20) NULL COMMENT 'ej: 2026, según política de acumulación vigente',
    solicitud_id BIGINT NULL,
    observaciones VARCHAR(255) NULL,
    creado_por BIGINT UNSIGNED NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    KEY vacmov_empleado_idx (empleado_id, fecha_efectiva),
    CONSTRAINT fk_vacmov_empleado FOREIGN KEY (empleado_id) REFERENCES empleados(id),
    CONSTRAINT fk_vacmov_solicitud FOREIGN KEY (solicitud_id) REFERENCES vacaciones_solicitudes(id)
) ENGINE=InnoDB;

-- vacaciones_acumulado ya existe en el v11 (tabla #17 de vigencia, única por empleado_id).
-- Se agrega la columna de saldo disponible materializado para lectura rápida en Fase 4;
-- se recalcula dentro de la misma transacción que inserta en vacaciones_movimientos.
ALTER TABLE vacaciones_acumulado
    ADD COLUMN dias_disponibles DECIMAL(5,2) NOT NULL DEFAULT 0 COMMENT 'materializado: dias_acumulados - dias_disfrutados';

-- Permisos nuevos del módulo. Excepción deliberada a la regla de "no sembrar
-- códigos de negocio" de la sección 5 del ADR-000: estos son códigos de permiso
-- del propio sistema (usados por RequirePermission), no códigos de conceptos
-- salariales del cliente, así que sí se siembran aquí.
INSERT IGNORE INTO permisos (codigo, nombre, modulo) VALUES
    ('asistencia.ver', 'Ver asistencia', 'asistencia'),
    ('asistencia.crear', 'Registrar asistencia', 'asistencia'),
    ('asistencia.editar', 'Editar asistencia', 'asistencia'),
    ('calendario.administrar', 'Administrar calendario laboral y turnos', 'asistencia'),
    ('incapacidades.gestionar', 'Gestionar incapacidades CCSS/INS', 'asistencia'),
    ('vacaciones.ver', 'Ver vacaciones', 'asistencia'),
    ('vacaciones.solicitar', 'Solicitar vacaciones', 'asistencia'),
    ('vacaciones.aprobar', 'Aprobar o rechazar solicitudes de vacaciones', 'asistencia'),
    ('vacaciones.registrar_disfrute', 'Registrar el disfrute de vacaciones aprobadas', 'asistencia');

