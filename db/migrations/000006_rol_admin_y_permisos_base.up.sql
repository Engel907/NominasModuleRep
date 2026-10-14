-- F12: esta migración YA NO crea ningún usuario ni contiene credenciales. El
-- administrador inicial se crea con cmd/seedadmin (contraseña aleatoria, se
-- imprime una sola vez). El rol se renombra a ADMINISTRADOR_SISTEMA y los
-- permisos "gestionar" de abajo se retiran en 000112_permisos_y_roles.
-- ============================================================================
-- Origen: Fase 3C — Rol ADMINISTRADOR y catálogo base de permisos
--
-- `permisos` ya tenía 8 códigos granulares sembrados en la Fase 4
-- (asistencia.ver/crear/editar, calendario.administrar,
-- incapacidades.gestionar, vacaciones.ver/solicitar/aprobar), todos con
-- modulo='asistencia'. Los códigos de abajo son deliberadamente distintos
-- (no duplican esos): son permisos de nivel "gestionar/administrar todo el
-- módulo", uno por cada acción administrativa listada en el enunciado,
-- pensados para un rol amplio como ADMINISTRADOR -- no reemplazan ni
-- reinsertan los granulares existentes.
--
-- Convención de codigo seguida (misma que la Fase 4): 'dominio.accion' en
-- minúsculas. Convención de modulo seguida (mismo vocabulario del COMMENT
-- de la columna: empleados, asistencia, parametros, planilla,
-- contabilizacion, integraciones, auditoria).
--
-- ASUNCIÓN A CONFIRMAR — modulo de 'usuarios_roles.administrar': ninguno de
-- los 7 valores de modulo documentados en el COMMENT de la columna cubre
-- administración de usuarios/roles del propio sistema. Se usa 'auditoria'
-- por ser el más cercano (permisos ya declara su propósito como "base para
-- segregación de funciones exigida por Ley 8292", y administrar
-- usuarios/roles es el núcleo de esa segregación) -- confirmar con el
-- equipo si prefieren introducir un valor de modulo nuevo (p.ej.
-- 'seguridad') en su lugar.
-- ============================================================================

-- -----------------------------------------------------------------------------
-- 1) permisos — catálogo base, uno por acción administrativa relevante.
--    Mismo estilo (INSERT IGNORE) que el bloque de permisos ya existente en
--    Fase 4, por consistencia dentro de la misma tabla.
-- -----------------------------------------------------------------------------
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

-- -----------------------------------------------------------------------------
-- 2) roles — rol ADMINISTRADOR.
-- -----------------------------------------------------------------------------
INSERT INTO roles (codigo, nombre)
SELECT 'ADMINISTRADOR', 'Administrador del sistema'
WHERE NOT EXISTS (SELECT 1 FROM roles WHERE codigo = 'ADMINISTRADOR');

-- -----------------------------------------------------------------------------
-- 3) rol_permisos — asigna al rol ADMINISTRADOR los 10 permisos del punto 1
--    (los 8 originales + asientos_contables.generar/.ver de la Fase 11B).
--    rol_id y permiso_id se resuelven ambos por codigo (nunca IDs fijos).
--    rol_permisos no tiene columnas propias más allá de la PK compuesta, así
--    que INSERT IGNORE es suficiente para la idempotencia.
-- -----------------------------------------------------------------------------
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