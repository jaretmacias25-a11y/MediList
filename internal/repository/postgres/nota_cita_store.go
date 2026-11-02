package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"medilist/internal/service/entities"
)

// NotaCitaStore implementa service.NotaCitaStore sobre PostgreSQL.
type NotaCitaStore struct {
	db *pgxpool.Pool
}

// NewNotaCitaStore construye el almacén de notas de consulta.
func NewNotaCitaStore(db *pgxpool.Pool) *NotaCitaStore {
	return &NotaCitaStore{db: db}
}

const notaCitaSelect = `
	SELECT id, cita_id, medico_id, motivo, evaluacion, plan,
	       version, created_at, updated_at
	FROM notas_cita`

func (s *NotaCitaStore) ObtenerPorCita(
	ctx context.Context,
	citaID int64,
) (*entities.NotaCita, error) {
	nota, err := scanNotaCita(s.db.QueryRow(
		ctx, notaCitaSelect+` WHERE cita_id = $1`, citaID,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, noEncontrado("nota de cita", citaID)
	}
	if err != nil {
		return nil, mapError(err)
	}
	return nota, nil
}

func (s *NotaCitaStore) Crear(ctx context.Context, n *entities.NotaCita) error {
	const q = `
		INSERT INTO notas_cita (cita_id, medico_id, motivo, evaluacion, plan)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, version, created_at, updated_at`
	err := s.db.QueryRow(ctx, q,
		n.CitaID, n.MedicoID, n.Motivo, n.Evaluacion, n.Plan,
	).Scan(&n.ID, &n.Version, &n.CreatedAt, &n.UpdatedAt)
	return mapError(err)
}

func (s *NotaCitaStore) Actualizar(ctx context.Context, n *entities.NotaCita) error {
	const q = `
		UPDATE notas_cita
		SET motivo = $2, evaluacion = $3, plan = $4,
		    version = version + 1
		WHERE id = $1 AND version = $5
		RETURNING version, updated_at`
	err := s.db.QueryRow(ctx, q,
		n.ID, n.Motivo, n.Evaluacion, n.Plan, n.Version,
	).Scan(&n.Version, &n.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return conflictoConcurrencia("nota de cita", n.ID)
	}
	return mapError(err)
}

func scanNotaCita(row rowScanner) (*entities.NotaCita, error) {
	var n entities.NotaCita
	err := row.Scan(
		&n.ID, &n.CitaID, &n.MedicoID, &n.Motivo, &n.Evaluacion, &n.Plan,
		&n.Version, &n.CreatedAt, &n.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &n, nil
}
