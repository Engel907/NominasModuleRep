package parametros

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/go-sql-driver/mysql"

	auditdb "planillas-ccdr/internal/db"
	"planillas-ccdr/internal/db/dto"
)

// TipoColumna: cómo se valida y normaliza el valor de una columna.
type TipoColumna int

const (
	ColTexto TipoColumna = iota
	ColDecimal
	ColEntero
	ColID // entero > 0 (FK)
	ColFecha
	ColHora
	ColBool
)

// Columna de una tabla paramétrica.
type Columna struct {
	Nombre string
	Tipo   TipoColumna
	Nula   bool
	Max    int // largo máximo (texto)
}

// Fila: valores normalizados por columna (nil = NULL).
type Fila map[string]*string

// EspecTabla describe una tabla paramétrica que se puede cambiar por propuesta.
type EspecTabla struct {
	Nombre string
	// ColumnasClave identifican la serie: se cierran sus filas vigentes. Vacío
	// = la tabla entera es una sola serie (escala única o fila única).
	ColumnasClave []Columna
	ColumnasValor []Columna
	// Escala: la propuesta trae todas las filas de la serie (reemplazo completo).
	Escala bool
	// PermiteBaja: una propuesta puede cerrar la serie sin reemplazarla.
	PermiteBaja bool
	// La tabla tiene fuente_legal / creado_por (se llenan desde la propuesta).
	FuenteLegal, CreadoPor bool
	Validar                func(filas []Fila) error
}

func (e EspecTabla) columnas() []Columna {
	return append(append([]Columna{}, e.ColumnasClave...), e.ColumnasValor...)
}

// Errores de propuestas (el handler los traduce).
var (
	ErrTablaNoSoportada      = errors.New("parametros: tabla paramétrica no soportada")
	ErrPropuestaInvalida     = errors.New("parametros: propuesta inválida")
	ErrPropuestaNoEncontrada = errors.New("parametros: la propuesta no existe")
	ErrPropuestaResuelta     = errors.New("parametros: la propuesta ya fue resuelta")
	ErrCuatroOjos            = errors.New("parametros: quien propone no puede aprobar ni rechazar su propia propuesta")
	ErrVigenciaNoPosterior   = errors.New("parametros: la vigencia propuesta debe iniciar después de la vigencia actual de la serie")
)

var reNombre = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Validadores que viven en paquetes que importan parametros (se inyectan).
type Validadores struct {
	Renta   func([]dto.TramoRentaVigenteRow) error
	Formula func(expresion string) error
}

