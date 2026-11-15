# Operación

## Job `acumular-vacaciones`

Acredita días de vacaciones a los empleados activos según la política
configurada. **Es seguro re-ejecutarlo**: cada acumulación es idempotente por
empleado y período (`vacaciones_movimientos.uk_vacmov_acumulacion_periodo`), así
que correrlo a diario, dos veces el mismo día o tras una falla no duplica días.

```bash
go run ./cmd/jobs acumular-vacaciones --usuario-id <ID> [--fecha YYYY-MM-DD]
```

- `--usuario-id` (obligatorio): usuario que queda como `creado_por` de los
  movimientos (auditoría). Use un usuario técnico dedicado al job.
- `--fecha`: fecha de corrida (por defecto, hoy en UTC).
- Salida: `0` todo en orden; `1` algún empleado falló (se listan); `2` uso
  incorrecto o error operativo (política sin configurar o inválida, BD).

Programación sugerida: una vez al día, por ejemplo con cron
`15 2 * * * cd /srv/planillas && ./jobs acumular-vacaciones --usuario-id 42`.
Como es idempotente, la frecuencia diaria también sirve para las políticas
anual y mensual (acredita el primer día en que corresponde).

### Política: `VACACIONES_POLITICA_ACUMULACION`

Parámetro general (`tipo = 'texto'`, con vigencia). **No se siembra**: sin valor
vigente el job sale con código 2.

| Valor | Cuándo acredita | `periodo_referencia` | Días |
|---|---|---|---|
| `aniversario` | al cumplir cada año de servicio (solo el último aniversario alcanzado) | `ANIV-n` | escala completa |
| `anio_calendario` | desde el 1 de enero, por el año anterior, si ingresó a más tardar el 1 de enero de ese año | `AAAA` | escala completa |
| `mensual` | desde el día 1, por el mes anterior, si ingresó a más tardar el día 1 de ese mes | `AAAA-MM` | escala ÷ 12 (2 decimales) |

Los días salen de `parametros_vacaciones_escala` según la antigüedad. Años o
meses incompletos no se prorratean.

## Qué paga `PAGO_VACACIONES`: `VACACIONES_MODO_PAGO`

Parámetro general (`tipo = 'texto'`, con vigencia). **No se siembra**: un
empleado con compensaciones pendientes o vacaciones aprobadas en el período no
se puede calcular hasta que exista un valor vigente (error explícito).

