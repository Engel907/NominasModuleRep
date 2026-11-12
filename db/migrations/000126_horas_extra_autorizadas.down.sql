DELETE rp FROM rol_permisos rp JOIN permisos p ON p.id = rp.permiso_id WHERE p.codigo = 'horas_extra.autorizar';
DELETE FROM permisos WHERE codigo IN ('horas_extra.autorizar');
DROP TABLE horas_extra_autorizadas;
