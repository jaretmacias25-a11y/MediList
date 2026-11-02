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

// DerivacionStore implementa service.DerivacionStore sobre PostgreSQL.
type DerivacionStore struct {
	db *pgxpool.Pool
}

// NewDerivacionStore construye el almacén de derivaciones.
func NewDerivacionStore(db *pgxpool.Pool) *DerivacionStore {
	return &DerivacionStore{db: db}
}

const derivacionSelect = `
	SELECT
		d.id, d.paciente_id,
		pe.nombres || ' ' || pe.apellidos AS paciente_nombre,
		d.medico_origen_id,
		mop.nombres || ' ' || mop.apellidos AS medico_origen_nombre,
		d.cita_origen_id,
		d.especialidad_destino_id,
		e.nombre AS especialidad_destino,
		d.medico_destino_id,
		COALESCE(mdp.nombres || ' ' || mdp.apellidos, '') AS medico_destino_nombre,
		d.motivo, d.prioridad, d.estado,
		d.vigente_desde, d.vigente_hasta, d.motivo_cancelacion,
		d.version, d.created_at, d.updated_at
	FROM derivaciones d
	JOIN pacientes p ON p.id = d.paciente_id
	JOIN personas pe ON pe.id = p.persona_id
	JOIN medicos mo ON mo.id = d.medico_origen_id
	JOIN personas mop ON mop.id = mo.persona_id
	JOIN especialidades e ON e.id = d.especialidad_destino_id
	LEFT JOIN medicos md ON md.id = d.medico_destino_id
	LEFT JOIN personas mdp ON mdp.id = md.persona_id`

func (s *DerivacionStore) Crear(ctx context.Context, d *entities.Derivacion) error {
	const q = `
		INSERT INTO derivaciones
			(paciente_id, medico_origen_id, cita_origen_id,
			 especialidad_destino_id, medico_destino_id, motivo,
			 prioridad, estado, vigente_desde, vigente_hasta,
			 motivo_cancelacion)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, version, created_at, updated_at`
	var medicoDestino any
	if d.MedicoDestinoID != nil {
		medicoDestino = *d.MedicoDestinoID
	}
	err := s.db.QueryRow(ctx, q,
		d.PacienteID, d.MedicoOrigenID, d.CitaOrigenID,
		d.EspecialidadDestinoID, medicoDestino, d.Motivo,
		string(d.Prioridad), string(d.Estado), d.VigenteDesde, d.VigenteHasta,
		d.MotivoCancelacion,
	).Scan(&d.ID, &d.Version, &d.CreatedAt, &d.UpdatedAt)
	return mapError(err)
}

func (s *DerivacionStore) ObtenerPorID(
	ctx context.Context,
	id int64,
) (*entities.Derivacion, error) {
	d, err := scanDerivacion(s.db.QueryRow(
		ctx, derivacionSelect+` WHERE d.id = $1`, id,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, noEncontrado("derivación", id)
	}
	if err != nil {
		return nil, mapError(err)
	}
	return d, nil
}

func (s *DerivacionStore) ListarPorMedicoOrigen(
	ctx context.Context,
	medicoID int64,
	opts entities.ListOptions,
) ([]entities.Derivacion, error) {
	rows, err := s.db.Query(ctx, derivacionSelect+`
		WHERE d.medico_origen_id = $1
		  AND (
			$2 = ''
			OR pe.nombres ILIKE '%' || $2 || '%'
			OR pe.apellidos ILIKE '%' || $2 || '%'
			OR e.nombre ILIKE '%' || $2 || '%'
			OR d.motivo ILIKE '%' || $2 || '%'
		  )
		ORDER BY d.created_at DESC
		LIMIT $3 OFFSET $4`,
		medicoID, opts.Search, opts.Limit, opts.Offset,
	)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	lista := make([]entities.Derivacion, 0)
	for rows.Next() {
		d, err := scanDerivacion(rows)
		if err != nil {
			return nil, mapError(err)
		}
		lista = append(lista, *d)
	}
	return lista, mapError(rows.Err())
}

func (s *DerivacionStore) ListarPorPaciente(
	ctx context.Context,
	pacienteID int64,
	opts entities.ListOptions,
) ([]entities.Derivacion, error) {
	rows, err := s.db.Query(ctx, derivacionSelect+`
		WHERE d.paciente_id = $1
		  AND (
			$2 = ''
			OR e.nombre ILIKE '%' || $2 || '%'
			OR d.motivo ILIKE '%' || $2 || '%'
		  )
		ORDER BY d.created_at DESC
		LIMIT $3 OFFSET $4`,
		pacienteID, opts.Search, opts.Limit, opts.Offset,
	)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	lista := make([]entities.Derivacion, 0)
	for rows.Next() {
		d, err := scanDerivacion(rows)
		if err != nil {
			return nil, mapError(err)
		}
		lista = append(lista, *d)
	}
	return lista, mapError(rows.Err())
}

func (s *DerivacionStore) MarcarVencidas(ctx context.Context, ahora time.Time) error {
	_, err := s.db.Exec(ctx, `
		UPDATE derivaciones
		SET estado = 'vencida', version = version + 1
		WHERE estado IN ('pendiente', 'agendada')
		  AND vigente_hasta <= $1`, ahora)
	return mapError(err)
}

func scanDerivacion(row rowScanner) (*entities.Derivacion, error) {
	var d entities.Derivacion
	var medicoDestinoID pgtype.Int8
	var prioridad, estado string
	err := row.Scan(
		&d.ID, &d.PacienteID, &d.PacienteNombre,
		&d.MedicoOrigenID, &d.MedicoOrigenNombre, &d.CitaOrigenID,
		&d.EspecialidadDestinoID, &d.EspecialidadDestino,
		&medicoDestinoID, &d.MedicoDestinoNombre,
		&d.Motivo, &prioridad, &estado,
		&d.VigenteDesde, &d.VigenteHasta, &d.MotivoCancelacion,
		&d.Version, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	d.Prioridad = entities.PrioridadDerivacion(prioridad)
	d.Estado = entities.EstadoDerivacion(estado)
	if medicoDestinoID.Valid {
		id := medicoDestinoID.Int64
		d.MedicoDestinoID = &id
	}
	return &d, nil
}
