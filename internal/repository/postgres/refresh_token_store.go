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
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return mapError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// La fila del usuario se comparte para que una revocación administrativa
	// concurrente espere a que termine la creación del token, o bien haga que
	// esta operación falle si la versión de sesión ya cambió.
	var activo bool
	var tokenVersion int
	err = tx.QueryRow(ctx, `
		SELECT activo, token_version
		FROM usuarios
		WHERE id = $1
		FOR SHARE`, rt.UsuarioID).Scan(&activo, &tokenVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return noEncontrado("usuario", rt.UsuarioID)
	}
	if err != nil {
		return mapError(err)
	}
	if !activo {
		return entities.ErrUnauthorized
	}
	if tokenVersion != rt.TokenVersion {
		return entities.ErrUnauthorized
	}

	const q = `
		INSERT INTO refresh_tokens (usuario_id, token_hash, token_version, expira_en, ip, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`
	err = tx.QueryRow(ctx, q,
		rt.UsuarioID, rt.TokenHash, rt.TokenVersion, rt.ExpiraEn, rt.IP, rt.UserAgent,
	).Scan(&rt.ID, &rt.CreatedAt)
	if err != nil {
		return mapError(err)
	}
	return mapError(tx.Commit(ctx))
}

func (s *RefreshTokenStore) BuscarPorHash(ctx context.Context, hash string) (*entities.RefreshToken, error) {
	const q = `
		SELECT id, usuario_id, token_hash, token_version, expira_en, revocado_en, ip, user_agent, created_at
		FROM refresh_tokens
		WHERE token_hash = $1`
	var rt entities.RefreshToken
	var revocadoEn pgtype.Timestamptz
	err := s.db.QueryRow(ctx, q, hash).Scan(
		&rt.ID, &rt.UsuarioID, &rt.TokenHash, &rt.TokenVersion, &rt.ExpiraEn, &revocadoEn,
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

// Rotar consume el token anterior y crea el nuevo en una sola transacción.
// Devuelve false sin error cuando el token ya fue consumido, expiró o no
// pertenece al usuario, lo que el servicio interpreta como reutilización.
func (s *RefreshTokenStore) Rotar(ctx context.Context, id int64, nuevo *entities.RefreshToken) (bool, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return false, mapError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var activo bool
	var tokenVersion int
	err = tx.QueryRow(ctx, `
		SELECT activo, token_version
		FROM usuarios
		WHERE id = $1
		FOR SHARE`, nuevo.UsuarioID).Scan(&activo, &tokenVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, entities.ErrUnauthorized
	}
	if err != nil {
		return false, mapError(err)
	}
	if !activo || tokenVersion != nuevo.TokenVersion {
		return false, entities.ErrUnauthorized
	}

	tag, err := tx.Exec(ctx, `
		UPDATE refresh_tokens
		SET revocado_en = now()
		WHERE id = $1
		  AND usuario_id = $2
		  AND revocado_en IS NULL
		  AND expira_en > now()`, id, nuevo.UsuarioID)
	if err != nil {
		return false, mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}

	const q = `
		INSERT INTO refresh_tokens (usuario_id, token_hash, token_version, expira_en, ip, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`
	err = tx.QueryRow(ctx, q,
		nuevo.UsuarioID, nuevo.TokenHash, nuevo.TokenVersion, nuevo.ExpiraEn,
		nuevo.IP, nuevo.UserAgent,
	).Scan(&nuevo.ID, &nuevo.CreatedAt)
	if err != nil {
		return false, mapError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, mapError(err)
	}
	return true, nil
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
