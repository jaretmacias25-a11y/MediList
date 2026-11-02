package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"medilist/internal/observability"
	"medilist/internal/service/entities"
)

// NotificacionConfig define el worker de outbox y los recordatorios.
type NotificacionConfig struct {
	Interval      time.Duration
	BatchSize     int
	MaxAttempts   int
	BackoffBase   time.Duration
	ReminderHours []int
	Now           func() time.Time
}

// NotificacionService construye notificaciones, administra el outbox y
// ejecuta recordatorios. Los cuerpos no incluyen datos clínicos sensibles.
type NotificacionService struct {
	store     NotificacionStore
	email     NotificacionEmailSender
	citas     CitaStore
	pacientes PacienteStore
	logger    *slog.Logger
	cfg       NotificacionConfig
	now       func() time.Time
}

// NewNotificacionService construye el servicio de notificaciones.
func NewNotificacionService(
	store NotificacionStore,
	email NotificacionEmailSender,
	citas CitaStore,
	pacientes PacienteStore,
	logger *slog.Logger,
	cfg NotificacionConfig,
) *NotificacionService {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 30 * time.Second
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 50
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 5
	}
	if cfg.BackoffBase <= 0 {
		cfg.BackoffBase = 30 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &NotificacionService{
		store: store, email: email, citas: citas, pacientes: pacientes,
		logger: logger, cfg: cfg, now: now,
	}
}

// CitaCreada construye la notificación de una cita nueva.
func (s *NotificacionService) CitaCreada(
	cita *entities.Cita, paciente *entities.Paciente,
) *entities.Notificacion {
	if cita == nil || paciente == nil {
		return nil
	}
	return s.notificacionCita(
		entities.NotificacionCitaCreada,
		fmt.Sprintf("cita_creada:%d", cita.ID),
		"Tu cita en MediList",
		"Tu cita quedó registrada",
		cita, paciente,
	)
}

// CitaConfirmada construye la notificación de confirmación.
func (s *NotificacionService) CitaConfirmada(
	cita *entities.Cita, paciente *entities.Paciente,
) *entities.Notificacion {
	if cita == nil || paciente == nil {
		return nil
	}
	return s.notificacionCita(
		entities.NotificacionCitaConfirmada,
		fmt.Sprintf("cita_confirmada:%d", cita.ID),
		"Tu cita fue confirmada",
		"Tu cita fue confirmada",
		cita, paciente,
	)
}

// CitaCancelada construye la notificación de cancelación.
func (s *NotificacionService) CitaCancelada(
	cita *entities.Cita, paciente *entities.Paciente,
) *entities.Notificacion {
	if cita == nil || paciente == nil {
		return nil
	}
	return s.notificacionCita(
		entities.NotificacionCitaCancelada,
		fmt.Sprintf("cita_cancelada:%d", cita.ID),
		"Tu cita fue cancelada",
		"Tu cita fue cancelada",
		cita, paciente,
	)
}

// CitaReprogramada construye la notificación de reprogramación.
func (s *NotificacionService) CitaReprogramada(
	cita *entities.Cita, paciente *entities.Paciente,
) *entities.Notificacion {
	if cita == nil || paciente == nil {
		return nil
	}
	return s.notificacionCita(
		entities.NotificacionCitaReprogramada,
		fmt.Sprintf("cita_reprogramada:%d:%d", cita.ID, cita.Version),
		"Tu cita fue reprogramada",
		"Tu cita fue reprogramada",
		cita, paciente,
	)
}

// DerivacionReservada notifica al médico cuando un paciente agenda con una
// derivación dirigida a él.
func (s *NotificacionService) DerivacionReservada(
	cita *entities.Cita, medico *entities.Medico,
) *entities.Notificacion {
	if cita == nil || medico == nil || strings.TrimSpace(medico.Persona.Email) == "" {
		return nil
	}
	cuerpo := fmt.Sprintf(
		"Hola %s:\n\nEl paciente %s agendó una cita con tu derivación.\n"+
			"Fecha: %s\nEspecialidad: %s\n\n"+
			"Puedes revisarla en la agenda de MediList.\n",
		medico.Persona.Nombres,
		cita.PacienteNombre,
		cita.FechaHoraInicio.Format(time.RFC3339),
		cita.EspecialidadNombre,
	)
	return &entities.Notificacion{
		Tipo:         entities.NotificacionDerivacionReservada,
		Destinatario: medico.Persona.Email,
		Nombre:       medico.Persona.Nombres,
		Asunto:       "Nueva cita con derivación",
		Cuerpo:       cuerpo,
		DedupeKey:    fmt.Sprintf("derivacion_reservada:%d", cita.ID),
	}
}

