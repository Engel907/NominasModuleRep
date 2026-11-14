INSERT INTO permisos (codigo, nombre, modulo)
SELECT n.codigo, n.nombre, n.modulo
FROM (
  SELECT 'configuracion.ver' AS codigo, 'Consultar los catálogos de configuración de la planilla' AS nombre, 'parametros' AS modulo
  UNION ALL SELECT 'configuracion.administrar', 'Administrar los catálogos de configuración de la planilla', 'parametros'
) n
WHERE NOT EXISTS (SELECT 1 FROM permisos p WHERE p.codigo = n.codigo);

INSERT INTO rol_permisos (rol_id, permiso_id)
SELECT r.id, p.id
FROM roles r
JOIN permisos p ON p.codigo = 'configuracion.ver'
WHERE r.codigo IN ('RRHH', 'PLANILLAS_PREPARA', 'PLANILLAS_APRUEBA', 'CONTABILIDAD', 'AUDITOR')
  AND NOT EXISTS (SELECT 1 FROM rol_permisos rp WHERE rp.rol_id = r.id AND rp.permiso_id = p.id);

INSERT INTO rol_permisos (rol_id, permiso_id)
SELECT r.id, p.id
FROM roles r
JOIN permisos p ON p.codigo = 'configuracion.administrar'
WHERE r.codigo = 'PLANILLAS_APRUEBA'
  AND NOT EXISTS (SELECT 1 FROM rol_permisos rp WHERE rp.rol_id = r.id AND rp.permiso_id = p.id);

UPDATE usuarios u
SET u.permisos_version = u.permisos_version + 1
WHERE EXISTS (SELECT 1 FROM usuario_roles ur JOIN roles r ON r.id = ur.rol_id
              WHERE ur.usuario_id = u.id AND ur.fecha_fin_vigencia IS NULL
                AND r.codigo IN ('RRHH', 'PLANILLAS_PREPARA', 'PLANILLAS_APRUEBA', 'CONTABILIDAD', 'AUDITOR'));
