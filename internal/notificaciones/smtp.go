package notificaciones

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strings"
	"time"

	"medilist/internal/service/entities"
)

// SMTPEmailSender envía correos mediante un servidor SMTP. No registra
// destinatarios, tokens ni cuerpos en logs.
type SMTPEmailSender struct {
	host   string
	port   int
	user   string
	pass   string
	from   string
	logger *slog.Logger
}

// NewSMTPEmailSender construye el proveedor SMTP.
func NewSMTPEmailSender(
	host string,
	port int,
	user, pass, from string,
	logger *slog.Logger,
) (*SMTPEmailSender, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, fmt.Errorf("SMTP_HOST no puede estar vacío")
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("SMTP_PORT no es válido")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &SMTPEmailSender{
		host: host, port: port, user: strings.TrimSpace(user),
		pass: pass, from: from, logger: logger,
	}, nil
}

func (s *SMTPEmailSender) EnviarVerificacion(
	ctx context.Context,
	destino, nombre, enlace string,
	expira time.Time,
) error {
	asunto := "Verifica tu cuenta de MediList"
	cuerpo := fmt.Sprintf(
		"Hola %s:\n\nPara activar tu cuenta abre este enlace:\n\n%s\n\nEl enlace vence el %s.\n",
		nombre, enlace, expira.Format(time.RFC3339),
	)
	return s.enviar(ctx, "verificacion_email", destino, asunto, cuerpo)
}

func (s *SMTPEmailSender) EnviarResetPassword(
	ctx context.Context,
	destino, nombre, enlace string,
	expira time.Time,
) error {
	asunto := "Recupera tu contraseña de MediList"
	cuerpo := fmt.Sprintf(
		"Hola %s:\n\nAbre este enlace para restablecer tu contraseña:\n\n%s\n\nEl enlace vence el %s.\n",
		nombre, enlace, expira.Format(time.RFC3339),
	)
	return s.enviar(ctx, "reset_password", destino, asunto, cuerpo)
}

func (s *SMTPEmailSender) EnviarCitaCreada(
	ctx context.Context,
	destino, nombre string,
	cita *entities.Cita,
) error {
	if cita == nil {
		return fmt.Errorf("cita vacía para notificación")
	}
	asunto := "Tu cita en MediList"
	cuerpo := fmt.Sprintf(
		"Hola %s:\n\nTu cita quedó registrada.\nCódigo: %s\nFecha: %s\nProfesional: %s\nEspecialidad: %s\n",
		nombre, cita.Codigo, cita.FechaHoraInicio.Format(time.RFC3339),
		cita.MedicoNombre, cita.EspecialidadNombre,
	)
	return s.enviar(ctx, "cita", destino, asunto, cuerpo)
}

func (s *SMTPEmailSender) EnviarNotificacion(
	ctx context.Context,
	n *entities.Notificacion,
) error {
	if n == nil {
		return fmt.Errorf("notificación vacía")
	}
	return s.enviar(ctx, string(n.Tipo), n.Destinatario, n.Asunto, n.Cuerpo)
}

func (s *SMTPEmailSender) enviar(
	ctx context.Context,
	tipo, destino, asunto, cuerpo string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(destino) == "" {
		return fmt.Errorf("destinatario de correo vacío")
	}
	addr := net.JoinHostPort(s.host, fmt.Sprintf("%d", s.port))
	var auth smtp.Auth
	if s.user != "" {
		auth = smtp.PlainAuth("", s.user, s.pass, s.host)
	}
	mensaje := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMIME-Version: 1.0\r\n"+
			"Content-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		s.from, destino, asunto, time.Now().Format(time.RFC1123Z), cuerpo,
	)
	if err := smtp.SendMail(addr, auth, s.from, []string{destino}, []byte(mensaje)); err != nil {
		return fmt.Errorf("enviar correo SMTP: %w", err)
	}
	s.logger.Info("correo enviado por SMTP", "tipo", tipo)
	return nil
}
