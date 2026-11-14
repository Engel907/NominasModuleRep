CREATE TABLE `sistemas_origen` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'Identificador interno del sistema origen',
  `codigo` varchar(20) NOT NULL COMMENT 'p.ej. SAP, PAYONE, JAISSON',
  `nombre` varchar(100) NOT NULL COMMENT 'Nombre descriptivo del sistema origen',
  PRIMARY KEY (`id`),
  UNIQUE KEY `codigo` (`codigo`)
) ENGINE=InnoDB COMMENT='Catálogo de sistemas externos de origen de datos importables'
;

DROP TRIGGER IF EXISTS trg_aud_sistemas_origen_ins;
CREATE TRIGGER trg_aud_sistemas_origen_ins AFTER INSERT ON sistemas_origen
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('sistemas_origen', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre));
END;

DROP TRIGGER IF EXISTS trg_aud_sistemas_origen_upd;
CREATE TRIGGER trg_aud_sistemas_origen_upd AFTER UPDATE ON sistemas_origen
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('sistemas_origen', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre), JSON_OBJECT('id', NEW.id, 'codigo', NEW.codigo, 'nombre', NEW.nombre));
END;

DROP TRIGGER IF EXISTS trg_aud_sistemas_origen_del;
CREATE TRIGGER trg_aud_sistemas_origen_del AFTER DELETE ON sistemas_origen
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('sistemas_origen', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'codigo', OLD.codigo, 'nombre', OLD.nombre), NULL);
END;

CREATE TABLE `interfaces_importacion` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'Identificador interno de la interfaz de importación',
  `sistema_origen_id` bigint unsigned NOT NULL COMMENT 'Sistema origen al que corresponde esta interfaz',
  `nombre` varchar(100) NOT NULL COMMENT 'Nombre descriptivo de la interfaz de importación',
  `formato_archivo` enum('delimitado','ancho_fijo','hoja_calculo') NOT NULL COMMENT 'Formato del archivo que procesa esta interfaz',
  `configuracion_mapeo` json NOT NULL COMMENT 'columnas, delimitadores, posiciones',
  `activo` tinyint(1) NOT NULL DEFAULT '1' COMMENT '1 = interfaz disponible para usarse, 0 = deshabilitada',
  PRIMARY KEY (`id`),
  KEY `fk_ii_sistema` (`sistema_origen_id`),
  CONSTRAINT `fk_ii_sistema` FOREIGN KEY (`sistema_origen_id`) REFERENCES `sistemas_origen` (`id`)
) ENGINE=InnoDB COMMENT='Definición configurable de cada interfaz de importación de datos'
;

DROP TRIGGER IF EXISTS trg_aud_interfaces_importacion_ins;
CREATE TRIGGER trg_aud_interfaces_importacion_ins AFTER INSERT ON interfaces_importacion
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('interfaces_importacion', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'sistema_origen_id', NEW.sistema_origen_id, 'nombre', NEW.nombre, 'formato_archivo', NEW.formato_archivo, 'configuracion_mapeo', NEW.configuracion_mapeo, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_interfaces_importacion_upd;
CREATE TRIGGER trg_aud_interfaces_importacion_upd AFTER UPDATE ON interfaces_importacion
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('interfaces_importacion', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'sistema_origen_id', OLD.sistema_origen_id, 'nombre', OLD.nombre, 'formato_archivo', OLD.formato_archivo, 'configuracion_mapeo', OLD.configuracion_mapeo, 'activo', OLD.activo), JSON_OBJECT('id', NEW.id, 'sistema_origen_id', NEW.sistema_origen_id, 'nombre', NEW.nombre, 'formato_archivo', NEW.formato_archivo, 'configuracion_mapeo', NEW.configuracion_mapeo, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_interfaces_importacion_del;
CREATE TRIGGER trg_aud_interfaces_importacion_del AFTER DELETE ON interfaces_importacion
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('interfaces_importacion', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'sistema_origen_id', OLD.sistema_origen_id, 'nombre', OLD.nombre, 'formato_archivo', OLD.formato_archivo, 'configuracion_mapeo', OLD.configuracion_mapeo, 'activo', OLD.activo), NULL);
END;

CREATE TABLE `importaciones_staging` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'Identificador interno de la carga',
  `interfaz_importacion_id` bigint unsigned NOT NULL COMMENT 'Interfaz de importación usada para procesar esta carga',
  `nombre_archivo_origen` varchar(255) NOT NULL COMMENT 'Nombre del archivo que se cargó',
  `fecha_carga` datetime NOT NULL COMMENT 'Fecha/hora en que se cargó el archivo',
  `estado` enum('cargado','validado','con_errores','consolidado') NOT NULL DEFAULT 'cargado' COMMENT 'Etapa actual del procesamiento de la carga',
  `contenido_raw` longtext COMMENT 'Contenido crudo del archivo cargado, tal como se recibió',
  `errores_validacion` json DEFAULT NULL COMMENT 'Detalle estructurado de los errores encontrados al validar la carga',
  PRIMARY KEY (`id`),
  KEY `fk_is_interfaz` (`interfaz_importacion_id`),
  CONSTRAINT `fk_is_interfaz` FOREIGN KEY (`interfaz_importacion_id`) REFERENCES `interfaces_importacion` (`id`)
) ENGINE=InnoDB COMMENT='Zona de aterrizaje antes de validar y consolidar'
;

