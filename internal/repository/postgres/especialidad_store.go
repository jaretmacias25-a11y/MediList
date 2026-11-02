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

const especialidadSelect = `
	SELECT
		id, codigo, nombre, descripcion,
		requiere_derivacion, activa, orden, duracion_cita_min,
		version, created_at, updated_at
	FROM especialidades`

func (s *EspecialidadStore) Crear(ctx context.Context, e *entities.Especialidad) error {
	const q = `
		INSERT INTO especialidades
			(codigo, nombre, descripcion, requiere_derivacion, activa, orden, duracion_cita_min)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, version, created_at, updated_at`
	err := s.db.QueryRow(ctx, q,
		e.Codigo, e.Nombre, e.Descripcion, e.RequiereDerivacion,
		e.Activa, e.Orden, e.DuracionCitaMin,
	).Scan(&e.ID, &e.Version, &e.CreatedAt, &e.UpdatedAt)
	return mapError(err)
}

func (s *EspecialidadStore) ObtenerPorID(ctx context.Context, id int64) (*entities.Especialidad, error) {
	q := especialidadSelect + ` WHERE id = $1`
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
	q := especialidadSelect + `
		WHERE ($1 = '' OR codigo ILIKE '%' || $1 || '%' OR nombre ILIKE '%' || $1 || '%')
		  AND (NOT $2 OR activa)
		ORDER BY nombre ASC
		LIMIT $3 OFFSET $4`
	rows, err := s.db.Query(ctx, q, opts.Search, opts.SoloActivos, opts.Limit, opts.Offset)
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
		SET codigo = $2,
			nombre = $3,
			descripcion = $4,
			requiere_derivacion = $5,
			activa = $6,
			orden = $7,
			duracion_cita_min = $8,
			version = version + 1
		WHERE id = $1 AND version = $9`
	tag, err := s.db.Exec(ctx, q, e.ID, e.Codigo, e.Nombre, e.Descripcion,
		e.RequiereDerivacion, e.Activa, e.Orden, e.DuracionCitaMin, e.Version)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return conflictoConcurrencia("especialidad", e.ID)
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
	err := row.Scan(
		&e.ID, &e.Codigo, &e.Nombre, &e.Descripcion,
		&e.RequiereDerivacion, &e.Activa, &e.Orden, &e.DuracionCitaMin,
		&e.Version, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &e, nil
}
