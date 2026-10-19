# Plan LITE — Backend Planillas CCDR (reemplaza F20–F32)

Base: código real tras F19 (leído del zip). Módulo `planillas-ccdr`, Go 1.23, Gin, MySQL 8, `shopspring/decimal`.
Todas las rutas son relativas a `ccdr-planillas-API-sin-migraciones/`.

---

## 1. Por qué F20–F26 no corrían y cómo lo evita este plan

| Causa | Qué hace este plan |
|---|---|
| Cada fase pedía 8–15 archivos grandes (motor 545 l, servicios 500+ l, `schema.sql`). | Máx. **3 archivos de entrada** por fase. El resto se da como **firmas** (`scripts/firmas.sh`) o como **contrato** escrito en la fase. |
| El DDL se pasaba por rangos de líneas de un schema gigante, con `COMMENT` de 150 caracteres. | `scripts/ddl.sh tabla…` imprime solo el `CREATE/ALTER` **sin COMMENT** de las migraciones. (4 tablas del período = 5.5 KB.) El DDL **nuevo** ya viene escrito en cada fase: la IA lo pega, no lo diseña. |
| Una fase mezclaba hidratar + calcular + persistir + endpoints + reglas legales. | Una fase = un concern. El motor se parte en **una regla por archivo** (L5); cada regla posterior toca solo su archivo. |
| La IA exploraba el repo para decidir. | Las decisiones ya están tomadas (§3) y las firmas fijadas. Si falta algo, **pide y se detiene**. |
| Bloque común de ~60 líneas pegado en cada chat. | Bloque común lite de 12 líneas (§4). |

**Regla de oro:** si una fase no cabe en tu IA, no la partas por tareas: corre solo las fases marcadas ⇄ por separado. Cada una ya es mínima.

---

## 2. Estado real del repo (verificado) y hallazgos que cambian el plan

Hecho (F00–F19): migraciones 000000–000119, auth reforzada, PASETO con `pv`/`kid`, rate-limit/CORS/headers, usuarios/roles/auditoría, `parametros_generales` + `Porcentaje` + `CargadorParametros`, ciclo de vida del período con segregación y triggers (`PeriodoService`).

Hallazgos (bugs o huecos que las fases viejas no contemplaban):

1. **`tipos_planilla` no tiene ningún seed** → hoy no se puede crear un período. (L3)
2. **`regimen_ccss_id` no existe en ningún empleado/contrato**: `CalculoEmpleadoInput.RegimenCCSSID` no tiene origen; la base mínima CCSS es imposible de resolver. (L15)
3. `PeriodoGanchos{}` va vacío en `internal/app/app.go`: `/planillas/:id/calcular` siempre responde 501. (L8)
4. `calculator.go` conserva `0.125`, `0.25`, `0.5`, `3` y `PrioridadPensionAlimentaria` aunque `parametros_embargo_escala` y `es_pension_alimentaria` ya existen y el cargador ya trae `EscalaEmbargo`. (L16)
5. `prestamos.EstadoPrestamoPagado = "pagado"` **no existe en el ENUM** (`activo|cancelado|suspendido`): cerrar un préstamo saldado fallaría. (L18)
6. `planilla_detalle_embargo` **no tiene trigger de inmutabilidad** (000119 solo cubre `pde` y `pdc`). (L2)
7. El modelo de salario por días (`DiasTrabajados <= DiasPeriodoCompleto = 30`) falla en meses de 31 días. Se define convención de **días comerciales**. (L10)
8. Contabilidad **recalcula** la cuota patronal y aborta si no cuadra; `ContabilizarPeriodo` no llama `VerificarSegregacion(EstadoContabilizada)`. (L23)
9. Aguinaldo/salario escolar: siguen las fechas escritas en Go (`fechaUTC`, `mesesAguinaldo`) aunque ya están sembradas en `parametros_generales`; `CalcularAguinaldo/SalarioEscolar` están duplicadas (`planilla` y `services`); la liquidación aún tiene `DivisorDiasMes = 30` y `> 6`, `+= 12`. (L19, L21)
10. `contratos.jornada` y `contratos.tipo_contrato` son texto libre; `TODO F22` (D5, fuente única de salario) sigue abierto en `empleado_service`/`contrato_service`. (L10, L12)
11. Los seeds con `ON DUPLICATE KEY UPDATE` (000002/3/5/6/109) **no se tocan**: solo corren sobre BD vacía y `golang-migrate` no re-ejecuta; el riesgo real es nulo. Solo se **prohíben** en migraciones nuevas (lint en L0).

---

## 3. Decisiones nuevas (ya tomadas; se suman a D1–D12 del ADR)

| # | Decisión |
|---|---|
| D13 | `SALARIO_MINIMO_INEMBARGABLE` es **un monto global** en `parametros_generales` (no por clasificación). **No se siembra** (valor legal): sin valor, el cálculo con embargos falla con error explícito. |
| D14 | La liquidación se paga por el **motor** como planilla de clase `liquidacion`, **un período por mes** de salida (no uno por empleado: choca con `uk_pp_tipo_fechas`). |
| D15 | **Días comerciales** (base `DIVISOR_DIAS_MES`): el mes cuenta 30; el último día del mes cuenta como día 30. `SUPUESTO A VALIDAR` con RR. HH. |
| D16 | El tipo de planilla tiene `clase ENUM('regular','aguinaldo','salario_escolar','liquidacion')`. Go **nunca** busca un tipo por su `codigo` de texto. |
| D17 | `regimen_ccss_id` vive en `empleado_datos_laborales` (vigencia por fila). Sin valor → error explícito. |
| D18 | Qué conceptos calcula cada tipo de planilla es **dato**: tabla `tipo_planilla_concepto`. |
| D19 | Embargo insatisfecho: se **persiste** (`monto_insatisfecho`); su arrastre al período siguiente queda **fuera de alcance** hasta decisión legal. Préstamo no cobrado: no se arrastra, el saldo simplemente no baja. |
| D20 | Cuota patronal: **persistida** al calcular; contabilidad/reportes la leen, nunca la recalculan. |
| D21 | Lógica en BD: integridad (CHECK/triggers/función de cuadre), inmutabilidad, vigencias sin traslape, segregación (CHECK) y cierre automático de préstamo. Go valida primero, la BD es la última defensa. |

Numeración de migraciones (reservada por fase; sustituye la regla `0001NN`):
`000120` L1 · `000121` L2 · `000122` L3 · `000123` L10 · `000124` L12 · `000125` L13 · `000126` L14 · `000127` L15 · `000128` L16 · `000129` L17 · `000130` L18 · `000131` L19 · `000132` L20 · `000133` L21 · `000134` L22 · `000135` L23 · `000136` L24 · `000137` L25 · `000138` L26.

---

## 4. BLOQUE COMÚN LITE (pegar al inicio de cada chat)

```
Go 1.23, Gin, MySQL 8, shopspring/decimal. Backend de planillas del sector público de Costa Rica: un error = consecuencia legal.
1. Lee SOLO lo que te adjunto. Si falta algo, pídelo y detente. No explores el repo, no inventes firmas ni columnas.
2. Cumple los CONTRATOS (firmas) de la fase tal cual. Archivos COMPLETOS con su ruta.
3. Cero valores legales/numéricos de negocio en Go: vienen de BD o parametros.Generales. Faltante = error explícito, nunca default.
4. decimal siempre; jamás float64/NewFromFloat. Porcentajes en dominio = fracción (solo conversión en internal/shared/porcentaje).
5. Escrituras solo con db.WithAuditContext(ctx, sqlDB, usuarioID int64, func(ctx context.Context, q *dto.Queries) error).
6. Errores de dominio tipados; al cliente nunca err.Error() interno. Comentarios y mensajes en español.
7. Migración: archivo nuevo NNNNNN_nombre.up.sql/.down.sql, sin ON DUPLICATE KEY UPDATE, INSERT IGNORE, DROP TABLE IF EXISTS ni DELIMITER. Seeds con INSERT…SELECT…WHERE NOT EXISTS.
   Valores legales sembrados llevan fuente_legal='PENDIENTE VALIDAR' y vigencia de 2 pasos. Si ALTERas una tabla con triggers de auditoría, regénerelos (scripts/gen_auditoria.sh).
8. Verifica `gofmt -l`, `go vet` y `go test` SOLO del paquete que tocas; si no puedes ejecutar, di "no verificado".
9. Al final agrega ≤8 líneas a docs/fases/LITE.md: archivos, firmas nuevas, migración, pendientes. Si te quedas sin espacio: cierra la tarea en curso y lista lo que falta.
```

**Cómo pasar el DDL:** `bash scripts/ddl.sh t1 t2 > /tmp/ddl.txt` y adjuntas `/tmp/ddl.txt`. Firmas de un paquete: `bash scripts/firmas.sh internal/planilla [patrón]`. Triggers de una tabla: `bash scripts/ddl.sh --triggers tabla`.


---

# A — BASE

## L0 — Herramientas y baseline
**Entra:** nada (usa los 3 scripts entregados junto a este plan).
**Hacer**
1. Copiar `scripts/ddl.sh`, `scripts/firmas.sh`, `scripts/gen_auditoria.sh` (ya probados).
2. `internal/db/migraciones_lint_test.go` (sin BD): para migraciones con número ≥ 000120 falla si el `.up.sql` contiene `ON DUPLICATE KEY UPDATE`, `INSERT IGNORE`, `DROP TABLE IF EXISTS`, `DELIMITER`, `CREATE DATABASE`, o si falta su `.down.sql`.
3. `docs/fases/LITE.md`: copia la tabla del §5 con columna de estado.
4. Correr `./run_full_test_suite.sh` y pegar los rojos en `LITE.md`. **No arreglar nada** salvo lo de una línea.
**Listo cuando:** el lint pasa y hay un baseline documentado.