// DerivacionEmitida construye la notificación al paciente.
func (s *NotificacionService) DerivacionEmitida(
	derivacion *entities.Derivacion, paciente *entities.Paciente,
) *entities.Notificacion {
	if derivacion == nil || paciente == nil || strings.TrimSpace(paciente.Persona.Email) == "" {
		return nil
	}
	cuerpo := fmt.Sprintf(
		"Hola %s:\n\n"+
			"Se emitió una derivación para ti.\n"+
			"Especialidad destino: %s\n"+
			"Vigente hasta: %s\n\n"+
			"Puedes consultar y usar la derivación desde el portal de MediList.\n",
		paciente.Persona.Nombres,
		derivacion.EspecialidadDestino,
		derivacion.VigenteHasta.Format(time.RFC3339),
	)
	return &entities.Notificacion{
		Tipo:         entities.NotificacionDerivacionEmitida,
		UsuarioID:    paciente.UsuarioID,
		Destinatario: paciente.Persona.Email,
		Nombre:       paciente.Persona.Nombres,
		Asunto:       "Nueva derivación en MediList",
		Cuerpo:       cuerpo,
		DedupeKey:    fmt.Sprintf("derivacion_emitida:%d", derivacion.ID),
	}
}

func (s *NotificacionService) notificacionCita(
	tipo entities.TipoNotificacion,
	dedupe, asunto, titulo string,
	cita *entities.Cita,
	paciente *entities.Paciente,
) *entities.Notificacion {
	if strings.TrimSpace(paciente.Persona.Email) == "" {
		return nil
	}
	codigo := ""
	if strings.TrimSpace(cita.Codigo) != "" {
		codigo = fmt.Sprintf("Código: %s\n", cita.Codigo)
	}
	cuerpo := fmt.Sprintf(
		"Hola %s:\n\n%s.\n\n%s"+
			"Fecha: %s\nProfesional: %s\nEspecialidad: %s\n\n"+
			"Puedes consultar tu cita desde el portal de MediList.\n",
		paciente.Persona.Nombres,
		titulo,
		codigo,
		cita.FechaHoraInicio.Format(time.RFC3339),
		cita.MedicoNombre,
		cita.EspecialidadNombre,
	)
	return &entities.Notificacion{
		Tipo:         tipo,
		UsuarioID:    paciente.UsuarioID,
		Destinatario: paciente.Persona.Email,
		Nombre:       paciente.Persona.Nombres,
		Asunto:       asunto,
		Cuerpo:       cuerpo,
		DedupeKey:    dedupe,
	}
}

// ObtenerPreferencias devuelve las preferencias de la cuenta autenticada.
func (s *NotificacionService) ObtenerPreferencias(
	ctx context.Context,
	actor *Actor,
) (*entities.PreferenciasNotificacion, error) {
	if actor == nil {
		return nil, fmt.Errorf("%w: actor no válido", entities.ErrForbidden)
	}
	return s.store.ObtenerPreferencias(ctx, actor.UsuarioID)
}

// ActualizarPreferencias guarda las preferencias de la cuenta autenticada.
func (s *NotificacionService) ActualizarPreferencias(
	ctx context.Context,
	actor *Actor,
	emailHabilitado, recordatoriosHabilitados bool,
) (*entities.PreferenciasNotificacion, error) {
	if actor == nil {
		return nil, fmt.Errorf("%w: actor no válido", entities.ErrForbidden)
	}
	preferencias := &entities.PreferenciasNotificacion{
		UsuarioID:                actor.UsuarioID,
		EmailHabilitado:          emailHabilitado,
		RecordatoriosHabilitados: recordatoriosHabilitados,
	}
	if err := s.store.ActualizarPreferencias(ctx, preferencias); err != nil {
		return nil, err
	}
	return preferencias, nil
}

// Encolar inserta una notificación en el outbox.
func (s *NotificacionService) Encolar(ctx context.Context, n *entities.Notificacion) error {
	if n == nil {
		return nil
	}
	return s.store.Encolar(ctx, n)
}

