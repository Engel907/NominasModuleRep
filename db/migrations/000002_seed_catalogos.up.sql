-- ============================================================================
-- Origen: migrations/000002_seed_catalogos.up.sql
-- ============================================================================
-- =============================================================================
-- Migración: 000002_seed_catalogos.up.sql
-- Tipo: DML (datos semilla) — SEPARADA de las migraciones de estructura (DDL)
-- Propósito: sembrar códigos estables de `conceptos_salariales` y parámetros
--            vigentes de normativa laboral/fiscal de Costa Rica.
--
-- Validado contra: planillas_ccdr_v11_completo.sql (65 tablas) +
-- 000001_seguridad_reforzada.up/down.sql (versión real, no ADR-000).
--
-- NOTAS DE IMPLEMENTACIÓN IMPORTANTES:
--   - `es_vigente` en TODAS las tablas de parámetros es
--     `GENERATED ALWAYS AS (...) STORED` -> NUNCA se incluye en el INSERT,
--     se calcula solo a partir de `fecha_fin_vigencia IS NULL`.
--   - `conceptos_salariales` exige `naturaleza` ENUM('ingreso','deduccion') y
--     `tipo_calculo` ENUM('fijo','formula','captura_manual'), NOT NULL sin
--     default. Los valores asignados abajo son una decisión de negocio
--     razonable pero NO estaban en ADR-000 -> marcados como ASUNCIÓN,
--     confirmar con el equipo de producto/nómina.
--   - `parametros_cargas_sociales` tiene una sola fila por `concepto_salarial_id`
--     con `porcentaje_patronal` Y `porcentaje_obrero` conviviendo en la misma
--     fila (comentario del propio DDL). Se respeta el diseño pero se siembra
--     con los códigos CCSS_OBR/CCSS_PAT pedidos explícitamente en la tarea
--     (ver nota en la sección 2 más abajo sobre esta tensión de diseño).
--   - `parametros_salario_minimo` referencia `clasificaciones_puesto`
--     (catálogo YA sembrado por el v11 con clasificaciones INTERNAS del CCDR:
--     OPERATIVO/ADMINISTRATIVO/GUARDA_VIGILANCIA/DIRECCION/SECRETARIA/
--     BODEGUERO/MISCELANEA) -- NO son las categorías nacionales del MTSS
--     (TONC/TOSC/TOC/etc.). Se requiere una decisión de RRHH para mapear cada
--     clasificación interna a su categoría correspondiente del Decreto de
--     Salarios Mínimos. Ver TODO crítico en sección 5.
--
-- Convención del equipo (ADR-000 sección 6): migraciones idempotentes.
-- Se logra con `ON DUPLICATE KEY UPDATE` apoyado en los UNIQUE KEY reales.
-- =============================================================================

SET @fecha_vigencia_2026 = '2026-01-01';

-- Recomendado (opcional): identificar el cambio en auditoria_cambios con un
-- usuario "sistema/migraciones" real si existe uno en `usuarios`. Si se deja
-- sin definir, los triggers de auditoría insertan usuario_id = NULL (columna
-- nullable, no rompe nada), solo se pierde la trazabilidad de "quién" sembró.
-- SET @auditoria_usuario_id = <id del usuario de sistema>;

