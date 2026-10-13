-- 000109_exportaciones_trazabilidad.up.sql
-- F09 / S14: trazabilidad de exportaciones bancarias. Cada archivo de
-- dispersión generado deja una fila en exportaciones_generadas con quién,
-- cuándo, período, formato, nombre, hash SHA-256, cantidad de filas y total.
--
-- NO borra ni modifica filas existentes. Las filas previas quedan con las
-- columnas nuevas en NULL.
--
-- Ya existían: periodo_planilla_id, formato_exportacion_id, fecha_generacion,
-- hash_verificacion (VARCHAR(128), alcanza para SHA-256 hex de 64) y
-- generado_por_usuario_id.

ALTER TABLE exportaciones_generadas
  ADD COLUMN nombre_archivo VARCHAR(255) NULL
    COMMENT 'Nombre de archivo entregado al usuario (sin ruta)',
  ADD COLUMN cantidad_filas INT UNSIGNED NULL
    COMMENT 'Cantidad de filas (empleados) incluidas en el archivo',
  ADD COLUMN total_monto DECIMAL(16,2) NULL
    COMMENT 'Suma de los montos del archivo exportado',
  -- El archivo no se guarda en disco: solo su hash. La columna debe aceptar NULL.
  MODIFY COLUMN ruta_archivo VARCHAR(255) NULL
    COMMENT 'Ruta donde quedó almacenado el archivo; NULL si solo se registra el hash (no se persiste el archivo)';

-- Formato de exportación usado por el TXT de dispersión (idempotente).
INSERT INTO formatos_exportacion (codigo, nombre, tipo_destino)
VALUES ('BANCO_TXT_COMAS', 'Dispersión bancaria TXT delimitado por comas', 'banco')
ON DUPLICATE KEY UPDATE codigo = codigo;

-- Códigos de permiso que usan las rutas de exportación (INSERT IGNORE: no pisa
-- los existentes). NO se asignan a ningún rol aquí: la asignación es decisión
-- de seguridad de quien administre roles.
INSERT IGNORE INTO permisos (codigo, nombre, modulo) VALUES
    ('planilla.exportar_bancario', 'Generar archivo de dispersión bancaria', 'planilla'),
    ('planilla.ver_boleta',        'Consultar boleta de pago de un empleado', 'planilla');