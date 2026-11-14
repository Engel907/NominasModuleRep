UPDATE exportaciones_generadas SET ruta_archivo = '' WHERE ruta_archivo IS NULL;

ALTER TABLE exportaciones_generadas
  MODIFY COLUMN ruta_archivo VARCHAR(255) NOT NULL
    COMMENT 'Ruta o ubicación donde quedó almacenado el archivo generado',
  DROP COLUMN total_monto,
  DROP COLUMN cantidad_filas,
  DROP COLUMN nombre_archivo;