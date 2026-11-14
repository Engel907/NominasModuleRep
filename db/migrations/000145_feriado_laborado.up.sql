INSERT INTO conceptos_salariales (codigo, nombre, naturaleza, tipo_calculo, afecta_base_ccss, afecta_base_renta,
    afecta_base_aguinaldo, afecta_base_cesantia, afecta_base_vacaciones, afecta_base_embargo, es_deduccion_obligatoria, activo)
SELECT 'FERIADO_LAB', 'Feriado laborado', 'ingreso', 'formula', c.afecta_base_ccss, c.afecta_base_renta,
       c.afecta_base_aguinaldo, c.afecta_base_cesantia, c.afecta_base_vacaciones, c.afecta_base_embargo, 0, 1
FROM conceptos_salariales c
WHERE c.codigo = 'HRS_EXTRA';

INSERT INTO concepto_salarial_formula (concepto_salarial_id, monto_fijo, expresion_formula, fecha_inicio_vigencia)
SELECT n.id, NULL, JSON_OBJECT('tipo', 'feriado_laborado', 'usa', 'FERIADO_LABORADO_FACTOR_*'), f.fecha_inicio_vigencia
FROM conceptos_salariales n
JOIN conceptos_salariales h ON h.codigo = 'HRS_EXTRA'
JOIN concepto_salarial_formula f ON f.concepto_salarial_id = h.id AND f.fecha_fin_vigencia IS NULL
WHERE n.codigo = 'FERIADO_LAB';

INSERT INTO orden_calculo_concepto (concepto_salarial_id, orden, fecha_inicio_vigencia)
SELECT n.id, o.orden, o.fecha_inicio_vigencia
FROM conceptos_salariales n
JOIN conceptos_salariales h ON h.codigo = 'HRS_EXTRA'
JOIN orden_calculo_concepto o ON o.concepto_salarial_id = h.id AND o.fecha_fin_vigencia IS NULL
WHERE n.codigo = 'FERIADO_LAB';

INSERT INTO regimen_concepto_aplicable (regimen_laboral_id, concepto_salarial_id, fecha_inicio_vigencia)
SELECT r.regimen_laboral_id, n.id, r.fecha_inicio_vigencia
FROM conceptos_salariales n
JOIN conceptos_salariales h ON h.codigo = 'HRS_EXTRA'
JOIN regimen_concepto_aplicable r ON r.concepto_salarial_id = h.id AND r.fecha_fin_vigencia IS NULL
WHERE n.codigo = 'FERIADO_LAB';

INSERT INTO tipo_planilla_concepto (tipo_planilla_id, concepto_salarial_id)
SELECT t.tipo_planilla_id, n.id
FROM conceptos_salariales n
JOIN conceptos_salariales h ON h.codigo = 'HRS_EXTRA'
JOIN tipo_planilla_concepto t ON t.concepto_salarial_id = h.id
WHERE n.codigo = 'FERIADO_LAB';

INSERT INTO mapeo_concepto_cuenta (concepto_salarial_id, programa_presupuestario_id, tipo_asiento_id, cuenta_contable_id,
    tipo_movimiento, proyecto, fecha_inicio_vigencia)
SELECT n.id, m.programa_presupuestario_id, m.tipo_asiento_id, m.cuenta_contable_id, m.tipo_movimiento, m.proyecto, m.fecha_inicio_vigencia
FROM conceptos_salariales n
JOIN conceptos_salariales h ON h.codigo = 'HRS_EXTRA'
JOIN mapeo_concepto_cuenta m ON m.concepto_salarial_id = h.id AND m.fecha_fin_vigencia IS NULL
WHERE n.codigo = 'FERIADO_LAB';

INSERT INTO parametros_generales
    (clave, valor, tipo, descripcion, fuente_legal, fecha_inicio_vigencia, fecha_fin_vigencia, creado_por)
SELECT 'FERIADO_LABORADO_FACTOR_OBLIGATORIO', '1', 'decimal',
       'Veces la hora ordinaria que se suma por cada hora ordinaria trabajada en un feriado de pago obligatorio',
       'Código de Trabajo, arts. 148-149 (pago doble)', '2000-01-01', NULL, 1
WHERE NOT EXISTS (SELECT 1 FROM parametros_generales WHERE clave = 'FERIADO_LABORADO_FACTOR_OBLIGATORIO');
