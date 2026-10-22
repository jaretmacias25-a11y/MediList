package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"medilist/internal/service/entities"
)

// RolStore implementa service.RolStore sobre PostgreSQL.
type RolStore struct {
	db *pgxpool.Pool
}

// NewRolStore construye el almacén.
func NewRolStore(db *pgxpool.Pool) *RolStore {
	return &RolStore{db: db}
}

const rolSelect = `
	SELECT
		r.id, r.codigo, r.nombre, r.descripcion, r.created_at, r.updated_at,
		COALESCE(array_agg(DISTINCT p.codigo) FILTER (WHERE p.codigo IS NOT NULL), '{}') AS permisos
	FROM roles r
	LEFT JOIN rol_permisos rp ON rp.rol_id = r.id
	LEFT JOIN permisos p ON p.id = rp.permiso_id`

func (s *RolStore) Listar(ctx context.Context) ([]entities.Rol, error) {
	q := rolSelect + `
		GROUP BY r.id
		ORDER BY r.nombre ASC`
	rows, err := s.db.Query(ctx, q)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	lista := make([]entities.Rol, 0)
	for rows.Next() {
		rol, err := scanRol(rows)
		if err != nil {
			return nil, mapError(err)
		}
		lista = append(lista, *rol)
	}
	return lista, mapError(rows.Err())
}

func (s *RolStore) ObtenerPorCodigos(ctx context.Context, codigos []string) ([]entities.Rol, error) {
	q := rolSelect + `
		WHERE r.codigo = ANY($1::text[])
		GROUP BY r.id
		ORDER BY r.codigo`
	rows, err := s.db.Query(ctx, q, codigos)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	lista := make([]entities.Rol, 0, len(codigos))
	for rows.Next() {
		rol, err := scanRol(rows)
		if err != nil {
			return nil, mapError(err)
		}
		lista = append(lista, *rol)
	}
	return lista, mapError(rows.Err())
}

func scanRol(row rowScanner) (*entities.Rol, error) {
	var r entities.Rol
	err := row.Scan(
		&r.ID, &r.Codigo, &r.Nombre, &r.Descripcion,
		&r.CreatedAt, &r.UpdatedAt, &r.Permisos,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}