| Valor | Efecto en la planilla regular |
|---|---|
| `compensacion` | Paga en dinero los días registrados con `POST /vacaciones/empleados/:id/compensaciones` (días × valor del día según `VACACIONES_SALARIO_BASE`; ver [salario de vacaciones e incapacidad](#salario-de-vacaciones-e-incapacidad-vigente-o-promedio)). Los días disfrutados siguen dentro de `SAL_BASE`. |
| `disfrute` | Los días de vacaciones aprobadas dentro del período salen de `SAL_BASE` y se pagan como `PAGO_VACACIONES` (mismo neto, otra línea). Una compensación pendiente da error. |
| `ambos` | Las dos cosas. |

Una compensación queda ligada al período que la calcula y se marca pagada
(`pagado_en`) al aprobarlo; descartar o recalcular la libera.

## Contratos

```http
POST /api/v1/contratos
{"empleado_id": 42, "puesto_id": 7, "regimen_laboral_id": 1, "tipo_contrato_id": 2,
 "salario_pactado": "450000", "modalidad_salario": "quincenal", "jornada_id": 1,
 "fecha_inicio_contrato": "2026-01-05", "fecha_efectiva": "2026-01-05"}
PUT  /api/v1/empleados/{id}/contrato        # nueva versión con los mismos campos (sin empleado_id)
GET  /api/v1/contratos?tipo_contrato_id=2&es_vigente=true
```

`tipo_contrato_id` y `jornada_id` son ids de los catálogos `tipos_contrato` y `jornadas`
(`/api/v1/configuracion/catalogos`); las respuestas traen además `tipo_contrato` y
`jornada` con el código de cada catálogo, de solo lectura.

## Modalidad del salario: mensual, quincenal o semanal

Cada versión de contrato dice qué período remunera `salario_pactado`
(`modalidad_salario`, obligatorio al crear o actualizar el contrato):

| Modalidad | Días con que se devenga | `SAL_BASE` del período | Valor del día vigente | Equivalente mensual (hora ordinaria, salario mínimo) |
|---|---|---|---|---|
| `mensual` | comerciales (`DIVISOR_DIAS_MES` por mes) | salario × días ÷ `DIVISOR_DIAS_MES` | salario ÷ `DIVISOR_DIAS_MES` | el salario |
| `quincenal` | comerciales | salario × 2 × días ÷ `DIVISOR_DIAS_MES` | salario × 2 ÷ `DIVISOR_DIAS_MES` | salario × 2 |
| `semanal` | naturales | salario × días ÷ 7 | salario ÷ 7 | salario ÷ 7 × `DIVISOR_DIAS_MES` |

- Una quincena completa vale 15 días comerciales en cualquier mes (también la segunda de
  febrero), así que paga exactamente el salario quincenal; una semana completa vale 7 días
  naturales y paga exactamente el salario semanal, aunque cruce de mes.
- El salario semanal incluye el descanso semanal, como el mensual incluye los descansos del mes.
- La hora ordinaria (horas extra, feriado laborado) es el equivalente mensual ÷ horas
  mensuales de la jornada (`parametros_jornada.divisor_horas_mensuales` o el divisor propio
  del empleado).
- Incapacidad y vacaciones descuentan de `SAL_BASE` los días en la base de la modalidad
  (comerciales o naturales).
- Al crear o actualizar un contrato el salario se valida contra el salario mínimo mensual del
  puesto llevado al mes con la misma tabla (el error informa el equivalente mensual).

La modalidad no obliga a una periodicidad de planilla: se siembran los tipos `mensual`
(fracción 1), `quincenal` (0.5) y `semanal` (0.2308 = 12 ÷ 52 de mes, que prorratea la base
mínima CCSS, los embargos y las asignaciones mensuales). Un salario mensual pagado en planilla
semanal se devenga por días comerciales y suma el salario completo al cerrar el mes.

## Salario de vacaciones e incapacidad: vigente o promedio

`PAGO_VACACIONES` (compensación y disfrute) y `SUB_INCAP` valoran el día según un parámetro
general (texto, con vigencia, **no se siembra**; se exige solo al empleado que tiene qué
pagar en el período):

| Clave | Valores | Ventana del promedio |
|---|---|---|
| `VACACIONES_SALARIO_BASE` | `vigente` \| `promedio` \| `mayor` | `VENTANA_PROMEDIO_VACACIONES_SEMANAS` semanas antes del inicio del período (sembrado en 50, Código de Trabajo, art. 157; es la misma ventana de la liquidación), sobre `base_vacaciones` |
| `INCAPACIDAD_SALARIO_BASE` | `vigente` \| `promedio` \| `mayor` | `INCAPACIDAD_PROMEDIO_MESES` meses antes del inicio de cada boleta (entero, **no se siembra**, se exige con `promedio` o `mayor`), sobre `base_ccss` |

- `vigente`: el salario del contrato de cada día (valor del día de su modalidad).
- `promedio`: Σ de la base de los períodos regulares aprobados, pagados o contabilizados con
  `fecha_fin` dentro de la ventana (más los meses del histórico previo a la puesta en marcha;
  para incapacidad el histórico aporta `monto_bruto`) ÷ Σ de su `fraccion_mensual` ÷
  `DIVISOR_DIAS_MES`, igual que el promedio de la liquidación.
- `mayor`: el mayor de los dos, día por día.

Sin planillas en la ventana (empleado nuevo) se paga con el salario vigente y el detalle del
empleado lleva una advertencia. Una boleta larga usa el mismo promedio en todos los períodos
que toca, porque la ventana se mide desde su inicio. Los períodos de ingreso a mitad de mes
cuentan su fracción completa y bajan el promedio; `mayor` protege ese caso.

## Retroactivos

Un aumento (o cualquier cambio de contrato, parámetro o registro) con fecha pasada sobre
períodos ya aprobados se paga en un período regular en borrador:

```http
POST   /api/v1/planillas/{id}/retroactivos
{"empleado_id": 42, "desde": "2026-01-01", "hasta": "2026-03-31", "motivo": "Aumento por resolución 12-2026"}
GET    /api/v1/planillas/{id}/retroactivos?empleado_id=42
DELETE /api/v1/planillas/{id}/retroactivos/{retroactivo_id}
```

`hasta` debe ser anterior al inicio del período; un empleado tiene a lo sumo un retroactivo
por período (para otro rango, elimínelo y regístrelo con el rango completo). Permisos: los
de los devengos (`planilla.crear` para registrar o eliminar, `planilla.ver` para listar).

Al calcular el período, por cada período regular aprobado, pagado o contabilizado del
empleado con `fecha_fin` en `[desde, hasta]` y en el que tuvo detalle:

1. se recalcula con los datos de hoy para esas fechas (versiones de contrato, parámetros
   vigentes en esas fechas, horas extra, incapacidades, vacaciones y devengos de ese período,
   y las compensaciones de vacaciones que ese período pagó; sin préstamos ni embargos, que
   no cambian lo devengado);
2. por concepto de ingreso, la diferencia = recalculado − lo que pagó el período − lo que ya
   pagaron por él otros retroactivos;
3. si la diferencia neta es positiva, cada diferencia se agrega como línea de su concepto
   (con sus banderas de base: cotiza, grava y acumula aguinaldo en el período que la paga),
   con `retroactivo_periodo_origen_id` = período de origen, `concepto_ajuste_de_id` = línea
   corregida y `motivo_ajuste`. Si es cero o negativa no se paga y queda una advertencia:
   recuperar lo pagado de más no se hace por retroactivo.

Las cargas, la renta y los embargos del período se calculan sobre el total, retroactivo
incluido. Un retroactivo que no se puede recalcular (falta un parámetro de aquel período, el
concepto ya no aplica al régimen) deja al empleado con error `retroactivo_invalido` y el
período no se calcula hasta corregirlo o eliminar el retroactivo.

## Cargas sociales: agregadas o por componente

`parametros_cargas_sociales` tiene una fila por concepto de carga social y, opcionalmente,
por régimen CCSS (`regimen_ccss_id`; NULL = todos los regímenes sin fila propia). El
concepto dice en su fórmula si es obrero o patronal (`{"tipo":"carga_social","lado":"obrero"}`).

- **Agregado (como se siembra):** `CCSS_OBR` y `CCSS_PAT` con las tasas totales vigentes.
- **Por componente:** cree un concepto por componente (SEM, IVM, Banco Popular, INA, IMAS,
  Asignaciones Familiares, riesgos del trabajo…) con su tasa y su mapeo contable, y retire del
  régimen el concepto agregado. Mezclar ambos cuenta doble.
- **Por régimen:** una fila con `regimen_ccss_id` sustituye la tasa general para los empleados
  de ese régimen (p. ej. quien ya está pensionado y no cotiza IVM).

La contabilidad toma la cuota patronal de cada concepto con su propio mapeo.

## Impuesto al salario en planillas de fracción de mes: `RENTA_METODO_PERIODO`

El impuesto es mensual. Las planillas mensuales y de liquidación gravan el total del mes y
retienen la diferencia con lo ya retenido en otros períodos aprobados del mismo mes. Para
planillas quincenales o semanales, `RENTA_METODO_PERIODO` (texto, con vigencia, **no se
siembra**) decide:

| Valor | Efecto |
|---|---|
| `acumulado_mes` | Igual que la planilla mensual: exacto al cierre del mes (la primera quincena retiene menos, la segunda completa). |
| `proyeccion` | Cada período se grava por separado con los tramos y los créditos fiscales × la fracción del mes. |

Sin valor vigente, calcular una planilla de fracción de mes responde 422
(`parametros_incompletos`). Apruebe cada período antes de calcular el siguiente del mismo mes:
solo los aprobados cuentan como "ya retenido".

El salario mínimo inembargable, el tope mensual de cada embargo, la base mínima CCSS y los
montos mensuales asignados se prorratean por la fracción del mes (la planilla de liquidación
calcula los embargos sobre el mes completo: ver
[embargos en la liquidación](#embargos-en-la-liquidación-liq_embargo_monto_fijo)).

## Conceptos asignados por empleado

Deducciones y pluses recurrentes (asociación solidarista, sindicato, pólizas, ahorro,
anualidades, dedicación exclusiva…) se configuran sin código:

1. El concepto lleva la fórmula `{"tipo":"asignacion_empleado","base":"bruto"}` (`base`:
   `bruto`, `salario_base`, `ccss`, `renta` o `liquido`; solo cuenta para porcentajes) y se
   agrega al régimen y al tipo de planilla. `ASOC_SOLIDARISTA` y `SINDICATO` ya vienen así.
2. A cada empleado se le asigna con vigencia:

```http
GET  /api/v1/empleados/{id}/conceptos-asignados
POST /api/v1/empleados/{id}/conceptos-asignados
{"concepto_salarial_id": 14, "modo": "porcentaje", "valor": "5", "tercero_id": 3,
 "referencia": "AF-1020", "fecha_inicio_vigencia": "2026-01-01"}
POST /api/v1/empleados/conceptos-asignados/{asignacion_id}/finalizar
{"fecha_fin_vigencia": "2026-12-31"}
```

`modo = monto` es un monto **mensual** (se prorratea en quincenas; no se cobra en planillas de
liquidación). Una nueva asignación del mismo concepto cierra la anterior el día previo. Las
deducciones voluntarias se reducen si el neto no alcanza (advertencia en el cálculo).
Permisos: `empleados.leer` y `empleados.actualizar`.

Para un ingreso o una deducción de una sola vez, use un concepto `captura_manual` (ingreso o
deducción) y `POST /api/v1/planillas/{id}/devengos`.

## Salarios anteriores a la puesta en marcha

Si el sistema arranca a mitad de año, cargue los meses previos de cada empleado: completan la
base del aguinaldo y del salario escolar y los promedios de preaviso, cesantía y vacaciones.

```http
POST   /api/v1/empleados/{id}/historico-salarios
{"fuente": "Sistema anterior", "meses": [{"anio": 2025, "mes": 12, "monto_bruto": "520000",
  "base_aguinaldo": "520000", "base_cesantia": "500000", "base_vacaciones": "520000"}]}
GET    /api/v1/empleados/{id}/historico-salarios
DELETE /api/v1/empleados/{id}/historico-salarios/{mes_id}
```

Un mes que el sistema ya calculó (período regular aprobado) o ya cargado se rechaza.

## Reportes

**Oficiales** (layout configurable en `formatos_exportacion_columnas`, archivo CSV registrado
con su hash en `exportaciones_generadas`):

| Alcance | Ruta | Permiso |
|---|---|---|
| Un período | `GET /api/v1/planillas/{id}/reportes/{ccss\|ins\|hacienda}` | `reportes.ccss`, `reportes.ins`, `reportes.hacienda` |
| Un mes (todas las planillas aprobadas que terminan en él) | `GET /api/v1/reportes/mensual/{ccss\|ins\|hacienda}?anio=&mes=` | ídem |

**De gestión** (`reportes.gestion`; JSON, o CSV con `?formato=csv`; los empleados se
identifican por código interno, sin descifrar datos personales):

| Reporte | Ruta |
|---|---|
| Total por concepto e incluidas las cargas patronales | `GET /api/v1/reportes/periodos/{id}/conceptos` |
| Costo por programa o centro de costo | `GET /api/v1/reportes/periodos/{id}/costo-programas` |
| Acumulado por empleado (constancias, declaración anual) | `GET /api/v1/reportes/acumulado-empleados?anio=&mes=` |
| Embargos y préstamos retenidos por tercero | `GET /api/v1/reportes/retenciones-terceros?anio=&mes=` |
| Saldos de vacaciones | `GET /api/v1/reportes/vacaciones/saldos?fecha=` |

Sin `mes`, los reportes por rango cubren el año completo.

## Subsistema de configuración (todo desde la API)

Nada de lo que compone la planilla requiere tocar código ni la base de datos.
Hay dos mecanismos, según si el dato tiene vigencia:

| Qué | Dónde | Control |
|---|---|---|
| Parámetros legales y configuración con vigencia: `parametros_generales` (y sus valores propios por empleado), cargas sociales, tramos de renta, créditos fiscales, horas extra, base mínima CCSS, escala de embargo, tipos de embargo, subsidios de incapacidad, jornadas, franja nocturna, motivos de salida, salario escolar, salario mínimo, escalas de cesantía, preaviso y vacaciones, **fórmula de cada concepto** (general o propia de un empleado), **orden de cálculo**, **conceptos por régimen** y **mapeo contable** | `/api/v1/parametros` | Propuesta + aprobación por otra persona (`parametros.proponer` / `parametros.aprobar`) |
| Catálogos sin vigencia: conceptos salariales y sus bases, tipos de planilla y sus conceptos, cuentas contables, tipos de asiento, formatos y columnas de los reportes oficiales, bancos, entidades emisoras de incapacidades, jornadas, sindicatos, regímenes laborales y CCSS, tipos de ausencia, contrato, crédito fiscal, hora extra e identificación | `/api/v1/configuracion/catalogos` | `configuracion.administrar` |
| Feriados de ley | `/api/v1/calendario/feriados-catalogo` | `calendario.administrar` |

Todo cambio queda en `auditoria_cambios` (quién, cuándo, valor anterior y
nuevo); esa tabla es solo inserción.

### Proponer un cambio con vigencia

```http
GET  /api/v1/parametros/tablas                      # tablas admitidas
GET  /api/v1/parametros/tablas/{tabla}?fecha=AAAA-MM-DD
POST /api/v1/parametros/propuestas
{
  "tabla": "parametros_generales",
  "fecha_inicio_vigencia": "2027-01-01",
  "fuente_legal": "Decreto N.° …",
  "payload": {"filas": [{"clave": "VACACIONES_MODO_PAGO", "valor": "ambos", "tipo": "texto", "descripcion": "…"}]}
}
POST /api/v1/parametros/propuestas/{id}/aprobar     # otra persona
```

Al aprobar se cierra la vigencia anterior (día previo) y rige la nueva; el
historial nunca se modifica. Una escala (renta, embargo, cesantía, preaviso,
vacaciones, subsidios) se propone completa. Para retirar una serie sin
reemplazo (un concepto de un régimen, un orden de cálculo, un mapeo contable o
un tipo de embargo) se envía `"baja": true` con una fila que solo trae la
clave. La fórmula de un concepto se valida contra el motor: un `tipo` que el
motor no sabe calcular se rechaza al proponer.

### Cambios para todos o para un empleado

Un cambio en la forma de calcular o de pagar se hace para todos o solo para un empleado,
por el mismo circuito de propuesta y aprobación:

| Para todos | Solo para un empleado (prevalece sobre la general) |
|---|---|
| `concepto_salarial_formula` (fórmula o monto de un concepto) | `empleado_concepto_formula` (clave `empleado_id` + `concepto_salarial_id`) |
| `parametros_generales` (p. ej. `VACACIONES_MODO_PAGO`, `VACACIONES_SALARIO_BASE`, `LIQ_PRESTAMOS_MODO`, `DIVISOR_DIAS_MES`) | `empleado_parametros_generales` (clave `empleado_id` + `clave`) |
| `parametros_horas_extra` (factor por tipo de hora extra) | `empleado_parametros_horas_extra` (clave `empleado_id` + `tipo_hora_extra_id`; `factor_recargo` > 1) |
| `parametros_jornada` (divisor de la hora ordinaria por jornada) | `empleado_parametros_jornada` (clave `empleado_id` + `jornada_id`; `divisor_horas_mensuales` > 0, rige mientras el empleado esté en esa jornada) |

```http
POST /api/v1/parametros/propuestas
{
  "tabla": "empleado_parametros_generales",
  "fecha_inicio_vigencia": "2027-01-01",
  "fuente_legal": "Adenda al contrato de 2026-12-15",
  "payload": {"filas": [{"empleado_id": 42, "clave": "VACACIONES_SALARIO_BASE", "valor": "mayor", "tipo": "texto"}]}
}
POST /api/v1/parametros/propuestas
{
  "tabla": "empleado_parametros_horas_extra",
  "fecha_inicio_vigencia": "2027-01-01",
  "fuente_legal": "Convención colectiva, cláusula 18",
  "payload": {"filas": [{"empleado_id": 42, "tipo_hora_extra_id": 1, "factor_recargo": "1.75"}]}
}
POST /api/v1/parametros/propuestas
{
  "tabla": "empleado_concepto_formula",
  "fecha_inicio_vigencia": "2027-01-01",
  "fuente_legal": "Convenio de afiliación",
  "payload": {"filas": [{"empleado_id": 42, "concepto_salarial_id": 14, "monto_fijo": null,
              "expresion_formula": "{\"tipo\":\"asignacion_empleado\",\"base\":\"liquido\"}"}]}
}
```

Con `"baja": true` (y solo la clave) el empleado vuelve a la regla general desde esa fecha.
La fórmula propia debe ser del mismo `tipo_calculo` que el concepto (`monto_fijo` para un
concepto fijo, `expresion_formula` para uno de fórmula). Cada línea de planilla registra qué
fórmula usó (`concepto_salarial_formula_id` o `empleado_concepto_formula_id`) y el
`parametros_snapshot` del período guarda los parámetros propios con que se calculó a cada
empleado (`ajustes_por_empleado`: claves de `parametros_generales` y, para las tablas,
`tabla.columna[clave]`). La liquidación también lee los parámetros generales
propios del empleado (p. ej. `LIQ_CESANTIA_METODO`).

Un cambio de modalidad o de salario se hace en el contrato del empleado
(`PUT /api/v1/empleados/{id}/contrato`), que es por naturaleza individual.

### Catálogos

```http
GET    /api/v1/configuracion/catalogos              # catálogos, campos, tipos, valores admitidos
GET    /api/v1/configuracion/catalogos/{catalogo}
POST   /api/v1/configuracion/catalogos/{catalogo}   # todos los campos
PATCH  /api/v1/configuracion/catalogos/{catalogo}/{id}
DELETE /api/v1/configuracion/catalogos/{catalogo}/{id}   # solo vínculos (columnas de reporte, conceptos por tipo de planilla)
```

Los campos marcados `inmutable` (códigos, naturaleza y tipo de cálculo de un
concepto, clase de un tipo de planilla, referencias de un vínculo) no cambian
después de crearse: se crea un registro nuevo y se desactiva el anterior.

## Calendario laboral

- **Días hábiles:** `DIAS_LABORALES_SEMANA` (texto, p. ej. `1,2,3,4,5` o
  `1,2,3,4,5,6`; 1 = lunes … 7 = domingo). **No se siembra**: generar el
  calendario sin él responde 422. Afecta el conteo de días hábiles de
  vacaciones.
- **Feriados:** `feriados_catalogo` con fecha fija o desplazamiento desde el
  Domingo de Pascua, pago obligatorio o no, fuente legal y años de vigencia.
  La semilla trae los feriados de 2026 (12 feriados; 31 de agosto y 1 de diciembre de
  pago no obligatorio). Un traslado a lunes que
  ordene una ley se carga como un feriado del año con su fecha.
- `POST /api/v1/calendario/generar/{anio}` vuelve a sincronizar el año: los
  feriados retirados del catálogo dejan de serlo.
- **Tolerancia de tardanza:** propia de cada turno
  (`tolerancia_tardanza_minutos`, 0–240), obligatoria al crear o editar.
- **Feriado laborado** (concepto `FERIADO_LAB`): el salario mensual ya
  incluye el feriado; quien trabaja uno recibe además factor × sus horas
  ordinarias de ese día a la hora ordinaria de su jornada (el exceso se paga
  como hora extra "doble"). `FERIADO_LABORADO_FACTOR_OBLIGATORIO` = 1 (pago
  doble, CT art. 149). `FERIADO_LABORADO_FACTOR_NO_OBLIGATORIO` **no se
  siembra**: si alguien trabaja un feriado de pago no obligatorio, su cálculo
  falla hasta definirlo (0 = sin pago adicional).

## Liquidaciones

### Qué paga cada motivo de salida

`parametros_motivos_salida`: qué
rubros paga cada motivo. Preaviso y cesantía se pagan solo si el motivo los
admite **y** RR. HH. marca `aplica_responsabilidad_patronal`. Cambiar una regla:
cerrar la vigencia de la fila del motivo e insertar la nueva.

### Préstamos en la liquidación: `LIQ_PRESTAMOS_MODO`

Parámetro general (`tipo = 'texto'`, con vigencia). **No se siembra**: un
empleado con préstamos activos no se puede calcular en la planilla de
liquidación hasta que exista un valor vigente (error explícito del empleado).

| Valor | Qué se retiene en la planilla de liquidación |
|---|---|
| `cuota` | la cuota periódica, como en cualquier período |
| `saldo` | todo el saldo pendiente (limitado por el tope de neto) |
| `ninguno` | nada; el saldo sigue pendiente |

### Embargos en la liquidación: `LIQ_EMBARGO_MONTO_FIJO`

La planilla de liquidación calcula los embargos sobre el mes de su fecha fin: al
líquido embargable de la liquidación le suma el de los períodos aprobados del mes,
aplica el salario mínimo inembargable, la escala y los topes mensuales completos, y
retiene por cada embargo lo que falte respecto de lo ya retenido en el mes (así no se
descuenta dos veces el mínimo inembargable ni se pasa el tope mensual). Apruebe la
planilla regular del mes antes de calcular la de liquidación: solo los períodos
aprobados cuentan como "ya retenido".

Para un embargo de **monto fijo**, `LIQ_EMBARGO_MONTO_FIJO` (texto, con vigencia,
**no se siembra**; se exige solo si hay uno en la liquidación) decide:

| Valor | Qué pide la liquidación |
|---|---|
| `mensual` | el monto fijo es el del mes: solo lo que falte por retener en el mes |
| `por_periodo` | la liquidación es un período más: vuelve a pedir el monto fijo, dentro de los límites del mes |

### Cesantía: `LIQ_CESANTIA_METODO`

El Art. 29 CT se aplica de dos formas en la práctica; el sistema no elige.
**No se siembra**: una liquidación que paga cesantía falla (422) hasta que
exista un valor vigente.

| Valor | Cálculo (con el tope de años de la escala) | 5 años |
|---|---|---|
| `acumulado_por_anio` | Σ días del escalón de cada año laborado | 19,5 + 20 + 20,5 + 21 + 21,24 = 102,24 |
| `tasa_antiguedad_total` | días del escalón de la antigüedad total × años | 21,24 × 5 = 106,2 |

La escala sembrada incluye el escalón de 13 años o más (20 días).

### Job `cerrar-salidas`

Una liquidación procesada con fecha de salida futura deja al empleado activo
hasta ese día. El job cierra contrato y datos laborales a la fecha de salida y
desactiva al empleado. **Es seguro re-ejecutarlo** (`liquidaciones.cierre_aplicado_en`).

```bash
go run ./cmd/jobs cerrar-salidas --usuario-id <ID> [--fecha YYYY-MM-DD]
```

Salida: `0` en orden; `1` alguna liquidación falló (se listan); `2` uso
incorrecto o error operativo. Programación sugerida: diario, después de la
medianoche (`30 0 * * *`).

### Período mensual de liquidaciones

Procesar una liquidación crea (en borrador) el período de clase `liquidacion`
del mes de la salida, o el del mes en curso si aquel ya se aprobó. Ese período
sigue el ciclo normal: calcular → aprobar → pagar → contabilizar.

## Contabilidad

- La cuota patronal se contabiliza tal como la persistió el cálculo.
- Quien pagó un período no puede contabilizarlo (403 y trigger de BD).
- **Pagar genera el asiento de pago** (`PAGO_PLANILLA`): debe la cuenta del neto
  del devengo, haber la cuenta de bancos. La cuenta de bancos por programa es
  dato de Contabilidad (plantilla en `000136_contabilidad_pago_segregacion.up.sql`):
  sin ella, `pagar` responde 422 `mapeo_faltante`.
- Los conceptos `LIQ_*` necesitan su mapeo de gasto antes de contabilizar una
  planilla de liquidación.

## Datos personales (Ley 8968)

- **Llaves obligatorias** (la API no arranca sin ellas): `DATA_KEYS_HEX`
  (`kid:hex,kid:hex`, 32 bytes por llave, `openssl rand -hex 32`),
  `DATA_ACTIVE_KID` y `DATA_HMAC_KEY_HEX`. Guárdelas en el gestor de secretos;
  perder una llave de cifrado hace ilegibles los datos cifrados con ella.
- **Los datos personales solo existen cifrados** (cédula, dirección,
  teléfono, correo y cuenta; migración 000146). Dispersión, boleta y reportes
  los descifran en la aplicación; la unicidad de la cédula usa su hash.
- **Actualizar una base que aún tiene datos en claro:** `migrate goto 145`,
  `jobs cifrar-existentes --usuario-id <ID>`, `migrate up`. La 000146 se
  niega a correr si queda algo sin cifrar (si ya lo intentó:
  `migrate force 145` y repita los pasos).
- **Rotación de la llave de datos:** 1) agregar la nueva (`d1:…,d2:…`) y
  desplegar; 2) `DATA_ACTIVE_KID=d2`; 3) `jobs cifrar-existentes --recifrar`;
  4) verificar que ninguna fila queda con `*_kid = 'd1'`; 5) recién entonces
  retirar `d1`. `DATA_HMAC_KEY_HEX` **no** rota (cambiarla invalida los hashes
  de búsqueda).
