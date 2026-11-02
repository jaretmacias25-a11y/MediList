package notificaciones

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"medilist/internal/service/entities"
)

// DevEmailSender escribe los correos en un buzón local. Es un adaptador de
// desarrollo: no envía correo real y no debe usarse como proveedor de
// producción. El token solo queda en el archivo del buzón, nunca en los logs.
type DevEmailSender struct {
	dir    string
	from   string
	logger *slog.Logger
}

// NewDevEmailSender construye el buzón local y crea el directorio si falta.
func NewDevEmailSender(dir, from string, logger *slog.Logger) (*DevEmailSender, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, fmt.Errorf("EMAIL_OUTBOX_DIR no puede estar vacío")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("crear buzón de correo de desarrollo: %w", err)
	}
	if logger == nil {
		logger = slog.Default()
	}
	from = strings.TrimSpace(from)
	if from == "" {
		from = "MediList <no-reply@medilist.local>"
	}
	return &DevEmailSender{dir: dir, from: from, logger: logger}, nil
}

// EnviarVerificacion escribe el correo con el enlace de verificación.
func (s *DevEmailSender) EnviarVerificacion(
	ctx context.Context,
	destino, nombre, enlace string,
	expira time.Time,
) error {
	asunto := "Verifica tu cuenta de MediList"
	cuerpo := fmt.Sprintf(
		"Hola %s:\n\n"+
			"Para activar tu cuenta de MediList abre este enlace:\n\n%s\n\n"+
			"El enlace vence el %s.\n\n"+
			"Si no solicitaste esta cuenta, puedes ignorar este mensaje.\n",
		nombre, enlace, expira.Format(time.RFC3339),
	)
	return s.escribir(ctx, "verificacion_email", destino, asunto, cuerpo)
}

// EnviarResetPassword escribe el correo con el enlace de recuperación.
func (s *DevEmailSender) EnviarResetPassword(
	ctx context.Context,
	destino, nombre, enlace string,
	expira time.Time,
) error {
	asunto := "Recupera tu contraseña de MediList"
	cuerpo := fmt.Sprintf(
		"Hola %s:\n\n"+
			"Recibimos una solicitud para restablecer tu contraseña. Abre este enlace:\n\n%s\n\n"+
			"El enlace vence el %s. Si no solicitaste el cambio, ignora este mensaje.\n",
		nombre, enlace, expira.Format(time.RFC3339),
	)
	return s.escribir(ctx, "reset_password", destino, asunto, cuerpo)
}

// EnviarNotificacion escribe una entrada del outbox ya renderizada.
func (s *DevEmailSender) EnviarNotificacion(
	ctx context.Context,
	n *entities.Notificacion,
) error {
	if n == nil {
		return fmt.Errorf("notificación vacía")
	}
	return s.escribir(ctx, string(n.Tipo), n.Destinatario, n.Asunto, n.Cuerpo)
}

// EnviarCitaCreada escribe la confirmación básica de una cita del portal. La
// Fase 8 la sustituirá por una notificación transaccional en outbox.
func (s *DevEmailSender) EnviarCitaCreada(
	ctx context.Context,
	destino, nombre string,
	cita *entities.Cita,
) error {
	if cita == nil {
		return fmt.Errorf("cita vacía para notificación")
	}
	asunto := "Tu cita en MediList"
	cuerpo := fmt.Sprintf(
		"Hola %s:\n\n"+
			"Tu cita %s quedó registrada para el %s.\n"+
			"Profesional: %s\nEspecialidad: %s\n\n"+
			"Puedes consultar o cancelar tu cita desde el portal de MediList.\n",
		nombre,
		cita.Codigo,
		cita.FechaHoraInicio.Format(time.RFC3339),
		cita.MedicoNombre,
		cita.EspecialidadNombre,
	)
	return s.escribir(ctx, "cita", destino, asunto, cuerpo)
}

func (s *DevEmailSender) escribir(
	ctx context.Context,
	tipo, destino, asunto, cuerpo string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(destino) == "" {
		return fmt.Errorf("destinatario de correo vacío")
	}

	sufijo, err := randomHex(8)
	if err != nil {
		return err
	}
	nombreArchivo := fmt.Sprintf(
		"%s-%s-%s.eml",
		time.Now().UTC().Format("20060102T150405.000000000Z"),
		tipo,
		sufijo,
	)
	ruta := filepath.Join(s.dir, nombreArchivo)
	contenido := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMIME-Version: 1.0\r\n"+
			"Content-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		s.from, destino, asunto, time.Now().Format(time.RFC1123Z), cuerpo,
	)

	tmp, err := os.CreateTemp(s.dir, ".medilist-*.tmp")
	if err != nil {
		return fmt.Errorf("crear archivo temporal de correo: %w", err)
	}
	tmpNombre := tmp.Name()
	defer func() { _ = os.Remove(tmpNombre) }()

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("proteger archivo de correo: %w", err)
	}
	if _, err := tmp.WriteString(contenido); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("escribir correo: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("cerrar correo: %w", err)
	}
	if err := os.Rename(tmpNombre, ruta); err != nil {
		return fmt.Errorf("publicar correo en el buzón: %w", err)
	}

	s.logger.Info("correo de desarrollo escrito", "tipo", tipo, "archivo", nombreArchivo)
	return nil
}

func randomHex(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generar sufijo de correo: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
