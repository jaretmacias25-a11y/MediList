package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"medilist/internal/service/entities"
)

// DerivacionAdminService gestiona la excepción administrativa de derivación.
// No crea derivaciones clínicas: registra una autorización excepcional.
type DerivacionAdminService struct {
	citas       CitaStore
	medicos     MedicoStore
	excepciones ExcepcionDerivacionStore
}

// NewDerivacionAdminService construye el servicio de excepciones.
func NewDerivacionAdminService(
	citas CitaStore,
	medicos MedicoStore,
	excepciones ExcepcionDerivacionStore,
) *DerivacionAdminService {
	return &DerivacionAdminService{
		citas:       citas,
		medicos:     medicos,
		excepciones: excepciones,
	}
}

// Sobrescribir registra una excepción para una cita de especialista sin
// derivación. Solo el rol admin con derivaciones:sobrescribir puede hacerlo.
func (s *DerivacionAdminService) Sobrescribir(
	ctx context.Context,
	actor *Actor,
	citaID int64,
	motivo string,
) (*entities.ExcepcionDerivacion, error) {
	if actor == nil || !actor.TieneRol("admin") ||
		!actor.TienePermiso("derivaciones:sobrescribir") {
		return nil, fmt.Errorf("%w: solo un administrador puede sobrescribir derivaciones", entities.ErrForbidden)
	}
	if citaID <= 0 {
		return nil, entities.NewValidationError("id de cita inválido")
	}
	motivo = strings.TrimSpace(motivo)
	if motivo == "" {
		return nil, entities.NewValidationError("motivo de la sobrescritura es obligatorio")
	}
	if utf8.RuneCountInString(motivo) > 2000 {
		return nil, entities.NewValidationError("motivo no puede superar los 2000 caracteres")
	}
	cita, err := s.citas.ObtenerPorID(ctx, citaID)
	if err != nil {
		return nil, err
	}
	if cita.DerivacionID != nil {
		return nil, fmt.Errorf("%w: la cita ya tiene una derivación clínica", entities.ErrConflict)
	}
	medico, err := s.medicos.ObtenerPorID(ctx, cita.MedicoID)
	if err != nil {
		return nil, err
	}
	if !medico.Especialidad.RequiereDerivacion {
		return nil, entities.NewValidationError(
			"la cita no pertenece a una especialidad que requiera derivación")
	}
	if _, err := s.excepciones.ObtenerPorCita(ctx, citaID); err == nil {
		return nil, fmt.Errorf("%w: la cita ya tiene una excepción registrada", entities.ErrConflict)
	} else if !errors.Is(err, entities.ErrNotFound) {
		return nil, err
	}
	usuarioID := actor.UsuarioID
	excepcion := &entities.ExcepcionDerivacion{
		CitaID:                 citaID,
		PacienteID:             cita.PacienteID,
		EspecialidadID:         medico.EspecialidadID,
		MedicoID:               cita.MedicoID,
		Motivo:                 motivo,
		AutorizadoPorUsuarioID: &usuarioID,
	}
	if err := s.excepciones.Crear(ctx, excepcion); err != nil {
		return nil, err
	}
	return excepcion, nil
}
