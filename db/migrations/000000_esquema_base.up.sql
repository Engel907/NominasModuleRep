SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

CREATE TABLE tipos_identificacion (
  id      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno',
  codigo  VARCHAR(20)  NOT NULL UNIQUE COMMENT 'p.ej. CEDULA_NACIONAL, DIMEX, PASAPORTE, CEDULA_RESIDENCIA',
  nombre  VARCHAR(100) NOT NULL COMMENT 'Nombre descriptivo del tipo de identificación',
  activo  TINYINT(1) NOT NULL DEFAULT 1 COMMENT '1 = tipo utilizable, 0 = descontinuado'
) ENGINE=InnoDB COMMENT='Catálogo abierto de tipos de identificación (antes ENUM en empleados.tipo_identificacion)';

INSERT INTO tipos_identificacion (codigo, nombre) VALUES
  ('CEDULA_NACIONAL','Cédula de identidad nacional'),
  ('DIMEX','Documento de identificación migratoria para extranjeros'),
  ('PASAPORTE','Pasaporte'),
  ('CEDULA_RESIDENCIA','Cédula de residencia');

CREATE TABLE clasificaciones_puesto (
  id      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno',
  codigo  VARCHAR(20)  NOT NULL UNIQUE COMMENT 'p.ej. OPERATIVO, ADMINISTRATIVO, GUARDA_VIGILANCIA, DIRECCION',
  nombre  VARCHAR(100) NOT NULL COMMENT 'Nombre de la categoría según Decreto de Salarios Mínimos / Ley 10159',
  activo  TINYINT(1) NOT NULL DEFAULT 1 COMMENT '1 = clasificación utilizable, 0 = descontinuada'
) ENGINE=InnoDB COMMENT='Catálogo de clasificaciones de puesto (antes VARCHAR libre en puestos.clasificacion)';

INSERT INTO clasificaciones_puesto (codigo, nombre) VALUES
  ('OPERATIVO','Operativo'),
  ('ADMINISTRATIVO','Administrativo'),
  ('GUARDA_VIGILANCIA','Guarda de vigilancia'),
  ('DIRECCION','Dirección'),
  ('SECRETARIA','Secretaría'),
  ('BODEGUERO','Bodeguero'),
  ('MISCELANEA','Miscelánea');

CREATE TABLE regimenes_ccss (
  id      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno',
  codigo  VARCHAR(20)  NOT NULL UNIQUE COMMENT 'p.ej. SEM, IVM, ROP, riesgos futuros',
  nombre  VARCHAR(100) NOT NULL COMMENT 'Nombre del régimen de la CCSS',
  activo  TINYINT(1) NOT NULL DEFAULT 1 COMMENT '1 = régimen utilizable, 0 = descontinuado'
) ENGINE=InnoDB COMMENT='Catálogo abierto de regímenes de la CCSS (antes ENUM SEM/IVM)';

INSERT INTO regimenes_ccss (codigo, nombre) VALUES
  ('SEM','Seguro de Enfermedad y Maternidad'),
  ('IVM','Invalidez, Vejez y Muerte');

CREATE TABLE tipos_credito_fiscal_renta (
  id      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno',
  codigo  VARCHAR(20)  NOT NULL UNIQUE COMMENT 'p.ej. CONYUGE, HIJO, ASCENDIENTE, DISCAPACIDAD',
  nombre  VARCHAR(100) NOT NULL COMMENT 'Nombre del tipo de crédito fiscal',
  activo  TINYINT(1) NOT NULL DEFAULT 1 COMMENT '1 = crédito utilizable, 0 = descontinuado'
) ENGINE=InnoDB COMMENT='Catálogo abierto de tipos de crédito fiscal Art. 33 Ley 7092 (antes ENUM conyuge/hijo)';

INSERT INTO tipos_credito_fiscal_renta (codigo, nombre) VALUES
  ('CONYUGE','Cónyuge'),
  ('HIJO','Hijo dependiente');

CREATE TABLE entidades_emisoras_incapacidad (
  id      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno',
  codigo  VARCHAR(20)  NOT NULL UNIQUE COMMENT 'p.ej. CCSS, INS, futura aseguradora privada',
  nombre  VARCHAR(100) NOT NULL COMMENT 'Nombre de la entidad emisora',
  activo  TINYINT(1) NOT NULL DEFAULT 1 COMMENT '1 = entidad utilizable, 0 = descontinuada'
) ENGINE=InnoDB COMMENT='Catálogo abierto de entidades que emiten incapacidades (antes ENUM CCSS/INS)';

INSERT INTO entidades_emisoras_incapacidad (codigo, nombre) VALUES
  ('CCSS','Caja Costarricense de Seguro Social'),
  ('INS','Instituto Nacional de Seguros');

CREATE TABLE tipos_hora_extra (
  id      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno',
  codigo  VARCHAR(20)  NOT NULL UNIQUE COMMENT 'p.ej. DIURNA, NOCTURNA, MIXTA, DOBLE, FERIADO',
  nombre  VARCHAR(100) NOT NULL COMMENT 'Nombre del tipo de hora extra',
  activo  TINYINT(1) NOT NULL DEFAULT 1 COMMENT '1 = tipo utilizable, 0 = descontinuado'
) ENGINE=InnoDB COMMENT='Catálogo abierto de tipos de hora extra (antes ENUM diurna/nocturna/mixta/doble)';

INSERT INTO tipos_hora_extra (codigo, nombre) VALUES
  ('DIURNA','Hora extra diurna'),
  ('NOCTURNA','Hora extra nocturna'),
  ('MIXTA','Hora extra mixta'),
  ('DOBLE','Hora doble');

CREATE TABLE unidades_organizativas (
  id                  BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno de la unidad',
  codigo              VARCHAR(20)  NOT NULL UNIQUE COMMENT 'Código corto de la unidad, único (p.ej. RRHH, PROV)',
  nombre              VARCHAR(150) NOT NULL COMMENT 'Nombre descriptivo de la unidad organizativa',
  unidad_padre_id     BIGINT UNSIGNED NULL COMMENT 'Unidad de la que depende (auto-referencia); NULL si es la raíz del organigrama',
  activo              TINYINT(1)   NOT NULL DEFAULT 1 COMMENT '1 = unidad vigente/en uso, 0 = dada de baja (no se borra para no perder histórico)',
  created_at          TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Fecha/hora en que se creó el registro',
  updated_at          TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Fecha/hora de la última modificación del registro',
  CONSTRAINT fk_unidad_padre FOREIGN KEY (unidad_padre_id) REFERENCES unidades_organizativas(id)
) ENGINE=InnoDB COMMENT='Departamentos/unidades con jerarquía';

CREATE TABLE programas_presupuestarios (
  id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del programa presupuestario',
  codigo      VARCHAR(20)  NOT NULL UNIQUE COMMENT 'Código corto y único del programa (p.ej. PROG1, PROG2)',
  nombre      VARCHAR(150) NOT NULL COMMENT 'Nombre del programa presupuestario (p.ej. "Administración", "Operativos")',
  activo      TINYINT(1)   NOT NULL DEFAULT 1 COMMENT '1 = programa vigente y utilizable, 0 = descontinuado',
  notas       VARCHAR(255) NULL COMMENT 'Observaciones del programa'
) ENGINE=InnoDB COMMENT='Programas presupuestarios o centros de costo a los que se imputa el gasto de planilla';

CREATE TABLE puestos (
  id                      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del puesto',
  codigo                  VARCHAR(20)  NOT NULL UNIQUE COMMENT 'Código corto y único del puesto',
  nombre                  VARCHAR(150) NOT NULL COMMENT 'Nombre/título del puesto (p.ej. "Analista de RRHH")',
  unidad_organizativa_id  BIGINT UNSIGNED NOT NULL COMMENT 'Unidad organizativa a la que pertenece el puesto',
  clasificacion_puesto_id BIGINT UNSIGNED NULL COMMENT 'Categoría del puesto para efectos de Ley 10159 / Decreto de Salarios Mínimos (FK)',
  activo                  TINYINT(1)   NOT NULL DEFAULT 1 COMMENT '1 = puesto vigente/disponible, 0 = eliminado del catálogo activo',
  CONSTRAINT fk_puesto_unidad FOREIGN KEY (unidad_organizativa_id) REFERENCES unidades_organizativas(id),
  CONSTRAINT fk_puesto_clasificacion FOREIGN KEY (clasificacion_puesto_id) REFERENCES clasificaciones_puesto(id)
) ENGINE=InnoDB COMMENT='Catálogo de puestos de trabajo, cada uno asociado a una unidad organizativa';

CREATE TABLE tipos_contrato (
  id           BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del tipo de contrato',
  codigo       VARCHAR(20)  NOT NULL UNIQUE COMMENT 'Código corto y único del tipo de contrato',
  nombre       VARCHAR(100) NOT NULL COMMENT 'Nombre del tipo de contrato (p.ej. "Indefinido", "Plazo fijo")',
  descripcion  VARCHAR(255) NULL COMMENT 'Detalle u observaciones adicionales sobre el tipo de contrato'
) ENGINE=InnoDB COMMENT='Catálogo de modalidades de contratación laboral';

CREATE TABLE sindicatos (
  id      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del sindicato',
  codigo  VARCHAR(20)  NOT NULL UNIQUE COMMENT 'Código corto y único del sindicato',
  nombre  VARCHAR(150) NOT NULL COMMENT 'Nombre completo del sindicato'
) ENGINE=InnoDB COMMENT='p.ej. ANEP - Asociación Nacional de Empleados Públicos';

CREATE TABLE bancos (
  id                        BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del banco',
  codigo                    VARCHAR(20)  NOT NULL UNIQUE COMMENT 'Código corto y único del banco',
  nombre                    VARCHAR(150) NOT NULL COMMENT 'Nombre comercial del banco',
  es_depositario_nomina     TINYINT(1)   NOT NULL DEFAULT 0 COMMENT '1 = el banco puede recibir depósitos de salario de los empleados',
  es_otorgante_prestamos    TINYINT(1)   NOT NULL DEFAULT 0 COMMENT '1 = el banco puede figurar como tercero otorgante de préstamos a empleados',
  activo                    TINYINT(1)   NOT NULL DEFAULT 1 COMMENT '1 = banco disponible para uso, 0 = dado de baja'
) ENGINE=InnoDB COMMENT='Catálogo de bancos, con roles de depositario de nómina y/o otorgante de préstamos';

CREATE TABLE empleados (
  id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del empleado (llave de negocio real, no la cédula)',
  codigo_interno         VARCHAR(10)  NOT NULL UNIQUE COMMENT 'p.ej. E001, E002 (patrón ya usado internamente)',
  tipo_identificacion_id BIGINT UNSIGNED NOT NULL COMMENT 'Tipo de documento de identidad del empleado (FK)',
  numero_identificacion  VARCHAR(20) NOT NULL COMMENT 'cédula (9 díg.), DIMEX (12 díg.), pasaporte o cédula de residencia',
  nombre_completo        VARCHAR(200) NOT NULL COMMENT 'Nombre completo del empleado',
  fecha_ingreso          DATE NOT NULL COMMENT 'Fecha en que el empleado ingresó a laborar (usada, entre otros, para calcular antigüedad, vacaciones y cesantía)',
  activo                 TINYINT(1)   NOT NULL DEFAULT 1 COMMENT '1 = empleado actualmente activo, 0 = inactivo/desvinculado',
  created_at             TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Fecha/hora en que se creó el registro',
  updated_at             TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Fecha/hora de la última modificación del registro',
  CONSTRAINT fk_emp_tipoid FOREIGN KEY (tipo_identificacion_id) REFERENCES tipos_identificacion(id),
  UNIQUE KEY uk_empleado_identificacion (tipo_identificacion_id, numero_identificacion)
) ENGINE=InnoDB COMMENT='Datos maestros invariantes del empleado';

CREATE TABLE empleado_datos_personales (
  id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del registro histórico',
  empleado_id            BIGINT UNSIGNED NOT NULL COMMENT 'Empleado al que pertenecen estos datos',
  direccion              VARCHAR(255) NULL COMMENT 'Dirección de residencia del empleado',
  telefono               VARCHAR(30)  NULL COMMENT 'Teléfono de contacto del empleado',
  correo_electronico     VARCHAR(150) NULL COMMENT 'Correo electrónico del empleado',
  fecha_inicio_vigencia  DATE NOT NULL COMMENT 'Fecha desde la cual estos datos son válidos',
  fecha_fin_vigencia     DATE NULL COMMENT 'NULL = vigente actualmente',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED
    COMMENT 'Columna técnica: 1 solo cuando la fila está vigente, NULL en el resto -- soporta el índice único uk_edp_una_vigente',
  CONSTRAINT fk_edp_empleado FOREIGN KEY (empleado_id) REFERENCES empleados(id),
  CONSTRAINT chk_edp_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  UNIQUE KEY uk_edp_una_vigente (empleado_id, es_vigente),
  KEY ix_edp_vigencia (empleado_id, fecha_inicio_vigencia, fecha_fin_vigencia)
) ENGINE=InnoDB COMMENT='datos personales con vigencia histórica';

CREATE TABLE empleado_datos_laborales (
  id                        BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del registro histórico',
  empleado_id               BIGINT UNSIGNED NOT NULL COMMENT 'Empleado al que pertenecen estos datos laborales',
  puesto_id                 BIGINT UNSIGNED NOT NULL COMMENT 'Puesto que ocupaba el empleado durante este período de vigencia',
  salario_base_nominal      DECIMAL(14,2) NOT NULL COMMENT 'salario mensual completo del expediente, NO el devengado del período',
  tipo_contrato_id          BIGINT UNSIGNED NOT NULL COMMENT 'Modalidad de contratación vigente en este período',
  programa_presupuestario_id BIGINT UNSIGNED NOT NULL COMMENT 'Programa presupuestario al que se cargaba el salario del empleado en este período',
  sindicato_id              BIGINT UNSIGNED NULL COMMENT 'Sindicato al que estaba afiliado el empleado en este período, si aplica',
  fecha_inicio_vigencia     DATE NOT NULL COMMENT 'Fecha desde la cual estos datos laborales son válidos',
  fecha_fin_vigencia        DATE NULL COMMENT 'NULL = vigente actualmente',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED
    COMMENT 'Columna técnica: soporta uk_edl_una_vigente',
  CONSTRAINT fk_edl_empleado FOREIGN KEY (empleado_id) REFERENCES empleados(id),
  CONSTRAINT fk_edl_puesto FOREIGN KEY (puesto_id) REFERENCES puestos(id),
  CONSTRAINT fk_edl_tipocontrato FOREIGN KEY (tipo_contrato_id) REFERENCES tipos_contrato(id),
  CONSTRAINT fk_edl_programa FOREIGN KEY (programa_presupuestario_id) REFERENCES programas_presupuestarios(id),
  CONSTRAINT fk_edl_sindicato FOREIGN KEY (sindicato_id) REFERENCES sindicatos(id),
  CONSTRAINT chk_edl_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  UNIQUE KEY uk_edl_una_vigente (empleado_id, es_vigente),
  KEY ix_edl_vigencia (empleado_id, fecha_inicio_vigencia, fecha_fin_vigencia)
) ENGINE=InnoDB COMMENT='datos laborales con vigencia histórica';

CREATE TABLE empleado_datos_bancarios (
  id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del registro histórico',
  empleado_id            BIGINT UNSIGNED NOT NULL COMMENT 'Empleado dueño de la cuenta',
  banco_id               BIGINT UNSIGNED NOT NULL COMMENT 'Banco donde está la cuenta de depósito de nómina',
  tipo_cuenta            ENUM('corriente','ahorros') NOT NULL COMMENT 'Tipo de cuenta bancaria',
  numero_cuenta          VARCHAR(40) NOT NULL COMMENT 'Número de cuenta o IBAN de depósito',
  fecha_inicio_vigencia  DATE NOT NULL COMMENT 'Fecha desde la cual esta cuenta es la vigente para depósito',
  fecha_fin_vigencia     DATE NULL COMMENT 'NULL = vigente actualmente',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED
    COMMENT 'Columna técnica: soporta uk_edb_una_vigente',
  CONSTRAINT fk_edb_empleado FOREIGN KEY (empleado_id) REFERENCES empleados(id),
  CONSTRAINT fk_edb_banco FOREIGN KEY (banco_id) REFERENCES bancos(id),
  CONSTRAINT chk_edb_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  UNIQUE KEY uk_edb_una_vigente (empleado_id, es_vigente),
  KEY ix_edb_vigencia (empleado_id, fecha_inicio_vigencia, fecha_fin_vigencia)
) ENGINE=InnoDB COMMENT='cuenta de depósito de nómina, con vigencia';

CREATE TABLE empleado_datos_familiares (
  id                        BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del registro histórico',
  empleado_id               BIGINT UNSIGNED NOT NULL COMMENT 'Empleado al que pertenece esta declaración familiar',
  aplica_credito_conyuge    TINYINT(1) NOT NULL DEFAULT 0 COMMENT '1 = el empleado declaró cónyuge y corresponde aplicar el crédito fiscal de renta por cónyuge',
  numero_hijos_dependientes INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'Cantidad de hijos declarados como dependientes para efectos del crédito fiscal de renta',
  fecha_inicio_vigencia     DATE NOT NULL COMMENT 'Fecha desde la cual esta declaración familiar es la vigente',
  fecha_fin_vigencia        DATE NULL COMMENT 'NULL = vigente actualmente',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED
    COMMENT 'Columna técnica: soporta uk_edf_una_vigente',
  CONSTRAINT fk_edf_empleado FOREIGN KEY (empleado_id) REFERENCES empleados(id),
  CONSTRAINT chk_edf_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  UNIQUE KEY uk_edf_una_vigente (empleado_id, es_vigente),
  KEY ix_edf_vigencia (empleado_id, fecha_inicio_vigencia, fecha_fin_vigencia)
) ENGINE=InnoDB COMMENT='composición familiar declarada para créditos fiscales de renta, con vigencia histórica';

CREATE TABLE regimenes_laborales (
    id INT AUTO_INCREMENT PRIMARY KEY,
    codigo VARCHAR(40) NOT NULL UNIQUE,
    nombre VARCHAR(150) NOT NULL,
    descripcion VARCHAR(255) NULL,
    activo TINYINT(1) NOT NULL DEFAULT 1
) ENGINE=InnoDB COMMENT='Catálogo de regímenes laborales aplicables (Ley Marco de Empleo Público y regímenes transitorios previos)';

CREATE TABLE contratos (
  id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del contrato / versión de contrato',
  empleado_id            BIGINT UNSIGNED NOT NULL COMMENT 'Empleado titular de este contrato',
  puesto_id              BIGINT UNSIGNED NOT NULL COMMENT 'Puesto pactado en esta versión del contrato',
  regimen_laboral_id     INT NOT NULL COMMENT 'Régimen laboral vigente para esta versión del contrato (Ley Marco de Empleo Público / regímenes transitorios); fuente de verdad del régimen',
  tipo_contrato          VARCHAR(30) NOT NULL COMMENT 'Modalidad de contratación (p.ej. indefinido, plazo_fijo, interino); string libre',
  salario_pactado        DECIMAL(14,2) NOT NULL COMMENT 'Salario mensual pactado/actualizado en esta versión del contrato',
  jornada                VARCHAR(30) NOT NULL COMMENT 'Jornada laboral pactada en el contrato (p.ej. completa, media_jornada)',
  fecha_inicio_contrato  DATE NOT NULL COMMENT 'Fecha de inicio de la relación contractual (puede ser anterior a fecha_inicio_vigencia en una renovación)',
  fecha_fin_contrato     DATE NULL COMMENT 'Fecha de finalización pactada del contrato (p.ej. plazo fijo); NULL para indefinido',
  fecha_inicio_vigencia  DATE NOT NULL COMMENT 'Fecha desde la cual esta versión del contrato es la vigente',
  fecha_fin_vigencia     DATE NULL COMMENT 'NULL = esta versión del contrato está vigente actualmente',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED
    COMMENT 'Columna técnica: 1 solo cuando la fila está vigente, NULL en el resto -- soporta uk_contrato_una_vigente',
  creado_por             BIGINT UNSIGNED NOT NULL COMMENT 'Usuario que registró esta versión del contrato (auditoría de negocio explícita, aparte de auditoria_cambios)',
  created_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Fecha/hora en que se creó este registro histórico',
  CONSTRAINT fk_contrato_empleado FOREIGN KEY (empleado_id) REFERENCES empleados(id),
  CONSTRAINT fk_contrato_puesto FOREIGN KEY (puesto_id) REFERENCES puestos(id),
  CONSTRAINT fk_contrato_regimen_laboral FOREIGN KEY (regimen_laboral_id) REFERENCES regimenes_laborales(id),
  CONSTRAINT fk_contrato_creador FOREIGN KEY (creado_por) REFERENCES usuarios(id),
  CONSTRAINT chk_contrato_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  CONSTRAINT chk_contrato_fechas CHECK (fecha_fin_contrato IS NULL OR fecha_fin_contrato >= fecha_inicio_contrato),
  UNIQUE KEY uk_contrato_una_vigente (empleado_id, es_vigente),
  KEY ix_contrato_vigencia (empleado_id, fecha_inicio_vigencia, fecha_fin_vigencia),
  KEY ix_contrato_puesto (puesto_id),
  KEY ix_contrato_tipo (tipo_contrato),
  KEY ix_contrato_regimen_laboral (regimen_laboral_id)
) ENGINE=InnoDB COMMENT='contrato laboral con vigencia histórica';

