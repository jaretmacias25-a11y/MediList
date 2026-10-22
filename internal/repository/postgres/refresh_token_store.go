package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"medilist/internal/service/entities"
)

// RefreshTokenStore implementa service.RefreshTokenStore sobre PostgreSQL.
type RefreshTokenStore struct {
	db *pgxpool.Pool
}

// NewRefreshTokenStore construye el almacén.
func NewRefreshTokenStore(db *pgxpool.Pool) *RefreshTokenStore {
	return &RefreshTokenStore{db: db}
}

func (s *RefreshTokenStore) Crear(ctx context.Context, rt *entities.RefreshToken) error {
	const q = `
		INSERT INTO refresh_tokens (usuario_id, token_hash, expira_en, ip, user_agent)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`
	err := s.db.QueryRow(ctx, q,
		rt.UsuarioID, rt.TokenHash, rt.ExpiraEn, rt.IP, rt.UserAgent,
	).Scan(&rt.ID, &rt.CreatedAt)
	return mapError(err)
}

func (s *RefreshTokenStore) BuscarPorHash(ctx context.Context, hash string) (*entities.RefreshToken, error) {
	const q = `
		SELECT id, usuario_id, token_hash, expira_en, revocado_en, ip, user_agent, created_at
		FROM refresh_tokens
		WHERE token_hash = $1`
	var rt entities.RefreshToken
	var revocadoEn pgtype.Timestamptz
	err := s.db.QueryRow(ctx, q, hash).Scan(
		&rt.ID, &rt.UsuarioID, &rt.TokenHash, &rt.ExpiraEn, &revocadoEn,
		&rt.IP, &rt.UserAgent, &rt.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, entities.ErrNotFound
	}
	if err != nil {
		return nil, mapError(err)
	}
	if revocadoEn.Valid {
		t := revocadoEn.Time
		rt.RevocadoEn = &t
	}
	return &rt, nil
}

func (s *RefreshTokenStore) Revocar(ctx context.Context, id int64) error {
	_, err := s.db.Exec(ctx, `
		UPDATE refresh_tokens
		SET revocado_en = now()
		WHERE id = $1 AND revocado_en IS NULL`, id)
	return mapError(err)
}

func (s *RefreshTokenStore) RevocarTodosPorUsuario(ctx context.Context, usuarioID int64) error {
	_, err := s.db.Exec(ctx, `
		UPDATE refresh_tokens
		SET revocado_en = now()
		WHERE usuario_id = $1 AND revocado_en IS NULL`, usuarioID)
	return mapError(err)
}
