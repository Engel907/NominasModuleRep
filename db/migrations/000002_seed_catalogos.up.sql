SET @fecha_vigencia_2026 = '2026-01-01';

INSERT INTO conceptos_salariales
    (codigo, nombre, naturaleza, tipo_calculo, afecta_base_ccss, afecta_base_renta, afecta_base_aguinaldo, afecta_base_cesantia, afecta_base_vacaciones, activo)
VALUES
    ('SAL_BASE',          'Salario Base',                             'ingreso',   'formula',        1, 1, 1, 1, 1, 1),
    ('HRS_EXTRA',         'Horas Extra',                              'ingreso',   'formula',        1, 1, 1, 1, 0, 1),
    ('COMISIONES',        'Comisiones',                               'ingreso',   'captura_manual', 1, 1, 1, 1, 1, 1),
    ('BONO_DISCRECIONAL', 'Bono Discrecional',                        'ingreso',   'captura_manual', 0, 0, 0, 0, 0, 1),
    ('VIATICOS',          'Viáticos (no sujetos)',                    'ingreso',   'captura_manual', 0, 0, 0, 0, 0, 1),
    ('SUB_INCAP',         'Subsidio por Incapacidad',                 'ingreso',   'formula',        0, 0, 0, 0, 0, 1),
    ('AGUINALDO',         'Aguinaldo',                                'ingreso',   'formula',        0, 0, 0, 0, 0, 1),
    ('SALARIO_ESCOLAR',   'Salario Escolar',                          'ingreso',   'formula',        0, 0, 0, 0, 0, 1),
    ('PAGO_VACACIONES',   'Pago de Vacaciones',                       'ingreso',   'formula',        1, 1, 1, 1, 0, 1),
    ('CCSS_OBR',          'Cuota Obrera CCSS',                        'deduccion', 'formula',        0, 0, 0, 0, 0, 1),
    ('IMP_RENTA',         'Impuesto sobre la Renta (salario)',        'deduccion', 'formula',        0, 0, 0, 0, 0, 1),
    ('EMBARGO',           'Embargo judicial',                        'deduccion', 'formula',        0, 0, 0, 0, 0, 1),
    ('PRESTAMO',          'Préstamo (cuota)',                         'deduccion', 'formula',        0, 0, 0, 0, 0, 1),
    ('ASOC_SOLIDARISTA',  'Cuota Asociación Solidarista',             'deduccion', 'captura_manual', 0, 0, 0, 0, 0, 1),
    ('SINDICATO',         'Cuota Sindical',                           'deduccion', 'captura_manual', 0, 0, 0, 0, 0, 1),
    ('CCSS_PAT',          'Cuota Patronal CCSS',                      'deduccion', 'formula',        0, 0, 0, 0, 0, 1),
    ('INS_RT',            'Aporte Patronal INS Riesgos del Trabajo',  'deduccion', 'captura_manual', 0, 0, 0, 0, 0, 1),
    ('BANPOP_PAT',        'Aporte Patronal Banco Popular',            'deduccion', 'captura_manual', 0, 0, 0, 0, 0, 1),
    ('FCL_PAT',           'Aporte Patronal Fondo de Capitalización Laboral', 'deduccion', 'captura_manual', 0, 0, 0, 0, 0, 1)
ON DUPLICATE KEY UPDATE
    nombre = VALUES(nombre);

INSERT INTO parametros_cargas_sociales
    (rubro, concepto_salarial_id, porcentaje_patronal, porcentaje_obrero, fecha_inicio_vigencia, fecha_fin_vigencia, fuente_legal)
SELECT 'CCSS total obrera (SEM+IVM+BPDC)', id, 0.0000, 10.8300, @fecha_vigencia_2026, NULL,
       'Reforma Reglamento IVM - Sesión CCSS N.° 9038 (2019), Transitorio XI, vigente 2026-2028'
FROM conceptos_salariales WHERE codigo = 'CCSS_OBR'
ON DUPLICATE KEY UPDATE porcentaje_obrero = VALUES(porcentaje_obrero), rubro = VALUES(rubro);

INSERT INTO parametros_cargas_sociales
    (rubro, concepto_salarial_id, porcentaje_patronal, porcentaje_obrero, fecha_inicio_vigencia, fecha_fin_vigencia, fuente_legal)
SELECT 'CCSS total patronal (SEM+IVM+LPT+Otras cargas)', id, 26.8300, 0.0000, @fecha_vigencia_2026, NULL,
       'Reforma Reglamento IVM - Sesión CCSS N.° 9038 (2019), Transitorio XI, vigente 2026-2028'
FROM conceptos_salariales WHERE codigo = 'CCSS_PAT'
ON DUPLICATE KEY UPDATE porcentaje_patronal = VALUES(porcentaje_patronal), rubro = VALUES(rubro);

