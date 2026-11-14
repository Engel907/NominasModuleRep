DROP TABLE empleado_conceptos_asignados;

ALTER TABLE terceros
  MODIFY tipo ENUM('juzgado','entidad_financiera','otro') NOT NULL COMMENT 'Categoría del tercero',
  COMMENT = 'Catálogo de terceros externos (juzgados, entidades financieras, otros)';
