ALTER TABLE usuarios
    ADD COLUMN permisos_version INT UNSIGNED NOT NULL DEFAULT 1
    COMMENT 'Sube al cambiar roles/estado/password; el access token lleva el valor vigente al emitirse (claim pv)';