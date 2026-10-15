package middleware

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// HeaderRequestID es la cabecera de correlación (se acepta del cliente/proxy
// si es válida; si no, se genera, y siempre se devuelve en la respuesta).
const HeaderRequestID = "X-Request-ID"

// CtxKeyRequestID es la clave en gin.Context con el request_id.
const CtxKeyRequestID = "request_id"

type ctxKeyRequestID struct{}

// Solo se acepta un X-Request-ID "simple": evita inyección de líneas/JSON en
// los logs y valores gigantes.
var requestIDValido = regexp.MustCompile(`^[A-Za-z0-9._-]{8,64}$`)

// RequestIDDesdeContext devuelve el request_id guardado en un context.Context
// ("" si no hay). Sirve para servicios que reciben ctx y quieren loguear con
// slog.*Context: el handler de NuevoLogger lo agrega solo.
func RequestIDDesdeContext(ctx context.Context) string {
	id, _ := ctx.Value(ctxKeyRequestID{}).(string)
	return id
}

// RequestIDDesdeGin devuelve el request_id de la petición en curso.
func RequestIDDesdeGin(c *gin.Context) string {
	return c.GetString(CtxKeyRequestID)
}

// RequestID asigna un request_id a cada petición: toma X-Request-ID si es
// válido o genera un UUID, lo deja en gin.Context y en c.Request.Context()
// (propagación a servicios), y lo devuelve en la cabecera de respuesta.
// Debe ser el PRIMER middleware.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(HeaderRequestID)
		if !requestIDValido.MatchString(id) {
			id = uuid.NewString()
		}
		c.Set(CtxKeyRequestID, id)
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxKeyRequestID{}, id))
		c.Header(HeaderRequestID, id)
		c.Next()
	}
}

// NuevoLogger crea el *slog.Logger de la aplicación: JSON, con request_id
// agregado automáticamente cuando el ctx lo trae, y con redacción de
// cualquier atributo cuyo nombre sugiera un secreto (defensa en profundidad:
// la regla es no loguear secretos, esto atrapa el descuido).
func NuevoLogger(w io.Writer, nivel slog.Level) *slog.Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: nivel, ReplaceAttr: redactarAtributo})
	return slog.New(&handlerRequestID{Handler: h})
}

type handlerRequestID struct{ slog.Handler }

func (h *handlerRequestID) Handle(ctx context.Context, r slog.Record) error {
	if id := RequestIDDesdeContext(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, r)
}

func (h *handlerRequestID) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &handlerRequestID{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *handlerRequestID) WithGroup(nombre string) slog.Handler {
	return &handlerRequestID{Handler: h.Handler.WithGroup(nombre)}
}

var fragmentosSensibles = []string{
	"password", "passwd", "contrasena", "contraseña", "clave", "secret",
	"token", "authorization", "cookie", "api_key", "apikey",
}

func redactarAtributo(_ []string, a slog.Attr) slog.Attr {
	k := strings.ToLower(a.Key)
	for _, f := range fragmentosSensibles {
		if strings.Contains(k, f) {
			return slog.String(a.Key, "[REDACTADO]")
		}
	}
	return a
}

// RequestLogger registra UNA línea JSON por petición, al terminar:
// método, ruta, estado, latencia, bytes, IP del cliente y usuario_id si hay
// sesión. La ruta es la PLANTILLA de la ruta (c.FullPath(), p. ej.
// "/api/v1/planillas/:id/boleta/:empleado_id"): sin query string ni valores
// de parámetros. Nunca se registran cuerpos, cabeceras ni tokens.
func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		inicio := time.Now()
		c.Next()

		ruta := c.FullPath()
		if ruta == "" {
			ruta = "(sin_ruta)" // 404/405: no se registra la URL cruda
		}
		status := c.Writer.Status()
		bytes := c.Writer.Size()
		if bytes < 0 {
			bytes = 0
		}

		attrs := []slog.Attr{
			slog.String("method", c.Request.Method),
			slog.String("route", ruta),
			slog.Int("status", status),
			slog.Float64("latency_ms", float64(time.Since(inicio).Microseconds())/1000),
			slog.Int("bytes", bytes),
			slog.String("client_ip", c.ClientIP()),
		}
		if uid := UsuarioID(c); uid > 0 {
			attrs = append(attrs, slog.Int64("usuario_id", uid))
		}

		nivel := slog.LevelInfo
		switch {
		case status >= 500:
			nivel = slog.LevelError
		case status >= 400:
			nivel = slog.LevelWarn
		}
		// Los probes de salud no deben inundar el log.
		if (ruta == "/healthz" || ruta == "/health") && nivel == slog.LevelInfo {
			nivel = slog.LevelDebug
		}

		logger.LogAttrs(c.Request.Context(), nivel, "http_request", attrs...)
	}
}

// Recovery reemplaza a gin.Recovery: convierte un panic en un 500 genérico
// (con request_id) y registra el panic y el stack con slog. Debe montarse
// FUERA de ErrorHandler (por eso escribe la respuesta directamente) y
// DENTRO de RequestLogger (para que el log de la petición vea el 500).
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			r := recover()
			if r == nil {
				return
			}
			// Protocolo de net/http para abortar una respuesta: se re-lanza.
			if err, ok := r.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(r)
			}
			logger.ErrorContext(c.Request.Context(), "panic_recuperado",
				slog.String("panic", fmt.Sprint(r)),
				slog.String("stack", string(debug.Stack())),
			)
			if c.Writer.Written() {
				c.Abort()
				return
			}
			respuestaErrorInterno(c)
		}()
		c.Next()
	}
}