// RegistroEspecs: especificaciones soportadas.
func RegistroEspecs(v Validadores) map[string]EspecTabla {
	validarRenta := v.Renta
	txt := func(n string, max int) Columna { return Columna{Nombre: n, Tipo: ColTexto, Max: max} }
	dec := func(n string) Columna { return Columna{Nombre: n, Tipo: ColDecimal} }
	decN := func(n string) Columna { return Columna{Nombre: n, Tipo: ColDecimal, Nula: true} }
	id := func(n string) Columna { return Columna{Nombre: n, Tipo: ColID} }
	idN := func(n string) Columna { return Columna{Nombre: n, Tipo: ColID, Nula: true} }
	b := func(n string) Columna { return Columna{Nombre: n, Tipo: ColBool} }
	especs := []EspecTabla{
		{Nombre: "parametros_generales", ColumnasClave: []Columna{txt("clave", 80)},
			ColumnasValor: []Columna{txt("valor", 255), txt("tipo", 20), txt("descripcion", 255)},
			FuenteLegal:   true, CreadoPor: true, Validar: validarGeneral},
		{Nombre: "parametros_cargas_sociales", ColumnasClave: []Columna{id("concepto_salarial_id"), idN("regimen_ccss_id")},
			ColumnasValor: []Columna{txt("rubro", 60), dec("porcentaje_patronal"), dec("porcentaje_obrero")},
			FuenteLegal:   true, Validar: cadaFila(func(f Fila) error {
				return enRango(f, 0, 100, "porcentaje_patronal", "porcentaje_obrero")
			})},
		{Nombre: "parametros_tramos_renta", Escala: true,
			ColumnasValor: []Columna{{Nombre: "orden_tramo", Tipo: ColEntero}, dec("monto_desde"), decN("monto_hasta"), dec("porcentaje")},
			Validar:       validarRentaCon(validarRenta)},
		{Nombre: "parametros_creditos_fiscales_renta", ColumnasClave: []Columna{id("tipo_credito_fiscal_id")},
			ColumnasValor: []Columna{dec("monto_credito")}, FuenteLegal: true,
			Validar: cadaFila(func(f Fila) error { return minimo(f, "monto_credito", 0, true) })},
		{Nombre: "parametros_horas_extra", ColumnasClave: []Columna{id("tipo_hora_extra_id")},
			ColumnasValor: []Columna{dec("factor_recargo")},
			Validar:       cadaFila(func(f Fila) error { return minimo(f, "factor_recargo", 1, false) })},
		{Nombre: "parametros_base_minima_contributiva", ColumnasClave: []Columna{id("regimen_ccss_id")},
			ColumnasValor: []Columna{dec("monto")},
			Validar:       cadaFila(func(f Fila) error { return minimo(f, "monto", 0, false) })},
		{Nombre: "parametros_embargo_escala", Escala: true,
			ColumnasValor: []Columna{dec("desde_multiplo"), decN("hasta_multiplo"), dec("porcentaje_maximo")},
			FuenteLegal:   true, CreadoPor: true, Validar: validarEscalaEmbargo},
		{Nombre: "parametros_subsidios_incapacidad", Escala: true, ColumnasClave: []Columna{id("entidad_emisora_id")},
			ColumnasValor: []Columna{{Nombre: "dia_desde", Tipo: ColEntero}, {Nombre: "dia_hasta", Tipo: ColEntero, Nula: true},
				dec("porcentaje_patronal"), decN("porcentaje_entidad")},
			Validar: validarSubsidios},
		{Nombre: "parametros_jornada", ColumnasClave: []Columna{id("jornada_id")},
			ColumnasValor: []Columna{decN("horas_diarias_ordinarias"), dec("divisor_horas_mensuales")},
			FuenteLegal:   true, CreadoPor: true, Validar: cadaFila(func(f Fila) error {
				if err := minimo(f, "divisor_horas_mensuales", 0, false); err != nil {
					return err
				}
				if f["horas_diarias_ordinarias"] == nil {
					return nil
				}
				return enRango(f, 0, 24, "horas_diarias_ordinarias")
			})},
		{Nombre: "parametros_franja_nocturna",
			ColumnasValor: []Columna{{Nombre: "hora_inicio", Tipo: ColHora}, {Nombre: "hora_fin", Tipo: ColHora}},
			FuenteLegal:   true, CreadoPor: true, Validar: cadaFila(func(f Fila) error {
				if *f["hora_inicio"] == *f["hora_fin"] {
					return errors.New("hora_inicio y hora_fin no pueden ser iguales")
				}
				return nil
			})},
		{Nombre: "parametros_motivos_salida", ColumnasClave: []Columna{txt("motivo_salida", 40)},
			ColumnasValor: []Columna{b("paga_preaviso"), b("paga_cesantia"), b("paga_vacaciones"), b("paga_aguinaldo"),
				b("paga_salario_escolar"), {Nombre: "observacion", Tipo: ColTexto, Nula: true, Max: 255}},
			FuenteLegal: true, CreadoPor: true, Validar: cadaFila(validarMotivo)},
		{Nombre: "parametros_salario_escolar", ColumnasValor: []Columna{dec("porcentaje")}, CreadoPor: true,
			Validar: cadaFila(func(f Fila) error {
				if err := minimo(f, "porcentaje", 0, false); err != nil {
					return err
				}
				return enRango(f, 0, 1, "porcentaje")
			})},
		{Nombre: "parametros_cesantia_escala", Escala: true, ColumnasClave: []Columna{id("regimen_laboral_id")},
			ColumnasValor: []Columna{dec("anios_servicio_desde"), decN("anios_servicio_hasta"), dec("dias_por_anio"), decN("tope_anios")},
			FuenteLegal:   true, CreadoPor: true, Validar: validarEscalaCesantia},
		{Nombre: "parametros_preaviso_escala", Escala: true, ColumnasClave: []Columna{id("regimen_laboral_id")},
			ColumnasValor: []Columna{{Nombre: "antiguedad_meses_desde", Tipo: ColEntero}, {Nombre: "antiguedad_meses_hasta", Tipo: ColEntero, Nula: true},
				{Nombre: "dias_preaviso", Tipo: ColEntero}},
			FuenteLegal: true, CreadoPor: true, Validar: func(fs []Fila) error {
				return validarTramosContiguos(fs, "antiguedad_meses_desde", "antiguedad_meses_hasta", "preaviso")
			}},
		{Nombre: "parametros_vacaciones_escala", Escala: true,
			ColumnasValor: []Columna{dec("anios_servicio_desde"), decN("anios_servicio_hasta"), dec("dias_vacaciones")},
			Validar: func(fs []Fila) error {
				if err := cadaFila(func(f Fila) error { return minimo(f, "dias_vacaciones", 0, false) })(fs); err != nil {
					return err
				}
				return validarTramosContiguos(fs, "anios_servicio_desde", "anios_servicio_hasta", "vacaciones")
			}},
		{Nombre: "parametros_salario_minimo", ColumnasClave: []Columna{id("clasificacion_puesto_id")},
			ColumnasValor: []Columna{dec("monto_mensual")}, FuenteLegal: true,
			Validar: cadaFila(func(f Fila) error { return minimo(f, "monto_mensual", 0, false) })},
		{Nombre: "parametros_tipos_embargo", ColumnasClave: []Columna{txt("codigo", 20)},
			ColumnasValor: []Columna{txt("nombre", 100), {Nombre: "prioridad_legal", Tipo: ColEntero}, b("es_pension_alimentaria"),
				decN("tope_porcentaje_liquido")},
			PermiteBaja: true, Validar: cadaFila(validarTipoEmbargo)},
		{Nombre: "concepto_salarial_formula", ColumnasClave: []Columna{id("concepto_salarial_id")},
			ColumnasValor: []Columna{decN("monto_fijo"), {Nombre: "expresion_formula", Tipo: ColTexto, Nula: true, Max: 2000}},
			Validar:       cadaFila(validarFormulaCon(v.Formula))},
		// Fórmula o monto de un concepto propio de un empleado: prevalece sobre
		// concepto_salarial_formula; la baja lo devuelve a la general.
		{Nombre: "empleado_concepto_formula", ColumnasClave: []Columna{id("empleado_id"), id("concepto_salarial_id")},
			ColumnasValor: []Columna{decN("monto_fijo"), {Nombre: "expresion_formula", Tipo: ColTexto, Nula: true, Max: 2000}},
			PermiteBaja:   true, FuenteLegal: true, CreadoPor: true,
			Validar: cadaFila(validarFormulaCon(v.Formula))},
		// Valor de un parámetro general propio de un empleado: prevalece sobre
		// parametros_generales; la baja lo devuelve al general.
		{Nombre: "empleado_parametros_generales", ColumnasClave: []Columna{id("empleado_id"), txt("clave", 80)},
			ColumnasValor: []Columna{txt("valor", 255), txt("tipo", 20)},
			PermiteBaja:   true, FuenteLegal: true, CreadoPor: true, Validar: validarGeneral},
		{Nombre: "orden_calculo_concepto", ColumnasClave: []Columna{id("concepto_salarial_id")},
			ColumnasValor: []Columna{{Nombre: "orden", Tipo: ColEntero}}, PermiteBaja: true,
			Validar: cadaFila(func(f Fila) error { return minimo(f, "orden", 0, false) })},
		{Nombre: "regimen_concepto_aplicable", ColumnasClave: []Columna{id("regimen_laboral_id"), id("concepto_salarial_id")},
			PermiteBaja: true},
		{Nombre: "mapeo_concepto_cuenta",
			ColumnasClave: []Columna{id("concepto_salarial_id"), id("programa_presupuestario_id"), id("tipo_asiento_id")},
			ColumnasValor: []Columna{id("cuenta_contable_id"), txt("tipo_movimiento", 5), txt("proyecto", 20)},
			PermiteBaja:   true, Validar: cadaFila(func(f Fila) error {
				if m := *f["tipo_movimiento"]; m != "debe" && m != "haber" {
					return fmt.Errorf("tipo_movimiento %q inválido (debe | haber)", m)
				}
				return nil
			})},
	}
	out := make(map[string]EspecTabla, len(especs))
	for _, e := range especs {
		for _, c := range e.columnas() {
			if !reNombre.MatchString(c.Nombre) {
				panic("parametros: nombre de columna inválido en la especificación: " + c.Nombre)
			}
		}
		out[e.Nombre] = e
	}
	return out
}

