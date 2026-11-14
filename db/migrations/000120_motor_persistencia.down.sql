DROP TRIGGER IF EXISTS trg_aud_planilla_devengos_manuales_del;
DROP TRIGGER IF EXISTS trg_aud_planilla_devengos_manuales_upd;
DROP TRIGGER IF EXISTS trg_aud_planilla_devengos_manuales_ins;
DROP TRIGGER IF EXISTS trg_aud_planilla_detalle_empleado_del;
DROP TRIGGER IF EXISTS trg_aud_planilla_detalle_empleado_upd;
DROP TRIGGER IF EXISTS trg_aud_planilla_detalle_empleado_ins;
DROP TRIGGER IF EXISTS trg_aud_periodos_planilla_del;
DROP TRIGGER IF EXISTS trg_aud_periodos_planilla_upd;
DROP TRIGGER IF EXISTS trg_aud_periodos_planilla_ins;

DROP TABLE planilla_devengos_manuales;

DROP TABLE planilla_detalle_prestamo;

DROP TABLE planilla_detalle_incapacidad;

ALTER TABLE planilla_detalle_embargo
  DROP CHECK chk_pdemb_calc;
ALTER TABLE planilla_detalle_embargo
  DROP COLUMN monto_insatisfecho,
  DROP COLUMN monto_calculado;

DROP TABLE planilla_detalle_patronal;

ALTER TABLE planilla_detalle_empleado
  DROP CHECK chk_pde_neto,
  DROP FOREIGN KEY fk_pde_contrato;
ALTER TABLE planilla_detalle_empleado
  DROP COLUMN advertencias,
  DROP COLUMN renta_neta,
  DROP COLUMN creditos_fiscales_aplicados,
  DROP COLUMN renta_bruta,
  DROP COLUMN cuota_patronal_ccss,
  DROP COLUMN cuota_obrera_ccss,
  DROP COLUMN base_vacaciones,
  DROP COLUMN base_cesantia,
  DROP COLUMN base_aguinaldo,
  DROP COLUMN base_renta,
  DROP COLUMN base_ccss,
  DROP COLUMN contrato_id;

ALTER TABLE periodos_planilla
  DROP COLUMN version_motor,
  DROP COLUMN parametros_snapshot;

CREATE TRIGGER trg_aud_periodos_planilla_ins AFTER INSERT ON periodos_planilla
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('periodos_planilla', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'tipo_planilla_id', NEW.tipo_planilla_id, 'fecha_inicio', NEW.fecha_inicio, 'fecha_fin', NEW.fecha_fin, 'estado', NEW.estado, 'fecha_generacion', NEW.fecha_generacion, 'preparado_por_usuario_id', NEW.preparado_por_usuario_id, 'fecha_preparacion', NEW.fecha_preparacion, 'revisado_por_usuario_id', NEW.revisado_por_usuario_id, 'fecha_revision', NEW.fecha_revision, 'fecha_aprobacion', NEW.fecha_aprobacion, 'aprobado_por', NEW.aprobado_por, 'aprobado_por_usuario_id', NEW.aprobado_por_usuario_id));
END;

