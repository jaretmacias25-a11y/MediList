package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"medilist/internal/service/entities"
)

// CatalogoStore implementa las lecturas públicas del catálogo y de los
// profesionales visibles en el portal.
type CatalogoStore struct {
	db *pgxpool.Pool
}

// NewCatalogoStore construye el almacén del catálogo.
func NewCatalogoStore(db *pgxpool.Pool) *CatalogoStore {
	return &CatalogoStore{db: db}
}

func (s *CatalogoStore) ListarEspecialidadesActivas(
	ctx context.Context,
) ([]entities.Especialidad, error) {
	rows, err := s.db.Query(ctx, especialidadSelect+`
		WHERE activa
		ORDER BY orden, nombre`)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	lista := make([]entities.Especialidad, 0)
	for rows.Next() {
		e, err := scanEspecialidad(rows)
		if err != nil {
			return nil, mapError(err)
		}
		lista = append(lista, *e)
	}
	return lista, mapError(rows.Err())
}

func (s *CatalogoStore) ListarMedicosPortal(
	ctx context.Context,
	especialidadID int64,
	opts entities.ListOptions,
) ([]entities.Medico, error) {
	q := medicoSelect + `
		WHERE m.activo
		  AND m.acepta_portal
		  AND e.activa
		  AND ($1::bigint = 0 OR m.especialidad_id = $1)
		  AND (
			$2 = ''
			OR pe.nombres ILIKE '%' || $2 || '%'
			OR pe.apellidos ILIKE '%' || $2 || '%'
			OR e.nombre ILIKE '%' || $2 || '%'
		  )
		ORDER BY e.orden, pe.apellidos, pe.nombres
		LIMIT $3 OFFSET $4`
	rows, err := s.db.Query(ctx, q,
		especialidadID, opts.Search, opts.Limit, opts.Offset,
	)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	lista := make([]entities.Medico, 0)
	for rows.Next() {
		m, err := scanMedico(rows)
		if err != nil {
			return nil, mapError(err)
		}
		lista = append(lista, *m)
	}
	return lista, mapError(rows.Err())
}