// payloadPropuesta: {"filas":[{"col":"valor"|número|booleano|null, ...}]};
// con "baja": true trae una sola fila con las columnas clave de la serie que
// se cierra.
type payloadPropuesta struct {
	Filas []map[string]any `json:"filas"`
	Baja  bool             `json:"baja,omitempty"`
}

// Cambio es una propuesta ya validada contra su especificación.
type Cambio struct {
	Filas []Fila
	Clave string
	Baja  bool
}

// ParsearPayload valida el JSON contra la especificación y devuelve las filas
// normalizadas, la clave natural de la serie y si es una baja.
func ParsearPayload(e EspecTabla, raw []byte) (Cambio, error) {
	jd := json.NewDecoder(bytes.NewReader(raw))
	jd.UseNumber()
	jd.DisallowUnknownFields()
	var p payloadPropuesta
	if err := jd.Decode(&p); err != nil {
		return Cambio{}, fmt.Errorf("%w: payload: %v", ErrPropuestaInvalida, err)
	}
	columnas := e.columnas()
	switch {
	case p.Baja && !e.PermiteBaja:
		return Cambio{}, fmt.Errorf("%w: %s no admite bajas; proponga un reemplazo", ErrPropuestaInvalida, e.Nombre)
	case p.Baja && len(p.Filas) != 1:
		return Cambio{}, fmt.Errorf("%w: una baja trae exactamente una fila con la clave de la serie", ErrPropuestaInvalida)
	case p.Baja:
		columnas = e.ColumnasClave
	case len(p.Filas) == 0 || (!e.Escala && len(p.Filas) != 1):
		return Cambio{}, fmt.Errorf("%w: %s espera %s", ErrPropuestaInvalida, e.Nombre, map[bool]string{true: "una o más filas (escala completa)", false: "exactamente una fila"}[e.Escala])
	}
	permitidas := map[string]Columna{}
	for _, c := range columnas {
		permitidas[c.Nombre] = c
	}
	filas := make([]Fila, 0, len(p.Filas))
	for i, crudo := range p.Filas {
		f := Fila{}
		for k := range crudo {
			if _, ok := permitidas[k]; !ok {
				return Cambio{}, fmt.Errorf("%w: fila %d: columna %q no permitida en %s", ErrPropuestaInvalida, i+1, k, e.Nombre)
			}
		}
		for _, c := range columnas {
			v, err := NormalizarValor(c, crudo[c.Nombre])
			if err != nil {
				return Cambio{}, fmt.Errorf("%w: fila %d, %s: %v", ErrPropuestaInvalida, i+1, c.Nombre, err)
			}
			f[c.Nombre] = v
		}
		filas = append(filas, f)
	}
	clave := claveNatural(e, filas[0])
	for i, f := range filas[1:] {
		if claveNatural(e, f) != clave {
			return Cambio{}, fmt.Errorf("%w: fila %d pertenece a otra serie (%s ≠ %s)", ErrPropuestaInvalida, i+2, claveNatural(e, f), clave)
		}
	}
	if e.Validar != nil && !p.Baja {
		if err := e.Validar(filas); err != nil {
			return Cambio{}, fmt.Errorf("%w: %v", ErrPropuestaInvalida, err)
		}
	}
	return Cambio{Filas: filas, Clave: clave, Baja: p.Baja}, nil
}

