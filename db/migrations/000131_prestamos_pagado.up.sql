ALTER TABLE prestamos
  MODIFY estado ENUM('activo','cancelado','suspendido','pagado') NOT NULL DEFAULT 'activo' COMMENT 'Estado actual del préstamo (pagado = saldo llevado a 0)';

CREATE TRIGGER trg_prestamos_saldado_bu BEFORE UPDATE ON prestamos
FOR EACH ROW
BEGIN
  IF NEW.saldo_pendiente = 0 AND OLD.saldo_pendiente > 0 AND NEW.estado = 'activo' THEN
    SET NEW.estado = 'pagado';
  END IF;
END;
