package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"medilist/internal/service/entities"
)

// UsuarioStore implementa service.UsuarioStore sobre PostgreSQL.
type UsuarioStore struct {
	db *pgxpool.Pool
}

// NewUsuarioStore construye el almacén.
func NewUsuarioStore(db *pgxpool.Pool) *UsuarioStore {
	return &UsuarioStore{db: db}
}

const usuarioSelect = `
	SELECT
		u.id, u.username, u.email, u.password_hash, u.nombres, u.apellidos,
		u.activo, u.token_version, u.intentos_fallidos, u.bloqueado_hasta, u.ultimo_acceso,
		u.created_at, u.updated_at,
		COALESCE(array_agg(DISTINCT r.codigo) FILTER (WHERE r.codigo IS NOT NULL), '{}') AS roles,
		COALESCE(array_agg(DISTINCT p.codigo) FILTER (WHERE p.codigo IS NOT NULL), '{}') AS permisos
	FROM usuarios u
	LEFT JOIN usuario_roles ur ON ur.usuario_id = u.id
	LEFT JOIN roles r ON r.id = ur.rol_id
	LEFT JOIN rol_permisos rp ON rp.rol_id = r.id
	LEFT JOIN permisos p ON p.id = rp.permiso_id`

func (s *UsuarioStore) BuscarPorUsername(ctx context.Context, username string) (*entities.Usuario, error) {
	q := usuarioSelect + `
		WHERE LOWER(u.username) = LOWER($1) OR LOWER(u.email) = LOWER($1)
		GROUP BY u.id`
	u, err := scanUsuario(s.db.QueryRow(ctx, q, username))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%w: usuario no encontrado", entities.ErrNotFound)
	}
	if err != nil {
		return nil, mapError(err)
	}
	return u, nil
}

func (s *UsuarioStore) ObtenerPorID(ctx context.Context, id int64) (*entities.Usuario, error) {
	q := usuarioSelect + ` WHERE u.id = $1 GROUP BY u.id`
	u, err := scanUsuario(s.db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, noEncontrado("usuario", id)
	}
	if err != nil {
		return nil, mapError(err)
	}
	return u, nil
}

func (s *UsuarioStore) Listar(ctx context.Context, opts entities.ListOptions) ([]entities.Usuario, error) {
	q := usuarioSelect + `
		WHERE (
			$1 = ''
			OR u.username ILIKE '%' || $1 || '%'
			OR u.email ILIKE '%' || $1 || '%'
			OR u.nombres ILIKE '%' || $1 || '%'
			OR u.apellidos ILIKE '%' || $1 || '%'
		)
		AND (NOT $2 OR u.activo)
		GROUP BY u.id
		ORDER BY u.id DESC
		LIMIT $3 OFFSET $4`
	rows, err := s.db.Query(ctx, q, opts.Search, opts.SoloActivos, opts.Limit, opts.Offset)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	lista := make([]entities.Usuario, 0)
	for rows.Next() {
		u, err := scanUsuario(rows)
		if err != nil {
			return nil, mapError(err)
		}
		lista = append(lista, *u)
	}
	return lista, mapError(rows.Err())
}

func (s *UsuarioStore) Crear(ctx context.Context, u *entities.Usuario) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return mapError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const insertUsuario = `
		INSERT INTO usuarios (username, email, password_hash, nombres, apellidos, activo)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at`
	err = tx.QueryRow(ctx, insertUsuario,
		u.Username, u.Email, u.PasswordHash, u.Nombres, u.Apellidos, u.Activo,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return mapError(err)
	}
	if err := asignarRoles(ctx, tx, u.ID, u.Roles); err != nil {
		return err
	}
	return mapError(tx.Commit(ctx))
}

