package service

import (
	"context"
	"fmt"
	"strings"

	"medilist/internal/service/entities"
)

// EspecialidadInput contiene los campos editables del catálogo. Los punteros
// de configuración permiten conservar los flags cuando la actualización no los
// envía.
type EspecialidadInput struct {
	Codigo             string
	Nombre             string
	Descripcion        string
	RequiereDerivacion *bool
	Activa             *bool
	Orden              *int
	DuracionCitaMin    *int
	Version            int64
}

// EspecialidadService contiene la lógica de negocio de especialidades.
type EspecialidadService struct {
	store EspecialidadStore
}

// NewEspecialidadService construye el servicio con su almacén.
func NewEspecialidadService(store EspecialidadStore) *EspecialidadService {
	return &EspecialidadService{store: store}
}

func (s *EspecialidadService) Crear(ctx context.Context, in EspecialidadInput) (*entities.Especialidad, error) {
	e := entities.Especialidad{
		Codigo:             in.Codigo,
		Nombre:             in.Nombre,
		Descripcion:        in.Descripcion,
		RequiereDerivacion: true,
		Activa:             true,
		DuracionCitaMin:    30,
	}
	if in.RequiereDerivacion != nil {
		e.RequiereDerivacion = *in.RequiereDerivacion
	}
	if in.Activa != nil {
		e.Activa = *in.Activa
	}
	if in.Orden != nil {
		e.Orden = *in.Orden
	}
	if in.DuracionCitaMin != nil {
		e.DuracionCitaMin = *in.DuracionCitaMin
	}
	normalizarEspecialidad(&e)
	if err := validarEspecialidad(e); err != nil {
		return nil, err
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

func (s *EspecialidadService) Actualizar(
	ctx context.Context,
	id int64,
	in EspecialidadInput,
) (*entities.Especialidad, error) {
	if id <= 0 {
		return nil, entities.NewValidationError("id de especialidad inválido")
	}
	actual, err := s.store.ObtenerPorID(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Version > 0 && in.Version != actual.Version {
		return nil, fmt.Errorf(
			"%w: la especialidad fue modificada por otra operación; vuelve a consultarla e inténtalo de nuevo",
			entities.ErrConflict)
	}

	e := *actual
	e.Codigo = in.Codigo
	e.Nombre = in.Nombre
	e.Descripcion = in.Descripcion
	if in.RequiereDerivacion != nil {
		e.RequiereDerivacion = *in.RequiereDerivacion
	}
	if in.Activa != nil {
		e.Activa = *in.Activa
	}
	if in.Orden != nil {
		e.Orden = *in.Orden
	}
	if in.DuracionCitaMin != nil {
		e.DuracionCitaMin = *in.DuracionCitaMin
	}
	normalizarEspecialidad(&e)
	e.ID = id
	e.Version = actual.Version
	if err := validarEspecialidad(e); err != nil {
		return nil, err
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
