package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"medilist/internal/service/entities"
)

// AgendaStore implementa los puertos de horarios, bloqueos y disponibilidad
// sobre PostgreSQL.
type AgendaStore struct {
	db *pgxpool.Pool
}

// NewAgendaStore construye el almacén de agenda.
func NewAgendaStore(db *pgxpool.Pool) *AgendaStore {
	return &AgendaStore{db: db}
}

const horarioSelect = `
	SELECT
		id, medico_id, dia_semana,
		to_char(hora_inicio, 'HH24:MI'),
		to_char(hora_fin, 'HH24:MI'),
		duracion_slot_min, vigente_desde, vigente_hasta,
		activo, version, created_at, updated_at
	FROM horarios_profesional`

func (s *AgendaStore) ListarHorariosPorMedico(
	ctx context.Context,
	medicoID int64,
) ([]entities.HorarioProfesional, error) {
	rows, err := s.db.Query(ctx, horarioSelect+`
		WHERE medico_id = $1
		ORDER BY dia_semana, hora_inicio`, medicoID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	lista := make([]entities.HorarioProfesional, 0)
	for rows.Next() {
		h, err := scanHorario(rows)
		if err != nil {
			return nil, mapError(err)
		}
		lista = append(lista, *h)
	}
	return lista, mapError(rows.Err())
}

func (s *AgendaStore) ObtenerHorarioPorID(
	ctx context.Context,
	medicoID, horarioID int64,
) (*entities.HorarioProfesional, error) {
	h, err := scanHorario(s.db.QueryRow(
		ctx, horarioSelect+` WHERE id = $1 AND medico_id = $2`, horarioID, medicoID,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, noEncontrado("horario", horarioID)
	}
	if err != nil {
		return nil, mapError(err)
	}
	return h, nil
}

func (s *AgendaStore) ListarHorariosVigentes(
	ctx context.Context,
	medicoID int64,
	desde, hasta time.Time,
) ([]entities.HorarioProfesional, error) {
	rows, err := s.db.Query(ctx, horarioSelect+`
		WHERE medico_id = $1
		  AND activo
		  AND vigente_desde <= $3::date
		  AND (vigente_hasta IS NULL OR vigente_hasta >= $2::date)
		ORDER BY dia_semana, hora_inicio`,
		medicoID,
		desde.Format("2006-01-02"),
		hasta.Format("2006-01-02"),
	)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	lista := make([]entities.HorarioProfesional, 0)
	for rows.Next() {
		h, err := scanHorario(rows)
		if err != nil {
			return nil, mapError(err)
		}
		lista = append(lista, *h)
	}
	return lista, mapError(rows.Err())
}

func (s *AgendaStore) CrearHorario(ctx context.Context, h *entities.HorarioProfesional) error {
	const q = `
		INSERT INTO horarios_profesional
			(medico_id, dia_semana, hora_inicio, hora_fin, duracion_slot_min,
			 vigente_desde, vigente_hasta, activo)
		VALUES ($1, $2, $3::time, $4::time, $5, $6::date, $7::date, $8)
		RETURNING id, version, created_at, updated_at`
	var vigenteHasta any
	if h.VigenteHasta != nil {
		vigenteHasta = h.VigenteHasta.Format("2006-01-02")
	}
	err := s.db.QueryRow(ctx, q,
		h.MedicoID, h.DiaSemana, h.HoraInicio, h.HoraFin, h.DuracionSlotMin,
		h.VigenteDesde.Format("2006-01-02"), vigenteHasta, h.Activo,
	).Scan(&h.ID, &h.Version, &h.CreatedAt, &h.UpdatedAt)
	return mapError(err)
}

func (s *AgendaStore) ActualizarHorario(ctx context.Context, h *entities.HorarioProfesional) error {
	const q = `
		UPDATE horarios_profesional
		SET dia_semana = $3,
			hora_inicio = $4::time,
			hora_fin = $5::time,
			duracion_slot_min = $6,
			vigente_desde = $7::date,
			vigente_hasta = $8::date,
			activo = $9,
			version = version + 1
		WHERE id = $1 AND medico_id = $2 AND version = $10
		RETURNING version, updated_at`
	var vigenteHasta any
	if h.VigenteHasta != nil {
		vigenteHasta = h.VigenteHasta.Format("2006-01-02")
	}
	err := s.db.QueryRow(ctx, q,
		h.ID, h.MedicoID, h.DiaSemana, h.HoraInicio, h.HoraFin, h.DuracionSlotMin,
		h.VigenteDesde.Format("2006-01-02"), vigenteHasta, h.Activo, h.Version,
	).Scan(&h.Version, &h.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return conflictoConcurrencia("horario", h.ID)
	}
	return mapError(err)
}

func (s *AgendaStore) DesactivarHorario(ctx context.Context, medicoID, horarioID int64) error {
	tag, err := s.db.Exec(ctx, `
		UPDATE horarios_profesional
		SET activo = FALSE, version = version + 1
		WHERE id = $1 AND medico_id = $2`, horarioID, medicoID)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return noEncontrado("horario", horarioID)
	}
	return nil
}

func (s *AgendaStore) ExisteSolapamientoHorario(
	ctx context.Context,
	medicoID int64,
	diaSemana int,
	horaInicio, horaFin string,
	vigenteDesde time.Time,
	vigenteHasta *time.Time,
	excluirID int64,
) (bool, error) {
	const q = `
		SELECT EXISTS (
			SELECT 1
			FROM horarios_profesional
			WHERE medico_id = $1
			  AND dia_semana = $2
			  AND activo
			  AND id <> $7
			  AND int4range(
					hora_a_segundos(hora_inicio),
					hora_a_segundos(hora_fin),
					'[)'
				  ) && int4range(
					hora_a_segundos($3::time),
					hora_a_segundos($4::time),
					'[)'
				  )
			  AND daterange(
					vigente_desde,
					COALESCE(vigente_hasta, 'infinity'::date),
					'[]'
				  ) && daterange(
					$5::date,
					COALESCE($6::date, 'infinity'::date),
					'[]'
				  )
		)`
	var vigenteHastaTexto any
	if vigenteHasta != nil {
		vigenteHastaTexto = vigenteHasta.Format("2006-01-02")
	}
	var solapa bool
	err := s.db.QueryRow(ctx, q,
		medicoID, diaSemana, horaInicio, horaFin,
		vigenteDesde.Format("2006-01-02"), vigenteHastaTexto, excluirID,
	).Scan(&solapa)
	return solapa, mapError(err)
}

const bloqueoSelect = `
	SELECT id, medico_id, inicio, fin, motivo, creado_por, created_at, updated_at
	FROM bloqueos_agenda`

func (s *AgendaStore) ListarBloqueosRango(
	ctx context.Context,
	medicoID int64,
	desde, hasta time.Time,
) ([]entities.BloqueoAgenda, error) {
	rows, err := s.db.Query(ctx, bloqueoSelect+`
		WHERE medico_id = $1 AND fin > $2 AND inicio < $3
		ORDER BY inicio`, medicoID, desde, hasta)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	lista := make([]entities.BloqueoAgenda, 0)
	for rows.Next() {
		b, err := scanBloqueo(rows)
		if err != nil {
			return nil, mapError(err)
		}
		lista = append(lista, *b)
	}
	return lista, mapError(rows.Err())
}

func (s *AgendaStore) CrearBloqueo(ctx context.Context, b *entities.BloqueoAgenda) error {
	const q = `
		INSERT INTO bloqueos_agenda (medico_id, inicio, fin, motivo, creado_por)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`
	err := s.db.QueryRow(ctx, q,
		b.MedicoID, b.Inicio, b.Fin, b.Motivo, b.CreadoPor,
	).Scan(&b.ID, &b.CreatedAt, &b.UpdatedAt)
	return mapError(err)
}

func (s *AgendaStore) ActualizarBloqueo(ctx context.Context, b *entities.BloqueoAgenda) error {
	const q = `
		UPDATE bloqueos_agenda
		SET inicio = $3, fin = $4, motivo = $5
		WHERE id = $1 AND medico_id = $2
		RETURNING creado_por, created_at, updated_at`
	var creadoPor pgtype.Int8
	err := s.db.QueryRow(ctx, q,
		b.ID, b.MedicoID, b.Inicio, b.Fin, b.Motivo,
	).Scan(&creadoPor, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return noEncontrado("bloqueo", b.ID)
	}
	if err != nil {
		return mapError(err)
	}
	if creadoPor.Valid {
		id := creadoPor.Int64
		b.CreadoPor = &id
	}
	return nil
}

func (s *AgendaStore) EliminarBloqueo(ctx context.Context, medicoID, bloqueoID int64) error {
	tag, err := s.db.Exec(ctx, `
		DELETE FROM bloqueos_agenda
		WHERE id = $1 AND medico_id = $2`, bloqueoID, medicoID)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return noEncontrado("bloqueo", bloqueoID)
	}
	return nil
}

