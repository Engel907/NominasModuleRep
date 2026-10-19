-- 000121 (L2): integridad en BD. Mismo estilo que 000119 (cuerpos BEGIN..END sin cambiar el delimitador).
-- Prefijos de mensaje que traduce el servicio Go:
--   detalle_descuadrado | periodo_inmutable | vigencia_traslapada | parametro_inmutable
-- REQUISITO: fn_detalle_cuadra es una FUNCION. Con binlog activo, MySQL exige
-- log_bin_trust_function_creators=1 o que el usuario de migraciones tenga SUPER
-- (si no: ERROR 1419). La funcion declara READS SQL DATA, que es lo que pide el binlog.

-- ---------------------------------------------------------------------------
-- 1) Cuadre del detalle: 1 solo si hay >=1 linea y cada linea cuadra con sus conceptos
--    (total_ingresos, total_deducciones) y con planilla_detalle_patronal (cuota_patronal_ccss).
-- ---------------------------------------------------------------------------
CREATE FUNCTION fn_detalle_cuadra(p_periodo BIGINT UNSIGNED) RETURNS TINYINT(1)
READS SQL DATA
RETURN IF(
  EXISTS (SELECT 1 FROM planilla_detalle_empleado WHERE periodo_planilla_id = p_periodo)
  AND NOT EXISTS (
    SELECT 1
      FROM planilla_detalle_empleado pde
     WHERE pde.periodo_planilla_id = p_periodo
       AND (
            pde.total_ingresos <> COALESCE((
              SELECT SUM(pdc.monto)
                FROM planilla_detalle_concepto pdc
                JOIN conceptos_salariales cs ON cs.id = pdc.concepto_salarial_id
               WHERE pdc.planilla_detalle_empleado_id = pde.id AND cs.naturaleza = 'ingreso'), 0)
         OR pde.total_deducciones <> COALESCE((
              SELECT SUM(pdc.monto)
                FROM planilla_detalle_concepto pdc
                JOIN conceptos_salariales cs ON cs.id = pdc.concepto_salarial_id
               WHERE pdc.planilla_detalle_empleado_id = pde.id AND cs.naturaleza = 'deduccion'), 0)
         OR pde.cuota_patronal_ccss <> COALESCE((
              SELECT SUM(pat.monto)
                FROM planilla_detalle_patronal pat
               WHERE pat.planilla_detalle_empleado_id = pde.id), 0)
       )
  ),
  1, 0);

-- ---------------------------------------------------------------------------
-- 2) Aprobar exige detalle cuadrado. Corre DESPUES de trg_pp_ciclo_bu (transiciones y segregacion).
-- ---------------------------------------------------------------------------
CREATE TRIGGER trg_pp_cuadre_bu BEFORE UPDATE ON periodos_planilla
FOR EACH ROW FOLLOWS trg_pp_ciclo_bu
BEGIN
  IF OLD.estado = 'calculada' AND NEW.estado = 'aprobada' AND fn_detalle_cuadra(NEW.id) = 0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'detalle_descuadrado: el detalle no cuadra con los totales ni con la cuota patronal';
  END IF;
END;

-- ---------------------------------------------------------------------------
-- 3) Inmutabilidad de los detalles hijos: se rechaza si el periodo padre no esta en borrador/calculada.
--    (LOCK IN SHARE MODE: igual que 000119, evita leer un estado obsoleto del periodo.)
-- ---------------------------------------------------------------------------
-- planilla_detalle_embargo
CREATE TRIGGER trg_pdemb_inmutable_bi BEFORE INSERT ON planilla_detalle_embargo
FOR EACH ROW
BEGIN
  DECLARE v_estado VARCHAR(20);
  SELECT pp.estado INTO v_estado
    FROM planilla_detalle_empleado pde
    JOIN periodos_planilla pp ON pp.id = pde.periodo_planilla_id
   WHERE pde.id = NEW.planilla_detalle_empleado_id LOCK IN SHARE MODE;
  IF v_estado IS NOT NULL AND v_estado NOT IN ('borrador','calculada') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'periodo_inmutable: no se agregan embargos a un periodo aprobado';
  END IF;
END;

