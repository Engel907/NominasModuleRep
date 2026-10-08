-- ============================================================================
-- Origen: Fase 3B — Sembrado de catálogos paramétricos restantes
--
-- Sigue el mismo patrón que la Fase 3A (000002_seed_catalogos.up.sql):
-- idempotente vía ON DUPLICATE KEY UPDATE donde la tabla tiene el índice
-- único uk_*_una_vigente, o vía NOT EXISTS donde la tabla no tiene índice
-- único propio (plantillas_asiento_linea). Todos los INSERT resuelven IDs
-- por SELECT ... WHERE codigo = '...', nunca IDs numéricos fijos a mano.
--
-- NO se reinsertan aquí (ya sembrados en fases previas, ver notas de cada
-- migración): conceptos_salariales, cuentas_contables, mapeo_concepto_cuenta
-- (Fase 3A) y tipos_asiento (sembrado junto con cuentas_contables/mapeo en
-- el mismo bloque de Fase 3A, aunque el enunciado de esta fase lo listara
-- de nuevo por error de alcance -- ya tiene DEVENGO_GASTO_PLANILLA y
-- DEVENGO_PASIVO_PLANILLA con ON DUPLICATE KEY UPDATE, así que reinsertarlo
-- sería redundante).
--
-- Deliberadamente NO se sacan filas de regimen_concepto_aplicable en este
-- bloque (instrucción explícita del alcance de esta fase): los conceptos
-- exclusivos de empleo público (ANUALIDAD, CARRERA_PROFESIONAL,
-- DEDICACION_EXCLUSIVA, PROHIBICION) no existen todavía en
-- conceptos_salariales. Tampoco se siembran LEY_MARCO_EMPLEO_PUBLICO ni
-- ESTATUTO_SERVICIO_CIVIL en regimenes_laborales (pendientes de validación
-- legal con el CCDR).
--
-- ASUNCIÓN A CONFIRMAR — creado_por: parametros_preaviso_escala y
-- parametros_salario_escolar exigen creado_por INT NOT NULL, pero no hay
-- todavía ninguna fila sembrada en `usuarios` (ni FK real declarada sobre
-- esa columna en el DDL de esas dos tablas). Se usa el valor 1 como
-- placeholder de "usuario de sistema / carga inicial" -- confirmar con el
-- equipo el id real una vez existan cuentas de usuario, y corregir estas
-- filas si no coincide.
--
-- ASUNCIÓN A CONFIRMAR — parametros_base_minima_contributiva.monto: se
-- siembra ₡232,441.00 como base mínima contributiva SEM/IVM de referencia;
-- no se pudo verificar el monto oficial vigente 2026 contra una circular
-- de la CCSS en este entorno. Ajustar con el monto oficial antes de usar en
-- producción.
-- ============================================================================
SET @fecha_vigencia_2026 = COALESCE(@fecha_vigencia_2026, '2026-01-01');

-- -----------------------------------------------------------------------------
-- 1) regimenes_laborales — Código de Trabajo (régimen general, no versionado).
--    LEY_MARCO_EMPLEO_PUBLICO y ESTATUTO_SERVICIO_CIVIL quedan pendientes.
-- -----------------------------------------------------------------------------
INSERT INTO regimenes_laborales (codigo, nombre, descripcion, activo)
VALUES (
    'CODIGO_TRABAJO',
    'Código de Trabajo',
    'Régimen laboral general regido por el Código de Trabajo; aplica por defecto mientras se valida con el CCDR la aplicabilidad de la Ley Marco de Empleo Público y el Estatuto de Servicio Civil.',
    1
)
ON DUPLICATE KEY UPDATE
    nombre      = VALUES(nombre),
    descripcion = VALUES(descripcion);

-- -----------------------------------------------------------------------------
-- 2) parametros_cesantia_escala — Art. 29 Código de Trabajo, ligada a
--    CODIGO_TRABAJO por regimen_laboral_id (resuelto por codigo, no a mano).
--    tope_anios = 8 en todos los tramos (tope legal de años reconocidos para
--    el cálculo de cesantía tras la reforma de la Ley de Protección al
--    Trabajador) -- ASUNCIÓN A CONFIRMAR si el CCDR aplica un tope distinto.
-- -----------------------------------------------------------------------------
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

-- -----------------------------------------------------------------------------
-- 3) parametros_preaviso_escala — Art. 28 Código de Trabajo, ligada a
--    CODIGO_TRABAJO por regimen_laboral_id (resuelto por codigo).
-- -----------------------------------------------------------------------------
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

