-- 000112_permisos_y_roles.up.sql
-- F12: alinea el catálogo de permisos con internal/authz, define los roles
-- reales con segregación de funciones (preparar / aprobar / pagar) y retira
-- los permisos "gestionar/ejecutar" que ninguna ruta usa.
--
-- Reglas de esta migración:
--   * Roles y permisos se resuelven SIEMPRE por codigo, nunca por id fijo.
--   * Solo INSERT ... SELECT ... WHERE NOT EXISTS: no se usan las variantes que
--     pisan datos existentes ni las que ocultan fallos.
--   * No contiene credenciales. El usuario administrador inicial se crea con
--     cmd/seedadmin.
--
-- La fuente de verdad del catálogo es internal/authz.Todos(). El test
-- internal/authz/migraciones_test.go falla si este archivo y authz se
-- desalinean (incluidos los permisos que esta migración elimina).

-- ---------------------------------------------------------------------------
-- 1) permisos: sembrar los que falten (los ya existentes no se tocan)
-- ---------------------------------------------------------------------------
INSERT INTO permisos (codigo, nombre, modulo)
SELECT n.codigo, n.nombre, n.modulo
FROM (
  SELECT 'empleados.crear' AS codigo, 'Crear expedientes de empleados' AS nombre, 'empleados' AS modulo
  UNION ALL SELECT 'empleados.leer', 'Consultar empleados', 'empleados'
  UNION ALL SELECT 'empleados.actualizar', 'Actualizar datos de empleados', 'empleados'
  UNION ALL SELECT 'contratos.crear', 'Crear contratos laborales', 'empleados'
  UNION ALL SELECT 'contratos.leer', 'Consultar contratos laborales', 'empleados'
  UNION ALL SELECT 'contratos.actualizar', 'Actualizar contratos laborales', 'empleados'
  UNION ALL SELECT 'organizacion.crear', 'Crear catálogos organizacionales', 'empleados'
  UNION ALL SELECT 'organizacion.leer', 'Consultar catálogos organizacionales', 'empleados'
  UNION ALL SELECT 'organizacion.actualizar', 'Actualizar catálogos organizacionales', 'empleados'
  UNION ALL SELECT 'datos_personales.ver_sensible', 'Ver datos personales sensibles', 'empleados'
  UNION ALL SELECT 'asistencia.crear', 'Registrar asistencia', 'asistencia'
  UNION ALL SELECT 'asistencia.ver', 'Ver asistencia', 'asistencia'
  UNION ALL SELECT 'asistencia.editar', 'Editar asistencia', 'asistencia'
  UNION ALL SELECT 'calendario.administrar', 'Administrar calendario laboral y turnos', 'asistencia'
  UNION ALL SELECT 'incapacidades.gestionar', 'Gestionar incapacidades CCSS/INS', 'asistencia'
  UNION ALL SELECT 'vacaciones.solicitar', 'Solicitar vacaciones', 'asistencia'
  UNION ALL SELECT 'vacaciones.aprobar', 'Aprobar o rechazar solicitudes de vacaciones', 'asistencia'
  UNION ALL SELECT 'vacaciones.registrar_disfrute', 'Registrar el disfrute de vacaciones aprobadas', 'asistencia'
  UNION ALL SELECT 'vacaciones.ver', 'Ver vacaciones', 'asistencia'
  UNION ALL SELECT 'vacaciones.solicitar_terceros', 'Solicitar vacaciones en nombre de terceros', 'asistencia'
  UNION ALL SELECT 'vacaciones.ver_terceros', 'Ver vacaciones de terceros', 'asistencia'
  UNION ALL SELECT 'prestamos.crear', 'Crear préstamos de empleados', 'planilla'
  UNION ALL SELECT 'prestamos.ver', 'Consultar préstamos de empleados', 'planilla'
  UNION ALL SELECT 'prestamos.abonar', 'Aplicar abonos a préstamos', 'planilla'
  UNION ALL SELECT 'prestamos.cancelar', 'Cancelar préstamos', 'planilla'
  UNION ALL SELECT 'embargos.crear', 'Crear embargos', 'planilla'
  UNION ALL SELECT 'embargos.ver', 'Consultar embargos', 'planilla'
  UNION ALL SELECT 'embargos.cambiar_estado', 'Cambiar el estado de un embargo', 'planilla'
  UNION ALL SELECT 'terceros.crear', 'Crear terceros acreedores', 'planilla'
  UNION ALL SELECT 'terceros.ver', 'Consultar terceros acreedores', 'planilla'
  UNION ALL SELECT 'liquidaciones.proyectar', 'Proyectar liquidaciones laborales', 'planilla'
  UNION ALL SELECT 'liquidaciones.procesar', 'Procesar liquidaciones laborales', 'planilla'
  UNION ALL SELECT 'planilla.crear', 'Crear períodos y corridas de planilla', 'planilla'
  UNION ALL SELECT 'planilla.calcular', 'Calcular planilla', 'planilla'
  UNION ALL SELECT 'planilla.ver', 'Consultar planilla', 'planilla'
  UNION ALL SELECT 'planilla.aprobar', 'Aprobar planilla', 'planilla'
  UNION ALL SELECT 'planilla.pagar', 'Registrar el pago de planilla', 'planilla'
  UNION ALL SELECT 'planilla.exportar_bancario', 'Generar archivo de dispersión bancaria', 'planilla'
  UNION ALL SELECT 'planilla.ver_boleta', 'Consultar boleta de pago de un empleado', 'planilla'
  UNION ALL SELECT 'planilla.ver_boleta_propia', 'Consultar la boleta de pago propia', 'planilla'
  UNION ALL SELECT 'aguinaldo.ejecutar', 'Ejecutar el cálculo de aguinaldo', 'planilla'
  UNION ALL SELECT 'salario_escolar.ejecutar', 'Ejecutar el cálculo de salario escolar', 'planilla'
  UNION ALL SELECT 'dashboard.ver', 'Ver el dashboard gerencial', 'planilla'
  UNION ALL SELECT 'parametros.ver', 'Consultar parámetros de planilla', 'parametros'
  UNION ALL SELECT 'parametros.proponer', 'Proponer cambios a parámetros de planilla', 'parametros'
  UNION ALL SELECT 'parametros.aprobar', 'Aprobar cambios a parámetros de planilla', 'parametros'
  UNION ALL SELECT 'asientos_contables.generar', 'Generar los asientos contables de un período de planilla', 'contabilizacion'
  UNION ALL SELECT 'asientos_contables.ver', 'Consultar asientos contables generados', 'contabilizacion'
  UNION ALL SELECT 'reportes.ccss', 'Generar reportes para la CCSS', 'integraciones'
  UNION ALL SELECT 'reportes.ins', 'Generar reportes para el INS', 'integraciones'
  UNION ALL SELECT 'reportes.hacienda', 'Generar reportes para Hacienda', 'integraciones'
  UNION ALL SELECT 'usuarios.asignar_rol', 'Asignar y revocar roles de usuarios', 'auditoria'
  UNION ALL SELECT 'usuarios.gestionar_sesiones', 'Cerrar todas las sesiones activas', 'auditoria'
  UNION ALL SELECT 'usuarios.crear', 'Crear usuarios del sistema', 'auditoria'
  UNION ALL SELECT 'usuarios.ver', 'Consultar usuarios del sistema', 'auditoria'
  UNION ALL SELECT 'usuarios.actualizar', 'Actualizar usuarios del sistema', 'auditoria'
  UNION ALL SELECT 'usuarios.reset_password', 'Restablecer contraseñas de usuarios', 'auditoria'
  UNION ALL SELECT 'auditoria.ver', 'Consultar la bitácora de auditoría', 'auditoria'
) AS n
WHERE NOT EXISTS (SELECT 1 FROM permisos p WHERE p.codigo = n.codigo);