CREATE TRIGGER trg_pdemb_inmutable_bu BEFORE UPDATE ON planilla_detalle_embargo
FOR EACH ROW
BEGIN
  DECLARE v_estado VARCHAR(20);
  IF NEW.planilla_detalle_empleado_id <> OLD.planilla_detalle_empleado_id THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'periodo_inmutable: un registro de embargos no cambia de linea';
  END IF;
  SELECT pp.estado INTO v_estado
    FROM planilla_detalle_empleado pde
    JOIN periodos_planilla pp ON pp.id = pde.periodo_planilla_id
   WHERE pde.id = OLD.planilla_detalle_empleado_id LOCK IN SHARE MODE;
  IF v_estado IS NOT NULL AND v_estado NOT IN ('borrador','calculada') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'periodo_inmutable: embargos de periodo aprobado no se modifican';
  END IF;
END;

CREATE TRIGGER trg_pdemb_inmutable_bd BEFORE DELETE ON planilla_detalle_embargo
FOR EACH ROW
BEGIN
  DECLARE v_estado VARCHAR(20);
  SELECT pp.estado INTO v_estado
    FROM planilla_detalle_empleado pde
    JOIN periodos_planilla pp ON pp.id = pde.periodo_planilla_id
   WHERE pde.id = OLD.planilla_detalle_empleado_id LOCK IN SHARE MODE;
  IF v_estado IS NOT NULL AND v_estado NOT IN ('borrador','calculada') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'periodo_inmutable: embargos de periodo aprobado no se eliminan';
  END IF;
END;

-- planilla_detalle_patronal
CREATE TRIGGER trg_pdpat_inmutable_bi BEFORE INSERT ON planilla_detalle_patronal
FOR EACH ROW
BEGIN
  DECLARE v_estado VARCHAR(20);
  SELECT pp.estado INTO v_estado
    FROM planilla_detalle_empleado pde
    JOIN periodos_planilla pp ON pp.id = pde.periodo_planilla_id
   WHERE pde.id = NEW.planilla_detalle_empleado_id LOCK IN SHARE MODE;
  IF v_estado IS NOT NULL AND v_estado NOT IN ('borrador','calculada') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'periodo_inmutable: no se agregan cuotas patronales a un periodo aprobado';
  END IF;
END;

CREATE TRIGGER trg_pdpat_inmutable_bu BEFORE UPDATE ON planilla_detalle_patronal
FOR EACH ROW
BEGIN
  DECLARE v_estado VARCHAR(20);
  IF NEW.planilla_detalle_empleado_id <> OLD.planilla_detalle_empleado_id THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'periodo_inmutable: un registro de cuotas patronales no cambia de linea';
  END IF;
  SELECT pp.estado INTO v_estado
    FROM planilla_detalle_empleado pde
    JOIN periodos_planilla pp ON pp.id = pde.periodo_planilla_id
   WHERE pde.id = OLD.planilla_detalle_empleado_id LOCK IN SHARE MODE;
  IF v_estado IS NOT NULL AND v_estado NOT IN ('borrador','calculada') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'periodo_inmutable: cuotas patronales de periodo aprobado no se modifican';
  END IF;
END;

CREATE TRIGGER trg_pdpat_inmutable_bd BEFORE DELETE ON planilla_detalle_patronal
FOR EACH ROW
BEGIN
  DECLARE v_estado VARCHAR(20);
  SELECT pp.estado INTO v_estado
    FROM planilla_detalle_empleado pde
    JOIN periodos_planilla pp ON pp.id = pde.periodo_planilla_id
   WHERE pde.id = OLD.planilla_detalle_empleado_id LOCK IN SHARE MODE;
  IF v_estado IS NOT NULL AND v_estado NOT IN ('borrador','calculada') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'periodo_inmutable: cuotas patronales de periodo aprobado no se eliminan';
  END IF;
END;

-- planilla_detalle_incapacidad
CREATE TRIGGER trg_pdinc_inmutable_bi BEFORE INSERT ON planilla_detalle_incapacidad
FOR EACH ROW
BEGIN
  DECLARE v_estado VARCHAR(20);
  SELECT pp.estado INTO v_estado
    FROM planilla_detalle_empleado pde
    JOIN periodos_planilla pp ON pp.id = pde.periodo_planilla_id
   WHERE pde.id = NEW.planilla_detalle_empleado_id LOCK IN SHARE MODE;
  IF v_estado IS NOT NULL AND v_estado NOT IN ('borrador','calculada') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'periodo_inmutable: no se agregan incapacidades a un periodo aprobado';
  END IF;
