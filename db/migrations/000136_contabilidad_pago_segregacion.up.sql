INSERT INTO tipos_asiento (codigo, nombre, descripcion)
SELECT 'PAGO_PLANILLA', 'Pago de planilla',
       'Cancelación del pasivo de salarios por pagar contra bancos al pagar el período.'
WHERE NOT EXISTS (SELECT 1 FROM tipos_asiento WHERE codigo = 'PAGO_PLANILLA');

INSERT INTO plantillas_asiento_linea (tipo_asiento_id, descripcion_linea, tipo_movimiento, orden)
SELECT ta.id, v.descripcion, v.mov, v.orden
FROM tipos_asiento ta
JOIN (
  SELECT 'Salarios por pagar (neto)' AS descripcion, 'debe' AS mov, 10 AS orden
  UNION ALL SELECT 'Bancos — cuenta de planillas', 'haber', 20
) v ON 1 = 1
WHERE ta.codigo = 'PAGO_PLANILLA'
  AND NOT EXISTS (SELECT 1 FROM plantillas_asiento_linea p WHERE p.tipo_asiento_id = ta.id AND p.orden = v.orden);

CREATE TRIGGER trg_pp_contabilizar_bu BEFORE UPDATE ON periodos_planilla
FOR EACH ROW FOLLOWS trg_pp_cuadre_bu
BEGIN
  IF NEW.estado = 'contabilizada' AND OLD.estado <> 'contabilizada'
     AND (NEW.contabilizado_por_usuario_id IS NULL OR NEW.fecha_contabilizacion IS NULL) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'transicion_invalida: contabilizar exige usuario y fecha de contabilizacion';
  END IF;
END;
