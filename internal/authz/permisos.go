// Package authz es la ÚNICA fuente de verdad de los códigos de permiso del
// sistema. Ningún otro paquete debe escribir un literal de permiso: las rutas
// usan estas constantes (middleware.RequirePermission(authz.PermXxx)) y la
// migración que siembra la tabla `permisos` se verifica contra Todos() en los
// tests de coherencia (ver migraciones_test.go y handlers/permisos_coherencia_test.go).
//
// Este paquete NO debe importar middleware ni handlers (lo importan ellos).
//
// Estado de cada código (comentario al final de la constante):
//   - "ruta"      : lo exige hoy alguna ruta de /api/v1.
//   - "reservado" : definido para fases futuras; evita choques de nombres entre
//     fases. Ninguna ruta lo exige todavía.
package authz

import "regexp"

// Permiso describe una fila del catálogo `permisos`.
type Permiso struct {
	Codigo string
	Nombre string
	Modulo string
}

// FormatoCodigo es el formato válido de un código de permiso: dominio.accion,
// en minúsculas, con guion bajo permitido. Máximo 60 caracteres (permisos.codigo).
var FormatoCodigo = regexp.MustCompile(`^[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*$`)

// --- Empleados, contratos y organización -----------------------------------
const (
	PermEmpleadosCrear             = "empleados.crear"               // ruta
	PermEmpleadosLeer              = "empleados.leer"                // ruta
	PermEmpleadosActualizar        = "empleados.actualizar"          // ruta
	PermContratosCrear             = "contratos.crear"               // ruta
	PermContratosLeer              = "contratos.leer"                // ruta
	PermContratosActualizar        = "contratos.actualizar"          // ruta
	PermOrganizacionCrear          = "organizacion.crear"            // ruta
	PermOrganizacionLeer           = "organizacion.leer"             // ruta
	PermOrganizacionActualizar     = "organizacion.actualizar"       // ruta
	PermDatosPersonalesVerSensible = "datos_personales.ver_sensible" // reservado
)

// --- Asistencia, calendario y vacaciones -----------------------------------
const (
	PermAsistenciaCrear             = "asistencia.crear"              // ruta
	PermAsistenciaVer               = "asistencia.ver"                // ruta
	PermAsistenciaEditar            = "asistencia.editar"             // reservado
	PermCalendarioAdministrar       = "calendario.administrar"        // ruta
	PermIncapacidadesGestionar      = "incapacidades.gestionar"       // ruta
	PermVacacionesSolicitar         = "vacaciones.solicitar"          // ruta
	PermVacacionesAprobar           = "vacaciones.aprobar"            // ruta
	PermVacacionesRegistrarDisfrute = "vacaciones.registrar_disfrute" // ruta
	PermVacacionesVer               = "vacaciones.ver"                // ruta
	PermVacacionesSolicitarTerceros = "vacaciones.solicitar_terceros" // reservado
	PermVacacionesVerTerceros       = "vacaciones.ver_terceros"       // reservado
)

// --- Préstamos, embargos, terceros y liquidaciones -------------------------
const (
	PermPrestamosCrear         = "prestamos.crear"         // ruta
	PermPrestamosVer           = "prestamos.ver"           // ruta
	PermPrestamosAbonar        = "prestamos.abonar"        // ruta
	PermPrestamosCancelar      = "prestamos.cancelar"      // ruta
	PermEmbargosCrear          = "embargos.crear"          // ruta
	PermEmbargosVer            = "embargos.ver"            // ruta
	PermEmbargosCambiarEstado  = "embargos.cambiar_estado" // ruta
	PermTercerosCrear          = "terceros.crear"          // ruta
	PermTercerosVer            = "terceros.ver"            // ruta
	PermLiquidacionesProyectar = "liquidaciones.proyectar" // ruta
	PermLiquidacionesProcesar  = "liquidaciones.procesar"  // ruta
)

// --- Planilla, aguinaldo, salario escolar y dashboard ----------------------
const (
	PermPlanillaCrear            = "planilla.crear"             // reservado
	PermPlanillaCalcular         = "planilla.calcular"          // reservado
	PermPlanillaVer              = "planilla.ver"               // reservado
	PermPlanillaAprobar          = "planilla.aprobar"           // reservado
	PermPlanillaPagar            = "planilla.pagar"             // reservado
	PermPlanillaExportarBancario = "planilla.exportar_bancario" // ruta
	PermPlanillaVerBoleta        = "planilla.ver_boleta"        // ruta
	PermPlanillaVerBoletaPropia  = "planilla.ver_boleta_propia" // reservado
	PermAguinaldoEjecutar        = "aguinaldo.ejecutar"         // reservado
	PermSalarioEscolarEjecutar   = "salario_escolar.ejecutar"   // reservado
	PermDashboardVer             = "dashboard.ver"              // ruta
)

