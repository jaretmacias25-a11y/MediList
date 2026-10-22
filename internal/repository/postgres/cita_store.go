package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"medilist/internal/service/entities"
)

// CitaStore implementa service.CitaStore sobre PostgreSQL.
type CitaStore struct {
	db *pgxpool.Pool
}

// NewCitaStore construye el almacén.
func NewCitaStore(db *pgxpool.Pool) *CitaStore {
	return &CitaStore{db: db}
}

const citaSelect = `
	SELECT
		c.id, c.codigo, c.paciente_id,
		pe.nombres || ' ' || pe.apellidos AS paciente_nombre,
		c.medico_id,
		me.nombres || ' ' || me.apellidos AS medico_nombre,
		e.nombre AS especialidad_nombre,
		c.fecha_hora_inicio, c.duracion_min, c.tipo, c.motivo, c.estado,
		c.created_at, c.updated_at
	FROM citas c
	JOIN pacientes p ON p.id = c.paciente_id
	JOIN personas pe ON pe.id = p.persona_id
	JOIN medicos m ON m.id = c.medico_id
	JOIN personas me ON me.id = m.persona_id
	JOIN especialidades e ON e.id = m.especialidad_id`

func (s *CitaStore) Crear(ctx context.Context, c *entities.Cita) error {
	const q = `
		INSERT INTO citas
			(paciente_id, medico_id, fecha_hora_inicio, duracion_min, tipo, motivo, estado)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, codigo, created_at, updated_at`
	err := s.db.QueryRow(ctx, q,
		c.PacienteID, c.MedicoID, c.FechaHoraInicio, c.DuracionMin,
		string(c.Tipo), c.Motivo, string(c.Estado),
	).Scan(&c.ID, &c.Codigo, &c.CreatedAt, &c.UpdatedAt)
	return mapError(err)
}

func (s *CitaStore) ObtenerPorID(ctx context.Context, id int64) (*entities.Cita, error) {
	c, err := scanCita(s.db.QueryRow(ctx, citaSelect+` WHERE c.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, noEncontrado("cita", id)
	}
	if err != nil {
		return nil, mapError(err)
	}
	return c, nil
}

func (s *CitaStore) Listar(ctx context.Context, filtro entities.CitaFiltro) ([]entities.Cita, error) {
	var desde, hasta any
	if filtro.Desde != nil {
		desde = *filtro.Desde
	}
	if filtro.Hasta != nil {
		hasta = *filtro.Hasta
	}

	q := citaSelect + `
		WHERE ($1::bigint = 0 OR c.paciente_id = $1)
		  AND ($2::bigint = 0 OR c.medico_id = $2)
		  AND ($3::text = '' OR c.estado = $3)
		  AND ($4::timestamptz IS NULL OR c.fecha_hora_inicio >= $4)
		  AND ($5::timestamptz IS NULL OR c.fecha_hora_inicio < $5)
		  AND (
			$6 = ''
			OR c.codigo ILIKE '%' || $6 || '%'
			OR pe.nombres ILIKE '%' || $6 || '%'
			OR pe.apellidos ILIKE '%' || $6 || '%'
			OR me.nombres ILIKE '%' || $6 || '%'
			OR me.apellidos ILIKE '%' || $6 || '%'
		  )
		ORDER BY c.fecha_hora_inicio DESC
		LIMIT $7 OFFSET $8`
	rows, err := s.db.Query(ctx, q,
		filtro.PacienteID, filtro.MedicoID, string(filtro.Estado),
		desde, hasta, filtro.Search, filtro.Limit, filtro.Offset,
	)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	lista := make([]entities.Cita, 0)
	for rows.Next() {
		c, err := scanCita(rows)
		if err != nil {
			return nil, mapError(err)
		}
		lista = append(lista, *c)
	}
	return lista, mapError(rows.Err())
}

func (s *CitaStore) Actualizar(ctx context.Context, c *entities.Cita) error {
	const q = `
		UPDATE citas
		SET paciente_id = $2,
			medico_id = $3,
			fecha_hora_inicio = $4,
			duracion_min = $5,
			tipo = $6,
			motivo = $7,
			estado = $8
		WHERE id = $1`
	tag, err := s.db.Exec(ctx, q,
		c.ID, c.PacienteID, c.MedicoID, c.FechaHoraInicio, c.DuracionMin,
		string(c.Tipo), c.Motivo, string(c.Estado),
	)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return noEncontrado("cita", c.ID)
	}
	return nil
}

// ExisteSolape detecta citas activas del mismo médico que se cruzan en el tiempo.
// La condición clásica de solape es: inicioA < finB && finA > inicioB.
func (s *CitaStore) ExisteSolape(ctx context.Context, medicoID int64, inicio time.Time, duracionMin int, excluirID int64) (bool, error) {
	const q = `
		SELECT EXISTS (
			SELECT 1
			FROM citas
			WHERE medico_id = $1
			  AND id <> $4
			  AND estado IN ('programada', 'confirmada')
			  AND fecha_hora_inicio < $3
			  AND fecha_hora_inicio + (duracion_min * interval '1 minute') > $2
		)`
	fin := inicio.Add(time.Duration(duracionMin) * time.Minute)
	var existe bool
	err := s.db.QueryRow(ctx, q, medicoID, inicio, fin, excluirID).Scan(&existe)
	return existe, mapError(err)
}

func scanCita(row rowScanner) (*entities.Cita, error) {
	var c entities.Cita
	var tipo, estado string
	err := row.Scan(
		&c.ID, &c.Codigo, &c.PacienteID, &c.PacienteNombre,
		&c.MedicoID, &c.MedicoNombre, &c.EspecialidadNombre,
		&c.FechaHoraInicio, &c.DuracionMin, &tipo, &c.Motivo, &estado,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	c.Tipo = entities.TipoCita(tipo)
	c.Estado = entities.EstadoCita(estado)
	return &c, nil
}
