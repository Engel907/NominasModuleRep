DELETE rp
FROM rol_permisos rp
JOIN roles r ON r.id = rp.rol_id
JOIN permisos p ON p.id = rp.permiso_id
WHERE r.codigo = 'ADMINISTRADOR_SISTEMA'
  AND p.codigo = 'roles.administrar';

DELETE FROM permisos
WHERE codigo = 'roles.administrar';
