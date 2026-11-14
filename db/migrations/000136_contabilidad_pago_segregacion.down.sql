DROP TRIGGER trg_pp_contabilizar_bu;
DELETE p FROM plantillas_asiento_linea p JOIN tipos_asiento ta ON ta.id = p.tipo_asiento_id WHERE ta.codigo = 'PAGO_PLANILLA';
DELETE FROM tipos_asiento WHERE codigo = 'PAGO_PLANILLA';
