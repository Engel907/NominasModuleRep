ALTER TABLE exportaciones_generadas
  ADD COLUMN nombre_archivo VARCHAR(255) NULL
    COMMENT 'Nombre de archivo entregado al usuario (sin ruta)',
  ADD COLUMN cantidad_filas INT UNSIGNED NULL
    COMMENT 'Cantidad de filas (empleados) incluidas en el archivo',
  ADD COLUMN total_monto DECIMAL(16,2) NULL
    COMMENT 'Suma de los montos del archivo exportado',
  MODIFY COLUMN ruta_archivo VARCHAR(255) NULL
    COMMENT 'Ruta donde quedó almacenado el archivo; NULL si solo se registra el hash (no se persiste el archivo)';

INSERT INTO formatos_exportacion (codigo, nombre, tipo_destino)
VALUES ('BANCO_TXT_COMAS', 'Dispersión bancaria TXT delimitado por comas', 'banco')
ON DUPLICATE KEY UPDATE codigo = codigo;

INSERT IGNORE INTO permisos (codigo, nombre, modulo) VALUES
    ('planilla.exportar_bancario', 'Generar archivo de dispersión bancaria', 'planilla'),
    ('planilla.ver_boleta',        'Consultar boleta de pago de un empleado', 'planilla');