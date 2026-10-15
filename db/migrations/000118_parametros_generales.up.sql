-- ============================================================================
-- Fase 18A — parametros_generales, escala de embargo y flags de tipos de embargo.
--
-- Convención de porcentajes en BD (D3): los porcentajes se guardan como
-- porcentaje (12.50 = 12.5 %), igual que parametros_cargas_sociales. La
-- conversión a fracción ocurre ÚNICAMENTE en internal/shared/porcentaje.
--
-- Patrón de vigencia de 2 pasos (igual que parametros_cargas_sociales):
--   1) UPDATE ... SET fecha_fin_vigencia = <día anterior> WHERE es_vigente = 1
--   2) INSERT de la nueva fila con fecha_fin_vigencia NULL
-- Los seeds usan INSERT ... SELECT ... WHERE NOT EXISTS (nunca ON DUPLICATE KEY).
--
-- ASUNCIÓN A CONFIRMAR — creado_por = 1 (usuario de sistema / carga inicial),
-- mismo placeholder que 000005_seed_catalogos_parametricos.
-- ASUNCIÓN A CONFIRMAR — vigencia desde 2026-01-01 (@fecha_vigencia_2026), como
-- los demás seeds: un cálculo con fecha efectiva anterior fallará con
-- ErrParametroNoConfigurado (D8) en lugar de usar un valor por defecto.
-- ============================================================================
SET @fecha_vigencia_2026 = COALESCE(@fecha_vigencia_2026, '2026-01-01');

-- ---------------------------------------------------------------------------
-- a) parametros_generales
-- ---------------------------------------------------------------------------
CREATE TABLE parametros_generales (
  id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del parámetro',
  clave                  VARCHAR(80)  NOT NULL COMMENT 'Clave estable consumida por el código (ej. DIVISOR_DIAS_MES)',
  valor                  VARCHAR(255) NOT NULL COMMENT 'Valor serializado como texto; se interpreta según tipo. porcentaje se guarda como porcentaje (10.83), fecha como YYYY-MM-DD, booleano como true/false',
  tipo                   ENUM('entero','decimal','porcentaje','texto','fecha','booleano') NOT NULL COMMENT 'Tipo con el que el servicio interpreta valor',
  descripcion            VARCHAR(255) NOT NULL COMMENT 'Qué controla este parámetro y dónde se usa',
  fuente_legal           VARCHAR(150) NOT NULL COMMENT 'Base legal o, mientras no se valide, la marca PENDIENTE VALIDAR',
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

-- ---------------------------------------------------------------------------
-- b) parametros_embargo_escala — Art. 172 Código de Trabajo
--    Tramos sobre el EXCEDENTE del salario embargable (remanente menos el
--    salario mínimo inembargable), medidos en múltiplos de ese salario mínimo.
--    Valores derivados de internal/planilla/calculator.go:43-45,39.
-- ---------------------------------------------------------------------------
CREATE TABLE parametros_embargo_escala (
  id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del tramo',
  desde_multiplo         DECIMAL(8,4) NOT NULL COMMENT 'Inicio del tramo, en múltiplos del salario mínimo inembargable, medido sobre el excedente',
  hasta_multiplo         DECIMAL(8,4) NULL COMMENT 'Fin del tramo en múltiplos; NULL = sin tope superior',
  porcentaje_maximo      DECIMAL(5,2) NOT NULL COMMENT 'Porcentaje máximo embargable de la porción del excedente dentro del tramo (12.50 = 12.5 %)',
  fuente_legal           VARCHAR(150) NOT NULL COMMENT 'Base legal o marca PENDIENTE VALIDAR',
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
SELECT v.desde, v.hasta, v.pct, 'PENDIENTE VALIDAR (Art. 172 Código de Trabajo)', @fecha_vigencia_2026, NULL, 1
FROM (
    SELECT 0.0000 AS desde, 3.0000 AS hasta, 12.50 AS pct
    UNION ALL SELECT 3.0000, NULL, 25.00
) v
WHERE NOT EXISTS (
    SELECT 1 FROM parametros_embargo_escala e
    WHERE e.desde_multiplo = v.desde AND e.es_vigente = 1
);

-- ---------------------------------------------------------------------------
-- c) parametros_tipos_embargo: identificar la pensión alimentaria por flag
--    (reemplaza PrioridadPensionAlimentaria = 1) y tope sobre el líquido.
--    Valor del tope derivado de calculator.go:45 (0.5 -> 50.00 %).
-- ---------------------------------------------------------------------------
ALTER TABLE parametros_tipos_embargo
  ADD COLUMN es_pension_alimentaria TINYINT(1) NOT NULL DEFAULT 0
    COMMENT '1 = pensión alimentaria (se practica primero y tiene tope propio); reemplaza identificar por prioridad_legal = 1',
  ADD COLUMN tope_porcentaje_liquido DECIMAL(5,2) NULL
    COMMENT 'Tope máximo del tipo como porcentaje del salario líquido (50.00 = 50 %); NULL = sin tope propio',
  ADD CONSTRAINT chk_pte_tope CHECK (tope_porcentaje_liquido IS NULL OR (tope_porcentaje_liquido > 0 AND tope_porcentaje_liquido <= 100));

