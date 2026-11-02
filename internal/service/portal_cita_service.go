package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"medilist/internal/observability"
	"medilist/internal/service/entities"
)

// PortalCitaConfig define el corte de cancelación y el estado inicial de las
// citas creadas por el paciente.
type PortalCitaConfig struct {
	CancelCutoff time.Duration
	AutoConfirm  bool
	Now          func() time.Time
}

// PortalCitaInput contiene los datos que el paciente puede enviar al reservar.
// El paciente y el canal nunca se toman de este contrato.
type PortalCitaInput struct {
	MedicoID        int64
	FechaHoraInicio time.Time
	Tipo            entities.TipoCita
	Motivo          string
	DerivacionID    *int64
}

// PortalCitaReprogramarInput solo permite cambiar el slot y, opcionalmente,
// el motivo. El médico, la especialidad y el paciente se conservan.
type PortalCitaReprogramarInput struct {
	FechaHoraInicio time.Time
	Motivo          *string
	Version         *int64
}

// PortalCitaService implementa la agenda propia del paciente. Todas las
// operaciones fuerzan el paciente desde el Actor y nunca desde el body/query.
type PortalCitaService struct {
	citas          CitaStore
	pacientes      PacienteStore
	medicos        MedicoStore
	derivaciones   DerivacionStore
	slots          SlotValidator
	email          EmailSender
	notificaciones *NotificacionService
	cancelCutoff   time.Duration
	autoConfirm    bool
	now            func() time.Time
}

// NewPortalCitaService construye el servicio de citas del portal.
func NewPortalCitaService(
	citas CitaStore,
	pacientes PacienteStore,
	medicos MedicoStore,
	derivaciones DerivacionStore,
	slots SlotValidator,
	cfg PortalCitaConfig,
	email ...EmailSender,
) *PortalCitaService {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	cutoff := cfg.CancelCutoff
	if cutoff < 0 {
		cutoff = 0
	}
	var notificador EmailSender
	if len(email) > 0 {
		notificador = email[0]
	}
	return &PortalCitaService{
		citas:        citas,
		pacientes:    pacientes,
		medicos:      medicos,
		derivaciones: derivaciones,
		slots:        slots,
		email:        notificador,
		cancelCutoff: cutoff,
		autoConfirm:  cfg.AutoConfirm,
		now:          now,
	}
}

// SetNotificaciones inyecta el servicio de outbox sin cambiar el constructor
// usado por las pruebas.
func (s *PortalCitaService) SetNotificaciones(n *NotificacionService) {
	s.notificaciones = n
}