CREATE TABLE conceptos_salariales (
  id                    BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del concepto salarial',
  codigo                VARCHAR(20)  NOT NULL UNIQUE COMMENT 'Código corto y único del concepto (p.ej. SAL_BASE, CCSS_OBR)',
  nombre                VARCHAR(150) NOT NULL COMMENT 'Nombre descriptivo del concepto (p.ej. "Salario base", "Horas extra")',
  naturaleza            ENUM('ingreso','deduccion') NOT NULL COMMENT 'Si el concepto suma (ingreso) o resta (deducción) al neto a pagar',
  tipo_calculo          ENUM('fijo','formula','captura_manual') NOT NULL COMMENT 'Cómo se obtiene el monto: monto fijo configurado, fórmula del motor de cálculo, o digitado manualmente en cada corrida',
  afecta_base_ccss      TINYINT(1) NOT NULL DEFAULT 0 COMMENT '1 = el monto de este concepto forma parte de la base para calcular cargas sociales CCSS',
  afecta_base_renta     TINYINT(1) NOT NULL DEFAULT 0 COMMENT '1 = el monto de este concepto forma parte de la base para calcular impuesto de renta',
  afecta_base_aguinaldo TINYINT(1) NOT NULL DEFAULT 0 COMMENT '1 = el monto de este concepto forma parte de la base para calcular aguinaldo',
  afecta_base_cesantia  TINYINT(1) NOT NULL DEFAULT 0 COMMENT '1 = el monto de este concepto forma parte de la base para calcular cesantía',
  afecta_base_vacaciones TINYINT(1) NOT NULL DEFAULT 0 COMMENT '1 = el monto de este concepto forma parte de la base para calcular pago de vacaciones',
  activo                TINYINT(1) NOT NULL DEFAULT 1 COMMENT '1 = concepto disponible para usarse en nuevas planillas, 0 = dado de baja',
  created_at            TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Fecha/hora en que se creó el registro'
) ENGINE=InnoDB COMMENT='catálogo editable de conceptos de ingreso/deducción';

CREATE TABLE concepto_salarial_formula (
  id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno de esta versión de fórmula/monto',
  concepto_salarial_id   BIGINT UNSIGNED NOT NULL COMMENT 'Concepto salarial al que pertenece esta versión de cálculo',
  monto_fijo             DECIMAL(14,4) NULL COMMENT 'Monto fijo a aplicar, cuando el concepto es de tipo_calculo = fijo',
  expresion_formula      TEXT NULL COMMENT 'DSL/JSON interpretado por el motor de cálculo',
  fecha_inicio_vigencia  DATE NOT NULL COMMENT 'Fecha desde la cual esta fórmula/monto es la vigente',
  fecha_fin_vigencia     DATE NULL COMMENT 'NULL = vigente actualmente',
  aprobado_por           VARCHAR(150) NULL COMMENT 'Nombre/cargo de quien aprobó esta versión de la fórmula/monto',
  fecha_aprobacion       DATE NULL COMMENT 'Fecha en que se aprobó esta versión',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED
    COMMENT 'Columna técnica: soporta uk_csf_una_vigente',
  CONSTRAINT fk_csf_concepto FOREIGN KEY (concepto_salarial_id) REFERENCES conceptos_salariales(id),
  CONSTRAINT chk_csf_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  UNIQUE KEY uk_csf_una_vigente (concepto_salarial_id, es_vigente),
  KEY ix_csf_vigencia (concepto_salarial_id, fecha_inicio_vigencia, fecha_fin_vigencia)
) ENGINE=InnoDB COMMENT='Versión histórica y vigente de la fórmula o monto de cada concepto salarial';

CREATE TABLE orden_calculo_concepto (
  id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno de la regla de orden',
  concepto_salarial_id   BIGINT UNSIGNED NOT NULL COMMENT 'Concepto salarial al que se le asigna una posición en la secuencia de cálculo',
  orden                  INT NOT NULL COMMENT 'Posición numérica del concepto dentro de la secuencia de cálculo (menor = se calcula primero)',
  fecha_inicio_vigencia  DATE NOT NULL COMMENT 'Fecha desde la cual este orden es el vigente',
  fecha_fin_vigencia     DATE NULL COMMENT 'NULL = vigente actualmente',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED
    COMMENT 'Columna técnica: soporta uk_occ_una_vigente',
  CONSTRAINT fk_occ_concepto FOREIGN KEY (concepto_salarial_id) REFERENCES conceptos_salariales(id),
  CONSTRAINT chk_occ_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  UNIQUE KEY uk_occ_una_vigente (concepto_salarial_id, es_vigente),
  KEY ix_occ_vigencia (concepto_salarial_id, fecha_inicio_vigencia, fecha_fin_vigencia)
) ENGINE=InnoDB COMMENT='Secuencia de cálculo: base -> horas extra -> bases especiales -> deducciones legales -> deducciones de terceros';

CREATE TABLE regimen_concepto_aplicable (
  id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno de la regla de aplicabilidad',
  regimen_laboral_id     INT NOT NULL COMMENT 'Régimen laboral al que aplica este concepto salarial',
  concepto_salarial_id   BIGINT UNSIGNED NOT NULL COMMENT 'Concepto salarial que aplica (o dejó de aplicar) al régimen indicado',
  fecha_inicio_vigencia  DATE NOT NULL COMMENT 'Fecha desde la cual esta aplicabilidad es la vigente',
  fecha_fin_vigencia     DATE NULL COMMENT 'NULL = vigente actualmente',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED
    COMMENT 'Columna técnica: soporta uk_rca_una_vigente',
  CONSTRAINT fk_rca_regimen FOREIGN KEY (regimen_laboral_id) REFERENCES regimenes_laborales(id),
  CONSTRAINT fk_rca_concepto FOREIGN KEY (concepto_salarial_id) REFERENCES conceptos_salariales(id),
  CONSTRAINT chk_rca_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  UNIQUE KEY uk_rca_una_vigente (regimen_laboral_id, concepto_salarial_id, es_vigente),
  KEY ix_rca_vigencia (regimen_laboral_id, concepto_salarial_id, fecha_inicio_vigencia, fecha_fin_vigencia)
) ENGINE=InnoDB COMMENT='Cruce régimen laboral <-> conceptos salariales aplicables, versionado por vigencia';

CREATE TABLE parametros_cargas_sociales (
  id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del parámetro',
  rubro                  VARCHAR(60) NOT NULL COMMENT 'Etiqueta legible: SEM, IVM, Banco Popular, Asignaciones Familiares, FCL, ROP -- NO usar para lógica de cálculo',
  concepto_salarial_id   BIGINT UNSIGNED NOT NULL COMMENT 'Concepto salarial real al que corresponde este rubro (FK)',
  porcentaje_patronal    DECIMAL(6,4) NOT NULL COMMENT 'Porcentaje que aporta el patrono sobre la base correspondiente',
  porcentaje_obrero      DECIMAL(6,4) NOT NULL COMMENT 'Porcentaje que se deduce al empleado sobre la base correspondiente',
  fecha_inicio_vigencia  DATE NOT NULL COMMENT 'Fecha desde la cual estos porcentajes son los vigentes',
  fecha_fin_vigencia     DATE NULL COMMENT 'NULL = vigente actualmente',
  fuente_legal           VARCHAR(150) NULL COMMENT 'p.ej. Ley Orgánica CCSS, Ley 7983',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED
    COMMENT 'Columna técnica: soporta uk_pcs_una_vigente. Nota: un solo registro vigente por concepto_salarial_id es correcto porque patronal/obrero ya conviven en una misma fila; no se necesita columna tipo_aporte.',
  CONSTRAINT fk_pcs_concepto FOREIGN KEY (concepto_salarial_id) REFERENCES conceptos_salariales(id),
  CONSTRAINT chk_pcs_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  UNIQUE KEY uk_pcs_una_vigente (concepto_salarial_id, es_vigente),
  KEY ix_pcs_vigencia (concepto_salarial_id, fecha_inicio_vigencia, fecha_fin_vigencia)
) ENGINE=InnoDB COMMENT='Porcentajes patronales/obreros de cargas sociales, versionados por vigencia';

CREATE TABLE parametros_tramos_renta (
  id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del tramo',
  orden_tramo            INT NOT NULL COMMENT 'Posición del tramo dentro de la escala progresiva (1 = primer tramo, el más bajo)',
  monto_desde            DECIMAL(14,2) NOT NULL COMMENT 'Monto de salario desde el cual aplica este tramo',
  monto_hasta            DECIMAL(14,2) NULL COMMENT 'NULL = sin tope superior',
  porcentaje             DECIMAL(5,2) NOT NULL COMMENT 'Porcentaje de impuesto de renta aplicable a este tramo',
  fecha_inicio_vigencia  DATE NOT NULL COMMENT 'Fecha desde la cual este tramo es el vigente',
  fecha_fin_vigencia     DATE NULL COMMENT 'NULL = vigente actualmente',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED
    COMMENT 'Columna técnica: soporta uk_ptr_una_vigente',
  CONSTRAINT chk_ptr_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  CONSTRAINT chk_ptr_rango CHECK (monto_hasta IS NULL OR monto_hasta > monto_desde),
  CONSTRAINT chk_ptr_porcentaje_rango CHECK (porcentaje >= 0 AND porcentaje <= 100),
  UNIQUE KEY uk_ptr_una_vigente (orden_tramo, es_vigente),
  KEY ix_ptr_vigencia (orden_tramo, fecha_inicio_vigencia, fecha_fin_vigencia)
) ENGINE=InnoDB COMMENT='Escala progresiva de impuesto de renta, versionada por vigencia';

CREATE TABLE parametros_creditos_fiscales_renta (
  id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del crédito fiscal',
  tipo_credito_fiscal_id BIGINT UNSIGNED NOT NULL COMMENT 'A qué tipo de dependiente aplica el crédito (FK)',
  monto_credito          DECIMAL(14,2) NOT NULL COMMENT 'Monto fijo a deducir del impuesto de renta ya calculado, por período de planilla',
  fecha_inicio_vigencia  DATE NOT NULL COMMENT 'Fecha desde la cual este monto es el vigente',
  fecha_fin_vigencia     DATE NULL COMMENT 'NULL = vigente actualmente',
  fuente_legal           VARCHAR(150) NULL COMMENT 'p.ej. Art. 33 Ley 7092',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED
    COMMENT 'Columna técnica: soporta uk_pcfr_una_vigente',
  CONSTRAINT fk_pcfr_tipo FOREIGN KEY (tipo_credito_fiscal_id) REFERENCES tipos_credito_fiscal_renta(id),
  CONSTRAINT chk_pcfr_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  UNIQUE KEY uk_pcfr_una_vigente (tipo_credito_fiscal_id, es_vigente),
  KEY ix_pcfr_vigencia (tipo_credito_fiscal_id, fecha_inicio_vigencia, fecha_fin_vigencia)
) ENGINE=InnoDB COMMENT='Créditos fiscales de renta por cónyuge/hijo dependiente, versionados por vigencia';

CREATE TABLE parametros_tipos_embargo (
  id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del tipo de embargo',
  codigo                 VARCHAR(20) NOT NULL COMMENT 'Código corto del tipo de embargo (ya NO es UNIQUE por sí solo: ver uk_pte_una_vigente)',
  nombre                 VARCHAR(100) NOT NULL COMMENT 'Nombre del tipo de embargo (p.ej. "Pensión alimentaria", "Embargo judicial")',
  prioridad_legal        INT NOT NULL COMMENT 'menor número = mayor prioridad; 1 = pensión alimentaria',
  fecha_inicio_vigencia  DATE NOT NULL COMMENT 'Fecha desde la cual esta prioridad legal es la vigente',
  fecha_fin_vigencia     DATE NULL COMMENT 'NULL = vigente actualmente',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED
    COMMENT 'Columna técnica: soporta uk_pte_una_vigente',
  CONSTRAINT chk_pte_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  UNIQUE KEY uk_pte_una_vigente (codigo, es_vigente),
  KEY ix_pte_vigencia (codigo, fecha_inicio_vigencia, fecha_fin_vigencia)
) ENGINE=InnoDB COMMENT='Catálogo de tipos de embargo y su prioridad legal de aplicación, versionado por vigencia';

CREATE TABLE parametros_horas_extra (
  id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del parámetro',
  tipo_hora_extra_id     BIGINT UNSIGNED NOT NULL COMMENT 'Categoría de hora extra a la que aplica el factor (FK)',
  factor_recargo         DECIMAL(5,4) NOT NULL COMMENT 'p.ej. 1.50 = 50% de recargo',
  fecha_inicio_vigencia  DATE NOT NULL COMMENT 'Fecha desde la cual este factor es el vigente',
  fecha_fin_vigencia     DATE NULL COMMENT 'NULL = vigente actualmente',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED
    COMMENT 'Columna técnica: soporta uk_phe_una_vigente',
  CONSTRAINT fk_phe_tipo FOREIGN KEY (tipo_hora_extra_id) REFERENCES tipos_hora_extra(id),
  CONSTRAINT chk_phe_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  UNIQUE KEY uk_phe_una_vigente (tipo_hora_extra_id, es_vigente),
  KEY ix_phe_vigencia (tipo_hora_extra_id, fecha_inicio_vigencia, fecha_fin_vigencia)
) ENGINE=InnoDB COMMENT='Factores de recargo por tipo de hora extra, versionados por vigencia';

CREATE TABLE parametros_vacaciones_escala (
  id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del escalón',
  anios_servicio_desde   DECIMAL(5,2) NOT NULL COMMENT 'Años de servicio desde los cuales aplica este escalón',
  anios_servicio_hasta   DECIMAL(5,2) NULL COMMENT 'Años de servicio hasta los cuales aplica; NULL = sin tope superior',
  dias_vacaciones        DECIMAL(6,2) NOT NULL COMMENT 'Días de vacaciones que corresponden por año dentro de este escalón',
  fecha_inicio_vigencia  DATE NOT NULL COMMENT 'Fecha desde la cual este escalón es el vigente',
  fecha_fin_vigencia     DATE NULL COMMENT 'NULL = vigente actualmente',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED
    COMMENT 'Columna técnica: soporta uk_pve_una_vigente',
  CONSTRAINT chk_pve_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  CONSTRAINT chk_pve_rango_anios CHECK (anios_servicio_hasta IS NULL OR anios_servicio_hasta > anios_servicio_desde),
  UNIQUE KEY uk_pve_una_vigente (anios_servicio_desde, es_vigente),
  KEY ix_pve_vigencia (anios_servicio_desde, fecha_inicio_vigencia, fecha_fin_vigencia)
) ENGINE=InnoDB COMMENT='Escala de días de vacaciones según años de servicio, versionada por vigencia';

CREATE TABLE parametros_cesantia_escala (
  id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del escalón',
  regimen_laboral_id     INT NOT NULL COMMENT 'Régimen laboral al que aplica este escalón de cesantía',
  anios_servicio_desde   DECIMAL(5,2) NOT NULL COMMENT 'Años de servicio desde los cuales aplica este escalón',
  anios_servicio_hasta   DECIMAL(5,2) NULL COMMENT 'Años de servicio hasta los cuales aplica; NULL = sin tope superior',
  dias_por_anio          DECIMAL(6,2) NOT NULL COMMENT 'Días de cesantía que se reconocen por cada año de servicio dentro de este escalón',
  tope_anios             DECIMAL(5,2) NULL COMMENT 'Tope máximo de años de servicio que se reconocen para el cálculo de cesantía, si aplica',
  fecha_inicio_vigencia  DATE NOT NULL COMMENT 'Fecha desde la cual este escalón es el vigente',
  fecha_fin_vigencia     DATE NULL COMMENT 'NULL = vigente actualmente',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED
    COMMENT 'Columna técnica: soporta uk_pce_una_vigente',
  CONSTRAINT fk_pce_regimen FOREIGN KEY (regimen_laboral_id) REFERENCES regimenes_laborales(id),
  CONSTRAINT chk_pce_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  CONSTRAINT chk_pce_rango_anios CHECK (anios_servicio_hasta IS NULL OR anios_servicio_hasta > anios_servicio_desde),
  UNIQUE KEY uk_pce_una_vigente (regimen_laboral_id, anios_servicio_desde, es_vigente),
  KEY ix_pce_vigencia (regimen_laboral_id, anios_servicio_desde, fecha_inicio_vigencia, fecha_fin_vigencia)
) ENGINE=InnoDB COMMENT='Escala de cesantía según años de servicio, por régimen laboral, versionada por vigencia';

CREATE TABLE parametros_preaviso_escala (
  id                       INT AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del escalón',
  regimen_laboral_id       INT NOT NULL COMMENT 'Régimen laboral al que aplica este escalón de preaviso',
  antiguedad_meses_desde   INT NOT NULL COMMENT 'Antigüedad en meses desde la cual aplica este escalón',
  antiguedad_meses_hasta   INT NULL COMMENT 'Antigüedad en meses hasta la cual aplica; NULL = último tramo, sin tope superior',
  dias_preaviso            INT NOT NULL COMMENT 'Días de preaviso que corresponden dentro de este escalón',
  fecha_inicio_vigencia    DATE NOT NULL COMMENT 'Fecha desde la cual este escalón es el vigente',
  fecha_fin_vigencia       DATE NULL COMMENT 'NULL = vigente actualmente',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED
    COMMENT 'Columna técnica: soporta uk_ppe_una_vigente',
  creado_por               INT NOT NULL COMMENT 'Usuario que registró este escalón (patrón copiado de parametros_salario_escolar)',
  created_at               DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Fecha/hora en que se creó el registro',
  CONSTRAINT fk_ppe_regimen FOREIGN KEY (regimen_laboral_id) REFERENCES regimenes_laborales(id),
  CONSTRAINT chk_ppe_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  CONSTRAINT chk_ppe_rango_meses CHECK (antiguedad_meses_hasta IS NULL OR antiguedad_meses_hasta > antiguedad_meses_desde),
  UNIQUE KEY uk_ppe_una_vigente (regimen_laboral_id, antiguedad_meses_desde, es_vigente),
  KEY ix_ppe_vigencia (regimen_laboral_id, fecha_inicio_vigencia, fecha_fin_vigencia)
) ENGINE=InnoDB COMMENT='Escala de días de preaviso según antigüedad (Art. 28 Código de Trabajo), por régimen laboral, versionada por vigencia';

CREATE TABLE parametros_base_minima_contributiva (
  id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del parámetro',
  regimen_ccss_id        BIGINT UNSIGNED NOT NULL COMMENT 'Régimen de la CCSS al que aplica esta base mínima (FK)',
  monto                  DECIMAL(14,2) NOT NULL COMMENT 'Monto mínimo contributivo vigente para el régimen indicado',
  fecha_inicio_vigencia  DATE NOT NULL COMMENT 'Fecha desde la cual este monto es el vigente',
  fecha_fin_vigencia     DATE NULL COMMENT 'NULL = vigente actualmente',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED
    COMMENT 'Columna técnica: soporta uk_bmc_una_vigente',
  CONSTRAINT fk_bmc_regimen FOREIGN KEY (regimen_ccss_id) REFERENCES regimenes_ccss(id),
  CONSTRAINT chk_bmc_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  UNIQUE KEY uk_bmc_una_vigente (regimen_ccss_id, es_vigente),
  KEY ix_bmc_vigencia (regimen_ccss_id, fecha_inicio_vigencia, fecha_fin_vigencia)
) ENGINE=InnoDB COMMENT='Base mínima contributiva SEM/IVM, versionada por vigencia';

CREATE TABLE parametros_subsidios_incapacidad (
  id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del tramo de subsidio',
  entidad_emisora_id     BIGINT UNSIGNED NOT NULL COMMENT 'Entidad que dictamina la incapacidad (FK)',
  dia_desde              INT UNSIGNED NOT NULL COMMENT 'Día de incapacidad desde el cual aplica este porcentaje (p.ej. 1 = primer día de incapacidad)',
  dia_hasta              INT UNSIGNED NULL COMMENT 'Día de incapacidad hasta el cual aplica este porcentaje; NULL = sin tope superior',
  porcentaje_patronal    DECIMAL(5,2) NOT NULL COMMENT 'Porcentaje del salario que reconoce la institución como patrono durante este tramo',
  porcentaje_entidad     DECIMAL(5,2) NULL COMMENT 'Porcentaje del salario que reconoce la CCSS/INS durante este tramo, como referencia informativa',
  fecha_inicio_vigencia  DATE NOT NULL COMMENT 'Fecha desde la cual esta escala es la vigente',
  fecha_fin_vigencia     DATE NULL COMMENT 'NULL = vigente actualmente',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED
    COMMENT 'Columna técnica: soporta uk_psi_una_vigente',
  CONSTRAINT fk_psi_entidad FOREIGN KEY (entidad_emisora_id) REFERENCES entidades_emisoras_incapacidad(id),
  CONSTRAINT chk_psi_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  CONSTRAINT chk_psi_dias CHECK (dia_hasta IS NULL OR dia_hasta >= dia_desde),
  UNIQUE KEY uk_psi_una_vigente (entidad_emisora_id, dia_desde, es_vigente),
  KEY ix_psi_vigencia (entidad_emisora_id, fecha_inicio_vigencia, fecha_fin_vigencia)
) ENGINE=InnoDB COMMENT='Escala de subsidio patronal/CCSS/INS por día de incapacidad, versionada por vigencia';

CREATE TABLE parametros_salario_minimo (
  id                       BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del parámetro',
  clasificacion_puesto_id  BIGINT UNSIGNED NOT NULL COMMENT 'Categoría del Decreto de Salarios Mínimos (FK a clasificaciones_puesto, no texto libre)',
  monto_mensual            DECIMAL(14,2) NOT NULL COMMENT 'Salario mínimo mensual legal para esta categoría',
  fecha_inicio_vigencia    DATE NOT NULL COMMENT 'Fecha desde la cual este monto es el vigente (normalmente semestral)',
  fecha_fin_vigencia       DATE NULL COMMENT 'NULL = vigente actualmente',
  fuente_legal             VARCHAR(150) NULL COMMENT 'p.ej. Decreto de Salarios Mínimos, Consejo Nacional de Salarios',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED
    COMMENT 'Columna técnica: soporta uk_psm_una_vigente',
  CONSTRAINT fk_psm_clasificacion FOREIGN KEY (clasificacion_puesto_id) REFERENCES clasificaciones_puesto(id),
  CONSTRAINT chk_psm_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  UNIQUE KEY uk_psm_una_vigente (clasificacion_puesto_id, es_vigente),
  KEY ix_psm_vigencia (clasificacion_puesto_id, fecha_inicio_vigencia, fecha_fin_vigencia)
) ENGINE=InnoDB COMMENT='Salario mínimo legal por categoría/clasificación de puesto, versionado por vigencia (Ley 10159 / Decreto de Salarios Mínimos)';

