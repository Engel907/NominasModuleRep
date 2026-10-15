DELETE rp
FROM rol_permisos rp
JOIN roles r ON r.id = rp.rol_id
JOIN permisos p ON p.id = rp.permiso_id
WHERE r.codigo = 'EMPLEADO'
  AND p.codigo IN ('planilla.ver_boleta_propia', 'vacaciones.solicitar');

DELETE ur
FROM usuario_roles ur
JOIN roles r ON r.id = ur.rol_id
WHERE r.codigo = 'EMPLEADO';

DELETE FROM roles WHERE codigo = 'EMPLEADO';
