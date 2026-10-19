-- F19: ciclo de vida del período y segregación de funciones.
-- Sin DELIMITER (mismo estilo que las migraciones con triggers existentes).

ALTER TABLE periodos_planilla
  ADD COLUMN calculado_por_usuario_id      BIGINT UNSIGNED NULL COMMENT 'Usuario que ejecutó el último cálculo',
  ADD COLUMN fecha_calculo                 DATETIME NULL COMMENT 'Fecha/hora del último cálculo',
  ADD COLUMN pagado_por_usuario_id         BIGINT UNSIGNED NULL COMMENT 'Usuario que registró el pago',
  ADD COLUMN fecha_pago                    DATETIME NULL COMMENT 'Fecha/hora del registro del pago',
  ADD COLUMN contabilizado_por_usuario_id  BIGINT UNSIGNED NULL COMMENT 'Usuario que contabilizó el período',
  ADD COLUMN fecha_contabilizacion         DATETIME NULL COMMENT 'Fecha/hora de la contabilización',
  ADD COLUMN version_calculo               INT NOT NULL DEFAULT 0 COMMENT 'Cuántas veces se calculó el período',
  ADD CONSTRAINT fk_pp_calculador      FOREIGN KEY (calculado_por_usuario_id)     REFERENCES usuarios(id),
  ADD CONSTRAINT fk_pp_pagador         FOREIGN KEY (pagado_por_usuario_id)        REFERENCES usuarios(id),
  ADD CONSTRAINT fk_pp_contabilizador  FOREIGN KEY (contabilizado_por_usuario_id) REFERENCES usuarios(id);

-- ---------------------------------------------------------------------------
-- periodos_planilla: transiciones válidas + segregación + inmutabilidad
-- Los mensajes empiezan con un prefijo que el servicio Go traduce a errores
-- de dominio: periodo_inmutable | transicion_invalida | segregacion_funciones
-- ---------------------------------------------------------------------------
CREATE TRIGGER trg_pp_ciclo_bu BEFORE UPDATE ON periodos_planilla
FOR EACH ROW
BEGIN
  IF OLD.estado IN ('aprobada','pagada','contabilizada') THEN
    IF NOT (NEW.tipo_planilla_id <=> OLD.tipo_planilla_id)
       OR NOT (NEW.fecha_inicio <=> OLD.fecha_inicio)
       OR NOT (NEW.fecha_fin <=> OLD.fecha_fin) THEN
      SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'periodo_inmutable: tipo y fechas no cambian desde aprobada';
    END IF;
    IF NEW.estado = OLD.estado AND (
         NOT (NEW.preparado_por_usuario_id <=> OLD.preparado_por_usuario_id)
      OR NOT (NEW.calculado_por_usuario_id <=> OLD.calculado_por_usuario_id)
      OR NOT (NEW.aprobado_por_usuario_id <=> OLD.aprobado_por_usuario_id)
      OR NOT (NEW.fecha_aprobacion <=> OLD.fecha_aprobacion)
      OR NOT (NEW.pagado_por_usuario_id <=> OLD.pagado_por_usuario_id)
      OR NOT (NEW.fecha_pago <=> OLD.fecha_pago)
      OR NOT (NEW.contabilizado_por_usuario_id <=> OLD.contabilizado_por_usuario_id)
      OR NOT (NEW.fecha_contabilizacion <=> OLD.fecha_contabilizacion)
      OR NOT (NEW.version_calculo <=> OLD.version_calculo)) THEN
      SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'periodo_inmutable: periodo aprobado/pagado/contabilizado no se modifica';
    END IF;
  END IF;

  IF NEW.estado <> OLD.estado THEN
    IF (OLD.estado, NEW.estado) NOT IN (
         ('borrador','calculada'),
         ('calculada','borrador'),
         ('calculada','aprobada'),
         ('aprobada','pagada'),
         ('pagada','contabilizada')) THEN
      SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'transicion_invalida: transicion de estado no permitida';
    END IF;

    IF NEW.estado = 'calculada' AND (NEW.calculado_por_usuario_id IS NULL OR NEW.preparado_por_usuario_id IS NULL) THEN
      SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'transicion_invalida: calcular exige preparado_por y calculado_por';
    END IF;

    IF NEW.estado = 'aprobada' THEN
      IF NEW.aprobado_por_usuario_id IS NULL OR NEW.fecha_aprobacion IS NULL THEN
        SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'transicion_invalida: aprobar exige usuario y fecha de aprobacion';
      END IF;
      IF NEW.aprobado_por_usuario_id <=> NEW.preparado_por_usuario_id
         OR NEW.aprobado_por_usuario_id <=> NEW.calculado_por_usuario_id THEN
        SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'segregacion_funciones: quien prepara o calcula no puede aprobar';
      END IF;
    END IF;

    IF NEW.estado = 'pagada' THEN
      IF NEW.pagado_por_usuario_id IS NULL OR NEW.fecha_pago IS NULL THEN
        SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'transicion_invalida: pagar exige usuario y fecha de pago';
      END IF;
      IF NEW.pagado_por_usuario_id <=> NEW.preparado_por_usuario_id
         OR NEW.pagado_por_usuario_id <=> NEW.calculado_por_usuario_id
         OR NEW.pagado_por_usuario_id <=> NEW.aprobado_por_usuario_id THEN
        SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'segregacion_funciones: quien prepara, calcula o aprueba no puede pagar';
      END IF;
    END IF;

    -- contabilizada: el NULL se tolera hasta F28 (el endpoint /contabilizar ya
    -- existe y aun no llena contabilizado_por_usuario_id). F28 debe exigirlo.
    IF NEW.estado = 'contabilizada' THEN
      IF NEW.contabilizado_por_usuario_id <=> NEW.pagado_por_usuario_id
         AND NEW.contabilizado_por_usuario_id IS NOT NULL THEN
        SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'segregacion_funciones: quien paga no puede contabilizar';
      END IF;
    END IF;
  END IF;
