// Package catalogos expone los catálogos no versionados que componen la
// planilla (conceptos, tipos de planilla, cuentas, layouts de reportes,
// entidades, jornadas, regímenes…) para que se administren desde la API.
package catalogos

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/parametros"
)

// Campo de un catálogo. Valores, si no está vacío, es la lista de valores
// admitidos (ENUM). Inmutable: se fija al crear y no cambia después.
type Campo struct {
	parametros.Columna
	Valores   []string
	Inmutable bool
}

// Fila: valores normalizados por campo (nil = NULL).
type Fila = parametros.Fila

// Catalogo describe una tabla administrable.
type Catalogo struct {
	Nombre        string
	Descripcion   string
	Campos        []Campo
	PermiteBorrar bool
	Validar       func(Fila) error
}

func (c Catalogo) campo(nombre string) (Campo, bool) {
	for _, f := range c.Campos {
		if f.Nombre == nombre {
			return f, true
		}
	}
	return Campo{}, false
}

func (c Catalogo) columnas() []string {
	cols := make([]string, 0, len(c.Campos)+1)
	cols = append(cols, "id")
	for _, f := range c.Campos {
		cols = append(cols, f.Nombre)
	}
	return cols
}

var (
	reNombre = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	reCodigo = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
)

func codigo(max int) Campo {
	return Campo{Columna: parametros.Columna{Nombre: "codigo", Tipo: parametros.ColTexto, Max: max}, Inmutable: true}
}

func texto(nombre string, max int) Campo {
	return Campo{Columna: parametros.Columna{Nombre: nombre, Tipo: parametros.ColTexto, Max: max}}
}

func textoNulo(nombre string, max int) Campo {
	return Campo{Columna: parametros.Columna{Nombre: nombre, Tipo: parametros.ColTexto, Max: max, Nula: true}}
}

func booleano(nombre string) Campo {
	return Campo{Columna: parametros.Columna{Nombre: nombre, Tipo: parametros.ColBool}}
}

func entero(nombre string) Campo {
	return Campo{Columna: parametros.Columna{Nombre: nombre, Tipo: parametros.ColEntero}}
}

func referencia(nombre string, nula bool) Campo {
	return Campo{Columna: parametros.Columna{Nombre: nombre, Tipo: parametros.ColID, Nula: nula}}
}

func enumeracion(nombre string, nula bool, valores ...string) Campo {
	return Campo{Columna: parametros.Columna{Nombre: nombre, Tipo: parametros.ColTexto, Nula: nula, Max: 40}, Valores: valores}
}

func inmutable(c Campo) Campo {
	c.Inmutable = true
	return c
}

func codigoMayusculas(f Fila) error {
	if v := f["codigo"]; v != nil && !reCodigo.MatchString(*v) {
		return fmt.Errorf("codigo %q inválido (MAYÚSCULAS, dígitos y _)", *v)
	}
	return nil
}

func positivo(nombre string) func(Fila) error {
	return func(f Fila) error {
		if v := f[nombre]; v != nil {
			if d, err := decimal.NewFromString(*v); err != nil || !d.IsPositive() {
				return fmt.Errorf("%s debe ser mayor que 0", nombre)
			}
		}
		return nil
	}
}

func todas(reglas ...func(Fila) error) func(Fila) error {
	return func(f Fila) error {
		for _, r := range reglas {
			if err := r(f); err != nil {
				return err
			}
		}
		return nil
	}
}

func validarTipoPlanilla(f Fila) error {
	fraccion := f["fraccion_mensual"]
	if f["clase"] != nil && *f["clase"] == "regular" && fraccion == nil {
		return errors.New("un tipo de planilla regular requiere fraccion_mensual")
	}
	if fraccion != nil {
		d, err := decimal.NewFromString(*fraccion)
		if err != nil || !d.IsPositive() || d.GreaterThan(decimal.NewFromInt(1)) {
			return errors.New("fraccion_mensual debe estar en (0, 1]")
		}
	}
	return nil
}

