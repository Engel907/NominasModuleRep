SET @fecha_vigencia_2026 = COALESCE(@fecha_vigencia_2026, '2026-01-01');

CREATE TABLE parametros_generales (
  id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del parámetro',
  clave                  VARCHAR(80)  NOT NULL COMMENT 'Clave estable consumida por el código (ej. DIVISOR_DIAS_MES)',
  valor                  VARCHAR(255) NOT NULL COMMENT 'Valor serializado como texto; se interpreta según tipo. porcentaje se guarda como porcentaje (10.83), fecha como YYYY-MM-DD, booleano como true/false',
  tipo                   ENUM('entero','decimal','porcentaje','texto','fecha','booleano') NOT NULL COMMENT 'Tipo con el que el servicio interpreta valor',
  descripcion            VARCHAR(255) NOT NULL COMMENT 'Qué controla este parámetro y dónde se usa',
  fuente_legal           VARCHAR(150) NOT NULL COMMENT 'Base legal del valor',
  fecha_inicio_vigencia  DATE NOT NULL COMMENT 'Fecha desde la cual este valor es el vigente',
  fecha_fin_vigencia     DATE NULL COMMENT 'NULL = vigente actualmente',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED
    COMMENT 'Columna técnica: soporta uk_pg_una_vigente',
  creado_por             INT NOT NULL COMMENT 'Usuario que registró el valor',
  created_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Fecha/hora de creación',
  CONSTRAINT chk_pg_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  UNIQUE KEY uk_pg_una_vigente (clave, es_vigente),
  KEY ix_pg_vigencia (clave, fecha_inicio_vigencia, fecha_fin_vigencia)
) ENGINE=InnoDB COMMENT='Parámetros legales/operativos generales (clave-valor tipado), versionados por vigencia';

CREATE TABLE parametros_embargo_escala (
  id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del tramo',
  desde_multiplo         DECIMAL(8,4) NOT NULL COMMENT 'Inicio del tramo, en múltiplos del salario mínimo inembargable, medido sobre el excedente',
  hasta_multiplo         DECIMAL(8,4) NULL COMMENT 'Fin del tramo en múltiplos; NULL = sin tope superior',
  porcentaje_maximo      DECIMAL(5,2) NOT NULL COMMENT 'Porcentaje máximo embargable de la porción del excedente dentro del tramo (12.50 = 12.5 %)',
  fuente_legal           VARCHAR(150) NOT NULL COMMENT 'Base legal del valor',
  fecha_inicio_vigencia  DATE NOT NULL COMMENT 'Fecha desde la cual este tramo es el vigente',
  fecha_fin_vigencia     DATE NULL COMMENT 'NULL = vigente actualmente',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED
    COMMENT 'Columna técnica: soporta uk_pee_una_vigente',
  creado_por             INT NOT NULL COMMENT 'Usuario que registró el tramo',
  created_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Fecha/hora de creación',
  CONSTRAINT chk_pee_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  CONSTRAINT chk_pee_rango CHECK (hasta_multiplo IS NULL OR hasta_multiplo > desde_multiplo),
  CONSTRAINT chk_pee_desde CHECK (desde_multiplo >= 0),
  CONSTRAINT chk_pee_porcentaje CHECK (porcentaje_maximo > 0 AND porcentaje_maximo <= 100),
  UNIQUE KEY uk_pee_una_vigente (desde_multiplo, es_vigente),
  KEY ix_pee_vigencia (desde_multiplo, fecha_inicio_vigencia, fecha_fin_vigencia)
) ENGINE=InnoDB COMMENT='Escala de embargo sobre el excedente del salario embargable (Art. 172 CT), versionada por vigencia';

INSERT INTO parametros_embargo_escala
    (desde_multiplo, hasta_multiplo, porcentaje_maximo, fuente_legal, fecha_inicio_vigencia, fecha_fin_vigencia, creado_por)
SELECT v.desde, v.hasta, v.pct, 'Código de Trabajo, art. 172', @fecha_vigencia_2026, NULL, 1
FROM (
    SELECT 0.0000 AS desde, 3.0000 AS hasta, 12.50 AS pct
    UNION ALL SELECT 3.0000, NULL, 25.00
) v
WHERE NOT EXISTS (
    SELECT 1 FROM parametros_embargo_escala e
    WHERE e.desde_multiplo = v.desde AND e.es_vigente = 1
);

ALTER TABLE parametros_tipos_embargo
  ADD COLUMN es_pension_alimentaria TINYINT(1) NOT NULL DEFAULT 0
    COMMENT '1 = pensión alimentaria (se practica primero y tiene tope propio)',
  ADD COLUMN tope_porcentaje_liquido DECIMAL(5,2) NULL
    COMMENT 'Tope máximo del tipo como porcentaje del salario líquido (50.00 = 50 %); NULL = sin tope propio',
  ADD CONSTRAINT chk_pte_tope CHECK (tope_porcentaje_liquido IS NULL OR (tope_porcentaje_liquido > 0 AND tope_porcentaje_liquido <= 100));

UPDATE parametros_tipos_embargo
   SET es_pension_alimentaria = 1,
       tope_porcentaje_liquido = 50.00
 WHERE codigo = 'PENSION_ALIMENTARIA'
   AND es_pension_alimentaria = 0;

INSERT INTO parametros_generales
    (clave, valor, tipo, descripcion, fuente_legal, fecha_inicio_vigencia, fecha_fin_vigencia, creado_por)