DROP TRIGGER IF EXISTS trg_aud_importaciones_staging_ins;
CREATE TRIGGER trg_aud_importaciones_staging_ins AFTER INSERT ON importaciones_staging
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('importaciones_staging', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'interfaz_importacion_id', NEW.interfaz_importacion_id, 'nombre_archivo_origen', NEW.nombre_archivo_origen, 'fecha_carga', NEW.fecha_carga, 'estado', NEW.estado, 'contenido_raw', NEW.contenido_raw, 'errores_validacion', NEW.errores_validacion));
END;

DROP TRIGGER IF EXISTS trg_aud_importaciones_staging_upd;
CREATE TRIGGER trg_aud_importaciones_staging_upd AFTER UPDATE ON importaciones_staging
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('importaciones_staging', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'interfaz_importacion_id', OLD.interfaz_importacion_id, 'nombre_archivo_origen', OLD.nombre_archivo_origen, 'fecha_carga', OLD.fecha_carga, 'estado', OLD.estado, 'contenido_raw', OLD.contenido_raw, 'errores_validacion', OLD.errores_validacion), JSON_OBJECT('id', NEW.id, 'interfaz_importacion_id', NEW.interfaz_importacion_id, 'nombre_archivo_origen', NEW.nombre_archivo_origen, 'fecha_carga', NEW.fecha_carga, 'estado', NEW.estado, 'contenido_raw', NEW.contenido_raw, 'errores_validacion', NEW.errores_validacion));
END;

DROP TRIGGER IF EXISTS trg_aud_importaciones_staging_del;
CREATE TRIGGER trg_aud_importaciones_staging_del AFTER DELETE ON importaciones_staging
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('importaciones_staging', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'interfaz_importacion_id', OLD.interfaz_importacion_id, 'nombre_archivo_origen', OLD.nombre_archivo_origen, 'fecha_carga', OLD.fecha_carga, 'estado', OLD.estado, 'contenido_raw', OLD.contenido_raw, 'errores_validacion', OLD.errores_validacion), NULL);
END;

