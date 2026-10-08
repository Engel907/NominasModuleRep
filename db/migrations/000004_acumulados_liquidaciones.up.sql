-- [split] AVISO: la asunción sobre la FK de empleado_id ya está resuelta: apunta a empleados(id) (BIGINT UNSIGNED).
-- ============================================================================
-- Origen: migrations/000004_acumulados_liquidaciones.up.sql
-- ============================================================================
-- 000004_acumulados_liquidaciones.up.sql
-- Fase 7: Acumulados (Aguinaldo, Salario Escolar) y Liquidaciones Laborales
--
-- NOTA DE ALCANCE (sigue la convención de ADR-000 de declarar supuestos
-- explícitamente en vez de asumirlos en silencio):
--
--   Ninguna de las 4 tablas de abajo existe en el v11 base ni en las
--   migraciones 000001-000003 provistas. Son necesarias para persistir el
--   resultado auditable de Fase 7 (mismo criterio que planilla_detalle_empleado
--   en Fase 6: el motor de cálculo es puro / sin I/O, pero su resultado se
--   persiste para auditoría y para que "procesar" sea idempotente/consultable).
--
--   ASUNCIÓN A CONFIRMAR — FK de empleado_id: el material fuente es
--   inconsistente entre fases sobre contra qué tabla se hace FK de
--   "empleado": Fase 3 (db/queries/empleados_puestos.sql) define `empleados`
--   como entidad ancla propia (CreateEmpleado inserta en `empleados`), pero
--   Fase 4 (000003_calendario_asistencia_vacaciones.up.sql) hace FK de
--   incapacidades/asistencia contra `usuarios(id)`. Aquí se sigue el patrón
--   de Fase 3 (`empleados(id)`) porque es el dominio directamente dueño del
--   expediente laboral que esta fase cierra (contratos, datos_laborales).
--   Confirmar con el equipo cuál es la convención real antes de aplicar en
--   un ambiente compartido; si `usuarios(id)` es lo correcto, cambiar las 4
--   FKs de abajo.
--
--   ASUNCIÓN A CONFIRMAR — periodos_planilla.fecha_inicio_periodo /
--   fecha_fin_periodo: ADR-000 y el contrato de Fase 6a (domain.go,
--   CalculoEmpleadoInput.FechaInicioPeriodo/FechaFinPeriodo) implican que
--   periodos_planilla tiene estas columnas, pero ningún .sql fuente entregado
--   las declara explícitamente. Los queries de este archivo asumen que
--   existen con esos nombres exactos.
--
--   Sigue la convención heredada: idempotente (IF NOT EXISTS), snake_case,
--   InnoDB, comentarios en columnas.

-- ============================================================================
-- Parámetro vigente: porcentaje de Salario Escolar.
-- Se versiona con el mismo patrón fecha_inicio_vigencia/fecha_fin_vigencia
-- que las 19 tablas de ADR-000 §2, para no hardcodear el porcentaje en Go
-- (misma directiva de "sin valores por defecto hardcodeados" de Fase 6a).
-- ============================================================================
CREATE TABLE IF NOT EXISTS parametros_salario_escolar (
    id INT AUTO_INCREMENT PRIMARY KEY,
    porcentaje DECIMAL(5,4) NOT NULL COMMENT 'porcentaje aplicado sobre salarios brutos del periodo anual, ej. 0.0833 para 1/12',
    fecha_inicio_vigencia DATE NOT NULL,
    fecha_fin_vigencia DATE NULL,
    es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED,
    creado_por BIGINT UNSIGNED NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY pse_una_vigente (es_vigente)
) ENGINE=InnoDB COMMENT='Porcentaje vigente de Salario Escolar (único registro vigente a la vez, no versionado por clasificación)';

-- ============================================================================
-- Resultado persistido de Aguinaldo (un registro por empleado+año de pago).
-- ============================================================================
CREATE TABLE IF NOT EXISTS aguinaldos (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    empleado_id BIGINT UNSIGNED NOT NULL,
    anio SMALLINT NOT NULL COMMENT 'año calendario de pago; cubre el periodo 1-dic del año anterior a 30-nov de este año',
    periodo_desde DATE NOT NULL,
    periodo_hasta DATE NOT NULL,
    total_salarios_brutos DECIMAL(14,2) NOT NULL,
    monto_aguinaldo DECIMAL(14,2) NOT NULL,
    estado ENUM('proyectado','procesado') NOT NULL DEFAULT 'proyectado',
    calculado_en DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    procesado_por BIGINT UNSIGNED NULL,
    procesado_en DATETIME NULL,
    UNIQUE KEY aguinaldos_empleado_anio_unique (empleado_id, anio),
    CONSTRAINT fk_aguinaldo_empleado FOREIGN KEY (empleado_id) REFERENCES empleados(id)
) ENGINE=InnoDB COMMENT='Resultado auditable del cálculo de aguinaldo por empleado y año';

