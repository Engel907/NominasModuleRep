DELETE rp FROM rol_permisos rp
JOIN permisos p ON p.id = rp.permiso_id
WHERE p.codigo IN ('configuracion.ver', 'configuracion.administrar');

DELETE FROM permisos WHERE codigo IN ('configuracion.ver', 'configuracion.administrar');