CREATE TABLE `calendario_laboral` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'Identificador interno del día calendario',
  `fecha` date NOT NULL COMMENT 'Fecha calendario que se está clasificando',
  `tipo_dia` enum('laboral','feriado','descanso') NOT NULL COMMENT 'Clasificación del día para efectos de cálculo de asistencia/planilla',
  `programa_presupuestario_id` bigint unsigned DEFAULT NULL COMMENT 'Programa presupuestario al que aplica este calendario, si los calendarios difieren por programa; NULL = aplica a todos',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_calendario` (`fecha`,`programa_presupuestario_id`),
  KEY `fk_cal_programa` (`programa_presupuestario_id`),
  CONSTRAINT `fk_cal_programa` FOREIGN KEY (`programa_presupuestario_id`) REFERENCES `programas_presupuestarios` (`id`)
) ENGINE=InnoDB COMMENT='Clasificación de cada fecha como día laboral, feriado o de descanso'
;

DROP TRIGGER IF EXISTS trg_aud_calendario_laboral_ins;
CREATE TRIGGER trg_aud_calendario_laboral_ins AFTER INSERT ON calendario_laboral
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('calendario_laboral', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'fecha', NEW.fecha, 'tipo_dia', NEW.tipo_dia, 'programa_presupuestario_id', NEW.programa_presupuestario_id));
END;

DROP TRIGGER IF EXISTS trg_aud_calendario_laboral_upd;
CREATE TRIGGER trg_aud_calendario_laboral_upd AFTER UPDATE ON calendario_laboral
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('calendario_laboral', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'fecha', OLD.fecha, 'tipo_dia', OLD.tipo_dia, 'programa_presupuestario_id', OLD.programa_presupuestario_id), JSON_OBJECT('id', NEW.id, 'fecha', NEW.fecha, 'tipo_dia', NEW.tipo_dia, 'programa_presupuestario_id', NEW.programa_presupuestario_id));
END;

DROP TRIGGER IF EXISTS trg_aud_calendario_laboral_del;
CREATE TRIGGER trg_aud_calendario_laboral_del AFTER DELETE ON calendario_laboral
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('calendario_laboral', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'fecha', OLD.fecha, 'tipo_dia', OLD.tipo_dia, 'programa_presupuestario_id', OLD.programa_presupuestario_id), NULL);
END;

CREATE TABLE `feriados_patron` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'Identificador interno del patrón de feriado',
  `nombre` varchar(100) NOT NULL COMMENT 'p.ej. "Día de Juan Santamaría", "Semana Santa - Jueves Santo"',
  `tipo_patron` enum('fijo','movible_pascua') NOT NULL DEFAULT 'fijo' COMMENT 'fijo = mismo mes/día cada año; movible_pascua = se calcula por offset de días respecto al Domingo de Pascua',
  `mes` tinyint unsigned DEFAULT NULL COMMENT 'Mes (1-12), solo cuando tipo_patron = fijo',
  `dia` tinyint unsigned DEFAULT NULL COMMENT 'Día del mes, solo cuando tipo_patron = fijo',
  `offset_dias_pascua` smallint DEFAULT NULL COMMENT 'Días de diferencia respecto al Domingo de Pascua (puede ser negativo), solo cuando tipo_patron = movible_pascua',
  `aplica_traslado_lunes` tinyint(1) NOT NULL DEFAULT '0' COMMENT '1 = si cae entre semana se traslada al lunes según Ley de Traslado de Feriados, cuando aplique',
  `pago_obligatorio` tinyint(1) NOT NULL DEFAULT '1' COMMENT '1 = feriado de pago obligatorio (Art. 148 Código de Trabajo), 0 = feriado no obligatorio/opcional',
  `activo` tinyint(1) NOT NULL DEFAULT '1' COMMENT '1 = patrón utilizable para generar calendario, 0 = descontinuado',
  PRIMARY KEY (`id`),
  CONSTRAINT `chk_fp_patron` CHECK ((((`tipo_patron` = _utf8mb4'fijo') and (`mes` is not null) and (`dia` is not null) and (`offset_dias_pascua` is null)) or ((`tipo_patron` = _utf8mb4'movible_pascua') and (`offset_dias_pascua` is not null) and (`mes` is null) and (`dia` is null))))
) ENGINE=InnoDB COMMENT='Patrón recurrente de feriados nacionales, para generar calendario_laboral automáticamente cada año'
;

DROP TRIGGER IF EXISTS trg_aud_feriados_patron_ins;
CREATE TRIGGER trg_aud_feriados_patron_ins AFTER INSERT ON feriados_patron
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('feriados_patron', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'nombre', NEW.nombre, 'tipo_patron', NEW.tipo_patron, 'mes', NEW.mes, 'dia', NEW.dia, 'offset_dias_pascua', NEW.offset_dias_pascua, 'aplica_traslado_lunes', NEW.aplica_traslado_lunes, 'pago_obligatorio', NEW.pago_obligatorio, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_feriados_patron_upd;
CREATE TRIGGER trg_aud_feriados_patron_upd AFTER UPDATE ON feriados_patron
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('feriados_patron', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'nombre', OLD.nombre, 'tipo_patron', OLD.tipo_patron, 'mes', OLD.mes, 'dia', OLD.dia, 'offset_dias_pascua', OLD.offset_dias_pascua, 'aplica_traslado_lunes', OLD.aplica_traslado_lunes, 'pago_obligatorio', OLD.pago_obligatorio, 'activo', OLD.activo), JSON_OBJECT('id', NEW.id, 'nombre', NEW.nombre, 'tipo_patron', NEW.tipo_patron, 'mes', NEW.mes, 'dia', NEW.dia, 'offset_dias_pascua', NEW.offset_dias_pascua, 'aplica_traslado_lunes', NEW.aplica_traslado_lunes, 'pago_obligatorio', NEW.pago_obligatorio, 'activo', NEW.activo));
END;

DROP TRIGGER IF EXISTS trg_aud_feriados_patron_del;
CREATE TRIGGER trg_aud_feriados_patron_del AFTER DELETE ON feriados_patron
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('feriados_patron', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'nombre', OLD.nombre, 'tipo_patron', OLD.tipo_patron, 'mes', OLD.mes, 'dia', OLD.dia, 'offset_dias_pascua', OLD.offset_dias_pascua, 'aplica_traslado_lunes', OLD.aplica_traslado_lunes, 'pago_obligatorio', OLD.pago_obligatorio, 'activo', OLD.activo), NULL);
END;

CREATE TABLE `registro_asistencia` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'Identificador interno del registro de asistencia',
  `empleado_id` bigint unsigned NOT NULL COMMENT 'Empleado al que corresponde el registro',
  `fecha` date NOT NULL COMMENT 'Fecha del registro de asistencia',
  `hora_entrada` time DEFAULT NULL COMMENT 'Hora en que el empleado marcó entrada',
  `hora_salida` time DEFAULT NULL COMMENT 'Hora en que el empleado marcó salida',
  `horas_ordinarias` decimal(6,2) NOT NULL DEFAULT '0.00' COMMENT 'Cantidad de horas ordinarias trabajadas ese día',
  `horas_extra_diurnas` decimal(6,2) NOT NULL DEFAULT '0.00' COMMENT 'Cantidad de horas extra diurnas trabajadas ese día',
  `horas_extra_nocturnas` decimal(6,2) NOT NULL DEFAULT '0.00' COMMENT 'Cantidad de horas extra nocturnas trabajadas ese día',
  `horas_extra_mixtas` decimal(6,2) NOT NULL DEFAULT '0.00' COMMENT 'Cantidad de horas extra mixtas trabajadas ese día',
  `horas_dobles` decimal(6,2) NOT NULL DEFAULT '0.00' COMMENT 'Cantidad de horas dobles trabajadas ese día',
  `observaciones` varchar(255) DEFAULT NULL COMMENT 'Notas adicionales sobre el registro de asistencia de ese día',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_asistencia` (`empleado_id`,`fecha`),
  CONSTRAINT `fk_ra_empleado` FOREIGN KEY (`empleado_id`) REFERENCES `empleados` (`id`)
) ENGINE=InnoDB COMMENT='Fuente de las horas extra diurna/nocturna/mixta y horas dobles'
;

