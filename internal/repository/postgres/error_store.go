package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"medilist/internal/service/entities"
)

// ErrorStore implementa service.ErrorStore sobre PostgreSQL.
type ErrorStore struct {
	db *pgxpool.Pool
}

// NewErrorStore construye el almacén de errores agregados.
func NewErrorStore(db *pgxpool.Pool) *ErrorStore {
	return &ErrorStore{db: db}
}

func (s *ErrorStore) Registrar(ctx context.Context, e *entities.ErrorAplicacion) error {
	const q = `
		INSERT INTO errores_aplicacion
			(trace_id, servicio, instancia, componente, nivel, metodo, ruta, estado_http,
			 codigo, mensaje, usuario_id, username, ip, user_agent, stack_trace)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id, created_at`
	err := s.db.QueryRow(ctx, q,
		e.TraceID, e.Servicio, e.Instancia, e.Componente, e.Nivel, e.Metodo, e.Ruta,
		e.EstadoHTTP, e.Codigo, e.Mensaje, e.UsuarioID, e.Username,
		e.IP, e.UserAgent, e.StackTrace,
	).Scan(&e.ID, &e.CreatedAt)
	return mapError(err)
}

func (s *ErrorStore) Listar(ctx context.Context, filtro entities.ErrorFiltro) ([]entities.ErrorAplicacion, error) {
	const q = `
		SELECT id, trace_id, servicio, instancia, componente, nivel, metodo, ruta, estado_http,
			codigo, mensaje, usuario_id, username, ip, user_agent, stack_trace, created_at
		FROM errores_aplicacion
		WHERE (
			$1 = ''
			OR mensaje ILIKE '%' || $1 || '%'
			OR codigo ILIKE '%' || $1 || '%'
			OR ruta ILIKE '%' || $1 || '%'
			OR username ILIKE '%' || $1 || '%'
			OR trace_id ILIKE '%' || $1 || '%'
		)
		AND ($2 = '' OR nivel = $2)
		AND ($3 = '' OR servicio = $3)
		AND ($4 = '' OR instancia = $4)
		AND ($5 = '' OR trace_id = $5)
		ORDER BY created_at DESC
		LIMIT $6 OFFSET $7`
	rows, err := s.db.Query(ctx, q,
		filtro.Search, filtro.Nivel, filtro.Servicio, filtro.Instancia,
		filtro.TraceID, filtro.Limit, filtro.Offset,
	)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	lista := make([]entities.ErrorAplicacion, 0)
	for rows.Next() {
		var e entities.ErrorAplicacion
		var usuarioID pgtype.Int8
		if err := rows.Scan(
			&e.ID, &e.TraceID, &e.Servicio, &e.Instancia, &e.Componente, &e.Nivel,
			&e.Metodo, &e.Ruta, &e.EstadoHTTP, &e.Codigo, &e.Mensaje, &usuarioID,
			&e.Username, &e.IP, &e.UserAgent, &e.StackTrace, &e.CreatedAt,
		); err != nil {
			return nil, mapError(err)
		}
		if usuarioID.Valid {
			id := usuarioID.Int64
			e.UsuarioID = &id
		}
		lista = append(lista, e)
	}
	return lista, mapError(rows.Err())
}
