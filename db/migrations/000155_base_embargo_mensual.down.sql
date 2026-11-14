ALTER TABLE planilla_detalle_empleado DROP COLUMN base_embargo;

DROP TRIGGER IF EXISTS trg_aud_planilla_detalle_empleado_ins;
CREATE TRIGGER trg_aud_planilla_detalle_empleado_ins AFTER INSERT ON planilla_detalle_empleado
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_empleado', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'periodo_planilla_id', NEW.periodo_planilla_id, 'empleado_id', NEW.empleado_id, 'programa_presupuestario_id', NEW.programa_presupuestario_id, 'salario_base_periodo', NEW.salario_base_periodo, 'total_ingresos', NEW.total_ingresos, 'total_deducciones', NEW.total_deducciones, 'neto_pagado', NEW.neto_pagado, 'estado', NEW.estado, 'contrato_id', NEW.contrato_id, 'base_ccss', NEW.base_ccss, 'base_renta', NEW.base_renta, 'base_aguinaldo', NEW.base_aguinaldo, 'base_cesantia', NEW.base_cesantia, 'base_vacaciones', NEW.base_vacaciones, 'cuota_obrera_ccss', NEW.cuota_obrera_ccss, 'cuota_patronal_ccss', NEW.cuota_patronal_ccss, 'renta_bruta', NEW.renta_bruta, 'creditos_fiscales_aplicados', NEW.creditos_fiscales_aplicados, 'renta_neta', NEW.renta_neta, 'advertencias', NEW.advertencias));
END;

DROP TRIGGER IF EXISTS trg_aud_planilla_detalle_empleado_upd;
CREATE TRIGGER trg_aud_planilla_detalle_empleado_upd AFTER UPDATE ON planilla_detalle_empleado
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_empleado', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'periodo_planilla_id', OLD.periodo_planilla_id, 'empleado_id', OLD.empleado_id, 'programa_presupuestario_id', OLD.programa_presupuestario_id, 'salario_base_periodo', OLD.salario_base_periodo, 'total_ingresos', OLD.total_ingresos, 'total_deducciones', OLD.total_deducciones, 'neto_pagado', OLD.neto_pagado, 'estado', OLD.estado, 'contrato_id', OLD.contrato_id, 'base_ccss', OLD.base_ccss, 'base_renta', OLD.base_renta, 'base_aguinaldo', OLD.base_aguinaldo, 'base_cesantia', OLD.base_cesantia, 'base_vacaciones', OLD.base_vacaciones, 'cuota_obrera_ccss', OLD.cuota_obrera_ccss, 'cuota_patronal_ccss', OLD.cuota_patronal_ccss, 'renta_bruta', OLD.renta_bruta, 'creditos_fiscales_aplicados', OLD.creditos_fiscales_aplicados, 'renta_neta', OLD.renta_neta, 'advertencias', OLD.advertencias), JSON_OBJECT('id', NEW.id, 'periodo_planilla_id', NEW.periodo_planilla_id, 'empleado_id', NEW.empleado_id, 'programa_presupuestario_id', NEW.programa_presupuestario_id, 'salario_base_periodo', NEW.salario_base_periodo, 'total_ingresos', NEW.total_ingresos, 'total_deducciones', NEW.total_deducciones, 'neto_pagado', NEW.neto_pagado, 'estado', NEW.estado, 'contrato_id', NEW.contrato_id, 'base_ccss', NEW.base_ccss, 'base_renta', NEW.base_renta, 'base_aguinaldo', NEW.base_aguinaldo, 'base_cesantia', NEW.base_cesantia, 'base_vacaciones', NEW.base_vacaciones, 'cuota_obrera_ccss', NEW.cuota_obrera_ccss, 'cuota_patronal_ccss', NEW.cuota_patronal_ccss, 'renta_bruta', NEW.renta_bruta, 'creditos_fiscales_aplicados', NEW.creditos_fiscales_aplicados, 'renta_neta', NEW.renta_neta, 'advertencias', NEW.advertencias));
END;

DROP TRIGGER IF EXISTS trg_aud_planilla_detalle_empleado_del;
CREATE TRIGGER trg_aud_planilla_detalle_empleado_del AFTER DELETE ON planilla_detalle_empleado
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_empleado', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'periodo_planilla_id', OLD.periodo_planilla_id, 'empleado_id', OLD.empleado_id, 'programa_presupuestario_id', OLD.programa_presupuestario_id, 'salario_base_periodo', OLD.salario_base_periodo, 'total_ingresos', OLD.total_ingresos, 'total_deducciones', OLD.total_deducciones, 'neto_pagado', OLD.neto_pagado, 'estado', OLD.estado, 'contrato_id', OLD.contrato_id, 'base_ccss', OLD.base_ccss, 'base_renta', OLD.base_renta, 'base_aguinaldo', OLD.base_aguinaldo, 'base_cesantia', OLD.base_cesantia, 'base_vacaciones', OLD.base_vacaciones, 'cuota_obrera_ccss', OLD.cuota_obrera_ccss, 'cuota_patronal_ccss', OLD.cuota_patronal_ccss, 'renta_bruta', OLD.renta_bruta, 'creditos_fiscales_aplicados', OLD.creditos_fiscales_aplicados, 'renta_neta', OLD.renta_neta, 'advertencias', OLD.advertencias), NULL);
END;