func claveNatural(e EspecTabla, f Fila) string {
	if len(e.ColumnasClave) == 0 {
		return "*"
	}
	partes := make([]string, 0, len(e.ColumnasClave))
	for _, c := range e.ColumnasClave {
		v := "NULL"
		if f[c.Nombre] != nil {
			v = *f[c.Nombre]
		}
		partes = append(partes, c.Nombre+"="+v)
	}
	return strings.Join(partes, ",")
}

// NormalizarValor valida un valor JSON contra la columna y lo devuelve como
// texto canónico (nil = NULL).
func NormalizarValor(c Columna, v any) (*string, error) {
	if v == nil {
		if c.Nula {
			return nil, nil
		}
		return nil, errors.New("es obligatorio")
	}
	var s string
	switch x := v.(type) {
	case string:
		s = strings.TrimSpace(x)
	case json.Number:
		s = x.String()
	case bool:
		if c.Tipo != ColBool {
			return nil, errors.New("tipo inválido")
		}
		s = map[bool]string{true: "1", false: "0"}[x]
	default:
		return nil, errors.New("tipo inválido (se espera texto, número, booleano o null)")
	}
	switch c.Tipo {
	case ColTexto:
		if s == "" && !c.Nula {
			return nil, errors.New("es obligatorio")
		}
		if c.Max > 0 && len([]rune(s)) > c.Max {
			return nil, fmt.Errorf("excede %d caracteres", c.Max)
		}
	case ColDecimal:
		d, err := decimal.NewFromString(s)
		if err != nil {
			return nil, errors.New("decimal inválido")
		}
		s = d.String()
	case ColEntero, ColID:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil || (c.Tipo == ColID && n <= 0) || n < 0 {
			return nil, errors.New("entero inválido")
		}
		s = strconv.FormatInt(n, 10)
	case ColFecha:
		if _, err := time.Parse(formatoFecha, s); err != nil {
			return nil, errors.New("fecha inválida (YYYY-MM-DD)")
		}
	case ColHora:
		t, err := time.Parse("15:04:05", s)
		if err != nil {
			if t, err = time.Parse("15:04", s); err != nil {
				return nil, errors.New("hora inválida (HH:MM o HH:MM:SS)")
			}
		}
		s = t.Format("15:04:05")
	case ColBool:
		bv, err := strconv.ParseBool(s)
		if err != nil {
			return nil, errors.New("booleano inválido")
		}
		s = map[bool]string{true: "1", false: "0"}[bv]
	}
	return &s, nil
}

func cadaFila(fn func(Fila) error) func([]Fila) error {
	return func(fs []Fila) error {
		for _, f := range fs {
			if err := fn(f); err != nil {
				return err
			}
		}
		return nil
	}
}

func decimalDe(f Fila, col string) decimal.Decimal {
	return decimal.RequireFromString(*f[col])
}

func enRango(f Fila, min, max int64, cols ...string) error {
	for _, c := range cols {
		v := decimalDe(f, c)
		if v.LessThan(decimal.NewFromInt(min)) || v.GreaterThan(decimal.NewFromInt(max)) {
			return fmt.Errorf("%s debe estar entre %d y %d (es %s)", c, min, max, v)
		}
	}
	return nil
}

func minimo(f Fila, col string, min int64, inclusivo bool) error {
	v, m := decimalDe(f, col), decimal.NewFromInt(min)
	if v.LessThan(m) || (!inclusivo && v.Equal(m)) {
		op := ">"
		if inclusivo {
			op = ">="
		}
		return fmt.Errorf("%s debe ser %s %d (es %s)", col, op, min, v)
	}
	return nil
}

