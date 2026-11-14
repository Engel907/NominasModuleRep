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