-- -----------------------------------------------------------------------------
-- 4) parametros_horas_extra — factor de recargo por tipo de hora extra
--    (Art. 139 CT: 50% de recargo para jornada extraordinaria ordinaria;
--    DOBLE se siembra al 100% de recargo para trabajo en feriado/descanso).
-- -----------------------------------------------------------------------------
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

-- -----------------------------------------------------------------------------
-- 5) parametros_vacaciones_escala — mínimo legal Art. 153 CT: 2 semanas
--    (12 días hábiles) por año para jornada de 6 días. Un solo tramo
--    cubriendo todos los años de servicio; ajustar/agregar tramos si el
--    CCDR reconoce escalas superiores por convención colectiva.
-- -----------------------------------------------------------------------------
INSERT INTO parametros_vacaciones_escala
    (anios_servicio_desde, anios_servicio_hasta, dias_vacaciones, fecha_inicio_vigencia, fecha_fin_vigencia)
VALUES
    (0.00, NULL, 12.00, @fecha_vigencia_2026, NULL)
ON DUPLICATE KEY UPDATE
    dias_vacaciones = VALUES(dias_vacaciones);

-- -----------------------------------------------------------------------------
-- 6) parametros_subsidios_incapacidad — CCSS (enfermedad): días 1-3 a cargo
--    del patrono (50%), desde el día 4 a cargo de la CCSS (60%). INS
--    (riesgos del trabajo): 100% a cargo del INS desde el día 1.
-- -----------------------------------------------------------------------------
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

-- -----------------------------------------------------------------------------
-- 7) parametros_base_minima_contributiva — base mínima contributiva SEM/IVM.
--    Ver ASUNCIÓN A CONFIRMAR sobre el monto, al inicio de este archivo.
-- -----------------------------------------------------------------------------
INSERT INTO parametros_base_minima_contributiva
    (regimen_ccss_id, monto, fecha_inicio_vigencia, fecha_fin_vigencia)
SELECT rc.id, 232441.00, @fecha_vigencia_2026, NULL
FROM regimenes_ccss rc
WHERE rc.codigo IN ('SEM', 'IVM')
ON DUPLICATE KEY UPDATE
    monto = VALUES(monto);

-- -----------------------------------------------------------------------------
-- 8) parametros_salario_escolar — 1/12 (8.33%) del total de salarios brutos
--    del período anual, patrón estándar de Salario Escolar en Costa Rica.
-- -----------------------------------------------------------------------------
INSERT INTO parametros_salario_escolar
    (porcentaje, fecha_inicio_vigencia, fecha_fin_vigencia, creado_por)
VALUES
    (0.0833, @fecha_vigencia_2026, NULL, 1)
ON DUPLICATE KEY UPDATE
    porcentaje = VALUES(porcentaje);

-- -----------------------------------------------------------------------------
-- 9) concepto_salarial_formula — una versión vigente por cada concepto de
--    tipo_calculo = 'formula' (los de tipo_calculo = 'captura_manual' no
--    llevan fórmula, se digitan a mano en cada corrida; ninguno es 'fijo'
--    todavía). expresion_formula es un JSON descriptivo/placeholder que
--    referencia de qué tabla de parámetros toma el cálculo -- el motor de
--    cálculo real deberá reemplazarlo por el DSL que finalmente adopte.
-- -----------------------------------------------------------------------------
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

-- -----------------------------------------------------------------------------
-- 10) orden_calculo_concepto — secuencia base -> horas extra -> bases
--     especiales -> deducciones legales -> deducciones de terceros ->
--     aportes patronales, para los 19 conceptos ya sembrados en Fase 3A.
--     Incrementos de 10 para poder insertar conceptos futuros sin renumerar.
-- -----------------------------------------------------------------------------
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

-- -----------------------------------------------------------------------------
-- 11) plantillas_asiento_linea — líneas por tipo de asiento, alineadas 1 a 1
--     con las cuentas ya usadas en mapeo_concepto_cuenta (Fase 3A): 5101-5106
--     al debe de DEVENGO_GASTO_PLANILLA, 2101-2106 al haber de
--     DEVENGO_PASIVO_PLANILLA. Esta tabla no tiene índice único propio, así
--     que la idempotencia se protege con NOT EXISTS en vez de
--     ON DUPLICATE KEY UPDATE.
-- -----------------------------------------------------------------------------
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