## L1 — DDL de persistencia del cálculo (000120)
**Entra:** `bash scripts/ddl.sh periodos_planilla planilla_detalle_empleado planilla_detalle_concepto planilla_detalle_embargo incapacidades prestamos`.
**Hacer:** crear `db/migrations/000120_motor_persistencia.{up,down}.sql` con este DDL; regenerar los triggers de auditoría existentes de `periodos_planilla` y `planilla_detalle_empleado` para incluir todas las columnas vigentes, y restaurar sus definiciones previas en el `.down`. `planilla_detalle_embargo` no tiene triggers de auditoría y no se le crean en esta fase: su inmutabilidad se implementa en L2. Los `.down` revierten en orden inverso.
```sql
ALTER TABLE periodos_planilla
  ADD COLUMN parametros_snapshot JSON NULL COMMENT 'Parámetros resueltos (en fracción) usados en el último cálculo',
  ADD COLUMN version_motor VARCHAR(40) NULL COMMENT 'Versión del motor que calculó el período';

ALTER TABLE planilla_detalle_empleado
  ADD COLUMN contrato_id BIGINT UNSIGNED NULL COMMENT 'Contrato principal vigente al calcular',
  ADD COLUMN base_ccss DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Base de cotización CCSS usada',
  ADD COLUMN base_renta DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Base de renta usada',
  ADD COLUMN base_aguinaldo DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Acumula para aguinaldo',
  ADD COLUMN base_cesantia DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Acumula para cesantía',
  ADD COLUMN base_vacaciones DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Acumula para vacaciones',
  ADD COLUMN cuota_obrera_ccss DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Cuota obrera CCSS retenida',
  ADD COLUMN cuota_patronal_ccss DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Cuota patronal total (persistida, D20)',
  ADD COLUMN renta_bruta DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Impuesto de renta antes de créditos',
  ADD COLUMN creditos_fiscales_aplicados DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Créditos fiscales efectivamente aplicados',
  ADD COLUMN renta_neta DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Renta retenida',
  ADD COLUMN advertencias JSON NULL COMMENT 'Advertencias no fatales del cálculo',
  ADD CONSTRAINT fk_pde_contrato FOREIGN KEY (contrato_id) REFERENCES contratos(id),
  ADD CONSTRAINT chk_pde_neto CHECK (neto_pagado >= 0 AND neto_pagado = total_ingresos - total_deducciones);

CREATE TABLE planilla_detalle_patronal (
  id                           BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  planilla_detalle_empleado_id BIGINT UNSIGNED NOT NULL COMMENT 'Línea del empleado',
  concepto_salarial_id         BIGINT UNSIGNED NOT NULL COMMENT 'Concepto patronal (CCSS_PAT en modelo agregado, D6)',
  base                         DECIMAL(14,2) NOT NULL COMMENT 'Base sobre la que se calculó',
  porcentaje                   DECIMAL(9,6)  NOT NULL COMMENT 'FRACCIÓN (0.2683), nunca porcentaje',
  monto                        DECIMAL(14,2) NOT NULL COMMENT 'Cuota patronal',
  CONSTRAINT fk_pdpat_detalle  FOREIGN KEY (planilla_detalle_empleado_id) REFERENCES planilla_detalle_empleado(id),
  CONSTRAINT fk_pdpat_concepto FOREIGN KEY (concepto_salarial_id) REFERENCES conceptos_salariales(id),
  CONSTRAINT chk_pdpat CHECK (base >= 0 AND monto >= 0 AND porcentaje >= 0 AND porcentaje <= 1),
  UNIQUE KEY uk_pdpat (planilla_detalle_empleado_id, concepto_salarial_id)
) ENGINE=InnoDB COMMENT='Cuota patronal calculada por empleado y período';

ALTER TABLE planilla_detalle_embargo
  ADD COLUMN monto_calculado   DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Lo que correspondía según % o monto fijo, antes de topes',
  ADD COLUMN monto_insatisfecho DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'monto_calculado - monto (D19: se persiste, no se arrastra)';
UPDATE planilla_detalle_embargo SET monto_calculado = monto WHERE monto_calculado = 0;
ALTER TABLE planilla_detalle_embargo
  ADD CONSTRAINT chk_pdemb_calc CHECK (monto_calculado >= monto AND monto_insatisfecho = monto_calculado - monto);

CREATE TABLE planilla_detalle_incapacidad (
  id                           BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  planilla_detalle_empleado_id BIGINT UNSIGNED NOT NULL COMMENT 'Línea del empleado',
  incapacidad_id               BIGINT NOT NULL COMMENT 'Incapacidad (incapacidades.id es BIGINT con signo)',
  dia_serie_desde              INT UNSIGNED NOT NULL COMMENT 'Día de la incapacidad continua en que empieza este tramo',
  dia_serie_hasta              INT UNSIGNED NOT NULL COMMENT 'Día de la incapacidad continua en que termina',
  dias                         DECIMAL(5,2)  NOT NULL COMMENT 'Días comerciales dentro del período',
  monto_patronal               DECIMAL(14,2) NOT NULL COMMENT 'Subsidio pagado por planilla',
  monto_entidad_informativo    DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT 'Parte a cargo de CCSS/INS (no se paga por planilla)',
  CONSTRAINT fk_pdinc_detalle FOREIGN KEY (planilla_detalle_empleado_id) REFERENCES planilla_detalle_empleado(id),
  CONSTRAINT fk_pdinc_inc     FOREIGN KEY (incapacidad_id) REFERENCES incapacidades(id),
  CONSTRAINT chk_pdinc CHECK (dia_serie_hasta >= dia_serie_desde AND dias >= 0 AND monto_patronal >= 0 AND monto_entidad_informativo >= 0),
  UNIQUE KEY uk_pdinc (planilla_detalle_empleado_id, incapacidad_id, dia_serie_desde)
) ENGINE=InnoDB COMMENT='Detalle de subsidio de incapacidad por tramo';

CREATE TABLE planilla_detalle_prestamo (
  id                           BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  planilla_detalle_empleado_id BIGINT UNSIGNED NOT NULL COMMENT 'Línea del empleado',
  prestamo_id                  BIGINT UNSIGNED NOT NULL COMMENT 'Préstamo',
  cuota_calculada              DECIMAL(14,2) NOT NULL COMMENT 'min(cuota periódica, saldo)',
  monto_retenido               DECIMAL(14,2) NOT NULL COMMENT 'Lo realmente retenido tras el tope de neto',
  aplicado_en                  DATETIME NULL COMMENT 'Se llena al aprobar el período (baja el saldo). NULL = no aplicado',
  CONSTRAINT fk_pdpre_detalle  FOREIGN KEY (planilla_detalle_empleado_id) REFERENCES planilla_detalle_empleado(id),
  CONSTRAINT fk_pdpre_prestamo FOREIGN KEY (prestamo_id) REFERENCES prestamos(id),
  CONSTRAINT chk_pdpre CHECK (cuota_calculada >= 0 AND monto_retenido >= 0 AND monto_retenido <= cuota_calculada),
  UNIQUE KEY uk_pdpre (planilla_detalle_empleado_id, prestamo_id)
) ENGINE=InnoDB COMMENT='Cuota de préstamo por línea de planilla; idempotencia de la aplicación al saldo';

CREATE TABLE planilla_devengos_manuales (
  id                   BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  periodo_planilla_id  BIGINT UNSIGNED NOT NULL COMMENT 'Período (solo en borrador)',
  empleado_id          BIGINT UNSIGNED NOT NULL COMMENT 'Empleado',
  concepto_salarial_id BIGINT UNSIGNED NOT NULL COMMENT 'Concepto de captura manual',
  monto                DECIMAL(14,2) NOT NULL COMMENT 'Monto del devengo',
  observacion          VARCHAR(255) NULL COMMENT 'Motivo/soporte',
  creado_por           BIGINT UNSIGNED NOT NULL COMMENT 'Usuario que lo registró',
  created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Fecha de registro',
  CONSTRAINT fk_pdm_periodo  FOREIGN KEY (periodo_planilla_id)  REFERENCES periodos_planilla(id),
  CONSTRAINT fk_pdm_empleado FOREIGN KEY (empleado_id)          REFERENCES empleados(id),
  CONSTRAINT fk_pdm_concepto FOREIGN KEY (concepto_salarial_id) REFERENCES conceptos_salariales(id),
  CONSTRAINT fk_pdm_usuario  FOREIGN KEY (creado_por)           REFERENCES usuarios(id),
  CONSTRAINT chk_pdm_monto CHECK (monto > 0),
  KEY ix_pdm_periodo_emp (periodo_planilla_id, empleado_id)
) ENGINE=InnoDB COMMENT='Devengos capturados a mano (comisiones, bonos, viáticos) consumidos por el cálculo';
```
Auditar con `gen_auditoria.sh`: `planilla_devengos_manuales` (cols: id periodo_planilla_id empleado_id concepto_salarial_id monto observacion creado_por). Las tablas derivadas se protegen por inmutabilidad (L2), no por auditoría.
**Listo cuando:** up/down/up pasa en MySQL de prueba y `contrato_sql_test` sigue verde.

## L2 — Integridad en BD (000121)
**Entra:** `bash scripts/ddl.sh --triggers periodos_planilla planilla_detalle_empleado planilla_detalle_concepto`, `db/migrations/000119_periodo_ciclo.up.sql` (líneas 21–191, patrón a copiar), `bash scripts/ddl.sh parametros_generales parametros_cargas_sociales parametros_tramos_renta parametros_creditos_fiscales_renta parametros_horas_extra parametros_base_minima_contributiva parametros_subsidios_incapacidad parametros_embargo_escala`.
**Hacer:** `000121_integridad_bd.{up,down}.sql` (sin DELIMITER, mismo estilo de 000119):
1. **Función de cuadre** `fn_detalle_cuadra(p_periodo BIGINT UNSIGNED) RETURNS TINYINT(1) READS SQL DATA`: 1 solo si existe ≥1 línea y, por cada `planilla_detalle_empleado`, `total_ingresos` = Σ `pdc.monto` de conceptos `naturaleza='ingreso'`, `total_deducciones` = Σ de `'deduccion'`, y `cuota_patronal_ccss` = Σ `planilla_detalle_patronal.monto`.
2. **Trigger** `trg_pp_cuadre_bu BEFORE UPDATE ON periodos_planilla … FOLLOWS trg_pp_ciclo_bu`: si `OLD.estado='calculada' AND NEW.estado='aprobada' AND fn_detalle_cuadra(NEW.id)=0` → `SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='detalle_descuadrado: …'`.
3. **Inmutabilidad** (copiar el patrón `trg_pde_inmutable_*`: BI/BU/BD rechazan si el período padre no está en `borrador`/`calculada`; mensaje `periodo_inmutable`) para `planilla_detalle_embargo` (**hoy sin protección**), `planilla_detalle_patronal`, `planilla_detalle_incapacidad` y `planilla_detalle_prestamo` (esta última permite solo `aplicado_en` NULL→valor en BU).
4. **Vigencias:** en `parametros_generales`, trigger BI que rechaza traslape de rangos para la misma `clave` (`vigencia_traslapada`). En las 8 tablas paramétricas de la entrada: BU que rechaza cambiar cualquier columna que no sea `fecha_fin_vigencia` (`parametro_inmutable`) y BD que rechaza todo borrado. Genera el SQL con un script de shell `scripts/gen_inmutabilidad_parametros.sh tabla col1 col2…` (columnas de valor sacadas del DDL de entrada) y commitéalo.
5. **Vista** `v_planilla_resumen_periodo` (período, nº empleados, Σ ingresos, Σ deducciones, Σ neto, Σ cuota patronal) para dashboard/reportes.
6. Go: `traducirErrorBD` (`internal/planilla/periodo_service.go`) agrega prefijos `detalle_descuadrado`→`ErrDetalleDescuadrado` (nuevo, 409). Anotar que MySQL con binlog exige `log_bin_trust_function_creators=1` o usuario con SUPER para crear la función.
**Test (integración):** insertar detalle descuadrado y aprobar → rechazado; modificar `planilla_detalle_embargo` de un período aprobado → rechazado; UPDATE de `porcentaje_obrero` → rechazado; cerrar vigencia → permitido; traslape en `parametros_generales` → rechazado.
**Si algún test de F18/F19 hacía DELETE/UPDATE de parámetros, ajústalo usando cierre de vigencia.**



por aqui 