// --- Parámetros ------------------------------------------------------------
const (
	PermParametrosVer      = "parametros.ver"      // reservado
	PermParametrosProponer = "parametros.proponer" // reservado
	PermParametrosAprobar  = "parametros.aprobar"  // reservado
)

// --- Contabilización y reportes --------------------------------------------
const (
	PermAsientosContablesGenerar = "asientos_contables.generar" // ruta
	PermAsientosContablesVer     = "asientos_contables.ver"     // ruta
	PermReportesCcss             = "reportes.ccss"              // reservado
	PermReportesIns              = "reportes.ins"               // reservado
	PermReportesHacienda         = "reportes.hacienda"          // reservado
)

// --- Usuarios y auditoría --------------------------------------------------
const (
	PermUsuariosAsignarRol        = "usuarios.asignar_rol"        // ruta
	PermUsuariosGestionarSesiones = "usuarios.gestionar_sesiones" // ruta
	PermUsuariosCrear             = "usuarios.crear"              // reservado
	PermUsuariosVer               = "usuarios.ver"                // reservado
	PermUsuariosActualizar        = "usuarios.actualizar"         // reservado
	PermUsuariosResetPassword     = "usuarios.reset_password"     // reservado
	PermRolesAdministrar          = "roles.administrar"           // ruta
	PermAuditoriaVer              = "auditoria.ver"               // reservado
)