-- -----------------------------------------------------------------------------
-- 1) conceptos_salariales — códigos estables
--
-- CORRECCIÓN (esta revisión): se reemplaza el seed parcial anterior (9
-- códigos) por el catálogo completo pedido para sector privado/Código de
-- Trabajo (19 códigos: devengos + deducciones + aportes patronales). Este
-- INSERT sigue yendo ANTES del INSERT...SELECT de parametros_cargas_sociales
-- (sección 2, más abajo), que depende de que CCSS_OBR y CCSS_PAT ya existan
-- aquí -- si esta tabla llegara a quedar vacía (p.ej. porque alguien borra
-- este bloque en una edición futura), ese SELECT no encontraría filas y el
-- INSERT...SELECT de la sección 2 insertaría 0 filas silenciosamente (no
-- lanza error, MySQL no falla un INSERT...SELECT con 0 filas de origen).
--
-- ASUNCIÓN (confirmar con negocio -- catálogo "punto de partida", no cerrado):
--   - naturaleza: 'ingreso' si aumenta lo que recibe el empleado, 'deduccion'
--     si lo disminuye. Los 4 aportes 100% patronales (CCSS_PAT, INS_RT,
--     BANPOP_PAT, FCL_PAT) no son ni ingreso ni deducción del empleado en
--     sentido estricto -- se modelan como 'deduccion' solo porque el ENUM no
--     tiene un tercer valor tipo "costo patronal" (misma tensión ya
--     documentada para CCSS_PAT desde la revisión anterior).
--   - tipo_calculo: 'formula' para todo lo que el motor de cálculo resuelve
--     automáticamente contra una tabla de parámetros (CCSS, renta, horas
--     extra, aguinaldo, salario escolar, vacaciones, subsidios) y para
--     SAL_BASE (lee empleado_datos_laborales.salario_base_periodo).
--     'captura_manual' para lo que no tiene todavía una tabla de parámetros
--     que lo respalde en este esquema (COMISIONES, BONO_DISCRECIONAL,
--     VIATICOS, ASOC_SOLIDARISTA, SINDICATO, INS_RT, BANPOP_PAT, FCL_PAT) --
--     RRHH/Contabilidad puede migrar estos a 'formula' más adelante si se
--     modelan sus propias tablas de parámetros.
--
-- Semántica de afecta_base_* (banderas del enunciado; Sí=1, No=0):
--   - Deducciones y aportes patronales (CCSS_OBR, IMP_RENTA, EMBARGO,
--     PRESTAMO, ASOC_SOLIDARISTA, SINDICATO, CCSS_PAT, INS_RT, BANPOP_PAT,
--     FCL_PAT): no forman parte de ninguna base (0 en las 5 columnas) --
--     estas banderas describen qué compone la base de cálculo de otros
--     rubros, no lo que un rubro le hace a la base de otro.
--   - AGUINALDO y SALARIO_ESCOLAR: son ellos mismos resultado de otro
--     cálculo (no insumo de otro), así que van en 0 en las 5 columnas
--     (columnas TINYINT(1) NOT NULL -- no aceptan NULL, se deja 0 como pide
--     el enunciado "0/NULL según el tipo de columna").
-- -----------------------------------------------------------------------------
INSERT INTO conceptos_salariales
    (codigo, nombre, naturaleza, tipo_calculo, afecta_base_ccss, afecta_base_renta, afecta_base_aguinaldo, afecta_base_cesantia, afecta_base_vacaciones, activo)
