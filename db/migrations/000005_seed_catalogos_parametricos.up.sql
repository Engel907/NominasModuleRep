SET @fecha_vigencia_2026 = COALESCE(@fecha_vigencia_2026, '2026-01-01');

INSERT INTO regimenes_laborales (codigo, nombre, descripcion, activo)
VALUES (
    'CODIGO_TRABAJO',
    'Código de Trabajo',
    'Régimen laboral general regido por el Código de Trabajo',
    1
)
ON DUPLICATE KEY UPDATE
    nombre      = VALUES(nombre),
    descripcion = VALUES(descripcion);

INSERT INTO parametros_cesantia_escala
    (regimen_laboral_id, anios_servicio_desde, anios_servicio_hasta, dias_por_anio, tope_anios, fecha_inicio_vigencia, fecha_fin_vigencia)
SELECT rl.id, v.anios_desde, v.anios_hasta, v.dias_por_anio, 8.00, @fecha_vigencia_2026, NULL
FROM regimenes_laborales rl
JOIN (
    SELECT  1.00 AS anios_desde,  2.00 AS anios_hasta, 19.50 AS dias_por_anio
    UNION ALL SELECT  2.00,  3.00, 20.00
    UNION ALL SELECT  3.00,  4.00, 20.50
    UNION ALL SELECT  4.00,  5.00, 21.00
    UNION ALL SELECT  5.00,  6.00, 21.24
    UNION ALL SELECT  6.00,  7.00, 21.50
    UNION ALL SELECT  7.00,  8.00, 22.00
    UNION ALL SELECT  8.00,  9.00, 22.00
    UNION ALL SELECT  9.00, 10.00, 22.00
    UNION ALL SELECT 10.00, 11.00, 21.50
    UNION ALL SELECT 11.00, 12.00, 21.00
    UNION ALL SELECT 12.00,  NULL, 20.50
) v ON 1=1
WHERE rl.codigo = 'CODIGO_TRABAJO'
ON DUPLICATE KEY UPDATE
    dias_por_anio = VALUES(dias_por_anio),
    tope_anios    = VALUES(tope_anios);

INSERT INTO parametros_preaviso_escala
    (regimen_laboral_id, antiguedad_meses_desde, antiguedad_meses_hasta, dias_preaviso, fecha_inicio_vigencia, fecha_fin_vigencia, creado_por)
SELECT rl.id, v.meses_desde, v.meses_hasta, v.dias_preaviso, @fecha_vigencia_2026, NULL, 1
FROM regimenes_laborales rl
JOIN (
    SELECT  0 AS meses_desde,  3 AS meses_hasta,  0 AS dias_preaviso
    UNION ALL SELECT  3,  6,  7
    UNION ALL SELECT  6, 12, 15
    UNION ALL SELECT 12, NULL, 30
) v ON 1=1
WHERE rl.codigo = 'CODIGO_TRABAJO'
ON DUPLICATE KEY UPDATE
    dias_preaviso = VALUES(dias_preaviso);

INSERT INTO parametros_horas_extra
    (tipo_hora_extra_id, factor_recargo, fecha_inicio_vigencia, fecha_fin_vigencia)
SELECT the.id, v.factor_recargo, @fecha_vigencia_2026, NULL
FROM tipos_hora_extra the
JOIN (
    SELECT 'DIURNA'   AS codigo, 1.5000 AS factor_recargo
    UNION ALL SELECT 'NOCTURNA', 1.5000
    UNION ALL SELECT 'MIXTA',    1.5000
    UNION ALL SELECT 'DOBLE',    2.0000
) v ON v.codigo = the.codigo
ON DUPLICATE KEY UPDATE
    factor_recargo = VALUES(factor_recargo);

INSERT INTO parametros_vacaciones_escala
    (anios_servicio_desde, anios_servicio_hasta, dias_vacaciones, fecha_inicio_vigencia, fecha_fin_vigencia)
VALUES
    (0.00, NULL, 12.00, @fecha_vigencia_2026, NULL)
