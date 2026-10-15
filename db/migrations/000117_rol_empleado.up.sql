-- S10/F17: fila mínima de permisos para cuentas vinculadas a un empleado.
INSERT INTO roles (codigo, nombre, descripcion, activo)
SELECT 'EMPLEADO', 'Empleado', 'Acceso del empleado a su propia boleta y solicitudes de vacaciones.', 1
WHERE NOT EXISTS (SELECT 1 FROM roles WHERE codigo = 'EMPLEADO');

INSERT INTO rol_permisos (rol_id, permiso_id)
SELECT r.id, p.id
FROM roles r
JOIN permisos p ON p.codigo IN ('planilla.ver_boleta_propia', 'vacaciones.solicitar')
WHERE r.codigo = 'EMPLEADO'
  AND NOT EXISTS (
      SELECT 1 FROM rol_permisos rp
      WHERE rp.rol_id = r.id AND rp.permiso_id = p.id
  );
