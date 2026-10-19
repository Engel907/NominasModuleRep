-- 000120: DDL de persistencia del cálculo de planilla (L1)

ALTER TABLE periodos_planilla
  ADD COLUMN parametros_snapshot JSON NULL COMMENT 'Parámetros resueltos (en fracción) usados en el último cálculo',
  ADD COLUMN version_motor VARCHAR(40) NULL COMMENT 'Versión del motor que calculó el período';

ALTER TABLE planilla_detalle_empleado
  ADD COLUMN contrato_id BIGINT UNSIGNED NULL COMMENT 'Contrato principal vigente al calcular',
  ADD COLUMN base_ccss DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Base de cotización CCSS usada',
  ADD COLUMN base_renta DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Base de renta usada',
  ADD COLUMN base_aguinaldo DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Acumula para aguinaldo',
  ADD COLUMN base_cesantia DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Acumula para cesantía',
  ADD COLUMN base_vacaciones DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Acumula para vacaciones',
  ADD COLUMN cuota_obrera_ccss DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Cuota obrera CCSS retenida',
  ADD COLUMN cuota_patronal_ccss DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Cuota patronal total (persistida, D20)',
  ADD COLUMN renta_bruta DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Impuesto de renta antes de créditos',
  ADD COLUMN creditos_fiscales_aplicados DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Créditos fiscales efectivamente aplicados',
  ADD COLUMN renta_neta DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Renta retenida',
  ADD COLUMN advertencias JSON NULL COMMENT 'Advertencias no fatales del cálculo',
  ADD CONSTRAINT fk_pde_contrato FOREIGN KEY (contrato_id) REFERENCES contratos(id),
  ADD CONSTRAINT chk_pde_neto CHECK (neto_pagado >= 0 AND neto_pagado = total_ingresos - total_deducciones);

CREATE TABLE planilla_detalle_patronal (
  id                           BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  planilla_detalle_empleado_id BIGINT UNSIGNED NOT NULL COMMENT 'Línea del empleado',
  concepto_salarial_id         BIGINT UNSIGNED NOT NULL COMMENT 'Concepto patronal (CCSS_PAT en modelo agregado, D6)',
  base                         DECIMAL(14,2) NOT NULL COMMENT 'Base sobre la que se calculó',
  porcentaje                   DECIMAL(9,6)  NOT NULL COMMENT 'FRACCIÓN (0.2683), nunca porcentaje',
  monto                        DECIMAL(14,2) NOT NULL COMMENT 'Cuota patronal',
  CONSTRAINT fk_pdpat_detalle  FOREIGN KEY (planilla_detalle_empleado_id) REFERENCES planilla_detalle_empleado(id),
  CONSTRAINT fk_pdpat_concepto FOREIGN KEY (concepto_salarial_id) REFERENCES conceptos_salariales(id),
  CONSTRAINT chk_pdpat CHECK (base >= 0 AND monto >= 0 AND porcentaje >= 0 AND porcentaje <= 1),
  UNIQUE KEY uk_pdpat (planilla_detalle_empleado_id, concepto_salarial_id)
) ENGINE=InnoDB COMMENT='Cuota patronal calculada por empleado y período';

ALTER TABLE planilla_detalle_embargo
  ADD COLUMN monto_calculado   DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Lo que correspondía según % o monto fijo, antes de topes',
  ADD COLUMN monto_insatisfecho DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'monto_calculado - monto (D19: se persiste, no se arrastra)';
UPDATE planilla_detalle_embargo SET monto_calculado = monto WHERE monto_calculado = 0;
ALTER TABLE planilla_detalle_embargo
  ADD CONSTRAINT chk_pdemb_calc CHECK (monto_calculado >= monto AND monto_insatisfecho = monto_calculado - monto);