-- ============================================================================
-- Resultado persistido de Salario Escolar (un registro por empleado+año).
-- ============================================================================
CREATE TABLE IF NOT EXISTS salarios_escolares (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    empleado_id BIGINT UNSIGNED NOT NULL,
    anio SMALLINT NOT NULL COMMENT 'año calendario, periodo 1-ene a 31-dic',
    periodo_desde DATE NOT NULL,
    periodo_hasta DATE NOT NULL,
    porcentaje_aplicado DECIMAL(5,4) NOT NULL COMMENT 'copia del parametros_salario_escolar.porcentaje vigente al momento del cálculo, para trazabilidad histórica',
    total_salarios_brutos DECIMAL(14,2) NOT NULL,
    monto_salario_escolar DECIMAL(14,2) NOT NULL,
    estado ENUM('proyectado','procesado') NOT NULL DEFAULT 'proyectado',
    calculado_en DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    procesado_por BIGINT UNSIGNED NULL,
    procesado_en DATETIME NULL,
    UNIQUE KEY salesc_empleado_anio_unique (empleado_id, anio),
    CONSTRAINT fk_salesc_empleado FOREIGN KEY (empleado_id) REFERENCES empleados(id)
) ENGINE=InnoDB COMMENT='Resultado auditable del cálculo de salario escolar por empleado y año';

-- ============================================================================
-- Resultado persistido de Liquidación por cesación de contrato.
-- Un registro por cesación (no versionado — es un evento terminal, no una
-- entidad con vigencia). "proyectar" NUNCA inserta aquí (ver servicio);
-- solo "procesar" persiste con estado='procesada'.
-- ============================================================================
CREATE TABLE IF NOT EXISTS liquidaciones (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    empleado_id BIGINT UNSIGNED NOT NULL,
    fecha_salida DATE NOT NULL,
    motivo_salida ENUM(
        'despido_sin_responsabilidad',
        'despido_con_responsabilidad',
        'despido_indirecto',
        'renuncia',
        'vencimiento_contrato',
        'mutuo_acuerdo',
        'fallecimiento'
    ) NOT NULL,
    -- Deliberadamente explícito y NO derivado automáticamente de
    -- motivo_salida: la decisión de si corresponde preaviso/cesantía es de
    -- negocio (ej. un despido_indirecto puede o no reconocerse con
    -- responsabilidad patronal según análisis legal caso a caso — ver
    -- ADR-000 estilo "sin valores por defecto implícitos").
    aplica_responsabilidad_patronal TINYINT(1) NOT NULL COMMENT 'controla si se calculan preaviso y cesantía; boolean "pagoPreaviso" del enunciado',
    fecha_ingreso DATE NOT NULL COMMENT 'copiado del contrato/datos_laborales vigente al momento del cálculo, para trazabilidad histórica',
    antiguedad_meses INT NOT NULL,
    salario_promedio_6meses DECIMAL(14,2) NOT NULL,
    salario_diario_promedio DECIMAL(14,4) NOT NULL,
    dias_preaviso SMALLINT NOT NULL DEFAULT 0,
    monto_preaviso DECIMAL(14,2) NOT NULL DEFAULT 0,
    dias_cesantia DECIMAL(6,2) NOT NULL DEFAULT 0,
    monto_cesantia DECIMAL(14,2) NOT NULL DEFAULT 0,
    dias_vacaciones_pendientes DECIMAL(5,2) NOT NULL DEFAULT 0,
    monto_vacaciones_proporcionales DECIMAL(14,2) NOT NULL DEFAULT 0,
    periodo_aguinaldo_desde DATE NOT NULL,
    total_salarios_brutos_aguinaldo_proporcional DECIMAL(14,2) NOT NULL DEFAULT 0,
    monto_aguinaldo_proporcional DECIMAL(14,2) NOT NULL DEFAULT 0,
    total_liquidacion DECIMAL(14,2) NOT NULL,
    estado ENUM('proyectada','procesada') NOT NULL DEFAULT 'proyectada',
    calculado_en DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    procesado_por BIGINT UNSIGNED NULL,
    procesado_en DATETIME NULL,
    KEY liquidaciones_empleado_idx (empleado_id, fecha_salida),
    CONSTRAINT fk_liquidacion_empleado FOREIGN KEY (empleado_id) REFERENCES empleados(id)
) ENGINE=InnoDB COMMENT='Resultado auditable de liquidaciones por cesación de contrato';