END;

CREATE TRIGGER trg_pdinc_inmutable_bu BEFORE UPDATE ON planilla_detalle_incapacidad
FOR EACH ROW
BEGIN
  DECLARE v_estado VARCHAR(20);
  IF NEW.planilla_detalle_empleado_id <> OLD.planilla_detalle_empleado_id THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'periodo_inmutable: un registro de incapacidades no cambia de linea';
  END IF;
  SELECT pp.estado INTO v_estado
    FROM planilla_detalle_empleado pde
    JOIN periodos_planilla pp ON pp.id = pde.periodo_planilla_id
   WHERE pde.id = OLD.planilla_detalle_empleado_id LOCK IN SHARE MODE;
  IF v_estado IS NOT NULL AND v_estado NOT IN ('borrador','calculada') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'periodo_inmutable: incapacidades de periodo aprobado no se modifican';
  END IF;
END;

CREATE TRIGGER trg_pdinc_inmutable_bd BEFORE DELETE ON planilla_detalle_incapacidad
FOR EACH ROW
BEGIN
  DECLARE v_estado VARCHAR(20);
  SELECT pp.estado INTO v_estado
    FROM planilla_detalle_empleado pde
    JOIN periodos_planilla pp ON pp.id = pde.periodo_planilla_id
   WHERE pde.id = OLD.planilla_detalle_empleado_id LOCK IN SHARE MODE;
  IF v_estado IS NOT NULL AND v_estado NOT IN ('borrador','calculada') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'periodo_inmutable: incapacidades de periodo aprobado no se eliminan';
  END IF;
END;

-- planilla_detalle_prestamo
CREATE TRIGGER trg_pdpre_inmutable_bi BEFORE INSERT ON planilla_detalle_prestamo
FOR EACH ROW
BEGIN
  DECLARE v_estado VARCHAR(20);
  SELECT pp.estado INTO v_estado
    FROM planilla_detalle_empleado pde
    JOIN periodos_planilla pp ON pp.id = pde.periodo_planilla_id
   WHERE pde.id = NEW.planilla_detalle_empleado_id LOCK IN SHARE MODE;
  IF v_estado IS NOT NULL AND v_estado NOT IN ('borrador','calculada') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'periodo_inmutable: no se agregan prestamos a un periodo aprobado';
  END IF;
END;

-- prestamo: en periodo aprobado/pagado/contabilizado solo se permite aplicado_en NULL -> valor
CREATE TRIGGER trg_pdpre_inmutable_bu BEFORE UPDATE ON planilla_detalle_prestamo
FOR EACH ROW
BEGIN
  DECLARE v_estado VARCHAR(20);
  IF NEW.planilla_detalle_empleado_id <> OLD.planilla_detalle_empleado_id THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'periodo_inmutable: una cuota de prestamo no cambia de linea';
  END IF;
  SELECT pp.estado INTO v_estado
    FROM planilla_detalle_empleado pde
    JOIN periodos_planilla pp ON pp.id = pde.periodo_planilla_id
   WHERE pde.id = OLD.planilla_detalle_empleado_id LOCK IN SHARE MODE;
  IF v_estado IS NOT NULL AND v_estado NOT IN ('borrador','calculada') THEN
    IF NOT (OLD.aplicado_en IS NULL AND NEW.aplicado_en IS NOT NULL
            AND NEW.prestamo_id <=> OLD.prestamo_id
            AND NEW.cuota_calculada <=> OLD.cuota_calculada
            AND NEW.monto_retenido <=> OLD.monto_retenido) THEN
      SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'periodo_inmutable: de un periodo aprobado solo se puede registrar aplicado_en';
    END IF;
  END IF;
END;

CREATE TRIGGER trg_pdpre_inmutable_bd BEFORE DELETE ON planilla_detalle_prestamo
FOR EACH ROW
BEGIN
  DECLARE v_estado VARCHAR(20);
  SELECT pp.estado INTO v_estado
    FROM planilla_detalle_empleado pde
    JOIN periodos_planilla pp ON pp.id = pde.periodo_planilla_id
   WHERE pde.id = OLD.planilla_detalle_empleado_id LOCK IN SHARE MODE;
  IF v_estado IS NOT NULL AND v_estado NOT IN ('borrador','calculada') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'periodo_inmutable: prestamos de periodo aprobado no se eliminan';
  END IF;