## L3 — Tipos de planilla y conceptos por tipo (000122)
**Entra:** `internal/planilla/estados.go`, `internal/planilla/periodo_service.go` (solo `Crear`), `bash scripts/ddl.sh tipos_planilla conceptos_salariales`.
**DDL**
```sql
ALTER TABLE tipos_planilla
  ADD COLUMN clase ENUM('regular','aguinaldo','salario_escolar','liquidacion') NOT NULL DEFAULT 'regular' COMMENT 'Comportamiento del tipo (D16); Go nunca usa codigo',
  ADD COLUMN fraccion_mensual DECIMAL(6,4) NULL COMMENT 'Fracción del mes que cubre un período (mensual 1.0000, quincenal 0.5000); NULL si no es regular',
  ADD CONSTRAINT chk_tp_fraccion CHECK ((clase = 'regular' AND fraccion_mensual IS NOT NULL AND fraccion_mensual > 0 AND fraccion_mensual <= 1) OR (clase <> 'regular' AND fraccion_mensual IS NULL));

INSERT INTO tipos_planilla (codigo, nombre, clase, fraccion_mensual)
SELECT v.codigo, v.nombre, v.clase, v.fraccion FROM (
  SELECT 'mensual' codigo, 'Planilla mensual' nombre, 'regular' clase, 1.0000 fraccion
  UNION ALL SELECT 'quincenal', 'Planilla quincenal', 'regular', 0.5000
  UNION ALL SELECT 'aguinaldo', 'Aguinaldo', 'aguinaldo', NULL
  UNION ALL SELECT 'salario_escolar', 'Salario escolar', 'salario_escolar', NULL
  UNION ALL SELECT 'liquidacion', 'Liquidación laboral', 'liquidacion', NULL
) v WHERE NOT EXISTS (SELECT 1 FROM tipos_planilla t WHERE t.codigo = v.codigo);

CREATE TABLE tipo_planilla_concepto (
  id                   BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  tipo_planilla_id     BIGINT UNSIGNED NOT NULL COMMENT 'Tipo de planilla',
  concepto_salarial_id BIGINT UNSIGNED NOT NULL COMMENT 'Concepto que ese tipo calcula (D18)',
  CONSTRAINT fk_tpc_tipo     FOREIGN KEY (tipo_planilla_id)     REFERENCES tipos_planilla(id),
  CONSTRAINT fk_tpc_concepto FOREIGN KEY (concepto_salarial_id) REFERENCES conceptos_salariales(id),
  UNIQUE KEY uk_tpc (tipo_planilla_id, concepto_salarial_id)
) ENGINE=InnoDB COMMENT='Qué conceptos calcula cada tipo de planilla';

INSERT INTO tipo_planilla_concepto (tipo_planilla_id, concepto_salarial_id)
SELECT t.id, c.id FROM tipos_planilla t JOIN conceptos_salariales c
 ON c.codigo IN ('SAL_BASE','HRS_EXTRA','COMISIONES','BONO_DISCRECIONAL','VIATICOS','SUB_INCAP','PAGO_VACACIONES',
                 'CCSS_OBR','IMP_RENTA','EMBARGO','PRESTAMO','ASOC_SOLIDARISTA','SINDICATO','CCSS_PAT')
WHERE t.clase = 'regular'
  AND NOT EXISTS (SELECT 1 FROM tipo_planilla_concepto x WHERE x.tipo_planilla_id = t.id AND x.concepto_salarial_id = c.id);
```
(Los conceptos de aguinaldo/escolar/liquidación los siembran L19/L22.) Auditar `tipos_planilla` (regenerar con `id codigo nombre clase fraccion_mensual`) y `tipo_planilla_concepto`.
**Go**
- `planilla.ClaseTipoPlanilla` (`ClaseRegular`, `ClaseAguinaldo`, `ClaseSalarioEscolar`, `ClaseLiquidacion`) + `Valida()`; sumarla a `estados_enum_test.go` contra el ENUM.
- `dto/planilla_tipo.go`: `type TipoPlanillaRow struct{ID int64; Codigo, Nombre, Clase string; FraccionMensual decimal.NullDecimal}`; `(q) ObtenerTipoPlanilla(ctx, id int64) (TipoPlanillaRow, error)`; `(q) ObtenerTipoPlanillaPorClase(ctx, clase string) (TipoPlanillaRow, error)` (error `ErrTipoPlanillaNoEncontrado` si no hay o hay más de uno activo para clases no regulares).
**Listo cuando:** se puede crear un período `mensual` y `quincenal` por API.



ola



## L4 ⇄ — Parámetros requeridos, verificador y cargador completo
**Entra:** `internal/planilla/cargador_parametros.go` (335 l), `internal/parametros/generales.go` (firmas: `scripts/firmas.sh internal/parametros`).
**Contrato**
```go
// internal/parametros/requeridos.go
type Requerido struct{ Clave, Tipo, Descripcion string }
func Requeridos() []Requerido   // DIVISOR_DIAS_MES, SALARIO_MINIMO_INEMBARGABLE (HORAS_MENSUALES_ORDINARIAS queda deprecada en L12)
func (g *Generales) Faltantes(ctx context.Context, fecha time.Time) ([]Requerido, error)
```
**Hacer**
1. `ParametrosCalculo` agrega `DivisorDiasMes decimal.Decimal` y `SalarioMinimoInembargable` (hoy declarado y nunca cargado): ambos desde `parametros_generales`. `Cargar` falla con `ErrParametroFaltante` y mensaje con la clave. Sin tocar la conversión de porcentajes.
2. Nuevo comando `cmd/jobs/main.go` con subcomando `verificar-parametros [--fecha YYYY-MM-DD]`: imprime los `Requerido` sin valor vigente y sale con código 1 si hay alguno. (L25/L27 agregan más subcomandos.)
3. Plantilla documentada en `docs/migraciones.md` para cargar `SALARIO_MINIMO_INEMBARGABLE` como migración de datos (cierra vigencia + inserta, `fuente_legal` obligatoria, sin valor inventado).
**Test:** `Cargar` sin `SALARIO_MINIMO_INEMBARGABLE` → error con la clave; con valor → se carga.

---

# B — ORQUESTADOR

## L5 — Motor: una regla por archivo (refactor sin cambio de comportamiento)
**Entra:** `internal/planilla/motor_calculo.go` (545 l). Firmas de `calculator.go`: `bash scripts/firmas.sh internal/planilla calculator`.
**Contrato**
```go
// internal/planilla/reglas.go
type ContextoRegla struct {
    Concepto ConceptoAplicable
    Contrato ContratoVigente
    In       CalculoEmpleadoInput
    P        ParametrosCalculo
    Manuales map[int64]decimal.Decimal
    Factores map[int64]ParametroHoraExtra
    St       *estadoCalculo
}
type Regla interface{ Aplicar(rc *ContextoRegla) error }
func reglasPorDefecto() map[string]Regla  // clave = valor de expresion_formula.tipo
```
**Hacer**
1. Mover cada `case` del `switch c.TipoFormula` de `calcularConcepto` a su archivo: `regla_salario.go` (`referencia_directa`), `regla_horas_extra.go`, `regla_incapacidad.go`, `regla_ccss.go` (incluye `resolverCCSS`), `regla_renta.go`, `regla_embargo.go`, `regla_prestamo.go`, `regla_diferida.go` (aguinaldo/salario_escolar/vacaciones: hoy no-op, se llenan en L19/L20).
2. `MotorEmpleado` guarda `reglas map[string]Regla`; `NewMotorEmpleado(repo)` mantiene su firma. `tipo` desconocido sigue dando error.
3. `motor_calculo.go` queda solo con `evaluarConcepto`, `CalcularEmpleado`, totales. `CalcularHorasExtra` se queda en `regla_horas_extra.go`.
**Listo cuando:** `go test ./internal/planilla/...` pasa **sin cambiar ninguna expectativa de test**.

## L6 ⇄ — Hidratador (solo lecturas)
**Entra:** `internal/planilla/domain.go` (255 l), firmas `bash scripts/firmas.sh internal/db/dto planilla_periodo` y `planilla_motor`, `bash scripts/ddl.sh empleado_datos_laborales empleado_datos_familiares puestos incapacidades`.
**Contrato**
```go
// internal/planilla/hidratador.go
type PeriodoParaCalculo struct {
    ID int64; Inicio, Fin time.Time
    Clase string; TipoPlanillaID int64; FraccionMensual decimal.Decimal
}
type EntradaEmpleado struct {
    Input CalculoEmpleadoInput
    ContratoID, ProgramaPresupuestarioID int64
}
type FuenteHidratacion interface { /* subconjunto de dto.Queries que usa; ver tarea 1 */ }
func NewHidratador(f FuenteHidratacion) *Hidratador
func (h *Hidratador) EmpleadosDelPeriodo(ctx context.Context, p PeriodoParaCalculo) ([]int64, error)
func (h *Hidratador) Hidratar(ctx context.Context, p PeriodoParaCalculo, empleadoID int64) (EntradaEmpleado, error)
```
**Hacer**
1. `FuenteHidratacion` = solo los métodos de `dto.Queries` que ya existen (`ListarEmpleadosActivosEnRango`, `ObtenerContratoVigenteEnFecha`, `SumarHorasExtraPeriodo`, `ListarIncapacidadesSolapadas`, `ListarPrestamosActivosEnFecha`, `ListarEmbargosActivosEnFecha`, `ObtenerDatosFamiliaresEnFecha`) **más** dos nuevos en `dto/planilla_hidratacion.go`: `ListarDevengosManualesEmpleado(ctx, periodoID, empleadoID)` y `ObtenerContextoLaboral(ctx, empleadoID, fecha)` → `{ProgramaPresupuestarioID, ClasificacionPuestoID, RegimenCCSSID *int64}` (`regimen_ccss_id` aún no existe en BD: devuelve nil; L15 crea la columna y lo completa).
2. `Input.SalarioBaseNominal` **no se llena** (D5). `FechaEfectiva = Fin`. `DiasTrabajados` queda a cargo de L10: aquí `= días comerciales del período` provisional con comentario `TODO L10`.
3. Empleado sin contrato vigente en el rango → error tipado `ErrEmpleadoSinContrato{EmpleadoID}`.
**Test:** repositorio falso con 3 empleados; devengo manual llega al input; empleado sin contrato → error.

## L7 ⇄ — Escritor de detalle (solo persistencia)
**Entra:** `bash scripts/firmas.sh internal/planilla domain` + `motor_calculo`, `bash scripts/ddl.sh planilla_detalle_empleado planilla_detalle_concepto planilla_detalle_patronal planilla_detalle_embargo planilla_detalle_incapacidad planilla_detalle_prestamo periodos_planilla`.
**Contrato**
```go
// internal/planilla/escritor_detalle.go
type ResultadoPersistible struct {
    Detalle    *CalculoEmpleadoDetalle
    ContratoID, ProgramaPresupuestarioID int64
    Patronal   []LineaPatronal      // {ConceptoSalarialID int64; Base, Porcentaje, Monto decimal.Decimal}
    Incapacidad []LineaIncapacidad  // L11
    Prestamos  []LineaPrestamo      // {PrestamoID int64; CuotaCalculada, MontoRetenido decimal.Decimal}
}
type EscritorDetalle struct{}
func (e *EscritorDetalle) Escribir(ctx context.Context, q *dto.Queries, periodoID int64, rs []ResultadoPersistible) error
func (e *EscritorDetalle) EliminarDetalle(ctx context.Context, q *dto.Queries, periodoID int64) error // implementa EliminadorDetalle (F19)
func (e *EscritorDetalle) GuardarSnapshot(ctx context.Context, q *dto.Queries, periodoID int64, snapshotJSON []byte, versionMotor string) error
```
**Hacer**
1. `dto/planilla_detalle.go` con las inserciones por lote (≤500 filas por sentencia) y `EliminarDetalle*` en orden de FK (incapacidad, prestamo, embargo, patronal, concepto, empleado). `planilla_detalle_concepto.concepto_salarial_formula_id` sale de `FormulaPorConcepto`.
2. `Escribir` rellena todas las columnas nuevas de `planilla_detalle_empleado` (bases, cuotas, renta, `advertencias` como JSON) y `planilla_detalle_embargo` (`monto`=aplicado, `monto_calculado`, `monto_insatisfecho`, `orden_prioridad`).
3. Devuelve error si el detalle no cuadra en memoria (Σ líneas ≠ totales) **antes** de insertar.
**Test (integración):** escribir 2 empleados → `fn_detalle_cuadra`=1; `EliminarDetalle` deja 0 filas en las 6 tablas.

