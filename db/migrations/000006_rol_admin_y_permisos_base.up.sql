-- [split] AVISO DE SEGURIDAD: esta migración siembra el usuario 'admin' con contraseña conocida.
-- [split] NO aplicar en ambientes compartidos ni producción; la fase F12 la reemplaza por cmd/seedadmin.
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
VALUES ('ADMINISTRADOR', 'Administrador del sistema')
ON DUPLICATE KEY UPDATE
    nombre = VALUES(nombre);

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

-- -----------------------------------------------------------------------------
-- 4) usuarios — cuenta administradora inicial ("seed" del rol ADMINISTRADOR
--    de arriba). Sin esta cuenta, ADMINISTRADOR queda sin nadie a quien
--    asignárselo y el sistema no tiene forma de arrancar (nadie con
--    'usuarios_roles.administrar' para dar de alta al resto).
--
--    HASH ARGON2ID — contraseña temporal `CambiarEsta123!`
--    Generado OFFLINE (sin correr el backend) con los MISMOS parámetros de
--    costo que internal/security/hash.go::paramsArgon2id:
--      memory=64*1024 KiB, iterations=3, parallelism=2, saltLength=16,
--      keyLength=32 -- mismo formato PHC ($argon2id$v=19$m=...,t=...,p=...$
--      salt$hash) que produce argon2id.CreateHash en Go. Verificado con
--      argon2id_verify de libargon2 antes de pegarlo aquí: valida
--      correctamente contra `CambiarEsta123!`.
--
--    requiere_cambio_password=1 fuerza que esta contraseña temporal se
--    reemplace en el primer login (columna agregada en el punto "d" de la
--    migración 000001_seguridad_reforzada, sección de arriba). No se toca
--    fecha_ultimo_cambio_password (queda NULL): todavía no ha habido un
--    cambio de contraseña real, solo esta carga inicial.
--
--    IMPORTANTE: cambiar `CambiarEsta123!` en el primer login. Este hash es
--    válido SOLO para esa contraseña temporal -- no reutilizar en ningún
--    ambiente que no sea de arranque inicial.
--
--    INSERT IGNORE por nombre_usuario/correo UNIQUE: si esta cuenta ya
--    existe (re-correr el script), no se pisa el password_hash de un admin
--    que ya cambió su contraseña.
-- -----------------------------------------------------------------------------
INSERT IGNORE INTO usuarios
    (nombre_usuario, nombre_completo, correo, password_hash, empleado_id,
     activo, requiere_cambio_password)
VALUES
    ('admin', 'Administrador del Sistema', 'admin@ccdrliberia.local',
     '$argon2id$v=19$m=65536,t=3,p=2$X+jYgewPt/7pE9uh6E+UPQ$DoGXYvK7iI2H8SDgkXpyzm8SHg++J9Qns/sEDgrWj9o',
     NULL, 1, 1);

-- -----------------------------------------------------------------------------
-- 5) usuario_roles — asigna ADMINISTRADOR al usuario `admin` recién sembrado.
--    usuario_id y rol_id se resuelven por código/nombre de login (nunca IDs
--    fijos), igual que el resto de este bloque. creado_por queda NULL: es
--    una carga de migración inicial sin operador administrador previo que la
--    haya otorgado (mismo criterio documentado en el comentario de la
--    columna). fecha_fin_vigencia se deja sin asignar (NULL) para que
--    es_vigente (columna generada) quede en 1, respetando el patrón de
--    vigencia de 2 pasos de esta tabla.
--
--    WHERE NOT EXISTS en vez de ON DUPLICATE KEY / INSERT IGNORE simple:
--    usuario_roles tiene PK autogenerado (id), así que una re-corrida del
--    script insertaría una fila vigente duplicada si solo se confiara en
--    INSERT IGNORE por PK; el índice único uk_ur_una_vigente sí lo evita a
--    nivel de motor, pero el NOT EXISTS lo hace explícito y evita además
--    depender de que MySQL trate el conflicto como no-fatal.
-- -----------------------------------------------------------------------------
INSERT INTO usuario_roles
    (usuario_id, rol_id, creado_por, fecha_inicio_vigencia, fecha_fin_vigencia)
SELECT u.id, r.id, NULL, CURDATE(), NULL
FROM usuarios u
JOIN roles r ON r.codigo = 'ADMINISTRADOR'
WHERE u.nombre_usuario = 'admin'
  AND NOT EXISTS (
    SELECT 1 FROM usuario_roles ur
    WHERE ur.usuario_id = u.id
      AND ur.rol_id = r.id
      AND ur.fecha_fin_vigencia IS NULL
  );