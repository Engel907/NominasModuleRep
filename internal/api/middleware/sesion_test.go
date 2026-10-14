package middleware

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSesionCacheTTL(t *testing.T) {
	var llamadas atomic.Int32
	activo, pv := true, uint32(1)
	c := NuevoSesionCache(func(context.Context, int64) (bool, uint32, error) {
		llamadas.Add(1)
		return activo, pv, nil
	}, 5*time.Second)
	ahora := time.Now()
	c.ahora = func() time.Time { return ahora }

	a, v, err := c.EstadoSesion(context.Background(), 1)
	require.NoError(t, err)
	require.True(t, a)
	require.EqualValues(t, 1, v)

	// Cambio en BD (rol revocado / desactivado) no se ve dentro del TTL...
	activo, pv = false, 2
	a, v, _ = c.EstadoSesion(context.Background(), 1)
	require.True(t, a)
	require.EqualValues(t, 1, v)
	require.EqualValues(t, 1, llamadas.Load())

	// ...y sí al vencer: la revocación cuesta como máximo AUTH_CACHE_TTL.
	ahora = ahora.Add(5*time.Second + time.Millisecond)
	a, v, _ = c.EstadoSesion(context.Background(), 1)
	require.False(t, a)
	require.EqualValues(t, 2, v)
	require.EqualValues(t, 2, llamadas.Load())
}

func TestSesionCacheUsuarioInexistenteEsInactivo(t *testing.T) {
	c := NuevoSesionCache(func(context.Context, int64) (bool, uint32, error) { return false, 0, sql.ErrNoRows }, time.Second)
	a, _, err := c.EstadoSesion(context.Background(), 9)
	require.NoError(t, err)
	require.False(t, a)
}

func TestSesionCacheNoCacheaErrores(t *testing.T) {
	var n atomic.Int32
	c := NuevoSesionCache(func(context.Context, int64) (bool, uint32, error) {
		if n.Add(1) == 1 {
			return false, 0, errors.New("bd caída")
		}
		return true, 1, nil
	}, time.Minute)
	_, _, err := c.EstadoSesion(context.Background(), 1)
	require.Error(t, err)
	a, _, err := c.EstadoSesion(context.Background(), 1)
	require.NoError(t, err)
	require.True(t, a)
}

func TestSesionCacheSingleflight(t *testing.T) {
	var llamadas atomic.Int32
	liberar := make(chan struct{})
	c := NuevoSesionCache(func(context.Context, int64) (bool, uint32, error) {
		llamadas.Add(1)
		<-liberar
		return true, 1, nil
	}, 0) // sin caché: solo singleflight deduplica

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _, _ = c.EstadoSesion(context.Background(), 1) }()
	}
	time.Sleep(100 * time.Millisecond)
	close(liberar)
	wg.Wait()
	require.EqualValues(t, 1, llamadas.Load())
}