VALUES
    -- Devengos
    ('SAL_BASE',          'Salario Base',                             'ingreso',   'formula',        1, 1, 1, 1, 1, 1),
    ('HRS_EXTRA',         'Horas Extra',                              'ingreso',   'formula',        1, 1, 1, 1, 0, 1),
    ('COMISIONES',        'Comisiones',                               'ingreso',   'captura_manual', 1, 1, 1, 1, 1, 1),
    ('BONO_DISCRECIONAL', 'Bono Discrecional',                        'ingreso',   'captura_manual', 0, 0, 0, 0, 0, 1),
    ('VIATICOS',          'Viáticos (no sujetos)',                    'ingreso',   'captura_manual', 0, 0, 0, 0, 0, 1),
    ('SUB_INCAP',         'Subsidio por Incapacidad',                 'ingreso',   'formula',        0, 0, 0, 0, 0, 1),
    ('AGUINALDO',         'Aguinaldo',                                'ingreso',   'formula',        0, 0, 0, 0, 0, 1),
    ('SALARIO_ESCOLAR',   'Salario Escolar',                          'ingreso',   'formula',        0, 0, 0, 0, 0, 1),
    ('PAGO_VACACIONES',   'Pago de Vacaciones',                       'ingreso',   'formula',        1, 1, 1, 1, 0, 1),
    -- Deducciones
    ('CCSS_OBR',          'Cuota Obrera CCSS',                        'deduccion', 'formula',        0, 0, 0, 0, 0, 1),
    ('IMP_RENTA',         'Impuesto sobre la Renta (salario)',        'deduccion', 'formula',        0, 0, 0, 0, 0, 1),
    ('EMBARGO',           'Embargo judicial',                        'deduccion', 'formula',        0, 0, 0, 0, 0, 1),
    ('PRESTAMO',          'Préstamo (cuota)',                         'deduccion', 'formula',        0, 0, 0, 0, 0, 1),
    ('ASOC_SOLIDARISTA',  'Cuota Asociación Solidarista',             'deduccion', 'captura_manual', 0, 0, 0, 0, 0, 1),
    ('SINDICATO',         'Cuota Sindical',                           'deduccion', 'captura_manual', 0, 0, 0, 0, 0, 1),
    -- Aportes patronales (ver ASUNCIÓN arriba sobre naturaleza='deduccion')
    ('CCSS_PAT',          'Cuota Patronal CCSS',                      'deduccion', 'formula',        0, 0, 0, 0, 0, 1),
    ('INS_RT',            'Aporte Patronal INS Riesgos del Trabajo',  'deduccion', 'captura_manual', 0, 0, 0, 0, 0, 1),
    ('BANPOP_PAT',        'Aporte Patronal Banco Popular',            'deduccion', 'captura_manual', 0, 0, 0, 0, 0, 1),
    ('FCL_PAT',           'Aporte Patronal Fondo de Capitalización Laboral', 'deduccion', 'captura_manual', 0, 0, 0, 0, 0, 1)
ON DUPLICATE KEY UPDATE
    nombre = VALUES(nombre); -- idempotente; no pisa flags si ya se ajustaron manualmente

-- -----------------------------------------------------------------------------
-- 2) parametros_cargas_sociales — % CCSS patronal/obrero vigentes desde
--    2026-01-01 (Reforma Reglamento IVM, Sesión Junta Directiva CCSS N.° 9038
--    del 20/06/2019, Transitorio XI — rige 2026-01-01 a 2028-12-31).
--
-- Fuente: totales oficiales confirmados (BDO, La Nación, CCSS) para el ajuste
-- vigente desde enero 2026: cuota patronal TOTAL 26.83%, cuota obrera TOTAL
-- 10.83%.
--
-- ⚠️ TENSIÓN DE DISEÑO A DECIDIR CON EL EQUIPO: el propio comentario del DDL
-- dice que patronal/obrero "ya conviven en una misma fila" por rubro (SEM,
-- IVM, Banco Popular, etc. -- ver columna `rubro`), lo que sugiere un modelo
-- granular por rubro en vez de un total agregado. Aquí se siembra el TOTAL
-- agregado bajo los códigos CCSS_OBR/CCSS_PAT pedidos explícitamente, dejando
-- en 0.0000 el lado no aplicable de cada concepto (ambas columnas son
-- DECIMAL(6,4) NOT NULL, no aceptan NULL). Si el negocio necesita desglose
-- por rubro para reportes SICERE, se requiere: (a) crear un concepto_salarial
-- por rubro, y (b) una fila de parametros_cargas_sociales por rubro con
-- ambos porcentajes reales -- no se fabricó ese desglose aquí porque no pude
-- verificar cada porcentaje por rubro con una fuente oficial confiable.
-- -----------------------------------------------------------------------------
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

-- -----------------------------------------------------------------------------
-- 3) parametros_tramos_renta — Impuesto al Salario, período fiscal 2026.
--    Fuente oficial: Ministerio de Hacienda, Decreto Ejecutivo N.° 45333-H
--    (La Gaceta N.° 229, Alcance 156, 05/12/2025), vigente desde 2026-01-01.
--    Montos mensuales sobre salario bruto (asalariados/jubilados/pensionados).
--    NOTA: esta tabla NO tiene columna fuente_legal (verificado en el DDL).
-- -----------------------------------------------------------------------------
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

