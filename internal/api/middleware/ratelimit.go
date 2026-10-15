package middleware

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// Limite es lo que necesitan los middlewares de rate limiting. La
// implementación de este archivo (*Limitador) es EN MEMORIA: cada proceso
// lleva sus propios contadores.
//
// LIMITACIÓN (S8): con UNA réplica es exacto. Con N réplicas detrás de un
// balanceador el límite efectivo es N veces mayor (cada réplica cuenta
// aparte). Para varias réplicas hay dos caminos, sin tocar los handlers:
//
//  1. Limitar en el borde (nginx limit_req / API gateway / WAF) para
//     /auth/login, /auth/refresh y reset, y dejar este middleware como
//     segunda línea; o
//  2. Implementar Limite sobre un almacén compartido (Redis con
//     INCR+EXPIRE o github.com/go-redis/redis_rate) y pasarlo a
//     LimitarPorIP / LimitarPorUsuario en lugar de *Limitador. El contrato es
//     solo Permitir(clave) (permitido, esperaDeReintento).
//
// Nota de seguridad: la clave por IP depende de que gin conozca la IP REAL del
// cliente. Eso exige router.SetTrustedProxies con SOLO los proxies propios
// (TRUSTED_PROXIES); si se confía en todo, un atacante rota X-Forwarded-For y
// evade el límite.
type Limite interface {
	Permitir(clave string) (permitido bool, reintentarEn time.Duration)
}

// Limitador es un conjunto de cubos de fichas (golang.org/x/time/rate), uno
// por clave, con purga perezosa de claves inactivas para acotar la memoria.
type Limitador struct {
	mu          sync.Mutex
	limite      rate.Limit
	burst       int
	ttl         time.Duration
	entradas    map[string]*entradaLimitador
	ultimaPurga time.Time
	ahora       func() time.Time
}

type entradaLimitador struct {
	lim   *rate.Limiter
	visto time.Time
}

// NuevoLimitador crea un limitador de porMinuto peticiones por minuto
// sostenidas con ráfagas de hasta burst. Ambos deben ser > 0: no existe un
// "0 = sin límite" silencioso (por eso la config los exige explícitos).
func NuevoLimitador(porMinuto, burst int) (*Limitador, error) {
	return NuevoLimitadorConReloj(porMinuto, burst, time.Now)
}

// NuevoLimitadorConReloj permite inyectar el reloj (tests).
func NuevoLimitadorConReloj(porMinuto, burst int, ahora func() time.Time) (*Limitador, error) {
	if porMinuto <= 0 || burst <= 0 {
		return nil, fmt.Errorf("middleware: el limitador requiere porMinuto > 0 y burst > 0 (recibió %d y %d)", porMinuto, burst)
	}
	lim := rate.Limit(float64(porMinuto) / 60.0)
	// Una clave inactiva más que el tiempo de rellenar el cubo completo (+1
	// min de margen) vuelve a tener el cubo lleno: se puede olvidar sin
	// cambiar el comportamiento observable.
	rellenado := time.Duration(float64(burst) / float64(lim) * float64(time.Second))
	return &Limitador{
		limite:      lim,
		burst:       burst,
		ttl:         rellenado + time.Minute,
		entradas:    make(map[string]*entradaLimitador),
		ultimaPurga: ahora(),
		ahora:       ahora,
	}, nil
}

// Permitir consume una ficha de la clave. Si no hay, devuelve false y cuánto
// falta para la próxima (sin consumirla).
func (l *Limitador) Permitir(clave string) (bool, time.Duration) {
	ahora := l.ahora()

	l.mu.Lock()
	defer l.mu.Unlock()

	l.purgar(ahora)

	e, ok := l.entradas[clave]
	if !ok {
		e = &entradaLimitador{lim: rate.NewLimiter(l.limite, l.burst)}
		l.entradas[clave] = e
	}
	e.visto = ahora

	r := e.lim.ReserveN(ahora, 1)
	if !r.OK() {
		return false, time.Minute
	}
	if espera := r.DelayFrom(ahora); espera > 0 {
		r.CancelAt(ahora) // no gastar la ficha de una petición rechazada
		return false, espera
	}
	return true, 0
}

// Tamano devuelve la cantidad de claves en memoria (tests/diagnóstico).
func (l *Limitador) Tamano() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entradas)
}

// purgar elimina, como mucho una vez por minuto, las claves inactivas.
// Requiere l.mu tomado.
func (l *Limitador) purgar(ahora time.Time) {
	if ahora.Sub(l.ultimaPurga) < time.Minute {
		return
	}
	for k, e := range l.entradas {
		if ahora.Sub(e.visto) > l.ttl {
			delete(l.entradas, k)
		}
	}
	l.ultimaPurga = ahora
}

// aplicarLimite devuelve true si la petición pasa; si no, escribe (vía Fail)
// el 429 con Retry-After.
func aplicarLimite(c *gin.Context, l Limite, clave string) bool {
	ok, espera := l.Permitir(clave)
	if ok {
		return true
	}
	seg := int(math.Ceil(espera.Seconds()))
	if seg < 1 {
		seg = 1
	}
	c.Header("Retry-After", strconv.Itoa(seg))
	Fail(c, http.StatusTooManyRequests, "demasiadas_solicitudes",
		"Demasiadas solicitudes. Intentá de nuevo en unos segundos.",
		gin.H{"reintentar_en_segundos": seg})
	return false
}

// LimitarPorIP aplica el límite por IP de cliente Y por ruta (c.FullPath()):
// así los intentos de /auth/login no consumen el cupo de /auth/refresh.
// Se monta en el grupo `publico` (login, refresh, logout y reset de
// contraseña). Detrás de NAT corporativo varias personas comparten IP: dimensionar
// RATE_LIMIT_AUTH_* con eso en mente (el bloqueo por cuenta de F14 es el
// freno específico contra adivinar contraseñas de un usuario).
func LimitarPorIP(l Limite) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !aplicarLimite(c, l, "ip:"+c.ClientIP()+"|"+c.FullPath()) {
			return
		}
		c.Next()
	}
}

// LimitarPorUsuario aplica un límite general, más laxo, por usuario
// autenticado. Debe montarse DESPUÉS de AuthPASETO (necesita el payload). Si
// por error no hay payload, cae a limitar por IP en vez de dejar pasar.
func LimitarPorUsuario(l Limite) gin.HandlerFunc {
	return func(c *gin.Context) {
		clave := "ip:" + c.ClientIP()
		if id := UsuarioID(c); id > 0 {
			clave = "u:" + strconv.FormatInt(id, 10)
		}
		if !aplicarLimite(c, l, clave) {
			return
		}
		c.Next()
	}
}
