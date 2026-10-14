-- F15 (S7): versión de permisos por usuario. Se incrementa en cada cambio que
-- debe invalidar los access tokens ya emitidos (rol asignado/revocado,
-- activación/desactivación, cambio o reset de contraseña, cierre de sesiones).
ALTER TABLE usuarios
    ADD COLUMN permisos_version INT UNSIGNED NOT NULL DEFAULT 1
    COMMENT 'F15: sube al cambiar roles/estado/password; el access token lleva el valor vigente al emitirse (claim pv)';