-- -----------------------------------------------------------------------------
-- 4) parametros_creditos_fiscales_renta — créditos por hijo y por cónyuge,
--    período fiscal 2026 (mismo Decreto 45333-H).
--
-- `tipos_credito_fiscal_renta` YA está sembrada por el v11 base
-- (codigo IN ('CONYUGE','HIJO')) -- confirmado en el .sql real, sin ASUNCIÓN.
-- -----------------------------------------------------------------------------
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

-- -----------------------------------------------------------------------------
-- 5) parametros_salario_minimo — salario mínimo legal de referencia.
--    Fuente oficial: MTSS, Decreto Ejecutivo N.° 45303-MTSS (La Gaceta N.° 229,
--    Alcance 156, 05/12/2025), vigente desde 2026-01-01.
--
-- ⚠️⚠️ TODO CRÍTICO — MAPEO PENDIENTE DE CONFIRMAR CON RRHH:
-- `clasificaciones_puesto` (ya sembrada por el v11) usa categorías INTERNAS
-- del CCDR: OPERATIVO, ADMINISTRATIVO, GUARDA_VIGILANCIA, DIRECCION,
-- SECRETARIA, BODEGUERO, MISCELANEA. El Decreto 45303-MTSS define el salario
-- mínimo por categorías NACIONALES distintas (Trabajador No Calificado
-- Genérico "TONC", Semicalificado "TOSC", Calificado "TOC", Especializado
-- "TOE", técnicos, profesionales, etc.). Alguien de RRHH/legal debe mapear
-- cada clasificación interna a su categoría correspondiente del decreto
-- ANTES de confiar en esta tabla para bloquear/advertir salarios por debajo
-- del mínimo (ese es el uso previsto, según el comentario del propio DDL).
--
-- Por prudencia, aquí solo se siembra UN mapeo de bajo riesgo de error:
-- MISCELANEA -> Trabajador No Calificado Genérico (TONC), el monto más bajo
-- de la escala nacional, ¢373,092.30 (Decreto 45303-MTSS, vigente 2026-01-01).
-- El resto de clasificaciones (OPERATIVO, ADMINISTRATIVO, GUARDA_VIGILANCIA,
-- DIRECCION, SECRETARIA, BODEGUERO) NO se sembraron -- fabricar ese mapeo sin
-- confirmación de RRHH es un riesgo de cumplimiento (art. 177 Código de
-- Trabajo). Ver reporte final de valores a confirmar.
-- -----------------------------------------------------------------------------
INSERT INTO parametros_salario_minimo
    (clasificacion_puesto_id, monto_mensual, fecha_inicio_vigencia, fecha_fin_vigencia, fuente_legal)
SELECT id, 373092.30, @fecha_vigencia_2026, NULL,
       'Decreto Ejecutivo N.° 45303-MTSS (La Gaceta N.° 229, Alcance 156, 05/12/2025) - categoría TONC (mapeo tentativo, confirmar con RRHH)'
FROM clasificaciones_puesto WHERE codigo = 'MISCELANEA'
ON DUPLICATE KEY UPDATE monto_mensual = VALUES(monto_mensual);

