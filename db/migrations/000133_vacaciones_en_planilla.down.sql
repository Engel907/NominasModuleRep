DROP TRIGGER IF EXISTS trg_aud_vacaciones_movimientos_ins;
DROP TRIGGER IF EXISTS trg_aud_vacaciones_movimientos_upd;
DROP TRIGGER IF EXISTS trg_aud_vacaciones_movimientos_del;

ALTER TABLE vacaciones_movimientos
  DROP FOREIGN KEY fk_vacmov_periodo,
  DROP COLUMN pagado_en,
  DROP COLUMN periodo_planilla_id,
  MODIFY tipo_movimiento ENUM('acumulacion','disfrute','ajuste') NOT NULL;