END;

-- ---------------------------------------------------------------------------
-- 4) Vigencias. parametros_generales: sin traslape de rangos para la misma clave (rangos inclusivos; fin NULL = abierto).
-- ---------------------------------------------------------------------------
CREATE TRIGGER trg_pg_vigencia_bi BEFORE INSERT ON parametros_generales
FOR EACH ROW
BEGIN
  DECLARE v_n INT;
  SELECT COUNT(*) INTO v_n
    FROM parametros_generales pg
   WHERE pg.clave = NEW.clave
     AND pg.fecha_inicio_vigencia <= COALESCE(NEW.fecha_fin_vigencia, '9999-12-31')
     AND COALESCE(pg.fecha_fin_vigencia, '9999-12-31') >= NEW.fecha_inicio_vigencia
   LOCK IN SHARE MODE;
  IF v_n > 0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'vigencia_traslapada: la clave ya tiene una vigencia que se traslapa con este rango';
  END IF;
END;

CREATE TRIGGER trg_pg_vigencia_bu BEFORE UPDATE ON parametros_generales
FOR EACH ROW
BEGIN
  DECLARE v_n INT;
  IF OLD.fecha_fin_vigencia IS NOT NULL
     AND NEW.fecha_fin_vigencia IS NULL THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'vigencia_cerrada: una vigencia cerrada no se reabre';
  END IF;
  IF NOT (NEW.fecha_fin_vigencia <=> OLD.fecha_fin_vigencia)
     AND NEW.fecha_fin_vigencia IS NOT NULL THEN
    SELECT COUNT(*) INTO v_n
      FROM parametros_generales pg
     WHERE pg.clave = NEW.clave
       AND pg.id <> OLD.id
       AND pg.fecha_inicio_vigencia <= NEW.fecha_fin_vigencia
       AND COALESCE(pg.fecha_fin_vigencia, '9999-12-31') >= NEW.fecha_inicio_vigencia
     LOCK IN SHARE MODE;
    IF v_n > 0 THEN
      SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'vigencia_traslapada: el cierre se traslapa con otra vigencia';
    END IF;
  END IF;
END;

-- Inmutabilidad de las 8 tablas parametricas (salida de scripts/gen_inmutabilidad_parametros.sh;
-- columnas = todas salvo fecha_fin_vigencia y es_vigente).
-- parametros_generales: solo se puede cerrar la vigencia; nunca borrar
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

CREATE TRIGGER trg_parametros_generales_inmut_bd BEFORE DELETE ON parametros_generales
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_generales no se borra; se cierra la vigencia';
END;

-- parametros_cargas_sociales: solo se puede cerrar la vigencia; nunca borrar
CREATE TRIGGER trg_parametros_cargas_sociales_inmut_bu BEFORE UPDATE ON parametros_cargas_sociales
FOR EACH ROW
BEGIN
  IF OLD.fecha_fin_vigencia IS NOT NULL
     AND NEW.fecha_fin_vigencia IS NULL THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: no se puede reabrir una vigencia cerrada';
  END IF;
  IF NOT (NEW.id <=> OLD.id)
     OR NOT (NEW.rubro <=> OLD.rubro)
     OR NOT (NEW.concepto_salarial_id <=> OLD.concepto_salarial_id)
     OR NOT (NEW.porcentaje_patronal <=> OLD.porcentaje_patronal)
     OR NOT (NEW.porcentaje_obrero <=> OLD.porcentaje_obrero)
     OR NOT (NEW.fecha_inicio_vigencia <=> OLD.fecha_inicio_vigencia)
     OR NOT (NEW.fuente_legal <=> OLD.fuente_legal) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_cargas_sociales solo permite cambiar fecha_fin_vigencia';
  END IF;
END;

CREATE TRIGGER trg_parametros_cargas_sociales_inmut_bd BEFORE DELETE ON parametros_cargas_sociales
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_cargas_sociales no se borra; se cierra la vigencia';
END;