END;

CREATE TRIGGER trg_pp_ciclo_bd BEFORE DELETE ON periodos_planilla
FOR EACH ROW
BEGIN
  IF OLD.estado <> 'borrador' THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'periodo_inmutable: solo se elimina un periodo en borrador';
  END IF;
END;

-- ---------------------------------------------------------------------------
-- planilla_detalle_empleado: inmutable desde aprobada (solo cambia `estado`,
-- y solo al valor que corresponde al estado del período)
-- LOCK IN SHARE MODE: evita leer un estado obsoleto del período (snapshot).
-- ---------------------------------------------------------------------------
CREATE TRIGGER trg_pde_inmutable_bi BEFORE INSERT ON planilla_detalle_empleado
FOR EACH ROW
BEGIN
  DECLARE v_estado VARCHAR(20);
  SELECT estado INTO v_estado FROM periodos_planilla WHERE id = NEW.periodo_planilla_id LOCK IN SHARE MODE;
  IF v_estado IS NOT NULL AND v_estado <> 'borrador' THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'periodo_inmutable: solo se agregan lineas con el periodo en borrador';
  END IF;
END;

CREATE TRIGGER trg_pde_inmutable_bu BEFORE UPDATE ON planilla_detalle_empleado
FOR EACH ROW
BEGIN
  DECLARE v_estado VARCHAR(20);
  IF NEW.periodo_planilla_id <> OLD.periodo_planilla_id THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'periodo_inmutable: una linea no cambia de periodo';
  END IF;
  SELECT estado INTO v_estado FROM periodos_planilla WHERE id = OLD.periodo_planilla_id LOCK IN SHARE MODE;
  IF v_estado IN ('aprobada','pagada','contabilizada') THEN
    IF NOT (NEW.empleado_id <=> OLD.empleado_id)
       OR NOT (NEW.programa_presupuestario_id <=> OLD.programa_presupuestario_id)
       OR NOT (NEW.salario_base_periodo <=> OLD.salario_base_periodo)
       OR NOT (NEW.total_ingresos <=> OLD.total_ingresos)
       OR NOT (NEW.total_deducciones <=> OLD.total_deducciones)
       OR NOT (NEW.neto_pagado <=> OLD.neto_pagado) THEN
      SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'periodo_inmutable: linea de periodo aprobado/pagado no se modifica';
    END IF;
    IF v_estado = 'aprobada' AND NEW.estado <> 'aprobado' THEN
      SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'periodo_inmutable: estado de linea no coincide con el periodo';
    END IF;
    IF v_estado IN ('pagada','contabilizada') AND NEW.estado <> 'pagado' THEN
      SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'periodo_inmutable: estado de linea no coincide con el periodo';
    END IF;
  END IF;
END;

CREATE TRIGGER trg_pde_inmutable_bd BEFORE DELETE ON planilla_detalle_empleado
FOR EACH ROW
BEGIN
  DECLARE v_estado VARCHAR(20);
  SELECT estado INTO v_estado FROM periodos_planilla WHERE id = OLD.periodo_planilla_id LOCK IN SHARE MODE;
  IF v_estado IS NOT NULL AND v_estado NOT IN ('borrador','calculada') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'periodo_inmutable: no se eliminan lineas de un periodo aprobado';
  END IF;
END;

-- ---------------------------------------------------------------------------
-- planilla_detalle_concepto: inmutable desde aprobada
-- ---------------------------------------------------------------------------
CREATE TRIGGER trg_pdc_inmutable_bi BEFORE INSERT ON planilla_detalle_concepto
FOR EACH ROW
BEGIN
  DECLARE v_estado VARCHAR(20);
  SELECT pp.estado INTO v_estado
    FROM planilla_detalle_empleado pde
    JOIN periodos_planilla pp ON pp.id = pde.periodo_planilla_id
   WHERE pde.id = NEW.planilla_detalle_empleado_id LOCK IN SHARE MODE;
  IF v_estado IN ('aprobada','pagada','contabilizada') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'periodo_inmutable: no se agregan conceptos a un periodo aprobado';
  END IF;
END;

CREATE TRIGGER trg_pdc_inmutable_bu BEFORE UPDATE ON planilla_detalle_concepto
FOR EACH ROW
BEGIN
  DECLARE v_estado VARCHAR(20);
  IF NEW.planilla_detalle_empleado_id <> OLD.planilla_detalle_empleado_id THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'periodo_inmutable: un concepto no cambia de linea';
  END IF;
  SELECT pp.estado INTO v_estado
    FROM planilla_detalle_empleado pde
    JOIN periodos_planilla pp ON pp.id = pde.periodo_planilla_id
   WHERE pde.id = OLD.planilla_detalle_empleado_id LOCK IN SHARE MODE;
  IF v_estado IN ('aprobada','pagada','contabilizada') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'periodo_inmutable: conceptos de periodo aprobado no se modifican';
  END IF;
END;

CREATE TRIGGER trg_pdc_inmutable_bd BEFORE DELETE ON planilla_detalle_concepto
FOR EACH ROW
BEGIN
  DECLARE v_estado VARCHAR(20);
  SELECT pp.estado INTO v_estado
    FROM planilla_detalle_empleado pde
    JOIN periodos_planilla pp ON pp.id = pde.periodo_planilla_id
   WHERE pde.id = OLD.planilla_detalle_empleado_id LOCK IN SHARE MODE;
  IF v_estado IN ('aprobada','pagada','contabilizada') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'periodo_inmutable: conceptos de periodo aprobado no se eliminan';
  END IF;
END;