-- -----------------------------------------------------------------------------
-- 6) parametros_tipos_embargo — prioridades legales para embargos.
--
-- ⚠️⚠️ TODO CRÍTICO — VALIDACIÓN LEGAL OBLIGATORIA ANTES DE PRODUCCIÓN:
-- Solo el principio de que la PENSIÓN ALIMENTARIA tiene prioridad absoluta
-- sobre cualquier otro embargo es jurídicamente firme (Código de Trabajo y
-- Ley de Pensiones Alimentarias N.° 7654). El orden relativo entre embargo
-- judicial ordinario / Hacienda / CCSS sembrado abajo es TENTATIVO y debe
-- confirmarse con asesoría legal antes de usarse para calcular embargos
-- reales sobre planillas.
--
-- Columnas reales (distintas a lo asumido en la primera versión): `nombre`
-- (no `descripcion`), `prioridad_legal` (no `prioridad`), sin columna
-- `activo`. `codigo` ya no es UNIQUE por sí solo -- la unicidad real es
-- (codigo, es_vigente), consistente con el patrón de vigencia.
-- -----------------------------------------------------------------------------
INSERT INTO parametros_tipos_embargo (codigo, nombre, prioridad_legal, fecha_inicio_vigencia, fecha_fin_vigencia)
VALUES
    ('PENSION_ALIMENTARIA', 'Pensión alimentaria (prioridad absoluta - Ley 7654)',       1, @fecha_vigencia_2026, NULL),
    ('EMBARGO_JUDICIAL',    'Embargo judicial ordinario (civil/comercial)',              2, @fecha_vigencia_2026, NULL),
    ('EMBARGO_HACIENDA',    'Embargo por deudas tributarias (Ministerio de Hacienda)',   3, @fecha_vigencia_2026, NULL),
    ('EMBARGO_CCSS',        'Embargo por cuotas obrero-patronales adeudadas a la CCSS',  4, @fecha_vigencia_2026, NULL)
ON DUPLICATE KEY UPDATE
    nombre          = VALUES(nombre),
    prioridad_legal = VALUES(prioridad_legal);

