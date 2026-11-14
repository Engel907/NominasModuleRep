ALTER TABLE incapacidades
  MODIFY entidad_emisora VARCHAR(20) NOT NULL COMMENT 'Código de entidades_emisoras_incapacidad',
  ADD CONSTRAINT fk_incapacidades_entidad_emisora FOREIGN KEY (entidad_emisora) REFERENCES entidades_emisoras_incapacidad(codigo);

DROP TRIGGER trg_parametros_cesantia_escala_inmut_bu;
DROP TRIGGER trg_parametros_cesantia_escala_inmut_bd;

UPDATE parametros_cesantia_escala p
SET p.anios_servicio_hasta = 13.00
WHERE p.anios_servicio_desde = 12.00 AND p.anios_servicio_hasta IS NULL AND p.fecha_fin_vigencia IS NULL
  AND NOT EXISTS (SELECT 1 FROM (SELECT regimen_laboral_id, anios_servicio_desde, fecha_fin_vigencia FROM parametros_cesantia_escala) x
                  WHERE x.regimen_laboral_id = p.regimen_laboral_id AND x.anios_servicio_desde = 13.00 AND x.fecha_fin_vigencia IS NULL);

INSERT INTO parametros_cesantia_escala (regimen_laboral_id, anios_servicio_desde, anios_servicio_hasta, dias_por_anio, tope_anios, fecha_inicio_vigencia, fuente_legal, creado_por)
SELECT p.regimen_laboral_id, 13.00, NULL, 20.00, p.tope_anios, p.fecha_inicio_vigencia,
       'Código de Trabajo, art. 29', p.creado_por
FROM parametros_cesantia_escala p
WHERE p.anios_servicio_desde = 12.00 AND p.anios_servicio_hasta = 13.00 AND p.fecha_fin_vigencia IS NULL
  AND NOT EXISTS (SELECT 1 FROM (SELECT regimen_laboral_id, anios_servicio_desde, fecha_fin_vigencia FROM parametros_cesantia_escala) x
                  WHERE x.regimen_laboral_id = p.regimen_laboral_id AND x.anios_servicio_desde = 13.00 AND x.fecha_fin_vigencia IS NULL);

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
