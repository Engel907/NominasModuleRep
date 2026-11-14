DROP TRIGGER trg_parametros_generales_inmut_bd;
DELETE FROM parametros_generales WHERE clave IN ('FERIADO_LABORADO_FACTOR_OBLIGATORIO', 'FERIADO_LABORADO_FACTOR_NO_OBLIGATORIO');
CREATE TRIGGER trg_parametros_generales_inmut_bd BEFORE DELETE ON parametros_generales
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_generales no se borra; se cierra la vigencia';
END;

DELETE m FROM mapeo_concepto_cuenta m JOIN conceptos_salariales c ON c.id = m.concepto_salarial_id WHERE c.codigo = 'FERIADO_LAB';
DELETE t FROM tipo_planilla_concepto t JOIN conceptos_salariales c ON c.id = t.concepto_salarial_id WHERE c.codigo = 'FERIADO_LAB';
DELETE r FROM regimen_concepto_aplicable r JOIN conceptos_salariales c ON c.id = r.concepto_salarial_id WHERE c.codigo = 'FERIADO_LAB';
DELETE o FROM orden_calculo_concepto o JOIN conceptos_salariales c ON c.id = o.concepto_salarial_id WHERE c.codigo = 'FERIADO_LAB';
DELETE f FROM concepto_salarial_formula f JOIN conceptos_salariales c ON c.id = f.concepto_salarial_id WHERE c.codigo = 'FERIADO_LAB';
DELETE FROM conceptos_salariales WHERE codigo = 'FERIADO_LAB';