- Revelar datos (`GET /empleados/:id/datos-sensibles`) exige
  `datos_personales.ver_sensible` y queda en `registro_accesos_datos` (solo
  inserción). Sin el permiso la respuesta viene enmascarada.


## Despliegue, probes y métricas

- Imagen: `docker build -t planillas:<versión> .` (distroless, usuario no
  root, sin shell). Binarios: `/app/api` (por defecto), `/app/jobs`,
  `/app/migrate`. Migrar antes de desplegar la API nueva:
  `docker run --rm --env-file prod.env planillas:<v> /app/migrate up`.
- Probes: `GET /healthz` = vivo (no consulta la BD: una caída de MySQL no debe
  reiniciar las réplicas); `GET /readyz` = listo (BD + parámetros requeridos;
  503 con la lista de `faltantes`). `/health` sigue disponible con latencia.
- Métricas: `GET /metrics` exige `Authorization: Bearer $METRICS_TOKEN`
  (mínimo 32 caracteres); sin token configurado responde 404. Series:
  `planillas_http_duracion_segundos{metodo,ruta,codigo}`,
  `planillas_http_errores_5xx_total`, `planillas_calculo_periodo_duracion_segundos{clase,resultado}`,
  `planillas_calculo_empleados_con_error_total{clase}` y las del runtime de Go.
  Alertas sugeridas: tasa de 5xx > 1 % en 5 min; p95 de `calcular` > 60 s;
  cualquier incremento de empleados con error en un cálculo de cierre.

