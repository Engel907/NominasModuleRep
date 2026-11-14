// Package parametros expone los parámetros legales/operativos versionados por
// vigencia. Generales lee parametros_generales (clave-valor tipado).
package parametros

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"

	"planillas-ccdr/internal/shared/porcentaje"
)

// Tipos válidos de parametros_generales.tipo.
const (
	TipoEntero     = "entero"
	TipoDecimal    = "decimal"
	TipoPorcentaje = "porcentaje"
	TipoTexto      = "texto"
	TipoFecha      = "fecha"
	TipoBooleano   = "booleano"
)

const formatoFecha = "2006-01-02"

// ErrNoEncontrado lo retorna el Repositorio cuando no hay fila vigente.
var ErrNoEncontrado = errors.New("parametros: sin fila vigente")

// ErrTipoIncorrecto: el parámetro existe pero su tipo no corresponde al accesor usado.
var ErrTipoIncorrecto = errors.New("parametros: tipo de parámetro incorrecto")

// ErrValorInvalido: el valor almacenado no se puede interpretar según su tipo.
var ErrValorInvalido = errors.New("parametros: valor almacenado inválido")

// ErrParametroNoConfigurado indica que la clave no existe o no está vigente a
// la fecha consultada (nunca un valor por defecto).
type ErrParametroNoConfigurado struct {
	Clave string
	Fecha time.Time
}

func (e ErrParametroNoConfigurado) Error() string {
	return fmt.Sprintf("parametros: parámetro general %q no configurado o no vigente al %s", e.Clave, e.Fecha.Format(formatoFecha))
}

// Registro es la fila vigente de parametros_generales.
type Registro struct {
	Clave string
	Valor string
	Tipo  string
}

// Repositorio resuelve la fila vigente de una clave a una fecha. Debe
// retornar ErrNoEncontrado (o un error que lo envuelva) si no hay fila.
type Repositorio interface {
	BuscarVigente(ctx context.Context, clave string, fecha time.Time) (Registro, error)
}

// RepositorioEmpleado resuelve el valor propio de un empleado
// (empleado_parametros_generales). ErrNoEncontrado si el empleado usa el
// valor general.
type RepositorioEmpleado interface {
	BuscarVigenteEmpleado(ctx context.Context, empleadoID int64, clave string, fecha time.Time) (Registro, error)
}

type claveCache struct {
	clave string
	fecha string // YYYY-MM-DD
}

type entradaCache struct {
	reg    Registro
	expira time.Time
}

// Generales es el servicio de lectura de parametros_generales, con caché en
// memoria de TTL corto. Es seguro para uso concurrente.
type Generales struct {
	repo    Repositorio
	repoEmp RepositorioEmpleado // nil = sin valores por empleado
	ttl     time.Duration
	ahora   func() time.Time

	mu    sync.Mutex
	cache map[claveCache]entradaCache
}

const maxEntradasCache = 4096

// NewGenerales crea el servicio. ttl <= 0 desactiva la caché. El TTL
// proviene de PARAMETROS_CACHE_TTL.
func NewGenerales(repo Repositorio, ttl time.Duration) *Generales {
	return &Generales{repo: repo, ttl: ttl, ahora: time.Now, cache: make(map[claveCache]entradaCache)}
}

// ConValoresPorEmpleado habilita ParaEmpleado.
func (g *Generales) ConValoresPorEmpleado(r RepositorioEmpleado) *Generales {
	g.repoEmp = r
	return g
}

// GeneralesEmpleado lee los parámetros generales como rigen para un empleado:
// su valor propio vigente si lo tiene, si no el general. Mismos accesores que
// Generales.
type GeneralesEmpleado struct {
	g          *Generales
	empleadoID int64
}

// ParaEmpleado: lector de parámetros generales de un empleado.
func (g *Generales) ParaEmpleado(empleadoID int64) *GeneralesEmpleado {
	return &GeneralesEmpleado{g: g, empleadoID: empleadoID}
}