-- ---------------------------------------------------------------------------
-- 2) roles
--    El rol ADMINISTRADOR (creado en 000006) pasa a ser ADMINISTRADOR_SISTEMA.
--    El UPDATE solo corre si el nombre nuevo no existe todavia. La subconsulta
--    derivada evita el error 1093 de MySQL (leer la tabla que se actualiza).
-- ---------------------------------------------------------------------------
UPDATE roles
SET codigo = 'ADMINISTRADOR_SISTEMA',
    nombre = 'Administrador del sistema'
WHERE codigo = 'ADMINISTRADOR'
  AND NOT EXISTS (SELECT 1 FROM (SELECT id FROM roles WHERE codigo = 'ADMINISTRADOR_SISTEMA') AS ya);

INSERT INTO roles (codigo, nombre, descripcion)
SELECT n.codigo, n.nombre, n.descripcion
FROM (
  SELECT 'ADMINISTRADOR_SISTEMA' AS codigo, 'Administrador del sistema' AS nombre, 'Administra usuarios, roles y sesiones. Sin permisos operativos de planilla.' AS descripcion
  UNION ALL SELECT 'RRHH', 'Recursos Humanos', 'Expedientes, contratos, asistencia, vacaciones, préstamos, embargos y terceros.'
  UNION ALL SELECT 'PLANILLAS_PREPARA', 'Planillas - Preparación', 'Prepara y calcula planillas, aguinaldo, salario escolar y liquidaciones. No aprueba ni paga.'
  UNION ALL SELECT 'PLANILLAS_APRUEBA', 'Planillas - Aprobación', 'Aprueba planillas y parámetros. No prepara ni paga.'
  UNION ALL SELECT 'TESORERIA', 'Tesorería', 'Exporta la dispersión bancaria y registra el pago. No prepara ni aprueba.'
  UNION ALL SELECT 'CONTABILIDAD', 'Contabilidad', 'Genera y consulta asientos contables y reportes.'
  UNION ALL SELECT 'AUDITOR', 'Auditor', 'Solo lectura sobre todo el sistema.'
) AS n
WHERE NOT EXISTS (SELECT 1 FROM roles r WHERE r.codigo = n.codigo);