func (s *AgendaStore) ExisteSolapamientoBloqueo(
	ctx context.Context,
	medicoID int64,
	inicio, fin time.Time,
	excluirID int64,
) (bool, error) {
	const q = `
		SELECT EXISTS (
			SELECT 1
			FROM bloqueos_agenda
			WHERE medico_id = $1
			  AND id <> $4
			  AND tstzrange(inicio, fin, '[)') && tstzrange($2, $3, '[)')
		)`
	var solapa bool
	err := s.db.QueryRow(ctx, q, medicoID, inicio, fin, excluirID).Scan(&solapa)
	return solapa, mapError(err)
}

func (s *AgendaStore) ListarCitasActivasRango(
	ctx context.Context,
	medicoID int64,
	desde, hasta time.Time,
) ([]entities.IntervaloOcupado, error) {
	rows, err := s.db.Query(ctx, `
		SELECT fecha_hora_inicio, cita_fin(fecha_hora_inicio, duracion_min)
		FROM citas
		WHERE medico_id = $1
		  AND estado IN ('programada', 'confirmada')
		  AND fecha_hora_inicio < $3
		  AND cita_fin(fecha_hora_inicio, duracion_min) > $2
		ORDER BY fecha_hora_inicio`, medicoID, desde, hasta)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	lista := make([]entities.IntervaloOcupado, 0)
	for rows.Next() {
		var intervalo entities.IntervaloOcupado
		if err := rows.Scan(&intervalo.Inicio, &intervalo.Fin); err != nil {
			return nil, mapError(err)
		}
		lista = append(lista, intervalo)
	}
	return lista, mapError(rows.Err())
}

func scanHorario(row rowScanner) (*entities.HorarioProfesional, error) {
	var h entities.HorarioProfesional
	var hasta pgtype.Date
	err := row.Scan(
		&h.ID, &h.MedicoID, &h.DiaSemana, &h.HoraInicio, &h.HoraFin,
		&h.DuracionSlotMin, &h.VigenteDesde, &hasta,
		&h.Activo, &h.Version, &h.CreatedAt, &h.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if hasta.Valid {
		fecha := hasta.Time
		h.VigenteHasta = &fecha
	}
	return &h, nil
}

func scanBloqueo(row rowScanner) (*entities.BloqueoAgenda, error) {
	var b entities.BloqueoAgenda
	var creadoPor pgtype.Int8
	err := row.Scan(
		&b.ID, &b.MedicoID, &b.Inicio, &b.Fin, &b.Motivo,
		&creadoPor, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if creadoPor.Valid {
		id := creadoPor.Int64
		b.CreadoPor = &id
	}
	return &b, nil
}
