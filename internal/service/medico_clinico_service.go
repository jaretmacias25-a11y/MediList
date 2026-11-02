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

// MedicoClinicoConfig define la vigencia de las derivaciones y el reloj
// inyectable para las pruebas.
type MedicoClinicoConfig struct {
	ReferralValidDays int
	Now               func() time.Time
}

// NotaCitaInput contiene la nota mínima editable por el médico.
type NotaCitaInput struct {
	Motivo     string
	Evaluacion string
	Plan       string
	Version    int64
}

// DerivacionInput contiene el destino clínico y el motivo de la derivación.
type DerivacionInput struct {
	EspecialidadDestinoID int64
	MedicoDestinoID       *int64
	Motivo                string
	Prioridad             entities.PrioridadDerivacion
}

// MedicoClinicoService contiene las acciones clínicas del médico sobre sus
// propias citas: estados, nota y derivaciones.
type MedicoClinicoService struct {
	citas             CitaStore
	notas             NotaCitaStore
	derivaciones      DerivacionStore
	medicos           MedicoStore
	especialidades    EspecialidadStore
	pacientes         PacienteStore
	notificaciones    *NotificacionService
	referralValidDays int
	now               func() time.Time
}

// NewMedicoClinicoService construye el servicio clínico del médico.
func NewMedicoClinicoService(
	citas CitaStore,
	notas NotaCitaStore,
	derivaciones DerivacionStore,
	medicos MedicoStore,
	especialidades EspecialidadStore,
	cfg MedicoClinicoConfig,
) *MedicoClinicoService {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	dias := cfg.ReferralValidDays
	if dias <= 0 {
		dias = 90
	}
	return &MedicoClinicoService{
		citas:             citas,
		notas:             notas,
		derivaciones:      derivaciones,
		medicos:           medicos,
		especialidades:    especialidades,
		referralValidDays: dias,
		now:               now,
	}
}

// SetNotificaciones inyecta el outbox y el acceso a pacientes para las
// notificaciones clínicas.
func (s *MedicoClinicoService) SetNotificaciones(
	n *NotificacionService,
	pacientes PacienteStore,
) {
	s.notificaciones = n
	s.pacientes = pacientes
}

// CambiarEstado aplica las transiciones clínicas permitidas al médico de la
// cita. No permite cancelar ni reprogramar desde este flujo.
func (s *MedicoClinicoService) CambiarEstado(
	ctx context.Context,
	actor *Actor,
	citaID int64,
	nuevo entities.EstadoCita,
) (*entities.Cita, error) {
	cita, err := s.citaPropia(ctx, actor, citaID)
	if err != nil {
		return nil, err
	}
	if !nuevo.Valido() {
		return nil, entities.NewValidationError("estado de cita no válido")
	}
	if cita.Estado == nuevo {
		return cita, nil
	}
	if !puedeMedicoCambiarA(cita.Estado, nuevo) {
		return nil, entities.NewValidationError(
			"transición de estado no permitida para el médico: %s → %s",
			cita.Estado, nuevo)
	}
	cita.Estado = nuevo
	if s.notificaciones != nil && nuevo == entities.EstadoCitaConfirmada {
		paciente, err := s.pacientes.ObtenerPorID(ctx, cita.PacienteID)
		if err != nil {
			return nil, err
		}
		cita.NotificacionPendiente = s.notificaciones.CitaConfirmada(cita, paciente)
	}
	var errActualizar error
	if cita.DerivacionID != nil &&
		(nuevo == entities.EstadoCitaAtendida || nuevo == entities.EstadoCitaNoAsistio) {
		estadoDerivacion := entities.EstadoDerivacionAtendida
		motivoDerivacion := ""
		if nuevo == entities.EstadoCitaNoAsistio {
			estadoDerivacion = entities.EstadoDerivacionCancelada
			motivoDerivacion = "no_show"
		}
		errActualizar = s.citas.CambiarEstadoConDerivacion(
			ctx, cita, estadoDerivacion, motivoDerivacion)
	} else {
		errActualizar = s.citas.Actualizar(ctx, cita)
	}
	if errActualizar != nil {
		return nil, errActualizar
	}
	return s.citas.ObtenerPorID(ctx, citaID)
}

// ObtenerNota devuelve la nota de una cita propia.
func (s *MedicoClinicoService) ObtenerNota(
	ctx context.Context,
	actor *Actor,
	citaID int64,
) (*entities.NotaCita, error) {
	if _, err := s.citaPropia(ctx, actor, citaID); err != nil {
		return nil, err
	}
	return s.notas.ObtenerPorCita(ctx, citaID)
}