CREATE TABLE calendario_laboral (
  id                          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del día calendario',
  fecha                       DATE NOT NULL COMMENT 'Fecha calendario que se está clasificando',
  tipo_dia                    ENUM('laboral','feriado','descanso') NOT NULL COMMENT 'Clasificación del día para efectos de cálculo de asistencia/planilla',
  programa_presupuestario_id  BIGINT UNSIGNED NULL COMMENT 'Programa presupuestario al que aplica este calendario, si los calendarios difieren por programa; NULL = aplica a todos',
  UNIQUE KEY uk_calendario (fecha, programa_presupuestario_id),
  CONSTRAINT fk_cal_programa FOREIGN KEY (programa_presupuestario_id) REFERENCES programas_presupuestarios(id)
) ENGINE=InnoDB COMMENT='Clasificación de cada fecha como día laboral, feriado o de descanso';

CREATE TABLE feriados_patron (
  id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del patrón de feriado',
  nombre                 VARCHAR(100) NOT NULL COMMENT 'p.ej. "Día de Juan Santamaría", "Semana Santa - Jueves Santo"',
  tipo_patron            ENUM('fijo','movible_pascua') NOT NULL DEFAULT 'fijo'
    COMMENT 'fijo = mismo mes/día cada año; movible_pascua = se calcula por offset de días respecto al Domingo de Pascua',
  mes                    TINYINT UNSIGNED NULL COMMENT 'Mes (1-12), solo cuando tipo_patron = fijo',
  dia                    TINYINT UNSIGNED NULL COMMENT 'Día del mes, solo cuando tipo_patron = fijo',
  offset_dias_pascua     SMALLINT NULL COMMENT 'Días de diferencia respecto al Domingo de Pascua (puede ser negativo), solo cuando tipo_patron = movible_pascua',
  aplica_traslado_lunes  TINYINT(1) NOT NULL DEFAULT 0
    COMMENT '1 = si cae entre semana se traslada al lunes según Ley de Traslado de Feriados, cuando aplique',
  pago_obligatorio       TINYINT(1) NOT NULL DEFAULT 1
    COMMENT '1 = feriado de pago obligatorio (Art. 148 Código de Trabajo), 0 = feriado no obligatorio/opcional',
  activo                 TINYINT(1) NOT NULL DEFAULT 1 COMMENT '1 = patrón utilizable para generar calendario, 0 = descontinuado',
  CONSTRAINT chk_fp_patron CHECK (
    (tipo_patron = 'fijo' AND mes IS NOT NULL AND dia IS NOT NULL AND offset_dias_pascua IS NULL) OR
    (tipo_patron = 'movible_pascua' AND offset_dias_pascua IS NOT NULL AND mes IS NULL AND dia IS NULL)
  )
) ENGINE=InnoDB COMMENT='Patrón recurrente de feriados nacionales, para generar calendario_laboral automáticamente cada año';

INSERT INTO feriados_patron (nombre, tipo_patron, mes, dia, aplica_traslado_lunes) VALUES
  ('Año Nuevo','fijo',1,1,0),
  ('Día de Juan Santamaría','fijo',4,11,1),
  ('Día del Trabajo','fijo',5,1,0),
  ('Anexión del Partido de Nicoya','fijo',7,25,0),
  ('Día de la Madre','fijo',8,15,0),
  ('Independencia','fijo',9,15,0),
  ('Navidad','fijo',12,25,0);

INSERT INTO feriados_patron (nombre, tipo_patron, offset_dias_pascua) VALUES
  ('Jueves Santo','movible_pascua',-3),
  ('Viernes Santo','movible_pascua',-2);

CREATE TABLE turnos (
  id           BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del turno',
  codigo       VARCHAR(20)  NOT NULL UNIQUE COMMENT 'Código corto y único del turno',
  nombre       VARCHAR(100) NOT NULL COMMENT 'Nombre descriptivo del turno (p.ej. "Turno diurno")',
  hora_inicio  TIME NOT NULL COMMENT 'Hora de inicio de la jornada de este turno',
  hora_fin     TIME NOT NULL COMMENT 'Hora de finalización de la jornada de este turno'
) ENGINE=InnoDB COMMENT='Catálogo de turnos/horarios de trabajo';

CREATE TABLE empleado_turno (
  id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno de la asignación',
  empleado_id            BIGINT UNSIGNED NOT NULL COMMENT 'Empleado al que se le asigna el turno',
  turno_id               BIGINT UNSIGNED NOT NULL COMMENT 'Turno asignado al empleado',
  fecha_inicio_vigencia  DATE NOT NULL COMMENT 'Fecha desde la cual este turno es el vigente para el empleado',
  fecha_fin_vigencia     DATE NULL COMMENT 'NULL = vigente actualmente',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED
    COMMENT 'Columna técnica: soporta uk_et_una_vigente',
  CONSTRAINT fk_et_empleado FOREIGN KEY (empleado_id) REFERENCES empleados(id),
  CONSTRAINT fk_et_turno FOREIGN KEY (turno_id) REFERENCES turnos(id),
  CONSTRAINT chk_et_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  UNIQUE KEY uk_et_una_vigente (empleado_id, es_vigente),
  KEY ix_et_vigencia (empleado_id, fecha_inicio_vigencia, fecha_fin_vigencia)
) ENGINE=InnoDB COMMENT='Historial de turnos asignados a cada empleado, con vigencia';

CREATE TABLE registro_asistencia (
  id                      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del registro de asistencia',
  empleado_id             BIGINT UNSIGNED NOT NULL COMMENT 'Empleado al que corresponde el registro',
  fecha                   DATE NOT NULL COMMENT 'Fecha del registro de asistencia',
  hora_entrada            TIME NULL COMMENT 'Hora en que el empleado marcó entrada',
  hora_salida             TIME NULL COMMENT 'Hora en que el empleado marcó salida',
  horas_ordinarias        DECIMAL(6,2) NOT NULL DEFAULT 0 COMMENT 'Cantidad de horas ordinarias trabajadas ese día',
  horas_extra_diurnas     DECIMAL(6,2) NOT NULL DEFAULT 0 COMMENT 'Cantidad de horas extra diurnas trabajadas ese día',
  horas_extra_nocturnas   DECIMAL(6,2) NOT NULL DEFAULT 0 COMMENT 'Cantidad de horas extra nocturnas trabajadas ese día',
  horas_extra_mixtas      DECIMAL(6,2) NOT NULL DEFAULT 0 COMMENT 'Cantidad de horas extra mixtas trabajadas ese día',
  horas_dobles            DECIMAL(6,2) NOT NULL DEFAULT 0 COMMENT 'Cantidad de horas dobles trabajadas ese día',
  observaciones           VARCHAR(255) NULL COMMENT 'Notas adicionales sobre el registro de asistencia de ese día',
  UNIQUE KEY uk_asistencia (empleado_id, fecha),
  CONSTRAINT fk_ra_empleado FOREIGN KEY (empleado_id) REFERENCES empleados(id)
) ENGINE=InnoDB COMMENT='Fuente de las horas extra diurna/nocturna/mixta y horas dobles';

CREATE TABLE tipos_ausencia (
  id                          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del tipo de ausencia',
  codigo                      VARCHAR(20) NOT NULL UNIQUE COMMENT 'Código corto y único del tipo de ausencia',
  nombre                      VARCHAR(100) NOT NULL COMMENT 'incapacidad CCSS, incapacidad INS, injustificada, permiso c/s goce, licencia, día libre, vacaciones',
  requiere_documento_soporte  TINYINT(1) NOT NULL DEFAULT 0 COMMENT '1 = este tipo de ausencia exige adjuntar un documento de respaldo (p.ej. boleta de incapacidad)'
) ENGINE=InnoDB COMMENT='Catálogo de tipos/motivos de ausencia laboral';

CREATE TABLE ausencias (
  id                      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno de la ausencia',
  empleado_id             BIGINT UNSIGNED NOT NULL COMMENT 'Empleado que se ausenta',
  tipo_ausencia_id        BIGINT UNSIGNED NOT NULL COMMENT 'Motivo de la ausencia',
  entidad_emisora_id      BIGINT UNSIGNED NULL COMMENT 'Entidad que emitió la incapacidad (FK), cuando tipo_ausencia_id corresponde a una incapacidad; determina qué escala de parametros_subsidios_incapacidad usar. NULL para ausencias que no son incapacidad',
  fecha_inicio            DATE NOT NULL COMMENT 'Fecha en que inicia la ausencia',
  fecha_fin               DATE NOT NULL COMMENT 'Fecha en que finaliza la ausencia',
  dias                    DECIMAL(6,2) NULL COMMENT 'Cantidad de días que abarca la ausencia, cuando se mide en días',
  horas                   DECIMAL(6,2) NULL COMMENT 'Cantidad de horas que abarca la ausencia, cuando se mide en horas',
  documento_soporte_url   VARCHAR(255) NULL COMMENT 'Enlace/ruta al documento de respaldo de la ausencia, si se adjuntó',
  estado                  ENUM('registrada','aprobada','rechazada') NOT NULL DEFAULT 'registrada' COMMENT 'Estado del trámite de la ausencia',
  CONSTRAINT fk_aus_empleado FOREIGN KEY (empleado_id) REFERENCES empleados(id),
  CONSTRAINT fk_aus_tipo FOREIGN KEY (tipo_ausencia_id) REFERENCES tipos_ausencia(id),
  CONSTRAINT fk_aus_entidad FOREIGN KEY (entidad_emisora_id) REFERENCES entidades_emisoras_incapacidad(id),
  CONSTRAINT chk_aus_fechas CHECK (fecha_fin >= fecha_inicio)
) ENGINE=InnoDB COMMENT='Base del "auxilio de incapacidad" y ausencias en días/horas';

CREATE TABLE tipos_planilla (
  id      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del tipo de planilla',
  codigo  VARCHAR(20)  NOT NULL UNIQUE COMMENT 'mensual, quincenal, extraordinaria, aguinaldo, liquidacion',
  nombre  VARCHAR(100) NOT NULL COMMENT 'Nombre descriptivo del tipo de planilla'
) ENGINE=InnoDB COMMENT='Catálogo de tipos/modalidades de planilla';

CREATE TABLE periodos_planilla (
  id                        BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del período de planilla',
  tipo_planilla_id          BIGINT UNSIGNED NOT NULL COMMENT 'Modalidad de planilla que se está corriendo en este período',
  fecha_inicio              DATE NOT NULL COMMENT 'p.ej. quincenas: 1-15, 16-fin de mes',
  fecha_fin                 DATE NOT NULL COMMENT 'Fecha final del período que cubre esta corrida',
  estado                    ENUM('borrador','calculada','aprobada','pagada','contabilizada') NOT NULL DEFAULT 'borrador' COMMENT 'Etapa actual del ciclo de vida de la corrida de planilla',
  fecha_generacion          DATETIME NULL COMMENT 'Fecha/hora en que se generó/calculó la planilla',
  preparado_por_usuario_id  BIGINT UNSIGNED NULL COMMENT 'Usuario que preparó/"HECHO POR" la planilla',
  fecha_preparacion         DATETIME NULL COMMENT 'Fecha/hora en que se preparó la planilla',
  revisado_por_usuario_id   BIGINT UNSIGNED NULL COMMENT 'Usuario que revisó/"REVISADO POR" la planilla',
  fecha_revision            DATETIME NULL COMMENT 'Fecha/hora en que se revisó la planilla',
  fecha_aprobacion          DATETIME NULL COMMENT 'Fecha/hora en que la planilla fue aprobada',
  aprobado_por              VARCHAR(150) NULL COMMENT 'Nombre/cargo de quien aprobó la planilla, en texto libre (respaldo cuando no hay usuario identificado)',
  aprobado_por_usuario_id   BIGINT UNSIGNED NULL COMMENT 'Usuario que aprobó la planilla (FK real, preferir sobre aprobado_por)',
  CONSTRAINT fk_pp_tipo FOREIGN KEY (tipo_planilla_id) REFERENCES tipos_planilla(id),
  CONSTRAINT fk_pp_preparador FOREIGN KEY (preparado_por_usuario_id) REFERENCES usuarios(id),
  CONSTRAINT fk_pp_revisor FOREIGN KEY (revisado_por_usuario_id) REFERENCES usuarios(id),
  CONSTRAINT fk_pp_aprobador FOREIGN KEY (aprobado_por_usuario_id) REFERENCES usuarios(id),
  CONSTRAINT chk_pp_fechas CHECK (fecha_fin >= fecha_inicio),
  UNIQUE KEY uk_pp_tipo_fechas (tipo_planilla_id, fecha_inicio, fecha_fin),
  KEY ix_pp_fechas (fecha_inicio, fecha_fin)
) ENGINE=InnoDB COMMENT='Una vez aprobada/pagada/contabilizada es inmutable';

CREATE TABLE planilla_detalle_empleado (
  id                          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno de la línea de planilla del empleado',
  periodo_planilla_id         BIGINT UNSIGNED NOT NULL COMMENT 'Corrida de planilla a la que pertenece esta línea',
  empleado_id                 BIGINT UNSIGNED NOT NULL COMMENT 'Empleado al que corresponde esta línea',
  programa_presupuestario_id  BIGINT UNSIGNED NOT NULL COMMENT 'copiado del expediente al momento del cálculo, para no alterar histórico si el empleado cambia luego de programa',
  salario_base_periodo        DECIMAL(14,2) NOT NULL COMMENT 'devengado del período, proporcional a días trabajados, distinto del salario nominal mensual',
  total_ingresos              DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Suma de todos los conceptos de ingreso del período para este empleado',
  total_deducciones           DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Suma de todos los conceptos de deducción del período para este empleado',
  neto_pagado                 DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Monto neto efectivamente pagado (total_ingresos - total_deducciones)',
  estado                      ENUM('calculado','aprobado','pagado') NOT NULL DEFAULT 'calculado' COMMENT 'Estado de esta línea dentro del ciclo de vida de la corrida',
  UNIQUE KEY uk_pde (periodo_planilla_id, empleado_id),
  CONSTRAINT fk_pde_periodo FOREIGN KEY (periodo_planilla_id) REFERENCES periodos_planilla(id),
  CONSTRAINT fk_pde_empleado FOREIGN KEY (empleado_id) REFERENCES empleados(id),
  CONSTRAINT fk_pde_programa FOREIGN KEY (programa_presupuestario_id) REFERENCES programas_presupuestarios(id)
) ENGINE=InnoDB COMMENT='Permite generar tanto el formato resumido como el detallado';

CREATE TABLE planilla_detalle_concepto (
  id                            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno de la línea de concepto',
  planilla_detalle_empleado_id  BIGINT UNSIGNED NOT NULL COMMENT 'Línea de resumen del empleado a la que pertenece este concepto',
  concepto_salarial_id          BIGINT UNSIGNED NOT NULL COMMENT 'Concepto salarial que se está aplicando en esta línea',
  concepto_salarial_formula_id  BIGINT UNSIGNED NULL COMMENT 'versión exacta de la fórmula/monto usada, para trazabilidad total',
  cantidad                      DECIMAL(10,2) NULL COMMENT 'horas o días, cuando aplique',
  monto                         DECIMAL(14,2) NOT NULL COMMENT 'Monto final calculado para este concepto en esta línea',
  concepto_ajuste_de_id         BIGINT UNSIGNED NULL COMMENT 'Si esta línea es una corrección, apunta a la línea original que corrige; NULL si es una línea normal',
  motivo_ajuste                 VARCHAR(255) NULL COMMENT 'Explicación breve de por qué se generó este ajuste',
  CONSTRAINT fk_pdc_detalle FOREIGN KEY (planilla_detalle_empleado_id) REFERENCES planilla_detalle_empleado(id),
  CONSTRAINT fk_pdc_concepto FOREIGN KEY (concepto_salarial_id) REFERENCES conceptos_salariales(id),
  CONSTRAINT fk_pdc_formula FOREIGN KEY (concepto_salarial_formula_id) REFERENCES concepto_salarial_formula(id),
  CONSTRAINT fk_pdc_ajuste FOREIGN KEY (concepto_ajuste_de_id) REFERENCES planilla_detalle_concepto(id),
  KEY ix_pdc_concepto (concepto_salarial_id)
) ENGINE=InnoDB COMMENT='Línea por concepto del cálculo de cada empleado';

DROP TRIGGER IF EXISTS trg_pdc_no_autoajuste_ins;
CREATE TRIGGER trg_pdc_no_autoajuste_ins BEFORE INSERT ON planilla_detalle_concepto
FOR EACH ROW
BEGIN
  IF NEW.concepto_ajuste_de_id IS NOT NULL AND NEW.concepto_ajuste_de_id = NEW.id THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'concepto_ajuste_de_id no puede apuntar a la misma fila (autoajuste)';
  END IF;
END;
DROP TRIGGER IF EXISTS trg_pdc_no_autoajuste_upd;
CREATE TRIGGER trg_pdc_no_autoajuste_upd BEFORE UPDATE ON planilla_detalle_concepto
FOR EACH ROW
BEGIN
  IF NEW.concepto_ajuste_de_id IS NOT NULL AND NEW.concepto_ajuste_de_id = NEW.id THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'concepto_ajuste_de_id no puede apuntar a la misma fila (autoajuste)';
  END IF;
END;

CREATE TABLE vacaciones_acumulado (
  id                  BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del corte de vacaciones',
  empleado_id         BIGINT UNSIGNED NOT NULL COMMENT 'Empleado al que corresponde el corte',
  fecha_corte         DATE NOT NULL COMMENT 'Fecha en la que se calculó este corte de vacaciones',
  dias_acumulados     DECIMAL(6,2) NOT NULL DEFAULT 0 COMMENT 'Días de vacaciones acumulados por el empleado hasta la fecha de corte',
  dias_disfrutados    DECIMAL(6,2) NOT NULL DEFAULT 0 COMMENT 'Días de vacaciones ya disfrutados por el empleado hasta la fecha de corte',
  saldo               DECIMAL(6,2) NOT NULL DEFAULT 0 COMMENT 'Saldo de días de vacaciones pendientes (acumulados - disfrutados)',
  origen_carga             ENUM('operacion','migracion_inicial') NOT NULL DEFAULT 'operacion' COMMENT 'operacion = generado por el sistema en marcha; migracion_inicial = saldo digitado a mano al arrancar el sistema, sin historial verificable detrás',
  registrado_por_usuario_id BIGINT UNSIGNED NULL COMMENT 'Usuario que registró este corte; obligatorio en la práctica cuando origen_carga = migracion_inicial, para poder rastrear quién respondió por el saldo inicial',
  CONSTRAINT fk_va_empleado FOREIGN KEY (empleado_id) REFERENCES empleados(id),
  CONSTRAINT fk_va_usuario FOREIGN KEY (registrado_por_usuario_id) REFERENCES usuarios(id),
  UNIQUE KEY uk_va (empleado_id, fecha_corte)
) ENGINE=InnoDB COMMENT='Cortes históricos de saldo de vacaciones por empleado';

CREATE TABLE vacaciones_disfrute (
  id             BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del disfrute',
  empleado_id    BIGINT UNSIGNED NOT NULL COMMENT 'Empleado que disfrutó las vacaciones',
  fecha_inicio   DATE NOT NULL COMMENT 'Fecha en que inició el disfrute de vacaciones',
  fecha_fin      DATE NOT NULL COMMENT 'Fecha en que finalizó el disfrute de vacaciones',
  dias           DECIMAL(6,2) NOT NULL COMMENT 'Cantidad de días de vacaciones disfrutados en este período',
  CONSTRAINT fk_vd_empleado FOREIGN KEY (empleado_id) REFERENCES empleados(id),
  CONSTRAINT chk_vd_fechas CHECK (fecha_fin >= fecha_inicio)
) ENGINE=InnoDB COMMENT='Historial de períodos de vacaciones disfrutados por empleado';

CREATE TABLE terceros (
  id      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del tercero',
  nombre  VARCHAR(150) NOT NULL COMMENT 'Nombre del tercero (juzgado, entidad financiera u otro)',
  tipo    ENUM('juzgado','entidad_financiera','otro') NOT NULL COMMENT 'Categoría del tercero'
) ENGINE=InnoDB COMMENT='Catálogo de terceros externos (juzgados, entidades financieras, otros)';

CREATE TABLE prestamos (
  id                  BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del préstamo',
  empleado_id         BIGINT UNSIGNED NOT NULL COMMENT 'Empleado beneficiario del préstamo',
  tercero_id          BIGINT UNSIGNED NOT NULL COMMENT 'p.ej. Banco Popular (préstamo, no la cuota de ley -- sección 4.1)',
  monto_total         DECIMAL(14,2) NOT NULL COMMENT 'Monto total original del préstamo',
  cuota_periodica     DECIMAL(14,2) NOT NULL COMMENT 'Monto de la cuota que se deduce en cada período de planilla',
  saldo_pendiente     DECIMAL(14,2) NOT NULL COMMENT 'Saldo del préstamo que aún falta por pagar',
  fecha_inicio        DATE NOT NULL COMMENT 'Fecha en que inició el préstamo/las deducciones',
  fecha_fin_estimada  DATE NULL COMMENT 'Fecha estimada en que el préstamo quedaría totalmente cancelado',
  estado              ENUM('activo','cancelado','suspendido') NOT NULL DEFAULT 'activo' COMMENT 'Estado actual del préstamo',
  origen_carga             ENUM('operacion','migracion_inicial') NOT NULL DEFAULT 'operacion' COMMENT 'operacion = préstamo dado de alta por el sistema en marcha; migracion_inicial = saldo pendiente digitado a mano al arrancar el sistema, sin historial de cuotas anteriores',
  registrado_por_usuario_id BIGINT UNSIGNED NULL COMMENT 'Usuario que registró este préstamo; obligatorio en la práctica cuando origen_carga = migracion_inicial',
  CONSTRAINT fk_pr_empleado FOREIGN KEY (empleado_id) REFERENCES empleados(id),
  CONSTRAINT fk_pr_tercero FOREIGN KEY (tercero_id) REFERENCES terceros(id),
  CONSTRAINT fk_pr_usuario FOREIGN KEY (registrado_por_usuario_id) REFERENCES usuarios(id),
  CONSTRAINT chk_pr_fechas CHECK (fecha_fin_estimada IS NULL OR fecha_fin_estimada >= fecha_inicio),
  CONSTRAINT chk_pr_saldo CHECK (saldo_pendiente >= 0 AND saldo_pendiente <= monto_total)
) ENGINE=InnoDB COMMENT='Préstamos de terceros a empleados, con saldo pendiente para deducción periódica';

