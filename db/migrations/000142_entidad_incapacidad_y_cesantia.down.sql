DROP TRIGGER trg_parametros_cesantia_escala_inmut_bu;
DROP TRIGGER trg_parametros_cesantia_escala_inmut_bd;

DELETE FROM parametros_cesantia_escala
WHERE anios_servicio_desde = 13.00 AND anios_servicio_hasta IS NULL AND dias_por_anio = 20.00
  AND fuente_legal = 'Código de Trabajo, art. 29';

UPDATE parametros_cesantia_escala
SET anios_servicio_hasta = NULL
WHERE anios_servicio_desde = 12.00 AND anios_servicio_hasta = 13.00 AND fecha_fin_vigencia IS NULL;

CREATE TRIGGER trg_parametros_cesantia_escala_inmut_bu BEFORE UPDATE ON parametros_cesantia_escala
FOR EACH ROW
BEGIN
  IF OLD.fecha_fin_vigencia IS NOT NULL
     AND NEW.fecha_fin_vigencia IS NULL THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: no se puede reabrir una vigencia cerrada';
  END IF;
  IF NOT (NEW.id <=> OLD.id)
     OR NOT (NEW.regimen_laboral_id <=> OLD.regimen_laboral_id)
     OR NOT (NEW.anios_servicio_desde <=> OLD.anios_servicio_desde)
     OR NOT (NEW.anios_servicio_hasta <=> OLD.anios_servicio_hasta)
     OR NOT (NEW.dias_por_anio <=> OLD.dias_por_anio)
     OR NOT (NEW.tope_anios <=> OLD.tope_anios)
     OR NOT (NEW.fecha_inicio_vigencia <=> OLD.fecha_inicio_vigencia)
     OR NOT (NEW.fuente_legal <=> OLD.fuente_legal)
     OR NOT (NEW.creado_por <=> OLD.creado_por) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_cesantia_escala solo permite cambiar fecha_fin_vigencia';
  END IF;
END;

CREATE TRIGGER trg_parametros_cesantia_escala_inmut_bd BEFORE DELETE ON parametros_cesantia_escala
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_cesantia_escala no se borra; se cierra la vigencia';
END;

ALTER TABLE incapacidades
  DROP FOREIGN KEY fk_incapacidades_entidad_emisora,
  MODIFY entidad_emisora ENUM('CCSS','INS') NOT NULL;