// Crear agenda una cita para el paciente autenticado. El slot se valida contra
// la disponibilidad real y la base sigue siendo la última defensa de carrera.
func (s *PortalCitaService) Crear(
	ctx context.Context,
	actor *Actor,
	in PortalCitaInput,
) (*entities.Cita, error) {
	paciente, err := s.pacientePropio(ctx, actor)
	if err != nil {
		return nil, err
	}
	if in.FechaHoraInicio.IsZero() {
		return nil, entities.NewValidationError("fecha_hora_inicio es obligatoria (formato RFC3339)")
	}
	medico, err := s.medicoPortalDisponible(ctx, in.MedicoID)
	if err != nil {
		return nil, err
	}
	if in.DerivacionID == nil && medico.Especialidad.RequiereDerivacion {
		return nil, fmt.Errorf(
			"%w: la especialidad %s requiere una derivación vigente",
			entities.ErrRequiereDerivacion, medico.Especialidad.Nombre)
	}
	if in.DerivacionID != nil {
		if !medico.Especialidad.RequiereDerivacion {
			return nil, entities.NewValidationError(
				"la especialidad seleccionada no requiere derivación")
		}
		if _, err := s.validarDerivacionReserva(
			ctx, *in.DerivacionID, paciente.ID, medico, false,
		); err != nil {
			return nil, err
		}
	}
	tipo, err := tipoCitaPortal(in.Tipo)
	if err != nil {
		return nil, err
	}
	motivo, err := motivoPortal(in.Motivo)
	if err != nil {
		return nil, err
	}
	slot, err := s.slots.ValidarSlot(ctx, medico.ID, in.FechaHoraInicio, 0)
	if err != nil {
		if errors.Is(err, entities.ErrConflict) {
			observability.IncConflictoSlot()
		}
		return nil, err
	}
	if slot == nil || slot.DuracionMin <= 0 {
		observability.IncConflictoSlot()
		return nil, fmt.Errorf("%w: el slot no está disponible", entities.ErrConflict)
	}

	estado := entities.EstadoCitaProgramada
	if s.autoConfirm {
		estado = entities.EstadoCitaConfirmada
	}
	usuarioID := actor.UsuarioID
	cita := &entities.Cita{
		PacienteID:         paciente.ID,
		PacienteNombre:     paciente.Persona.NombreCompleto(),
		MedicoID:           medico.ID,
		MedicoNombre:       medico.Persona.NombreCompleto(),
		EspecialidadNombre: medico.Especialidad.Nombre,
		FechaHoraInicio:    slot.Inicio,
		DuracionMin:        slot.DuracionMin,
		Tipo:               tipo,
		Motivo:             motivo,
		Estado:             estado,
		Canal:              entities.CanalCitaPortalPaciente,
		CreadaPorUsuarioID: &usuarioID,
		DerivacionID:       in.DerivacionID,
	}
	if s.notificaciones != nil {
		cita.NotificacionPendiente = s.notificaciones.CitaCreada(cita, paciente)
		if in.DerivacionID != nil {
			cita.NotificacionMedicoPendiente = s.notificaciones.DerivacionReservada(cita, medico)
		}
	}
	if in.DerivacionID != nil {
		if err := s.citas.CrearConDerivacion(ctx, cita, *in.DerivacionID); err != nil {
			if errors.Is(err, entities.ErrConflict) {
				observability.IncConflictoSlot()
			}
			return nil, err
		}
	} else if err := s.citas.Crear(ctx, cita); err != nil {
		if errors.Is(err, entities.ErrConflict) {
			observability.IncConflictoSlot()
		}
		return nil, err
	}
	observability.IncReserva()
	if in.DerivacionID != nil {
		observability.IncConversion()
	}
	creada, err := s.citas.ObtenerPorID(ctx, cita.ID)
	if err != nil {
		return nil, err
	}
	destino := strings.TrimSpace(paciente.Persona.Email)
	if destino == "" {
		destino = strings.TrimSpace(actor.Email)
	}
	if s.email != nil && destino != "" {
		nombre := strings.TrimSpace(paciente.Persona.Nombres)
		if nombre == "" {
			nombre = actor.Username
		}
		// La notificación es best-effort: un fallo de correo no revierte la
		// cita. La Fase 8 la reemplazará por el outbox transaccional.
		_ = s.email.EnviarCitaCreada(ctx, destino, nombre, creada)
	}
	return creada, nil
}

// Listar devuelve únicamente las citas del paciente autenticado.
func (s *PortalCitaService) Listar(
	ctx context.Context,
	actor *Actor,
	filtro entities.CitaFiltro,
) ([]entities.Cita, error) {
	paciente, err := s.pacientePropio(ctx, actor)
	if err != nil {
		return nil, err
	}
	filtro, err = prepararFiltroCitas(filtro)
	if err != nil {
		return nil, err
	}
	filtro.PacienteID = paciente.ID
	return s.citas.Listar(ctx, filtro)
}

// Obtener devuelve una cita solo si pertenece al paciente autenticado.
func (s *PortalCitaService) Obtener(
	ctx context.Context,
	actor *Actor,
	citaID int64,
) (*entities.Cita, error) {
	if citaID <= 0 {
		return nil, entities.NewValidationError("id de cita inválido")
	}
	return s.citaPropia(ctx, actor, citaID)
}

