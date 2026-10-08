-- down 000003_calendario_asistencia_vacaciones
-- (turnos y tipos_ausencia pertenecen a 000000; NO se tocan aquí)
ALTER TABLE vacaciones_acumulado DROP COLUMN dias_disponibles;
DROP TABLE IF EXISTS vacaciones_movimientos, vacaciones_solicitudes, registro_asistencia_detalle, incapacidades, calendario_laboral_dia, feriados;
DELETE rp FROM rol_permisos rp JOIN permisos p ON p.id = rp.permiso_id WHERE p.codigo IN ('asistencia.ver','asistencia.crear','asistencia.editar','calendario.administrar','incapacidades.gestionar','vacaciones.ver','vacaciones.solicitar','vacaciones.aprobar','vacaciones.registrar_disfrute');
DELETE FROM permisos WHERE codigo IN ('asistencia.ver','asistencia.crear','asistencia.editar','calendario.administrar','incapacidades.gestionar','vacaciones.ver','vacaciones.solicitar','vacaciones.aprobar','vacaciones.registrar_disfrute');