CREATE TABLE embargos (
  id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del embargo',
  empleado_id            BIGINT UNSIGNED NOT NULL COMMENT 'Empleado afectado por el embargo',
  tercero_id             BIGINT UNSIGNED NOT NULL COMMENT 'juzgado o entidad que ordena el embargo',
  tipo_embargo_id        BIGINT UNSIGNED NOT NULL COMMENT 'Tipo de embargo (define su prioridad legal base)',
  numero_expediente      VARCHAR(60) NULL COMMENT 'Número de expediente judicial u oficio que ordena el embargo',
  porcentaje             DECIMAL(5,2) NULL COMMENT 'Porcentaje del salario a embargar, cuando el embargo se calcula como porcentaje',
  monto_fijo             DECIMAL(14,2) NULL COMMENT 'Monto fijo a embargar por período, cuando el embargo se calcula como monto fijo',
  prioridad_aplicacion   INT NOT NULL COMMENT 'orden efectivo de aplicación para este empleado; deriva de parametros_tipos_embargo.prioridad_legal',
  fecha_inicio_vigencia  DATE NOT NULL COMMENT 'Fecha desde la cual este embargo está vigente',
  fecha_fin_vigencia     DATE NULL COMMENT 'NULL = vigente actualmente',
  estado                 ENUM('activo','finalizado','suspendido') NOT NULL DEFAULT 'activo' COMMENT 'Estado actual del embargo',
  CONSTRAINT fk_emb_empleado FOREIGN KEY (empleado_id) REFERENCES empleados(id),
  CONSTRAINT fk_emb_tercero FOREIGN KEY (tercero_id) REFERENCES terceros(id),
  CONSTRAINT fk_emb_tipo FOREIGN KEY (tipo_embargo_id) REFERENCES parametros_tipos_embargo(id),
  CONSTRAINT chk_emb_porcentaje_o_monto CHECK (
    (porcentaje IS NOT NULL AND monto_fijo IS NULL) OR
    (porcentaje IS NULL AND monto_fijo IS NOT NULL)
  ),
  CONSTRAINT chk_emb_porcentaje_rango CHECK (porcentaje IS NULL OR (porcentaje > 0 AND porcentaje <= 100)),
  CONSTRAINT chk_emb_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  KEY ix_emb_empleado_prioridad (empleado_id, prioridad_aplicacion)
) ENGINE=InnoDB COMMENT='Embargos judiciales/de terceros por empleado, con prioridad de aplicación';

CREATE TABLE planilla_detalle_embargo (
  id                            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno de la línea de embargo aplicado',
  planilla_detalle_empleado_id  BIGINT UNSIGNED NOT NULL COMMENT 'Línea de resumen del empleado (periodo de planilla) a la que pertenece este monto de embargo',
  embargo_id                    BIGINT UNSIGNED NOT NULL COMMENT 'Embargo (catálogo de embargos del empleado) que se está aplicando en este período',
  orden_prioridad               INT NOT NULL COMMENT 'Copia de embargos.prioridad_aplicacion vigente al momento del cálculo, para trazabilidad histórica si la prioridad cambia después',
  monto                         DECIMAL(14,2) NOT NULL COMMENT 'Monto efectivamente retenido por este embargo en este período (puede ser menor al calculado por porcentaje/monto_fijo si el neto disponible no alcanza)',
  CONSTRAINT fk_pdemb_detalle FOREIGN KEY (planilla_detalle_empleado_id) REFERENCES planilla_detalle_empleado(id),
  CONSTRAINT fk_pdemb_embargo FOREIGN KEY (embargo_id) REFERENCES embargos(id),
  CONSTRAINT chk_pdemb_monto CHECK (monto >= 0),
  UNIQUE KEY uk_pdemb (planilla_detalle_empleado_id, embargo_id),
  KEY ix_pdemb_embargo (embargo_id)
) ENGINE=InnoDB COMMENT='Monto de embargo efectivamente retenido por línea de planilla, con la prioridad usada -- ver advertencia de inconsistencia arriba';

CREATE TABLE cuentas_contables (
  id               BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno de la cuenta contable',
  codigo           VARCHAR(40) NOT NULL UNIQUE COMMENT 'p.ej. 5.1.1.05.01.00.0',
  nombre           VARCHAR(200) NOT NULL COMMENT 'Nombre descriptivo de la cuenta contable',
  cuenta_padre_id  BIGINT UNSIGNED NULL COMMENT 'Cuenta contable de la que depende dentro del catálogo jerárquico (auto-referencia); NULL si es cuenta raíz',
  nivel            TINYINT UNSIGNED NOT NULL COMMENT 'Nivel jerárquico de la cuenta dentro del catálogo de 7 segmentos',
  naturaleza       ENUM('activo','pasivo','gasto','ingreso','patrimonio') NOT NULL COMMENT 'Naturaleza contable de la cuenta',
  activo           TINYINT(1) NOT NULL DEFAULT 1 COMMENT '1 = cuenta contable disponible para usarse, 0 = dada de baja',
  CONSTRAINT fk_cc_padre FOREIGN KEY (cuenta_padre_id) REFERENCES cuentas_contables(id)
) ENGINE=InnoDB COMMENT='Catálogo jerárquico de cuentas contables (7 segmentos)';

CREATE TABLE tipos_asiento (
  id           BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del tipo de asiento',
  codigo       VARCHAR(40) NOT NULL UNIQUE COMMENT 'p.ej. GASTO_SALARIOS_PASIVOS, CARGAS_SOCIALES_PATRONALES',
  nombre       VARCHAR(150) NOT NULL COMMENT 'Nombre descriptivo del tipo de asiento contable',
  descripcion  VARCHAR(255) NULL COMMENT 'Detalle adicional sobre el propósito de este tipo de asiento'
) ENGINE=InnoDB COMMENT='Catálogo de tipos de asiento contable que puede generar una corrida de planilla';

CREATE TABLE plantillas_asiento_linea (
  id                   BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno de la línea plantilla',
  tipo_asiento_id      BIGINT UNSIGNED NOT NULL COMMENT 'Tipo de asiento al que pertenece esta línea plantilla',
  descripcion_linea    VARCHAR(200) NOT NULL COMMENT 'Descripción de la línea (aparecerá en el asiento generado)',
  tipo_movimiento      ENUM('debe','haber') NOT NULL COMMENT 'Si la línea es un movimiento débito o crédito',
  orden                INT NOT NULL COMMENT 'Posición de la línea dentro del asiento generado',
  CONSTRAINT fk_pal_tipo FOREIGN KEY (tipo_asiento_id) REFERENCES tipos_asiento(id)
) ENGINE=InnoDB COMMENT='Plantilla de líneas por tipo de asiento, configurable sin tocar código';

CREATE TABLE mapeo_concepto_cuenta (
  id                          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno de la regla de mapeo',
  concepto_salarial_id        BIGINT UNSIGNED NOT NULL COMMENT 'Concepto salarial que se está mapeando a una cuenta contable',
  programa_presupuestario_id  BIGINT UNSIGNED NOT NULL COMMENT 'Programa presupuestario al que aplica este mapeo',
  tipo_asiento_id             BIGINT UNSIGNED NOT NULL COMMENT 'Tipo de asiento en el que debe usarse esta cuenta',
  cuenta_contable_id          BIGINT UNSIGNED NOT NULL COMMENT 'Cuenta contable a la que se debe registrar el concepto en este contexto',
  tipo_movimiento             ENUM('debe','haber') NOT NULL COMMENT 'Si el concepto se registra como débito o crédito en este mapeo',
  proyecto                    VARCHAR(20) NOT NULL DEFAULT '99' COMMENT 'Código de proyecto presupuestario asociado (configurable, no constante fija)',
  fecha_inicio_vigencia       DATE NOT NULL COMMENT 'Fecha desde la cual este mapeo es el vigente',
  fecha_fin_vigencia          DATE NULL COMMENT 'NULL = vigente actualmente',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED
    COMMENT 'Columna técnica: soporta uk_mcc_una_vigente',
  CONSTRAINT fk_mcc_concepto FOREIGN KEY (concepto_salarial_id) REFERENCES conceptos_salariales(id),
  CONSTRAINT fk_mcc_programa FOREIGN KEY (programa_presupuestario_id) REFERENCES programas_presupuestarios(id),
  CONSTRAINT fk_mcc_tipoasiento FOREIGN KEY (tipo_asiento_id) REFERENCES tipos_asiento(id),
  CONSTRAINT fk_mcc_cuenta FOREIGN KEY (cuenta_contable_id) REFERENCES cuentas_contables(id),
  CONSTRAINT chk_mcc_vigencia CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  UNIQUE KEY uk_mcc_una_vigente (concepto_salarial_id, programa_presupuestario_id, tipo_asiento_id, es_vigente),
  KEY ix_mcc_busqueda (concepto_salarial_id, programa_presupuestario_id, tipo_asiento_id)
) ENGINE=InnoDB COMMENT='Mapeo configurable de cada concepto salarial a la cuenta contable correspondiente';

CREATE TABLE asientos_contables (
  id                   BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del asiento contable',
  periodo_planilla_id  BIGINT UNSIGNED NOT NULL COMMENT 'Corrida de planilla que originó este asiento',
  tipo_asiento_id      BIGINT UNSIGNED NOT NULL COMMENT 'Tipo de asiento generado',
  numero_asiento       VARCHAR(40) NOT NULL COMMENT 'Número/folio de asiento asignado (correlativo contable)',
  fecha_generacion     DATETIME NOT NULL COMMENT 'Fecha/hora en que se generó el asiento',
  estado               ENUM('generado','exportado') NOT NULL DEFAULT 'generado' COMMENT 'Estado del asiento: generado en el sistema o ya exportado a contabilidad/SAP',
  UNIQUE KEY uk_asiento (periodo_planilla_id, tipo_asiento_id),
  CONSTRAINT fk_ac_periodo FOREIGN KEY (periodo_planilla_id) REFERENCES periodos_planilla(id),
  CONSTRAINT fk_ac_tipo FOREIGN KEY (tipo_asiento_id) REFERENCES tipos_asiento(id)
) ENGINE=InnoDB COMMENT='Al menos 2 asientos por corrida de planilla';

CREATE TABLE asientos_contables_detalle (
  id                          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno de la línea del asiento',
  asiento_contable_id         BIGINT UNSIGNED NOT NULL COMMENT 'Asiento contable al que pertenece esta línea',
  cuenta_contable_id          BIGINT UNSIGNED NOT NULL COMMENT 'Cuenta contable afectada por esta línea',
  programa_presupuestario_id  BIGINT UNSIGNED NOT NULL COMMENT 'Programa presupuestario asociado a esta línea',
  proyecto                    VARCHAR(20) NOT NULL DEFAULT '99' COMMENT 'Código de proyecto presupuestario asociado a esta línea',
  empleado_id                 BIGINT UNSIGNED NULL COMMENT 'NULL cuando la línea es un consolidado (p.ej. salarios por pagar global)',
  concepto_salarial_id        BIGINT UNSIGNED NULL COMMENT 'Concepto salarial de origen de esta línea, cuando aplica',
  monto_debe                  DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Monto registrado como débito en esta línea',
  monto_haber                 DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Monto registrado como crédito en esta línea',
  CONSTRAINT fk_acd_asiento FOREIGN KEY (asiento_contable_id) REFERENCES asientos_contables(id),
  CONSTRAINT fk_acd_cuenta FOREIGN KEY (cuenta_contable_id) REFERENCES cuentas_contables(id),
  CONSTRAINT fk_acd_programa FOREIGN KEY (programa_presupuestario_id) REFERENCES programas_presupuestarios(id),
  CONSTRAINT fk_acd_empleado FOREIGN KEY (empleado_id) REFERENCES empleados(id),
  CONSTRAINT fk_acd_concepto FOREIGN KEY (concepto_salarial_id) REFERENCES conceptos_salariales(id),
  CONSTRAINT chk_acd_debe_o_haber CHECK (
    (monto_debe = 0 AND monto_haber > 0) OR (monto_debe > 0 AND monto_haber = 0)
  )
) ENGINE=InnoDB COMMENT='Débitos deben cuadrar contra créditos por asiento (validado en aplicación); cada línea es débito XOR crédito (validado en base de datos)';

CREATE TABLE sistemas_origen (
  id      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del sistema origen',
  codigo  VARCHAR(20)  NOT NULL UNIQUE COMMENT 'p.ej. SAP, PAYONE, JAISSON',
  nombre  VARCHAR(100) NOT NULL COMMENT 'Nombre descriptivo del sistema origen'
) ENGINE=InnoDB COMMENT='Catálogo de sistemas externos de origen de datos importables';

CREATE TABLE interfaces_importacion (
  id                  BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno de la interfaz de importación',
  sistema_origen_id   BIGINT UNSIGNED NOT NULL COMMENT 'Sistema origen al que corresponde esta interfaz',
  nombre              VARCHAR(100) NOT NULL COMMENT 'Nombre descriptivo de la interfaz de importación',
  formato_archivo     ENUM('delimitado','ancho_fijo','hoja_calculo') NOT NULL COMMENT 'Formato del archivo que procesa esta interfaz',
  configuracion_mapeo JSON NOT NULL COMMENT 'columnas, delimitadores, posiciones',
  activo              TINYINT(1) NOT NULL DEFAULT 1 COMMENT '1 = interfaz disponible para usarse, 0 = deshabilitada',
  CONSTRAINT fk_ii_sistema FOREIGN KEY (sistema_origen_id) REFERENCES sistemas_origen(id)
) ENGINE=InnoDB COMMENT='Definición configurable de cada interfaz de importación de datos';

CREATE TABLE importaciones_staging (
  id                       BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno de la carga',
  interfaz_importacion_id  BIGINT UNSIGNED NOT NULL COMMENT 'Interfaz de importación usada para procesar esta carga',
  nombre_archivo_origen    VARCHAR(255) NOT NULL COMMENT 'Nombre del archivo que se cargó',
  fecha_carga              DATETIME NOT NULL COMMENT 'Fecha/hora en que se cargó el archivo',
  estado                   ENUM('cargado','validado','con_errores','consolidado') NOT NULL DEFAULT 'cargado' COMMENT 'Etapa actual del procesamiento de la carga',
  contenido_raw            LONGTEXT NULL COMMENT 'Contenido crudo del archivo cargado, tal como se recibió',
  errores_validacion       JSON NULL COMMENT 'Detalle estructurado de los errores encontrados al validar la carga',
  CONSTRAINT fk_is_interfaz FOREIGN KEY (interfaz_importacion_id) REFERENCES interfaces_importacion(id)
) ENGINE=InnoDB COMMENT='Zona de aterrizaje antes de validar y consolidar';

CREATE TABLE historico_salarios_pre_migracion (
  id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del registro histórico',
  empleado_id            BIGINT UNSIGNED NOT NULL COMMENT 'Empleado al que corresponde el salario devengado',
  anio                   SMALLINT UNSIGNED NOT NULL COMMENT 'Año calendario del salario devengado',
  mes                    TINYINT UNSIGNED NOT NULL COMMENT 'Mes calendario (1-12) del salario devengado',
  concepto_salarial_id   BIGINT UNSIGNED NULL COMMENT 'Concepto salarial específico, si se tiene el desglose; NULL si solo se migró el total del mes',
  monto                  DECIMAL(14,2) NOT NULL COMMENT 'Monto devengado (ingreso computable) ese mes, previo a la puesta en marcha del sistema',
  fuente                 VARCHAR(100) NULL COMMENT 'Origen del dato migrado (p.ej. "Planilla Excel 2025-2026", "Sistema anterior")',
  cargado_por_usuario_id BIGINT UNSIGNED NULL COMMENT 'Usuario que realizó la carga histórica',
  fecha_carga            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Fecha/hora en que se cargó este dato al sistema',
  CONSTRAINT fk_hsm_empleado FOREIGN KEY (empleado_id) REFERENCES empleados(id),
  CONSTRAINT fk_hsm_concepto FOREIGN KEY (concepto_salarial_id) REFERENCES conceptos_salariales(id),
  CONSTRAINT fk_hsm_usuario FOREIGN KEY (cargado_por_usuario_id) REFERENCES usuarios(id),
  CONSTRAINT chk_hsm_mes CHECK (mes BETWEEN 1 AND 12),
  UNIQUE KEY uk_hsm (empleado_id, anio, mes, concepto_salarial_id)
) ENGINE=InnoDB COMMENT='Salarios devengados antes de la puesta en marcha del sistema, usados únicamente como insumo para aguinaldo/cesantía/renta anual; nunca genera asientos contables ni pasa por el ciclo de vida de periodos_planilla';

CREATE TABLE formatos_exportacion (
  id      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del formato de exportación',
  codigo  VARCHAR(30)  NOT NULL UNIQUE COMMENT 'p.ej. BANCO_TXT_ANCHO_FIJO, CSV, XLSX, PDF_FIRMABLE',
  nombre  VARCHAR(100) NOT NULL COMMENT 'Nombre descriptivo del formato de exportación',
  tipo_destino ENUM('banco','autoridad','interno') NOT NULL COMMENT 'Tipo de destinatario al que se dirige esta exportación'
) ENGINE=InnoDB COMMENT='Catálogo de formatos/generadores de exportación disponibles';

CREATE TABLE exportaciones_generadas (
  id                      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno de la exportación generada',
  periodo_planilla_id     BIGINT UNSIGNED NOT NULL COMMENT 'Corrida de planilla que originó esta exportación',
  formato_exportacion_id  BIGINT UNSIGNED NOT NULL COMMENT 'Formato utilizado para generar el archivo',
  fecha_generacion        DATETIME NOT NULL COMMENT 'Fecha/hora en que se generó el archivo de exportación',
  ruta_archivo            VARCHAR(255) NOT NULL COMMENT 'Ruta o ubicación donde quedó almacenado el archivo generado',
  hash_verificacion       VARCHAR(128) NOT NULL COMMENT 'Hash del archivo generado, para verificar su integridad posteriormente',
  generado_por_usuario_id BIGINT UNSIGNED NULL COMMENT 'Usuario del sistema que generó la exportación',
  CONSTRAINT fk_eg_periodo FOREIGN KEY (periodo_planilla_id) REFERENCES periodos_planilla(id),
  CONSTRAINT fk_eg_formato FOREIGN KEY (formato_exportacion_id) REFERENCES formatos_exportacion(id),
  CONSTRAINT fk_eg_usuario FOREIGN KEY (generado_por_usuario_id) REFERENCES usuarios(id)
) ENGINE=InnoDB COMMENT='Registro histórico de exportaciones, con hash para banco/autoridades/PDF firmable';

CREATE TABLE usuarios (
  id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del usuario del sistema',
  nombre_usuario  VARCHAR(60)  NOT NULL UNIQUE COMMENT 'Nombre de usuario (login) único',
  nombre_completo VARCHAR(200) NOT NULL COMMENT 'Nombre completo de la persona usuaria',
  correo          VARCHAR(150) NOT NULL UNIQUE COMMENT 'Correo electrónico del usuario, único en el sistema',
  password_hash   VARCHAR(255) NOT NULL COMMENT 'Hash de la contraseña del usuario (nunca se guarda en texto plano)',
  empleado_id     BIGINT UNSIGNED NULL COMMENT 'vínculo opcional si el usuario del sistema también es empleado',
  activo          TINYINT(1) NOT NULL DEFAULT 1 COMMENT '1 = cuenta habilitada para iniciar sesión, 0 = deshabilitada',
  CONSTRAINT fk_usr_empleado FOREIGN KEY (empleado_id) REFERENCES empleados(id)
) ENGINE=InnoDB COMMENT='Cuentas de usuario que acceden al sistema';

CREATE TABLE roles (
  id      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del rol',
  codigo  VARCHAR(40)  NOT NULL UNIQUE COMMENT 'Código corto y único del rol',
  nombre  VARCHAR(100) NOT NULL COMMENT 'p.ej. Encargada RRHH/Contabilidad, Director Ejecutivo'
) ENGINE=InnoDB COMMENT='Catálogo de roles funcionales del sistema';

CREATE TABLE permisos (
  id      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del permiso',
  codigo  VARCHAR(60)  NOT NULL UNIQUE COMMENT 'Código corto y único del permiso',
  nombre  VARCHAR(150) NOT NULL COMMENT 'Nombre descriptivo del permiso',
  modulo  VARCHAR(60)  NOT NULL COMMENT 'empleados, asistencia, parametros, planilla, contabilizacion, integraciones, auditoria'
) ENGINE=InnoDB COMMENT='Base para segregación de funciones exigida por Ley 8292';

CREATE TABLE rol_permisos (
  rol_id      BIGINT UNSIGNED NOT NULL COMMENT 'Rol al que se le asigna el permiso',
  permiso_id  BIGINT UNSIGNED NOT NULL COMMENT 'Permiso asignado al rol',
  PRIMARY KEY (rol_id, permiso_id),
  CONSTRAINT fk_rp_rol FOREIGN KEY (rol_id) REFERENCES roles(id),
  CONSTRAINT fk_rp_permiso FOREIGN KEY (permiso_id) REFERENCES permisos(id)
) ENGINE=InnoDB COMMENT='Asignación de permisos a cada rol';

