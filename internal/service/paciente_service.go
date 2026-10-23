package service

import (
	"context"
	"fmt"
	"strings"

	"medilist/internal/service/entities"
)

// PacienteService contiene la lógica de negocio de pacientes.
type PacienteService struct {
	store PacienteStore
}

// NewPacienteService construye el servicio con su almacén.
func NewPacienteService(store PacienteStore) *PacienteService {
	return &PacienteService{store: store}
}

func (s *PacienteService) Crear(ctx context.Context, p entities.Paciente) (*entities.Paciente, error) {
	normalizarPaciente(&p)
	if err := validarPaciente(p); err != nil {
		return nil, err
	}
	p.Activo = true
	if err := s.store.Crear(ctx, &p); err != nil {
		return nil, err
	}
	return s.store.ObtenerPorID(ctx, p.ID)
}

func (s *PacienteService) Obtener(ctx context.Context, id int64) (*entities.Paciente, error) {
	if id <= 0 {
		return nil, entities.NewValidationError("id de paciente inválido")
	}
	return s.store.ObtenerPorID(ctx, id)
}

func (s *PacienteService) Listar(ctx context.Context, opts entities.ListOptions) ([]entities.Paciente, error) {
	opts = opts.Normalized()
	opts.Search = strings.TrimSpace(opts.Search)
	return s.store.Listar(ctx, opts)
}

func (s *PacienteService) Actualizar(ctx context.Context, id int64, p entities.Paciente) (*entities.Paciente, error) {
	if id <= 0 {
		return nil, entities.NewValidationError("id de paciente inválido")
	}
	actual, err := s.store.ObtenerPorID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.Version > 0 && p.Version != actual.Version {
		return nil, fmt.Errorf(
			"%w: el paciente fue modificado por otra operación; vuelve a consultarlo e inténtalo de nuevo",
			entities.ErrConflict)
	}

	normalizarPaciente(&p)
	p.ID = id
	p.Persona.ID = actual.Persona.ID
	p.NumeroHistoria = actual.NumeroHistoria
	p.Activo = actual.Activo
	p.Version = actual.Version
	if err := validarPaciente(p); err != nil {
		return nil, err
	}
	if err := s.store.Actualizar(ctx, &p); err != nil {
		return nil, err
	}
	return s.store.ObtenerPorID(ctx, id)
}

// Eliminar realiza una baja lógica para conservar el historial clínico.
func (s *PacienteService) Eliminar(ctx context.Context, id int64) error {
	if id <= 0 {
		return entities.NewValidationError("id de paciente inválido")
	}
	return s.store.Eliminar(ctx, id)
}
