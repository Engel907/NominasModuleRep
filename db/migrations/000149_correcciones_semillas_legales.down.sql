DROP TRIGGER trg_parametros_generales_inmut_bu;

UPDATE parametros_generales
   SET valor = '26',
       fuente_legal = 'Configuración inicial'
 WHERE clave = 'VENTANA_PROMEDIO_VACACIONES_SEMANAS' AND valor = '50' AND fecha_fin_vigencia IS NULL;

CREATE TRIGGER trg_parametros_generales_inmut_bu BEFORE UPDATE ON parametros_generales
FOR EACH ROW
BEGIN
  IF OLD.fecha_fin_vigencia IS NOT NULL
     AND NEW.fecha_fin_vigencia IS NULL THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: no se puede reabrir una vigencia cerrada';
  END IF;
  IF NOT (NEW.id <=> OLD.id)
     OR NOT (NEW.clave <=> OLD.clave)
     OR NOT (NEW.valor <=> OLD.valor)
     OR NOT (NEW.tipo <=> OLD.tipo)
     OR NOT (NEW.descripcion <=> OLD.descripcion)
     OR NOT (NEW.fuente_legal <=> OLD.fuente_legal)
     OR NOT (NEW.fecha_inicio_vigencia <=> OLD.fecha_inicio_vigencia)
     OR NOT (NEW.creado_por <=> OLD.creado_por)
     OR NOT (NEW.created_at <=> OLD.created_at) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_generales solo permite cambiar fecha_fin_vigencia';
  END IF;
END;

UPDATE conceptos_salariales
   SET afecta_base_vacaciones = 0
 WHERE codigo IN ('HRS_EXTRA', 'PAGO_VACACIONES', 'FERIADO_LAB');

UPDATE conceptos_salariales
   SET afecta_base_ccss = 0, afecta_base_renta = 0, afecta_base_aguinaldo = 0,
       afecta_base_cesantia = 0, afecta_base_vacaciones = 0, afecta_base_embargo = 0,
       nombre = 'Bono Discrecional'
 WHERE codigo = 'BONO_DISCRECIONAL';
