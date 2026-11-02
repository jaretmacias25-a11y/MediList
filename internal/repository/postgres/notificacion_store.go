package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"medilist/internal/service/entities"
)

// NotificacionStore implementa el outbox durable y las preferencias.
type NotificacionStore struct {
	db *pgxpool.Pool
}

// NewNotificacionStore construye el almacén del outbox.
func NewNotificacionStore(db *pgxpool.Pool) *NotificacionStore {
	return &NotificacionStore{db: db}
}

const notificacionSelect = `
	SELECT id, tipo, usuario_id, destinatario, nombre, asunto, cuerpo, estado,
	       intentos, proximo_intento, ultimo_error, dedupe_key,
	       created_at, updated_at, enviada_en
	FROM notificaciones_outbox`

func (s *NotificacionStore) Encolar(
	ctx context.Context,
	n *entities.Notificacion,
) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO notificaciones_outbox
			(tipo, usuario_id, destinatario, nombre, asunto, cuerpo, estado,
			 proximo_intento, dedupe_key)
		VALUES ($1, $2, $3, $4, $5, $6, 'pendiente', now(), $7)
		ON CONFLICT (dedupe_key) DO NOTHING`,
		string(n.Tipo), n.UsuarioID, n.Destinatario, n.Nombre, n.Asunto,
		n.Cuerpo, n.DedupeKey,
	)
	return mapError(err)
}

func (s *NotificacionStore) ReclamarPendientes(
	ctx context.Context,
	ahora time.Time,
	limite int,
) ([]entities.Notificacion, error) {
	rows, err := s.db.Query(ctx, `
		UPDATE notificaciones_outbox
		SET estado = 'enviando', updated_at = now()
		WHERE id IN (
			SELECT id
			FROM notificaciones_outbox
			WHERE (estado = 'pendiente' AND proximo_intento <= $1)
			   OR (estado = 'enviando' AND updated_at < now() - interval '5 minutes')
			ORDER BY proximo_intento, id
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, tipo, usuario_id, destinatario, nombre, asunto, cuerpo, estado,
		          intentos, proximo_intento, ultimo_error, dedupe_key,
		          created_at, updated_at, enviada_en`, ahora, limite)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	lista := make([]entities.Notificacion, 0)
	for rows.Next() {
		n, err := scanNotificacion(rows)
		if err != nil {
			return nil, mapError(err)
		}
		lista = append(lista, *n)
	}
	return lista, mapError(rows.Err())
}

func (s *NotificacionStore) MarcarEnviada(
	ctx context.Context,
	id int64,
	cuando time.Time,
) error {
	_, err := s.db.Exec(ctx, `
		UPDATE notificaciones_outbox
		SET estado = 'enviada',
			intentos = intentos + 1,
			enviada_en = $2,
			ultimo_error = '',
			updated_at = now()
		WHERE id = $1`, id, cuando)
	return mapError(err)
}

func (s *NotificacionStore) MarcarFallo(
	ctx context.Context,
	id int64,
	motivo string,
	proximoIntento time.Time,
	maxIntentos int,
) error {
	_, err := s.db.Exec(ctx, `
		UPDATE notificaciones_outbox
		SET intentos = intentos + 1,
			ultimo_error = $2,
			proximo_intento = $3,
			estado = CASE WHEN intentos + 1 >= $4 THEN 'fallida' ELSE 'pendiente' END,
			updated_at = now()
		WHERE id = $1`, id, motivo, proximoIntento, maxIntentos)
	return mapError(err)
}

func (s *NotificacionStore) ObtenerPreferencias(
	ctx context.Context,
	usuarioID int64,
) (*entities.PreferenciasNotificacion, error) {
	_, err := s.db.Exec(ctx, `
		INSERT INTO preferencias_notificacion (usuario_id)
		VALUES ($1)
		ON CONFLICT (usuario_id) DO NOTHING`, usuarioID)
	if err != nil {
		return nil, mapError(err)
	}
	var p entities.PreferenciasNotificacion
	err = s.db.QueryRow(ctx, `
		SELECT usuario_id, email_habilitado, recordatorios_habilitados, updated_at
		FROM preferencias_notificacion
		WHERE usuario_id = $1`, usuarioID,
	).Scan(&p.UsuarioID, &p.EmailHabilitado, &p.RecordatoriosHabilitados, &p.UpdatedAt)
	if err != nil {
		return nil, mapError(err)
	}
	return &p, nil
}

func (s *NotificacionStore) ActualizarPreferencias(
	ctx context.Context,
	p *entities.PreferenciasNotificacion,
) error {
	err := s.db.QueryRow(ctx, `
		INSERT INTO preferencias_notificacion
			(usuario_id, email_habilitado, recordatorios_habilitados)
		VALUES ($1, $2, $3)
		ON CONFLICT (usuario_id) DO UPDATE
		SET email_habilitado = EXCLUDED.email_habilitado,
		    recordatorios_habilitados = EXCLUDED.recordatorios_habilitados
		RETURNING updated_at`,
		p.UsuarioID, p.EmailHabilitado, p.RecordatoriosHabilitados,
	).Scan(&p.UpdatedAt)
	return mapError(err)
}

func scanNotificacion(row rowScanner) (*entities.Notificacion, error) {
	var n entities.Notificacion
	var estado, tipo string
	var usuarioID pgtype.Int8
	var enviadaEn pgtype.Timestamptz
	err := row.Scan(
		&n.ID, &tipo, &usuarioID, &n.Destinatario, &n.Nombre, &n.Asunto, &n.Cuerpo,
		&estado, &n.Intentos, &n.ProximoIntento, &n.UltimoError,
		&n.DedupeKey, &n.CreatedAt, &n.UpdatedAt, &enviadaEn,
	)
	if err != nil {
		return nil, err
	}
	n.Tipo = entities.TipoNotificacion(tipo)
	n.Estado = entities.EstadoNotificacion(estado)
	if usuarioID.Valid {
		id := usuarioID.Int64
		n.UsuarioID = &id
	}
	if enviadaEn.Valid {
		cuando := enviadaEn.Time
		n.EnviadaEn = &cuando
	}
	return &n, nil
}

// insertarNotificacionTx es usado por los stores de citas/derivaciones para
// encolar en la misma transacción del evento. La clave única hace idempotente
// el reintento de la operación.
func insertarNotificacionTx(
	ctx context.Context,
	tx pgx.Tx,
	n *entities.Notificacion,
) error {
	if n == nil {
		return nil
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO notificaciones_outbox
			(tipo, usuario_id, destinatario, nombre, asunto, cuerpo, estado,
			 proximo_intento, dedupe_key)
		VALUES ($1, $2, $3, $4, $5, $6, 'pendiente', now(), $7)
		ON CONFLICT (dedupe_key) DO NOTHING`,
		string(n.Tipo), n.UsuarioID, n.Destinatario, n.Nombre, n.Asunto,
		n.Cuerpo, n.DedupeKey,
	)
	return mapError(err)
}