-- ---------------------------------------------------------------------------
-- 3) Retirar de ADMINISTRADOR_SISTEMA los permisos operativos heredados de
--    000006 que no corresponden a su perfil (segregacion de funciones).
-- ---------------------------------------------------------------------------
DELETE rp
FROM rol_permisos rp
JOIN roles r ON r.id = rp.rol_id
JOIN permisos p ON p.id = rp.permiso_id
WHERE r.codigo = 'ADMINISTRADOR_SISTEMA'
  AND p.codigo IN ('asientos_contables.generar', 'asientos_contables.ver');

-- ---------------------------------------------------------------------------
-- 4) Asignacion inicial de permisos por rol (ajustable despues)
-- ---------------------------------------------------------------------------

-- ADMINISTRADOR_SISTEMA
INSERT INTO rol_permisos (rol_id, permiso_id)
SELECT r.id, p.id
FROM roles r
JOIN permisos p ON p.codigo IN (
    'usuarios.asignar_rol',
    'usuarios.gestionar_sesiones',
    'usuarios.crear',
    'usuarios.ver',
    'usuarios.actualizar',
    'usuarios.reset_password',
    'auditoria.ver',
    'dashboard.ver',
    'parametros.ver'
)
WHERE r.codigo = 'ADMINISTRADOR_SISTEMA'
  AND NOT EXISTS (
    SELECT 1 FROM rol_permisos rp WHERE rp.rol_id = r.id AND rp.permiso_id = p.id
  );


-- RRHH
INSERT INTO rol_permisos (rol_id, permiso_id)
SELECT r.id, p.id
FROM roles r
JOIN permisos p ON p.codigo IN (
    'empleados.crear',
    'empleados.leer',
    'empleados.actualizar',
    'contratos.crear',
    'contratos.leer',
    'contratos.actualizar',
    'organizacion.crear',
    'organizacion.leer',
    'organizacion.actualizar',
    'asistencia.crear',
    'asistencia.ver',
    'asistencia.editar',
    'calendario.administrar',
    'incapacidades.gestionar',
    'vacaciones.solicitar',
    'vacaciones.aprobar',
    'vacaciones.registrar_disfrute',
    'vacaciones.ver',
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
    'dashboard.ver'
)
WHERE r.codigo = 'RRHH'
  AND NOT EXISTS (
    SELECT 1 FROM rol_permisos rp WHERE rp.rol_id = r.id AND rp.permiso_id = p.id
  );


-- PLANILLAS_PREPARA
INSERT INTO rol_permisos (rol_id, permiso_id)
SELECT r.id, p.id
FROM roles r
JOIN permisos p ON p.codigo IN (
    'planilla.crear',
    'planilla.calcular',
    'planilla.ver',
    'planilla.ver_boleta',
    'aguinaldo.ejecutar',
    'salario_escolar.ejecutar',
    'liquidaciones.proyectar',
    'liquidaciones.procesar',
    'parametros.proponer',
    'reportes.ccss',
    'reportes.ins',
    'reportes.hacienda'
)
WHERE r.codigo = 'PLANILLAS_PREPARA'
  AND NOT EXISTS (
    SELECT 1 FROM rol_permisos rp WHERE rp.rol_id = r.id AND rp.permiso_id = p.id
  );


