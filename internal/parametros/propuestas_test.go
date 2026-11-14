package parametros

import (
	"errors"
	"testing"

	"planillas-ccdr/internal/db/dto"
)

func especs() map[string]EspecTabla {
	return RegistroEspecs(Validadores{
		Renta: func(rows []dto.TramoRentaVigenteRow) error {
			for i := 1; i < len(rows); i++ {
				if !rows[i-1].MontoHasta.Valid || !rows[i].MontoDesde.Equal(rows[i-1].MontoHasta.Decimal) {
					return errors.New("hueco")
				}
			}
			return nil
		},
		Formula: func(expresion string) error {
			if expresion != `{"tipo":"renta"}` {
				return errors.New("fórmula no admitida")
			}
			return nil
		},
	})
}

func TestRegistroCubreLasTablasVersionadas(t *testing.T) {
	want := []string{
		"parametros_generales", "parametros_cargas_sociales", "parametros_tramos_renta",
		"parametros_creditos_fiscales_renta", "parametros_horas_extra", "parametros_base_minima_contributiva",
		"parametros_embargo_escala", "parametros_subsidios_incapacidad", "parametros_jornada",
		"parametros_franja_nocturna", "parametros_motivos_salida", "parametros_salario_escolar",
		"parametros_cesantia_escala", "parametros_preaviso_escala", "parametros_vacaciones_escala",
		"parametros_salario_minimo", "parametros_tipos_embargo",
		"concepto_salarial_formula", "orden_calculo_concepto", "regimen_concepto_aplicable", "mapeo_concepto_cuenta",
		"empleado_concepto_formula", "empleado_parametros_generales",
	}
	e := especs()
	if len(e) != len(want) {
		t.Fatalf("%d especificaciones, se esperaban %d", len(e), len(want))
	}
	for _, n := range want {
		if _, ok := e[n]; !ok {
			t.Errorf("falta %s", n)
		}
	}
}

