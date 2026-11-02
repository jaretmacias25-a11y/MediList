package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

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
	citas        CitaStore
	pacientes    PacienteStore
	medicos      MedicoStore
	slots        SlotValidator
	email        EmailSender
	cancelCutoff time.Duration
	autoConfirm  bool
	now          func() time.Time
}

// NewPortalCitaService construye el servicio de citas del portal.
func NewPortalCitaService(
	citas CitaStore,
	pacientes PacienteStore,
	medicos MedicoStore,
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
		slots:        slots,
		email:        notificador,
		cancelCutoff: cutoff,
		autoConfirm:  cfg.AutoConfirm,
		now:          now,
	}
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
	medico, err := s.validarMedicoDirecto(ctx, in.MedicoID)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	if slot == nil || slot.DuracionMin <= 0 {
		return nil, fmt.Errorf("%w: el slot no está disponible", entities.ErrConflict)
	}

	estado := entities.EstadoCitaProgramada
	if s.autoConfirm {
		estado = entities.EstadoCitaConfirmada
	}
	usuarioID := actor.UsuarioID
	cita := &entities.Cita{
		PacienteID:         paciente.ID,
		MedicoID:           medico.ID,
		FechaHoraInicio:    slot.Inicio,
		DuracionMin:        slot.DuracionMin,
		Tipo:               tipo,
		Motivo:             motivo,
		Estado:             estado,
		Canal:              entities.CanalCitaPortalPaciente,
		CreadaPorUsuarioID: &usuarioID,
	}
	if err := s.citas.Crear(ctx, cita); err != nil {
		return nil, err
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
	if _, err := s.validarMedicoDirecto(ctx, cita.MedicoID); err != nil {
		return nil, err
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
	if s.cancelCutoff > 0 && s.now().Add(s.cancelCutoff).After(cita.FechaHoraInicio) {
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
	if err := s.citas.Cancelar(ctx, cita); err != nil {
		// Si otra solicitud canceló la cita entre la lectura y la escritura,
		// la operación es idempotente.
		if errors.Is(err, entities.ErrConflict) {
			reciente, errLectura := s.citas.ObtenerPorID(ctx, citaID)
			if errLectura == nil && reciente.Estado == entities.EstadoCitaCancelada {
				return reciente, nil
			}
		}
		return nil, err
	}
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

func (s *PortalCitaService) validarMedicoDirecto(
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
	if medico.Especialidad.RequiereDerivacion {
		return nil, fmt.Errorf(
			"%w: la especialidad %s no es de acceso directo",
			entities.ErrRequiereDerivacion, medico.Especialidad.Nombre)
	}
	return medico, nil
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