-- PLANILLAS_APRUEBA
INSERT INTO rol_permisos (rol_id, permiso_id)
SELECT r.id, p.id
FROM roles r
JOIN permisos p ON p.codigo IN (
    'planilla.ver',
    'planilla.aprobar',
    'planilla.ver_boleta',
    'parametros.aprobar',
    'dashboard.ver'
)
WHERE r.codigo = 'PLANILLAS_APRUEBA'
  AND NOT EXISTS (
    SELECT 1 FROM rol_permisos rp WHERE rp.rol_id = r.id AND rp.permiso_id = p.id
  );


-- TESORERIA
INSERT INTO rol_permisos (rol_id, permiso_id)
SELECT r.id, p.id
FROM roles r
JOIN permisos p ON p.codigo IN (
    'planilla.ver',
    'planilla.exportar_bancario',
    'planilla.pagar'
)
WHERE r.codigo = 'TESORERIA'
  AND NOT EXISTS (
    SELECT 1 FROM rol_permisos rp WHERE rp.rol_id = r.id AND rp.permiso_id = p.id
  );


-- CONTABILIDAD
INSERT INTO rol_permisos (rol_id, permiso_id)
SELECT r.id, p.id
FROM roles r
JOIN permisos p ON p.codigo IN (
    'asientos_contables.generar',
    'asientos_contables.ver',
    'planilla.ver',
    'reportes.ccss',
    'reportes.ins',
    'reportes.hacienda'
)
WHERE r.codigo = 'CONTABILIDAD'
  AND NOT EXISTS (
    SELECT 1 FROM rol_permisos rp WHERE rp.rol_id = r.id AND rp.permiso_id = p.id
  );


-- AUDITOR
INSERT INTO rol_permisos (rol_id, permiso_id)
SELECT r.id, p.id
FROM roles r
JOIN permisos p ON p.codigo IN (
    'empleados.leer',
    'contratos.leer',
    'organizacion.leer',
    'asistencia.ver',
    'vacaciones.ver',
    'prestamos.ver',
    'embargos.ver',
    'terceros.ver',
    'planilla.ver',
    'dashboard.ver',
    'parametros.ver',
    'asientos_contables.ver',
    'usuarios.ver',
    'auditoria.ver'
)
WHERE r.codigo = 'AUDITOR'
  AND NOT EXISTS (
    SELECT 1 FROM rol_permisos rp WHERE rp.rol_id = r.id AND rp.permiso_id = p.id
  );

-- ---------------------------------------------------------------------------
-- 5) Eliminar permisos obsoletos de 000006 (ninguna ruta ni constante de
--    internal/authz los usa). Primero sus asignaciones, luego el permiso.
--    Si algun codigo fuera de las rutas revisadas aun los usara, NO aplicar:
--    buscar antes en el repo (ver nota de entrega).
-- ---------------------------------------------------------------------------
DELETE rp
FROM rol_permisos rp
JOIN permisos p ON p.id = rp.permiso_id
WHERE p.codigo IN (
    'empleados.gestionar',
    'contratos.gestionar',
    'asistencia.gestionar',
    'vacaciones.gestionar',
    'prestamos_embargos.gestionar',
    'planilla.ejecutar',
    'asientos_contables.gestionar',
    'usuarios_roles.administrar'
);

DELETE FROM permisos
WHERE codigo IN (
    'empleados.gestionar',
    'contratos.gestionar',
    'asistencia.gestionar',
    'vacaciones.gestionar',
    'prestamos_embargos.gestionar',
    'planilla.ejecutar',
    'asientos_contables.gestionar',
    'usuarios_roles.administrar'
);

-- ---------------------------------------------------------------------------
-- 6) Cuenta heredada de 000006 (version anterior): si en algun ambiente ya se
--    aplico esa migracion, existe un usuario admin con una contrasena temporal
--    conocida. Se deshabilita solo si sigue sin cambio de contrasena, que es la
--    senal de que nadie la reemplazo. El administrador real se crea con
--    cmd/seedadmin (con otro nombre de usuario, porque admin queda ocupado).
-- ---------------------------------------------------------------------------
UPDATE usuarios
SET activo = 0
WHERE nombre_usuario = 'admin'
  AND correo = 'admin@ccdrliberia.local'
  AND fecha_ultimo_cambio_password IS NULL
  AND activo = 1;