package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"medilist/internal/service/entities"
)

// ActorStore implementa service.ActorStore sobre PostgreSQL.
type ActorStore struct {
	db *pgxpool.Pool
}

// NewActorStore construye el almacén de vínculos de cuenta.
func NewActorStore(db *pgxpool.Pool) *ActorStore {
	return &ActorStore{db: db}
}

// ResolverVinculos devuelve los perfiles activos asociados a la cuenta. Los
// vínculos inactivos se tratan como inexistentes para que el portal exija una
// revisión administrativa antes de volver a exponer datos clínicos.
func (s *ActorStore) ResolverVinculos(ctx context.Context, usuarioID int64) (*entities.VinculosUsuario, error) {
	const q = `
		SELECT
			u.email,
			u.email_verificado_en IS NOT NULL,
			(SELECT p.id FROM pacientes p
			  WHERE p.usuario_id = u.id AND p.activo
			  LIMIT 1),
			(SELECT m.id FROM medicos m
			  WHERE m.usuario_id = u.id AND m.activo
			  LIMIT 1)
		FROM usuarios u
		WHERE u.id = $1`

	var vinculos entities.VinculosUsuario
	var pacienteID, medicoID pgtype.Int8
	err := s.db.QueryRow(ctx, q, usuarioID).Scan(
		&vinculos.Email,
		&vinculos.EmailVerificado,
		&pacienteID,
		&medicoID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, noEncontrado("usuario", usuarioID)
	}
	if err != nil {
		return nil, mapError(err)
	}
	if pacienteID.Valid {
		id := pacienteID.Int64
		vinculos.PacienteID = &id
	}
	if medicoID.Valid {
		id := medicoID.Int64
		vinculos.MedicoID = &id
	}
	return &vinculos, nil
}
