package catalogos

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"
	"github.com/shopspring/decimal"

	auditdb "planillas-ccdr/internal/db"
	"planillas-ccdr/internal/db/dto"
	"planillas-ccdr/internal/parametros"
)

var (
	ErrCatalogoNoSoportado = errors.New("configuracion: catálogo no soportado")
	ErrRegistroNoExiste    = errors.New("configuracion: el registro no existe")
	ErrDatosInvalidos      = errors.New("configuracion: datos inválidos")
	ErrCampoInmutable      = errors.New("configuracion: el campo no se puede cambiar una vez creado")
	ErrBorradoNoPermitido  = errors.New("configuracion: este catálogo no permite borrar; desactive el registro")
	ErrDuplicado           = errors.New("configuracion: ya existe un registro con esos datos únicos")
	ErrEnUso               = errors.New("configuracion: el registro está en uso y no se puede borrar")
)

// Service administra los catálogos de la lista blanca.
type Service struct {
	db        *sql.DB
	catalogos map[string]Catalogo
}

func NewService(db *sql.DB, catalogos map[string]Catalogo) *Service {
	return &Service{db: db, catalogos: catalogos}
}

// CampoMeta describe un campo para que un cliente arme el formulario.
type CampoMeta struct {
	Nombre    string   `json:"nombre"`
	Tipo      string   `json:"tipo"`
	Nulo      bool     `json:"nulo"`
	Maximo    int      `json:"maximo,omitempty"`
	Valores   []string `json:"valores,omitempty"`
	Inmutable bool     `json:"inmutable"`
}

// CatalogoMeta: metadatos públicos de un catálogo.
type CatalogoMeta struct {
	Nombre        string      `json:"nombre"`
	Descripcion   string      `json:"descripcion"`
	PermiteBorrar bool        `json:"permite_borrar"`
	Campos        []CampoMeta `json:"campos"`
}

var nombresTipo = map[parametros.TipoColumna]string{
	parametros.ColTexto: "texto", parametros.ColDecimal: "decimal", parametros.ColEntero: "entero",
	parametros.ColID: "referencia", parametros.ColFecha: "fecha", parametros.ColHora: "hora", parametros.ColBool: "booleano",
}

func (s *Service) Catalogos() []CatalogoMeta {
	out := make([]CatalogoMeta, 0, len(s.catalogos))
	for _, n := range Nombres(s.catalogos) {
		c := s.catalogos[n]
		m := CatalogoMeta{Nombre: c.Nombre, Descripcion: c.Descripcion, PermiteBorrar: c.PermiteBorrar}
		for _, f := range c.Campos {
			m.Campos = append(m.Campos, CampoMeta{Nombre: f.Nombre, Tipo: nombresTipo[f.Tipo], Nulo: f.Nula,
				Maximo: f.Max, Valores: f.Valores, Inmutable: f.Inmutable})
		}
		out = append(out, m)
	}
	return out
}

func (s *Service) catalogo(nombre string) (Catalogo, error) {
	c, ok := s.catalogos[nombre]
	if !ok {
		return Catalogo{}, fmt.Errorf("%w: %q", ErrCatalogoNoSoportado, nombre)
	}
	return c, nil
}

func (s *Service) Listar(ctx context.Context, nombre string, limit, offset int32) ([]map[string]*string, int64, error) {
	c, err := s.catalogo(nombre)
	if err != nil {
		return nil, 0, err
	}
	q := dto.New(s.db)
	total, err := q.ContarFilasCatalogo(ctx, c.Nombre)
	if err != nil {
		return nil, 0, err
	}
	filas, err := q.ListarFilasCatalogo(ctx, c.Nombre, c.columnas(), limit, offset)
	return filas, total, err
}

func (s *Service) Obtener(ctx context.Context, nombre string, id int64) (map[string]*string, error) {
	c, err := s.catalogo(nombre)
	if err != nil {
		return nil, err
	}
	fila, err := dto.New(s.db).ObtenerFilaCatalogo(ctx, c.Nombre, c.columnas(), id, false)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRegistroNoExiste
	}
	return fila, err
}

func decodificar(raw []byte) (map[string]any, error) {
	jd := json.NewDecoder(bytes.NewReader(raw))
	jd.UseNumber()
	var cuerpo map[string]any
	if err := jd.Decode(&cuerpo); err != nil || cuerpo == nil {
		return nil, fmt.Errorf("%w: se espera un objeto JSON", ErrDatosInvalidos)
	}
	return cuerpo, nil
}

func normalizarCampo(c Campo, v any) (*string, error) {
	n, err := parametros.NormalizarValor(c.Columna, v)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrDatosInvalidos, c.Nombre, err)
	}
	if err := validarEnumeracion(c, n); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDatosInvalidos, err)
	}
	return n, nil
}

func valorBD(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}