func TestParsearPayload(t *testing.T) {
	e := especs()
	casos := []struct {
		nombre, tabla, payload string
		ok                     bool
		clave                  string
	}{
		{"general entero", "parametros_generales", `{"filas":[{"clave":"DIVISOR_DIAS_MES","valor":"30","tipo":"entero","descripcion":"d"}]}`, true, "clave=DIVISOR_DIAS_MES"},
		{"general valor no entero", "parametros_generales", `{"filas":[{"clave":"DIVISOR_DIAS_MES","valor":"x","tipo":"entero","descripcion":"d"}]}`, false, ""},
		{"general clave minúscula", "parametros_generales", `{"filas":[{"clave":"divisor","valor":"30","tipo":"entero","descripcion":"d"}]}`, false, ""},
		{"columna desconocida", "parametros_jornada", `{"filas":[{"jornada_id":1,"divisor_horas_mensuales":"240","hack":"1"}]}`, false, ""},
		{"jornada con número JSON", "parametros_jornada", `{"filas":[{"jornada_id":2,"horas_diarias_ordinarias":null,"divisor_horas_mensuales":210}]}`, true, "jornada_id=2"},
		{"jornada divisor cero", "parametros_jornada", `{"filas":[{"jornada_id":2,"horas_diarias_ordinarias":null,"divisor_horas_mensuales":"0"}]}`, false, ""},
		{"dos filas en tabla no escala", "parametros_horas_extra", `{"filas":[{"tipo_hora_extra_id":1,"factor_recargo":"1.5"},{"tipo_hora_extra_id":1,"factor_recargo":"1.5"}]}`, false, ""},
		{"renta contigua", "parametros_tramos_renta", `{"filas":[{"orden_tramo":1,"monto_desde":"0","monto_hasta":"100","porcentaje":"0"},{"orden_tramo":2,"monto_desde":"100","monto_hasta":null,"porcentaje":"10"}]}`, true, "*"},
		{"renta con hueco", "parametros_tramos_renta", `{"filas":[{"orden_tramo":1,"monto_desde":"0","monto_hasta":"100","porcentaje":"0"},{"orden_tramo":2,"monto_desde":"150","monto_hasta":null,"porcentaje":"10"}]}`, false, ""},
		{"embargo sin 0", "parametros_embargo_escala", `{"filas":[{"desde_multiplo":"1","hasta_multiplo":null,"porcentaje_maximo":"25"}]}`, false, ""},
		{"embargo ok", "parametros_embargo_escala", `{"filas":[{"desde_multiplo":"0","hasta_multiplo":"3","porcentaje_maximo":"12.5"},{"desde_multiplo":"3","hasta_multiplo":null,"porcentaje_maximo":"25"}]}`, true, "*"},
		{"subsidios otra entidad en la serie", "parametros_subsidios_incapacidad", `{"filas":[{"entidad_emisora_id":1,"dia_desde":1,"dia_hasta":3,"porcentaje_patronal":"50","porcentaje_entidad":null},{"entidad_emisora_id":2,"dia_desde":4,"dia_hasta":null,"porcentaje_patronal":"0","porcentaje_entidad":"60"}]}`, false, ""},
		{"subsidios ok", "parametros_subsidios_incapacidad", `{"filas":[{"entidad_emisora_id":1,"dia_desde":1,"dia_hasta":3,"porcentaje_patronal":"50","porcentaje_entidad":null},{"entidad_emisora_id":1,"dia_desde":4,"dia_hasta":null,"porcentaje_patronal":"0","porcentaje_entidad":"60"}]}`, true, "entidad_emisora_id=1"},
		{"franja iguales", "parametros_franja_nocturna", `{"filas":[{"hora_inicio":"19:00","hora_fin":"19:00:00"}]}`, false, ""},
		{"motivo inválido", "parametros_motivos_salida", `{"filas":[{"motivo_salida":"jubilacion","paga_preaviso":false,"paga_cesantia":false,"paga_vacaciones":true,"paga_aguinaldo":true,"paga_salario_escolar":true,"observacion":null}]}`, false, ""},
		{"salario escolar en puntos", "parametros_salario_escolar", `{"filas":[{"porcentaje":"8.33"}]}`, false, ""},
		{"salario escolar fracción", "parametros_salario_escolar", `{"filas":[{"porcentaje":"0.0819"}]}`, true, "*"},
		{"fórmula admitida", "concepto_salarial_formula", `{"filas":[{"concepto_salarial_id":11,"monto_fijo":null,"expresion_formula":"{\"tipo\":\"renta\"}"}]}`, true, "concepto_salarial_id=11"},
		{"fórmula rechazada por el motor", "concepto_salarial_formula", `{"filas":[{"concepto_salarial_id":11,"monto_fijo":null,"expresion_formula":"{\"tipo\":\"magia\"}"}]}`, false, ""},
		{"monto fijo", "concepto_salarial_formula", `{"filas":[{"concepto_salarial_id":3,"monto_fijo":"15000","expresion_formula":null}]}`, true, "concepto_salarial_id=3"},
		{"fijo y fórmula a la vez", "concepto_salarial_formula", `{"filas":[{"concepto_salarial_id":3,"monto_fijo":"1","expresion_formula":"{\"tipo\":\"renta\"}"}]}`, false, ""},
		{"fórmula sin baja", "concepto_salarial_formula", `{"baja":true,"filas":[{"concepto_salarial_id":3}]}`, false, ""},
		{"orden cero", "orden_calculo_concepto", `{"filas":[{"concepto_salarial_id":3,"orden":0}]}`, false, ""},
		{"régimen aplicable", "regimen_concepto_aplicable", `{"filas":[{"regimen_laboral_id":1,"concepto_salarial_id":3}]}`, true, "regimen_laboral_id=1,concepto_salarial_id=3"},
		{"baja de régimen", "regimen_concepto_aplicable", `{"baja":true,"filas":[{"regimen_laboral_id":1,"concepto_salarial_id":3}]}`, true, "regimen_laboral_id=1,concepto_salarial_id=3"},
		{"baja con columnas de valor", "orden_calculo_concepto", `{"baja":true,"filas":[{"concepto_salarial_id":3,"orden":5}]}`, false, ""},
		{"baja de dos filas", "regimen_concepto_aplicable", `{"baja":true,"filas":[{"regimen_laboral_id":1,"concepto_salarial_id":3},{"regimen_laboral_id":1,"concepto_salarial_id":4}]}`, false, ""},
		{"mapeo movimiento inválido", "mapeo_concepto_cuenta", `{"filas":[{"concepto_salarial_id":1,"programa_presupuestario_id":1,"tipo_asiento_id":1,"cuenta_contable_id":2,"tipo_movimiento":"credito","proyecto":"P1"}]}`, false, ""},
		{"preaviso Art. 28", "parametros_preaviso_escala", `{"filas":[{"regimen_laboral_id":1,"antiguedad_meses_desde":0,"antiguedad_meses_hasta":3,"dias_preaviso":0},{"regimen_laboral_id":1,"antiguedad_meses_desde":3,"antiguedad_meses_hasta":6,"dias_preaviso":7},{"regimen_laboral_id":1,"antiguedad_meses_desde":6,"antiguedad_meses_hasta":12,"dias_preaviso":15},{"regimen_laboral_id":1,"antiguedad_meses_desde":12,"antiguedad_meses_hasta":null,"dias_preaviso":30}]}`, true, "regimen_laboral_id=1"},
		{"preaviso con hueco", "parametros_preaviso_escala", `{"filas":[{"regimen_laboral_id":1,"antiguedad_meses_desde":0,"antiguedad_meses_hasta":3,"dias_preaviso":0},{"regimen_laboral_id":1,"antiguedad_meses_desde":4,"antiguedad_meses_hasta":null,"dias_preaviso":7}]}`, false, ""},
		{"cesantía topes distintos", "parametros_cesantia_escala", `{"filas":[{"regimen_laboral_id":1,"anios_servicio_desde":"0","anios_servicio_hasta":"1","dias_por_anio":"14","tope_anios":"8"},{"regimen_laboral_id":1,"anios_servicio_desde":"1","anios_servicio_hasta":null,"dias_por_anio":"19.5","tope_anios":"6"}]}`, false, ""},
		{"cesantía ok", "parametros_cesantia_escala", `{"filas":[{"regimen_laboral_id":1,"anios_servicio_desde":"0","anios_servicio_hasta":"1","dias_por_anio":"14","tope_anios":"8"},{"regimen_laboral_id":1,"anios_servicio_desde":"1","anios_servicio_hasta":null,"dias_por_anio":"19.5","tope_anios":"8"}]}`, true, "regimen_laboral_id=1"},
		{"vacaciones municipales", "parametros_vacaciones_escala", `{"filas":[{"anios_servicio_desde":"0","anios_servicio_hasta":"5","dias_vacaciones":"15"},{"anios_servicio_desde":"5","anios_servicio_hasta":null,"dias_vacaciones":"26"}]}`, true, "*"},
		{"vacaciones cero días", "parametros_vacaciones_escala", `{"filas":[{"anios_servicio_desde":"0","anios_servicio_hasta":null,"dias_vacaciones":"0"}]}`, false, ""},
		{"salario mínimo", "parametros_salario_minimo", `{"filas":[{"clasificacion_puesto_id":7,"monto_mensual":"373092.30"}]}`, true, "clasificacion_puesto_id=7"},
		{"embargo tope fuera de rango", "parametros_tipos_embargo", `{"filas":[{"codigo":"PENSION_ALIMENTARIA","nombre":"Pensión","prioridad_legal":1,"es_pension_alimentaria":true,"tope_porcentaje_liquido":"150"}]}`, false, ""},
		{"baja de tipo de embargo", "parametros_tipos_embargo", `{"baja":true,"filas":[{"codigo":"EMBARGO_CCSS"}]}`, true, "codigo=EMBARGO_CCSS"},
		{"mapeo ok", "mapeo_concepto_cuenta", `{"filas":[{"concepto_salarial_id":1,"programa_presupuestario_id":1,"tipo_asiento_id":1,"cuenta_contable_id":2,"tipo_movimiento":"debe","proyecto":"P1"}]}`, true, "concepto_salarial_id=1,programa_presupuestario_id=1,tipo_asiento_id=1"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			cambio, err := ParsearPayload(e[c.tabla], []byte(c.payload))
			clave := cambio.Clave
			if c.ok != (err == nil) {
				t.Fatalf("ok esperado %v, err = %v", c.ok, err)
			}
			if err != nil && !errors.Is(err, ErrPropuestaInvalida) {
				t.Fatalf("el error debe ser ErrPropuestaInvalida: %v", err)
			}
			if c.ok && clave != c.clave {
				t.Errorf("clave %q, esperada %q", clave, c.clave)
			}
		})
	}
}