ON DUPLICATE KEY UPDATE
    dias_vacaciones = VALUES(dias_vacaciones);

INSERT INTO parametros_subsidios_incapacidad
    (entidad_emisora_id, dia_desde, dia_hasta, porcentaje_patronal, porcentaje_entidad, fecha_inicio_vigencia, fecha_fin_vigencia)
SELECT eei.id, v.dia_desde, v.dia_hasta, v.porcentaje_patronal, v.porcentaje_entidad, @fecha_vigencia_2026, NULL
FROM entidades_emisoras_incapacidad eei
JOIN (
    SELECT 'CCSS' AS codigo, 1 AS dia_desde, 3    AS dia_hasta, 50.00 AS porcentaje_patronal,  0.00 AS porcentaje_entidad
    UNION ALL SELECT 'CCSS', 4, NULL,  0.00,  60.00
    UNION ALL SELECT 'INS',  1, NULL,  0.00, 100.00
) v ON v.codigo = eei.codigo
ON DUPLICATE KEY UPDATE
    porcentaje_patronal = VALUES(porcentaje_patronal),
    porcentaje_entidad  = VALUES(porcentaje_entidad);

INSERT INTO parametros_base_minima_contributiva
    (regimen_ccss_id, monto, fecha_inicio_vigencia, fecha_fin_vigencia)
SELECT rc.id, 232441.00, @fecha_vigencia_2026, NULL
FROM regimenes_ccss rc
WHERE rc.codigo IN ('SEM', 'IVM')
ON DUPLICATE KEY UPDATE
    monto = VALUES(monto);

INSERT INTO parametros_salario_escolar
    (porcentaje, fecha_inicio_vigencia, fecha_fin_vigencia, creado_por)
VALUES
    (0.0833, @fecha_vigencia_2026, NULL, 1)
ON DUPLICATE KEY UPDATE
    porcentaje = VALUES(porcentaje);

INSERT INTO concepto_salarial_formula
    (concepto_salarial_id, monto_fijo, expresion_formula, fecha_inicio_vigencia, fecha_fin_vigencia)
SELECT cs.id, NULL, v.expresion_formula, @fecha_vigencia_2026, NULL
FROM conceptos_salariales cs
JOIN (
    SELECT 'SAL_BASE'        AS codigo, JSON_OBJECT('tipo','referencia_directa','fuente','contratos.salario_pactado') AS expresion_formula
    UNION ALL SELECT 'HRS_EXTRA',       JSON_OBJECT('tipo','horas_extra','usa','parametros_horas_extra','ref_base','SAL_BASE')
    UNION ALL SELECT 'SUB_INCAP',       JSON_OBJECT('tipo','incapacidad','usa','parametros_subsidios_incapacidad')
    UNION ALL SELECT 'AGUINALDO',       JSON_OBJECT('tipo','aguinaldo','usa','afecta_base_aguinaldo','periodo','01-dic a 30-nov')
    UNION ALL SELECT 'SALARIO_ESCOLAR', JSON_OBJECT('tipo','salario_escolar','usa','parametros_salario_escolar')
    UNION ALL SELECT 'PAGO_VACACIONES', JSON_OBJECT('tipo','vacaciones','usa','parametros_vacaciones_escala')
    UNION ALL SELECT 'CCSS_OBR',        JSON_OBJECT('tipo','carga_social','usa','parametros_cargas_sociales','lado','obrero')
    UNION ALL SELECT 'CCSS_PAT',        JSON_OBJECT('tipo','carga_social','usa','parametros_cargas_sociales','lado','patronal')
    UNION ALL SELECT 'IMP_RENTA',       JSON_OBJECT('tipo','renta','usa','parametros_tramos_renta','creditos','parametros_creditos_fiscales_renta')
    UNION ALL SELECT 'EMBARGO',         JSON_OBJECT('tipo','embargo','usa','embargos','tipos','parametros_tipos_embargo')
    UNION ALL SELECT 'PRESTAMO',        JSON_OBJECT('tipo','prestamo','usa','prestamos')
) v ON v.codigo = cs.codigo
ON DUPLICATE KEY UPDATE
    expresion_formula = VALUES(expresion_formula);

