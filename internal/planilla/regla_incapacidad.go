package planilla

import "errors"

// reglaIncapacidad (incapacidad): el subsidio por incapacidad aún no se
// calcula (lo implementa L11 con parametros_subsidios_incapacidad). Mientras
// tanto, un empleado con incapacidades en el período produce error explícito
// para no pagarle dos veces ni omitir el subsidio en silencio.
type reglaIncapacidad struct{}

func (reglaIncapacidad) Aplicar(rc *ContextoRegla) error {
	if len(rc.In.Incapacidades) > 0 {
		return errors.New("subsidio de incapacidad aún no implementado en este flujo (requiere parametros_subsidios_incapacidad)")
	}
	return nil
}