CREATE TRIGGER trg_aud_periodos_planilla_upd AFTER UPDATE ON periodos_planilla
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('periodos_planilla', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'tipo_planilla_id', OLD.tipo_planilla_id, 'fecha_inicio', OLD.fecha_inicio, 'fecha_fin', OLD.fecha_fin, 'estado', OLD.estado, 'fecha_generacion', OLD.fecha_generacion, 'preparado_por_usuario_id', OLD.preparado_por_usuario_id, 'fecha_preparacion', OLD.fecha_preparacion, 'revisado_por_usuario_id', OLD.revisado_por_usuario_id, 'fecha_revision', OLD.fecha_revision, 'fecha_aprobacion', OLD.fecha_aprobacion, 'aprobado_por', OLD.aprobado_por, 'aprobado_por_usuario_id', OLD.aprobado_por_usuario_id), JSON_OBJECT('id', NEW.id, 'tipo_planilla_id', NEW.tipo_planilla_id, 'fecha_inicio', NEW.fecha_inicio, 'fecha_fin', NEW.fecha_fin, 'estado', NEW.estado, 'fecha_generacion', NEW.fecha_generacion, 'preparado_por_usuario_id', NEW.preparado_por_usuario_id, 'fecha_preparacion', NEW.fecha_preparacion, 'revisado_por_usuario_id', NEW.revisado_por_usuario_id, 'fecha_revision', NEW.fecha_revision, 'fecha_aprobacion', NEW.fecha_aprobacion, 'aprobado_por', NEW.aprobado_por, 'aprobado_por_usuario_id', NEW.aprobado_por_usuario_id));
END;

CREATE TRIGGER trg_aud_periodos_planilla_del AFTER DELETE ON periodos_planilla
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('periodos_planilla', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'tipo_planilla_id', OLD.tipo_planilla_id, 'fecha_inicio', OLD.fecha_inicio, 'fecha_fin', OLD.fecha_fin, 'estado', OLD.estado, 'fecha_generacion', OLD.fecha_generacion, 'preparado_por_usuario_id', OLD.preparado_por_usuario_id, 'fecha_preparacion', OLD.fecha_preparacion, 'revisado_por_usuario_id', OLD.revisado_por_usuario_id, 'fecha_revision', OLD.fecha_revision, 'fecha_aprobacion', OLD.fecha_aprobacion, 'aprobado_por', OLD.aprobado_por, 'aprobado_por_usuario_id', OLD.aprobado_por_usuario_id), NULL);
END;

CREATE TRIGGER trg_aud_planilla_detalle_empleado_ins AFTER INSERT ON planilla_detalle_empleado
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_empleado', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'periodo_planilla_id', NEW.periodo_planilla_id, 'empleado_id', NEW.empleado_id, 'programa_presupuestario_id', NEW.programa_presupuestario_id, 'salario_base_periodo', NEW.salario_base_periodo, 'total_ingresos', NEW.total_ingresos, 'total_deducciones', NEW.total_deducciones, 'neto_pagado', NEW.neto_pagado, 'estado', NEW.estado));
END;

CREATE TRIGGER trg_aud_planilla_detalle_empleado_upd AFTER UPDATE ON planilla_detalle_empleado
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_empleado', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'periodo_planilla_id', OLD.periodo_planilla_id, 'empleado_id', OLD.empleado_id, 'programa_presupuestario_id', OLD.programa_presupuestario_id, 'salario_base_periodo', OLD.salario_base_periodo, 'total_ingresos', OLD.total_ingresos, 'total_deducciones', OLD.total_deducciones, 'neto_pagado', OLD.neto_pagado, 'estado', OLD.estado), JSON_OBJECT('id', NEW.id, 'periodo_planilla_id', NEW.periodo_planilla_id, 'empleado_id', NEW.empleado_id, 'programa_presupuestario_id', NEW.programa_presupuestario_id, 'salario_base_periodo', NEW.salario_base_periodo, 'total_ingresos', NEW.total_ingresos, 'total_deducciones', NEW.total_deducciones, 'neto_pagado', NEW.neto_pagado, 'estado', NEW.estado));
END;

CREATE TRIGGER trg_aud_planilla_detalle_empleado_del AFTER DELETE ON planilla_detalle_empleado
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('planilla_detalle_empleado', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'periodo_planilla_id', OLD.periodo_planilla_id, 'empleado_id', OLD.empleado_id, 'programa_presupuestario_id', OLD.programa_presupuestario_id, 'salario_base_periodo', OLD.salario_base_periodo, 'total_ingresos', OLD.total_ingresos, 'total_deducciones', OLD.total_deducciones, 'neto_pagado', OLD.neto_pagado, 'estado', OLD.estado), NULL);
END;