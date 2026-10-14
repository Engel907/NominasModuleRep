package auth

// Notificador (F14 / S2): canal por el que el servicio entrega al usuario el
// token en claro del reset de contraseña. El servicio solo persiste el hash.

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/url"
	"strings"
	"time"

	"planillas-ccdr/internal/config"
)

type Notificador interface {
	EnviarResetPassword(ctx context.Context, correo, nombre, token string, expira time.Time) error
}

// ErrLogNotificadorSoloDesarrollo: LogNotificador escribe el token en claro
// en el log; con cualquier APP_ENV distinto de "development" se rehúsa a existir.
var ErrLogNotificadorSoloDesarrollo = errors.New("auth: LogNotificador solo puede activarse con APP_ENV=development")

// NuevoNotificador elige la implementación según la configuración:
// SMTP si hay SMTP_HOST; LogNotificador solo si APP_ENV=development; en
// cualquier otro caso devuelve error (el arranque debe fallar).
func NuevoNotificador(cfg *config.Config, logger *slog.Logger) (Notificador, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.SMTP.Host != "" {
		return NuevoSMTPNotificador(cfg.SMTP)
	}
	if cfg.App.Env == "development" {
		return NuevoLogNotificador(cfg.App.Env, cfg.SMTP.URLBaseReset, logger)
	}
	return nil, fmt.Errorf("auth: sin SMTP_HOST y APP_ENV=%q: no hay forma de entregar el token de reset", cfg.App.Env)
}

// ----------------------------------------------------------------------------
// LogNotificador (solo desarrollo)
// ----------------------------------------------------------------------------

type LogNotificador struct {
	logger  *slog.Logger
	urlBase string
}

