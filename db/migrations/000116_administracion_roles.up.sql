-- F15: separar la administración de permisos globales de la asignación
-- individual de roles a usuarios.
INSERT INTO permisos (codigo, nombre, modulo)
SELECT 'roles.administrar', 'Administrar roles y sus permisos', 'auditoria'
WHERE NOT EXISTS (
    SELECT 1 FROM permisos WHERE codigo = 'roles.administrar'
);

INSERT INTO rol_permisos (rol_id, permiso_id)
SELECT r.id, p.id
FROM roles r
JOIN permisos p ON p.codigo = 'roles.administrar'
WHERE r.codigo = 'ADMINISTRADOR_SISTEMA'
  AND NOT EXISTS (
      SELECT 1
      FROM rol_permisos rp
      WHERE rp.rol_id = r.id AND rp.permiso_id = p.id
  );