INSERT INTO parametros_tramos_renta
    (orden_tramo, monto_desde, monto_hasta, porcentaje, fecha_inicio_vigencia, fecha_fin_vigencia)
VALUES
    (1,       0.00,  918000.00,  0.00, @fecha_vigencia_2026, NULL),
    (2,  918000.00, 1347000.00, 10.00, @fecha_vigencia_2026, NULL),
    (3, 1347000.00, 2364000.00, 15.00, @fecha_vigencia_2026, NULL),
    (4, 2364000.00, 4727000.00, 20.00, @fecha_vigencia_2026, NULL),
    (5, 4727000.00,       NULL, 25.00, @fecha_vigencia_2026, NULL)
ON DUPLICATE KEY UPDATE
    monto_desde = VALUES(monto_desde),
    monto_hasta = VALUES(monto_hasta),
    porcentaje  = VALUES(porcentaje);

INSERT INTO parametros_creditos_fiscales_renta
    (tipo_credito_fiscal_id, monto_credito, fecha_inicio_vigencia, fecha_fin_vigencia, fuente_legal)
SELECT id, 1710.00, @fecha_vigencia_2026, NULL, 'Art. 33 Ley N.° 7092, Decreto Ejecutivo N.° 45333-H (período fiscal 2026)'
FROM tipos_credito_fiscal_renta WHERE codigo = 'HIJO'
ON DUPLICATE KEY UPDATE monto_credito = VALUES(monto_credito);

INSERT INTO parametros_creditos_fiscales_renta
    (tipo_credito_fiscal_id, monto_credito, fecha_inicio_vigencia, fecha_fin_vigencia, fuente_legal)
SELECT id, 2590.00, @fecha_vigencia_2026, NULL, 'Art. 33 Ley N.° 7092, Decreto Ejecutivo N.° 45333-H (período fiscal 2026)'
FROM tipos_credito_fiscal_renta WHERE codigo = 'CONYUGE'
ON DUPLICATE KEY UPDATE monto_credito = VALUES(monto_credito);

INSERT INTO parametros_salario_minimo
    (clasificacion_puesto_id, monto_mensual, fecha_inicio_vigencia, fecha_fin_vigencia, fuente_legal)
SELECT id, 373092.30, @fecha_vigencia_2026, NULL,
       'Decreto Ejecutivo N.° 45303-MTSS (La Gaceta N.° 229, Alcance 156, 05/12/2025) - categoría TONC'
FROM clasificaciones_puesto WHERE codigo = 'MISCELANEA'
ON DUPLICATE KEY UPDATE monto_mensual = VALUES(monto_mensual);

INSERT INTO parametros_tipos_embargo (codigo, nombre, prioridad_legal, fecha_inicio_vigencia, fecha_fin_vigencia)
VALUES
    ('PENSION_ALIMENTARIA', 'Pensión alimentaria (prioridad absoluta - Ley 7654)',       1, @fecha_vigencia_2026, NULL),
    ('EMBARGO_JUDICIAL',    'Embargo judicial ordinario (civil/comercial)',              2, @fecha_vigencia_2026, NULL),
    ('EMBARGO_HACIENDA',    'Embargo por deudas tributarias (Ministerio de Hacienda)',   3, @fecha_vigencia_2026, NULL),
    ('EMBARGO_CCSS',        'Embargo por cuotas obrero-patronales adeudadas a la CCSS',  4, @fecha_vigencia_2026, NULL)
ON DUPLICATE KEY UPDATE
    nombre          = VALUES(nombre),
    prioridad_legal = VALUES(prioridad_legal);

INSERT INTO programas_presupuestarios (codigo, nombre, activo)
VALUES ('PROG1', 'Administración', 1)
ON DUPLICATE KEY UPDATE nombre = VALUES(nombre);

INSERT INTO cuentas_contables (codigo, nombre, cuenta_padre_id, nivel, naturaleza, activo)
VALUES
    ('5101', 'Gasto — Sueldos y salarios',                       NULL, 1, 'gasto',  1),
    ('5102', 'Gasto — Horas extra',                               NULL, 1, 'gasto',  1),
    ('5103', 'Gasto — Aguinaldo',                                 NULL, 1, 'gasto',  1),
    ('5104', 'Gasto — Salario escolar',                           NULL, 1, 'gasto',  1),
    ('5105', 'Gasto — Vacaciones',                                NULL, 1, 'gasto',  1),
    ('5106', 'Gasto — Cargas sociales patronales',                NULL, 1, 'gasto',  1),
    ('2101', 'CCSS por pagar',                                    NULL, 1, 'pasivo', 1),
    ('2102', 'Retenciones de renta por pagar',                    NULL, 1, 'pasivo', 1),
    ('2103', 'Embargos por pagar a terceros',                     NULL, 1, 'pasivo', 1),
    ('2104', 'Préstamos a empleados por cobrar',                  NULL, 1, 'activo', 1),
    ('2105', 'Asociación solidarista / sindicato por pagar',      NULL, 1, 'pasivo', 1),
    ('2106', 'Salarios por pagar (neto)',                         NULL, 1, 'pasivo', 1),
    ('1101', 'Banco — cuenta de planillas',                       NULL, 1, 'activo', 1)
