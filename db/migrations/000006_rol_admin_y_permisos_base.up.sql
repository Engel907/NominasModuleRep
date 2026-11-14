INSERT IGNORE INTO permisos (codigo, nombre, modulo) VALUES
    ('empleados.gestionar',           'Gestionar empleados (expediente, puestos, datos laborales)', 'empleados'),
    ('contratos.gestionar',           'Gestionar contratos laborales',                              'empleados'),
    ('asistencia.gestionar',          'Administrar el módulo de asistencia',                        'asistencia'),
    ('vacaciones.gestionar',          'Administrar el módulo de vacaciones',                        'asistencia'),
    ('prestamos_embargos.gestionar',  'Gestionar préstamos y embargos de empleados',                'planilla'),
    ('planilla.ejecutar',             'Ejecutar corridas de planilla',                              'planilla'),
    ('asientos_contables.gestionar',  'Generar y exportar asientos contables',                      'contabilizacion'),
    ('asientos_contables.generar',    'Generar los asientos contables de un período de planilla',   'contabilizacion'),
    ('asientos_contables.ver',        'Consultar asientos contables generados',                     'contabilizacion'),
    ('usuarios_roles.administrar',    'Administrar usuarios, roles y permisos del sistema',         'auditoria');

INSERT INTO roles (codigo, nombre)
SELECT 'ADMINISTRADOR', 'Administrador del sistema'
WHERE NOT EXISTS (SELECT 1 FROM roles WHERE codigo = 'ADMINISTRADOR');

INSERT IGNORE INTO rol_permisos (rol_id, permiso_id)
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
    'asientos_contables.generar',
    'asientos_contables.ver',
    'usuarios_roles.administrar'
)
WHERE r.codigo = 'ADMINISTRADOR';