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
