DROP TRIGGER trg_aud_contratos_ins;
DROP TRIGGER trg_aud_contratos_upd;
DROP TRIGGER trg_aud_contratos_del;
DROP TRIGGER trg_contratos_sync_salario_ai;
DROP TRIGGER trg_edl_salario_derivado_bi;
ALTER TABLE contratos DROP FOREIGN KEY fk_contratos_tipo;
ALTER TABLE contratos DROP COLUMN tipo_contrato_id;