CREATE TABLE planilla_detalle_incapacidad (
  id                           BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  planilla_detalle_empleado_id BIGINT UNSIGNED NOT NULL COMMENT 'Línea del empleado',
  incapacidad_id               BIGINT NOT NULL COMMENT 'Incapacidad (incapacidades.id es BIGINT con signo)',
  dia_serie_desde              INT UNSIGNED NOT NULL COMMENT 'Día de la incapacidad continua en que empieza este tramo',
  dia_serie_hasta              INT UNSIGNED NOT NULL COMMENT 'Día de la incapacidad continua en que termina',
  dias                         DECIMAL(5,2)  NOT NULL COMMENT 'Días comerciales dentro del período',
  monto_patronal               DECIMAL(14,2) NOT NULL COMMENT 'Subsidio pagado por planilla',
  monto_entidad_informativo    DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Parte a cargo de CCSS/INS (no se paga por planilla)',
  CONSTRAINT fk_pdinc_detalle FOREIGN KEY (planilla_detalle_empleado_id) REFERENCES planilla_detalle_empleado(id),
  CONSTRAINT fk_pdinc_inc     FOREIGN KEY (incapacidad_id) REFERENCES incapacidades(id),
  CONSTRAINT chk_pdinc CHECK (dia_serie_hasta >= dia_serie_desde AND dias >= 0 AND monto_patronal >= 0 AND monto_entidad_informativo >= 0),
  UNIQUE KEY uk_pdinc (planilla_detalle_empleado_id, incapacidad_id, dia_serie_desde)
) ENGINE=InnoDB COMMENT='Detalle de subsidio de incapacidad por tramo';

CREATE TABLE planilla_detalle_prestamo (
  id                           BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  planilla_detalle_empleado_id BIGINT UNSIGNED NOT NULL COMMENT 'Línea del empleado',
  prestamo_id                  BIGINT UNSIGNED NOT NULL COMMENT 'Préstamo',
  cuota_calculada              DECIMAL(14,2) NOT NULL COMMENT 'min(cuota periódica, saldo)',
  monto_retenido               DECIMAL(14,2) NOT NULL COMMENT 'Lo realmente retenido tras el tope de neto',
  aplicado_en                  DATETIME NULL COMMENT 'Se llena al aprobar el período (baja el saldo). NULL = no aplicado',
  CONSTRAINT fk_pdpre_detalle  FOREIGN KEY (planilla_detalle_empleado_id) REFERENCES planilla_detalle_empleado(id),
  CONSTRAINT fk_pdpre_prestamo FOREIGN KEY (prestamo_id) REFERENCES prestamos(id),
  CONSTRAINT chk_pdpre CHECK (cuota_calculada >= 0 AND monto_retenido >= 0 AND monto_retenido <= cuota_calculada),
  UNIQUE KEY uk_pdpre (planilla_detalle_empleado_id, prestamo_id)
) ENGINE=InnoDB COMMENT='Cuota de préstamo por línea de planilla; idempotencia de la aplicación al saldo';

CREATE TABLE planilla_devengos_manuales (
  id                   BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  periodo_planilla_id  BIGINT UNSIGNED NOT NULL COMMENT 'Período (solo en borrador)',
  empleado_id          BIGINT UNSIGNED NOT NULL COMMENT 'Empleado',
  concepto_salarial_id BIGINT UNSIGNED NOT NULL COMMENT 'Concepto de captura manual',
  monto                DECIMAL(14,2) NOT NULL COMMENT 'Monto del devengo',
  observacion          VARCHAR(255) NULL COMMENT 'Motivo/soporte',
  creado_por           BIGINT UNSIGNED NOT NULL COMMENT 'Usuario que lo registró',
  created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Fecha de registro',
  CONSTRAINT fk_pdm_periodo  FOREIGN KEY (periodo_planilla_id)  REFERENCES periodos_planilla(id),
  CONSTRAINT fk_pdm_empleado FOREIGN KEY (empleado_id)          REFERENCES empleados(id),
  CONSTRAINT fk_pdm_concepto FOREIGN KEY (concepto_salarial_id) REFERENCES conceptos_salariales(id),
  CONSTRAINT fk_pdm_usuario  FOREIGN KEY (creado_por)           REFERENCES usuarios(id),
  CONSTRAINT chk_pdm_monto CHECK (monto > 0),
  KEY ix_pdm_periodo_emp (periodo_planilla_id, empleado_id)
) ENGINE=InnoDB COMMENT='Devengos capturados a mano (comisiones, bonos, viáticos) consumidos por el cálculo';

-- Auditoría de planilla_devengos_manuales (salida de scripts/gen_auditoria.sh)
DROP TRIGGER IF EXISTS trg_aud_planilla_devengos_manuales_ins;
CREATE TRIGGER trg_aud_planilla_devengos_manuales_ins AFTER INSERT ON planilla_devengos_manuales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_devengos_manuales', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'periodo_planilla_id', NEW.periodo_planilla_id, 'empleado_id', NEW.empleado_id, 'concepto_salarial_id', NEW.concepto_salarial_id, 'monto', NEW.monto, 'observacion', NEW.observacion, 'creado_por', NEW.creado_por));
END;