## Respaldos

- `scripts/respaldar_bd.sh`: `mysqldump --single-transaction --routines
  --triggers --events --hex-blob` comprimido y **cifrado con age** a las llaves
  públicas de `AGE_RECIPIENTS_FILE`, con `.sha256` y retención
  (`RETENCION_DIAS`, por defecto 35). Exige el `mysqldump` de **MySQL 8**: el de
  MariaDB vuelca las columnas generadas y el respaldo no se puede restaurar.
- `scripts/probar_restauracion.sh <respaldo>`: restaura en una base temporal, verifica
  sha256, `schema_migrations` sin dirty, `fn_detalle_cuadra` del último período
  aprobado y que los asientos de los períodos contabilizados cuadren; luego la
  borra. Correrlo **al menos una vez al mes** y tras cada cambio de esquema.
- La llave privada de age vive fuera del servidor de BD (custodia de TI); sin
  ella los respaldos no sirven.

### RPO / RTO

| Objetivo | Valor | Cómo se cumple |
|---|---|---|
| RPO (pérdida máxima) | 24 h; 15 min en cierre de mes | respaldo diario (`0 2 * * *`) + binlog con retención ≥ 7 días para recuperación a un punto en el tiempo; respaldo adicional antes de aprobar/pagar el cierre |
| RTO (tiempo de recuperación) | 4 h | restaurar el último respaldo (`probar_restauracion.sh` mide el tiempo real), aplicar binlog, `migrate version`, `/readyz` |