-- -----------------------------------------------------------------------------
-- 7) Plan de cuentas contable base + mapeo_concepto_cuenta.
--
-- "Punto de partida razonable, no una verdad legal/contable cerrada" (mismo
-- criterio que el catálogo de conceptos_salariales de la sección 1) --
-- Contabilidad lo ajustará después por panel. Catálogo plano (nivel 1, sin
-- jerarquía de 7 segmentos todavía) a propósito: no se inventó una
-- codificación de 7 segmentos sin que el equipo contable la confirme.
--
-- DEPENDENCIAS QUE ESTA SECCIÓN TAMBIÉN RESUELVE (bloqueaban mapeo_concepto_cuenta,
-- que tiene programa_presupuestario_id y tipo_asiento_id NOT NULL y ninguno de
-- los dos tenía seed en el esquema):
--   - programas_presupuestarios: se siembra un único programa por defecto
--     ('PROG1'/"Administración", mismo código de ejemplo que ya trae el
--     comentario del propio DDL) solo para poder registrar el mapeo.
--     Presupuesto/Contabilidad debe confirmar si los conceptos deben
--     repartirse entre varios programas antes de producción.
--   - tipos_asiento: el DDL tiene UNIQUE KEY uk_mcc_una_vigente
--     (concepto_salarial_id, programa_presupuestario_id, tipo_asiento_id,
--     es_vigente) -- NO incluye cuenta_contable_id, así que un mismo
--     concepto no puede tener dos filas vigentes con el mismo tipo_asiento
--     (p.ej. CCSS_PAT necesita una línea de gasto Y una de pasivo dentro del
--     mismo asiento contable de un devengo real). Se resuelve sembrando DOS
--     tipos_asiento -- uno para el lado de gasto y otro para el lado de
--     pasivo del mismo devengo -- en vez de uno solo; ambos se generan juntos
--     en la misma corrida de planilla, así que en la práctica siguen siendo
--     "un" asiento de devengo con sus líneas repartidas en 2 tipos.
--
-- ASUNCIÓN -- "neto→2106" (pedido explícito de la tarea): no existe un
-- concepto_salarial dedicado a "neto" en el catálogo (es un total derivado,
-- no un wage type) y concepto_salarial_id es NOT NULL en mapeo_concepto_cuenta,
-- así que se usa SAL_BASE como concepto ancla para esa línea. Contabilidad
-- puede reemplazar esto por el mecanismo que prefiera si el motor de asientos
-- termina generando la línea de neto por otro lado (p.ej. directo en
-- asientos_contables_detalle, sin pasar por mapeo_concepto_cuenta).
--
-- La cuenta 1101 (Banco - cuenta de planillas) se siembra en cuentas_contables
-- porque la pidió la tarea, pero NO se mapea en mapeo_concepto_cuenta: no está
-- atada a ningún concepto_salarial, es la cuenta de contrapartida del asiento
-- de PAGO (desembolso), que es un movimiento agregado, no por concepto.
-- -----------------------------------------------------------------------------

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
    -- Lado de gasto (Debe): 5101-5106
    SELECT 'SAL_BASE'        AS concepto_codigo, '5101' AS cuenta_codigo, 'DEVENGO_GASTO_PLANILLA'  AS tipo_asiento_codigo, 'debe'  AS tipo_movimiento
    UNION ALL SELECT 'HRS_EXTRA',       '5102', 'DEVENGO_GASTO_PLANILLA',  'debe'
    UNION ALL SELECT 'AGUINALDO',       '5103', 'DEVENGO_GASTO_PLANILLA',  'debe'
    UNION ALL SELECT 'SALARIO_ESCOLAR', '5104', 'DEVENGO_GASTO_PLANILLA',  'debe'
    UNION ALL SELECT 'PAGO_VACACIONES', '5105', 'DEVENGO_GASTO_PLANILLA',  'debe'
    UNION ALL SELECT 'CCSS_PAT',        '5106', 'DEVENGO_GASTO_PLANILLA',  'debe'
    UNION ALL SELECT 'INS_RT',          '5106', 'DEVENGO_GASTO_PLANILLA',  'debe'
    UNION ALL SELECT 'BANPOP_PAT',      '5106', 'DEVENGO_GASTO_PLANILLA',  'debe'
    UNION ALL SELECT 'FCL_PAT',         '5106', 'DEVENGO_GASTO_PLANILLA',  'debe'
    -- Lado de pasivo (Haber): 2101-2106
    UNION ALL SELECT 'CCSS_OBR',         '2101', 'DEVENGO_PASIVO_PLANILLA', 'haber'
    UNION ALL SELECT 'CCSS_PAT',         '2101', 'DEVENGO_PASIVO_PLANILLA', 'haber'
    UNION ALL SELECT 'IMP_RENTA',        '2102', 'DEVENGO_PASIVO_PLANILLA', 'haber'
    UNION ALL SELECT 'EMBARGO',          '2103', 'DEVENGO_PASIVO_PLANILLA', 'haber'
    UNION ALL SELECT 'PRESTAMO',         '2104', 'DEVENGO_PASIVO_PLANILLA', 'haber' -- contra-cuenta: se acredita (reduce el saldo por cobrar) al deducir la cuota de la planilla
    UNION ALL SELECT 'ASOC_SOLIDARISTA', '2105', 'DEVENGO_PASIVO_PLANILLA', 'haber'
    UNION ALL SELECT 'SINDICATO',        '2105', 'DEVENGO_PASIVO_PLANILLA', 'haber'
    UNION ALL SELECT 'SAL_BASE',         '2106', 'DEVENGO_PASIVO_PLANILLA', 'haber' -- neto: ver ASUNCIÓN arriba, SAL_BASE como ancla
) m
JOIN conceptos_salariales    cs ON cs.codigo = m.concepto_codigo
JOIN cuentas_contables       cc ON cc.codigo = m.cuenta_codigo
JOIN tipos_asiento           ta ON ta.codigo = m.tipo_asiento_codigo
JOIN programas_presupuestarios pp ON pp.codigo = 'PROG1'
ON DUPLICATE KEY UPDATE
    cuenta_contable_id = VALUES(cuenta_contable_id),
    tipo_movimiento     = VALUES(tipo_movimiento);