DROP TRIGGER IF EXISTS trg_aud_registro_asistencia_ins;
CREATE TRIGGER trg_aud_registro_asistencia_ins AFTER INSERT ON registro_asistencia
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('registro_asistencia', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'fecha', NEW.fecha, 'hora_entrada', NEW.hora_entrada, 'hora_salida', NEW.hora_salida, 'horas_ordinarias', NEW.horas_ordinarias, 'horas_extra_diurnas', NEW.horas_extra_diurnas, 'horas_extra_nocturnas', NEW.horas_extra_nocturnas, 'horas_extra_mixtas', NEW.horas_extra_mixtas, 'horas_dobles', NEW.horas_dobles, 'observaciones', NEW.observaciones));
END;

DROP TRIGGER IF EXISTS trg_aud_registro_asistencia_upd;
CREATE TRIGGER trg_aud_registro_asistencia_upd AFTER UPDATE ON registro_asistencia
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('registro_asistencia', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'fecha', OLD.fecha, 'hora_entrada', OLD.hora_entrada, 'hora_salida', OLD.hora_salida, 'horas_ordinarias', OLD.horas_ordinarias, 'horas_extra_diurnas', OLD.horas_extra_diurnas, 'horas_extra_nocturnas', OLD.horas_extra_nocturnas, 'horas_extra_mixtas', OLD.horas_extra_mixtas, 'horas_dobles', OLD.horas_dobles, 'observaciones', OLD.observaciones), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'fecha', NEW.fecha, 'hora_entrada', NEW.hora_entrada, 'hora_salida', NEW.hora_salida, 'horas_ordinarias', NEW.horas_ordinarias, 'horas_extra_diurnas', NEW.horas_extra_diurnas, 'horas_extra_nocturnas', NEW.horas_extra_nocturnas, 'horas_extra_mixtas', NEW.horas_extra_mixtas, 'horas_dobles', NEW.horas_dobles, 'observaciones', NEW.observaciones));
END;