CREATE TABLE usuario_roles (
  usuario_id  BIGINT UNSIGNED NOT NULL COMMENT 'Usuario al que se le asigna el rol',
  rol_id      BIGINT UNSIGNED NOT NULL COMMENT 'Rol asignado al usuario',
  PRIMARY KEY (usuario_id, rol_id),
  CONSTRAINT fk_ur_usuario FOREIGN KEY (usuario_id) REFERENCES usuarios(id),
  CONSTRAINT fk_ur_rol FOREIGN KEY (rol_id) REFERENCES roles(id)
) ENGINE=InnoDB COMMENT='Asignación de roles a cada usuario del sistema';

CREATE TABLE auditoria_cambios (
  id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador interno del registro de auditoría',
  tabla_afectada  VARCHAR(100) NOT NULL COMMENT 'Nombre de la tabla del sistema donde ocurrió el cambio',
  registro_id     BIGINT UNSIGNED NOT NULL COMMENT 'Identificador (PK) del registro afectado dentro de esa tabla',
  usuario_id      BIGINT UNSIGNED NULL COMMENT 'Usuario que realizó el cambio, si se pudo identificar',
  accion          ENUM('insert','update','delete') NOT NULL COMMENT 'Tipo de operación realizada sobre el registro',
  valor_anterior  JSON NULL COMMENT 'Valor del registro antes del cambio (NULL si la acción fue insert)',
  valor_nuevo     JSON NULL COMMENT 'Valor del registro después del cambio (NULL si la acción fue delete)',
  fecha_cambio    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Fecha/hora en que ocurrió el cambio',
  CONSTRAINT fk_aud_usuario FOREIGN KEY (usuario_id) REFERENCES usuarios(id),
  KEY ix_aud_tabla_registro (tabla_afectada, registro_id),
  KEY ix_aud_fecha (fecha_cambio)
) ENGINE=InnoDB COMMENT='Trazabilidad completa: quién, cuándo, valor anterior/nuevo';

DROP TRIGGER IF EXISTS trg_aud_tipos_identificacion_ins;
CREATE TRIGGER trg_aud_tipos_identificacion_ins AFTER INSERT ON tipos_identificacion
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_identificacion', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_tipos_identificacion_upd;
CREATE TRIGGER trg_aud_tipos_identificacion_upd AFTER UPDATE ON tipos_identificacion
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_identificacion', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'activo', OLD.activo), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_tipos_identificacion_del;
CREATE TRIGGER trg_aud_tipos_identificacion_del AFTER DELETE ON tipos_identificacion
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_identificacion', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'activo', OLD.activo), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_clasificaciones_puesto_ins;
CREATE TRIGGER trg_aud_clasificaciones_puesto_ins AFTER INSERT ON clasificaciones_puesto
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('clasificaciones_puesto', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_clasificaciones_puesto_upd;
CREATE TRIGGER trg_aud_clasificaciones_puesto_upd AFTER UPDATE ON clasificaciones_puesto
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('clasificaciones_puesto', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'activo', OLD.activo), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_clasificaciones_puesto_del;
CREATE TRIGGER trg_aud_clasificaciones_puesto_del AFTER DELETE ON clasificaciones_puesto
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('clasificaciones_puesto', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'activo', OLD.activo), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_regimenes_ccss_ins;
CREATE TRIGGER trg_aud_regimenes_ccss_ins AFTER INSERT ON regimenes_ccss
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('regimenes_ccss', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_regimenes_ccss_upd;
CREATE TRIGGER trg_aud_regimenes_ccss_upd AFTER UPDATE ON regimenes_ccss
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('regimenes_ccss', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'activo', OLD.activo), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_regimenes_ccss_del;
CREATE TRIGGER trg_aud_regimenes_ccss_del AFTER DELETE ON regimenes_ccss
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('regimenes_ccss', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'activo', OLD.activo), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_tipos_credito_fiscal_renta_ins;
CREATE TRIGGER trg_aud_tipos_credito_fiscal_renta_ins AFTER INSERT ON tipos_credito_fiscal_renta
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_credito_fiscal_renta', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_tipos_credito_fiscal_renta_upd;
CREATE TRIGGER trg_aud_tipos_credito_fiscal_renta_upd AFTER UPDATE ON tipos_credito_fiscal_renta
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_credito_fiscal_renta', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'activo', OLD.activo), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_tipos_credito_fiscal_renta_del;
CREATE TRIGGER trg_aud_tipos_credito_fiscal_renta_del AFTER DELETE ON tipos_credito_fiscal_renta
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_credito_fiscal_renta', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'activo', OLD.activo), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_entidades_emisoras_incapacidad_ins;
CREATE TRIGGER trg_aud_entidades_emisoras_incapacidad_ins AFTER INSERT ON entidades_emisoras_incapacidad
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('entidades_emisoras_incapacidad', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_entidades_emisoras_incapacidad_upd;
CREATE TRIGGER trg_aud_entidades_emisoras_incapacidad_upd AFTER UPDATE ON entidades_emisoras_incapacidad
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('entidades_emisoras_incapacidad', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'activo', OLD.activo), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_entidades_emisoras_incapacidad_del;
CREATE TRIGGER trg_aud_entidades_emisoras_incapacidad_del AFTER DELETE ON entidades_emisoras_incapacidad
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('entidades_emisoras_incapacidad', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'activo', OLD.activo), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_tipos_hora_extra_ins;
CREATE TRIGGER trg_aud_tipos_hora_extra_ins AFTER INSERT ON tipos_hora_extra
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_hora_extra', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_tipos_hora_extra_upd;
CREATE TRIGGER trg_aud_tipos_hora_extra_upd AFTER UPDATE ON tipos_hora_extra
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_hora_extra', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'activo', OLD.activo), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_tipos_hora_extra_del;
CREATE TRIGGER trg_aud_tipos_hora_extra_del AFTER DELETE ON tipos_hora_extra
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_hora_extra', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'activo', OLD.activo), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_unidades_organizativas_ins;
CREATE TRIGGER trg_aud_unidades_organizativas_ins AFTER INSERT ON unidades_organizativas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('unidades_organizativas', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'unidad_padre_id', NEW.unidad_padre_id, 'activo', NEW.activo, 'created_at', NEW.created_at, 'updated_at', NEW.updated_at));
END;

DROP TRIGGER IF EXISTS trg_aud_unidades_organizativas_upd;
CREATE TRIGGER trg_aud_unidades_organizativas_upd AFTER UPDATE ON unidades_organizativas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('unidades_organizativas', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'unidad_padre_id', OLD.unidad_padre_id, 'activo', OLD.activo, 'created_at', OLD.created_at, 'updated_at', OLD.updated_at), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'unidad_padre_id', NEW.unidad_padre_id, 'activo', NEW.activo, 'created_at', NEW.created_at, 'updated_at', NEW.updated_at));
END;

DROP TRIGGER IF EXISTS trg_aud_unidades_organizativas_del;
CREATE TRIGGER trg_aud_unidades_organizativas_del AFTER DELETE ON unidades_organizativas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('unidades_organizativas', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'unidad_padre_id', OLD.unidad_padre_id, 'activo', OLD.activo, 'created_at', OLD.created_at, 'updated_at', OLD.updated_at), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_programas_presupuestarios_ins;
CREATE TRIGGER trg_aud_programas_presupuestarios_ins AFTER INSERT ON programas_presupuestarios
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('programas_presupuestarios', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'activo', NEW.activo, 'notas', NEW.notas));
END;

DROP TRIGGER IF EXISTS trg_aud_programas_presupuestarios_upd;
CREATE TRIGGER trg_aud_programas_presupuestarios_upd AFTER UPDATE ON programas_presupuestarios
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('programas_presupuestarios', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'activo', OLD.activo, 'notas', OLD.notas), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'activo', NEW.activo, 'notas', NEW.notas));
END;

DROP TRIGGER IF EXISTS trg_aud_programas_presupuestarios_del;
CREATE TRIGGER trg_aud_programas_presupuestarios_del AFTER DELETE ON programas_presupuestarios
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('programas_presupuestarios', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'activo', OLD.activo, 'notas', OLD.notas), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_puestos_ins;
CREATE TRIGGER trg_aud_puestos_ins AFTER INSERT ON puestos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('puestos', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'unidad_organizativa_id', NEW.unidad_organizativa_id, 'clasificacion_puesto_id', NEW.clasificacion_puesto_id, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_puestos_upd;
CREATE TRIGGER trg_aud_puestos_upd AFTER UPDATE ON puestos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('puestos', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'unidad_organizativa_id', OLD.unidad_organizativa_id, 'clasificacion_puesto_id', OLD.clasificacion_puesto_id, 'activo', OLD.activo), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'unidad_organizativa_id', NEW.unidad_organizativa_id, 'clasificacion_puesto_id', NEW.clasificacion_puesto_id, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_puestos_del;
CREATE TRIGGER trg_aud_puestos_del AFTER DELETE ON puestos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('puestos', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'unidad_organizativa_id', OLD.unidad_organizativa_id, 'clasificacion_puesto_id', OLD.clasificacion_puesto_id, 'activo', OLD.activo), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_tipos_contrato_ins;
CREATE TRIGGER trg_aud_tipos_contrato_ins AFTER INSERT ON tipos_contrato
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_contrato', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'descripcion', NEW.descripcion));
END;

DROP TRIGGER IF EXISTS trg_aud_tipos_contrato_upd;
CREATE TRIGGER trg_aud_tipos_contrato_upd AFTER UPDATE ON tipos_contrato
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_contrato', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'descripcion', OLD.descripcion), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'descripcion', NEW.descripcion));
END;

DROP TRIGGER IF EXISTS trg_aud_tipos_contrato_del;
CREATE TRIGGER trg_aud_tipos_contrato_del AFTER DELETE ON tipos_contrato
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_contrato', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'descripcion', OLD.descripcion), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_sindicatos_ins;
CREATE TRIGGER trg_aud_sindicatos_ins AFTER INSERT ON sindicatos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('sindicatos', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre));
END;

DROP TRIGGER IF EXISTS trg_aud_sindicatos_upd;
CREATE TRIGGER trg_aud_sindicatos_upd AFTER UPDATE ON sindicatos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('sindicatos', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre));
END;

DROP TRIGGER IF EXISTS trg_aud_sindicatos_del;
CREATE TRIGGER trg_aud_sindicatos_del AFTER DELETE ON sindicatos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('sindicatos', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_bancos_ins;
CREATE TRIGGER trg_aud_bancos_ins AFTER INSERT ON bancos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('bancos', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'es_depositario_nomina', NEW.es_depositario_nomina, 'es_otorgante_prestamos', NEW.es_otorgante_prestamos, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_bancos_upd;
CREATE TRIGGER trg_aud_bancos_upd AFTER UPDATE ON bancos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('bancos', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'es_depositario_nomina', OLD.es_depositario_nomina, 'es_otorgante_prestamos', OLD.es_otorgante_prestamos, 'activo', OLD.activo), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'es_depositario_nomina', NEW.es_depositario_nomina, 'es_otorgante_prestamos', NEW.es_otorgante_prestamos, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_bancos_del;
CREATE TRIGGER trg_aud_bancos_del AFTER DELETE ON bancos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('bancos', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'es_depositario_nomina', OLD.es_depositario_nomina, 'es_otorgante_prestamos', OLD.es_otorgante_prestamos, 'activo', OLD.activo), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_empleados_ins;
CREATE TRIGGER trg_aud_empleados_ins AFTER INSERT ON empleados
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleados', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo_interno', NEW.codigo_interno, 'tipo_identificacion_id', NEW.tipo_identificacion_id, 'numero_identificacion', NEW.numero_identificacion, 'nombre_completo', NEW.nombre_completo, 'fecha_ingreso', NEW.fecha_ingreso, 'activo', NEW.activo, 'created_at', NEW.created_at, 'updated_at', NEW.updated_at));
END;

DROP TRIGGER IF EXISTS trg_aud_empleados_upd;
CREATE TRIGGER trg_aud_empleados_upd AFTER UPDATE ON empleados
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleados', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo_interno', OLD.codigo_interno, 'tipo_identificacion_id', OLD.tipo_identificacion_id, 'numero_identificacion', OLD.numero_identificacion, 'nombre_completo', OLD.nombre_completo, 'fecha_ingreso', OLD.fecha_ingreso, 'activo', OLD.activo, 'created_at', OLD.created_at, 'updated_at', OLD.updated_at), JSON_OBJECT('id', NEW.id, 'codigo_interno', NEW.codigo_interno, 'tipo_identificacion_id', NEW.tipo_identificacion_id, 'numero_identificacion', NEW.numero_identificacion, 'nombre_completo', NEW.nombre_completo, 'fecha_ingreso', NEW.fecha_ingreso, 'activo', NEW.activo, 'created_at', NEW.created_at, 'updated_at', NEW.updated_at));
END;

DROP TRIGGER IF EXISTS trg_aud_empleados_del;
CREATE TRIGGER trg_aud_empleados_del AFTER DELETE ON empleados
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleados', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo_interno', OLD.codigo_interno, 'tipo_identificacion_id', OLD.tipo_identificacion_id, 'numero_identificacion', OLD.numero_identificacion, 'nombre_completo', OLD.nombre_completo, 'fecha_ingreso', OLD.fecha_ingreso, 'activo', OLD.activo, 'created_at', OLD.created_at, 'updated_at', OLD.updated_at), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_datos_personales_ins;
CREATE TRIGGER trg_aud_empleado_datos_personales_ins AFTER INSERT ON empleado_datos_personales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_datos_personales', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'direccion', NEW.direccion, 'telefono', NEW.telefono, 'correo_electronico', NEW.correo_electronico, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_datos_personales_upd;
CREATE TRIGGER trg_aud_empleado_datos_personales_upd AFTER UPDATE ON empleado_datos_personales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_datos_personales', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'direccion', OLD.direccion, 'telefono', OLD.telefono, 'correo_electronico', OLD.correo_electronico, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'direccion', NEW.direccion, 'telefono', NEW.telefono, 'correo_electronico', NEW.correo_electronico, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_datos_personales_del;
CREATE TRIGGER trg_aud_empleado_datos_personales_del AFTER DELETE ON empleado_datos_personales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_datos_personales', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'direccion', OLD.direccion, 'telefono', OLD.telefono, 'correo_electronico', OLD.correo_electronico, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_datos_laborales_ins;
CREATE TRIGGER trg_aud_empleado_datos_laborales_ins AFTER INSERT ON empleado_datos_laborales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_datos_laborales', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'puesto_id', NEW.puesto_id, 'salario_base_nominal', NEW.salario_base_nominal, 'tipo_contrato_id', NEW.tipo_contrato_id, 'programa_presupuestario_id', NEW.programa_presupuestario_id, 'sindicato_id', NEW.sindicato_id, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_datos_laborales_upd;
CREATE TRIGGER trg_aud_empleado_datos_laborales_upd AFTER UPDATE ON empleado_datos_laborales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_datos_laborales', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'puesto_id', OLD.puesto_id, 'salario_base_nominal', OLD.salario_base_nominal, 'tipo_contrato_id', OLD.tipo_contrato_id, 'programa_presupuestario_id', OLD.programa_presupuestario_id, 'sindicato_id', OLD.sindicato_id, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'puesto_id', NEW.puesto_id, 'salario_base_nominal', NEW.salario_base_nominal, 'tipo_contrato_id', NEW.tipo_contrato_id, 'programa_presupuestario_id', NEW.programa_presupuestario_id, 'sindicato_id', NEW.sindicato_id, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_datos_laborales_del;
CREATE TRIGGER trg_aud_empleado_datos_laborales_del AFTER DELETE ON empleado_datos_laborales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_datos_laborales', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'puesto_id', OLD.puesto_id, 'salario_base_nominal', OLD.salario_base_nominal, 'tipo_contrato_id', OLD.tipo_contrato_id, 'programa_presupuestario_id', OLD.programa_presupuestario_id, 'sindicato_id', OLD.sindicato_id, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_datos_bancarios_ins;
CREATE TRIGGER trg_aud_empleado_datos_bancarios_ins AFTER INSERT ON empleado_datos_bancarios
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_datos_bancarios', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'banco_id', NEW.banco_id, 'tipo_cuenta', NEW.tipo_cuenta, 'numero_cuenta', NEW.numero_cuenta, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_datos_bancarios_upd;
CREATE TRIGGER trg_aud_empleado_datos_bancarios_upd AFTER UPDATE ON empleado_datos_bancarios
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_datos_bancarios', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'banco_id', OLD.banco_id, 'tipo_cuenta', OLD.tipo_cuenta, 'numero_cuenta', OLD.numero_cuenta, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'banco_id', NEW.banco_id, 'tipo_cuenta', NEW.tipo_cuenta, 'numero_cuenta', NEW.numero_cuenta, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_datos_bancarios_del;
CREATE TRIGGER trg_aud_empleado_datos_bancarios_del AFTER DELETE ON empleado_datos_bancarios
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_datos_bancarios', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'banco_id', OLD.banco_id, 'tipo_cuenta', OLD.tipo_cuenta, 'numero_cuenta', OLD.numero_cuenta, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_datos_familiares_ins;
CREATE TRIGGER trg_aud_empleado_datos_familiares_ins AFTER INSERT ON empleado_datos_familiares
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_datos_familiares', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'aplica_credito_conyuge', NEW.aplica_credito_conyuge, 'numero_hijos_dependientes', NEW.numero_hijos_dependientes, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_datos_familiares_upd;
CREATE TRIGGER trg_aud_empleado_datos_familiares_upd AFTER UPDATE ON empleado_datos_familiares
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_datos_familiares', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'aplica_credito_conyuge', OLD.aplica_credito_conyuge, 'numero_hijos_dependientes', OLD.numero_hijos_dependientes, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'aplica_credito_conyuge', NEW.aplica_credito_conyuge, 'numero_hijos_dependientes', NEW.numero_hijos_dependientes, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_datos_familiares_del;
CREATE TRIGGER trg_aud_empleado_datos_familiares_del AFTER DELETE ON empleado_datos_familiares
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_datos_familiares', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'aplica_credito_conyuge', OLD.aplica_credito_conyuge, 'numero_hijos_dependientes', OLD.numero_hijos_dependientes, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_conceptos_salariales_ins;
CREATE TRIGGER trg_aud_conceptos_salariales_ins AFTER INSERT ON conceptos_salariales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('conceptos_salariales', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'naturaleza', NEW.naturaleza, 'tipo_calculo', NEW.tipo_calculo, 'afecta_base_ccss', NEW.afecta_base_ccss, 'afecta_base_renta', NEW.afecta_base_renta, 'afecta_base_aguinaldo', NEW.afecta_base_aguinaldo, 'afecta_base_cesantia', NEW.afecta_base_cesantia, 'afecta_base_vacaciones', NEW.afecta_base_vacaciones, 'activo', NEW.activo, 'created_at', NEW.created_at));
END;

DROP TRIGGER IF EXISTS trg_aud_conceptos_salariales_upd;
CREATE TRIGGER trg_aud_conceptos_salariales_upd AFTER UPDATE ON conceptos_salariales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('conceptos_salariales', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'naturaleza', OLD.naturaleza, 'tipo_calculo', OLD.tipo_calculo, 'afecta_base_ccss', OLD.afecta_base_ccss, 'afecta_base_renta', OLD.afecta_base_renta, 'afecta_base_aguinaldo', OLD.afecta_base_aguinaldo, 'afecta_base_cesantia', OLD.afecta_base_cesantia, 'afecta_base_vacaciones', OLD.afecta_base_vacaciones, 'activo', OLD.activo, 'created_at', OLD.created_at), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'naturaleza', NEW.naturaleza, 'tipo_calculo', NEW.tipo_calculo, 'afecta_base_ccss', NEW.afecta_base_ccss, 'afecta_base_renta', NEW.afecta_base_renta, 'afecta_base_aguinaldo', NEW.afecta_base_aguinaldo, 'afecta_base_cesantia', NEW.afecta_base_cesantia, 'afecta_base_vacaciones', NEW.afecta_base_vacaciones, 'activo', NEW.activo, 'created_at', NEW.created_at));
END;

