package planilla

// reglaDiferida (aguinaldo, salario_escolar, vacaciones): hoy no genera línea
// en la planilla ordinaria; esos conceptos se liquidan por su propio proceso
// (acumulados/liquidaciones). L19 y L20 los llevan al ciclo de planilla.
type reglaDiferida struct{}

func (reglaDiferida) Aplicar(*ContextoRegla) error { return nil }