## Rotación de llaves

- **PASETO** (tokens): agregar `k2` a `PASETO_LOCAL_KEYS_HEX` y desplegar;
  `PASETO_ACTIVE_KID=k2`; tras la vigencia del refresh token (7 días) retirar `k1`.
- **Datos personales**: ver la sección "Datos personales"; nunca retirar
  una llave con datos todavía cifrados con ella.
- **age** (respaldos): agregar la llave pública nueva a `AGE_RECIPIENTS_FILE`;
  conservar la privada vieja mientras existan respaldos cifrados con ella
  (35 días con la retención por defecto).
- **METRICS_TOKEN**: cambiar en el servidor y en Prometheus a la vez.

## Cierre de mes (orden sugerido)

1. `jobs verificar-parametros --fecha <fin de mes>` sin faltantes.
2. Procesar las liquidaciones del mes; `jobs cerrar-salidas` diario.
3. Planilla regular: calcular → revisar errores por empleado → aprobar (otra
   persona) → dispersión bancaria → pagar (genera el asiento de pago) →
   contabilizar (otra persona distinta de quien pagó).
4. Planilla de liquidaciones del mes: mismo ciclo.
5. Reportes oficiales del mes (`/reportes/mensual/{ccss|ins|hacienda}?anio=&mes=`),
   retenciones por tercero para girar embargos y préstamos, y respaldo adicional
   (`respaldar_bd.sh`) con su `probar_restauracion.sh`.