DROP TRIGGER IF EXISTS trg_aud_planilla_devengos_manuales_upd;
CREATE TRIGGER trg_aud_planilla_devengos_manuales_upd AFTER UPDATE ON planilla_devengos_manuales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_devengos_manuales', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'periodo_planilla_id', OLD.periodo_planilla_id, 'empleado_id', OLD.empleado_id, 'concepto_salarial_id', OLD.concepto_salarial_id, 'monto', OLD.monto, 'observacion', OLD.observacion, 'creado_por', OLD.creado_por), JSON_OBJECT('id', NEW.id, 'periodo_planilla_id', NEW.periodo_planilla_id, 'empleado_id', NEW.empleado_id, 'concepto_salarial_id', NEW.concepto_salarial_id, 'monto', NEW.monto, 'observacion', NEW.observacion, 'creado_por', NEW.creado_por));
END;

DROP TRIGGER IF EXISTS trg_aud_planilla_devengos_manuales_del;
CREATE TRIGGER trg_aud_planilla_devengos_manuales_del AFTER DELETE ON planilla_devengos_manuales
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_devengos_manuales', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'periodo_planilla_id', OLD.periodo_planilla_id, 'empleado_id', OLD.empleado_id, 'concepto_salarial_id', OLD.concepto_salarial_id, 'monto', OLD.monto, 'observacion', OLD.observacion, 'creado_por', OLD.creado_por), NULL);
END;

-- Regenera los triggers existentes para incluir las columnas incorporadas en
-- 000119 y 000120. planilla_detalle_embargo no tenía triggers de auditoría;
-- su protección por inmutabilidad se agrega en 000121.
DROP TRIGGER IF EXISTS trg_aud_periodos_planilla_ins;
CREATE TRIGGER trg_aud_periodos_planilla_ins AFTER INSERT ON periodos_planilla
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('periodos_planilla', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'tipo_planilla_id', NEW.tipo_planilla_id, 'fecha_inicio', NEW.fecha_inicio, 'fecha_fin', NEW.fecha_fin, 'estado', NEW.estado, 'fecha_generacion', NEW.fecha_generacion, 'preparado_por_usuario_id', NEW.preparado_por_usuario_id, 'fecha_preparacion', NEW.fecha_preparacion, 'revisado_por_usuario_id', NEW.revisado_por_usuario_id, 'fecha_revision', NEW.fecha_revision, 'fecha_aprobacion', NEW.fecha_aprobacion, 'aprobado_por', NEW.aprobado_por, 'aprobado_por_usuario_id', NEW.aprobado_por_usuario_id, 'calculado_por_usuario_id', NEW.calculado_por_usuario_id, 'fecha_calculo', NEW.fecha_calculo, 'pagado_por_usuario_id', NEW.pagado_por_usuario_id, 'fecha_pago', NEW.fecha_pago, 'contabilizado_por_usuario_id', NEW.contabilizado_por_usuario_id, 'fecha_contabilizacion', NEW.fecha_contabilizacion, 'version_calculo', NEW.version_calculo, 'parametros_snapshot', NEW.parametros_snapshot, 'version_motor', NEW.version_motor));
END;