func (e *GeneralesEmpleado) registro(ctx context.Context, clave string, fecha time.Time) (Registro, error) {
	if e.g.repoEmp != nil {
		reg, err := e.g.repoEmp.BuscarVigenteEmpleado(ctx, e.empleadoID, clave, soloFecha(fecha))
		if err == nil {
			return reg, nil
		}
		if !errors.Is(err, ErrNoEncontrado) {
			return Registro{}, fmt.Errorf("parametros: leyendo %q del empleado %d: %w", clave, e.empleadoID, err)
		}
	}
	return e.g.registro(ctx, clave, fecha)
}

func (e *GeneralesEmpleado) Entero(ctx context.Context, clave string, fecha time.Time) (int64, error) {
	return enteroDe(e.registro(ctx, clave, fecha))
}
func (e *GeneralesEmpleado) Decimal(ctx context.Context, clave string, fecha time.Time) (decimal.Decimal, error) {
	return decimalDeRegistro(e.registro(ctx, clave, fecha))
}
func (e *GeneralesEmpleado) Porcentaje(ctx context.Context, clave string, fecha time.Time) (porcentaje.Porcentaje, error) {
	return porcentajeDe(e.registro(ctx, clave, fecha))
}
func (e *GeneralesEmpleado) Texto(ctx context.Context, clave string, fecha time.Time) (string, error) {
	return textoDe(e.registro(ctx, clave, fecha))
}
func (e *GeneralesEmpleado) Fecha(ctx context.Context, clave string, fecha time.Time) (time.Time, error) {
	return fechaDe(e.registro(ctx, clave, fecha))
}
func (e *GeneralesEmpleado) Booleano(ctx context.Context, clave string, fecha time.Time) (bool, error) {
	return booleanoDe(e.registro(ctx, clave, fecha))
}

// Invalidar vacía la caché (lo usará al modificar parámetros).
func (g *Generales) Invalidar() {
	g.mu.Lock()
	g.cache = make(map[claveCache]entradaCache)
	g.mu.Unlock()
}

func (g *Generales) registro(ctx context.Context, clave string, fecha time.Time) (Registro, error) {
	f := soloFecha(fecha)
	k := claveCache{clave: clave, fecha: f.Format(formatoFecha)}
	ahora := g.ahora()

	if g.ttl > 0 {
		g.mu.Lock()
		if e, ok := g.cache[k]; ok && ahora.Before(e.expira) {
			g.mu.Unlock()
			return e.reg, nil
		}
		g.mu.Unlock()
	}

	reg, err := g.repo.BuscarVigente(ctx, clave, f)
	if err != nil {
		if errors.Is(err, ErrNoEncontrado) {
			return Registro{}, ErrParametroNoConfigurado{Clave: clave, Fecha: f}
		}
		return Registro{}, fmt.Errorf("parametros: leyendo %q: %w", clave, err)
	}

	if g.ttl > 0 {
		g.mu.Lock()
		if len(g.cache) >= maxEntradasCache {
			for kk, e := range g.cache {
				if !ahora.Before(e.expira) {
					delete(g.cache, kk)
				}
			}
			if len(g.cache) >= maxEntradasCache {
				g.cache = make(map[claveCache]entradaCache)
			}
		}
		g.cache[k] = entradaCache{reg: reg, expira: ahora.Add(g.ttl)}
		g.mu.Unlock()
	}
	return reg, nil
}

func exigirTipo(reg Registro, validos ...string) error {
	for _, t := range validos {
		if reg.Tipo == t {
			return nil
		}
	}
	return fmt.Errorf("%w: %q es %q, se esperaba %s", ErrTipoIncorrecto, reg.Clave, reg.Tipo, strings.Join(validos, " o "))
}

func valorInvalido(reg Registro, err error) error {
	return fmt.Errorf("%w: %q = %q: %v", ErrValorInvalido, reg.Clave, reg.Valor, err)
}

// Entero devuelve el parámetro de tipo entero.
func (g *Generales) Entero(ctx context.Context, clave string, fecha time.Time) (int64, error) {
	return enteroDe(g.registro(ctx, clave, fecha))
}