// Procesar reclama y envía un lote de notificaciones pendientes.
func (s *NotificacionService) Procesar(ctx context.Context) error {
	if s.email == nil {
		return nil
	}
	lista, err := s.store.ReclamarPendientes(ctx, s.now(), s.cfg.BatchSize)
	if err != nil {
		return err
	}
	for i := range lista {
		n := &lista[i]
		if n.UsuarioID != nil {
			preferencias, err := s.store.ObtenerPreferencias(ctx, *n.UsuarioID)
			if err == nil {
				omitir := !preferencias.EmailHabilitado ||
					(n.Tipo == entities.NotificacionRecordatorio &&
						!preferencias.RecordatoriosHabilitados)
				if omitir {
					if err := s.store.MarcarEnviada(ctx, n.ID, s.now()); err != nil {
						s.logger.Error("no se pudo omitir notificación", "id", n.ID, "error", err)
					}
					continue
				}
			}
		}
		if err := s.email.EnviarNotificacion(ctx, n); err != nil {
			intentos := n.Intentos + 1
			backoff := s.cfg.BackoffBase * time.Duration(1<<min(intentos, 8))
			if backoff > time.Hour {
				backoff = time.Hour
			}
			if errMarca := s.store.MarcarFallo(
				ctx, n.ID, err.Error(), s.now().Add(backoff), s.cfg.MaxAttempts,
			); errMarca != nil {
				s.logger.Error("no se pudo marcar fallo de notificación", "id", n.ID, "error", errMarca)
			}
			observability.IncFalloEmail()
			s.logger.Warn("falló el envío de notificación", "id", n.ID, "tipo", n.Tipo, "error", err)
			continue
		}
		observability.IncNotificacionOK()
		if err := s.store.MarcarEnviada(ctx, n.ID, s.now()); err != nil {
			s.logger.Error("no se pudo marcar notificación enviada", "id", n.ID, "error", err)
		}
	}
	return nil
}

// ProcesarRecordatorios encola recordatorios para las citas próximas.
func (s *NotificacionService) ProcesarRecordatorios(ctx context.Context) error {
	if len(s.cfg.ReminderHours) == 0 || s.citas == nil || s.pacientes == nil {
		return nil
	}
	maxHoras := 0
	for _, horas := range s.cfg.ReminderHours {
		if horas > maxHoras {
			maxHoras = horas
		}
	}
	if maxHoras <= 0 {
		return nil
	}
	ahora := s.now()
	desde := ahora
	hasta := ahora.Add(time.Duration(maxHoras) * time.Hour)
	lista, err := s.citas.Listar(ctx, entities.CitaFiltro{
		ListOptions: entities.ListOptions{Limit: 500},
		Desde:       &desde,
		Hasta:       &hasta,
	})
	if err != nil {
		return err
	}
	for _, cita := range lista {
		if cita.Estado != entities.EstadoCitaProgramada &&
			cita.Estado != entities.EstadoCitaConfirmada {
			continue
		}
		for _, horas := range s.cfg.ReminderHours {
			if horas <= 0 {
				continue
			}
			objetivo := cita.FechaHoraInicio.Add(-time.Duration(horas) * time.Hour)
			if ahora.Before(objetivo) || !cita.FechaHoraInicio.After(ahora) {
				continue
			}
			paciente, err := s.pacientes.ObtenerPorID(ctx, cita.PacienteID)
			if err != nil || strings.TrimSpace(paciente.Persona.Email) == "" {
				continue
			}
			notificacion := s.notificacionCita(
				entities.NotificacionRecordatorio,
				fmt.Sprintf("recordatorio:%dh:%d", horas, cita.ID),
				fmt.Sprintf("Recordatorio de cita en %d horas", horas),
				fmt.Sprintf("Tu cita es en aproximadamente %d horas", horas),
				&cita, paciente,
			)
			if notificacion == nil {
				continue
			}
			if err := s.store.Encolar(ctx, notificacion); err != nil {
				s.logger.Error("no se pudo encolar recordatorio", "cita_id", cita.ID, "error", err)
			}
		}
	}
	return nil
}

// Run ejecuta el worker hasta que se cancele el contexto.
func (s *NotificacionService) Run(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()
	s.procesarCiclo(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.procesarCiclo(ctx)
		}
	}
}

func (s *NotificacionService) procesarCiclo(ctx context.Context) {
	if err := s.ProcesarRecordatorios(ctx); err != nil {
		s.logger.Error("error al procesar recordatorios", "error", err)
	}
	if err := s.Procesar(ctx); err != nil {
		s.logger.Error("error al procesar outbox", "error", err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
