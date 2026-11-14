DROP TRIGGER trg_prestamos_saldado_bu;

UPDATE prestamos SET estado = 'cancelado' WHERE estado = 'pagado';

ALTER TABLE prestamos
  MODIFY estado ENUM('activo','cancelado','suspendido') NOT NULL DEFAULT 'activo' COMMENT 'Estado actual del préstamo';
