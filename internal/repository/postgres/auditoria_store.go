package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"medilist/internal/service/entities"
)

// AuditoriaStore implementa service.AuditoriaStore sobre PostgreSQL.
type AuditoriaStore struct {
	db *pgxpool.Pool
}

// NewAuditoriaStore construye el almacén.
func NewAuditoriaStore(db *pgxpool.Pool) *AuditoriaStore {
	return &AuditoriaStore{db: db}
}

func (s *AuditoriaStore) Registrar(ctx context.Context, a *entities.Auditoria) error {
	const q = `
		INSERT INTO auditoria
			(usuario_id, username, accion, recurso, metodo, ruta, estado_http, ip, user_agent, trace_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at`
	err := s.db.QueryRow(ctx, q,
		a.UsuarioID, a.Username, a.Accion, a.Recurso, a.Metodo, a.Ruta,
		a.EstadoHTTP, a.IP, a.UserAgent, a.TraceID,
	).Scan(&a.ID, &a.CreatedAt)
	return mapError(err)
}

func (s *AuditoriaStore) Listar(ctx context.Context, filtro entities.AuditoriaFiltro) ([]entities.Auditoria, error) {
	const q = `
		SELECT id, usuario_id, username, accion, recurso, metodo, ruta,
			estado_http, ip, user_agent, trace_id, created_at
		FROM auditoria
		WHERE (
			$1 = ''
			OR username ILIKE '%' || $1 || '%'
			OR accion ILIKE '%' || $1 || '%'
			OR recurso ILIKE '%' || $1 || '%'
			OR ruta ILIKE '%' || $1 || '%'
			OR trace_id ILIKE '%' || $1 || '%'
		)
		AND ($2::bigint = 0 OR usuario_id = $2)
		AND ($3 = '' OR accion = $3)
		AND ($4 = '' OR trace_id = $4)
		ORDER BY created_at DESC
		LIMIT $5 OFFSET $6`
	rows, err := s.db.Query(ctx, q,
		filtro.Search, filtro.UsuarioID, filtro.Accion, filtro.TraceID,
		filtro.Limit, filtro.Offset,
	)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	lista := make([]entities.Auditoria, 0)
	for rows.Next() {
		var a entities.Auditoria
		var usuarioID pgtype.Int8
		err := rows.Scan(
			&a.ID, &usuarioID, &a.Username, &a.Accion, &a.Recurso,
			&a.Metodo, &a.Ruta, &a.EstadoHTTP, &a.IP, &a.UserAgent,
			&a.TraceID, &a.CreatedAt,
		)
		if err != nil {
			return nil, mapError(err)
		}
		if usuarioID.Valid {
			id := usuarioID.Int64
			a.UsuarioID = &id
		}
		lista = append(lista, a)
	}
	return lista, mapError(rows.Err())
}