INSERT INTO orden_calculo_concepto
    (concepto_salarial_id, orden, fecha_inicio_vigencia, fecha_fin_vigencia)
SELECT cs.id, v.orden, @fecha_vigencia_2026, NULL
FROM conceptos_salariales cs
JOIN (
    SELECT 'SAL_BASE' AS codigo, 10 AS orden
    UNION ALL SELECT 'HRS_EXTRA', 20
    UNION ALL SELECT 'COMISIONES', 30
    UNION ALL SELECT 'BONO_DISCRECIONAL', 40
    UNION ALL SELECT 'VIATICOS', 50
    UNION ALL SELECT 'SUB_INCAP', 60
    UNION ALL SELECT 'AGUINALDO', 70
    UNION ALL SELECT 'SALARIO_ESCOLAR', 80
    UNION ALL SELECT 'PAGO_VACACIONES', 90
    UNION ALL SELECT 'CCSS_OBR', 100
    UNION ALL SELECT 'IMP_RENTA', 110
    UNION ALL SELECT 'EMBARGO', 120
    UNION ALL SELECT 'PRESTAMO', 130
    UNION ALL SELECT 'ASOC_SOLIDARISTA', 140
    UNION ALL SELECT 'SINDICATO', 150
    UNION ALL SELECT 'CCSS_PAT', 160
    UNION ALL SELECT 'INS_RT', 170
    UNION ALL SELECT 'BANPOP_PAT', 180
    UNION ALL SELECT 'FCL_PAT', 190
) v ON v.codigo = cs.codigo
ON DUPLICATE KEY UPDATE
    orden = VALUES(orden);

INSERT INTO plantillas_asiento_linea
    (tipo_asiento_id, descripcion_linea, tipo_movimiento, orden)
SELECT ta.id, v.descripcion_linea, v.tipo_movimiento, v.orden
FROM tipos_asiento ta
JOIN (
    SELECT 'DEVENGO_GASTO_PLANILLA'  AS ta_codigo, 'Gasto — Sueldos y salarios'                    AS descripcion_linea, 'debe'  AS tipo_movimiento, 10 AS orden
    UNION ALL SELECT 'DEVENGO_GASTO_PLANILLA',  'Gasto — Horas extra',                          'debe',  20
    UNION ALL SELECT 'DEVENGO_GASTO_PLANILLA',  'Gasto — Aguinaldo',                            'debe',  30
    UNION ALL SELECT 'DEVENGO_GASTO_PLANILLA',  'Gasto — Salario escolar',                      'debe',  40
    UNION ALL SELECT 'DEVENGO_GASTO_PLANILLA',  'Gasto — Vacaciones',                           'debe',  50
    UNION ALL SELECT 'DEVENGO_GASTO_PLANILLA',  'Gasto — Cargas sociales patronales',           'debe',  60
    UNION ALL SELECT 'DEVENGO_PASIVO_PLANILLA', 'CCSS por pagar',                               'haber', 10
    UNION ALL SELECT 'DEVENGO_PASIVO_PLANILLA', 'Retenciones de renta por pagar',               'haber', 20
    UNION ALL SELECT 'DEVENGO_PASIVO_PLANILLA', 'Embargos por pagar a terceros',                'haber', 30
    UNION ALL SELECT 'DEVENGO_PASIVO_PLANILLA', 'Préstamos a empleados por cobrar',             'haber', 40
    UNION ALL SELECT 'DEVENGO_PASIVO_PLANILLA', 'Asociación solidarista / sindicato por pagar', 'haber', 50
    UNION ALL SELECT 'DEVENGO_PASIVO_PLANILLA', 'Salarios por pagar (neto)',                    'haber', 60
) v ON v.ta_codigo = ta.codigo
WHERE NOT EXISTS (
    SELECT 1 FROM plantillas_asiento_linea pal
    WHERE pal.tipo_asiento_id = ta.id
      AND pal.descripcion_linea = v.descripcion_linea
);

