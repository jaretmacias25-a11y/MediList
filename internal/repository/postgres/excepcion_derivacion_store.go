package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"medilist/internal/service/entities"
)

// ExcepcionDerivacionStore implementa el historial de excepciones
// administrativas de derivación sobre PostgreSQL.
type ExcepcionDerivacionStore struct {
	db *pgxpool.Pool
}

// NewExcepcionDerivacionStore construye el almacén de excepciones.
func NewExcepcionDerivacionStore(db *pgxpool.Pool) *ExcepcionDerivacionStore {
	return &ExcepcionDerivacionStore{db: db}
}

const excepcionDerivacionSelect = `
	SELECT id, cita_id, paciente_id, especialidad_id, medico_id,
	       motivo, autorizado_por_usuario_id, created_at
	FROM excepciones_derivacion`

func (s *ExcepcionDerivacionStore) Crear(
	ctx context.Context,
	e *entities.ExcepcionDerivacion,
) error {
	const q = `
		INSERT INTO excepciones_derivacion
			(cita_id, paciente_id, especialidad_id, medico_id, motivo,
			 autorizado_por_usuario_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`
	err := s.db.QueryRow(ctx, q,
		e.CitaID, e.PacienteID, e.EspecialidadID, e.MedicoID, e.Motivo,
		e.AutorizadoPorUsuarioID,
	).Scan(&e.ID, &e.CreatedAt)
	return mapError(err)
}

func (s *ExcepcionDerivacionStore) ObtenerPorCita(
	ctx context.Context,
	citaID int64,
) (*entities.ExcepcionDerivacion, error) {
	e, err := scanExcepcionDerivacion(s.db.QueryRow(
		ctx, excepcionDerivacionSelect+` WHERE cita_id = $1`, citaID,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, noEncontrado("excepción de derivación para cita", citaID)
	}
	if err != nil {
		return nil, mapError(err)
	}
	return e, nil
}

func scanExcepcionDerivacion(row rowScanner) (*entities.ExcepcionDerivacion, error) {
	var e entities.ExcepcionDerivacion
	var autorizadoPor pgtype.Int8
	err := row.Scan(
		&e.ID, &e.CitaID, &e.PacienteID, &e.EspecialidadID, &e.MedicoID,
		&e.Motivo, &autorizadoPor, &e.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if autorizadoPor.Valid {
		id := autorizadoPor.Int64
		e.AutorizadoPorUsuarioID = &id
	}
	return &e, nil
}
