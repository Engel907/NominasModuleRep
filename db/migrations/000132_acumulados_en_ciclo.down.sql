DROP TRIGGER IF EXISTS trg_aud_aguinaldos_ins;
DROP TRIGGER IF EXISTS trg_aud_aguinaldos_upd;
DROP TRIGGER IF EXISTS trg_aud_aguinaldos_del;
DROP TRIGGER IF EXISTS trg_aud_salarios_escolares_ins;
DROP TRIGGER IF EXISTS trg_aud_salarios_escolares_upd;
DROP TRIGGER IF EXISTS trg_aud_salarios_escolares_del;

DELETE x FROM tipo_planilla_concepto x
JOIN tipos_planilla t ON t.id = x.tipo_planilla_id
JOIN conceptos_salariales c ON c.id = x.concepto_salarial_id
WHERE (t.clase = 'aguinaldo' AND c.codigo = 'AGUINALDO') OR (t.clase = 'salario_escolar' AND c.codigo = 'SALARIO_ESCOLAR');

ALTER TABLE salarios_escolares DROP FOREIGN KEY fk_salesc_periodo, DROP COLUMN periodo_planilla_id;
ALTER TABLE aguinaldos DROP FOREIGN KEY fk_aguinaldo_periodo, DROP COLUMN periodo_planilla_id;