// Crear inserta un registro con todos los campos del catálogo.
func (s *Service) Crear(ctx context.Context, usuarioID int64, nombre string, raw []byte) (map[string]*string, error) {
	c, err := s.catalogo(nombre)
	if err != nil {
		return nil, err
	}
	cuerpo, err := decodificar(raw)
	if err != nil {
		return nil, err
	}
	for k := range cuerpo {
		if _, ok := c.campo(k); !ok {
			return nil, fmt.Errorf("%w: campo %q no existe en %s", ErrDatosInvalidos, k, c.Nombre)
		}
	}
	fila := Fila{}
	cols := make([]string, 0, len(c.Campos))
	vals := make([]any, 0, len(c.Campos))
	for _, f := range c.Campos {
		v, err := normalizarCampo(f, cuerpo[f.Nombre])
		if err != nil {
			return nil, err
		}
		fila[f.Nombre] = v
		cols = append(cols, f.Nombre)
		vals = append(vals, valorBD(v))
	}
	if c.Validar != nil {
		if err := c.Validar(fila); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrDatosInvalidos, err)
		}
	}
	var creado map[string]*string
	err = auditdb.WithAuditContext(ctx, s.db, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		id, err := q.InsertarFilaCatalogo(ctx, c.Nombre, cols, vals)
		if err != nil {
			return traducirErrorBD(err)
		}
		creado, err = q.ObtenerFilaCatalogo(ctx, c.Nombre, c.columnas(), id, false)
		return err
	})
	return creado, err
}

// Actualizar cambia solo los campos enviados; un campo inmutable solo se
// acepta si conserva su valor.
func (s *Service) Actualizar(ctx context.Context, usuarioID int64, nombre string, id int64, raw []byte) (map[string]*string, error) {
	c, err := s.catalogo(nombre)
	if err != nil {
		return nil, err
	}
	cuerpo, err := decodificar(raw)
	if err != nil {
		return nil, err
	}
	if len(cuerpo) == 0 {
		return nil, fmt.Errorf("%w: no se envió ningún campo", ErrDatosInvalidos)
	}
	var actualizado map[string]*string
	err = auditdb.WithAuditContext(ctx, s.db, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		actual, err := q.ObtenerFilaCatalogo(ctx, c.Nombre, c.columnas(), id, true)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrRegistroNoExiste
		}
		if err != nil {
			return err
		}
		fila := Fila{}
		for _, f := range c.Campos {
			fila[f.Nombre] = actual[f.Nombre]
		}
		var cols []string
		var vals []any
		for k, crudo := range cuerpo {
			f, ok := c.campo(k)
			if !ok {
				return fmt.Errorf("%w: campo %q no existe en %s", ErrDatosInvalidos, k, c.Nombre)
			}
			v, err := normalizarCampo(f, crudo)
			if err != nil {
				return err
			}
			if f.Inmutable {
				if !iguales(v, actual[k]) {
					return fmt.Errorf("%w: %s", ErrCampoInmutable, k)
				}
				continue
			}
			fila[k] = v
			cols = append(cols, k)
			vals = append(vals, valorBD(v))
		}
		if c.Validar != nil {
			if err := c.Validar(fila); err != nil {
				return fmt.Errorf("%w: %v", ErrDatosInvalidos, err)
			}
		}
		if len(cols) > 0 {
			if err := q.ActualizarFilaCatalogo(ctx, c.Nombre, cols, vals, id); err != nil {
				return traducirErrorBD(err)
			}
		}
		actualizado, err = q.ObtenerFilaCatalogo(ctx, c.Nombre, c.columnas(), id, false)
		return err
	})
	return actualizado, err
}

func (s *Service) Borrar(ctx context.Context, usuarioID int64, nombre string, id int64) error {
	c, err := s.catalogo(nombre)
	if err != nil {
		return err
	}
	if !c.PermiteBorrar {
		return ErrBorradoNoPermitido
	}
	return auditdb.WithAuditContext(ctx, s.db, usuarioID, func(ctx context.Context, q *dto.Queries) error {
		borrado, err := q.BorrarFilaCatalogo(ctx, c.Nombre, id)
		if err != nil {
			return traducirErrorBD(err)
		}
		if !borrado {
			return ErrRegistroNoExiste
		}
		return nil
	})
}

func iguales(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if *a == *b {
		return true
	}
	da, errA := decimal.NewFromString(*a)
	db, errB := decimal.NewFromString(*b)
	return errA == nil && errB == nil && da.Equal(db)
}

func traducirErrorBD(err error) error {
	var me *mysql.MySQLError
	if !errors.As(err, &me) {
		return err
	}
	switch me.Number {
	case 1062:
		return ErrDuplicado
	case 1451:
		return ErrEnUso
	case 1452:
		return fmt.Errorf("%w: hace referencia a un registro que no existe", ErrDatosInvalidos)
	case 3819:
		return fmt.Errorf("%w: un valor no cumple las reglas de la tabla", ErrDatosInvalidos)
	case 1644:
		return fmt.Errorf("%w: %s", ErrDatosInvalidos, me.Message)
	}
	return err
}