func columnaDeReporte(f Fila) error {
	if v := f["columna_origen"]; v != nil && !reNombre.MatchString(*v) {
		return fmt.Errorf("columna_origen %q inválida (minúsculas, dígitos y _)", *v)
	}
	if v := f["columna_total"]; v != nil && !reNombre.MatchString(*v) {
		return fmt.Errorf("columna_total %q inválida (minúsculas, dígitos y _)", *v)
	}
	return nil
}

// Registro devuelve la lista blanca de catálogos administrables.
func Registro() map[string]Catalogo {
	catalogos := []Catalogo{
		{Nombre: "conceptos_salariales", Descripcion: "Conceptos que componen la planilla (ingresos y deducciones) y las bases a las que afectan",
			Campos: []Campo{codigo(20), texto("nombre", 150),
				inmutable(enumeracion("naturaleza", false, "ingreso", "deduccion")),
				inmutable(enumeracion("tipo_calculo", false, "fijo", "formula", "captura_manual")),
				booleano("afecta_base_ccss"), booleano("afecta_base_renta"), booleano("afecta_base_aguinaldo"),
				booleano("afecta_base_cesantia"), booleano("afecta_base_vacaciones"), booleano("afecta_base_embargo"),
				booleano("es_deduccion_obligatoria"), booleano("activo")},
			Validar: codigoMayusculas},
		{Nombre: "tipos_planilla", Descripcion: "Tipos de planilla (mensual, quincenal, semanal, aguinaldo…) y su fracción del mes",
			Campos: []Campo{codigo(20), texto("nombre", 100),
				inmutable(enumeracion("clase", false, "regular", "aguinaldo", "salario_escolar", "liquidacion")),
				{Columna: parametros.Columna{Nombre: "fraccion_mensual", Tipo: parametros.ColDecimal, Nula: true}}},
			Validar: todas(codigoMayusculas, validarTipoPlanilla)},
		{Nombre: "tipo_planilla_concepto", Descripcion: "Conceptos que se calculan en cada tipo de planilla",
			Campos:        []Campo{inmutable(referencia("tipo_planilla_id", false)), inmutable(referencia("concepto_salarial_id", false))},
			PermiteBorrar: true},
		{Nombre: "cuentas_contables", Descripcion: "Catálogo de cuentas contables",
			Campos: []Campo{codigo(40), texto("nombre", 200), referencia("cuenta_padre_id", true), entero("nivel"),
				enumeracion("naturaleza", false, "activo", "pasivo", "gasto", "ingreso", "patrimonio"), booleano("activo")},
			Validar: positivo("nivel")},
		{Nombre: "tipos_asiento", Descripcion: "Tipos de asiento contable",
			Campos:  []Campo{codigo(40), texto("nombre", 150), textoNulo("descripcion", 255)},
			Validar: codigoMayusculas},
		{Nombre: "formatos_exportacion", Descripcion: "Formatos de archivos de salida (banco, CCSS, INS, Hacienda)",
			Campos: []Campo{codigo(30), texto("nombre", 100), enumeracion("tipo_destino", false, "banco", "autoridad", "interno"),
				enumeracion("reporte_oficial", true, "ccss", "ins", "hacienda"), textoNulo("columna_total", 64)},
			Validar: todas(codigoMayusculas, columnaDeReporte)},
		{Nombre: "formatos_exportacion_columnas", Descripcion: "Columnas, orden y encabezados de cada formato de salida",
			Campos: []Campo{inmutable(referencia("formato_exportacion_id", false)), entero("orden"),
				texto("columna_origen", 64), texto("encabezado", 100)},
			PermiteBorrar: true, Validar: todas(positivo("orden"), columnaDeReporte)},
		{Nombre: "bancos", Descripcion: "Bancos depositarios de planilla y otorgantes de préstamos",
			Campos: []Campo{codigo(20), texto("nombre", 150), booleano("es_depositario_nomina"),
				booleano("es_otorgante_prestamos"), booleano("activo")},
			Validar: codigoMayusculas},
		{Nombre: "entidades_emisoras_incapacidad", Descripcion: "Entidades que emiten incapacidades (CCSS, INS, aseguradoras)",
			Campos: []Campo{codigo(20), texto("nombre", 100), booleano("activo")}, Validar: codigoMayusculas},
		{Nombre: "jornadas", Descripcion: "Jornadas de trabajo y su clase legal (diurna, mixta, nocturna)",
			Campos: []Campo{codigo(20), texto("nombre", 100), inmutable(enumeracion("clase", false, "diurna", "mixta", "nocturna")),
				booleano("activo")},
			Validar: codigoMayusculas},
		{Nombre: "sindicatos", Descripcion: "Sindicatos a los que se afilian los empleados",
			Campos: []Campo{codigo(20), texto("nombre", 150)}, Validar: codigoMayusculas},
		{Nombre: "regimenes_ccss", Descripcion: "Regímenes de cotización a la CCSS",
			Campos: []Campo{codigo(20), texto("nombre", 100), booleano("activo")}, Validar: codigoMayusculas},
		{Nombre: "regimenes_laborales", Descripcion: "Regímenes laborales (Código de Trabajo, estatutos)",
			Campos:  []Campo{codigo(40), texto("nombre", 150), textoNulo("descripcion", 255), booleano("activo")},
			Validar: codigoMayusculas},
		{Nombre: "tipos_ausencia", Descripcion: "Tipos de ausencia",
			Campos:  []Campo{codigo(20), texto("nombre", 100), booleano("requiere_documento_soporte")},
			Validar: codigoMayusculas},
		{Nombre: "tipos_contrato", Descripcion: "Tipos de contrato laboral",
			Campos:  []Campo{codigo(20), texto("nombre", 100), textoNulo("descripcion", 255)},
			Validar: codigoMayusculas},
		{Nombre: "tipos_credito_fiscal_renta", Descripcion: "Créditos fiscales del impuesto al salario",
			Campos: []Campo{codigo(20), texto("nombre", 100), booleano("activo")}, Validar: codigoMayusculas},
		{Nombre: "tipos_hora_extra", Descripcion: "Tipos de hora extra y su clase",
			Campos: []Campo{codigo(20), texto("nombre", 100), booleano("activo"),
				enumeracion("clase", true, "diurna", "nocturna", "mixta", "doble")},
			Validar: codigoMayusculas},
		{Nombre: "tipos_identificacion", Descripcion: "Tipos de identificación de personas",
			Campos: []Campo{codigo(20), texto("nombre", 100), booleano("activo")}, Validar: codigoMayusculas},
	}
	out := make(map[string]Catalogo, len(catalogos))
	for _, c := range catalogos {
		if !reNombre.MatchString(c.Nombre) {
			panic("configuracion: nombre de catálogo inválido: " + c.Nombre)
		}
		for _, f := range c.Campos {
			if !reNombre.MatchString(f.Nombre) || f.Nombre == "id" {
				panic("configuracion: campo inválido en " + c.Nombre + ": " + f.Nombre)
			}
		}
		out[c.Nombre] = c
	}
	return out
}

// Nombres de los catálogos, ordenados.
func Nombres(r map[string]Catalogo) []string {
	nombres := make([]string, 0, len(r))
	for n := range r {
		nombres = append(nombres, n)
	}
	sort.Strings(nombres)
	return nombres
}

func validarEnumeracion(c Campo, v *string) error {
	if v == nil || len(c.Valores) == 0 || slices.Contains(c.Valores, *v) {
		return nil
	}
	return fmt.Errorf("%s %q inválido (%v)", c.Nombre, *v, c.Valores)
}