DROP TRIGGER IF EXISTS trg_aud_periodos_planilla_upd;
CREATE TRIGGER trg_aud_periodos_planilla_upd AFTER UPDATE ON periodos_planilla
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('periodos_planilla', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'tipo_planilla_id', OLD.tipo_planilla_id, 'fecha_inicio', OLD.fecha_inicio, 'fecha_fin', OLD.fecha_fin, 'estado', OLD.estado, 'fecha_generacion', OLD.fecha_generacion, 'preparado_por_usuario_id', OLD.preparado_por_usuario_id, 'fecha_preparacion', OLD.fecha_preparacion, 'revisado_por_usuario_id', OLD.revisado_por_usuario_id, 'fecha_revision', OLD.fecha_revision, 'fecha_aprobacion', OLD.fecha_aprobacion, 'aprobado_por', OLD.aprobado_por, 'aprobado_por_usuario_id', OLD.aprobado_por_usuario_id, 'calculado_por_usuario_id', OLD.calculado_por_usuario_id, 'fecha_calculo', OLD.fecha_calculo, 'pagado_por_usuario_id', OLD.pagado_por_usuario_id, 'fecha_pago', OLD.fecha_pago, 'contabilizado_por_usuario_id', OLD.contabilizado_por_usuario_id, 'fecha_contabilizacion', OLD.fecha_contabilizacion, 'version_calculo', OLD.version_calculo, 'parametros_snapshot', OLD.parametros_snapshot, 'version_motor', OLD.version_motor), JSON_OBJECT('id', NEW.id, 'tipo_planilla_id', NEW.tipo_planilla_id, 'fecha_inicio', NEW.fecha_inicio, 'fecha_fin', NEW.fecha_fin, 'estado', NEW.estado, 'fecha_generacion', NEW.fecha_generacion, 'preparado_por_usuario_id', NEW.preparado_por_usuario_id, 'fecha_preparacion', NEW.fecha_preparacion, 'revisado_por_usuario_id', NEW.revisado_por_usuario_id, 'fecha_revision', NEW.fecha_revision, 'fecha_aprobacion', NEW.fecha_aprobacion, 'aprobado_por', NEW.aprobado_por, 'aprobado_por_usuario_id', NEW.aprobado_por_usuario_id, 'calculado_por_usuario_id', NEW.calculado_por_usuario_id, 'fecha_calculo', NEW.fecha_calculo, 'pagado_por_usuario_id', NEW.pagado_por_usuario_id, 'fecha_pago', NEW.fecha_pago, 'contabilizado_por_usuario_id', NEW.contabilizado_por_usuario_id, 'fecha_contabilizacion', NEW.fecha_contabilizacion, 'version_calculo', NEW.version_calculo, 'parametros_snapshot', NEW.parametros_snapshot, 'version_motor', NEW.version_motor));
END;

DROP TRIGGER IF EXISTS trg_aud_periodos_planilla_del;
CREATE TRIGGER trg_aud_periodos_planilla_del AFTER DELETE ON periodos_planilla
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('periodos_planilla', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'tipo_planilla_id', OLD.tipo_planilla_id, 'fecha_inicio', OLD.fecha_inicio, 'fecha_fin', OLD.fecha_fin, 'estado', OLD.estado, 'fecha_generacion', OLD.fecha_generacion, 'preparado_por_usuario_id', OLD.preparado_por_usuario_id, 'fecha_preparacion', OLD.fecha_preparacion, 'revisado_por_usuario_id', OLD.revisado_por_usuario_id, 'fecha_revision', OLD.fecha_revision, 'fecha_aprobacion', OLD.fecha_aprobacion, 'aprobado_por', OLD.aprobado_por, 'aprobado_por_usuario_id', OLD.aprobado_por_usuario_id, 'calculado_por_usuario_id', OLD.calculado_por_usuario_id, 'fecha_calculo', OLD.fecha_calculo, 'pagado_por_usuario_id', OLD.pagado_por_usuario_id, 'fecha_pago', OLD.fecha_pago, 'contabilizado_por_usuario_id', OLD.contabilizado_por_usuario_id, 'fecha_contabilizacion', OLD.fecha_contabilizacion, 'version_calculo', OLD.version_calculo, 'parametros_snapshot', OLD.parametros_snapshot, 'version_motor', OLD.version_motor), NULL);
END;

DROP TRIGGER IF EXISTS trg_aud_planilla_detalle_empleado_ins;
CREATE TRIGGER trg_aud_planilla_detalle_empleado_ins AFTER INSERT ON planilla_detalle_empleado
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_empleado', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'periodo_planilla_id', NEW.periodo_planilla_id, 'empleado_id', NEW.empleado_id, 'programa_presupuestario_id', NEW.programa_presupuestario_id, 'salario_base_periodo', NEW.salario_base_periodo, 'total_ingresos', NEW.total_ingresos, 'total_deducciones', NEW.total_deducciones, 'neto_pagado', NEW.neto_pagado, 'estado', NEW.estado, 'contrato_id', NEW.contrato_id, 'base_ccss', NEW.base_ccss, 'base_renta', NEW.base_renta, 'base_aguinaldo', NEW.base_aguinaldo, 'base_cesantia', NEW.base_cesantia, 'base_vacaciones', NEW.base_vacaciones, 'cuota_obrera_ccss', NEW.cuota_obrera_ccss, 'cuota_patronal_ccss', NEW.cuota_patronal_ccss, 'renta_bruta', NEW.renta_bruta, 'creditos_fiscales_aplicados', NEW.creditos_fiscales_aplicados, 'renta_neta', NEW.renta_neta, 'advertencias', NEW.advertencias));
END;