## Checklist de producción

Marcar cada punto con fecha y responsable antes del primer pago real.

**Parámetros y datos legales**
- [ ] `jobs verificar-parametros --fecha <hoy>` sale con 0 (sin faltantes); `GET /readyz` responde 200.
- [ ] `SALARIO_MINIMO_INEMBARGABLE` cargado con monto, fecha y fuente legal confirmados.
- [ ] Configuración que el sistema exige y no siembra: `VACACIONES_MODO_PAGO`, `VACACIONES_POLITICA_ACUMULACION`, `LIQ_PRESTAMOS_MODO`, `LIQ_EMBARGO_MONTO_FIJO` (si hay embargos de monto fijo), `LIQ_CESANTIA_METODO`, `VACACIONES_SALARIO_BASE`, `INCAPACIDAD_SALARIO_BASE` (e `INCAPACIDAD_PROMEDIO_MESES` si es `promedio` o `mayor`), `DIAS_LABORALES_SEMANA`, `FERIADO_LABORADO_FACTOR_NO_OBLIGATORIO`, `RENTA_METODO_PERIODO` (si hay planillas quincenales o semanales); divisores de jornada MIXTA/NOCTURNA y `parametros_franja_nocturna` (si hay horas extra nocturnas o mixtas).
- [ ] Cargas sociales revisadas para el tipo de empleador (sector público o privado) y, si hay pensionados u otros regímenes, filas por `regimen_ccss_id`; tasa de la póliza de riesgos del trabajo (`INS_RT`) si se contabiliza.
- [ ] Conceptos asignados de cada empleado (asociación, sindicato, pólizas) y, si se arranca a mitad de año, histórico de salarios cargado.
- [ ] Valores iniciales de `parametros_*` revisados por RR. HH./Legal (`GET /api/v1/parametros/tablas/{tabla}`), incluidos `parametros_motivos_salida`, las escalas de cesantía/preaviso y `VENTANA_PROMEDIO_VACACIONES_SEMANAS`.
- [ ] Modalidad de salario de cada contrato revisada (`mensual`, `quincenal`, `semanal`) y, si se migran contratos, todos con `tipo_contrato_id` y `jornada_id` del catálogo antes de la migración 000153.
- [ ] Criterios de cálculo revisados: días comerciales, base mínima CCSS prorrateada, banderas de LIQ_*/AGUINALDO/SUB_INCAP/PAGO_VACACIONES, renta y embargo en la planilla de liquidación.
- [ ] Catálogo de feriados revisado (`GET /calendario/feriados-catalogo`) y calendario del año generado; tolerancia de tardanza definida en cada turno.
- [ ] `regimen_ccss_id` cargado a cada empleado (`GET /empleados?sin_regimen_ccss=true` vacío).
- [ ] Cuentas contables: `mapeo_concepto_cuenta` completo por programa (incluye bancos de `PAGO_PLANILLA` y `LIQ_*`); layouts oficiales en `formatos_exportacion_columnas` (CCSS, INS, Hacienda).

