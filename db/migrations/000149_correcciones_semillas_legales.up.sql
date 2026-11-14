-- Art. 157 CT: las vacaciones se pagan con el promedio de las remuneraciones
-- ordinarias y extraordinarias de las últimas cincuenta semanas.
DROP TRIGGER trg_parametros_generales_inmut_bu;

UPDATE parametros_generales
   SET valor = '50',
       fuente_legal = 'Código de Trabajo, art. 157'
 WHERE clave = 'VENTANA_PROMEDIO_VACACIONES_SEMANAS' AND valor = '26' AND fecha_fin_vigencia IS NULL;

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

-- El divisor de la hora ordinaria sale de parametros_jornada (por jornada).
UPDATE parametros_generales
   SET fecha_fin_vigencia = fecha_inicio_vigencia
 WHERE clave = 'HORAS_MENSUALES_ORDINARIAS' AND fecha_fin_vigencia IS NULL;

-- Art. 157 CT: horas extra, feriados laborados y vacaciones pagadas integran
-- el promedio de vacaciones.
UPDATE conceptos_salariales
   SET afecta_base_vacaciones = 1
 WHERE codigo IN ('HRS_EXTRA', 'PAGO_VACACIONES', 'FERIADO_LAB');

-- Un bono que remunera el trabajo es salario: cotiza a la CCSS, grava renta y
-- entra en aguinaldo, cesantía, vacaciones y embargo.
UPDATE conceptos_salariales
   SET afecta_base_ccss = 1, afecta_base_renta = 1, afecta_base_aguinaldo = 1,
       afecta_base_cesantia = 1, afecta_base_vacaciones = 1, afecta_base_embargo = 1,
       nombre = 'Bono'
 WHERE codigo = 'BONO_DISCRECIONAL';