UPDATE parametros_tipos_embargo
   SET es_pension_alimentaria = 1,
       tope_porcentaje_liquido = 50.00
 WHERE codigo = 'PENSION_ALIMENTARIA'
   AND es_pension_alimentaria = 0;

-- ---------------------------------------------------------------------------
-- d) Siembra en parametros_generales de las constantes legales hoy en Go.
--    Solo valores presentes en el código (no se inventa ninguno).
-- ---------------------------------------------------------------------------
INSERT INTO parametros_generales
    (clave, valor, tipo, descripcion, fuente_legal, fecha_inicio_vigencia, fecha_fin_vigencia, creado_por)
SELECT v.clave, v.valor, v.tipo, v.descripcion, 'VALOR HISTÓRICO DEL CÓDIGO - PENDIENTE VALIDAR', @fecha_vigencia_2026, NULL, 1
FROM (
    SELECT 'HORAS_MENSUALES_ORDINARIAS' AS clave, '240' AS valor, 'entero' AS tipo, 'Horas ordinarias mensuales para calcular valor hora y horas extra (8 horas x 30 días; confirmar según jornada institucional)' AS descripcion
    UNION ALL SELECT 'DIVISOR_DIAS_MES', '30', 'entero', 'Divisor para obtener el salario diario a partir del mensual (liquidacion_service.go:58,425)'
    UNION ALL SELECT 'AGUINALDO_MESES', '12', 'entero', 'Divisor del aguinaldo: total devengado en el período legal / N (acumulados_periodicos.go:22,29)'
    UNION ALL SELECT 'LIQ_FRACCION_MESES_UMBRAL', '6', 'entero', 'Una fracción de años cuenta como año completo si los meses de fracción son estrictamente mayores a este valor (liquidacion_service.go:372)'
    UNION ALL SELECT 'AGUINALDO_PERIODO_INICIO_MES', '12', 'entero', 'Aguinaldo, inicio del período legal: mes (acumulados_periodicos.go:112)'
    UNION ALL SELECT 'AGUINALDO_PERIODO_INICIO_DIA', '1', 'entero', 'Aguinaldo, inicio del período legal: día (acumulados_periodicos.go:112)'
    UNION ALL SELECT 'AGUINALDO_PERIODO_INICIO_ANIO_OFFSET', '-1', 'entero', 'Aguinaldo, inicio del período legal: desplazamiento de año respecto al año de la corrida (acumulados_periodicos.go:112)'
    UNION ALL SELECT 'AGUINALDO_PERIODO_FIN_MES', '11', 'entero', 'Aguinaldo, fin del período legal: mes (acumulados_periodicos.go:112)'
    UNION ALL SELECT 'AGUINALDO_PERIODO_FIN_DIA', '30', 'entero', 'Aguinaldo, fin del período legal: día (acumulados_periodicos.go:112)'
    UNION ALL SELECT 'AGUINALDO_PERIODO_FIN_ANIO_OFFSET', '0', 'entero', 'Aguinaldo, fin del período legal: desplazamiento de año respecto al año de la corrida (acumulados_periodicos.go:112)'
    UNION ALL SELECT 'AGUINALDO_VENTANA_INICIO_MES', '12', 'entero', 'Aguinaldo, inicio de la ventana de ejecución: mes (acumulados_periodicos.go:116)'
    UNION ALL SELECT 'AGUINALDO_VENTANA_INICIO_DIA', '1', 'entero', 'Aguinaldo, inicio de la ventana de ejecución: día (acumulados_periodicos.go:116)'
    UNION ALL SELECT 'AGUINALDO_VENTANA_INICIO_ANIO_OFFSET', '0', 'entero', 'Aguinaldo, inicio de la ventana de ejecución: desplazamiento de año respecto al año de la corrida (acumulados_periodicos.go:116)'
    UNION ALL SELECT 'AGUINALDO_VENTANA_FIN_MES', '12', 'entero', 'Aguinaldo, fin de la ventana de ejecución: mes (acumulados_periodicos.go:116)'
    UNION ALL SELECT 'AGUINALDO_VENTANA_FIN_DIA', '31', 'entero', 'Aguinaldo, fin de la ventana de ejecución: día (acumulados_periodicos.go:116)'
    UNION ALL SELECT 'AGUINALDO_VENTANA_FIN_ANIO_OFFSET', '0', 'entero', 'Aguinaldo, fin de la ventana de ejecución: desplazamiento de año respecto al año de la corrida (acumulados_periodicos.go:116)'
    UNION ALL SELECT 'AGUINALDO_PAGO_LIMITE_MES', '12', 'entero', 'Aguinaldo, fecha límite legal de pago: mes (acumulados_periodicos.go:121)'
    UNION ALL SELECT 'AGUINALDO_PAGO_LIMITE_DIA', '20', 'entero', 'Aguinaldo, fecha límite legal de pago: día (acumulados_periodicos.go:121)'
    UNION ALL SELECT 'AGUINALDO_PAGO_LIMITE_ANIO_OFFSET', '0', 'entero', 'Aguinaldo, fecha límite legal de pago: desplazamiento de año respecto al año de la corrida (acumulados_periodicos.go:121)'
    UNION ALL SELECT 'SALESC_PERIODO_INICIO_MES', '1', 'entero', 'Salario escolar, inicio del período: mes (acumulados_periodicos.go:173)'
    UNION ALL SELECT 'SALESC_PERIODO_INICIO_DIA', '1', 'entero', 'Salario escolar, inicio del período: día (acumulados_periodicos.go:173)'
    UNION ALL SELECT 'SALESC_PERIODO_INICIO_ANIO_OFFSET', '0', 'entero', 'Salario escolar, inicio del período: desplazamiento de año respecto al año de la corrida (acumulados_periodicos.go:173)'
    UNION ALL SELECT 'SALESC_PERIODO_FIN_MES', '12', 'entero', 'Salario escolar, fin del período: mes (acumulados_periodicos.go:173)'
    UNION ALL SELECT 'SALESC_PERIODO_FIN_DIA', '31', 'entero', 'Salario escolar, fin del período: día (acumulados_periodicos.go:173)'
    UNION ALL SELECT 'SALESC_PERIODO_FIN_ANIO_OFFSET', '0', 'entero', 'Salario escolar, fin del período: desplazamiento de año respecto al año de la corrida (acumulados_periodicos.go:173)'
    UNION ALL SELECT 'SALESC_VENTANA_INICIO_MES', '1', 'entero', 'Salario escolar, inicio de la ventana de ejecución: mes (acumulados_periodicos.go:177)'
    UNION ALL SELECT 'SALESC_VENTANA_INICIO_DIA', '1', 'entero', 'Salario escolar, inicio de la ventana de ejecución: día (acumulados_periodicos.go:177)'
    UNION ALL SELECT 'SALESC_VENTANA_INICIO_ANIO_OFFSET', '1', 'entero', 'Salario escolar, inicio de la ventana de ejecución: desplazamiento de año respecto al año de la corrida (acumulados_periodicos.go:177)'
    UNION ALL SELECT 'SALESC_VENTANA_FIN_MES', '1', 'entero', 'Salario escolar, fin de la ventana de ejecución: mes (acumulados_periodicos.go:177)'
    UNION ALL SELECT 'SALESC_VENTANA_FIN_DIA', '31', 'entero', 'Salario escolar, fin de la ventana de ejecución: día (acumulados_periodicos.go:177)'
    UNION ALL SELECT 'SALESC_VENTANA_FIN_ANIO_OFFSET', '1', 'entero', 'Salario escolar, fin de la ventana de ejecución: desplazamiento de año respecto al año de la corrida (acumulados_periodicos.go:177)'
) v
WHERE NOT EXISTS (
    SELECT 1 FROM parametros_generales g
    WHERE g.clave = v.clave AND g.es_vigente = 1
);