## L8 — Orquestador y cableado
**Entra:** `internal/planilla/periodo_service.go` (250 l, solo leer los ganchos), `internal/app/app.go` (214 l), `internal/api/handlers/planilla_periodo_handler.go`. Firmas de L4–L7 ya escritas arriba.
**Contrato**
```go
// internal/planilla/service.go (archivo NUEVO)
const VersionMotor = "planilla-motor-1.0.0"
func NewOrquestador(h *Hidratador, m *MotorEmpleado, c *CargadorParametros, w *EscritorDetalle, tipos TipoPlanillaLector) *Orquestador
func (o *Orquestador) Calcular(ctx context.Context, q *dto.Queries, p dto.PeriodoCicloRow, usuarioID int64) error // implementa Calculador
type ErrCalculoConErrores struct{ Errores []EmpleadoErrorOutput }  // EmpleadoErrorOutput{EmpleadoID int64; Codigo, Mensaje string}
```
**Hacer**
1. `Calcular` corre **dentro** de la transacción que ya abre `PeriodoService` (recibe `q`): por eso es todo-o-nada. Pasos: tipo y `PeriodoParaCalculo` → `Cargar(inicio, fin)` una sola vez → por empleado `Hidratar` + `CalcularEmpleado`, acumulando **todos** los errores → si hay errores, devuelve `ErrCalculoConErrores` (la transacción se revierte, BD intacta) → `Escribir` → `GuardarSnapshot` (JSON de `ParametrosCalculo` + fecha efectiva).
2. `Fraccion`/`DiasPeriodoCompleto` salen de `tipos_planilla.fraccion_mensual` y `DivisorDiasMes`: `DiasPeriodoCompleto = DivisorDiasMes × FraccionMensual`.
3. Filtro por tipo (D18): el repositorio del motor recibe `TipoPlanillaID` y `ListarConceptosAplicablesEnFecha` hace `JOIN tipo_planilla_concepto`. Cambio de firma declarado: `(ctx, regimenLaboralID, fecha)` → `(ctx, regimenLaboralID, tipoPlanillaID, fecha)` (actualizar `infra/motor_repository.go` y quien la llame).
4. `app.go`: `PeriodoGanchos{Calculador: orq, Eliminador: escritor}`. El handler traduce `ErrCalculoConErrores` a **422** con la lista completa; `ErrCalculoNoDisponible` desaparece del flujo.
5. Quita `MotorEmpleado`/`Acumulados` sueltos de `Deps` si ya no se usan (L19 los reemplaza).
**Test (integración, BD de F13):** período con 3 empleados (uno con embargo, uno con préstamo, uno con hora extra) calculado de punta a punta: `fn_detalle_cuadra`=1, snapshot guardado; un empleado sin contrato → 422 y **cero** filas; `recalcular` no duplica.
**Listo cuando:** crear → calcular → aprobar → pagar funciona por API con dos usuarios distintos.

## L9 ⇄ — Devengos manuales (API)
**Entra:** `internal/api/handlers/planilla_periodo_handler.go`, `internal/authz/permisos.go` (solo `planilla.*`).
**Hacer:** `POST|GET|DELETE /planillas/:id/devengos` (permiso `planilla.crear`; **solo** período en `borrador`; 409 si no). Valida que el concepto sea `tipo_calculo='captura_manual'` y `naturaleza='ingreso'`, monto > 0, empleado activo. Servicio `planilla/devengos_service.go` con `WithAuditContext`; queries en `dto/planilla_devengos.go`. Paginado con `internal/pagination`.
**Test:** devengo en período calculado → 409; el motor lo consume (L8).

---

# C — REGLAS (cada una toca su archivo `regla_*.go` + tests)

## L10 — Salario: fuente única, días comerciales y vigencias (000123)
**Entra:** `internal/planilla/regla_salario.go` (de L5), `internal/services/contrato_service.go` y `empleado_service.go` (solo los `TODO F22`: líneas ~202, 261, 71, 202, 236, 351, 375), `bash scripts/ddl.sh contratos empleado_datos_laborales tipos_contrato`.
**DDL (000123)**
```sql
ALTER TABLE contratos
  ADD COLUMN tipo_contrato_id BIGINT UNSIGNED NULL COMMENT 'FK a tipos_contrato; reemplaza tipo_contrato (texto, deprecado)',
  ADD CONSTRAINT fk_contratos_tipo FOREIGN KEY (tipo_contrato_id) REFERENCES tipos_contrato(id);
UPDATE contratos c JOIN tipos_contrato t ON UPPER(t.codigo) = UPPER(c.tipo_contrato) OR UPPER(t.nombre) = UPPER(c.tipo_contrato)
   SET c.tipo_contrato_id = t.id WHERE c.tipo_contrato_id IS NULL;
-- Informe (no borra): SELECT id, tipo_contrato FROM contratos WHERE tipo_contrato_id IS NULL;

CREATE TRIGGER trg_edl_salario_derivado_bi BEFORE INSERT ON empleado_datos_laborales
FOR EACH ROW
BEGIN
  DECLARE v_sal DECIMAL(14,2);
  SELECT salario_pactado INTO v_sal FROM contratos
   WHERE empleado_id = NEW.empleado_id AND fecha_fin_vigencia IS NULL
   ORDER BY fecha_inicio_vigencia DESC LIMIT 1;
  IF v_sal IS NOT NULL THEN SET NEW.salario_base_nominal = v_sal; END IF;
END;

CREATE TRIGGER trg_contratos_sync_salario_ai AFTER INSERT ON contratos
FOR EACH ROW
BEGIN
  IF NEW.fecha_fin_vigencia IS NULL THEN
    UPDATE empleado_datos_laborales SET salario_base_nominal = NEW.salario_pactado
     WHERE empleado_id = NEW.empleado_id AND fecha_fin_vigencia IS NULL;
  END IF;
END;
```
(`salario_base_nominal` pasa a **copia derivada** mantenida por la BD; regenerar auditoría de `contratos` y `empleado_datos_laborales`.)
**Contrato (funciones puras, `planilla/salario_periodo.go`)**
```go
func DiasComerciales(desde, hasta time.Time, divisor int) int  // D15; test: feb 28/29, meses de 31, quincenas
type VigenciaContrato struct{ ContratoID int64; Desde, Hasta time.Time; Salario decimal.Decimal }
type SegmentoSalario struct{ Inicio, Fin time.Time; Salario decimal.Decimal; ContratoID int64; Dias int }
func SegmentarPeriodo(desde, hasta time.Time, v []VigenciaContrato) ([]SegmentoSalario, error) // error en solape; los días sin contrato quedan fuera
func SalarioBasePeriodo(s []SegmentoSalario, divisor decimal.Decimal) decimal.Decimal           // Σ(salario×dias/divisor), un solo Round(2) al final
```
**Hacer**
1. `dto.ListarContratosEnRango(ctx, empleadoID, desde, hasta)`; el hidratador (L6) llena `Input.Segmentos`; `regla_salario` usa `SalarioBasePeriodo` y deja de usar `DiasTrabajados`/`DiasPeriodoCompleto`/`SalarioBaseNominal` (elimina la advertencia de "salarios difieren").
2. `Input.DiasTrabajados` pasa a ser el total de días comerciales en segmentos **menos** incapacidad (lo ajusta L11). Horas extra siguen usando el salario mensual del segmento vigente a `Fin`.
3. Servicios: borrar la escritura de `salario_base_nominal` (los 4 `TODO F22`); `contrato_service` escribe `tipo_contrato_id`.
**Tests golden:** mes completo; ingreso día 10; aumento día 16 (2 segmentos); cese día 20; febrero completo = salario mensual exacto; quincena 16–31 = medio salario.

## L11 ⇄ — Incapacidades
**Entra:** `internal/planilla/regla_incapacidad.go`, `bash scripts/ddl.sh parametros_subsidios_incapacidad incapacidades planilla_detalle_incapacidad`, firma de `ListarIncapacidadesSolapadas` y `TramoSubsidioIncapacidadRow` (`scripts/firmas.sh internal/db/dto planilla_periodo`).
**Contrato**
```go
type IncapacidadInput struct{ IncapacidadID, EntidadEmisoraID int64; Desde, Hasta time.Time; DiaSerieDesde int32 } // días 1.. de la incapacidad CONTINUA (puede venir de períodos anteriores)
type TramoSubsidio struct{ EntidadEmisoraID int64; DiaDesde int32; DiaHasta *int32; PctPatronal, PctEntidad decimal.Decimal } // fracciones
type SubsidioIncapacidad struct{ Dias int; MontoPatronal, MontoEntidadInformativo decimal.Decimal; Desglose []LineaIncapacidad }
func CalcularSubsidioIncapacidad(salarioDiario decimal.Decimal, inc IncapacidadInput, tramos []TramoSubsidio, divisor int) (SubsidioIncapacidad, error)
```
**Hacer**
1. Ajustar `TramoSubsidioIncapacidad` del cargador para traer `dia_hasta` y `porcentaje_entidad` (ya existen en la tabla). Salario diario = salario mensual del segmento / `DivisorDiasMes`.
2. `regla_incapacidad`: descuenta los días comerciales de incapacidad del salario base (no se paga dos veces), agrega `SUB_INCAP` por el monto **patronal** y deja el tramo de CCSS/INS solo en `planilla_detalle_incapacidad.monto_entidad_informativo` (no suma al neto). El efecto en bases CCSS/renta lo deciden las banderas del concepto.
3. Eliminar el error "aún no implementado" y el `case` vacío.
**Tests golden:** 2 días dentro del período; 10 días que cruzan dos períodos (numeración continua); incapacidad que cubre todo el período (salario base 0, subsidio por tramos); tramo sin `dia_hasta`.
`SUPUESTO A VALIDAR:` quién absorbe los primeros días y si el subsidio cotiza a CCSS (lo decide la BD, no el código).