DROP TRIGGER IF EXISTS trg_aud_conceptos_salariales_del;
CREATE TRIGGER trg_aud_conceptos_salariales_del AFTER DELETE ON conceptos_salariales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('conceptos_salariales', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'naturaleza', OLD.naturaleza, 'tipo_calculo', OLD.tipo_calculo, 'afecta_base_ccss', OLD.afecta_base_ccss, 'afecta_base_renta', OLD.afecta_base_renta, 'afecta_base_aguinaldo', OLD.afecta_base_aguinaldo, 'afecta_base_cesantia', OLD.afecta_base_cesantia, 'afecta_base_vacaciones', OLD.afecta_base_vacaciones, 'activo', OLD.activo, 'created_at', OLD.created_at), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_concepto_salarial_formula_ins;
CREATE TRIGGER trg_aud_concepto_salarial_formula_ins AFTER INSERT ON concepto_salarial_formula
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('concepto_salarial_formula', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'concepto_salarial_id', NEW.concepto_salarial_id, 'monto_fijo', NEW.monto_fijo, 'expresion_formula', NEW.expresion_formula, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'aprobado_por', NEW.aprobado_por, 'fecha_aprobacion', NEW.fecha_aprobacion, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_concepto_salarial_formula_upd;
CREATE TRIGGER trg_aud_concepto_salarial_formula_upd AFTER UPDATE ON concepto_salarial_formula
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('concepto_salarial_formula', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'concepto_salarial_id', OLD.concepto_salarial_id, 'monto_fijo', OLD.monto_fijo, 'expresion_formula', OLD.expresion_formula, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'aprobado_por', OLD.aprobado_por, 'fecha_aprobacion', OLD.fecha_aprobacion, 'es_vigente', OLD.es_vigente), JSON_OBJECT('id', NEW.id, 'concepto_salarial_id', NEW.concepto_salarial_id, 'monto_fijo', NEW.monto_fijo, 'expresion_formula', NEW.expresion_formula, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'aprobado_por', NEW.aprobado_por, 'fecha_aprobacion', NEW.fecha_aprobacion, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_concepto_salarial_formula_del;
CREATE TRIGGER trg_aud_concepto_salarial_formula_del AFTER DELETE ON concepto_salarial_formula
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('concepto_salarial_formula', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'concepto_salarial_id', OLD.concepto_salarial_id, 'monto_fijo', OLD.monto_fijo, 'expresion_formula', OLD.expresion_formula, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'aprobado_por', OLD.aprobado_por, 'fecha_aprobacion', OLD.fecha_aprobacion, 'es_vigente', OLD.es_vigente), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_orden_calculo_concepto_ins;
CREATE TRIGGER trg_aud_orden_calculo_concepto_ins AFTER INSERT ON orden_calculo_concepto
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('orden_calculo_concepto', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'concepto_salarial_id', NEW.concepto_salarial_id, 'orden', NEW.orden, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_orden_calculo_concepto_upd;
CREATE TRIGGER trg_aud_orden_calculo_concepto_upd AFTER UPDATE ON orden_calculo_concepto
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('orden_calculo_concepto', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'concepto_salarial_id', OLD.concepto_salarial_id, 'orden', OLD.orden, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), JSON_OBJECT('id', NEW.id, 'concepto_salarial_id', NEW.concepto_salarial_id, 'orden', NEW.orden, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_orden_calculo_concepto_del;
CREATE TRIGGER trg_aud_orden_calculo_concepto_del AFTER DELETE ON orden_calculo_concepto
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('orden_calculo_concepto', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'concepto_salarial_id', OLD.concepto_salarial_id, 'orden', OLD.orden, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_cargas_sociales_ins;
CREATE TRIGGER trg_aud_parametros_cargas_sociales_ins AFTER INSERT ON parametros_cargas_sociales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_cargas_sociales', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'rubro', NEW.rubro, 'concepto_salarial_id', NEW.concepto_salarial_id, 'porcentaje_patronal', NEW.porcentaje_patronal, 'porcentaje_obrero', NEW.porcentaje_obrero, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'fuente_legal', NEW.fuente_legal, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_cargas_sociales_upd;
CREATE TRIGGER trg_aud_parametros_cargas_sociales_upd AFTER UPDATE ON parametros_cargas_sociales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_cargas_sociales', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'rubro', OLD.rubro, 'concepto_salarial_id', OLD.concepto_salarial_id, 'porcentaje_patronal', OLD.porcentaje_patronal, 'porcentaje_obrero', OLD.porcentaje_obrero, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'fuente_legal', OLD.fuente_legal, 'es_vigente', OLD.es_vigente), JSON_OBJECT('id', NEW.id, 'rubro', NEW.rubro, 'concepto_salarial_id', NEW.concepto_salarial_id, 'porcentaje_patronal', NEW.porcentaje_patronal, 'porcentaje_obrero', NEW.porcentaje_obrero, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'fuente_legal', NEW.fuente_legal, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_cargas_sociales_del;
CREATE TRIGGER trg_aud_parametros_cargas_sociales_del AFTER DELETE ON parametros_cargas_sociales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_cargas_sociales', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'rubro', OLD.rubro, 'concepto_salarial_id', OLD.concepto_salarial_id, 'porcentaje_patronal', OLD.porcentaje_patronal, 'porcentaje_obrero', OLD.porcentaje_obrero, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'fuente_legal', OLD.fuente_legal, 'es_vigente', OLD.es_vigente), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_tramos_renta_ins;
CREATE TRIGGER trg_aud_parametros_tramos_renta_ins AFTER INSERT ON parametros_tramos_renta
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_tramos_renta', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'orden_tramo', NEW.orden_tramo, 'monto_desde', NEW.monto_desde, 'monto_hasta', NEW.monto_hasta, 'porcentaje', NEW.porcentaje, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_tramos_renta_upd;
CREATE TRIGGER trg_aud_parametros_tramos_renta_upd AFTER UPDATE ON parametros_tramos_renta
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_tramos_renta', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'orden_tramo', OLD.orden_tramo, 'monto_desde', OLD.monto_desde, 'monto_hasta', OLD.monto_hasta, 'porcentaje', OLD.porcentaje, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), JSON_OBJECT('id', NEW.id, 'orden_tramo', NEW.orden_tramo, 'monto_desde', NEW.monto_desde, 'monto_hasta', NEW.monto_hasta, 'porcentaje', NEW.porcentaje, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_tramos_renta_del;
CREATE TRIGGER trg_aud_parametros_tramos_renta_del AFTER DELETE ON parametros_tramos_renta
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_tramos_renta', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'orden_tramo', OLD.orden_tramo, 'monto_desde', OLD.monto_desde, 'monto_hasta', OLD.monto_hasta, 'porcentaje', OLD.porcentaje, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_creditos_fiscales_renta_ins;
CREATE TRIGGER trg_aud_parametros_creditos_fiscales_renta_ins AFTER INSERT ON parametros_creditos_fiscales_renta
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_creditos_fiscales_renta', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'tipo_credito_fiscal_id', NEW.tipo_credito_fiscal_id, 'monto_credito', NEW.monto_credito, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'fuente_legal', NEW.fuente_legal, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_creditos_fiscales_renta_upd;
CREATE TRIGGER trg_aud_parametros_creditos_fiscales_renta_upd AFTER UPDATE ON parametros_creditos_fiscales_renta
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_creditos_fiscales_renta', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'tipo_credito_fiscal_id', OLD.tipo_credito_fiscal_id, 'monto_credito', OLD.monto_credito, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'fuente_legal', OLD.fuente_legal, 'es_vigente', OLD.es_vigente), JSON_OBJECT('id', NEW.id, 'tipo_credito_fiscal_id', NEW.tipo_credito_fiscal_id, 'monto_credito', NEW.monto_credito, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'fuente_legal', NEW.fuente_legal, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_creditos_fiscales_renta_del;
CREATE TRIGGER trg_aud_parametros_creditos_fiscales_renta_del AFTER DELETE ON parametros_creditos_fiscales_renta
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_creditos_fiscales_renta', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'tipo_credito_fiscal_id', OLD.tipo_credito_fiscal_id, 'monto_credito', OLD.monto_credito, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'fuente_legal', OLD.fuente_legal, 'es_vigente', OLD.es_vigente), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_tipos_embargo_ins;
CREATE TRIGGER trg_aud_parametros_tipos_embargo_ins AFTER INSERT ON parametros_tipos_embargo
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_tipos_embargo', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'prioridad_legal', NEW.prioridad_legal, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_tipos_embargo_upd;
CREATE TRIGGER trg_aud_parametros_tipos_embargo_upd AFTER UPDATE ON parametros_tipos_embargo
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_tipos_embargo', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'prioridad_legal', OLD.prioridad_legal, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'prioridad_legal', NEW.prioridad_legal, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_tipos_embargo_del;
CREATE TRIGGER trg_aud_parametros_tipos_embargo_del AFTER DELETE ON parametros_tipos_embargo
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_tipos_embargo', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'prioridad_legal', OLD.prioridad_legal, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_horas_extra_ins;
CREATE TRIGGER trg_aud_parametros_horas_extra_ins AFTER INSERT ON parametros_horas_extra
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_horas_extra', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'tipo_hora_extra_id', NEW.tipo_hora_extra_id, 'factor_recargo', NEW.factor_recargo, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_horas_extra_upd;
CREATE TRIGGER trg_aud_parametros_horas_extra_upd AFTER UPDATE ON parametros_horas_extra
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_horas_extra', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'tipo_hora_extra_id', OLD.tipo_hora_extra_id, 'factor_recargo', OLD.factor_recargo, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), JSON_OBJECT('id', NEW.id, 'tipo_hora_extra_id', NEW.tipo_hora_extra_id, 'factor_recargo', NEW.factor_recargo, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_horas_extra_del;
CREATE TRIGGER trg_aud_parametros_horas_extra_del AFTER DELETE ON parametros_horas_extra
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_horas_extra', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'tipo_hora_extra_id', OLD.tipo_hora_extra_id, 'factor_recargo', OLD.factor_recargo, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_vacaciones_escala_ins;
CREATE TRIGGER trg_aud_parametros_vacaciones_escala_ins AFTER INSERT ON parametros_vacaciones_escala
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_vacaciones_escala', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'anios_servicio_desde', NEW.anios_servicio_desde, 'anios_servicio_hasta', NEW.anios_servicio_hasta, 'dias_vacaciones', NEW.dias_vacaciones, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_vacaciones_escala_upd;
CREATE TRIGGER trg_aud_parametros_vacaciones_escala_upd AFTER UPDATE ON parametros_vacaciones_escala
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_vacaciones_escala', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'anios_servicio_desde', OLD.anios_servicio_desde, 'anios_servicio_hasta', OLD.anios_servicio_hasta, 'dias_vacaciones', OLD.dias_vacaciones, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), JSON_OBJECT('id', NEW.id, 'anios_servicio_desde', NEW.anios_servicio_desde, 'anios_servicio_hasta', NEW.anios_servicio_hasta, 'dias_vacaciones', NEW.dias_vacaciones, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_vacaciones_escala_del;
CREATE TRIGGER trg_aud_parametros_vacaciones_escala_del AFTER DELETE ON parametros_vacaciones_escala
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_vacaciones_escala', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'anios_servicio_desde', OLD.anios_servicio_desde, 'anios_servicio_hasta', OLD.anios_servicio_hasta, 'dias_vacaciones', OLD.dias_vacaciones, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_cesantia_escala_ins;
CREATE TRIGGER trg_aud_parametros_cesantia_escala_ins AFTER INSERT ON parametros_cesantia_escala
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_cesantia_escala', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'anios_servicio_desde', NEW.anios_servicio_desde, 'anios_servicio_hasta', NEW.anios_servicio_hasta, 'dias_por_anio', NEW.dias_por_anio, 'tope_anios', NEW.tope_anios, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_cesantia_escala_upd;
CREATE TRIGGER trg_aud_parametros_cesantia_escala_upd AFTER UPDATE ON parametros_cesantia_escala
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_cesantia_escala', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'anios_servicio_desde', OLD.anios_servicio_desde, 'anios_servicio_hasta', OLD.anios_servicio_hasta, 'dias_por_anio', OLD.dias_por_anio, 'tope_anios', OLD.tope_anios, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), JSON_OBJECT('id', NEW.id, 'anios_servicio_desde', NEW.anios_servicio_desde, 'anios_servicio_hasta', NEW.anios_servicio_hasta, 'dias_por_anio', NEW.dias_por_anio, 'tope_anios', NEW.tope_anios, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_cesantia_escala_del;
CREATE TRIGGER trg_aud_parametros_cesantia_escala_del AFTER DELETE ON parametros_cesantia_escala
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_cesantia_escala', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'anios_servicio_desde', OLD.anios_servicio_desde, 'anios_servicio_hasta', OLD.anios_servicio_hasta, 'dias_por_anio', OLD.dias_por_anio, 'tope_anios', OLD.tope_anios, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_base_minima_contributiva_ins;
CREATE TRIGGER trg_aud_parametros_base_minima_contributiva_ins AFTER INSERT ON parametros_base_minima_contributiva
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_base_minima_contributiva', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'regimen_ccss_id', NEW.regimen_ccss_id, 'monto', NEW.monto, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_base_minima_contributiva_upd;
CREATE TRIGGER trg_aud_parametros_base_minima_contributiva_upd AFTER UPDATE ON parametros_base_minima_contributiva
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_base_minima_contributiva', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'regimen_ccss_id', OLD.regimen_ccss_id, 'monto', OLD.monto, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), JSON_OBJECT('id', NEW.id, 'regimen_ccss_id', NEW.regimen_ccss_id, 'monto', NEW.monto, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_base_minima_contributiva_del;
CREATE TRIGGER trg_aud_parametros_base_minima_contributiva_del AFTER DELETE ON parametros_base_minima_contributiva
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_base_minima_contributiva', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'regimen_ccss_id', OLD.regimen_ccss_id, 'monto', OLD.monto, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_subsidios_incapacidad_ins;
CREATE TRIGGER trg_aud_parametros_subsidios_incapacidad_ins AFTER INSERT ON parametros_subsidios_incapacidad
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_subsidios_incapacidad', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'entidad_emisora_id', NEW.entidad_emisora_id, 'dia_desde', NEW.dia_desde, 'dia_hasta', NEW.dia_hasta, 'porcentaje_patronal', NEW.porcentaje_patronal, 'porcentaje_entidad', NEW.porcentaje_entidad, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_subsidios_incapacidad_upd;
CREATE TRIGGER trg_aud_parametros_subsidios_incapacidad_upd AFTER UPDATE ON parametros_subsidios_incapacidad
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_subsidios_incapacidad', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'entidad_emisora_id', OLD.entidad_emisora_id, 'dia_desde', OLD.dia_desde, 'dia_hasta', OLD.dia_hasta, 'porcentaje_patronal', OLD.porcentaje_patronal, 'porcentaje_entidad', OLD.porcentaje_entidad, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), JSON_OBJECT('id', NEW.id, 'entidad_emisora_id', NEW.entidad_emisora_id, 'dia_desde', NEW.dia_desde, 'dia_hasta', NEW.dia_hasta, 'porcentaje_patronal', NEW.porcentaje_patronal, 'porcentaje_entidad', NEW.porcentaje_entidad, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_subsidios_incapacidad_del;
CREATE TRIGGER trg_aud_parametros_subsidios_incapacidad_del AFTER DELETE ON parametros_subsidios_incapacidad
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_subsidios_incapacidad', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'entidad_emisora_id', OLD.entidad_emisora_id, 'dia_desde', OLD.dia_desde, 'dia_hasta', OLD.dia_hasta, 'porcentaje_patronal', OLD.porcentaje_patronal, 'porcentaje_entidad', OLD.porcentaje_entidad, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_salario_minimo_ins;
CREATE TRIGGER trg_aud_parametros_salario_minimo_ins AFTER INSERT ON parametros_salario_minimo
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_salario_minimo', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'clasificacion_puesto_id', NEW.clasificacion_puesto_id, 'monto_mensual', NEW.monto_mensual, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'fuente_legal', NEW.fuente_legal, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_salario_minimo_upd;
CREATE TRIGGER trg_aud_parametros_salario_minimo_upd AFTER UPDATE ON parametros_salario_minimo
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_salario_minimo', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'clasificacion_puesto_id', OLD.clasificacion_puesto_id, 'monto_mensual', OLD.monto_mensual, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'fuente_legal', OLD.fuente_legal, 'es_vigente', OLD.es_vigente), JSON_OBJECT('id', NEW.id, 'clasificacion_puesto_id', NEW.clasificacion_puesto_id, 'monto_mensual', NEW.monto_mensual, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'fuente_legal', NEW.fuente_legal, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_parametros_salario_minimo_del;
CREATE TRIGGER trg_aud_parametros_salario_minimo_del AFTER DELETE ON parametros_salario_minimo
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('parametros_salario_minimo', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'clasificacion_puesto_id', OLD.clasificacion_puesto_id, 'monto_mensual', OLD.monto_mensual, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'fuente_legal', OLD.fuente_legal, 'es_vigente', OLD.es_vigente), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_calendario_laboral_ins;
CREATE TRIGGER trg_aud_calendario_laboral_ins AFTER INSERT ON calendario_laboral
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('calendario_laboral', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'fecha', NEW.fecha, 'tipo_dia', NEW.tipo_dia, 'programa_presupuestario_id', NEW.programa_presupuestario_id));
END;

DROP TRIGGER IF EXISTS trg_aud_calendario_laboral_upd;
CREATE TRIGGER trg_aud_calendario_laboral_upd AFTER UPDATE ON calendario_laboral
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('calendario_laboral', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'fecha', OLD.fecha, 'tipo_dia', OLD.tipo_dia, 'programa_presupuestario_id', OLD.programa_presupuestario_id), JSON_OBJECT('id', NEW.id, 'fecha', NEW.fecha, 'tipo_dia', NEW.tipo_dia, 'programa_presupuestario_id', NEW.programa_presupuestario_id));
END;

DROP TRIGGER IF EXISTS trg_aud_calendario_laboral_del;
CREATE TRIGGER trg_aud_calendario_laboral_del AFTER DELETE ON calendario_laboral
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('calendario_laboral', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'fecha', OLD.fecha, 'tipo_dia', OLD.tipo_dia, 'programa_presupuestario_id', OLD.programa_presupuestario_id), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_feriados_patron_ins;
CREATE TRIGGER trg_aud_feriados_patron_ins AFTER INSERT ON feriados_patron
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('feriados_patron', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'nombre', NEW.nombre, 'tipo_patron', NEW.tipo_patron, 'mes', NEW.mes, 'dia', NEW.dia, 'offset_dias_pascua', NEW.offset_dias_pascua, 'aplica_traslado_lunes', NEW.aplica_traslado_lunes, 'pago_obligatorio', NEW.pago_obligatorio, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_feriados_patron_upd;
CREATE TRIGGER trg_aud_feriados_patron_upd AFTER UPDATE ON feriados_patron
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('feriados_patron', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'nombre', OLD.nombre, 'tipo_patron', OLD.tipo_patron, 'mes', OLD.mes, 'dia', OLD.dia, 'offset_dias_pascua', OLD.offset_dias_pascua, 'aplica_traslado_lunes', OLD.aplica_traslado_lunes, 'pago_obligatorio', OLD.pago_obligatorio, 'activo', OLD.activo), JSON_OBJECT('id', NEW.id, 'nombre', NEW.nombre, 'tipo_patron', NEW.tipo_patron, 'mes', NEW.mes, 'dia', NEW.dia, 'offset_dias_pascua', NEW.offset_dias_pascua, 'aplica_traslado_lunes', NEW.aplica_traslado_lunes, 'pago_obligatorio', NEW.pago_obligatorio, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_feriados_patron_del;
CREATE TRIGGER trg_aud_feriados_patron_del AFTER DELETE ON feriados_patron
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('feriados_patron', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'nombre', OLD.nombre, 'tipo_patron', OLD.tipo_patron, 'mes', OLD.mes, 'dia', OLD.dia, 'offset_dias_pascua', OLD.offset_dias_pascua, 'aplica_traslado_lunes', OLD.aplica_traslado_lunes, 'pago_obligatorio', OLD.pago_obligatorio, 'activo', OLD.activo), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_turnos_ins;
CREATE TRIGGER trg_aud_turnos_ins AFTER INSERT ON turnos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('turnos', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'hora_inicio', NEW.hora_inicio, 'hora_fin', NEW.hora_fin));
END;

DROP TRIGGER IF EXISTS trg_aud_turnos_upd;
CREATE TRIGGER trg_aud_turnos_upd AFTER UPDATE ON turnos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('turnos', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'hora_inicio', OLD.hora_inicio, 'hora_fin', OLD.hora_fin), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'hora_inicio', NEW.hora_inicio, 'hora_fin', NEW.hora_fin));
END;

DROP TRIGGER IF EXISTS trg_aud_turnos_del;
CREATE TRIGGER trg_aud_turnos_del AFTER DELETE ON turnos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('turnos', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'hora_inicio', OLD.hora_inicio, 'hora_fin', OLD.hora_fin), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_turno_ins;
CREATE TRIGGER trg_aud_empleado_turno_ins AFTER INSERT ON empleado_turno
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_turno', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'turno_id', NEW.turno_id, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_turno_upd;
CREATE TRIGGER trg_aud_empleado_turno_upd AFTER UPDATE ON empleado_turno
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_turno', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'turno_id', OLD.turno_id, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'turno_id', NEW.turno_id, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_empleado_turno_del;
CREATE TRIGGER trg_aud_empleado_turno_del AFTER DELETE ON empleado_turno
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('empleado_turno', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'turno_id', OLD.turno_id, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_registro_asistencia_ins;
CREATE TRIGGER trg_aud_registro_asistencia_ins AFTER INSERT ON registro_asistencia
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('registro_asistencia', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'fecha', NEW.fecha, 'hora_entrada', NEW.hora_entrada, 'hora_salida', NEW.hora_salida, 'horas_ordinarias', NEW.horas_ordinarias, 'horas_extra_diurnas', NEW.horas_extra_diurnas, 'horas_extra_nocturnas', NEW.horas_extra_nocturnas, 'horas_extra_mixtas', NEW.horas_extra_mixtas, 'horas_dobles', NEW.horas_dobles, 'observaciones', NEW.observaciones));
END;

DROP TRIGGER IF EXISTS trg_aud_registro_asistencia_upd;
CREATE TRIGGER trg_aud_registro_asistencia_upd AFTER UPDATE ON registro_asistencia
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('registro_asistencia', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'fecha', OLD.fecha, 'hora_entrada', OLD.hora_entrada, 'hora_salida', OLD.hora_salida, 'horas_ordinarias', OLD.horas_ordinarias, 'horas_extra_diurnas', OLD.horas_extra_diurnas, 'horas_extra_nocturnas', OLD.horas_extra_nocturnas, 'horas_extra_mixtas', OLD.horas_extra_mixtas, 'horas_dobles', OLD.horas_dobles, 'observaciones', OLD.observaciones), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'fecha', NEW.fecha, 'hora_entrada', NEW.hora_entrada, 'hora_salida', NEW.hora_salida, 'horas_ordinarias', NEW.horas_ordinarias, 'horas_extra_diurnas', NEW.horas_extra_diurnas, 'horas_extra_nocturnas', NEW.horas_extra_nocturnas, 'horas_extra_mixtas', NEW.horas_extra_mixtas, 'horas_dobles', NEW.horas_dobles, 'observaciones', NEW.observaciones));
END;