-- parametros_tramos_renta: solo se puede cerrar la vigencia; nunca borrar
CREATE TRIGGER trg_parametros_tramos_renta_inmut_bu BEFORE UPDATE ON parametros_tramos_renta
FOR EACH ROW
BEGIN
  IF OLD.fecha_fin_vigencia IS NOT NULL
     AND NEW.fecha_fin_vigencia IS NULL THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: no se puede reabrir una vigencia cerrada';
  END IF;
  IF NOT (NEW.id <=> OLD.id)
     OR NOT (NEW.orden_tramo <=> OLD.orden_tramo)
     OR NOT (NEW.monto_desde <=> OLD.monto_desde)
     OR NOT (NEW.monto_hasta <=> OLD.monto_hasta)
     OR NOT (NEW.porcentaje <=> OLD.porcentaje)
     OR NOT (NEW.fecha_inicio_vigencia <=> OLD.fecha_inicio_vigencia) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_tramos_renta solo permite cambiar fecha_fin_vigencia';
  END IF;
END;

CREATE TRIGGER trg_parametros_tramos_renta_inmut_bd BEFORE DELETE ON parametros_tramos_renta
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_tramos_renta no se borra; se cierra la vigencia';
END;

-- parametros_creditos_fiscales_renta: solo se puede cerrar la vigencia; nunca borrar
CREATE TRIGGER trg_parametros_creditos_fiscales_renta_inmut_bu BEFORE UPDATE ON parametros_creditos_fiscales_renta
FOR EACH ROW
BEGIN
  IF OLD.fecha_fin_vigencia IS NOT NULL
     AND NEW.fecha_fin_vigencia IS NULL THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: no se puede reabrir una vigencia cerrada';
  END IF;
  IF NOT (NEW.id <=> OLD.id)
     OR NOT (NEW.tipo_credito_fiscal_id <=> OLD.tipo_credito_fiscal_id)
     OR NOT (NEW.monto_credito <=> OLD.monto_credito)
     OR NOT (NEW.fecha_inicio_vigencia <=> OLD.fecha_inicio_vigencia)
     OR NOT (NEW.fuente_legal <=> OLD.fuente_legal) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_creditos_fiscales_renta solo permite cambiar fecha_fin_vigencia';
  END IF;
END;

CREATE TRIGGER trg_parametros_creditos_fiscales_renta_inmut_bd BEFORE DELETE ON parametros_creditos_fiscales_renta
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_creditos_fiscales_renta no se borra; se cierra la vigencia';
END;

-- parametros_horas_extra: solo se puede cerrar la vigencia; nunca borrar
CREATE TRIGGER trg_parametros_horas_extra_inmut_bu BEFORE UPDATE ON parametros_horas_extra
FOR EACH ROW
BEGIN
  IF OLD.fecha_fin_vigencia IS NOT NULL
     AND NEW.fecha_fin_vigencia IS NULL THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: no se puede reabrir una vigencia cerrada';
  END IF;
  IF NOT (NEW.id <=> OLD.id)
     OR NOT (NEW.tipo_hora_extra_id <=> OLD.tipo_hora_extra_id)
     OR NOT (NEW.factor_recargo <=> OLD.factor_recargo)
     OR NOT (NEW.fecha_inicio_vigencia <=> OLD.fecha_inicio_vigencia) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_horas_extra solo permite cambiar fecha_fin_vigencia';
  END IF;
END;

CREATE TRIGGER trg_parametros_horas_extra_inmut_bd BEFORE DELETE ON parametros_horas_extra
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_horas_extra no se borra; se cierra la vigencia';
END;

-- parametros_base_minima_contributiva: solo se puede cerrar la vigencia; nunca borrar
CREATE TRIGGER trg_parametros_base_minima_contributiva_inmut_bu BEFORE UPDATE ON parametros_base_minima_contributiva
FOR EACH ROW
BEGIN
  IF OLD.fecha_fin_vigencia IS NOT NULL
     AND NEW.fecha_fin_vigencia IS NULL THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: no se puede reabrir una vigencia cerrada';
  END IF;
  IF NOT (NEW.id <=> OLD.id)
     OR NOT (NEW.regimen_ccss_id <=> OLD.regimen_ccss_id)
     OR NOT (NEW.monto <=> OLD.monto)
     OR NOT (NEW.fecha_inicio_vigencia <=> OLD.fecha_inicio_vigencia) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_base_minima_contributiva solo permite cambiar fecha_fin_vigencia';
  END IF;
END;

