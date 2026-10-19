DROP TRIGGER IF EXISTS trg_pdc_inmutable_bd;
DROP TRIGGER IF EXISTS trg_pdc_inmutable_bu;
DROP TRIGGER IF EXISTS trg_pdc_inmutable_bi;
DROP TRIGGER IF EXISTS trg_pde_inmutable_bd;
DROP TRIGGER IF EXISTS trg_pde_inmutable_bu;
DROP TRIGGER IF EXISTS trg_pde_inmutable_bi;
DROP TRIGGER IF EXISTS trg_pp_ciclo_bd;
DROP TRIGGER IF EXISTS trg_pp_ciclo_bu;

ALTER TABLE periodos_planilla
  DROP FOREIGN KEY fk_pp_calculador,
  DROP FOREIGN KEY fk_pp_pagador,
  DROP FOREIGN KEY fk_pp_contabilizador;

ALTER TABLE periodos_planilla
  DROP COLUMN calculado_por_usuario_id,
  DROP COLUMN fecha_calculo,
  DROP COLUMN pagado_por_usuario_id,
  DROP COLUMN fecha_pago,
  DROP COLUMN contabilizado_por_usuario_id,
  DROP COLUMN fecha_contabilizacion,
  DROP COLUMN version_calculo;