func NuevoLogNotificador(appEnv, urlBaseReset string, logger *slog.Logger) (*LogNotificador, error) {
	if appEnv != "development" {
		return nil, ErrLogNotificadorSoloDesarrollo
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &LogNotificador{logger: logger, urlBase: urlBaseReset}, nil
}

func (n *LogNotificador) EnviarResetPassword(ctx context.Context, correo, nombre, token string, expira time.Time) error {
	n.logger.WarnContext(ctx, "[DESARROLLO] reset de contraseña: el token NO se envió por correo",
		"correo", correo,
		"nombre", nombre,
		"token", token,
		"enlace", enlaceReset(n.urlBase, token),
		"expira", expira.Format(time.RFC3339),
	)
	return nil
}

// ----------------------------------------------------------------------------
// SMTPNotificador
// ----------------------------------------------------------------------------

type SMTPNotificador struct {
	host, port, user, pass string
	from                   mail.Address
	urlBase                string
	timeout                time.Duration
}

func NuevoSMTPNotificador(cfg config.SMTPConfig) (*SMTPNotificador, error) {
	var faltan []string
	if cfg.Host == "" {
		faltan = append(faltan, "SMTP_HOST")
	}
	if cfg.Port == "" {
		faltan = append(faltan, "SMTP_PORT")
	}
	if cfg.From == "" {
		faltan = append(faltan, "SMTP_FROM")
	}
	if cfg.URLBaseReset == "" {
		faltan = append(faltan, "URL_BASE_RESET")
	}
	if len(faltan) > 0 {
		return nil, fmt.Errorf("auth: configuración SMTP incompleta, faltan %v", faltan)
	}
	from, err := mail.ParseAddress(cfg.From)
	if err != nil {
		return nil, fmt.Errorf("auth: SMTP_FROM inválido: %w", err)
	}
	return &SMTPNotificador{
		host: cfg.Host, port: cfg.Port, user: cfg.User, pass: cfg.Pass,
		from: *from, urlBase: cfg.URLBaseReset, timeout: 15 * time.Second,
	}, nil
}

func (n *SMTPNotificador) EnviarResetPassword(ctx context.Context, correo, nombre, token string, expira time.Time) error {
	// mail.ParseAddress rechaza CR/LF: evita inyección de cabeceras vía correo.
	destino, err := mail.ParseAddress(correo)
	if err != nil {
		return fmt.Errorf("auth: correo de destino inválido: %w", err)
	}
	nombre = limpiarCabecera(nombre)
	destino.Name = nombre

	// Costa Rica no tiene horario de verano: UTC-6 fijo evita depender de tzdata.
	hora := expira.In(time.FixedZone("CR", -6*3600)).Format("02/01/2006 15:04")
	saludo := "Hola"
	if nombre != "" {
		saludo = "Hola " + nombre
	}
	cuerpo := fmt.Sprintf("%s,\r\n\r\n"+
		"Recibimos una solicitud para restablecer la contraseña de su cuenta en Planillas CCDR Liberia.\r\n"+
		"Use el siguiente enlace antes del %s (hora de Costa Rica); sirve una sola vez:\r\n\r\n"+
		"%s\r\n\r\n"+
		"Si usted no hizo la solicitud, ignore este mensaje: su contraseña no cambiará.\r\n",
		saludo, hora, enlaceReset(n.urlBase, token))

	var msg strings.Builder
	fmt.Fprintf(&msg, "From: %s\r\n", n.from.String())
	fmt.Fprintf(&msg, "To: %s\r\n", destino.String())
	fmt.Fprintf(&msg, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", "Restablecer su contraseña - Planillas CCDR"))
	fmt.Fprintf(&msg, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	qp := quotedprintable.NewWriter(&msg)
	if _, err := io.WriteString(qp, cuerpo); err != nil {
		return err
	}
	if err := qp.Close(); err != nil {
		return err
	}

	return n.enviar(ctx, destino.Address, []byte(msg.String()))
}

// enviar abre la conexión con TLS obligatorio: TLS implícito en el puerto 465
// o STARTTLS (requerido, no opcional) en cualquier otro puerto.
func (n *SMTPNotificador) enviar(ctx context.Context, destino string, msg []byte) error {
	tlsCfg := &tls.Config{ServerName: n.host, MinVersion: tls.VersionTLS12}
	dialer := &net.Dialer{Timeout: n.timeout}
	addr := net.JoinHostPort(n.host, n.port)

	var (
		conn net.Conn
		err  error
	)
	if n.port == "465" {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: tlsCfg}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("auth: conexión SMTP: %w", err)
	}
	defer conn.Close()

	limite := time.Now().Add(n.timeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(limite) {
		limite = dl
	}
	_ = conn.SetDeadline(limite)

	c, err := smtp.NewClient(conn, n.host)
	if err != nil {
		return fmt.Errorf("auth: saludo SMTP: %w", err)
	}
	defer c.Close()

	if n.port != "465" {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return errors.New("auth: el servidor SMTP no ofrece STARTTLS; no se envía en claro")
		}
		if err := c.StartTLS(tlsCfg); err != nil {
			return fmt.Errorf("auth: STARTTLS: %w", err)
		}
	}
	if n.user != "" {
		if err := c.Auth(smtp.PlainAuth("", n.user, n.pass, n.host)); err != nil {
			return fmt.Errorf("auth: autenticación SMTP: %w", err)
		}
	}
	if err := c.Mail(n.from.Address); err != nil {
		return fmt.Errorf("auth: MAIL FROM: %w", err)
	}
	if err := c.Rcpt(destino); err != nil {
		return fmt.Errorf("auth: RCPT TO: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("auth: DATA: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("auth: escribiendo mensaje: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("auth: cerrando mensaje: %w", err)
	}
	return c.Quit()
}

// ----------------------------------------------------------------------------

// enlaceReset arma URL_BASE_RESET?token=<token>. Sin URL base devuelve "".
func enlaceReset(urlBase, token string) string {
	if urlBase == "" {
		return ""
	}
	sep := "?"
	if strings.Contains(urlBase, "?") {
		sep = "&"
	}
	return urlBase + sep + "token=" + url.QueryEscape(token)
}

func limpiarCabecera(s string) string {
	return strings.TrimSpace(strings.NewReplacer("\r", " ", "\n", " ").Replace(s))
}