CREATE TRIGGER trg_parametros_base_minima_contributiva_inmut_bd BEFORE DELETE ON parametros_base_minima_contributiva
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_base_minima_contributiva no se borra; se cierra la vigencia';
END;

-- parametros_subsidios_incapacidad: solo se puede cerrar la vigencia; nunca borrar
CREATE TRIGGER trg_parametros_subsidios_incapacidad_inmut_bu BEFORE UPDATE ON parametros_subsidios_incapacidad
FOR EACH ROW
BEGIN
  IF OLD.fecha_fin_vigencia IS NOT NULL
     AND NEW.fecha_fin_vigencia IS NULL THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: no se puede reabrir una vigencia cerrada';
  END IF;
  IF NOT (NEW.id <=> OLD.id)
     OR NOT (NEW.entidad_emisora_id <=> OLD.entidad_emisora_id)
     OR NOT (NEW.dia_desde <=> OLD.dia_desde)
     OR NOT (NEW.dia_hasta <=> OLD.dia_hasta)
     OR NOT (NEW.porcentaje_patronal <=> OLD.porcentaje_patronal)
     OR NOT (NEW.porcentaje_entidad <=> OLD.porcentaje_entidad)
     OR NOT (NEW.fecha_inicio_vigencia <=> OLD.fecha_inicio_vigencia) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_subsidios_incapacidad solo permite cambiar fecha_fin_vigencia';
  END IF;
END;

CREATE TRIGGER trg_parametros_subsidios_incapacidad_inmut_bd BEFORE DELETE ON parametros_subsidios_incapacidad
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_subsidios_incapacidad no se borra; se cierra la vigencia';
END;

-- parametros_embargo_escala: solo se puede cerrar la vigencia; nunca borrar
CREATE TRIGGER trg_parametros_embargo_escala_inmut_bu BEFORE UPDATE ON parametros_embargo_escala
FOR EACH ROW
BEGIN
  IF OLD.fecha_fin_vigencia IS NOT NULL
     AND NEW.fecha_fin_vigencia IS NULL THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: no se puede reabrir una vigencia cerrada';
  END IF;
  IF NOT (NEW.id <=> OLD.id)
     OR NOT (NEW.desde_multiplo <=> OLD.desde_multiplo)
     OR NOT (NEW.hasta_multiplo <=> OLD.hasta_multiplo)
     OR NOT (NEW.porcentaje_maximo <=> OLD.porcentaje_maximo)
     OR NOT (NEW.fuente_legal <=> OLD.fuente_legal)
     OR NOT (NEW.fecha_inicio_vigencia <=> OLD.fecha_inicio_vigencia)
     OR NOT (NEW.creado_por <=> OLD.creado_por)
     OR NOT (NEW.created_at <=> OLD.created_at) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_embargo_escala solo permite cambiar fecha_fin_vigencia';
  END IF;
END;

CREATE TRIGGER trg_parametros_embargo_escala_inmut_bd BEFORE DELETE ON parametros_embargo_escala
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'parametro_inmutable: parametros_embargo_escala no se borra; se cierra la vigencia';
END;

-- ---------------------------------------------------------------------------
-- 5) Vista de resumen por periodo (dashboard/reportes). LEFT JOIN: un periodo sin lineas sale con ceros.
-- ---------------------------------------------------------------------------
CREATE SQL SECURITY INVOKER VIEW v_planilla_resumen_periodo AS
SELECT pp.id                                  AS periodo_planilla_id,
       pp.tipo_planilla_id                    AS tipo_planilla_id,
       pp.fecha_inicio                        AS fecha_inicio,
       pp.fecha_fin                           AS fecha_fin,
       pp.estado                              AS estado,
       COUNT(pde.id)                          AS num_empleados,
       COALESCE(SUM(pde.total_ingresos), 0)      AS total_ingresos,
       COALESCE(SUM(pde.total_deducciones), 0)   AS total_deducciones,
       COALESCE(SUM(pde.neto_pagado), 0)         AS total_neto,
       COALESCE(SUM(pde.cuota_patronal_ccss), 0) AS total_cuota_patronal
  FROM periodos_planilla pp
  LEFT JOIN planilla_detalle_empleado pde ON pde.periodo_planilla_id = pp.id
 GROUP BY pp.id, pp.tipo_planilla_id, pp.fecha_inicio, pp.fecha_fin, pp.estado;