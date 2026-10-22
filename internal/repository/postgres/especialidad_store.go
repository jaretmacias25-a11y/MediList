package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"medilist/internal/service/entities"
)

// EspecialidadStore implementa service.EspecialidadStore sobre PostgreSQL.
type EspecialidadStore struct {
	db *pgxpool.Pool
}

// NewEspecialidadStore construye el almacén.
func NewEspecialidadStore(db *pgxpool.Pool) *EspecialidadStore {
	return &EspecialidadStore{db: db}
}

func (s *EspecialidadStore) Crear(ctx context.Context, e *entities.Especialidad) error {
	const q = `
		INSERT INTO especialidades (codigo, nombre, descripcion)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at`
	err := s.db.QueryRow(ctx, q, e.Codigo, e.Nombre, e.Descripcion).
		Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)
	return mapError(err)
}

func (s *EspecialidadStore) ObtenerPorID(ctx context.Context, id int64) (*entities.Especialidad, error) {
	const q = `
		SELECT id, codigo, nombre, descripcion, created_at, updated_at
		FROM especialidades
		WHERE id = $1`
	e, err := scanEspecialidad(s.db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, noEncontrado("especialidad", id)
	}
	if err != nil {
		return nil, mapError(err)
	}
	return e, nil
}

func (s *EspecialidadStore) Listar(ctx context.Context, opts entities.ListOptions) ([]entities.Especialidad, error) {
	const q = `
		SELECT id, codigo, nombre, descripcion, created_at, updated_at
		FROM especialidades
		WHERE ($1 = '' OR codigo ILIKE '%' || $1 || '%' OR nombre ILIKE '%' || $1 || '%')
		ORDER BY nombre ASC
		LIMIT $2 OFFSET $3`
	rows, err := s.db.Query(ctx, q, opts.Search, opts.Limit, opts.Offset)
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

func (s *EspecialidadStore) Actualizar(ctx context.Context, e *entities.Especialidad) error {
	const q = `
		UPDATE especialidades
		SET codigo = $2, nombre = $3, descripcion = $4
		WHERE id = $1`
	tag, err := s.db.Exec(ctx, q, e.ID, e.Codigo, e.Nombre, e.Descripcion)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return noEncontrado("especialidad", e.ID)
	}
	return nil
}

func (s *EspecialidadStore) Eliminar(ctx context.Context, id int64) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM especialidades WHERE id = $1`, id)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return noEncontrado("especialidad", id)
	}
	return nil
}

func scanEspecialidad(row rowScanner) (*entities.Especialidad, error) {
	var e entities.Especialidad
	err := row.Scan(&e.ID, &e.Codigo, &e.Nombre, &e.Descripcion, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &e, nil
}
