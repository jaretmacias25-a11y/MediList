package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"medilist/internal/service/entities"
)

// CitaService contiene la lógica de negocio de la agenda.
type CitaService struct {
	store         CitaStore
	pacienteStore PacienteStore
	medicoStore   MedicoStore
}

// NewCitaService construye el servicio con sus almacenes.
func NewCitaService(store CitaStore, pacienteStore PacienteStore, medicoStore MedicoStore) *CitaService {
	return &CitaService{store: store, pacienteStore: pacienteStore, medicoStore: medicoStore}
}

func (s *CitaService) validarParticipantes(ctx context.Context, pacienteID, medicoID int64) error {
	paciente, err := s.pacienteStore.ObtenerPorID(ctx, pacienteID)
	if err != nil {
		if errors.Is(err, entities.ErrNotFound) {
			return entities.NewValidationError("el paciente %d no existe", pacienteID)
		}
		return err
	}
	if !paciente.Activo {
		return entities.NewValidationError("el paciente %d está inactivo", pacienteID)
	}

	medico, err := s.medicoStore.ObtenerPorID(ctx, medicoID)
	if err != nil {
		if errors.Is(err, entities.ErrNotFound) {
			return entities.NewValidationError("el médico %d no existe", medicoID)
		}
		return err
	}
	if !medico.Activo {
		return entities.NewValidationError("el médico %d está inactivo", medicoID)
	}
	return nil
}

func (s *CitaService) validarSolape(ctx context.Context, c entities.Cita) error {
	solapa, err := s.store.ExisteSolape(ctx, c.MedicoID, c.FechaHoraInicio, c.DuracionMin, c.ID)
	if err != nil {
		return err
	}
	if solapa {
		return fmt.Errorf("%w: el médico ya tiene una cita en ese horario", entities.ErrConflict)
	}
	return nil
}

func (s *CitaService) Crear(ctx context.Context, c entities.Cita) (*entities.Cita, error) {
	normalizarCita(&c)
	c.Estado = entities.EstadoCitaProgramada

	if err := validarDatosCita(c); err != nil {
		return nil, err
	}
	if c.FechaHoraInicio.Before(time.Now()) {
		return nil, entities.NewValidationError("fecha_hora_inicio no puede estar en el pasado")
	}
	if err := s.validarParticipantes(ctx, c.PacienteID, c.MedicoID); err != nil {
		return nil, err
	}
	if err := s.validarSolape(ctx, c); err != nil {
		return nil, err
	}
	if err := s.store.Crear(ctx, &c); err != nil {
		return nil, err
	}
	return s.store.ObtenerPorID(ctx, c.ID)
}

func (s *CitaService) Obtener(ctx context.Context, id int64) (*entities.Cita, error) {
	if id <= 0 {
		return nil, entities.NewValidationError("id de cita inválido")
	}
	return s.store.ObtenerPorID(ctx, id)
}

func (s *CitaService) Listar(ctx context.Context, filtro entities.CitaFiltro) ([]entities.Cita, error) {
	filtro.ListOptions = filtro.ListOptions.Normalized()
	filtro.Search = strings.TrimSpace(filtro.Search)

	if filtro.Estado != "" && !filtro.Estado.Valido() {
		return nil, entities.NewValidationError("estado de cita no válido")
	}
	if filtro.Desde != nil && filtro.Hasta != nil && filtro.Desde.After(*filtro.Hasta) {
		return nil, entities.NewValidationError("el rango de fechas es inválido: desde es posterior a hasta")
	}
	return s.store.Listar(ctx, filtro)
}

func (s *CitaService) Actualizar(ctx context.Context, id int64, c entities.Cita) (*entities.Cita, error) {
	if id <= 0 {
		return nil, entities.NewValidationError("id de cita inválido")
	}
	actual, err := s.store.ObtenerPorID(ctx, id)
	if err != nil {
		return nil, err
	}
	if actual.Estado.EsTerminal() {
		return nil, entities.NewValidationError(
			"la cita está en estado %s y ya no puede modificarse", actual.Estado)
	}

	normalizarCita(&c)
	if c.Estado == "" {
		c.Estado = actual.Estado
	}
	if err := validarDatosCita(c); err != nil {
		return nil, err
	}
	if !c.FechaHoraInicio.Equal(actual.FechaHoraInicio) && c.FechaHoraInicio.Before(time.Now()) {
		return nil, entities.NewValidationError("fecha_hora_inicio no puede estar en el pasado")
	}
	if c.Estado != actual.Estado && !actual.Estado.PuedePasarA(c.Estado) {
		return nil, entities.NewValidationError(
			"transición de estado no permitida: %s → %s", actual.Estado, c.Estado)
	}
	if err := s.validarParticipantes(ctx, c.PacienteID, c.MedicoID); err != nil {
		return nil, err
	}
	if c.Estado == entities.EstadoCitaProgramada || c.Estado == entities.EstadoCitaConfirmada {
		c.ID = id
		if err := s.validarSolape(ctx, c); err != nil {
			return nil, err
		}
	}

	c.ID = id
	c.Codigo = actual.Codigo
	c.CreatedAt = actual.CreatedAt
	if err := s.store.Actualizar(ctx, &c); err != nil {
		return nil, err
	}
	return s.store.ObtenerPorID(ctx, id)
}

// CambiarEstado aplica una transición validada del ciclo de vida de la cita.
func (s *CitaService) CambiarEstado(ctx context.Context, id int64, nuevo entities.EstadoCita) (*entities.Cita, error) {
	if id <= 0 {
		return nil, entities.NewValidationError("id de cita inválido")
	}
	if !nuevo.Valido() {
		return nil, entities.NewValidationError("estado de cita no válido")
	}
	actual, err := s.store.ObtenerPorID(ctx, id)
	if err != nil {
		return nil, err
	}
	if actual.Estado == nuevo {
		return actual, nil
	}
	if !actual.Estado.PuedePasarA(nuevo) {
		return nil, entities.NewValidationError(
			"transición de estado no permitida: %s → %s", actual.Estado, nuevo)
	}
	actual.Estado = nuevo
	if err := s.store.Actualizar(ctx, actual); err != nil {
		return nil, err
	}
	return s.store.ObtenerPorID(ctx, id)
}

// Eliminar cancela la cita sin borrar el registro (trazabilidad).
func (s *CitaService) Eliminar(ctx context.Context, id int64) error {
	if id <= 0 {
		return entities.NewValidationError("id de cita inválido")
	}
	actual, err := s.store.ObtenerPorID(ctx, id)
	if err != nil {
		return err
	}
	if actual.Estado == entities.EstadoCitaCancelada {
		return nil
	}
	if actual.Estado.EsTerminal() {
		return entities.NewValidationError(
			"la cita está en estado %s y no puede cancelarse", actual.Estado)
	}
	actual.Estado = entities.EstadoCitaCancelada
	return s.store.Actualizar(ctx, actual)
}