DROP TRIGGER IF EXISTS trg_aud_registro_asistencia_del;
CREATE TRIGGER trg_aud_registro_asistencia_del AFTER DELETE ON registro_asistencia
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('registro_asistencia', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'fecha', OLD.fecha, 'hora_entrada', OLD.hora_entrada, 'hora_salida', OLD.hora_salida, 'horas_ordinarias', OLD.horas_ordinarias, 'horas_extra_diurnas', OLD.horas_extra_diurnas, 'horas_extra_nocturnas', OLD.horas_extra_nocturnas, 'horas_extra_mixtas', OLD.horas_extra_mixtas, 'horas_dobles', OLD.horas_dobles, 'observaciones', OLD.observaciones), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_tipos_ausencia_ins;
CREATE TRIGGER trg_aud_tipos_ausencia_ins AFTER INSERT ON tipos_ausencia
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_ausencia', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'requiere_documento_soporte', NEW.requiere_documento_soporte));
END;

DROP TRIGGER IF EXISTS trg_aud_tipos_ausencia_upd;
CREATE TRIGGER trg_aud_tipos_ausencia_upd AFTER UPDATE ON tipos_ausencia
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_ausencia', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'requiere_documento_soporte', OLD.requiere_documento_soporte), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'requiere_documento_soporte', NEW.requiere_documento_soporte));
END;

DROP TRIGGER IF EXISTS trg_aud_tipos_ausencia_del;
CREATE TRIGGER trg_aud_tipos_ausencia_del AFTER DELETE ON tipos_ausencia
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_ausencia', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'requiere_documento_soporte', OLD.requiere_documento_soporte), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_ausencias_ins;
CREATE TRIGGER trg_aud_ausencias_ins AFTER INSERT ON ausencias
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('ausencias', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'tipo_ausencia_id', NEW.tipo_ausencia_id, 'entidad_emisora_id', NEW.entidad_emisora_id, 'fecha_inicio', NEW.fecha_inicio, 'fecha_fin', NEW.fecha_fin, 'dias', NEW.dias, 'horas', NEW.horas, 'documento_soporte_url', NEW.documento_soporte_url, 'estado', NEW.estado));
END;

DROP TRIGGER IF EXISTS trg_aud_ausencias_upd;
CREATE TRIGGER trg_aud_ausencias_upd AFTER UPDATE ON ausencias
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('ausencias', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'tipo_ausencia_id', OLD.tipo_ausencia_id, 'entidad_emisora_id', OLD.entidad_emisora_id, 'fecha_inicio', OLD.fecha_inicio, 'fecha_fin', OLD.fecha_fin, 'dias', OLD.dias, 'horas', OLD.horas, 'documento_soporte_url', OLD.documento_soporte_url, 'estado', OLD.estado), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'tipo_ausencia_id', NEW.tipo_ausencia_id, 'entidad_emisora_id', NEW.entidad_emisora_id, 'fecha_inicio', NEW.fecha_inicio, 'fecha_fin', NEW.fecha_fin, 'dias', NEW.dias, 'horas', NEW.horas, 'documento_soporte_url', NEW.documento_soporte_url, 'estado', NEW.estado));
END;

DROP TRIGGER IF EXISTS trg_aud_ausencias_del;
CREATE TRIGGER trg_aud_ausencias_del AFTER DELETE ON ausencias
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('ausencias', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'tipo_ausencia_id', OLD.tipo_ausencia_id, 'entidad_emisora_id', OLD.entidad_emisora_id, 'fecha_inicio', OLD.fecha_inicio, 'fecha_fin', OLD.fecha_fin, 'dias', OLD.dias, 'horas', OLD.horas, 'documento_soporte_url', OLD.documento_soporte_url, 'estado', OLD.estado), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_tipos_planilla_ins;
CREATE TRIGGER trg_aud_tipos_planilla_ins AFTER INSERT ON tipos_planilla
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_planilla', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre));
END;

DROP TRIGGER IF EXISTS trg_aud_tipos_planilla_upd;
CREATE TRIGGER trg_aud_tipos_planilla_upd AFTER UPDATE ON tipos_planilla
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_planilla', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre));
END;

DROP TRIGGER IF EXISTS trg_aud_tipos_planilla_del;
CREATE TRIGGER trg_aud_tipos_planilla_del AFTER DELETE ON tipos_planilla
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_planilla', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_periodos_planilla_ins;
CREATE TRIGGER trg_aud_periodos_planilla_ins AFTER INSERT ON periodos_planilla
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('periodos_planilla', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'tipo_planilla_id', NEW.tipo_planilla_id, 'fecha_inicio', NEW.fecha_inicio, 'fecha_fin', NEW.fecha_fin, 'estado', NEW.estado, 'fecha_generacion', NEW.fecha_generacion, 'preparado_por_usuario_id', NEW.preparado_por_usuario_id, 'fecha_preparacion', NEW.fecha_preparacion, 'revisado_por_usuario_id', NEW.revisado_por_usuario_id, 'fecha_revision', NEW.fecha_revision, 'fecha_aprobacion', NEW.fecha_aprobacion, 'aprobado_por', NEW.aprobado_por, 'aprobado_por_usuario_id', NEW.aprobado_por_usuario_id));
END;

DROP TRIGGER IF EXISTS trg_aud_periodos_planilla_upd;
CREATE TRIGGER trg_aud_periodos_planilla_upd AFTER UPDATE ON periodos_planilla
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('periodos_planilla', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'tipo_planilla_id', OLD.tipo_planilla_id, 'fecha_inicio', OLD.fecha_inicio, 'fecha_fin', OLD.fecha_fin, 'estado', OLD.estado, 'fecha_generacion', OLD.fecha_generacion, 'preparado_por_usuario_id', OLD.preparado_por_usuario_id, 'fecha_preparacion', OLD.fecha_preparacion, 'revisado_por_usuario_id', OLD.revisado_por_usuario_id, 'fecha_revision', OLD.fecha_revision, 'fecha_aprobacion', OLD.fecha_aprobacion, 'aprobado_por', OLD.aprobado_por, 'aprobado_por_usuario_id', OLD.aprobado_por_usuario_id), JSON_OBJECT('id', NEW.id, 'tipo_planilla_id', NEW.tipo_planilla_id, 'fecha_inicio', NEW.fecha_inicio, 'fecha_fin', NEW.fecha_fin, 'estado', NEW.estado, 'fecha_generacion', NEW.fecha_generacion, 'preparado_por_usuario_id', NEW.preparado_por_usuario_id, 'fecha_preparacion', NEW.fecha_preparacion, 'revisado_por_usuario_id', NEW.revisado_por_usuario_id, 'fecha_revision', NEW.fecha_revision, 'fecha_aprobacion', NEW.fecha_aprobacion, 'aprobado_por', NEW.aprobado_por, 'aprobado_por_usuario_id', NEW.aprobado_por_usuario_id));
END;

DROP TRIGGER IF EXISTS trg_aud_periodos_planilla_del;
CREATE TRIGGER trg_aud_periodos_planilla_del AFTER DELETE ON periodos_planilla
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('periodos_planilla', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'tipo_planilla_id', OLD.tipo_planilla_id, 'fecha_inicio', OLD.fecha_inicio, 'fecha_fin', OLD.fecha_fin, 'estado', OLD.estado, 'fecha_generacion', OLD.fecha_generacion, 'preparado_por_usuario_id', OLD.preparado_por_usuario_id, 'fecha_preparacion', OLD.fecha_preparacion, 'revisado_por_usuario_id', OLD.revisado_por_usuario_id, 'fecha_revision', OLD.fecha_revision, 'fecha_aprobacion', OLD.fecha_aprobacion, 'aprobado_por', OLD.aprobado_por, 'aprobado_por_usuario_id', OLD.aprobado_por_usuario_id), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_planilla_detalle_empleado_ins;
CREATE TRIGGER trg_aud_planilla_detalle_empleado_ins AFTER INSERT ON planilla_detalle_empleado
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_empleado', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'periodo_planilla_id', NEW.periodo_planilla_id, 'empleado_id', NEW.empleado_id, 'programa_presupuestario_id', NEW.programa_presupuestario_id, 'salario_base_periodo', NEW.salario_base_periodo, 'total_ingresos', NEW.total_ingresos, 'total_deducciones', NEW.total_deducciones, 'neto_pagado', NEW.neto_pagado, 'estado', NEW.estado));
END;

DROP TRIGGER IF EXISTS trg_aud_planilla_detalle_empleado_upd;
CREATE TRIGGER trg_aud_planilla_detalle_empleado_upd AFTER UPDATE ON planilla_detalle_empleado
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_empleado', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'periodo_planilla_id', OLD.periodo_planilla_id, 'empleado_id', OLD.empleado_id, 'programa_presupuestario_id', OLD.programa_presupuestario_id, 'salario_base_periodo', OLD.salario_base_periodo, 'total_ingresos', OLD.total_ingresos, 'total_deducciones', OLD.total_deducciones, 'neto_pagado', OLD.neto_pagado, 'estado', OLD.estado), JSON_OBJECT('id', NEW.id, 'periodo_planilla_id', NEW.periodo_planilla_id, 'empleado_id', NEW.empleado_id, 'programa_presupuestario_id', NEW.programa_presupuestario_id, 'salario_base_periodo', NEW.salario_base_periodo, 'total_ingresos', NEW.total_ingresos, 'total_deducciones', NEW.total_deducciones, 'neto_pagado', NEW.neto_pagado, 'estado', NEW.estado));
END;

DROP TRIGGER IF EXISTS trg_aud_planilla_detalle_empleado_del;
CREATE TRIGGER trg_aud_planilla_detalle_empleado_del AFTER DELETE ON planilla_detalle_empleado
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_empleado', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'periodo_planilla_id', OLD.periodo_planilla_id, 'empleado_id', OLD.empleado_id, 'programa_presupuestario_id', OLD.programa_presupuestario_id, 'salario_base_periodo', OLD.salario_base_periodo, 'total_ingresos', OLD.total_ingresos, 'total_deducciones', OLD.total_deducciones, 'neto_pagado', OLD.neto_pagado, 'estado', OLD.estado), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_planilla_detalle_concepto_ins;
CREATE TRIGGER trg_aud_planilla_detalle_concepto_ins AFTER INSERT ON planilla_detalle_concepto
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_concepto', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'planilla_detalle_empleado_id', NEW.planilla_detalle_empleado_id, 'concepto_salarial_id', NEW.concepto_salarial_id, 'concepto_salarial_formula_id', NEW.concepto_salarial_formula_id, 'cantidad', NEW.cantidad, 'monto', NEW.monto, 'concepto_ajuste_de_id', NEW.concepto_ajuste_de_id, 'motivo_ajuste', NEW.motivo_ajuste));
END;

DROP TRIGGER IF EXISTS trg_aud_planilla_detalle_concepto_upd;
CREATE TRIGGER trg_aud_planilla_detalle_concepto_upd AFTER UPDATE ON planilla_detalle_concepto
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_concepto', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'planilla_detalle_empleado_id', OLD.planilla_detalle_empleado_id, 'concepto_salarial_id', OLD.concepto_salarial_id, 'concepto_salarial_formula_id', OLD.concepto_salarial_formula_id, 'cantidad', OLD.cantidad, 'monto', OLD.monto, 'concepto_ajuste_de_id', OLD.concepto_ajuste_de_id, 'motivo_ajuste', OLD.motivo_ajuste), JSON_OBJECT('id', NEW.id, 'planilla_detalle_empleado_id', NEW.planilla_detalle_empleado_id, 'concepto_salarial_id', NEW.concepto_salarial_id, 'concepto_salarial_formula_id', NEW.concepto_salarial_formula_id, 'cantidad', NEW.cantidad, 'monto', NEW.monto, 'concepto_ajuste_de_id', NEW.concepto_ajuste_de_id, 'motivo_ajuste', NEW.motivo_ajuste));
END;

DROP TRIGGER IF EXISTS trg_aud_planilla_detalle_concepto_del;
CREATE TRIGGER trg_aud_planilla_detalle_concepto_del AFTER DELETE ON planilla_detalle_concepto
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_concepto', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'planilla_detalle_empleado_id', OLD.planilla_detalle_empleado_id, 'concepto_salarial_id', OLD.concepto_salarial_id, 'concepto_salarial_formula_id', OLD.concepto_salarial_formula_id, 'cantidad', OLD.cantidad, 'monto', OLD.monto, 'concepto_ajuste_de_id', OLD.concepto_ajuste_de_id, 'motivo_ajuste', OLD.motivo_ajuste), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_vacaciones_acumulado_ins;
CREATE TRIGGER trg_aud_vacaciones_acumulado_ins AFTER INSERT ON vacaciones_acumulado
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('vacaciones_acumulado', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'fecha_corte', NEW.fecha_corte, 'dias_acumulados', NEW.dias_acumulados, 'dias_disfrutados', NEW.dias_disfrutados, 'saldo', NEW.saldo, 'origen_carga', NEW.origen_carga, 'registrado_por_usuario_id', NEW.registrado_por_usuario_id));
END;

DROP TRIGGER IF EXISTS trg_aud_vacaciones_acumulado_upd;
CREATE TRIGGER trg_aud_vacaciones_acumulado_upd AFTER UPDATE ON vacaciones_acumulado
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('vacaciones_acumulado', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'fecha_corte', OLD.fecha_corte, 'dias_acumulados', OLD.dias_acumulados, 'dias_disfrutados', OLD.dias_disfrutados, 'saldo', OLD.saldo, 'origen_carga', OLD.origen_carga, 'registrado_por_usuario_id', OLD.registrado_por_usuario_id), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'fecha_corte', NEW.fecha_corte, 'dias_acumulados', NEW.dias_acumulados, 'dias_disfrutados', NEW.dias_disfrutados, 'saldo', NEW.saldo, 'origen_carga', NEW.origen_carga, 'registrado_por_usuario_id', NEW.registrado_por_usuario_id));
END;

DROP TRIGGER IF EXISTS trg_aud_vacaciones_acumulado_del;
CREATE TRIGGER trg_aud_vacaciones_acumulado_del AFTER DELETE ON vacaciones_acumulado
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('vacaciones_acumulado', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'fecha_corte', OLD.fecha_corte, 'dias_acumulados', OLD.dias_acumulados, 'dias_disfrutados', OLD.dias_disfrutados, 'saldo', OLD.saldo, 'origen_carga', OLD.origen_carga, 'registrado_por_usuario_id', OLD.registrado_por_usuario_id), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_vacaciones_disfrute_ins;
CREATE TRIGGER trg_aud_vacaciones_disfrute_ins AFTER INSERT ON vacaciones_disfrute
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('vacaciones_disfrute', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'fecha_inicio', NEW.fecha_inicio, 'fecha_fin', NEW.fecha_fin, 'dias', NEW.dias));
END;

DROP TRIGGER IF EXISTS trg_aud_vacaciones_disfrute_upd;
CREATE TRIGGER trg_aud_vacaciones_disfrute_upd AFTER UPDATE ON vacaciones_disfrute
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('vacaciones_disfrute', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'fecha_inicio', OLD.fecha_inicio, 'fecha_fin', OLD.fecha_fin, 'dias', OLD.dias), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'fecha_inicio', NEW.fecha_inicio, 'fecha_fin', NEW.fecha_fin, 'dias', NEW.dias));
END;

DROP TRIGGER IF EXISTS trg_aud_vacaciones_disfrute_del;
CREATE TRIGGER trg_aud_vacaciones_disfrute_del AFTER DELETE ON vacaciones_disfrute
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('vacaciones_disfrute', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'fecha_inicio', OLD.fecha_inicio, 'fecha_fin', OLD.fecha_fin, 'dias', OLD.dias), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_terceros_ins;
CREATE TRIGGER trg_aud_terceros_ins AFTER INSERT ON terceros
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('terceros', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'nombre', NEW.nombre, 'tipo', NEW.tipo));
END;

DROP TRIGGER IF EXISTS trg_aud_terceros_upd;
CREATE TRIGGER trg_aud_terceros_upd AFTER UPDATE ON terceros
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('terceros', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'nombre', OLD.nombre, 'tipo', OLD.tipo), JSON_OBJECT('id', NEW.id, 'nombre', NEW.nombre, 'tipo', NEW.tipo));
END;

DROP TRIGGER IF EXISTS trg_aud_terceros_del;
CREATE TRIGGER trg_aud_terceros_del AFTER DELETE ON terceros
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('terceros', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'nombre', OLD.nombre, 'tipo', OLD.tipo), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_prestamos_ins;
CREATE TRIGGER trg_aud_prestamos_ins AFTER INSERT ON prestamos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('prestamos', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'tercero_id', NEW.tercero_id, 'monto_total', NEW.monto_total, 'cuota_periodica', NEW.cuota_periodica, 'saldo_pendiente', NEW.saldo_pendiente, 'fecha_inicio', NEW.fecha_inicio, 'fecha_fin_estimada', NEW.fecha_fin_estimada, 'estado', NEW.estado, 'origen_carga', NEW.origen_carga, 'registrado_por_usuario_id', NEW.registrado_por_usuario_id));
END;

DROP TRIGGER IF EXISTS trg_aud_prestamos_upd;
CREATE TRIGGER trg_aud_prestamos_upd AFTER UPDATE ON prestamos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('prestamos', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'tercero_id', OLD.tercero_id, 'monto_total', OLD.monto_total, 'cuota_periodica', OLD.cuota_periodica, 'saldo_pendiente', OLD.saldo_pendiente, 'fecha_inicio', OLD.fecha_inicio, 'fecha_fin_estimada', OLD.fecha_fin_estimada, 'estado', OLD.estado, 'origen_carga', OLD.origen_carga, 'registrado_por_usuario_id', OLD.registrado_por_usuario_id), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'tercero_id', NEW.tercero_id, 'monto_total', NEW.monto_total, 'cuota_periodica', NEW.cuota_periodica, 'saldo_pendiente', NEW.saldo_pendiente, 'fecha_inicio', NEW.fecha_inicio, 'fecha_fin_estimada', NEW.fecha_fin_estimada, 'estado', NEW.estado, 'origen_carga', NEW.origen_carga, 'registrado_por_usuario_id', NEW.registrado_por_usuario_id));
END;

DROP TRIGGER IF EXISTS trg_aud_prestamos_del;
CREATE TRIGGER trg_aud_prestamos_del AFTER DELETE ON prestamos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('prestamos', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'tercero_id', OLD.tercero_id, 'monto_total', OLD.monto_total, 'cuota_periodica', OLD.cuota_periodica, 'saldo_pendiente', OLD.saldo_pendiente, 'fecha_inicio', OLD.fecha_inicio, 'fecha_fin_estimada', OLD.fecha_fin_estimada, 'estado', OLD.estado, 'origen_carga', OLD.origen_carga, 'registrado_por_usuario_id', OLD.registrado_por_usuario_id), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_embargos_ins;
CREATE TRIGGER trg_aud_embargos_ins AFTER INSERT ON embargos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('embargos', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'tercero_id', NEW.tercero_id, 'tipo_embargo_id', NEW.tipo_embargo_id, 'numero_expediente', NEW.numero_expediente, 'porcentaje', NEW.porcentaje, 'monto_fijo', NEW.monto_fijo, 'prioridad_aplicacion', NEW.prioridad_aplicacion, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'estado', NEW.estado));
END;

DROP TRIGGER IF EXISTS trg_aud_embargos_upd;
CREATE TRIGGER trg_aud_embargos_upd AFTER UPDATE ON embargos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('embargos', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'tercero_id', OLD.tercero_id, 'tipo_embargo_id', OLD.tipo_embargo_id, 'numero_expediente', OLD.numero_expediente, 'porcentaje', OLD.porcentaje, 'monto_fijo', OLD.monto_fijo, 'prioridad_aplicacion', OLD.prioridad_aplicacion, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'estado', OLD.estado), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'tercero_id', NEW.tercero_id, 'tipo_embargo_id', NEW.tipo_embargo_id, 'numero_expediente', NEW.numero_expediente, 'porcentaje', NEW.porcentaje, 'monto_fijo', NEW.monto_fijo, 'prioridad_aplicacion', NEW.prioridad_aplicacion, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'estado', NEW.estado));
END;

DROP TRIGGER IF EXISTS trg_aud_embargos_del;
CREATE TRIGGER trg_aud_embargos_del AFTER DELETE ON embargos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('embargos', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'tercero_id', OLD.tercero_id, 'tipo_embargo_id', OLD.tipo_embargo_id, 'numero_expediente', OLD.numero_expediente, 'porcentaje', OLD.porcentaje, 'monto_fijo', OLD.monto_fijo, 'prioridad_aplicacion', OLD.prioridad_aplicacion, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'estado', OLD.estado), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_cuentas_contables_ins;
CREATE TRIGGER trg_aud_cuentas_contables_ins AFTER INSERT ON cuentas_contables
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('cuentas_contables', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'cuenta_padre_id', NEW.cuenta_padre_id, 'nivel', NEW.nivel, 'naturaleza', NEW.naturaleza, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_cuentas_contables_upd;
CREATE TRIGGER trg_aud_cuentas_contables_upd AFTER UPDATE ON cuentas_contables
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('cuentas_contables', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'cuenta_padre_id', OLD.cuenta_padre_id, 'nivel', OLD.nivel, 'naturaleza', OLD.naturaleza, 'activo', OLD.activo), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'cuenta_padre_id', NEW.cuenta_padre_id, 'nivel', NEW.nivel, 'naturaleza', NEW.naturaleza, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_cuentas_contables_del;
CREATE TRIGGER trg_aud_cuentas_contables_del AFTER DELETE ON cuentas_contables
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('cuentas_contables', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'cuenta_padre_id', OLD.cuenta_padre_id, 'nivel', OLD.nivel, 'naturaleza', OLD.naturaleza, 'activo', OLD.activo), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_tipos_asiento_ins;
CREATE TRIGGER trg_aud_tipos_asiento_ins AFTER INSERT ON tipos_asiento
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_asiento', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'descripcion', NEW.descripcion));
END;

DROP TRIGGER IF EXISTS trg_aud_tipos_asiento_upd;
CREATE TRIGGER trg_aud_tipos_asiento_upd AFTER UPDATE ON tipos_asiento
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_asiento', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'descripcion', OLD.descripcion), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'descripcion', NEW.descripcion));
END;

DROP TRIGGER IF EXISTS trg_aud_tipos_asiento_del;
CREATE TRIGGER trg_aud_tipos_asiento_del AFTER DELETE ON tipos_asiento
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('tipos_asiento', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'descripcion', OLD.descripcion), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_plantillas_asiento_linea_ins;
CREATE TRIGGER trg_aud_plantillas_asiento_linea_ins AFTER INSERT ON plantillas_asiento_linea
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('plantillas_asiento_linea', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'tipo_asiento_id', NEW.tipo_asiento_id, 'descripcion_linea', NEW.descripcion_linea, 'tipo_movimiento', NEW.tipo_movimiento, 'orden', NEW.orden));
END;

