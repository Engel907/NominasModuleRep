-- 000112_permisos_y_roles.down.sql
-- Revierte la estructura de permisos/roles de F12. LIMITACIONES:
--   * No revierte el UPDATE que deshabilito la cuenta heredada admin (6 del up):
--     reactivar una cuenta con contrasena conocida no es un rollback razonable.
--   * Falla (a proposito) si hay usuario_roles apuntando a los roles nuevos:
--     primero hay que revocar esas asignaciones.

-- 1) Restaurar los permisos obsoletos de 000006 y su asignacion al rol original
INSERT INTO permisos (codigo, nombre, modulo)
SELECT n.codigo, n.nombre, n.modulo
FROM (
  SELECT 'empleados.gestionar' AS codigo, 'Gestionar empleados (expediente, puestos, datos laborales)' AS nombre, 'empleados' AS modulo
  UNION ALL SELECT 'contratos.gestionar', 'Gestionar contratos laborales', 'empleados'
  UNION ALL SELECT 'asistencia.gestionar', 'Administrar el modulo de asistencia', 'asistencia'
  UNION ALL SELECT 'vacaciones.gestionar', 'Administrar el modulo de vacaciones', 'asistencia'
  UNION ALL SELECT 'prestamos_embargos.gestionar', 'Gestionar prestamos y embargos de empleados', 'planilla'
  UNION ALL SELECT 'planilla.ejecutar', 'Ejecutar corridas de planilla', 'planilla'
  UNION ALL SELECT 'asientos_contables.gestionar', 'Generar y exportar asientos contables', 'contabilizacion'
  UNION ALL SELECT 'usuarios_roles.administrar', 'Administrar usuarios, roles y permisos del sistema', 'auditoria'
) AS n
WHERE NOT EXISTS (SELECT 1 FROM permisos p WHERE p.codigo = n.codigo);

-- 2) Quitar las asignaciones de los roles nuevos y de ADMINISTRADOR_SISTEMA
DELETE rp
FROM rol_permisos rp
JOIN roles r ON r.id = rp.rol_id
WHERE r.codigo IN ('ADMINISTRADOR_SISTEMA', 'RRHH', 'PLANILLAS_PREPARA', 'PLANILLAS_APRUEBA', 'TESORERIA', 'CONTABILIDAD', 'AUDITOR');

-- 3) Borrar los roles nuevos (falla si tienen usuarios asignados)
DELETE FROM roles
WHERE codigo IN ('RRHH', 'PLANILLAS_PREPARA', 'PLANILLAS_APRUEBA', 'TESORERIA', 'CONTABILIDAD', 'AUDITOR');

-- 4) Devolver el nombre original del rol administrador
UPDATE roles
SET codigo = 'ADMINISTRADOR',
    nombre = 'Administrador del sistema'
WHERE codigo = 'ADMINISTRADOR_SISTEMA'
  AND NOT EXISTS (SELECT 1 FROM (SELECT id FROM roles WHERE codigo = 'ADMINISTRADOR') AS ya);

-- 5) Reasignar al rol original sus 10 permisos de 000006
INSERT INTO rol_permisos (rol_id, permiso_id)
SELECT r.id, p.id
FROM roles r
JOIN permisos p ON p.codigo IN (
    'empleados.gestionar',
    'contratos.gestionar',
    'asistencia.gestionar',
    'vacaciones.gestionar',
    'prestamos_embargos.gestionar',
    'planilla.ejecutar',
    'asientos_contables.gestionar',
    'usuarios_roles.administrar',
    'asientos_contables.generar',
    'asientos_contables.ver'
)
WHERE r.codigo = 'ADMINISTRADOR'
  AND NOT EXISTS (
    SELECT 1 FROM rol_permisos rp WHERE rp.rol_id = r.id AND rp.permiso_id = p.id
  );

-- 6) Borrar los permisos que agrego el up (los previos de 000003/000006/000109 se conservan)
DELETE rp
FROM rol_permisos rp
JOIN permisos p ON p.id = rp.permiso_id
WHERE p.codigo IN (
    'empleados.crear',
    'empleados.leer',
    'empleados.actualizar',
    'contratos.crear',
    'contratos.leer',
    'contratos.actualizar',
    'organizacion.crear',
    'organizacion.leer',
    'organizacion.actualizar',
    'datos_personales.ver_sensible',
    'vacaciones.solicitar_terceros',
    'vacaciones.ver_terceros',
    'prestamos.crear',
    'prestamos.ver',
    'prestamos.abonar',
    'prestamos.cancelar',
    'embargos.crear',
    'embargos.ver',
    'embargos.cambiar_estado',
    'terceros.crear',
    'terceros.ver',
    'liquidaciones.proyectar',
    'liquidaciones.procesar',
    'planilla.crear',
    'planilla.calcular',
    'planilla.ver',
    'planilla.aprobar',
    'planilla.pagar',
    'planilla.ver_boleta_propia',
    'aguinaldo.ejecutar',
    'salario_escolar.ejecutar',
    'dashboard.ver',
    'parametros.ver',
    'parametros.proponer',
    'parametros.aprobar',
    'reportes.ccss',
    'reportes.ins',
    'reportes.hacienda',
    'usuarios.asignar_rol',
    'usuarios.gestionar_sesiones',
    'usuarios.crear',
    'usuarios.ver',
    'usuarios.actualizar',
    'usuarios.reset_password',
    'auditoria.ver'
);

DELETE FROM permisos
WHERE codigo IN (
    'empleados.crear',
    'empleados.leer',
    'empleados.actualizar',
    'contratos.crear',
    'contratos.leer',
    'contratos.actualizar',
    'organizacion.crear',
    'organizacion.leer',
    'organizacion.actualizar',
    'datos_personales.ver_sensible',
    'vacaciones.solicitar_terceros',
    'vacaciones.ver_terceros',
    'prestamos.crear',
    'prestamos.ver',
    'prestamos.abonar',
    'prestamos.cancelar',
    'embargos.crear',
    'embargos.ver',
    'embargos.cambiar_estado',
    'terceros.crear',
    'terceros.ver',
    'liquidaciones.proyectar',
    'liquidaciones.procesar',
    'planilla.crear',
    'planilla.calcular',
    'planilla.ver',
    'planilla.aprobar',
    'planilla.pagar',
    'planilla.ver_boleta_propia',
    'aguinaldo.ejecutar',
    'salario_escolar.ejecutar',
    'dashboard.ver',
    'parametros.ver',
    'parametros.proponer',
    'parametros.aprobar',
    'reportes.ccss',
    'reportes.ins',
    'reportes.hacienda',
    'usuarios.asignar_rol',
    'usuarios.gestionar_sesiones',
    'usuarios.crear',
    'usuarios.ver',
    'usuarios.actualizar',
    'usuarios.reset_password',
    'auditoria.ver'
);