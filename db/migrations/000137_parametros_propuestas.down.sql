DROP TRIGGER trg_aud_parametros_propuestas_ins;
DROP TRIGGER trg_aud_parametros_propuestas_upd;
DROP TRIGGER trg_aud_parametros_propuestas_del;
DELETE rp FROM rol_permisos rp
JOIN roles r ON r.id = rp.rol_id
JOIN permisos p ON p.id = rp.permiso_id
WHERE p.codigo = 'parametros.ver' AND r.codigo IN ('PLANILLAS_PREPARA', 'PLANILLAS_APRUEBA', 'AUDITOR');
DROP TABLE parametros_propuestas;