DROP TRIGGER IF EXISTS trg_aud_plantillas_asiento_linea_upd;
CREATE TRIGGER trg_aud_plantillas_asiento_linea_upd AFTER UPDATE ON plantillas_asiento_linea
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('plantillas_asiento_linea', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'tipo_asiento_id', OLD.tipo_asiento_id, 'descripcion_linea', OLD.descripcion_linea, 'tipo_movimiento', OLD.tipo_movimiento, 'orden', OLD.orden), JSON_OBJECT('id', NEW.id, 'tipo_asiento_id', NEW.tipo_asiento_id, 'descripcion_linea', NEW.descripcion_linea, 'tipo_movimiento', NEW.tipo_movimiento, 'orden', NEW.orden));
END;

DROP TRIGGER IF EXISTS trg_aud_plantillas_asiento_linea_del;
CREATE TRIGGER trg_aud_plantillas_asiento_linea_del AFTER DELETE ON plantillas_asiento_linea
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('plantillas_asiento_linea', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'tipo_asiento_id', OLD.tipo_asiento_id, 'descripcion_linea', OLD.descripcion_linea, 'tipo_movimiento', OLD.tipo_movimiento, 'orden', OLD.orden), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_mapeo_concepto_cuenta_ins;
CREATE TRIGGER trg_aud_mapeo_concepto_cuenta_ins AFTER INSERT ON mapeo_concepto_cuenta
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('mapeo_concepto_cuenta', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'concepto_salarial_id', NEW.concepto_salarial_id, 'programa_presupuestario_id', NEW.programa_presupuestario_id, 'tipo_asiento_id', NEW.tipo_asiento_id, 'cuenta_contable_id', NEW.cuenta_contable_id, 'tipo_movimiento', NEW.tipo_movimiento, 'proyecto', NEW.proyecto, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_mapeo_concepto_cuenta_upd;
CREATE TRIGGER trg_aud_mapeo_concepto_cuenta_upd AFTER UPDATE ON mapeo_concepto_cuenta
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('mapeo_concepto_cuenta', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'concepto_salarial_id', OLD.concepto_salarial_id, 'programa_presupuestario_id', OLD.programa_presupuestario_id, 'tipo_asiento_id', OLD.tipo_asiento_id, 'cuenta_contable_id', OLD.cuenta_contable_id, 'tipo_movimiento', OLD.tipo_movimiento, 'proyecto', OLD.proyecto, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), JSON_OBJECT('id', NEW.id, 'concepto_salarial_id', NEW.concepto_salarial_id, 'programa_presupuestario_id', NEW.programa_presupuestario_id, 'tipo_asiento_id', NEW.tipo_asiento_id, 'cuenta_contable_id', NEW.cuenta_contable_id, 'tipo_movimiento', NEW.tipo_movimiento, 'proyecto', NEW.proyecto, 'fecha_inicio_vigencia', NEW.fecha_inicio_vigencia, 'fecha_fin_vigencia', NEW.fecha_fin_vigencia, 'es_vigente', NEW.es_vigente));
END;

DROP TRIGGER IF EXISTS trg_aud_mapeo_concepto_cuenta_del;
CREATE TRIGGER trg_aud_mapeo_concepto_cuenta_del AFTER DELETE ON mapeo_concepto_cuenta
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('mapeo_concepto_cuenta', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'concepto_salarial_id', OLD.concepto_salarial_id, 'programa_presupuestario_id', OLD.programa_presupuestario_id, 'tipo_asiento_id', OLD.tipo_asiento_id, 'cuenta_contable_id', OLD.cuenta_contable_id, 'tipo_movimiento', OLD.tipo_movimiento, 'proyecto', OLD.proyecto, 'fecha_inicio_vigencia', OLD.fecha_inicio_vigencia, 'fecha_fin_vigencia', OLD.fecha_fin_vigencia, 'es_vigente', OLD.es_vigente), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_asientos_contables_ins;
CREATE TRIGGER trg_aud_asientos_contables_ins AFTER INSERT ON asientos_contables
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('asientos_contables', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'periodo_planilla_id', NEW.periodo_planilla_id, 'tipo_asiento_id', NEW.tipo_asiento_id, 'numero_asiento', NEW.numero_asiento, 'fecha_generacion', NEW.fecha_generacion, 'estado', NEW.estado));
END;

DROP TRIGGER IF EXISTS trg_aud_asientos_contables_upd;
CREATE TRIGGER trg_aud_asientos_contables_upd AFTER UPDATE ON asientos_contables
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('asientos_contables', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'periodo_planilla_id', OLD.periodo_planilla_id, 'tipo_asiento_id', OLD.tipo_asiento_id, 'numero_asiento', OLD.numero_asiento, 'fecha_generacion', OLD.fecha_generacion, 'estado', OLD.estado), JSON_OBJECT('id', NEW.id, 'periodo_planilla_id', NEW.periodo_planilla_id, 'tipo_asiento_id', NEW.tipo_asiento_id, 'numero_asiento', NEW.numero_asiento, 'fecha_generacion', NEW.fecha_generacion, 'estado', NEW.estado));
END;

DROP TRIGGER IF EXISTS trg_aud_asientos_contables_del;
CREATE TRIGGER trg_aud_asientos_contables_del AFTER DELETE ON asientos_contables
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('asientos_contables', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'periodo_planilla_id', OLD.periodo_planilla_id, 'tipo_asiento_id', OLD.tipo_asiento_id, 'numero_asiento', OLD.numero_asiento, 'fecha_generacion', OLD.fecha_generacion, 'estado', OLD.estado), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_asientos_contables_detalle_ins;
CREATE TRIGGER trg_aud_asientos_contables_detalle_ins AFTER INSERT ON asientos_contables_detalle
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('asientos_contables_detalle', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'asiento_contable_id', NEW.asiento_contable_id, 'cuenta_contable_id', NEW.cuenta_contable_id, 'programa_presupuestario_id', NEW.programa_presupuestario_id, 'proyecto', NEW.proyecto, 'empleado_id', NEW.empleado_id, 'concepto_salarial_id', NEW.concepto_salarial_id, 'monto_debe', NEW.monto_debe, 'monto_haber', NEW.monto_haber));
END;

DROP TRIGGER IF EXISTS trg_aud_asientos_contables_detalle_upd;
CREATE TRIGGER trg_aud_asientos_contables_detalle_upd AFTER UPDATE ON asientos_contables_detalle
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('asientos_contables_detalle', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'asiento_contable_id', OLD.asiento_contable_id, 'cuenta_contable_id', OLD.cuenta_contable_id, 'programa_presupuestario_id', OLD.programa_presupuestario_id, 'proyecto', OLD.proyecto, 'empleado_id', OLD.empleado_id, 'concepto_salarial_id', OLD.concepto_salarial_id, 'monto_debe', OLD.monto_debe, 'monto_haber', OLD.monto_haber), JSON_OBJECT('id', NEW.id, 'asiento_contable_id', NEW.asiento_contable_id, 'cuenta_contable_id', NEW.cuenta_contable_id, 'programa_presupuestario_id', NEW.programa_presupuestario_id, 'proyecto', NEW.proyecto, 'empleado_id', NEW.empleado_id, 'concepto_salarial_id', NEW.concepto_salarial_id, 'monto_debe', NEW.monto_debe, 'monto_haber', NEW.monto_haber));
END;

DROP TRIGGER IF EXISTS trg_aud_asientos_contables_detalle_del;
CREATE TRIGGER trg_aud_asientos_contables_detalle_del AFTER DELETE ON asientos_contables_detalle
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('asientos_contables_detalle', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'asiento_contable_id', OLD.asiento_contable_id, 'cuenta_contable_id', OLD.cuenta_contable_id, 'programa_presupuestario_id', OLD.programa_presupuestario_id, 'proyecto', OLD.proyecto, 'empleado_id', OLD.empleado_id, 'concepto_salarial_id', OLD.concepto_salarial_id, 'monto_debe', OLD.monto_debe, 'monto_haber', OLD.monto_haber), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_sistemas_origen_ins;
CREATE TRIGGER trg_aud_sistemas_origen_ins AFTER INSERT ON sistemas_origen
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('sistemas_origen', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre));
END;

DROP TRIGGER IF EXISTS trg_aud_sistemas_origen_upd;
CREATE TRIGGER trg_aud_sistemas_origen_upd AFTER UPDATE ON sistemas_origen
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('sistemas_origen', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre));
END;

DROP TRIGGER IF EXISTS trg_aud_sistemas_origen_del;
CREATE TRIGGER trg_aud_sistemas_origen_del AFTER DELETE ON sistemas_origen
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('sistemas_origen', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_interfaces_importacion_ins;
CREATE TRIGGER trg_aud_interfaces_importacion_ins AFTER INSERT ON interfaces_importacion
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('interfaces_importacion', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'sistema_origen_id', NEW.sistema_origen_id, 'nombre', NEW.nombre, 'formato_archivo', NEW.formato_archivo, 'configuracion_mapeo', NEW.configuracion_mapeo, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_interfaces_importacion_upd;
CREATE TRIGGER trg_aud_interfaces_importacion_upd AFTER UPDATE ON interfaces_importacion
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('interfaces_importacion', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'sistema_origen_id', OLD.sistema_origen_id, 'nombre', OLD.nombre, 'formato_archivo', OLD.formato_archivo, 'configuracion_mapeo', OLD.configuracion_mapeo, 'activo', OLD.activo), JSON_OBJECT('id', NEW.id, 'sistema_origen_id', NEW.sistema_origen_id, 'nombre', NEW.nombre, 'formato_archivo', NEW.formato_archivo, 'configuracion_mapeo', NEW.configuracion_mapeo, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_interfaces_importacion_del;
CREATE TRIGGER trg_aud_interfaces_importacion_del AFTER DELETE ON interfaces_importacion
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('interfaces_importacion', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'sistema_origen_id', OLD.sistema_origen_id, 'nombre', OLD.nombre, 'formato_archivo', OLD.formato_archivo, 'configuracion_mapeo', OLD.configuracion_mapeo, 'activo', OLD.activo), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_importaciones_staging_ins;
CREATE TRIGGER trg_aud_importaciones_staging_ins AFTER INSERT ON importaciones_staging
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('importaciones_staging', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'interfaz_importacion_id', NEW.interfaz_importacion_id, 'nombre_archivo_origen', NEW.nombre_archivo_origen, 'fecha_carga', NEW.fecha_carga, 'estado', NEW.estado, 'contenido_raw', NEW.contenido_raw, 'errores_validacion', NEW.errores_validacion));
END;

DROP TRIGGER IF EXISTS trg_aud_importaciones_staging_upd;
CREATE TRIGGER trg_aud_importaciones_staging_upd AFTER UPDATE ON importaciones_staging
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('importaciones_staging', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'interfaz_importacion_id', OLD.interfaz_importacion_id, 'nombre_archivo_origen', OLD.nombre_archivo_origen, 'fecha_carga', OLD.fecha_carga, 'estado', OLD.estado, 'contenido_raw', OLD.contenido_raw, 'errores_validacion', OLD.errores_validacion), JSON_OBJECT('id', NEW.id, 'interfaz_importacion_id', NEW.interfaz_importacion_id, 'nombre_archivo_origen', NEW.nombre_archivo_origen, 'fecha_carga', NEW.fecha_carga, 'estado', NEW.estado, 'contenido_raw', NEW.contenido_raw, 'errores_validacion', NEW.errores_validacion));
END;

DROP TRIGGER IF EXISTS trg_aud_importaciones_staging_del;
CREATE TRIGGER trg_aud_importaciones_staging_del AFTER DELETE ON importaciones_staging
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('importaciones_staging', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'interfaz_importacion_id', OLD.interfaz_importacion_id, 'nombre_archivo_origen', OLD.nombre_archivo_origen, 'fecha_carga', OLD.fecha_carga, 'estado', OLD.estado, 'contenido_raw', OLD.contenido_raw, 'errores_validacion', OLD.errores_validacion), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_historico_salarios_pre_migracion_ins;
CREATE TRIGGER trg_aud_historico_salarios_pre_migracion_ins AFTER INSERT ON historico_salarios_pre_migracion
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('historico_salarios_pre_migracion', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'anio', NEW.anio, 'mes', NEW.mes, 'concepto_salarial_id', NEW.concepto_salarial_id, 'monto', NEW.monto, 'fuente', NEW.fuente, 'cargado_por_usuario_id', NEW.cargado_por_usuario_id, 'fecha_carga', NEW.fecha_carga));
END;

DROP TRIGGER IF EXISTS trg_aud_historico_salarios_pre_migracion_upd;
CREATE TRIGGER trg_aud_historico_salarios_pre_migracion_upd AFTER UPDATE ON historico_salarios_pre_migracion
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('historico_salarios_pre_migracion', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'anio', OLD.anio, 'mes', OLD.mes, 'concepto_salarial_id', OLD.concepto_salarial_id, 'monto', OLD.monto, 'fuente', OLD.fuente, 'cargado_por_usuario_id', OLD.cargado_por_usuario_id, 'fecha_carga', OLD.fecha_carga), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'anio', NEW.anio, 'mes', NEW.mes, 'concepto_salarial_id', NEW.concepto_salarial_id, 'monto', NEW.monto, 'fuente', NEW.fuente, 'cargado_por_usuario_id', NEW.cargado_por_usuario_id, 'fecha_carga', NEW.fecha_carga));
END;

DROP TRIGGER IF EXISTS trg_aud_historico_salarios_pre_migracion_del;
CREATE TRIGGER trg_aud_historico_salarios_pre_migracion_del AFTER DELETE ON historico_salarios_pre_migracion
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('historico_salarios_pre_migracion', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'anio', OLD.anio, 'mes', OLD.mes, 'concepto_salarial_id', OLD.concepto_salarial_id, 'monto', OLD.monto, 'fuente', OLD.fuente, 'cargado_por_usuario_id', OLD.cargado_por_usuario_id, 'fecha_carga', OLD.fecha_carga), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_formatos_exportacion_ins;
CREATE TRIGGER trg_aud_formatos_exportacion_ins AFTER INSERT ON formatos_exportacion
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('formatos_exportacion', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'tipo_destino', NEW.tipo_destino));
END;

DROP TRIGGER IF EXISTS trg_aud_formatos_exportacion_upd;
CREATE TRIGGER trg_aud_formatos_exportacion_upd AFTER UPDATE ON formatos_exportacion
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('formatos_exportacion', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'tipo_destino', OLD.tipo_destino), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'tipo_destino', NEW.tipo_destino));
END;

DROP TRIGGER IF EXISTS trg_aud_formatos_exportacion_del;
CREATE TRIGGER trg_aud_formatos_exportacion_del AFTER DELETE ON formatos_exportacion
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('formatos_exportacion', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'tipo_destino', OLD.tipo_destino), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_exportaciones_generadas_ins;
CREATE TRIGGER trg_aud_exportaciones_generadas_ins AFTER INSERT ON exportaciones_generadas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('exportaciones_generadas', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'periodo_planilla_id', NEW.periodo_planilla_id, 'formato_exportacion_id', NEW.formato_exportacion_id, 'fecha_generacion', NEW.fecha_generacion, 'ruta_archivo', NEW.ruta_archivo, 'hash_verificacion', NEW.hash_verificacion, 'generado_por_usuario_id', NEW.generado_por_usuario_id));
END;

DROP TRIGGER IF EXISTS trg_aud_exportaciones_generadas_upd;
CREATE TRIGGER trg_aud_exportaciones_generadas_upd AFTER UPDATE ON exportaciones_generadas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('exportaciones_generadas', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'periodo_planilla_id', OLD.periodo_planilla_id, 'formato_exportacion_id', OLD.formato_exportacion_id, 'fecha_generacion', OLD.fecha_generacion, 'ruta_archivo', OLD.ruta_archivo, 'hash_verificacion', OLD.hash_verificacion, 'generado_por_usuario_id', OLD.generado_por_usuario_id), JSON_OBJECT('id', NEW.id, 'periodo_planilla_id', NEW.periodo_planilla_id, 'formato_exportacion_id', NEW.formato_exportacion_id, 'fecha_generacion', NEW.fecha_generacion, 'ruta_archivo', NEW.ruta_archivo, 'hash_verificacion', NEW.hash_verificacion, 'generado_por_usuario_id', NEW.generado_por_usuario_id));
END;

DROP TRIGGER IF EXISTS trg_aud_exportaciones_generadas_del;
CREATE TRIGGER trg_aud_exportaciones_generadas_del AFTER DELETE ON exportaciones_generadas
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('exportaciones_generadas', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'periodo_planilla_id', OLD.periodo_planilla_id, 'formato_exportacion_id', OLD.formato_exportacion_id, 'fecha_generacion', OLD.fecha_generacion, 'ruta_archivo', OLD.ruta_archivo, 'hash_verificacion', OLD.hash_verificacion, 'generado_por_usuario_id', OLD.generado_por_usuario_id), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_usuarios_ins;
CREATE TRIGGER trg_aud_usuarios_ins AFTER INSERT ON usuarios
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('usuarios', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'nombre_usuario', NEW.nombre_usuario, 'nombre_completo', NEW.nombre_completo, 'correo', NEW.correo, 'password_hash', '***REDACTED***', 'empleado_id', NEW.empleado_id, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_usuarios_upd;
CREATE TRIGGER trg_aud_usuarios_upd AFTER UPDATE ON usuarios
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('usuarios', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'nombre_usuario', OLD.nombre_usuario, 'nombre_completo', OLD.nombre_completo, 'correo', OLD.correo, 'password_hash', '***REDACTED***', 'empleado_id', OLD.empleado_id, 'activo', OLD.activo), JSON_OBJECT('id', NEW.id, 'nombre_usuario', NEW.nombre_usuario, 'nombre_completo', NEW.nombre_completo, 'correo', NEW.correo, 'password_hash', '***REDACTED***', 'empleado_id', NEW.empleado_id, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_usuarios_del;
CREATE TRIGGER trg_aud_usuarios_del AFTER DELETE ON usuarios
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('usuarios', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'nombre_usuario', OLD.nombre_usuario, 'nombre_completo', OLD.nombre_completo, 'correo', OLD.correo, 'password_hash', '***REDACTED***', 'empleado_id', OLD.empleado_id, 'activo', OLD.activo), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_roles_ins;
CREATE TRIGGER trg_aud_roles_ins AFTER INSERT ON roles
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('roles', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre));
END;

DROP TRIGGER IF EXISTS trg_aud_roles_upd;
CREATE TRIGGER trg_aud_roles_upd AFTER UPDATE ON roles
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('roles', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre));
END;

DROP TRIGGER IF EXISTS trg_aud_roles_del;
CREATE TRIGGER trg_aud_roles_del AFTER DELETE ON roles
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('roles', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_permisos_ins;
CREATE TRIGGER trg_aud_permisos_ins AFTER INSERT ON permisos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('permisos', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'modulo', NEW.modulo));
END;

DROP TRIGGER IF EXISTS trg_aud_permisos_upd;
CREATE TRIGGER trg_aud_permisos_upd AFTER UPDATE ON permisos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('permisos', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'modulo', OLD.modulo), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre, 'modulo', NEW.modulo));
END;

DROP TRIGGER IF EXISTS trg_aud_permisos_del;
CREATE TRIGGER trg_aud_permisos_del AFTER DELETE ON permisos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('permisos', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre, 'modulo', OLD.modulo), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_rol_permisos_ins;
CREATE TRIGGER trg_aud_rol_permisos_ins AFTER INSERT ON rol_permisos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('rol_permisos', 0, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('rol_id', NEW.rol_id, 'permiso_id', NEW.permiso_id));
END;

DROP TRIGGER IF EXISTS trg_aud_rol_permisos_upd;
CREATE TRIGGER trg_aud_rol_permisos_upd AFTER UPDATE ON rol_permisos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('rol_permisos', 0, @auditoria_usuario_id, 'update', JSON_OBJECT('rol_id', OLD.rol_id, 'permiso_id', OLD.permiso_id), JSON_OBJECT('rol_id', NEW.rol_id, 'permiso_id', NEW.permiso_id));
END;

DROP TRIGGER IF EXISTS trg_aud_rol_permisos_del;
CREATE TRIGGER trg_aud_rol_permisos_del AFTER DELETE ON rol_permisos
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('rol_permisos', 0, @auditoria_usuario_id, 'delete', JSON_OBJECT('rol_id', OLD.rol_id, 'permiso_id', OLD.permiso_id), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_usuario_roles_ins;
CREATE TRIGGER trg_aud_usuario_roles_ins AFTER INSERT ON usuario_roles
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('usuario_roles', 0, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('usuario_id', NEW.usuario_id, 'rol_id', NEW.rol_id));
END;

DROP TRIGGER IF EXISTS trg_aud_usuario_roles_upd;
CREATE TRIGGER trg_aud_usuario_roles_upd AFTER UPDATE ON usuario_roles
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('usuario_roles', 0, @auditoria_usuario_id, 'update', JSON_OBJECT('usuario_id', OLD.usuario_id, 'rol_id', OLD.rol_id), JSON_OBJECT('usuario_id', NEW.usuario_id, 'rol_id', NEW.rol_id));
END;

DROP TRIGGER IF EXISTS trg_aud_usuario_roles_del;
CREATE TRIGGER trg_aud_usuario_roles_del AFTER DELETE ON usuario_roles
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('usuario_roles', 0, @auditoria_usuario_id, 'delete', JSON_OBJECT('usuario_id', OLD.usuario_id, 'rol_id', OLD.rol_id), NULL);
END;

SET FOREIGN_KEY_CHECKS = 1;

