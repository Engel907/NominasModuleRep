-- L8 — Conceptos aplicables al régimen CODIGO_TRABAJO.
--
-- Hallazgo: regimen_concepto_aplicable no tenía ninguna fila sembrada, así que
-- el motor no encontraba conceptos para ningún empleado y ningún período se
-- podía calcular. Se siembran TODOS los conceptos activos del catálogo para el
-- único régimen sembrado (CODIGO_TRABAJO), EXCEPTO INS_RT, BANPOP_PAT y FCL_PAT
-- (D6: cargas en modelo agregado CCSS_OBR/CCSS_PAT; capturarlos sería doble
-- conteo). Qué concepto calcula cada TIPO de planilla lo decide
-- tipo_planilla_concepto (L3, D18); esta tabla solo dice qué aplica al régimen.
--
-- SUPUESTO A VALIDAR (RR. HH.): todos los conceptos del catálogo aplican al
-- régimen del Código de Trabajo. Vigencia desde 2026-01-01, igual que los
-- demás seeds paramétricos.
INSERT INTO regimen_concepto_aplicable (regimen_laboral_id, concepto_salarial_id, fecha_inicio_vigencia, fecha_fin_vigencia)
SELECT rl.id, cs.id, '2026-01-01', NULL
FROM regimenes_laborales rl
JOIN conceptos_salariales cs ON cs.activo = 1
WHERE rl.codigo = 'CODIGO_TRABAJO'
  AND cs.codigo NOT IN ('INS_RT', 'BANPOP_PAT', 'FCL_PAT')
  AND NOT EXISTS (
        SELECT 1 FROM regimen_concepto_aplicable r
        WHERE r.regimen_laboral_id = rl.id
          AND r.concepto_salarial_id = cs.id
          AND r.fecha_fin_vigencia IS NULL
  );