// GuardarNota crea o actualiza la nota de una cita atendida. La actualización
// exige la versión de la nota para evitar pérdidas concurrentes.
func (s *MedicoClinicoService) GuardarNota(
	ctx context.Context,
	actor *Actor,
	citaID int64,
	in NotaCitaInput,
) (*entities.NotaCita, error) {
	cita, err := s.citaPropia(ctx, actor, citaID)
	if err != nil {
		return nil, err
	}
	if cita.Estado != entities.EstadoCitaAtendida {
		return nil, entities.NewValidationError(
			"la nota solo puede registrarse en una cita atendida")
	}
	motivo, err := textoNota(in.Motivo, "motivo")
	if err != nil {
		return nil, err
	}
	evaluacion, err := textoNota(in.Evaluacion, "evaluacion")
	if err != nil {
		return nil, err
	}
	plan, err := textoNota(in.Plan, "plan")
	if err != nil {
		return nil, err
	}

	actual, err := s.notas.ObtenerPorCita(ctx, citaID)
	if errors.Is(err, entities.ErrNotFound) {
		nota := &entities.NotaCita{
			CitaID:     citaID,
			MedicoID:   cita.MedicoID,
			Motivo:     motivo,
			Evaluacion: evaluacion,
			Plan:       plan,
		}
		if err := s.notas.Crear(ctx, nota); err != nil {
			return nil, err
		}
		return s.notas.ObtenerPorCita(ctx, citaID)
	}
	if err != nil {
		return nil, err
	}
	if in.Version <= 0 {
		return nil, entities.NewValidationError("version es obligatoria para actualizar la nota")
	}
	if in.Version != actual.Version {
		return nil, fmt.Errorf(
			"%w: la nota fue modificada por otra operación; vuelve a consultarla e inténtalo de nuevo",
			entities.ErrConflict)
	}
	actual.Motivo = motivo
	actual.Evaluacion = evaluacion
	actual.Plan = plan
	if err := s.notas.Actualizar(ctx, actual); err != nil {
		return nil, err
	}
	return s.notas.ObtenerPorCita(ctx, citaID)
}