## L12 — Jornada y divisor de hora extra (000124)
**Entra:** `internal/planilla/regla_horas_extra.go`, `bash scripts/ddl.sh turnos empleado_turno contratos`.
**DDL**
```sql
CREATE TABLE jornadas (
  id     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  codigo VARCHAR(20)  NOT NULL UNIQUE COMMENT 'DIURNA, MIXTA, NOCTURNA',
  nombre VARCHAR(100) NOT NULL COMMENT 'Nombre de la jornada',
  activo TINYINT(1) NOT NULL DEFAULT 1 COMMENT '1 = utilizable'
) ENGINE=InnoDB COMMENT='Catálogo de jornadas';
INSERT INTO jornadas (codigo, nombre)
SELECT v.c, v.n FROM (SELECT 'DIURNA' c, 'Jornada diurna' n UNION ALL SELECT 'MIXTA','Jornada mixta' UNION ALL SELECT 'NOCTURNA','Jornada nocturna') v
WHERE NOT EXISTS (SELECT 1 FROM jornadas j WHERE j.codigo = v.c);

CREATE TABLE parametros_jornada (
  id                       BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  jornada_id               BIGINT UNSIGNED NOT NULL COMMENT 'Jornada',
  horas_diarias_ordinarias DECIMAL(4,2) NOT NULL COMMENT 'Horas ordinarias por día',
  divisor_horas_mensuales  DECIMAL(6,2) NOT NULL COMMENT 'Divisor del valor hora mensual',
  fuente_legal             VARCHAR(150) NOT NULL COMMENT 'Base legal o PENDIENTE VALIDAR',
  fecha_inicio_vigencia    DATE NOT NULL COMMENT 'Inicio de vigencia',
  fecha_fin_vigencia       DATE NULL COMMENT 'NULL = vigente',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED COMMENT 'Técnica: una vigente por jornada',
  creado_por               INT NOT NULL COMMENT 'Usuario',
  created_at               DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creación',
  CONSTRAINT fk_pj_jornada FOREIGN KEY (jornada_id) REFERENCES jornadas(id),
  CONSTRAINT chk_pj CHECK (horas_diarias_ordinarias > 0 AND horas_diarias_ordinarias <= 24 AND divisor_horas_mensuales > 0),
  CONSTRAINT chk_pj_vig CHECK (fecha_fin_vigencia IS NULL OR fecha_fin_vigencia >= fecha_inicio_vigencia),
  UNIQUE KEY uk_pj_una_vigente (jornada_id, es_vigente)
) ENGINE=InnoDB COMMENT='Divisor de la hora ordinaria por jornada, versionado por vigencia';

-- Sembrar SOLO lo que hoy está en el código (HORAS_MENSUALES_ORDINARIAS=240) para la jornada DIURNA, fuente_legal='VALOR HISTÓRICO DEL CÓDIGO - PENDIENTE VALIDAR'.
-- MIXTA y NOCTURNA NO se siembran (valor legal): los carga Planillas por L24 o migración de datos.

ALTER TABLE contratos ADD COLUMN jornada_id BIGINT UNSIGNED NULL COMMENT 'FK a jornadas; reemplaza jornada (texto, deprecado)',
  ADD CONSTRAINT fk_contratos_jornada FOREIGN KEY (jornada_id) REFERENCES jornadas(id);
UPDATE contratos c JOIN jornadas j ON UPPER(j.codigo) = UPPER(c.jornada) SET c.jornada_id = j.id WHERE c.jornada_id IS NULL;
ALTER TABLE turnos ADD COLUMN jornada_id BIGINT UNSIGNED NULL COMMENT 'Jornada a la que pertenece el turno',
  ADD CONSTRAINT fk_turnos_jornada FOREIGN KEY (jornada_id) REFERENCES jornadas(id);
```
Regenerar auditoría de `contratos`, `turnos`.
**Go:** el cargador entrega `DivisorPorJornada map[int64]decimal.Decimal`; `ParametrosCalculo.HorasMensualesOrdinarias` se elimina; `CalcularHorasExtra(salario, divisorJornada, …)`. `ContratoVigente` trae `JornadaID`. Jornada sin parámetro vigente → error explícito. `HORAS_MENSUALES_ORDINARIAS` sale de `Requeridos()`. `contrato_service` escribe `jornada_id`.
**Tests golden:** diurna con factor 1.5; jornada sin divisor → error.

## L13 ⇄ — Horas extra: autorización (000125)
**Entra:** `internal/asistencia/service.go` (solo exceso/candidatas), `internal/authz/permisos.go`, `bash scripts/ddl.sh registro_asistencia_detalle tipos_hora_extra` + firma de `SumarHorasExtraPeriodo`.
**DDL**
```sql
CREATE TABLE horas_extra_autorizadas (
  id                 BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  empleado_id        BIGINT UNSIGNED NOT NULL COMMENT 'Empleado',
  fecha              DATE NOT NULL COMMENT 'Día de la hora extra',
  horas_autorizadas  DECIMAL(5,2) NOT NULL COMMENT 'Tope autorizado',
  tipo_hora_extra_id BIGINT UNSIGNED NULL COMMENT 'Tipo, si se fija de antemano',
  motivo             VARCHAR(255) NOT NULL COMMENT 'Justificación',
  estado             ENUM('pendiente','autorizada','rechazada') NOT NULL DEFAULT 'pendiente' COMMENT 'Estado',
  solicitado_por     BIGINT UNSIGNED NOT NULL COMMENT 'Quien solicita',
  fecha_solicitud    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Solicitud',
  resuelto_por       BIGINT UNSIGNED NULL COMMENT 'Quien autoriza/rechaza',
  fecha_resolucion   DATETIME NULL COMMENT 'Resolución',
  motivo_resolucion  VARCHAR(255) NULL COMMENT 'Motivo del rechazo',
  CONSTRAINT fk_hea_emp   FOREIGN KEY (empleado_id)        REFERENCES empleados(id),
  CONSTRAINT fk_hea_tipo  FOREIGN KEY (tipo_hora_extra_id) REFERENCES tipos_hora_extra(id),
  CONSTRAINT fk_hea_sol   FOREIGN KEY (solicitado_por)     REFERENCES usuarios(id),
  CONSTRAINT fk_hea_res   FOREIGN KEY (resuelto_por)       REFERENCES usuarios(id),
  CONSTRAINT chk_hea_horas CHECK (horas_autorizadas > 0 AND horas_autorizadas <= 24),
  CONSTRAINT chk_hea_segregacion CHECK (resuelto_por IS NULL OR resuelto_por <> solicitado_por),
  CONSTRAINT chk_hea_estado CHECK ((estado = 'pendiente' AND resuelto_por IS NULL) OR (estado <> 'pendiente' AND resuelto_por IS NOT NULL)),
  UNIQUE KEY uk_hea (empleado_id, fecha),
  KEY ix_hea_estado (estado, fecha)
) ENGINE=InnoDB COMMENT='Autorización previa de horas extra; quien solicita no puede resolver (CHECK)';
```
+ permiso `horas_extra.autorizar` (INSERT…SELECT…WHERE NOT EXISTS en `permisos` y asignación al rol `RRHH` por `codigo`, no por id) y su constante en `authz.Todos()` (el test de coherencia lo exige). Auditar la tabla.
**Hacer:** endpoints `POST /horas-extra/autorizaciones` (permiso `asistencia.crear`), `POST …/:id/autorizar|rechazar` (`horas_extra.autorizar`), `GET …`. `dto.SumarHorasExtraPeriodo` suma **solo** `min(horas extra calculadas del día, horas_autorizadas)` de días `autorizada`.
**Test:** exceso sin autorización → no llega a planilla; autorizado menos de lo trabajado → se paga lo autorizado; mismo usuario solicita y autoriza → 403 (Go) y rechazo del CHECK (BD).

## L14 ⇄ — Horas extra: tipificación (000126)
**Entra:** `internal/asistencia/horas.go` (95 l), `bash scripts/ddl.sh tipos_hora_extra registro_asistencia_detalle turnos`, `bash scripts/ddl.sh --triggers calendario_laboral_dia`.
**DDL**
```sql
ALTER TABLE tipos_hora_extra
  ADD COLUMN clase ENUM('diurna','nocturna','mixta','doble') NULL COMMENT 'Regla de tipificación; Go no usa codigo',
  ADD UNIQUE KEY uk_the_clase (clase);
UPDATE tipos_hora_extra SET clase = LOWER(codigo) WHERE LOWER(codigo) IN ('diurna','nocturna','mixta','doble') AND clase IS NULL;

CREATE TABLE parametros_franja_nocturna (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  hora_inicio TIME NOT NULL COMMENT 'Inicio de la franja nocturna', hora_fin TIME NOT NULL COMMENT 'Fin de la franja nocturna (puede cruzar medianoche)',
  fuente_legal VARCHAR(150) NOT NULL COMMENT 'Base legal o PENDIENTE VALIDAR',
  fecha_inicio_vigencia DATE NOT NULL COMMENT 'Inicio', fecha_fin_vigencia DATE NULL COMMENT 'NULL = vigente',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED COMMENT 'Técnica',
  creado_por INT NOT NULL COMMENT 'Usuario', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creación',
  UNIQUE KEY uk_pfn_una_vigente (es_vigente)
) ENGINE=InnoDB COMMENT='Franja horaria nocturna vigente (NO se siembra: valor legal)';

CREATE TABLE asistencia_hora_extra_tipificada (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  empleado_id BIGINT UNSIGNED NOT NULL COMMENT 'Empleado', fecha DATE NOT NULL COMMENT 'Día',
  tipo_hora_extra_id BIGINT UNSIGNED NOT NULL COMMENT 'Tipo resultante', horas DECIMAL(5,2) NOT NULL COMMENT 'Horas de ese tipo',
  CONSTRAINT fk_ahet_emp FOREIGN KEY (empleado_id) REFERENCES empleados(id),
  CONSTRAINT fk_ahet_tipo FOREIGN KEY (tipo_hora_extra_id) REFERENCES tipos_hora_extra(id),
  CONSTRAINT chk_ahet CHECK (horas > 0),
  UNIQUE KEY uk_ahet (empleado_id, fecha, tipo_hora_extra_id)
) ENGINE=InnoDB COMMENT='Horas extra tipificadas por día';
```
**Contrato**
```go
type HoraExtraTipificada struct{ Clase string; Horas decimal.Decimal }
func TipificarHorasExtra(entrada, salida, turnoIni, turnoFin time.Time, franja FranjaNocturna, esFeriado, esDescanso bool, jornadaMixta bool) ([]HoraExtraTipificada, error)
```
Por minuto (enteros, sin float): exceso fuera del turno; feriado/descanso → `doble`; si no, franja nocturna → `nocturna` (o `mixta` si `jornadaMixta`), resto `diurna`. `SUPUESTO A VALIDAR` para `mixta`. Sin franja vigente → error explícito. `dto.SumarHorasExtraPeriodo` agrupa por `tipo_hora_extra_id` desde esta tabla (resuelve el id por `clase`).
**Tests golden:** diurna; cruza a nocturna; feriado; día de descanso; turno nocturno que cruza medianoche.

