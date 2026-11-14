package dto

import (
	"context"
	"database/sql"
	"fmt"
)

// CampoSensible: dato personal; se guarda en Columna_cifrado/_hash/_kid.
type CampoSensible struct{ Tabla, Columna string }

// Campo: "tabla.columna", dato adicional autenticado del cifrado (no cambiar:
// los valores ya cifrados quedan ligados a este nombre).
func (c CampoSensible) Campo() string { return c.Tabla + "." + c.Columna }

var (
	CampoIdentificacion = CampoSensible{"empleados", "numero_identificacion"}
	CampoDireccion      = CampoSensible{"empleado_datos_personales", "direccion"}
	CampoTelefono       = CampoSensible{"empleado_datos_personales", "telefono"}
	CampoCorreo         = CampoSensible{"empleado_datos_personales", "correo_electronico"}
	CampoCuenta         = CampoSensible{"empleado_datos_bancarios", "numero_cuenta"}
)

// CamposSensibles: lista blanca.
var CamposSensibles = []CampoSensible{CampoIdentificacion, CampoDireccion, CampoTelefono, CampoCorreo, CampoCuenta}

func campoPermitido(c CampoSensible) error {
	for _, x := range CamposSensibles {
		if x == c {
			return nil
		}
	}
	return fmt.Errorf("dto: campo sensible no permitido %s", c.Campo())
}

// ValorCifrado: cifrado, hash de búsqueda y llave usada.
type ValorCifrado struct {
	Cifrado []byte
	Hash    string
	Kid     string
}

// GuardarCifrado escribe X_cifrado/X_hash/X_kid de una fila.
func (q *Queries) GuardarCifrado(ctx context.Context, c CampoSensible, id int64, v ValorCifrado) error {
	if err := campoPermitido(c); err != nil {
		return err
	}
	_, err := q.db.ExecContext(ctx, `UPDATE `+c.Tabla+` SET `+c.Columna+`_cifrado = ?, `+c.Columna+`_hash = ?, `+c.Columna+`_kid = ? WHERE id = ?`,
		v.Cifrado, v.Hash, v.Kid, id)
	return err
}

// ValorSensible: valor cifrado de una columna (nil = sin dato).
type ValorSensible struct {
	Cifrado []byte
	Kid     sql.NullString
}

// DatosSensiblesRow: datos sensibles vigentes de un empleado.
type DatosSensiblesRow struct {
	EmpleadoID           int64
	NumeroIdentificacion ValorSensible
	Direccion            ValorSensible
	Telefono             ValorSensible
	Correo               ValorSensible
	NumeroCuenta         ValorSensible
}

// LeerDatosSensibles: sql.ErrNoRows si el empleado no existe.
func (q *Queries) LeerDatosSensibles(ctx context.Context, empleadoID int64) (DatosSensiblesRow, error) {
	r := DatosSensiblesRow{EmpleadoID: empleadoID}
	err := q.db.QueryRowContext(ctx, `
SELECT e.numero_identificacion_cifrado, e.numero_identificacion_kid,
       p.direccion_cifrado, p.direccion_kid,
       p.telefono_cifrado, p.telefono_kid,
       p.correo_electronico_cifrado, p.correo_electronico_kid,
       b.numero_cuenta_cifrado, b.numero_cuenta_kid
FROM empleados e
LEFT JOIN empleado_datos_personales p ON p.empleado_id = e.id AND p.es_vigente = 1
LEFT JOIN empleado_datos_bancarios b ON b.empleado_id = e.id AND b.es_vigente = 1
WHERE e.id = ?`, empleadoID).Scan(
		&r.NumeroIdentificacion.Cifrado, &r.NumeroIdentificacion.Kid,
		&r.Direccion.Cifrado, &r.Direccion.Kid,
		&r.Telefono.Cifrado, &r.Telefono.Kid,
		&r.Correo.Cifrado, &r.Correo.Kid,
		&r.NumeroCuenta.Cifrado, &r.NumeroCuenta.Kid)
	return r, err
}

// RegistroAcceso: fila de registro_accesos_datos.
type RegistroAcceso struct {
	UsuarioID  int64
	EmpleadoID int64
	Recurso    string
	Campos     string
	IP         string
	RequestID  string
}

// InsertarRegistroAcceso (solo inserción: la BD rechaza UPDATE/DELETE).
func (q *Queries) InsertarRegistroAcceso(ctx context.Context, r RegistroAcceso) error {
	_, err := q.db.ExecContext(ctx, `
INSERT INTO registro_accesos_datos (usuario_id, empleado_id, recurso, campos, ip, request_id)
VALUES (?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''))`, r.UsuarioID, r.EmpleadoID, r.Recurso, r.Campos, r.IP, r.RequestID)
	return err
}

// ExisteColumnaEnClaro indica si la base aún conserva la columna en claro;
// solo entonces hay datos que cifrar por primera vez.
func (q *Queries) ExisteColumnaEnClaro(ctx context.Context, c CampoSensible) (bool, error) {
	if err := campoPermitido(c); err != nil {
		return false, err
	}
	var n int
	err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns
WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`, c.Tabla, c.Columna).Scan(&n)
	return n > 0, err
}

// PendienteCifrado: valor en claro aún sin cifrar.
type PendienteCifrado struct {
	ID    int64
	Valor string
}

// ListarPendientesCifrado: filas con la columna en claro llena y sin cifrar.
// Solo es válida si ExisteColumnaEnClaro.
func (q *Queries) ListarPendientesCifrado(ctx context.Context, c CampoSensible, desdeID int64, limite int) ([]PendienteCifrado, error) {
	if err := campoPermitido(c); err != nil {
		return nil, err
	}
	col := c.Columna
	rows, err := q.db.QueryContext(ctx, `SELECT id, `+col+` FROM `+c.Tabla+`
WHERE id > ? AND `+col+` IS NOT NULL AND `+col+` <> '' AND `+col+`_cifrado IS NULL
ORDER BY id LIMIT ?`, desdeID, limite)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PendienteCifrado
	for rows.Next() {
		var p PendienteCifrado
		if err := rows.Scan(&p.ID, &p.Valor); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CifradoConOtraLlave: valor cifrado con una llave distinta de la activa.
type CifradoConOtraLlave struct {
	ID      int64
	Cifrado []byte
	Kid     string
}

// ListarCifradosConOtraLlave: filas cifradas con una llave distinta de
// kidActivo (rotación), ordenadas por id para avanzar por lotes.
func (q *Queries) ListarCifradosConOtraLlave(ctx context.Context, c CampoSensible, kidActivo string, desdeID int64, limite int) ([]CifradoConOtraLlave, error) {
	if err := campoPermitido(c); err != nil {
		return nil, err
	}
	col := c.Columna
	rows, err := q.db.QueryContext(ctx, `SELECT id, `+col+`_cifrado, `+col+`_kid FROM `+c.Tabla+`
WHERE id > ? AND `+col+`_cifrado IS NOT NULL AND `+col+`_kid <> ?
ORDER BY id LIMIT ?`, desdeID, kidActivo, limite)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CifradoConOtraLlave
	for rows.Next() {
		var p CifradoConOtraLlave
		if err := rows.Scan(&p.ID, &p.Cifrado, &p.Kid); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