SELECT v.clave, v.valor, v.tipo, v.descripcion, 'Configuración inicial', @fecha_vigencia_2026, NULL, 1
FROM (
    SELECT 'HORAS_MENSUALES_ORDINARIAS' AS clave, '240' AS valor, 'entero' AS tipo, 'Horas ordinarias mensuales para calcular valor hora y horas extra (8 horas x 30 días)' AS descripcion
    UNION ALL SELECT 'DIVISOR_DIAS_MES', '30', 'entero', 'Divisor para obtener el salario diario a partir del mensual'
    UNION ALL SELECT 'AGUINALDO_MESES', '12', 'entero', 'Divisor del aguinaldo: total devengado en el período legal / N'
    UNION ALL SELECT 'LIQ_FRACCION_MESES_UMBRAL', '6', 'entero', 'Una fracción de años cuenta como año completo si los meses de fracción son estrictamente mayores a este valor'
    UNION ALL SELECT 'AGUINALDO_PERIODO_INICIO_MES', '12', 'entero', 'Aguinaldo, inicio del período legal: mes'
    UNION ALL SELECT 'AGUINALDO_PERIODO_INICIO_DIA', '1', 'entero', 'Aguinaldo, inicio del período legal: día'
    UNION ALL SELECT 'AGUINALDO_PERIODO_INICIO_ANIO_OFFSET', '-1', 'entero', 'Aguinaldo, inicio del período legal: desplazamiento de año respecto al año de la corrida'
    UNION ALL SELECT 'AGUINALDO_PERIODO_FIN_MES', '11', 'entero', 'Aguinaldo, fin del período legal: mes'
    UNION ALL SELECT 'AGUINALDO_PERIODO_FIN_DIA', '30', 'entero', 'Aguinaldo, fin del período legal: día'
    UNION ALL SELECT 'AGUINALDO_PERIODO_FIN_ANIO_OFFSET', '0', 'entero', 'Aguinaldo, fin del período legal: desplazamiento de año respecto al año de la corrida'
    UNION ALL SELECT 'AGUINALDO_VENTANA_INICIO_MES', '12', 'entero', 'Aguinaldo, inicio de la ventana de ejecución: mes'
    UNION ALL SELECT 'AGUINALDO_VENTANA_INICIO_DIA', '1', 'entero', 'Aguinaldo, inicio de la ventana de ejecución: día'
    UNION ALL SELECT 'AGUINALDO_VENTANA_INICIO_ANIO_OFFSET', '0', 'entero', 'Aguinaldo, inicio de la ventana de ejecución: desplazamiento de año respecto al año de la corrida'
    UNION ALL SELECT 'AGUINALDO_VENTANA_FIN_MES', '12', 'entero', 'Aguinaldo, fin de la ventana de ejecución: mes'
    UNION ALL SELECT 'AGUINALDO_VENTANA_FIN_DIA', '31', 'entero', 'Aguinaldo, fin de la ventana de ejecución: día'
    UNION ALL SELECT 'AGUINALDO_VENTANA_FIN_ANIO_OFFSET', '0', 'entero', 'Aguinaldo, fin de la ventana de ejecución: desplazamiento de año respecto al año de la corrida'
    UNION ALL SELECT 'AGUINALDO_PAGO_LIMITE_MES', '12', 'entero', 'Aguinaldo, fecha límite legal de pago: mes'
    UNION ALL SELECT 'AGUINALDO_PAGO_LIMITE_DIA', '20', 'entero', 'Aguinaldo, fecha límite legal de pago: día'
    UNION ALL SELECT 'AGUINALDO_PAGO_LIMITE_ANIO_OFFSET', '0', 'entero', 'Aguinaldo, fecha límite legal de pago: desplazamiento de año respecto al año de la corrida'
    UNION ALL SELECT 'SALESC_PERIODO_INICIO_MES', '1', 'entero', 'Salario escolar, inicio del período: mes'
    UNION ALL SELECT 'SALESC_PERIODO_INICIO_DIA', '1', 'entero', 'Salario escolar, inicio del período: día'
    UNION ALL SELECT 'SALESC_PERIODO_INICIO_ANIO_OFFSET', '0', 'entero', 'Salario escolar, inicio del período: desplazamiento de año respecto al año de la corrida'
    UNION ALL SELECT 'SALESC_PERIODO_FIN_MES', '12', 'entero', 'Salario escolar, fin del período: mes'
    UNION ALL SELECT 'SALESC_PERIODO_FIN_DIA', '31', 'entero', 'Salario escolar, fin del período: día'
    UNION ALL SELECT 'SALESC_PERIODO_FIN_ANIO_OFFSET', '0', 'entero', 'Salario escolar, fin del período: desplazamiento de año respecto al año de la corrida'
    UNION ALL SELECT 'SALESC_VENTANA_INICIO_MES', '1', 'entero', 'Salario escolar, inicio de la ventana de ejecución: mes'
    UNION ALL SELECT 'SALESC_VENTANA_INICIO_DIA', '1', 'entero', 'Salario escolar, inicio de la ventana de ejecución: día'
    UNION ALL SELECT 'SALESC_VENTANA_INICIO_ANIO_OFFSET', '1', 'entero', 'Salario escolar, inicio de la ventana de ejecución: desplazamiento de año respecto al año de la corrida'
    UNION ALL SELECT 'SALESC_VENTANA_FIN_MES', '1', 'entero', 'Salario escolar, fin de la ventana de ejecución: mes'
    UNION ALL SELECT 'SALESC_VENTANA_FIN_DIA', '31', 'entero', 'Salario escolar, fin de la ventana de ejecución: día'
    UNION ALL SELECT 'SALESC_VENTANA_FIN_ANIO_OFFSET', '1', 'entero', 'Salario escolar, fin de la ventana de ejecución: desplazamiento de año respecto al año de la corrida'
) v
WHERE NOT EXISTS (
    SELECT 1 FROM parametros_generales g
    WHERE g.clave = v.clave AND g.es_vigente = 1
);