## L15 ⇄ — CCSS con base mínima y `regimen_ccss_id` (000127)
**Entra:** `internal/planilla/regla_ccss.go`, `internal/services/empleado_service.go` (solo alta/actualización de datos laborales), `bash scripts/ddl.sh empleado_datos_laborales regimenes_ccss parametros_base_minima_contributiva`.
**DDL**
```sql
ALTER TABLE empleado_datos_laborales
  ADD COLUMN regimen_ccss_id BIGINT UNSIGNED NULL COMMENT 'Régimen CCSS del empleado (D17); sin valor = el cálculo falla',
  ADD CONSTRAINT fk_edl_regimen_ccss FOREIGN KEY (regimen_ccss_id) REFERENCES regimenes_ccss(id);
```
(regenerar auditoría de `empleado_datos_laborales` con la columna nueva). **No hay backfill**: la asignación es dato humano; `SELECT` de empleados sin régimen documentado en el `.up.sql`.
**Hacer**
1. API/servicio de empleados: aceptar y validar `regimen_ccss_id` en datos laborales; listar empleados sin régimen (`GET /empleados?sin_regimen_ccss=true`).
2. `ObtenerContextoLaboral` (L6) devuelve `RegimenCCSSID`; hidratador: nil → `ErrEmpleadoSinRegimenCCSS`.
3. `regla_ccss`: `base = max(base real, baseMinima[régimen] × fracciónMensual)` — `SUPUESTO A VALIDAR` (prorrateo de la mínima por período). Sin base mínima vigente → error explícito. Persistir la base **usada** en `base_ccss` y en `planilla_detalle_patronal`. D6: ningún concepto `INS_RT/BANPOP_PAT/FCL_PAT` se calcula ni se captura.
**Tests golden:** base bajo la mínima; base sobre la mínima; quincena; empleado sin régimen → error.

## L16 ⇄ — Embargos 100 % desde BD (000128)
**Entra:** `internal/planilla/calculator.go` (líneas 20–50 y 210–345), `internal/planilla/regla_embargo.go`, `bash scripts/ddl.sh embargos parametros_tipos_embargo parametros_embargo_escala conceptos_salariales`.
**DDL**
```sql
ALTER TABLE embargos ADD COLUMN tope_mensual DECIMAL(14,2) NULL COMMENT 'Tope mensual propio del embargo; NULL = sin tope';
ALTER TABLE conceptos_salariales
  ADD COLUMN afecta_base_embargo TINYINT(1) NOT NULL DEFAULT 0 COMMENT '1 = el ingreso integra el líquido embargable (salarial)';
UPDATE conceptos_salariales SET afecta_base_embargo = afecta_base_renta WHERE naturaleza = 'ingreso';
```
(regenerar auditoría de `embargos` y `conceptos_salariales`.)
**Contrato**
```go
type EmbargoInput struct { EmbargoID, TipoEmbargoID int64; Prioridad int32; PrioridadLegal int32
    EsPension bool; TopePctLiquido decimal.Decimal /*fracción, 0=sin tope*/; Porcentaje, MontoFijo, TopeMensual decimal.Decimal }
func CalcularEmbargos(emb []EmbargoInput, liquidoEmbargable, salarioMinInembargable decimal.Decimal, escala []TramoEscalaEmbargo) ([]EmbargoAplicadoOutput, error)
```
**Hacer**
1. Eliminar de `calculator.go` `PrioridadPensionAlimentaria`, `art172MultiplicadorTramoUno`, `fraccionTramoUno/Dos`, `fraccionMaximaPensionAlimentaria`. **Preservar la semántica** de las líneas 216–300 usando `escala` (múltiplos del mínimo inembargable, % por tramo) y `EsPension`.
2. Tope de pensión **acumulado** sobre todas las pensiones del empleado (no por embargo). `TopeMensual` real desde `embargos.tope_mensual`.
3. Líquido embargable = Σ ingresos con `afecta_base_embargo` − CCSS obrera − renta neta. Orden por `PrioridadLegal` de `parametros_tipos_embargo` (dato).
4. Persistir `monto_calculado`/`monto`/`monto_insatisfecho` (L7). D19: no arrastrar.
**Tests golden:** una pensión; dos pensiones que juntas pasan el tope; judicial + pensión por prioridad; líquido ≤ mínimo inembargable; sin `SALARIO_MINIMO_INEMBARGABLE` → error. Adaptar `calculator_test.go` (los números esperados no cambian si la escala sembrada es la histórica).

## L17 — Tope de neto: obligatorias vs voluntarias (000129)
**Entra:** `internal/planilla/motor_calculo.go` (solo totales y chequeo de neto negativo), `bash scripts/ddl.sh conceptos_salariales`.
**DDL**
```sql
ALTER TABLE conceptos_salariales
  ADD COLUMN es_deduccion_obligatoria TINYINT(1) NOT NULL DEFAULT 0 COMMENT '1 = se aplica antes que las voluntarias y no se topa por neto';
UPDATE conceptos_salariales SET es_deduccion_obligatoria = 1 WHERE codigo IN ('CCSS_OBR','IMP_RENTA','EMBARGO');
```
(regenerar auditoría.) **Hacer** en `internal/planilla/tope_neto.go`: tras evaluar todos los conceptos, `AplicarTopeNeto(st)`: remanente = ingresos − obligatorias; las voluntarias se aplican en `orden_calculo` hasta agotar el remanente; la que no cabe se reduce, las siguientes quedan en 0 y se registran como advertencia. `monto_retenido < cuota_calculada` en `planilla_detalle_prestamo` (D19: no se arrastra). Se elimina el aborto por "deducciones exceden ingresos"; el neto **nunca** es negativo; si las obligatorias solas lo superan (solo posible con ingresos negativos de ajuste) → error explícito.
**Tests golden:** deducciones > remanente → neto 0 y préstamo parcial; todas caben; solo obligatorias.

## L18 — Préstamos al aprobar y gancho múltiple (000130)
**Entra:** `internal/planilla/periodo.go` (ganchos), `internal/prestamos/service.go` + `estados.go`, `bash scripts/ddl.sh prestamos planilla_detalle_prestamo`.
**DDL**
```sql
ALTER TABLE prestamos MODIFY estado ENUM('activo','cancelado','suspendido','pagado') NOT NULL DEFAULT 'activo';

CREATE TRIGGER trg_prestamos_saldado_bu BEFORE UPDATE ON prestamos
FOR EACH ROW
BEGIN
  IF NEW.saldo_pendiente = 0 AND OLD.saldo_pendiente > 0 AND NEW.estado = 'activo' THEN
    SET NEW.estado = 'pagado';
  END IF;
END;
```
**Contrato**
```go
// planilla/periodo.go
type GanchosAlAprobar []GanchoAlAprobar          // implementa GanchoAlAprobar, ejecuta en orden y corta en el primer error
// planilla/aplicador_prestamos.go
func NewAplicadorPrestamos() GanchoAlAprobar
```
**Hacer:** `AlAprobar`: por cada `planilla_detalle_prestamo` con `aplicado_en IS NULL` del período → `SELECT … FOR UPDATE` del préstamo, `saldo -= monto_retenido` (error si quedaría < 0), `aplicado_en = NOW()`. **Idempotente** (reintento no duplica). `PeriodoGanchos.AlAprobar` pasa a `GanchosAlAprobar{préstamos}` (L19/L20 agregan los suyos). Recalcular antes de aprobar no toca saldos. `EstadoPrestamoPagado` queda confirmado por el test de ENUM.
**Tests (integración):** aprobar baja el saldo exactamente una vez; doble aprobación concurrente (una gana); préstamo que se salda → `pagado`.

## L19 — Aguinaldo y salario escolar dentro del ciclo (000131)
**Entra:** `internal/planilla/acumulados_periodicos.go` (238 l), `internal/planilla/regla_diferida.go`, `internal/services/liquidacion_service.go` (solo `grep -n "Aguinaldo\|SalarioEscolar"`), `bash scripts/ddl.sh aguinaldos salarios_escolares tipos_planilla conceptos_salariales`.
**DDL**
```sql
ALTER TABLE aguinaldos         ADD COLUMN periodo_planilla_id BIGINT UNSIGNED NULL COMMENT 'Período que lo calculó',
  ADD CONSTRAINT fk_aguinaldo_periodo FOREIGN KEY (periodo_planilla_id) REFERENCES periodos_planilla(id);
ALTER TABLE salarios_escolares ADD COLUMN periodo_planilla_id BIGINT UNSIGNED NULL COMMENT 'Período que lo calculó',
  ADD CONSTRAINT fk_salesc_periodo FOREIGN KEY (periodo_planilla_id) REFERENCES periodos_planilla(id);

INSERT INTO tipo_planilla_concepto (tipo_planilla_id, concepto_salarial_id)
SELECT t.id, c.id FROM tipos_planilla t JOIN conceptos_salariales c
  ON (t.clase = 'aguinaldo' AND c.codigo = 'AGUINALDO') OR (t.clase = 'salario_escolar' AND c.codigo = 'SALARIO_ESCOLAR')
WHERE NOT EXISTS (SELECT 1 FROM tipo_planilla_concepto x WHERE x.tipo_planilla_id = t.id AND x.concepto_salarial_id = c.id);
```
(regenerar auditoría de ambas tablas.)
**Contrato**
```go
// planilla/base_acumulada.go  (D4: períodos aprobada|pagada|contabilizada)
func (s *BaseAcumuladaService) Sumar(ctx context.Context, q *dto.Queries, empleadoID int64, desde, hasta time.Time, bandera string) (decimal.Decimal, error) // suma planilla_detalle_empleado.base_aguinaldo|base_vacaciones|base_cesantia
// planilla/ventanas.go
func (v *Ventanas) Aguinaldo(ctx context.Context, anio int, hoy time.Time) (VentanaAcumulado, error)       // lee AGUINALDO_* y SALESC_* de parametros_generales (ya sembrados)
func (v *Ventanas) SalarioEscolar(ctx context.Context, anio int, hoy time.Time) (VentanaAcumulado, error)
```
**Hacer**
1. `regla_diferida`: `aguinaldo` = `BaseAguinaldoAcumulada / AGUINALDO_MESES`; `salario_escolar` = `BaseSalarioEscolarAcumulada × porcentaje` (vía cargador, fracción). Un solo redondeo a 2 decimales al final. Se llenan en `Input` por el hidratador cuando `Clase` ≠ `regular`. Sin cargas ni renta: lo dicen las banderas de `AGUINALDO`/`SALARIO_ESCOLAR`, no el código (`SUPUESTO A VALIDAR`: reportar banderas sembradas).
2. `POST /aguinaldo/:anio/ejecutar` (`aguinaldo.ejecutar`) y `POST /salario-escolar/:anio/ejecutar` (`salario_escolar.ejecutar`): crean el **período** de la clase correspondiente (`ObtenerTipoPlanillaPorClase`) con las fechas de la ventana de cálculo, validan la ventana de ejecución (fuera de ventana → 409, sin opción de forzar) y devuelven el período. Luego siguen el ciclo normal (`/planillas/:id/calcular|aprobar|pagar`, dispersión, contabilizar).
3. Al calcular: el escritor (L7) hace *upsert* en `aguinaldos`/`salarios_escolares` (`estado='proyectado'`, `periodo_planilla_id`); gancho `AlAprobar` los pasa a `procesado`. `UNIQUE (empleado, anio)` evita doble corrida.
4. **Eliminar** duplicados y constantes: `services.CalcularAguinaldo/CalcularSalarioEscolar`, `mesesAguinaldo`, `fechaUTC`, y `AcumuladosService` con sus `Ejecutar*` y repositorio `infra/acumulados_repository.go` (el código muerto sale en esta fase; ajustar `Deps` y `liquidacion_service` solo en lo que referencie).
5. `PorcentajeSalarioEscolar`: se mantiene 8.33 % sembrado; `PENDIENTE VALIDAR (8.19 %)` en informe.
**Tests golden:** 12 meses completos; ingreso a mitad de año; empleado con incapacidad (según banderas); doble ejecución → 409; fuera de ventana → 409.