func (s *UsuarioStore) Actualizar(ctx context.Context, u *entities.Usuario, passwordHash string, revocarSesiones bool) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return mapError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const updateUsuario = `
		UPDATE usuarios
		SET email = $2,
			nombres = $3,
			apellidos = $4,
			activo = $5,
			password_hash = COALESCE(NULLIF($6, ''), password_hash),
			token_version = token_version + CASE WHEN $7 THEN 1 ELSE 0 END
		WHERE id = $1
		RETURNING id`
	var id int64
	err = tx.QueryRow(ctx, updateUsuario,
		u.ID, u.Email, u.Nombres, u.Apellidos, u.Activo, passwordHash, revocarSesiones,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return noEncontrado("usuario", u.ID)
	}
	if err != nil {
		return mapError(err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM usuario_roles WHERE usuario_id = $1`, u.ID); err != nil {
		return mapError(err)
	}
	if err := asignarRoles(ctx, tx, u.ID, u.Roles); err != nil {
		return err
	}
	if revocarSesiones {
		if _, err := tx.Exec(ctx,
			`UPDATE refresh_tokens SET revocado_en = now() WHERE usuario_id = $1 AND revocado_en IS NULL`,
			u.ID,
		); err != nil {
			return mapError(err)
		}
	}
	return mapError(tx.Commit(ctx))
}

func asignarRoles(ctx context.Context, tx pgx.Tx, usuarioID int64, roles []string) error {
	const q = `
		INSERT INTO usuario_roles (usuario_id, rol_id)
		SELECT $1, id FROM roles WHERE codigo = ANY($2::text[])`
	tag, err := tx.Exec(ctx, q, usuarioID, roles)
	if err != nil {
		return mapError(err)
	}
	if int(tag.RowsAffected()) != len(roles) {
		return entities.NewValidationError("uno o más roles no existen")
	}
	return nil
}

func (s *UsuarioStore) RegistrarLoginExitoso(ctx context.Context, id int64) error {
	_, err := s.db.Exec(ctx, `
		UPDATE usuarios
		SET ultimo_acceso = now(), intentos_fallidos = 0, bloqueado_hasta = NULL
		WHERE id = $1`, id)
	return mapError(err)
}

func (s *UsuarioStore) RegistrarIntentoFallido(ctx context.Context, id int64, intentos int, bloqueadoHasta *time.Time) error {
	_, err := s.db.Exec(ctx, `
		UPDATE usuarios
		SET intentos_fallidos = $2, bloqueado_hasta = $3
		WHERE id = $1`, id, intentos, bloqueadoHasta)
	return mapError(err)
}

func (s *UsuarioStore) RevocarSesiones(ctx context.Context, usuarioID int64) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return mapError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		UPDATE usuarios
		SET token_version = token_version + 1
		WHERE id = $1`, usuarioID)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return noEncontrado("usuario", usuarioID)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE refresh_tokens
		SET revocado_en = now()
		WHERE usuario_id = $1 AND revocado_en IS NULL`, usuarioID); err != nil {
		return mapError(err)
	}
	return mapError(tx.Commit(ctx))
}

func (s *UsuarioStore) SesionValida(ctx context.Context, usuarioID int64, tokenVersion int) (bool, error) {
	const q = `
		SELECT EXISTS (
			SELECT 1 FROM usuarios
			WHERE id = $1 AND activo AND token_version = $2
		)`
	var valida bool
	if err := s.db.QueryRow(ctx, q, usuarioID, tokenVersion).Scan(&valida); err != nil {
		return false, mapError(err)
	}
	return valida, nil
}

func (s *UsuarioStore) Desbloquear(ctx context.Context, id int64) error {
	tag, err := s.db.Exec(ctx, `
		UPDATE usuarios
		SET intentos_fallidos = 0, bloqueado_hasta = NULL
		WHERE id = $1`, id)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return noEncontrado("usuario", id)
	}
	return nil
}

func scanUsuario(row rowScanner) (*entities.Usuario, error) {
	var u entities.Usuario
	var bloqueadoHasta, ultimoAcceso pgtype.Timestamptz
	err := row.Scan(
		&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Nombres, &u.Apellidos,
		&u.Activo, &u.TokenVersion, &u.IntentosFallidos, &bloqueadoHasta, &ultimoAcceso,
		&u.CreatedAt, &u.UpdatedAt, &u.Roles, &u.Permisos,
	)
	if err != nil {
		return nil, err
	}
	if bloqueadoHasta.Valid {
		t := bloqueadoHasta.Time
		u.BloqueadoHasta = &t
	}
	if ultimoAcceso.Valid {
		t := ultimoAcceso.Time
		u.UltimoAcceso = &t
	}
	return &u, nil
}