DROP TRIGGER IF EXISTS trg_aud_planilla_detalle_empleado_upd;
CREATE TRIGGER trg_aud_planilla_detalle_empleado_upd AFTER UPDATE ON planilla_detalle_empleado
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_empleado', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'periodo_planilla_id', OLD.periodo_planilla_id, 'empleado_id', OLD.empleado_id, 'programa_presupuestario_id', OLD.programa_presupuestario_id, 'salario_base_periodo', OLD.salario_base_periodo, 'total_ingresos', OLD.total_ingresos, 'total_deducciones', OLD.total_deducciones, 'neto_pagado', OLD.neto_pagado, 'estado', OLD.estado, 'contrato_id', OLD.contrato_id, 'base_ccss', OLD.base_ccss, 'base_renta', OLD.base_renta, 'base_aguinaldo', OLD.base_aguinaldo, 'base_cesantia', OLD.base_cesantia, 'base_vacaciones', OLD.base_vacaciones, 'cuota_obrera_ccss', OLD.cuota_obrera_ccss, 'cuota_patronal_ccss', OLD.cuota_patronal_ccss, 'renta_bruta', OLD.renta_bruta, 'creditos_fiscales_aplicados', OLD.creditos_fiscales_aplicados, 'renta_neta', OLD.renta_neta, 'advertencias', OLD.advertencias), JSON_OBJECT('id', NEW.id, 'periodo_planilla_id', NEW.periodo_planilla_id, 'empleado_id', NEW.empleado_id, 'programa_presupuestario_id', NEW.programa_presupuestario_id, 'salario_base_periodo', NEW.salario_base_periodo, 'total_ingresos', NEW.total_ingresos, 'total_deducciones', NEW.total_deducciones, 'neto_pagado', NEW.neto_pagado, 'estado', NEW.estado, 'contrato_id', NEW.contrato_id, 'base_ccss', NEW.base_ccss, 'base_renta', NEW.base_renta, 'base_aguinaldo', NEW.base_aguinaldo, 'base_cesantia', NEW.base_cesantia, 'base_vacaciones', NEW.base_vacaciones, 'cuota_obrera_ccss', NEW.cuota_obrera_ccss, 'cuota_patronal_ccss', NEW.cuota_patronal_ccss, 'renta_bruta', NEW.renta_bruta, 'creditos_fiscales_aplicados', NEW.creditos_fiscales_aplicados, 'renta_neta', NEW.renta_neta, 'advertencias', NEW.advertencias));
END;

DROP TRIGGER IF EXISTS trg_aud_planilla_detalle_empleado_del;
CREATE TRIGGER trg_aud_planilla_detalle_empleado_del AFTER DELETE ON planilla_detalle_empleado
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_empleado', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'periodo_planilla_id', OLD.periodo_planilla_id, 'empleado_id', OLD.empleado_id, 'programa_presupuestario_id', OLD.programa_presupuestario_id, 'salario_base_periodo', OLD.salario_base_periodo, 'total_ingresos', OLD.total_ingresos, 'total_deducciones', OLD.total_deducciones, 'neto_pagado', OLD.neto_pagado, 'estado', OLD.estado, 'contrato_id', OLD.contrato_id, 'base_ccss', OLD.base_ccss, 'base_renta', OLD.base_renta, 'base_aguinaldo', OLD.base_aguinaldo, 'base_cesantia', OLD.base_cesantia, 'base_vacaciones', OLD.base_vacaciones, 'cuota_obrera_ccss', OLD.cuota_obrera_ccss, 'cuota_patronal_ccss', OLD.cuota_patronal_ccss, 'renta_bruta', OLD.renta_bruta, 'creditos_fiscales_aplicados', OLD.creditos_fiscales_aplicados, 'renta_neta', OLD.renta_neta, 'advertencias', OLD.advertencias), NULL);
END;