// Reprogramar cambia el slot de una cita propia. El médico, el paciente y la
// especialidad se conservan; la versión evita sobrescribir cambios ajenos.
func (s *PortalCitaService) Reprogramar(
	ctx context.Context,
	actor *Actor,
	citaID int64,
	in PortalCitaReprogramarInput,
) (*entities.Cita, error) {
	if citaID <= 0 {
		return nil, entities.NewValidationError("id de cita inválido")
	}
	cita, err := s.citaPropia(ctx, actor, citaID)
	if err != nil {
		return nil, err
	}
	if cita.Estado.EsTerminal() {
		return nil, entities.NewValidationError(
			"la cita está en estado %s y no puede reprogramarse", cita.Estado)
	}
	if in.Version == nil {
		return nil, entities.NewValidationError("version es obligatoria para reprogramar")
	}
	if *in.Version != cita.Version {
		return nil, fmt.Errorf(
			"%w: la cita fue modificada por otra operación; vuelve a consultarla e inténtalo de nuevo",
			entities.ErrConflict)
	}
	if in.FechaHoraInicio.IsZero() {
		return nil, entities.NewValidationError("fecha_hora_inicio es obligatoria (formato RFC3339)")
	}
	medico, err := s.medicoPortalDisponible(ctx, cita.MedicoID)
	if err != nil {
		return nil, err
	}
	if cita.DerivacionID != nil {
		if _, err := s.validarDerivacionReserva(
			ctx, *cita.DerivacionID, cita.PacienteID, medico, true,
		); err != nil {
			return nil, err
		}
	} else if medico.Especialidad.RequiereDerivacion {
		return nil, fmt.Errorf(
			"%w: la cita no tiene una derivación asociada", entities.ErrRequiereDerivacion)
	}
	slot, err := s.slots.ValidarSlot(ctx, cita.MedicoID, in.FechaHoraInicio, 0)
	if err != nil {
		return nil, err
	}
	if slot == nil || slot.DuracionMin <= 0 {
		return nil, fmt.Errorf("%w: el slot no está disponible", entities.ErrConflict)
	}

	cita.FechaHoraInicio = slot.Inicio
	cita.DuracionMin = slot.DuracionMin
	if in.Motivo != nil {
		motivo, err := motivoPortal(*in.Motivo)
		if err != nil {
			return nil, err
		}
		cita.Motivo = motivo
	}
	if s.notificaciones != nil {
		paciente, err := s.pacientePropio(ctx, actor)
		if err != nil {
			return nil, err
		}
		cita.NotificacionPendiente = s.notificaciones.CitaReprogramada(cita, paciente)
	}
	if err := s.citas.Actualizar(ctx, cita); err != nil {
		return nil, err
	}
	return s.citas.ObtenerPorID(ctx, citaID)
}

// Cancelar aplica la política de cancelación y registra el motivo. Es
// idempotente si la cita ya está cancelada.
func (s *PortalCitaService) Cancelar(
	ctx context.Context,
	actor *Actor,
	citaID int64,
	motivo string,
) (*entities.Cita, error) {
	if citaID <= 0 {
		return nil, entities.NewValidationError("id de cita inválido")
	}
	cita, err := s.citaPropia(ctx, actor, citaID)
	if err != nil {
		return nil, err
	}
	if cita.Estado == entities.EstadoCitaCancelada {
		return cita, nil
	}
	if cita.Estado.EsTerminal() {
		return nil, entities.NewValidationError(
			"la cita está en estado %s y no puede cancelarse", cita.Estado)
	}
	late := s.cancelCutoff > 0 &&
		s.now().Add(s.cancelCutoff).After(cita.FechaHoraInicio)
	if late && cita.DerivacionID == nil {
		return nil, fmt.Errorf(
			"%w: la cita está fuera del plazo de cancelación de %s",
			entities.ErrConflict, s.cancelCutoff)
	}
	motivo = strings.TrimSpace(motivo)
	if motivo == "" {
		motivo = "cancelacion_paciente"
	}
	if utf8.RuneCountInString(motivo) > 500 {
		return nil, entities.NewValidationError("motivo no puede superar los 500 caracteres")
	}

	cita.Estado = entities.EstadoCitaCancelada
	cita.MotivoCancelacion = motivo
	if s.notificaciones != nil {
		paciente, err := s.pacientePropio(ctx, actor)
		if err != nil {
			return nil, err
		}
		cita.NotificacionPendiente = s.notificaciones.CitaCancelada(cita, paciente)
	}
	var errCancelar error
	if cita.DerivacionID != nil {
		motivoDerivacion := ""
		if late {
			motivoDerivacion = "cancelacion_tardia"
		}
		errCancelar = s.citas.CancelarConDerivacion(ctx, cita, !late, motivoDerivacion)
	} else {
		errCancelar = s.citas.Cancelar(ctx, cita)
	}
	if errCancelar != nil {
		// Si otra solicitud canceló la cita entre la lectura y la escritura,
		// la operación es idempotente.
		if errors.Is(errCancelar, entities.ErrConflict) {
			reciente, errLectura := s.citas.ObtenerPorID(ctx, citaID)
			if errLectura == nil && reciente.Estado == entities.EstadoCitaCancelada {
				return reciente, nil
			}
		}
		return nil, errCancelar
	}
	observability.IncCancelacion()
	return s.citas.ObtenerPorID(ctx, citaID)
}