var reClave = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,79}$`)

// validarGeneral: clave en mayúsculas, tipo del ENUM y valor interpretable
// con el mismo criterio que los accesores de Generales.
func validarGeneral(fs []Fila) error {
	f := fs[0]
	if !reClave.MatchString(*f["clave"]) {
		return fmt.Errorf("clave %q inválida (MAYÚSCULAS, dígitos y _)", *f["clave"])
	}
	v := *f["valor"]
	switch *f["tipo"] {
	case TipoEntero:
		if _, err := strconv.ParseInt(v, 10, 64); err != nil {
			return fmt.Errorf("valor %q no es entero", v)
		}
	case TipoDecimal:
		if _, err := decimal.NewFromString(v); err != nil {
			return fmt.Errorf("valor %q no es decimal", v)
		}
	case TipoPorcentaje:
		d, err := decimal.NewFromString(v)
		if err != nil || d.IsNegative() || d.GreaterThan(decimal.NewFromInt(100)) {
			return fmt.Errorf("valor %q no es un porcentaje entre 0 y 100", v)
		}
	case TipoTexto:
	case TipoFecha:
		if _, err := time.Parse(formatoFecha, v); err != nil {
			return fmt.Errorf("valor %q no es fecha YYYY-MM-DD", v)
		}
	case TipoBooleano:
		if _, err := strconv.ParseBool(v); err != nil {
			return fmt.Errorf("valor %q no es booleano", v)
		}
	default:
		return fmt.Errorf("tipo %q inválido", *f["tipo"])
	}
	return nil
}

func validarRentaCon(validar func([]dto.TramoRentaVigenteRow) error) func([]Fila) error {
	return func(fs []Fila) error {
		rows := make([]dto.TramoRentaVigenteRow, 0, len(fs))
		for _, f := range fs {
			n, _ := strconv.ParseInt(*f["orden_tramo"], 10, 32)
			r := dto.TramoRentaVigenteRow{OrdenTramo: int32(n), MontoDesde: decimalDe(f, "monto_desde"), Porcentaje: decimalDe(f, "porcentaje")}
			if f["monto_hasta"] != nil {
				r.MontoHasta = decimal.NewNullDecimal(decimalDe(f, "monto_hasta"))
			}
			if err := enRango(f, 0, 100, "porcentaje"); err != nil {
				return err
			}
			rows = append(rows, r)
		}
		if validar == nil {
			return errors.New("validador de tramos de renta no configurado")
		}
		return validar(rows)
	}
}

// validarEscalaEmbargo: múltiplos contiguos desde 0, solo el último abierto,
// porcentaje en (0, 100] (mismo criterio que la escala del motor).
func validarEscalaEmbargo(fs []Fila) error {
	s := append([]Fila(nil), fs...)
	sort.Slice(s, func(i, j int) bool {
		return decimalDe(s[i], "desde_multiplo").LessThan(decimalDe(s[j], "desde_multiplo"))
	})
	if !decimalDe(s[0], "desde_multiplo").IsZero() {
		return errors.New("la escala de embargo debe iniciar en el múltiplo 0")
	}
	for i, f := range s {
		if err := minimo(f, "porcentaje_maximo", 0, false); err != nil {
			return err
		}
		if err := enRango(f, 0, 100, "porcentaje_maximo"); err != nil {
			return err
		}
		ultimo := i == len(s)-1
		if f["hasta_multiplo"] == nil {
			if !ultimo {
				return errors.New("solo el último tramo de la escala de embargo puede ser abierto")
			}
			continue
		}
		if ultimo {
			return errors.New("el último tramo de la escala de embargo debe ser abierto")
		}
		if !decimalDe(f, "hasta_multiplo").GreaterThan(decimalDe(f, "desde_multiplo")) {
			return errors.New("tramo de embargo con hasta <= desde")
		}
		if !decimalDe(s[i+1], "desde_multiplo").Equal(decimalDe(f, "hasta_multiplo")) {
			return fmt.Errorf("hueco o solape en la escala de embargo en el múltiplo %s", *f["hasta_multiplo"])
		}
	}
	return nil
}

// validarSubsidios: tramos de días contiguos desde el día 1 (dia_hasta
// inclusivo), solo el último abierto; porcentajes 0..100.
func validarSubsidios(fs []Fila) error {
	n := func(f Fila, c string) int64 { x, _ := strconv.ParseInt(*f[c], 10, 64); return x }
	s := append([]Fila(nil), fs...)
	sort.Slice(s, func(i, j int) bool { return n(s[i], "dia_desde") < n(s[j], "dia_desde") })
	if n(s[0], "dia_desde") != 1 {
		return errors.New("los tramos de subsidio deben iniciar en el día 1")
	}
	for i, f := range s {
		if err := enRango(f, 0, 100, "porcentaje_patronal"); err != nil {
			return err
		}
		if f["porcentaje_entidad"] != nil {
			if err := enRango(f, 0, 100, "porcentaje_entidad"); err != nil {
				return err
			}
		}
		ultimo := i == len(s)-1
		if f["dia_hasta"] == nil {
			if !ultimo {
				return errors.New("solo el último tramo de subsidio puede ser abierto")
			}
			continue
		}
		if n(f, "dia_hasta") < n(f, "dia_desde") {
			return errors.New("tramo de subsidio con dia_hasta < dia_desde")
		}
		if !ultimo && n(s[i+1], "dia_desde") != n(f, "dia_hasta")+1 {
			return fmt.Errorf("hueco o solape en los tramos de subsidio después del día %d", n(f, "dia_hasta"))
		}
	}
	return nil
}

// validarTramosContiguos: tramos [desde, hasta) contiguos desde 0, con solo
// el último abierto.
func validarTramosContiguos(fs []Fila, desde, hasta, nombre string) error {
	s := append([]Fila(nil), fs...)
	sort.Slice(s, func(i, j int) bool { return decimalDe(s[i], desde).LessThan(decimalDe(s[j], desde)) })
	if !decimalDe(s[0], desde).IsZero() {
		return fmt.Errorf("la escala de %s debe iniciar en 0", nombre)
	}
	for i, f := range s {
		ultimo := i == len(s)-1
		if f[hasta] == nil {
			if !ultimo {
				return fmt.Errorf("solo el último tramo de la escala de %s puede ser abierto", nombre)
			}
			continue
		}
		if ultimo {
			return fmt.Errorf("el último tramo de la escala de %s debe ser abierto", nombre)
		}
		if !decimalDe(f, hasta).GreaterThan(decimalDe(f, desde)) {
			return fmt.Errorf("tramo de %s con hasta <= desde", nombre)
		}
		if !decimalDe(s[i+1], desde).Equal(decimalDe(f, hasta)) {
			return fmt.Errorf("hueco o solape en la escala de %s en %s", nombre, *f[hasta])
		}
	}
	return nil
}

// validarEscalaCesantia: tramos contiguos desde 0 años, días >= 0 y un mismo
// tope de años en toda la escala.
func validarEscalaCesantia(fs []Fila) error {
	tope := fs[0]["tope_anios"]
	for _, f := range fs {
		if err := minimo(f, "dias_por_anio", 0, true); err != nil {
			return err
		}
		if (f["tope_anios"] == nil) != (tope == nil) || (tope != nil && !decimalDe(f, "tope_anios").Equal(decimalDe(fs[0], "tope_anios"))) {
			return errors.New("tope_anios debe ser el mismo en toda la escala de cesantía")
		}
	}
	if tope != nil {
		if err := minimo(fs[0], "tope_anios", 0, false); err != nil {
			return err
		}
	}
	return validarTramosContiguos(fs, "anios_servicio_desde", "anios_servicio_hasta", "cesantía")
}

func validarTipoEmbargo(f Fila) error {
	if !reClave.MatchString(*f["codigo"]) {
		return fmt.Errorf("codigo %q inválido (MAYÚSCULAS, dígitos y _)", *f["codigo"])
	}
	if err := minimo(f, "prioridad_legal", 0, false); err != nil {
		return err
	}
	if f["tope_porcentaje_liquido"] != nil {
		return enRango(f, 0, 100, "tope_porcentaje_liquido")
	}
	return nil
}

// validarFormulaCon: exactamente uno de monto_fijo o expresion_formula; la
// expresión debe poder ejecutarla el motor.
func validarFormulaCon(validar func(string) error) func(Fila) error {
	return func(f Fila) error {
		fijo, expresion := f["monto_fijo"], f["expresion_formula"]
		switch {
		case (fijo == nil) == (expresion == nil):
			return errors.New("indique monto_fijo o expresion_formula, no ambos ni ninguno")
		case fijo != nil:
			return minimo(f, "monto_fijo", 0, true)
		case validar == nil:
			return errors.New("validador de fórmulas no configurado")
		default:
			return validar(*expresion)
		}
	}
}

var motivosSalida = map[string]bool{
	"despido_sin_responsabilidad": true, "despido_con_responsabilidad": true, "despido_indirecto": true,
	"renuncia": true, "vencimiento_contrato": true, "mutuo_acuerdo": true, "fallecimiento": true,
}

func validarMotivo(f Fila) error {
	if !motivosSalida[*f["motivo_salida"]] {
		return fmt.Errorf("motivo_salida %q inválido", *f["motivo_salida"])
	}
	return nil
}

// Propuesta: fila de parametros_propuestas.
type Propuesta struct {
	ID                  int64           `json:"id"`
	Tabla               string          `json:"tabla"`
	ClaveNatural        string          `json:"clave_natural"`
	Payload             json.RawMessage `json:"payload"`
	FechaInicioVigencia string          `json:"fecha_inicio_vigencia"`
	FuenteLegal         string          `json:"fuente_legal"`
	Estado              string          `json:"estado"`
	PropuestoPor        int64           `json:"propuesto_por"`
	FechaPropuesta      time.Time       `json:"fecha_propuesta"`
	ResueltoPor         *int64          `json:"resuelto_por,omitempty"`
	FechaResolucion     *time.Time      `json:"fecha_resolucion,omitempty"`
	MotivoRechazo       *string         `json:"motivo_rechazo,omitempty"`
}

// Estados de parametros_propuestas.estado.
const (
	EstadoPropuestaPendiente = "pendiente"
	EstadoPropuestaRechazada = "rechazada"
	EstadoPropuestaAplicada  = "aplicada"
)

// Propuestas: servicio de propuesta/aprobación.
type Propuestas struct {
	db        *sql.DB
	especs    map[string]EspecTabla
	invalidar func()
}

// NewPropuestas: invalidar típicamente (*Generales).Invalidar.
func NewPropuestas(db *sql.DB, especs map[string]EspecTabla, invalidar func()) *Propuestas {
	return &Propuestas{db: db, especs: especs, invalidar: invalidar}
}

// Tablas: nombres soportados, ordenados.
func (s *Propuestas) Tablas() []string {
	out := make([]string, 0, len(s.especs))
	for n := range s.especs {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func (s *Propuestas) espec(tabla string) (EspecTabla, error) {
	e, ok := s.especs[tabla]
	if !ok {
		return EspecTabla{}, fmt.Errorf("%w: %q", ErrTablaNoSoportada, tabla)
	}
	return e, nil
}

// NuevaPropuesta: datos de entrada de Proponer.
type NuevaPropuesta struct {
	Tabla               string
	Payload             json.RawMessage
	FechaInicioVigencia time.Time
	FuenteLegal         string
}

// Proponer valida y registra la propuesta en 'pendiente'.
func (s *Propuestas) Proponer(ctx context.Context, usuarioID int64, n NuevaPropuesta) (Propuesta, error) {
	e, err := s.espec(n.Tabla)
	if err != nil {
		return Propuesta{}, err
	}
	fuente := strings.TrimSpace(n.FuenteLegal)
	if fuente == "" || len([]rune(fuente)) > 150 {
		return Propuesta{}, fmt.Errorf("%w: fuente_legal es obligatoria (máximo 150 caracteres)", ErrPropuestaInvalida)
	}
	if n.FechaInicioVigencia.IsZero() {
		return Propuesta{}, fmt.Errorf("%w: fecha_inicio_vigencia es obligatoria", ErrPropuestaInvalida)
	}
	cambio, err := ParsearPayload(e, n.Payload)
	if err != nil {
		return Propuesta{}, err
	}
	normal, err := json.Marshal(payloadNormalizado{Filas: cambio.Filas, Baja: cambio.Baja})
	if err != nil {
		return Propuesta{}, err
	}
	var id int64
	err = auditdb.WithAuditContext(ctx, s.db, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		id, err = q.InsertarPropuestaParametro(ctx, n.Tabla, cambio.Clave, normal, soloFecha(n.FechaInicioVigencia), fuente, usuarioID)
		return err
	})
	if err != nil {
		return Propuesta{}, err
	}
	return s.Obtener(ctx, id)
}

type payloadNormalizado struct {
	Filas []Fila `json:"filas"`
	Baja  bool   `json:"baja,omitempty"`
}

// Obtener una propuesta.
func (s *Propuestas) Obtener(ctx context.Context, id int64) (Propuesta, error) {
	r, err := dto.New(s.db).ObtenerPropuestaParametro(ctx, id, false)
	if errors.Is(err, sql.ErrNoRows) {
		return Propuesta{}, ErrPropuestaNoEncontrada
	}
	return desdeFila(r), err
}

// Listar propuestas (filtros opcionales).
func (s *Propuestas) Listar(ctx context.Context, estado, tabla string, limit, offset int32) ([]Propuesta, int64, error) {
	q := dto.New(s.db)
	total, err := q.ContarPropuestasParametro(ctx, estado, tabla)
	if err != nil {
		return nil, 0, err
	}
	rows, err := q.ListarPropuestasParametro(ctx, estado, tabla, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	out := make([]Propuesta, 0, len(rows))
	for _, r := range rows {
		out = append(out, desdeFila(r))
	}
	return out, total, nil
}

func desdeFila(r dto.PropuestaParametroRow) Propuesta {
	p := Propuesta{ID: r.ID, Tabla: r.Tabla, ClaveNatural: r.ClaveNatural, Payload: json.RawMessage(r.Payload),
		FechaInicioVigencia: r.FechaInicioVigencia.Format(formatoFecha), FuenteLegal: r.FuenteLegal, Estado: r.Estado,
		PropuestoPor: r.PropuestoPor, FechaPropuesta: r.FechaPropuesta, MotivoRechazo: r.MotivoRechazo}
	if r.ResueltoPor.Valid {
		v := r.ResueltoPor.Int64
		p.ResueltoPor = &v
	}
	if r.FechaResolucion.Valid {
		v := r.FechaResolucion.Time
		p.FechaResolucion = &v
	}
	return p
}

// Aprobar aplica la propuesta: cierra la vigencia previa de la serie e inserta
// las filas nuevas, todo en una transacción auditada; luego invalida la caché.
func (s *Propuestas) Aprobar(ctx context.Context, usuarioID, id int64) (Propuesta, error) {
	err := auditdb.WithAuditContext(ctx, s.db, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		r, err := s.bloquearPendiente(ctx, q, usuarioID, id)
		if err != nil {
			return err
		}
		e, err := s.espec(r.Tabla)
		if err != nil {
			return err
		}
		cambio, err := ParsearPayload(e, r.Payload)
		if err != nil {
			return err
		}
		if cambio.Clave != r.ClaveNatural {
			return fmt.Errorf("%w: la clave natural no coincide con el payload", ErrPropuestaInvalida)
		}
		desde := soloFecha(r.FechaInicioVigencia)
		if err := traducirErrorBD(aplicar(ctx, q, e, cambio, desde, r.FuenteLegal, usuarioID)); err != nil {
			return err
		}
		return q.ResolverPropuestaParametro(ctx, id, EstadoPropuestaAplicada, usuarioID, nil)
	})
	if err != nil {
		return Propuesta{}, err
	}
	if s.invalidar != nil {
		s.invalidar()
	}
	return s.Obtener(ctx, id)
}

// Rechazar: motivo obligatorio; quien propone no rechaza (cuatro ojos).
func (s *Propuestas) Rechazar(ctx context.Context, usuarioID, id int64, motivo string) (Propuesta, error) {
	motivo = strings.TrimSpace(motivo)
	if motivo == "" || len([]rune(motivo)) > 255 {
		return Propuesta{}, fmt.Errorf("%w: el motivo del rechazo es obligatorio (máximo 255 caracteres)", ErrPropuestaInvalida)
	}
	err := auditdb.WithAuditContext(ctx, s.db, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		if _, err := s.bloquearPendiente(ctx, q, usuarioID, id); err != nil {
			return err
		}
		return q.ResolverPropuestaParametro(ctx, id, EstadoPropuestaRechazada, usuarioID, &motivo)
	})
	if err != nil {
		return Propuesta{}, err
	}
	return s.Obtener(ctx, id)
}

func (s *Propuestas) bloquearPendiente(ctx context.Context, q *dto.Queries, usuarioID, id int64) (dto.PropuestaParametroRow, error) {
	r, err := q.ObtenerPropuestaParametro(ctx, id, true)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrPropuestaNoEncontrada
	}
	if err != nil {
		return r, err
	}
	if r.Estado != EstadoPropuestaPendiente {
		return r, fmt.Errorf("%w (estado %s)", ErrPropuestaResuelta, r.Estado)
	}
	if r.PropuestoPor == usuarioID {
		return r, ErrCuatroOjos
	}
	return r, nil
}

// ErrSerieSinVigente: una baja sobre una serie sin vigencia abierta.
var ErrSerieSinVigente = errors.New("parametros: la serie no tiene una vigencia abierta que dar de baja")

// traducirErrorBD convierte los rechazos de integridad de MySQL en una
// propuesta inválida (referencia inexistente, duplicado, CHECK o trigger).
func traducirErrorBD(err error) error {
	var me *mysql.MySQLError
	if !errors.As(err, &me) {
		return err
	}
	switch me.Number {
	case 1452:
		return fmt.Errorf("%w: hace referencia a un registro que no existe", ErrPropuestaInvalida)
	case 1062:
		return fmt.Errorf("%w: ya existe una fila vigente para esa serie", ErrPropuestaInvalida)
	case 3819:
		return fmt.Errorf("%w: un valor no cumple las reglas de la tabla", ErrPropuestaInvalida)
	case 1644:
		return fmt.Errorf("%w: %s", ErrPropuestaInvalida, me.Message)
	}
	return err
}

// aplicar: patrón de vigencia en 2 pasos sobre la serie de la especificación.
func aplicar(ctx context.Context, q *dto.Queries, e EspecTabla, cambio Cambio, desde time.Time, fuente string, usuarioID int64) error {
	where, args := filtroSerie(e, cambio.Filas[0])
	n, err := q.ContarFilasParametro(ctx, e.Nombre, where+" AND fecha_inicio_vigencia >= ?", append(args, desde)...)
	if err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("%w (%s, desde %s)", ErrVigenciaNoPosterior, e.Nombre, desde.Format(formatoFecha))
	}
	if cambio.Baja {
		abiertas, err := q.ContarFilasParametro(ctx, e.Nombre, where+" AND fecha_fin_vigencia IS NULL", args...)
		if err != nil {
			return err
		}
		if abiertas == 0 {
			return ErrSerieSinVigente
		}
	}
	if err := q.CerrarVigenciaParametro(ctx, e.Nombre, where, desde.AddDate(0, 0, -1), args...); err != nil {
		return fmt.Errorf("parametros: cerrando vigencia de %s: %w", e.Nombre, err)
	}
	if cambio.Baja {
		return nil
	}
	filas := cambio.Filas
	cols := make([]string, 0, len(e.columnas())+3)
	for _, c := range e.columnas() {
		cols = append(cols, c.Nombre)
	}
	cols = append(cols, "fecha_inicio_vigencia")
	if e.FuenteLegal {
		cols = append(cols, "fuente_legal")
	}
	if e.CreadoPor {
		cols = append(cols, "creado_por")
	}
	for _, f := range filas {
		vals := make([]any, 0, len(cols))
		for _, c := range e.columnas() {
			if f[c.Nombre] == nil {
				vals = append(vals, nil)
			} else {
				vals = append(vals, *f[c.Nombre])
			}
		}
		vals = append(vals, desde)
		if e.FuenteLegal {
			vals = append(vals, fuente)
		}
		if e.CreadoPor {
			vals = append(vals, usuarioID)
		}
		if err := q.InsertarFilaParametro(ctx, e.Nombre, cols, vals); err != nil {
			return fmt.Errorf("parametros: insertando en %s: %w", e.Nombre, err)
		}
	}
	return nil
}

// filtroSerie: "1 = 1 AND col <=> ? ..." con las columnas clave (nombres de la
// lista blanca, valores como argumentos; una clave nula identifica la serie
// general).
func filtroSerie(e EspecTabla, f Fila) (string, []any) {
	w := "1 = 1"
	var args []any
	for _, c := range e.ColumnasClave {
		w += " AND " + c.Nombre + " <=> ?"
		if v := f[c.Nombre]; v != nil {
			args = append(args, *v)
		} else {
			args = append(args, nil)
		}
	}
	return w, args
}

// Vigentes: filas de la tabla vigentes a `fecha` (valores como texto).
func (s *Propuestas) Vigentes(ctx context.Context, tabla string, fecha time.Time) ([]map[string]*string, error) {
	e, err := s.espec(tabla)
	if err != nil {
		return nil, err
	}
	cols := []string{"id"}
	for _, c := range e.columnas() {
		cols = append(cols, c.Nombre)
	}
	cols = append(cols, "fecha_inicio_vigencia", "fecha_fin_vigencia")
	if e.FuenteLegal {
		cols = append(cols, "fuente_legal")
	}
	return dto.New(s.db).ListarFilasParametroVigentes(ctx, e.Nombre, cols, soloFecha(fecha))
}