DROP TRIGGER IF EXISTS trg_aud_registro_asistencia_del;
CREATE TRIGGER trg_aud_registro_asistencia_del AFTER DELETE ON registro_asistencia
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('registro_asistencia', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'fecha', OLD.fecha, 'hora_entrada', OLD.hora_entrada, 'hora_salida', OLD.hora_salida, 'horas_ordinarias', OLD.horas_ordinarias, 'horas_extra_diurnas', OLD.horas_extra_diurnas, 'horas_extra_nocturnas', OLD.horas_extra_nocturnas, 'horas_extra_mixtas', OLD.horas_extra_mixtas, 'horas_dobles', OLD.horas_dobles, 'observaciones', OLD.observaciones), NULL);
END;

CREATE TABLE `vacaciones_disfrute` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'Identificador interno del disfrute',
  `empleado_id` bigint unsigned NOT NULL COMMENT 'Empleado que disfrutó las vacaciones',
  `fecha_inicio` date NOT NULL COMMENT 'Fecha en que inició el disfrute de vacaciones',
  `fecha_fin` date NOT NULL COMMENT 'Fecha en que finalizó el disfrute de vacaciones',
  `dias` decimal(6,2) NOT NULL COMMENT 'Cantidad de días de vacaciones disfrutados en este período',
  PRIMARY KEY (`id`),
  KEY `fk_vd_empleado` (`empleado_id`),
  CONSTRAINT `fk_vd_empleado` FOREIGN KEY (`empleado_id`) REFERENCES `empleados` (`id`),
  CONSTRAINT `chk_vd_fechas` CHECK ((`fecha_fin` >= `fecha_inicio`))
) ENGINE=InnoDB COMMENT='Historial de períodos de vacaciones disfrutados por empleado'
;

DROP TRIGGER IF EXISTS trg_aud_vacaciones_disfrute_ins;
CREATE TRIGGER trg_aud_vacaciones_disfrute_ins AFTER INSERT ON vacaciones_disfrute
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('vacaciones_disfrute', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'fecha_inicio', NEW.fecha_inicio, 'fecha_fin', NEW.fecha_fin, 'dias', NEW.dias));
END;