// Todos devuelve el catálogo completo, en un orden estable. Es lo que la
// migración de permisos debe sembrar, ni más ni menos.
func Todos() []Permiso {
	return []Permiso{
		{Codigo: PermEmpleadosCrear, Nombre: "Crear expedientes de empleados", Modulo: "empleados"},
		{Codigo: PermEmpleadosLeer, Nombre: "Consultar empleados", Modulo: "empleados"},
		{Codigo: PermEmpleadosActualizar, Nombre: "Actualizar datos de empleados", Modulo: "empleados"},
		{Codigo: PermContratosCrear, Nombre: "Crear contratos laborales", Modulo: "empleados"},
		{Codigo: PermContratosLeer, Nombre: "Consultar contratos laborales", Modulo: "empleados"},
		{Codigo: PermContratosActualizar, Nombre: "Actualizar contratos laborales", Modulo: "empleados"},
		{Codigo: PermOrganizacionCrear, Nombre: "Crear catálogos organizacionales", Modulo: "empleados"},
		{Codigo: PermOrganizacionLeer, Nombre: "Consultar catálogos organizacionales", Modulo: "empleados"},
		{Codigo: PermOrganizacionActualizar, Nombre: "Actualizar catálogos organizacionales", Modulo: "empleados"},
		{Codigo: PermDatosPersonalesVerSensible, Nombre: "Ver datos personales sensibles", Modulo: "empleados"},
		{Codigo: PermAsistenciaCrear, Nombre: "Registrar asistencia", Modulo: "asistencia"},
		{Codigo: PermAsistenciaVer, Nombre: "Ver asistencia", Modulo: "asistencia"},
		{Codigo: PermAsistenciaEditar, Nombre: "Editar asistencia", Modulo: "asistencia"},
		{Codigo: PermCalendarioAdministrar, Nombre: "Administrar calendario laboral y turnos", Modulo: "asistencia"},
		{Codigo: PermIncapacidadesGestionar, Nombre: "Gestionar incapacidades CCSS/INS", Modulo: "asistencia"},
		{Codigo: PermVacacionesSolicitar, Nombre: "Solicitar vacaciones", Modulo: "asistencia"},
		{Codigo: PermVacacionesAprobar, Nombre: "Aprobar o rechazar solicitudes de vacaciones", Modulo: "asistencia"},
		{Codigo: PermVacacionesRegistrarDisfrute, Nombre: "Registrar el disfrute de vacaciones aprobadas", Modulo: "asistencia"},
		{Codigo: PermVacacionesVer, Nombre: "Ver vacaciones", Modulo: "asistencia"},
		{Codigo: PermVacacionesSolicitarTerceros, Nombre: "Solicitar vacaciones en nombre de terceros", Modulo: "asistencia"},
		{Codigo: PermVacacionesVerTerceros, Nombre: "Ver vacaciones de terceros", Modulo: "asistencia"},
		{Codigo: PermPrestamosCrear, Nombre: "Crear préstamos de empleados", Modulo: "planilla"},
		{Codigo: PermPrestamosVer, Nombre: "Consultar préstamos de empleados", Modulo: "planilla"},
		{Codigo: PermPrestamosAbonar, Nombre: "Aplicar abonos a préstamos", Modulo: "planilla"},
		{Codigo: PermPrestamosCancelar, Nombre: "Cancelar préstamos", Modulo: "planilla"},
		{Codigo: PermEmbargosCrear, Nombre: "Crear embargos", Modulo: "planilla"},
		{Codigo: PermEmbargosVer, Nombre: "Consultar embargos", Modulo: "planilla"},
		{Codigo: PermEmbargosCambiarEstado, Nombre: "Cambiar el estado de un embargo", Modulo: "planilla"},
		{Codigo: PermTercerosCrear, Nombre: "Crear terceros acreedores", Modulo: "planilla"},
		{Codigo: PermTercerosVer, Nombre: "Consultar terceros acreedores", Modulo: "planilla"},
		{Codigo: PermLiquidacionesProyectar, Nombre: "Proyectar liquidaciones laborales", Modulo: "planilla"},
		{Codigo: PermLiquidacionesProcesar, Nombre: "Procesar liquidaciones laborales", Modulo: "planilla"},
		{Codigo: PermPlanillaCrear, Nombre: "Crear períodos y corridas de planilla", Modulo: "planilla"},
		{Codigo: PermPlanillaCalcular, Nombre: "Calcular planilla", Modulo: "planilla"},
		{Codigo: PermPlanillaVer, Nombre: "Consultar planilla", Modulo: "planilla"},
		{Codigo: PermPlanillaAprobar, Nombre: "Aprobar planilla", Modulo: "planilla"},
		{Codigo: PermPlanillaPagar, Nombre: "Registrar el pago de planilla", Modulo: "planilla"},
		{Codigo: PermPlanillaExportarBancario, Nombre: "Generar archivo de dispersión bancaria", Modulo: "planilla"},
		{Codigo: PermPlanillaVerBoleta, Nombre: "Consultar boleta de pago de un empleado", Modulo: "planilla"},
		{Codigo: PermPlanillaVerBoletaPropia, Nombre: "Consultar la boleta de pago propia", Modulo: "planilla"},
		{Codigo: PermAguinaldoEjecutar, Nombre: "Ejecutar el cálculo de aguinaldo", Modulo: "planilla"},
		{Codigo: PermSalarioEscolarEjecutar, Nombre: "Ejecutar el cálculo de salario escolar", Modulo: "planilla"},
		{Codigo: PermDashboardVer, Nombre: "Ver el dashboard gerencial", Modulo: "planilla"},
		{Codigo: PermParametrosVer, Nombre: "Consultar parámetros de planilla", Modulo: "parametros"},
		{Codigo: PermParametrosProponer, Nombre: "Proponer cambios a parámetros de planilla", Modulo: "parametros"},
		{Codigo: PermParametrosAprobar, Nombre: "Aprobar cambios a parámetros de planilla", Modulo: "parametros"},
		{Codigo: PermAsientosContablesGenerar, Nombre: "Generar los asientos contables de un período de planilla", Modulo: "contabilizacion"},
		{Codigo: PermAsientosContablesVer, Nombre: "Consultar asientos contables generados", Modulo: "contabilizacion"},
		{Codigo: PermReportesCcss, Nombre: "Generar reportes para la CCSS", Modulo: "integraciones"},
		{Codigo: PermReportesIns, Nombre: "Generar reportes para el INS", Modulo: "integraciones"},
		{Codigo: PermReportesHacienda, Nombre: "Generar reportes para Hacienda", Modulo: "integraciones"},
		{Codigo: PermUsuariosAsignarRol, Nombre: "Asignar y revocar roles de usuarios", Modulo: "auditoria"},
		{Codigo: PermUsuariosGestionarSesiones, Nombre: "Cerrar todas las sesiones activas", Modulo: "auditoria"},
		{Codigo: PermUsuariosCrear, Nombre: "Crear usuarios del sistema", Modulo: "auditoria"},
		{Codigo: PermUsuariosVer, Nombre: "Consultar usuarios del sistema", Modulo: "auditoria"},
		{Codigo: PermUsuariosActualizar, Nombre: "Actualizar usuarios del sistema", Modulo: "auditoria"},
		{Codigo: PermUsuariosResetPassword, Nombre: "Restablecer contraseñas de usuarios", Modulo: "auditoria"},
		{Codigo: PermRolesAdministrar, Nombre: "Administrar roles y sus permisos", Modulo: "auditoria"},
		{Codigo: PermAuditoriaVer, Nombre: "Consultar la bitácora de auditoría", Modulo: "auditoria"},
	}
}

// Codigos devuelve solo los códigos de Todos().
func Codigos() []string {
	todos := Todos()
	out := make([]string, len(todos))
	for i, p := range todos {
		out[i] = p.Codigo
	}
	return out
}

// Existe indica si un código pertenece al catálogo.
func Existe(codigo string) bool {
	for _, p := range Todos() {
		if p.Codigo == codigo {
			return true
		}
	}
	return false
}