// Decimal devuelve el parámetro de tipo decimal (acepta también entero).
func (g *Generales) Decimal(ctx context.Context, clave string, fecha time.Time) (decimal.Decimal, error) {
	return decimalDeRegistro(g.registro(ctx, clave, fecha))
}

// Porcentaje devuelve el parámetro de tipo porcentaje. El valor se almacena
// como porcentaje (10.83); la conversión la hace internal/shared/porcentaje.
func (g *Generales) Porcentaje(ctx context.Context, clave string, fecha time.Time) (porcentaje.Porcentaje, error) {
	return porcentajeDe(g.registro(ctx, clave, fecha))
}

// Texto devuelve el parámetro de tipo texto.
func (g *Generales) Texto(ctx context.Context, clave string, fecha time.Time) (string, error) {
	return textoDe(g.registro(ctx, clave, fecha))
}

// Fecha devuelve el parámetro de tipo fecha (formato YYYY-MM-DD, en UTC).
func (g *Generales) Fecha(ctx context.Context, clave string, fecha time.Time) (time.Time, error) {
	return fechaDe(g.registro(ctx, clave, fecha))
}

// Booleano devuelve el parámetro de tipo booleano ("true"/"false"/"1"/"0").
func (g *Generales) Booleano(ctx context.Context, clave string, fecha time.Time) (bool, error) {
	return booleanoDe(g.registro(ctx, clave, fecha))
}

func enteroDe(reg Registro, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	if err := exigirTipo(reg, TipoEntero); err != nil {
		return 0, err
	}
	n, err := strconv.ParseInt(strings.TrimSpace(reg.Valor), 10, 64)
	if err != nil {
		return 0, valorInvalido(reg, err)
	}
	return n, nil
}

func decimalDeRegistro(reg Registro, err error) (decimal.Decimal, error) {
	if err != nil {
		return decimal.Zero, err
	}
	if err := exigirTipo(reg, TipoDecimal, TipoEntero); err != nil {
		return decimal.Zero, err
	}
	d, err := decimal.NewFromString(strings.TrimSpace(reg.Valor))
	if err != nil {
		return decimal.Zero, valorInvalido(reg, err)
	}
	return d, nil
}

func porcentajeDe(reg Registro, err error) (porcentaje.Porcentaje, error) {
	if err != nil {
		return porcentaje.Porcentaje{}, err
	}
	if err := exigirTipo(reg, TipoPorcentaje); err != nil {
		return porcentaje.Porcentaje{}, err
	}
	d, err := decimal.NewFromString(strings.TrimSpace(reg.Valor))
	if err != nil {
		return porcentaje.Porcentaje{}, valorInvalido(reg, err)
	}
	p, err := porcentaje.DesdePorcentaje(d)
	if err != nil {
		return porcentaje.Porcentaje{}, valorInvalido(reg, err)
	}
	return p, nil
}

func textoDe(reg Registro, err error) (string, error) {
	if err != nil {
		return "", err
	}
	if err := exigirTipo(reg, TipoTexto); err != nil {
		return "", err
	}
	return reg.Valor, nil
}

func fechaDe(reg Registro, err error) (time.Time, error) {
	if err != nil {
		return time.Time{}, err
	}
	if err := exigirTipo(reg, TipoFecha); err != nil {
		return time.Time{}, err
	}
	t, err := time.ParseInLocation(formatoFecha, strings.TrimSpace(reg.Valor), time.UTC)
	if err != nil {
		return time.Time{}, valorInvalido(reg, err)
	}
	return t, nil
}

func booleanoDe(reg Registro, err error) (bool, error) {
	if err != nil {
		return false, err
	}
	if err := exigirTipo(reg, TipoBooleano); err != nil {
		return false, err
	}
	b, err := strconv.ParseBool(strings.TrimSpace(reg.Valor))
	if err != nil {
		return false, valorInvalido(reg, err)
	}
	return b, nil
}

// soloFecha descarta la hora y deja la fecha calendario en UTC.
func soloFecha(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
