package service

import (
	"context"
	"strings"

	"medilist/internal/service/entities"
)

// EspecialidadService contiene la lógica de negocio de especialidades.
type EspecialidadService struct {
	store EspecialidadStore
}

// NewEspecialidadService construye el servicio con su almacén.
func NewEspecialidadService(store EspecialidadStore) *EspecialidadService {
	return &EspecialidadService{store: store}
}

func (s *EspecialidadService) Crear(ctx context.Context, e entities.Especialidad) (*entities.Especialidad, error) {
	e.Codigo = strings.ToUpper(strings.TrimSpace(e.Codigo))
	e.Nombre = strings.TrimSpace(e.Nombre)
	e.Descripcion = strings.TrimSpace(e.Descripcion)

	if e.Codigo == "" {
		return nil, entities.NewValidationError("codigo es obligatorio")
	}
	if e.Nombre == "" {
		return nil, entities.NewValidationError("nombre es obligatorio")
	}
	if err := s.store.Crear(ctx, &e); err != nil {
		return nil, err
	}
	return s.store.ObtenerPorID(ctx, e.ID)
}

func (s *EspecialidadService) Obtener(ctx context.Context, id int64) (*entities.Especialidad, error) {
	if id <= 0 {
		return nil, entities.NewValidationError("id de especialidad inválido")
	}
	return s.store.ObtenerPorID(ctx, id)
}

func (s *EspecialidadService) Listar(ctx context.Context, opts entities.ListOptions) ([]entities.Especialidad, error) {
	opts = opts.Normalized()
	opts.Search = strings.TrimSpace(opts.Search)
	return s.store.Listar(ctx, opts)
}

func (s *EspecialidadService) Actualizar(ctx context.Context, id int64, e entities.Especialidad) (*entities.Especialidad, error) {
	if id <= 0 {
		return nil, entities.NewValidationError("id de especialidad inválido")
	}
	e.ID = id
	e.Codigo = strings.ToUpper(strings.TrimSpace(e.Codigo))
	e.Nombre = strings.TrimSpace(e.Nombre)
	e.Descripcion = strings.TrimSpace(e.Descripcion)
	if e.Codigo == "" {
		return nil, entities.NewValidationError("codigo es obligatorio")
	}
	if e.Nombre == "" {
		return nil, entities.NewValidationError("nombre es obligatorio")
	}
	if err := s.store.Actualizar(ctx, &e); err != nil {
		return nil, err
	}
	return s.store.ObtenerPorID(ctx, id)
}

func (s *EspecialidadService) Eliminar(ctx context.Context, id int64) error {
	if id <= 0 {
		return entities.NewValidationError("id de especialidad inválido")
	}
	return s.store.Eliminar(ctx, id)
}