## L20 — Vacaciones en planilla y acumulación automática (000132)
**Entra:** `internal/planilla/regla_diferida.go`, `internal/asistencia/vacaciones_service.go` (solo acumulación y movimientos), `bash scripts/ddl.sh vacaciones_movimientos vacaciones_acumulado conceptos_salariales concepto_salarial_formula regimen_concepto_aplicable`.
**Paso 0 (obligatorio):** antes de programar, escribe en `LITE.md` qué entendiste que debe pagarse con `PAGO_VACACIONES` (compensación en dinero vs días disfrutados que ya van en el salario). Si los metadatos no lo deciden, **detente y pregunta**.
**DDL**
```sql
ALTER TABLE vacaciones_movimientos
  MODIFY tipo_movimiento ENUM('acumulacion','disfrute','ajuste','compensacion') NOT NULL,
  ADD COLUMN periodo_planilla_id BIGINT UNSIGNED NULL COMMENT 'Período que pagó la compensación',
  ADD COLUMN pagado_en DATETIME NULL COMMENT 'Se llena al aprobar el período; NULL = no pagado',
  ADD CONSTRAINT fk_vacmov_periodo FOREIGN KEY (periodo_planilla_id) REFERENCES periodos_planilla(id);
```
**Hacer**
1. `regla_diferida` `vacaciones`: monto = días de movimientos `compensacion` del empleado **sin `pagado_en`** × (salario del segmento ÷ `DivisorDiasMes`). Los días `disfrute` **no** reducen `DiasTrabajados`.
2. Gancho `AlAprobar` (agregarlo a `GanchosAlAprobar`): marca `pagado_en` e idempotente (`WHERE pagado_en IS NULL`).
3. `cmd/jobs acumular-vacaciones [--fecha]`: recorre empleados activos y llama `AcumularVacaciones` (idempotente desde F07; escala desde `parametros_vacaciones_escala`). `docs/operacion.md`: cómo programarlo y que es seguro re-ejecutarlo.
**Tests:** acumulación diaria repetida no duplica; disfrute no descuenta; compensación genera monto y queda pagada al aprobar, una sola vez.

## L21 — Liquidación: parámetros y cálculo puro (000133)
**Entra:** `internal/services/liquidacion_service.go` (738 l: solo funciones de cálculo), `bash scripts/ddl.sh parametros_cesantia_escala parametros_preaviso_escala liquidaciones regimenes_laborales`.
**DDL (completar tras leer los DDL de entrada)**
```sql
CREATE TABLE parametros_motivos_salida (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  motivo_salida ENUM('despido_sin_responsabilidad','despido_con_responsabilidad','despido_indirecto','renuncia','vencimiento_contrato','mutuo_acuerdo','fallecimiento') NOT NULL COMMENT 'Mismo ENUM que liquidaciones.motivo_salida',
  paga_preaviso TINYINT(1) NOT NULL COMMENT 'Corresponde preaviso', paga_cesantia TINYINT(1) NOT NULL COMMENT 'Corresponde cesantía',
  paga_vacaciones TINYINT(1) NOT NULL COMMENT 'Vacaciones proporcionales', paga_aguinaldo TINYINT(1) NOT NULL COMMENT 'Aguinaldo proporcional',
  paga_salario_escolar TINYINT(1) NOT NULL COMMENT 'Salario escolar proporcional',
  observacion VARCHAR(255) NULL COMMENT 'Notas', fuente_legal VARCHAR(150) NOT NULL COMMENT 'Base legal o PENDIENTE VALIDAR',
  fecha_inicio_vigencia DATE NOT NULL COMMENT 'Inicio', fecha_fin_vigencia DATE NULL COMMENT 'NULL = vigente',
  es_vigente TINYINT GENERATED ALWAYS AS (CASE WHEN fecha_fin_vigencia IS NULL THEN 1 ELSE NULL END) STORED COMMENT 'Técnica',
  creado_por INT NOT NULL COMMENT 'Usuario', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creación',
  UNIQUE KEY uk_pms_una_vigente (motivo_salida, es_vigente)
) ENGINE=InnoDB COMMENT='Qué rubros paga cada motivo de salida (D8: dato, no if en Go)';
```
Seed conservador con `PENDIENTE VALIDAR`: vacaciones/aguinaldo/escolar = 1 en todos; preaviso/cesantía = 1 solo en `despido_con_responsabilidad` y `despido_indirecto`; el resto 0; reportar cada duda.
**Hacer**
1. Extender `parametros_cesantia_escala`/`parametros_preaviso_escala` (migración, con vigencia y `fuente_legal='VALOR HISTÓRICO DEL CÓDIGO - PENDIENTE VALIDAR'`) para cubrir los tramos que hoy están en el código (3, 6, 12 meses; 7 y 14 días; año tope). Sacar del servicio `DivisorDiasMes`, `mesesFraccion > 6` (`LIQ_FRACCION_MESES_UMBRAL`, ya sembrado) y `meses += 12`.
2. Antigüedad desde `empleados.fecha_ingreso` (continuidad entre contratos). `SUPUESTO A VALIDAR` si hubo interrupciones.
3. Ventana del promedio de vacaciones parametrizada: `VENTANA_PROMEDIO_VACACIONES_SEMANAS` (sembrar el equivalente actual a 6 meses, `PENDIENTE VALIDAR: Art. 156 usa 50 semanas`).
4. Extraer a `internal/liquidacion/calculo.go` (puro): `CalcularRubros(in EntradaLiquidacion, p ParametrosLiquidacion) (RubrosLiquidacion, error)`; `proyectar` y `procesar` usan **el mismo** cálculo.
**Tests golden:** 2 contratos consecutivos (antigüedad continua); renuncia con 5 meses; despido con 3 años; fracción exacta en el umbral; escala sin tramo → error.

## L22 — Liquidación por el motor (000134)
**Entra:** `internal/services/liquidacion_service.go` (solo `Procesar`/cierre de expediente), `internal/planilla/hidratador.go`, `bash scripts/ddl.sh liquidaciones conceptos_salariales`.
**DDL**
```sql
ALTER TABLE liquidaciones ADD COLUMN periodo_planilla_id BIGINT UNSIGNED NULL COMMENT 'Período mensual de liquidaciones que la paga (D14)',
  ADD CONSTRAINT fk_liq_periodo FOREIGN KEY (periodo_planilla_id) REFERENCES periodos_planilla(id);

INSERT INTO conceptos_salariales (codigo, nombre, naturaleza, tipo_calculo, afecta_base_ccss, afecta_base_renta, afecta_base_aguinaldo, afecta_base_cesantia, afecta_base_vacaciones, activo)
SELECT v.codigo, v.nombre, 'ingreso', 'formula', v.ccss, v.renta, 0, 0, 0, 1 FROM (
  SELECT 'LIQ_PREAVISO' codigo, 'Preaviso' nombre, 0 ccss, 0 renta
  UNION ALL SELECT 'LIQ_CESANTIA', 'Cesantía', 0, 0
  UNION ALL SELECT 'LIQ_VACACIONES', 'Vacaciones proporcionales', 1, 1
  UNION ALL SELECT 'LIQ_AGUINALDO', 'Aguinaldo proporcional', 0, 0
  UNION ALL SELECT 'LIQ_SAL_ESCOLAR', 'Salario escolar proporcional', 0, 0
) v WHERE NOT EXISTS (SELECT 1 FROM conceptos_salariales c WHERE c.codigo = v.codigo);
-- + concepto_salarial_formula (expresion_formula {"tipo":"liquidacion","rubro":"<preaviso|cesantia|vacaciones|aguinaldo|salario_escolar>"}), orden_calculo_concepto, regimen_concepto_aplicable y tipo_planilla_concepto (clase 'liquidacion': los LIQ_* + CCSS_OBR + IMP_RENTA + EMBARGO), copiando el patrón de 000002/000005.
-- Banderas de LIQ_*: SUPUESTO A VALIDAR (conservador: preaviso/cesantía sin CCSS ni renta).
```
(regenerar auditoría de `liquidaciones`.)
**Hacer**
1. `regla_liquidacion.go` (formula `liquidacion`): el monto sale de `Input.Liquidacion.Rubros[rubro]` calculado en L21; las deducciones (CCSS, renta, embargo, préstamos) las aplican las reglas normales. Los rubros respetan las banderas de su concepto.
2. `Procesar`: en **una** transacción auditada crea la `liquidaciones` (`procesada`), cierra contrato y datos laborales y desactiva al empleado **solo si `fecha_salida <= hoy`**; si es futura queda programada y la cierra `cmd/jobs cerrar-salidas` (idempotente).
3. Período mensual de clase `liquidacion` (se crea si no existe para el mes de `fecha_salida`); el hidratador, para esa clase, toma las `liquidaciones` `procesada` con `periodo_planilla_id IS NULL` del mes y las asocia al calcular. Luego: aprobar → pagar → dispersión → contabilizar (L23).
4. `proyectar` no escribe nada.
**Tests golden:** salida futura (no se desactiva hoy); embargo activo aplicado; dos liquidaciones el mismo día (mismo período); recálculo no duplica.

---

# D — SISTEMA COMPLETO

## L23 — Contabilidad (000135)
**Entra:** `internal/contabilidad/service.go` (397 l), `internal/contabilidad/sql_adapters.go` (296 l), `bash scripts/ddl.sh tipos_asiento plantillas_asiento_linea mapeo_concepto_cuenta asientos_contables asientos_contables_detalle`.
**Hacer**
1. **Cuota patronal persistida (D20):** `DetallesSQL` lee `planilla_detalle_empleado.cuota_patronal_ccss` y `planilla_detalle_patronal`; eliminar `porcentajeVigente`, el recálculo y el aborto "cuota obrera recalculada no coincide".
2. **Segregación:** `ContabilizarPeriodo` llama `planilla.VerificarSegregacion(EstadoContabilizada, usuarioID, …)` y completa `contabilizado_por_usuario_id`/`fecha_contabilizacion` en la misma transacción (hoy no lo hace).
3. **Asiento de pago:** al pagar (gancho `AlPagar`, F19) generar el asiento de pago (pasivo de planilla contra bancos) con la plantilla del tipo de asiento; cuadre debe = haber verificado (`ValidarPartidaDoble`).
4. Soporta clases `aguinaldo`, `salario_escolar` y `liquidacion` por `mapeo_concepto_cuenta`. **Las cuentas son dato de Contabilidad**: la migración 000135 solo agrega lo estructural (p. ej. tipo de asiento de pago) y deja plantilla `.sql` comentada de `mapeo_concepto_cuenta` para que Contabilidad la complete; sin mapeo vigente → error explícito que lista el concepto sin cuenta.
**Tests:** asiento descuadrado → error; mismo usuario que pagó intenta contabilizar → `ErrSegregacionFunciones`; período en estado incorrecto; concepto sin mapeo.

