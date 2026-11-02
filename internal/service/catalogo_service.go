package service

import (
	"context"
	"strings"

	"medilist/internal/service/entities"
)

// CatalogoService expone el catálogo visible para el paciente. Solo devuelve
// especialidades activas y médicos activos que aceptan reservas por portal.
type CatalogoService struct {
	store CatalogoStore
}

// NewCatalogoService construye el servicio del catálogo público.
func NewCatalogoService(store CatalogoStore) *CatalogoService {
	return &CatalogoService{store: store}
}

// Especialidades lista las especialidades activas en el orden configurado.
func (s *CatalogoService) Especialidades(ctx context.Context) ([]entities.Especialidad, error) {
	return s.store.ListarEspecialidadesActivas(ctx)
}

// Medicos lista los profesionales visibles, opcionalmente filtrados por
// especialidad. El ID de especialidad es un filtro de catálogo, no un dato de
// propiedad clínica.
func (s *CatalogoService) Medicos(
	ctx context.Context,
	especialidadID int64,
	opts entities.ListOptions,
) ([]entities.Medico, error) {
	if especialidadID < 0 {
		return nil, entities.NewValidationError("especialidad_id no es válido")
	}
	opts = opts.Normalized()
	opts.Search = strings.TrimSpace(opts.Search)
	return s.store.ListarMedicosPortal(ctx, especialidadID, opts)
}