ON DUPLICATE KEY UPDATE nombre = VALUES(nombre);

INSERT INTO tipos_asiento (codigo, nombre, descripcion)
VALUES
    ('DEVENGO_GASTO_PLANILLA',  'Devengo de planilla — gasto',
     'Lado de gasto del devengo de la corrida: salario base, horas extra, aguinaldo, salario escolar, vacaciones y cargas sociales patronales.'),
    ('DEVENGO_PASIVO_PLANILLA', 'Devengo de planilla — pasivo',
     'Lado de pasivo del devengo de la corrida: retenciones/aportes por pagar (CCSS, renta, embargos, préstamos, asociación/sindicato) y el neto por pagar a los empleados.')
ON DUPLICATE KEY UPDATE nombre = VALUES(nombre);

INSERT INTO mapeo_concepto_cuenta
    (concepto_salarial_id, programa_presupuestario_id, tipo_asiento_id, cuenta_contable_id, tipo_movimiento, fecha_inicio_vigencia, fecha_fin_vigencia)
SELECT cs.id, pp.id, ta.id, cc.id, m.tipo_movimiento, @fecha_vigencia_2026, NULL
FROM (
    SELECT 'SAL_BASE'        AS concepto_codigo, '5101' AS cuenta_codigo, 'DEVENGO_GASTO_PLANILLA'  AS tipo_asiento_codigo, 'debe'  AS tipo_movimiento
    UNION ALL SELECT 'HRS_EXTRA',       '5102', 'DEVENGO_GASTO_PLANILLA',  'debe'
    UNION ALL SELECT 'AGUINALDO',       '5103', 'DEVENGO_GASTO_PLANILLA',  'debe'
    UNION ALL SELECT 'SALARIO_ESCOLAR', '5104', 'DEVENGO_GASTO_PLANILLA',  'debe'
    UNION ALL SELECT 'PAGO_VACACIONES', '5105', 'DEVENGO_GASTO_PLANILLA',  'debe'
    UNION ALL SELECT 'CCSS_PAT',        '5106', 'DEVENGO_GASTO_PLANILLA',  'debe'
    UNION ALL SELECT 'INS_RT',          '5106', 'DEVENGO_GASTO_PLANILLA',  'debe'
    UNION ALL SELECT 'BANPOP_PAT',      '5106', 'DEVENGO_GASTO_PLANILLA',  'debe'
    UNION ALL SELECT 'FCL_PAT',         '5106', 'DEVENGO_GASTO_PLANILLA',  'debe'
    UNION ALL SELECT 'CCSS_OBR',         '2101', 'DEVENGO_PASIVO_PLANILLA', 'haber'
    UNION ALL SELECT 'CCSS_PAT',         '2101', 'DEVENGO_PASIVO_PLANILLA', 'haber'
    UNION ALL SELECT 'IMP_RENTA',        '2102', 'DEVENGO_PASIVO_PLANILLA', 'haber'
    UNION ALL SELECT 'EMBARGO',          '2103', 'DEVENGO_PASIVO_PLANILLA', 'haber'
    UNION ALL SELECT 'PRESTAMO',         '2104', 'DEVENGO_PASIVO_PLANILLA', 'haber'
    UNION ALL SELECT 'ASOC_SOLIDARISTA', '2105', 'DEVENGO_PASIVO_PLANILLA', 'haber'
    UNION ALL SELECT 'SINDICATO',        '2105', 'DEVENGO_PASIVO_PLANILLA', 'haber'
    UNION ALL SELECT 'SAL_BASE',         '2106', 'DEVENGO_PASIVO_PLANILLA', 'haber'
) m
JOIN conceptos_salariales    cs ON cs.codigo = m.concepto_codigo
JOIN cuentas_contables       cc ON cc.codigo = m.cuenta_codigo
JOIN tipos_asiento           ta ON ta.codigo = m.tipo_asiento_codigo
JOIN programas_presupuestarios pp ON pp.codigo = 'PROG1'
ON DUPLICATE KEY UPDATE
    cuenta_contable_id = VALUES(cuenta_contable_id),
    tipo_movimiento     = VALUES(tipo_movimiento);

