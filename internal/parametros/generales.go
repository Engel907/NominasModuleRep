// Package parametros expone los parámetros legales/operativos versionados por
// vigencia. Generales lee parametros_generales (clave-valor tipado).
//
// Regla D8: si una clave no está vigente a la fecha pedida, se retorna
// ErrParametroNoConfigurado. Este paquete nunca devuelve un valor por defecto.
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
// la fecha consultada (D8: nunca un valor por defecto).
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
	repo  Repositorio
	ttl   time.Duration
	ahora func() time.Time

	mu    sync.Mutex
	cache map[claveCache]entradaCache
}

const maxEntradasCache = 4096

// NewGenerales crea el servicio. ttl <= 0 desactiva la caché. El TTL
// proviene de PARAMETROS_CACHE_TTL (ver config).
func NewGenerales(repo Repositorio, ttl time.Duration) *Generales {
	return &Generales{repo: repo, ttl: ttl, ahora: time.Now, cache: make(map[claveCache]entradaCache)}
}

// Invalidar vacía la caché (lo usará F27 al modificar parámetros).
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

	// Solo se cachean aciertos: un parámetro recién configurado debe verse de inmediato.
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
	reg, err := g.registro(ctx, clave, fecha)
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

// Decimal devuelve el parámetro de tipo decimal (acepta también entero).
func (g *Generales) Decimal(ctx context.Context, clave string, fecha time.Time) (decimal.Decimal, error) {
	reg, err := g.registro(ctx, clave, fecha)
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

// Porcentaje devuelve el parámetro de tipo porcentaje. El valor se almacena
// como porcentaje (10.83); la conversión la hace internal/shared/porcentaje.
func (g *Generales) Porcentaje(ctx context.Context, clave string, fecha time.Time) (porcentaje.Porcentaje, error) {
	reg, err := g.registro(ctx, clave, fecha)
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

// Texto devuelve el parámetro de tipo texto.
func (g *Generales) Texto(ctx context.Context, clave string, fecha time.Time) (string, error) {
	reg, err := g.registro(ctx, clave, fecha)
	if err != nil {
		return "", err
	}
	if err := exigirTipo(reg, TipoTexto); err != nil {
		return "", err
	}
	return reg.Valor, nil
}

// Fecha devuelve el parámetro de tipo fecha (formato YYYY-MM-DD, en UTC).
func (g *Generales) Fecha(ctx context.Context, clave string, fecha time.Time) (time.Time, error) {
	reg, err := g.registro(ctx, clave, fecha)
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

// Booleano devuelve el parámetro de tipo booleano ("true"/"false"/"1"/"0").
func (g *Generales) Booleano(ctx context.Context, clave string, fecha time.Time) (bool, error) {
	reg, err := g.registro(ctx, clave, fecha)
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