func (s *PortalCitaService) pacientePropio(
	ctx context.Context,
	actor *Actor,
) (*entities.Paciente, error) {
	if actor == nil {
		return nil, fmt.Errorf("%w: actor no válido", entities.ErrForbidden)
	}
	pacienteID, err := actor.PacienteVinculado()
	if err != nil {
		return nil, err
	}
	paciente, err := s.pacientes.ObtenerPorID(ctx, pacienteID)
	if err != nil {
		return nil, err
	}
	if !paciente.Activo ||
		paciente.UsuarioID == nil ||
		*paciente.UsuarioID != actor.UsuarioID {
		return nil, fmt.Errorf(
			"%w: el paciente no pertenece a la cuenta autenticada", entities.ErrForbidden)
	}
	return paciente, nil
}

func (s *PortalCitaService) citaPropia(
	ctx context.Context,
	actor *Actor,
	citaID int64,
) (*entities.Cita, error) {
	paciente, err := s.pacientePropio(ctx, actor)
	if err != nil {
		return nil, err
	}
	cita, err := s.citas.ObtenerPorID(ctx, citaID)
	if err != nil {
		return nil, err
	}
	if cita.PacienteID != paciente.ID {
		return nil, fmt.Errorf(
			"%w: la cita no pertenece al paciente autenticado", entities.ErrForbidden)
	}
	return cita, nil
}

func (s *PortalCitaService) medicoPortalDisponible(
	ctx context.Context,
	medicoID int64,
) (*entities.Medico, error) {
	if medicoID <= 0 {
		return nil, entities.NewValidationError("medico_id es obligatorio")
	}
	medico, err := s.medicos.ObtenerPorID(ctx, medicoID)
	if err != nil {
		return nil, err
	}
	if !medico.Activo || !medico.AceptaPortal || !medico.Especialidad.Activa {
		return nil, fmt.Errorf(
			"%w: el médico no está disponible para reservas del portal", entities.ErrNotFound)
	}
	return medico, nil
}

