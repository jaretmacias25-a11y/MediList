package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"medilist/internal/service/entities"
)

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

func (s *MedicoService) Crear(ctx context.Context, m entities.Medico) (*entities.Medico, error) {
	normalizarMedico(&m)
	if err := validarMedico(m); err != nil {
		return nil, err
	}
	if err := s.validarEspecialidad(ctx, m.EspecialidadID); err != nil {
		return nil, err
	}
	m.Activo = true
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

func (s *MedicoService) Actualizar(ctx context.Context, id int64, m entities.Medico) (*entities.Medico, error) {
	if id <= 0 {
		return nil, entities.NewValidationError("id de médico inválido")
	}
	actual, err := s.store.ObtenerPorID(ctx, id)
	if err != nil {
		return nil, err
	}
	if m.Version > 0 && m.Version != actual.Version {
		return nil, fmt.Errorf(
			"%w: el médico fue modificado por otra operación; vuelve a consultarlo e inténtalo de nuevo",
			entities.ErrConflict)
	}

	normalizarMedico(&m)
	m.ID = id
	m.Persona.ID = actual.Persona.ID
	m.Activo = actual.Activo
	m.Version = actual.Version
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