**Seguridad**
- [ ] Administrador inicial con contraseña cambiada (`cmd/seedadmin` deja contraseña temporal).
- [ ] `NOMBRE_SISTEMA`, `SMTP_FROM` y `URL_BASE_RESET` con los datos de la organización.
- [ ] `DB_TLS_MODE=verify-full` con la CA del servidor MySQL.
- [ ] Llaves PASETO, de datos (`DATA_*`) y `METRICS_TOKEN` generadas para producción (no las de ejemplo), en el gestor de secretos, con su procedimiento de rotación probado.
- [ ] Migración 000146 aplicada (sin columnas de datos personales en claro).
- [ ] Usuarios asignados por rol respetando la segregación (quien prepara ≠ aprueba ≠ paga ≠ contabiliza; quien propone parámetros ≠ quien aprueba).

**Operación**
- [ ] Respaldo diario programado y **restaurado al menos una vez** con `scripts/probar_restauracion.sh` (anotar el tiempo real frente al RTO).
- [ ] Jobs programados: `acumular-vacaciones` y `cerrar-salidas` (diarios).
- [ ] CI en verde en la rama de despliegue (`.github/workflows/ci.yml`), incluido govulncheck con el último parche de Go.
- [ ] Prometheus raspando `/metrics` con el token y alertas configuradas.

