package middleware

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// SesionVerificador (F15 / S7) responde, por usuario, si la cuenta sigue activa
// y cuál es su versión de permisos vigente. AuthPASETO compara contra el "pv"
// del token: si el usuario está inactivo o pv difiere, la sesión es inválida.
type SesionVerificador interface {
	EstadoSesion(ctx context.Context, usuarioID int64) (activo bool, permisosVersion uint32, err error)
}

// FuenteEstadoSesion es la lectura a BD (la implementa *dto.Queries adaptado,
// ver app.go). Debe devolver sql.ErrNoRows si el usuario no existe.
type FuenteEstadoSesion func(ctx context.Context, usuarioID int64) (activo bool, permisosVersion uint32, err error)

type entradaSesion struct {
	activo bool
	pv     uint32
	expira time.Time
}

// SesionCache es un SesionVerificador con caché en memoria de TTL corto y
// singleflight: N requests concurrentes del mismo usuario con la caché vencida
// provocan UNA consulta a BD. La revocación tarda como máximo `ttl` en notarse
// (por proceso; con varias instancias, cada una tiene su propia caché).
type SesionCache struct {
	fuente FuenteEstadoSesion
	ttl    time.Duration
	ahora  func() time.Time

	mu       sync.Mutex
	entradas map[int64]entradaSesion
	sf       singleflight.Group
}

const (
	maxEntradasCache  = 20000
	timeoutConsultaBD = 3 * time.Second
)

func NuevoSesionCache(fuente FuenteEstadoSesion, ttl time.Duration) *SesionCache {
	return &SesionCache{fuente: fuente, ttl: ttl, ahora: time.Now, entradas: make(map[int64]entradaSesion)}
}

func (c *SesionCache) EstadoSesion(ctx context.Context, usuarioID int64) (bool, uint32, error) {
	if c.ttl > 0 {
		c.mu.Lock()
		e, ok := c.entradas[usuarioID]
		c.mu.Unlock()
		if ok && c.ahora().Before(e.expira) {
			return e.activo, e.pv, nil
		}
	}

	// La consulta compartida no depende del contexto de un request concreto:
	// si ese cliente cancela, los demás que esperan no deben fallar.
	v, err, _ := c.sf.Do(fmt.Sprint(usuarioID), func() (any, error) {
		qctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeoutConsultaBD)
		defer cancel()
		activo, pv, err := c.fuente(qctx, usuarioID)
		if errors.Is(err, sql.ErrNoRows) {
			activo, pv, err = false, 0, nil // usuario inexistente = sesión inválida
		}
		if err != nil {
			return nil, err // no se cachean errores
		}
		e := entradaSesion{activo: activo, pv: pv, expira: c.ahora().Add(c.ttl)}
		if c.ttl > 0 {
			c.guardar(usuarioID, e)
		}
		return e, nil
	})
	if err != nil {
		return false, 0, err
	}
	e := v.(entradaSesion)
	return e.activo, e.pv, nil
}

func (c *SesionCache) guardar(id int64, e entradaSesion) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entradas) >= maxEntradasCache {
		ahora := c.ahora()
		for k, v := range c.entradas {
			if !ahora.Before(v.expira) {
				delete(c.entradas, k)
			}
		}
		if len(c.entradas) >= maxEntradasCache { // todo vigente: vaciar, es solo caché
			c.entradas = make(map[int64]entradaSesion)
		}
	}
	c.entradas[id] = e
}