DROP TRIGGER IF EXISTS trg_aud_vacaciones_disfrute_upd;
CREATE TRIGGER trg_aud_vacaciones_disfrute_upd AFTER UPDATE ON vacaciones_disfrute
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('vacaciones_disfrute', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'fecha_inicio', OLD.fecha_inicio, 'fecha_fin', OLD.fecha_fin, 'dias', OLD.dias), JSON_OBJECT('id', NEW.id, 'empleado_id', NEW.empleado_id, 'fecha_inicio', NEW.fecha_inicio, 'fecha_fin', NEW.fecha_fin, 'dias', NEW.dias));
END;

DROP TRIGGER IF EXISTS trg_aud_vacaciones_disfrute_del;
CREATE TRIGGER trg_aud_vacaciones_disfrute_del AFTER DELETE ON vacaciones_disfrute
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('vacaciones_disfrute', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'empleado_id', OLD.empleado_id, 'fecha_inicio', OLD.fecha_inicio, 'fecha_fin', OLD.fecha_fin, 'dias', OLD.dias), NULL);
END;

CREATE TABLE `plantillas_asiento_linea` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'Identificador interno de la línea plantilla',
  `tipo_asiento_id` bigint unsigned NOT NULL COMMENT 'Tipo de asiento al que pertenece esta línea plantilla',
  `descripcion_linea` varchar(200) NOT NULL COMMENT 'Descripción de la línea (aparecerá en el asiento generado)',
  `tipo_movimiento` enum('debe','haber') NOT NULL COMMENT 'Si la línea es un movimiento débito o crédito',
  `orden` int NOT NULL COMMENT 'Posición de la línea dentro del asiento generado',
  PRIMARY KEY (`id`),
  KEY `fk_pal_tipo` (`tipo_asiento_id`),
  CONSTRAINT `fk_pal_tipo` FOREIGN KEY (`tipo_asiento_id`) REFERENCES `tipos_asiento` (`id`)
) ENGINE=InnoDB COMMENT='Plantilla de líneas por tipo de asiento, configurable sin tocar código'
;

DROP TRIGGER IF EXISTS trg_aud_plantillas_asiento_linea_ins;
CREATE TRIGGER trg_aud_plantillas_asiento_linea_ins AFTER INSERT ON plantillas_asiento_linea
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('plantillas_asiento_linea', NEW.id, @auditoria_usuario_id, 'insert', NULL, JSON_OBJECT('id', NEW.id, 'tipo_asiento_id', NEW.tipo_asiento_id, 'descripcion_linea', NEW.descripcion_linea, 'tipo_movimiento', NEW.tipo_movimiento, 'orden', NEW.orden));
END;

DROP TRIGGER IF EXISTS trg_aud_plantillas_asiento_linea_upd;
CREATE TRIGGER trg_aud_plantillas_asiento_linea_upd AFTER UPDATE ON plantillas_asiento_linea
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('plantillas_asiento_linea', NEW.id, @auditoria_usuario_id, 'update', JSON_OBJECT('id', OLD.id, 'tipo_asiento_id', OLD.tipo_asiento_id, 'descripcion_linea', OLD.descripcion_linea, 'tipo_movimiento', OLD.tipo_movimiento, 'orden', OLD.orden), JSON_OBJECT('id', NEW.id, 'tipo_asiento_id', NEW.tipo_asiento_id, 'descripcion_linea', NEW.descripcion_linea, 'tipo_movimiento', NEW.tipo_movimiento, 'orden', NEW.orden));
END;

DROP TRIGGER IF EXISTS trg_aud_plantillas_asiento_linea_del;
CREATE TRIGGER trg_aud_plantillas_asiento_linea_del AFTER DELETE ON plantillas_asiento_linea
FOR EACH ROW
BEGIN
  INSERT INTO auditoria_cambios (tabla_afectada, registro_id, usuario_id, accion, valor_anterior, valor_nuevo)
  VALUES ('plantillas_asiento_linea', OLD.id, @auditoria_usuario_id, 'delete', JSON_OBJECT('id', OLD.id, 'tipo_asiento_id', OLD.tipo_asiento_id, 'descripcion_linea', OLD.descripcion_linea, 'tipo_movimiento', OLD.tipo_movimiento, 'orden', OLD.orden), NULL);
END;