func (s *PortalCitaService) validarDerivacionReserva(
	ctx context.Context,
	derivacionID, pacienteID int64,
	medico *entities.Medico,
	permitirAgendada bool,
) (*entities.Derivacion, error) {
	if derivacionID <= 0 {
		return nil, entities.NewValidationError("derivacion_id no es válido")
	}
	if s.derivaciones == nil {
		return nil, fmt.Errorf("%w: no hay almacén de derivaciones disponible", entities.ErrConflict)
	}
	if err := s.derivaciones.MarcarVencidas(ctx, s.now()); err != nil {
		return nil, err
	}
	derivacion, err := s.derivaciones.ObtenerPorID(ctx, derivacionID)
	if err != nil {
		if errors.Is(err, entities.ErrNotFound) {
			return nil, fmt.Errorf(
				"%w: la derivación %d no existe o no está disponible",
				entities.ErrConflict, derivacionID)
		}
		return nil, err
	}
	if derivacion.PacienteID != pacienteID {
		return nil, fmt.Errorf(
			"%w: la derivación no pertenece al paciente autenticado", entities.ErrForbidden)
	}
	if derivacion.EspecialidadDestinoID != medico.EspecialidadID {
		return nil, fmt.Errorf(
			"%w: la especialidad de la derivación no coincide con el médico seleccionado",
			entities.ErrConflict)
	}
	if derivacion.MedicoDestinoID != nil && *derivacion.MedicoDestinoID != medico.ID {
		return nil, fmt.Errorf(
			"%w: la derivación está dirigida a otro médico", entities.ErrConflict)
	}
	estadoPermitido := derivacion.Estado == entities.EstadoDerivacionPendiente ||
		(permitirAgendada && derivacion.Estado == entities.EstadoDerivacionAgendada)
	if !estadoPermitido {
		return nil, fmt.Errorf(
			"%w: la derivación no está disponible para reservar (estado: %s)",
			entities.ErrConflict, derivacion.Estado)
	}
	if !derivacion.VigenteHasta.After(s.now()) {
		return nil, fmt.Errorf("%w: la derivación está vencida", entities.ErrConflict)
	}
	return derivacion, nil
}

// ListarDerivaciones devuelve solo las derivaciones del paciente autenticado.
func (s *PortalCitaService) ListarDerivaciones(
	ctx context.Context,
	actor *Actor,
	opts entities.ListOptions,
) ([]entities.Derivacion, error) {
	paciente, err := s.pacientePropio(ctx, actor)
	if err != nil {
		return nil, err
	}
	if s.derivaciones == nil {
		return nil, fmt.Errorf("%w: no hay almacén de derivaciones disponible", entities.ErrConflict)
	}
	if err := s.derivaciones.MarcarVencidas(ctx, s.now()); err != nil {
		return nil, err
	}
	opts = opts.Normalized()
	opts.Search = strings.TrimSpace(opts.Search)
	return s.derivaciones.ListarPorPaciente(ctx, paciente.ID, opts)
}

// ObtenerDerivacion devuelve una derivación solo si pertenece al paciente.
func (s *PortalCitaService) ObtenerDerivacion(
	ctx context.Context,
	actor *Actor,
	derivacionID int64,
) (*entities.Derivacion, error) {
	if derivacionID <= 0 {
		return nil, entities.NewValidationError("id de derivación inválido")
	}
	paciente, err := s.pacientePropio(ctx, actor)
	if err != nil {
		return nil, err
	}
	if s.derivaciones == nil {
		return nil, fmt.Errorf("%w: no hay almacén de derivaciones disponible", entities.ErrConflict)
	}
	if err := s.derivaciones.MarcarVencidas(ctx, s.now()); err != nil {
		return nil, err
	}
	derivacion, err := s.derivaciones.ObtenerPorID(ctx, derivacionID)
	if err != nil {
		return nil, err
	}
	if derivacion.PacienteID != paciente.ID {
		return nil, fmt.Errorf(
			"%w: la derivación no pertenece al paciente autenticado", entities.ErrForbidden)
	}
	return derivacion, nil
}

func tipoCitaPortal(tipo entities.TipoCita) (entities.TipoCita, error) {
	if tipo == "" {
		return entities.TipoCitaPrimeraVez, nil
	}
	switch tipo {
	case entities.TipoCitaPrimeraVez, entities.TipoCitaControl, entities.TipoCitaTelemedicina:
		return tipo, nil
	default:
		return "", entities.NewValidationError(
			"tipo debe ser primera_vez, control o telemedicina")
	}
}

func motivoPortal(motivo string) (string, error) {
	motivo = strings.TrimSpace(motivo)
	if utf8.RuneCountInString(motivo) > 1000 {
		return "", entities.NewValidationError("motivo no puede superar los 1000 caracteres")
	}
	return motivo, nil
}