## L24 ⇄ — API de parámetros legales (000136)
**Entra:** `internal/parametros/generales.go` (257 l), `internal/authz/permisos.go` (`parametros.*`), `internal/api/handlers/routes.go` (solo el grupo `protegido`).
**DDL**
```sql
CREATE TABLE parametros_propuestas (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  tabla VARCHAR(60) NOT NULL COMMENT 'Tabla paramétrica destino (lista blanca en Go)',
  clave_natural VARCHAR(120) NOT NULL COMMENT 'Clave natural (p. ej. clave, concepto_salarial_id, jornada_id)',
  payload JSON NOT NULL COMMENT 'Valores propuestos',
  fecha_inicio_vigencia DATE NOT NULL COMMENT 'Desde cuándo rige',
  fuente_legal VARCHAR(150) NOT NULL COMMENT 'Base legal (obligatoria)',
  estado ENUM('pendiente','aprobada','rechazada','aplicada') NOT NULL DEFAULT 'pendiente' COMMENT 'Estado',
  propuesto_por BIGINT UNSIGNED NOT NULL COMMENT 'Quien propone', fecha_propuesta DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Propuesta',
  resuelto_por BIGINT UNSIGNED NULL COMMENT 'Quien aprueba/rechaza', fecha_resolucion DATETIME NULL COMMENT 'Resolución',
  CONSTRAINT fk_ppr_prop FOREIGN KEY (propuesto_por) REFERENCES usuarios(id),
  CONSTRAINT fk_ppr_res  FOREIGN KEY (resuelto_por)  REFERENCES usuarios(id),
  CONSTRAINT chk_ppr_cuatro_ojos CHECK (resuelto_por IS NULL OR resuelto_por <> propuesto_por),
  CONSTRAINT chk_ppr_estado CHECK ((estado = 'pendiente' AND resuelto_por IS NULL) OR (estado <> 'pendiente' AND resuelto_por IS NOT NULL)),
  KEY ix_ppr_estado (estado, tabla)
) ENGINE=InnoDB COMMENT='Cambios de parámetros legales con doble control (D21)';
```
Auditar la tabla. **Hacer:** `internal/parametros/propuestas.go` con un **registro de especificaciones por tabla** (`EspecTabla{Nombre, ColumnasClave, ColumnasValor, Validar func(payload) error}`) para: `parametros_generales`, `parametros_cargas_sociales`, `parametros_tramos_renta` (reemplazo de escala completa, validada con `ValidarTramosRenta`), `parametros_creditos_fiscales_renta`, `parametros_horas_extra`, `parametros_base_minima_contributiva`, `parametros_embargo_escala`, `parametros_subsidios_incapacidad`, `parametros_jornada`, `parametros_franja_nocturna`, `parametros_motivos_salida`, `parametros_salario_escolar`. Endpoints: `GET /parametros/:tabla` (`parametros.ver`), `POST /parametros/propuestas` (`parametros.proponer`), `POST /parametros/propuestas/:id/aprobar|rechazar` (`parametros.aprobar`; ≠ proponente). Al aprobar, **en una transacción** cierra la vigencia previa y la inserta (patrón de 2 pasos, compatible con los triggers de L2) y llama `Generales.Invalidar()`. Se puede partir en L24a (motor genérico + `parametros_generales`) y L24b (resto de especificaciones, una por commit).
**Tests:** misma persona propone y aprueba → 403 y CHECK; aprobar crea vigencia nueva sin traslape; tramos de renta con hueco → rechazada la propuesta.

## L25 ⇄ — Reportes oficiales y boleta PDF (000137)
**Entra:** `internal/planilla/export_service.go` (408 l, solo la parte de boleta), `bash scripts/ddl.sh formatos_exportacion exportaciones_generadas`, `v_planilla_resumen_periodo` (L2).
**Hacer** (los **layouts oficiales** los entrega RR. HH./Contabilidad como configuración, no se inventan):
1. Vistas SQL en 000137: `v_reporte_ccss` (por empleado: base, cuota obrera, cuota patronal desglosada por `rubro` de `parametros_cargas_sociales` a la fecha efectiva), `v_reporte_ins` (base salarial), `v_reporte_renta` (base, créditos, retención).
2. Exportador genérico CSV que toma columnas, orden y encabezados de `formatos_exportacion` (tabla ya existente); cada exportación registra fila en `exportaciones_generadas` (hash, usuario, filas, total) como ya hace la dispersión (F09). Permisos `reportes.ccss|ins|hacienda`.
3. Boleta en PDF (`go-pdf/fpdf`): mismo servicio y misma regla de fila (`ver_boleta` vs `ver_boleta_propia`), muestra devengos, deducciones, embargos, incapacidades; estados `aprobada|pagada|contabilizada`.
**Tests:** reporte de período no aprobado → 409; empleado ve solo su boleta; hash del archivo coincide con el registrado.

## L26 ⇄ — Ley 8968: cifrado, enmascarado y registro de accesos (000138)
**Entra:** `internal/services/empleado_service.go` (solo lectura/escritura de datos personales y bancarios), `bash scripts/ddl.sh empleado_datos_personales empleado_datos_bancarios`, `internal/config/config.go` (bloque PASETO, de modelo para claves).
**DDL (columnas exactas tras leer los DDL de entrada)**
```sql
-- Por cada dato sensible X (cédula, IBAN, teléfono…): X_cifrado VARBINARY(512) NULL, X_hash CHAR(64) NULL (HMAC-SHA256 para búsqueda exacta/unicidad), X_kid VARCHAR(20) NULL.
CREATE TABLE registro_accesos_datos (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'Identificador',
  usuario_id BIGINT UNSIGNED NOT NULL COMMENT 'Quien accedió', empleado_id BIGINT UNSIGNED NULL COMMENT 'Titular de los datos',
  recurso VARCHAR(60) NOT NULL COMMENT 'Endpoint/recurso', campos VARCHAR(255) NOT NULL COMMENT 'Campos sensibles revelados',
  ip VARCHAR(45) NULL COMMENT 'IP', request_id VARCHAR(64) NULL COMMENT 'Correlación con logs',
  fecha_acceso DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Cuándo',
  CONSTRAINT fk_rad_usuario FOREIGN KEY (usuario_id) REFERENCES usuarios(id),
  KEY ix_rad_emp (empleado_id, fecha_acceso), KEY ix_rad_usr (usuario_id, fecha_acceso)
) ENGINE=InnoDB COMMENT='Bitácora de accesos a datos sensibles (Ley 8968), solo inserción';
CREATE TRIGGER trg_rad_no_upd BEFORE UPDATE ON registro_accesos_datos FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'registro_inmutable: la bitácora es solo inserción';
CREATE TRIGGER trg_rad_no_del BEFORE DELETE ON registro_accesos_datos FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'registro_inmutable: la bitácora es solo inserción';
```
**Hacer:** `internal/seguridad/cifrado.go` (AES-256-GCM, llavero `DATA_KEYS_HEX="kid:hex,…"` + `DATA_ACTIVE_KID`, validado en `config.Validate()`; nonce aleatorio por valor); `Cifrar/Descifrar/HashBusqueda`; el servicio escribe cifrado + hash y lee descifrado **solo** con permiso `datos_personales.ver_sensible` (si no, enmascarado) y, al revelar, inserta en `registro_accesos_datos` (en la misma petición, con `request_id`). `cmd/jobs cifrar-existentes` migra los datos en claro por lotes (idempotente) y luego una migración posterior elimina las columnas en claro. Los triggers de auditoría **no** deben volcar valores sensibles en claro: regenerarlos sin esas columnas (o con valor `'***'`).
**Tests:** ida y vuelta; llave desconocida → error; sin permiso → enmascarado y **sin** registro de acceso de revelado; con permiso → un registro por acceso; `auditoria_cambios` no contiene cédula/IBAN.

## L27 ⇄ — CI/CD, observabilidad y backups
**Entra:** `run_full_test_suite.sh`, `docker-compose.yml`, `Makefile`, `cmd/api/main.go` (solo arranque).
**Hacer**
1. `.github/workflows/ci.yml`: `gofmt -l`, `go vet`, `go build`, `go test`, **integración** con servicio MySQL 8, `govulncheck`, `gosec` y el ciclo de migraciones `up → down → up`.
2. `Dockerfile` multi-etapa, usuario no root, sin shell; `/healthz` (vivo) y `/readyz` (BD + parámetros requeridos con `verificar-parametros`).
3. Métricas Prometheus en `/metrics` (solo red interna o con token): latencia por ruta, errores 5xx, duración del cálculo de período, nº de empleados con error.
4. `scripts/backup_mysql.sh` (`mysqldump --single-transaction --routines --triggers`, cifrado con `age`, retención) y `scripts/restore_test.sh` que restaura en BD temporal y corre `fn_detalle_cuadra` sobre el último período. `docs/operacion.md`: RPO/RTO, rotación de llaves PASETO y de datos, cierre de mes.
**Listo cuando:** la CI falla si alguien mete una migración con `ON DUPLICATE KEY UPDATE`.

## L28 — QA final y checklist de producción
**Entra:** `docs/fases/LITE.md` + resultados de la CI.
**Hacer**
1. Pruebas doradas de punta a punta (integración): período mensual con ≥10 empleados que cubren: ingreso/cese a mitad de mes, aumento, incapacidad que cruza períodos, horas extra autorizadas, 2 pensiones + 1 embargo judicial, préstamo que se salda, deducciones > remanente, devengo manual. Los números esperados se calculan **a mano en el propio test, comentados**.
2. Aguinaldo y salario escolar del año, y una liquidación, hasta contabilizar; verificar cuadre `fn_detalle_cuadra`=1, asiento cuadrado y dispersión = Σ neto.
3. Pruebas de seguridad: los 403 por rutas sin permiso (test de F12 vigente), segregación en las 4 transiciones, reutilización de refresh token, límites de tasa.
4. Checklist de producción en `docs/operacion.md`: `verificar-parametros` sin faltantes; admin inicial con contraseña cambiada; `DB_TLS_MODE=verify-full`; llaves rotadas; backup restaurado al menos una vez; todos los `PENDIENTE VALIDAR` de `parametros_*` revisados o aceptados por escrito.

---

## 6. Valores legales que NO se inventan (trabajo humano, no bloquea el desarrollo)

| Parámetro | Dónde se carga | Estado |
|---|---|---|
| `SALARIO_MINIMO_INEMBARGABLE` | migración de datos / L24 | **sin valor**: bloquea calcular con embargos |
| Divisor de jornada MIXTA y NOCTURNA | `parametros_jornada` | sin sembrar |
| Franja nocturna | `parametros_franja_nocturna` | sin sembrar |
| Escala de embargo (1/8, 1/4, múltiplo 3) | `parametros_embargo_escala` | sembrada `PENDIENTE VALIDAR` |
| Salario escolar 8.33 % vs 8.19 % | `parametros_salario_escolar` | sembrada 8.33 % |
| Rubros por motivo de salida | `parametros_motivos_salida` | sembrada conservadora |
| Escalas de cesantía/preaviso < 12 meses, ventana de vacaciones (50 semanas) | L21 | histórica del código, `PENDIENTE VALIDAR` |
| Banderas de `LIQ_*`, `AGUINALDO`, `SUB_INCAP`, `PAGO_VACACIONES` | `conceptos_salariales` | a validar |
| Cuentas contables por concepto | `mapeo_concepto_cuenta` | las entrega Contabilidad |
| Layouts CCSS/INS/Hacienda | `formatos_exportacion` | los entrega RR. HH./Contabilidad |
| `regimen_ccss_id` de cada empleado | L15 | carga humana |
| Días comerciales (D15) y base mínima prorrateada (L15) | código + test | `SUPUESTO A VALIDAR` |
