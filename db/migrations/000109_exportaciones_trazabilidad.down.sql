-- 000109_exportaciones_trazabilidad.down.sql
-- Revierte la estructura de 000109. Se pierden los datos de las 3 columnas
-- nuevas (nombre_archivo, cantidad_filas, total_monto): es inherente a revertir.
-- Las filas con ruta_archivo NULL reciben '' para poder volver a NOT NULL.
-- No se borran formatos_exportacion ni permisos sembrados: pueden estar
-- referenciados por exportaciones_generadas y por rol_permisos.

UPDATE exportaciones_generadas SET ruta_archivo = '' WHERE ruta_archivo IS NULL;

ALTER TABLE exportaciones_generadas
  MODIFY COLUMN ruta_archivo VARCHAR(255) NOT NULL
    COMMENT 'Ruta o ubicación donde quedó almacenado el archivo generado',
  DROP COLUMN total_monto,
  DROP COLUMN cantidad_filas,
  DROP COLUMN nombre_archivo;