// CrearDerivacion emite una derivación desde una cita de Medicina General
// atendida. El paciente y el médico de origen salen de la cita y del actor.
func (s *MedicoClinicoService) CrearDerivacion(
	ctx context.Context,
	actor *Actor,
	citaID int64,
	in DerivacionInput,
) (*entities.Derivacion, error) {
	cita, err := s.citaPropia(ctx, actor, citaID)
	if err != nil {
		return nil, err
	}
	if cita.Estado != entities.EstadoCitaAtendida {
		return nil, entities.NewValidationError(
			"la cita de origen debe estar atendida para derivar")
	}
	medicoID, err := actor.MedicoVinculado()
	if err != nil {
		return nil, err
	}
	medico, err := s.medicos.ObtenerPorID(ctx, medicoID)
	if err != nil {
		return nil, err
	}
	if !medico.Activo {
		return nil, entities.NewValidationError("el médico de origen está inactivo")
	}
	if medico.Especialidad.Codigo != "MED-GEN" {
		return nil, entities.NewValidationError(
			"la derivación solo puede originarse en Medicina General")
	}
	if in.EspecialidadDestinoID <= 0 {
		return nil, entities.NewValidationError("especialidad_destino_id es obligatorio")
	}
	destino, err := s.especialidades.ObtenerPorID(ctx, in.EspecialidadDestinoID)
	if err != nil {
		if errors.Is(err, entities.ErrNotFound) {
			return nil, entities.NewValidationError(
				"la especialidad destino %d no existe", in.EspecialidadDestinoID)
		}
		return nil, err
	}
	if !destino.Activa || !destino.RequiereDerivacion {
		return nil, entities.NewValidationError(
			"la especialidad destino debe estar activa y requerir derivación")
	}
	if in.MedicoDestinoID != nil {
		if *in.MedicoDestinoID <= 0 {
			return nil, entities.NewValidationError("medico_destino_id no es válido")
		}
		medicoDestino, err := s.medicos.ObtenerPorID(ctx, *in.MedicoDestinoID)
		if err != nil {
			if errors.Is(err, entities.ErrNotFound) {
				return nil, entities.NewValidationError(
					"el médico destino %d no existe", *in.MedicoDestinoID)
			}
			return nil, err
		}
		if !medicoDestino.Activo || medicoDestino.EspecialidadID != destino.ID {
			return nil, entities.NewValidationError(
				"el médico destino debe estar activo y pertenecer a la especialidad destino")
		}
	}
	motivo := strings.TrimSpace(in.Motivo)
	if motivo == "" {
		return nil, entities.NewValidationError("motivo de la derivación es obligatorio")
	}
	if utf8.RuneCountInString(motivo) > 2000 {
		return nil, entities.NewValidationError("motivo no puede superar los 2000 caracteres")
	}
	prioridad := in.Prioridad
	if prioridad == "" {
		prioridad = entities.PrioridadDerivacionNormal
	}
	if !prioridad.Valida() {
		return nil, entities.NewValidationError("prioridad de derivación no válida")
	}

	ahora := s.now()
	derivacion := &entities.Derivacion{
		PacienteID:            cita.PacienteID,
		PacienteNombre:        cita.PacienteNombre,
		MedicoOrigenID:        medicoID,
		MedicoOrigenNombre:    cita.MedicoNombre,
		CitaOrigenID:          citaID,
		EspecialidadDestinoID: destino.ID,
		EspecialidadDestino:   destino.Nombre,
		MedicoDestinoID:       in.MedicoDestinoID,
		Motivo:                motivo,
		Prioridad:             prioridad,
		Estado:                entities.EstadoDerivacionPendiente,
		VigenteDesde:          ahora,
		VigenteHasta:          ahora.AddDate(0, 0, s.referralValidDays),
	}
	if s.notificaciones != nil && s.pacientes != nil {
		paciente, err := s.pacientes.ObtenerPorID(ctx, cita.PacienteID)
		if err != nil {
			return nil, err
		}
		derivacion.NotificacionPendiente = s.notificaciones.DerivacionEmitida(derivacion, paciente)
	}
	if err := s.derivaciones.Crear(ctx, derivacion); err != nil {
		return nil, err
	}
	observability.IncDerivacion()
	return s.derivaciones.ObtenerPorID(ctx, derivacion.ID)
}

// ListarDerivaciones lista solo las derivaciones emitidas por el médico.
func (s *MedicoClinicoService) ListarDerivaciones(
	ctx context.Context,
	actor *Actor,
	opts entities.ListOptions,
) ([]entities.Derivacion, error) {
	if actor == nil {
		return nil, fmt.Errorf("%w: actor no válido", entities.ErrForbidden)
	}
	medicoID, err := actor.MedicoVinculado()
	if err != nil {
		return nil, err
	}
	opts = opts.Normalized()
	opts.Search = strings.TrimSpace(opts.Search)
	return s.derivaciones.ListarPorMedicoOrigen(ctx, medicoID, opts)
}

func (s *MedicoClinicoService) citaPropia(
	ctx context.Context,
	actor *Actor,
	citaID int64,
) (*entities.Cita, error) {
	if actor == nil {
		return nil, fmt.Errorf("%w: actor no válido", entities.ErrForbidden)
	}
	medicoID, err := actor.MedicoVinculado()
	if err != nil {
		return nil, err
	}
	if citaID <= 0 {
		return nil, entities.NewValidationError("id de cita inválido")
	}
	cita, err := s.citas.ObtenerPorID(ctx, citaID)
	if err != nil {
		return nil, err
	}
	if cita.MedicoID != medicoID {
		return nil, fmt.Errorf("%w: la cita no pertenece al médico autenticado", entities.ErrForbidden)
	}
	return cita, nil
}

func puedeMedicoCambiarA(actual, nuevo entities.EstadoCita) bool {
	switch actual {
	case entities.EstadoCitaProgramada:
		return nuevo == entities.EstadoCitaConfirmada ||
			nuevo == entities.EstadoCitaAtendida ||
			nuevo == entities.EstadoCitaNoAsistio
	case entities.EstadoCitaConfirmada:
		return nuevo == entities.EstadoCitaAtendida ||
			nuevo == entities.EstadoCitaNoAsistio
	default:
		return false
	}
}

func textoNota(texto, campo string) (string, error) {
	texto = strings.TrimSpace(texto)
	if utf8.RuneCountInString(texto) > 5000 {
		return "", entities.NewValidationError("%s no puede superar los 5000 caracteres", campo)
	}
	return texto, nil
}
