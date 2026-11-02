package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"medilist/internal/service/entities"
)

// MedicoInput contiene los datos editables del profesional. Los campos de
// perfil público son punteros para que una actualización parcial no los borre.
type MedicoInput struct {
	Persona         entities.Persona
	EspecialidadID  int64
	NumeroLicencia  string
	TipoProfesional string
	DuracionCitaMin *int
	Bio             *string
	FotoURL         *string
	AceptaPortal    *bool
	Version         int64
}

// MedicoService contiene la lógica de negocio de profesionales de la salud.
type MedicoService struct {
	store             MedicoStore
	especialidadStore EspecialidadStore
}

// NewMedicoService construye el servicio con sus almacenes.
func NewMedicoService(store MedicoStore, especialidadStore EspecialidadStore) *MedicoService {
	return &MedicoService{store: store, especialidadStore: especialidadStore}
}

func (s *MedicoService) validarEspecialidad(ctx context.Context, id int64) error {
	if _, err := s.especialidadStore.ObtenerPorID(ctx, id); err != nil {
		if errors.Is(err, entities.ErrNotFound) {
			return entities.NewValidationError("la especialidad %d no existe", id)
		}
		return err
	}
	return nil
}

func (s *MedicoService) Crear(ctx context.Context, in MedicoInput) (*entities.Medico, error) {
	m := entities.Medico{
		Persona:         in.Persona,
		EspecialidadID:  in.EspecialidadID,
		NumeroLicencia:  in.NumeroLicencia,
		TipoProfesional: in.TipoProfesional,
		DuracionCitaMin: 30,
		AceptaPortal:    true,
		Activo:          true,
	}
	if in.DuracionCitaMin != nil {
		m.DuracionCitaMin = *in.DuracionCitaMin
	}
	if in.Bio != nil {
		m.Bio = *in.Bio
	}
	if in.FotoURL != nil {
		m.FotoURL = *in.FotoURL
	}
	if in.AceptaPortal != nil {
		m.AceptaPortal = *in.AceptaPortal
	}
	normalizarMedico(&m)
	if err := validarMedico(m); err != nil {
		return nil, err
	}
	if err := s.validarEspecialidad(ctx, m.EspecialidadID); err != nil {
		return nil, err
	}
	if err := s.store.Crear(ctx, &m); err != nil {
		return nil, err
	}
	return s.store.ObtenerPorID(ctx, m.ID)
}

func (s *MedicoService) Obtener(ctx context.Context, id int64) (*entities.Medico, error) {
	if id <= 0 {
		return nil, entities.NewValidationError("id de médico inválido")
	}
	return s.store.ObtenerPorID(ctx, id)
}

func (s *MedicoService) Listar(ctx context.Context, opts entities.ListOptions) ([]entities.Medico, error) {
	opts = opts.Normalized()
	opts.Search = strings.TrimSpace(opts.Search)
	return s.store.Listar(ctx, opts)
}

func (s *MedicoService) Actualizar(
	ctx context.Context,
	id int64,
	in MedicoInput,
) (*entities.Medico, error) {
	if id <= 0 {
		return nil, entities.NewValidationError("id de médico inválido")
	}
	actual, err := s.store.ObtenerPorID(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Version > 0 && in.Version != actual.Version {
		return nil, fmt.Errorf(
			"%w: el médico fue modificado por otra operación; vuelve a consultarlo e inténtalo de nuevo",
			entities.ErrConflict)
	}

	m := *actual
	m.Persona = in.Persona
	m.Persona.ID = actual.Persona.ID
	m.EspecialidadID = in.EspecialidadID
	m.NumeroLicencia = in.NumeroLicencia
	m.TipoProfesional = in.TipoProfesional
	if in.DuracionCitaMin != nil {
		m.DuracionCitaMin = *in.DuracionCitaMin
	}
	if in.Bio != nil {
		m.Bio = *in.Bio
	}
	if in.FotoURL != nil {
		m.FotoURL = *in.FotoURL
	}
	if in.AceptaPortal != nil {
		m.AceptaPortal = *in.AceptaPortal
	}
	normalizarMedico(&m)
	m.ID = id
	m.Version = actual.Version
	m.UsuarioID = actual.UsuarioID
	m.Activo = actual.Activo
	if err := validarMedico(m); err != nil {
		return nil, err
	}
	if err := s.validarEspecialidad(ctx, m.EspecialidadID); err != nil {
		return nil, err
	}
	if err := s.store.Actualizar(ctx, &m); err != nil {
		return nil, err
	}
	return s.store.ObtenerPorID(ctx, id)
}

// Eliminar realiza una baja lógica para conservar las citas históricas.
func (s *MedicoService) Eliminar(ctx context.Context, id int64) error {
	if id <= 0 {
		return entities.NewValidationError("id de médico inválido")
	}
	return s.store.Eliminar(ctx, id)
}
