-- Fase 18A (down). Orden inverso al up.
ALTER TABLE parametros_tipos_embargo
  DROP CONSTRAINT chk_pte_tope,
  DROP COLUMN tope_porcentaje_liquido,
  DROP COLUMN es_pension_alimentaria;

DROP TABLE IF EXISTS parametros_embargo_escala;
DROP TABLE IF EXISTS parametros_generales;