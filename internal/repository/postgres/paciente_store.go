package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"medilist/internal/service/entities"
)

// PacienteStore implementa service.PacienteStore sobre PostgreSQL.
type PacienteStore struct {
	db *pgxpool.Pool
}

// NewPacienteStore construye el almacén.
func NewPacienteStore(db *pgxpool.Pool) *PacienteStore {
	return &PacienteStore{db: db}
}

const pacienteSelect = `
	SELECT
		p.id, p.numero_historia, p.grupo_sanguineo, p.factor_rh,
		p.contacto_emergencia_nombre, p.contacto_emergencia_telefono,
		p.activo, p.created_at, p.updated_at,
		pe.id, pe.tipo_documento, pe.numero_documento, pe.nombres, pe.apellidos,
		pe.fecha_nacimiento, pe.sexo, pe.telefono, pe.email, pe.direccion
	FROM pacientes p
	JOIN personas pe ON pe.id = p.persona_id`

func (s *PacienteStore) Crear(ctx context.Context, p *entities.Paciente) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return mapError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const insertPersona = `
		INSERT INTO personas
			(tipo_documento, numero_documento, nombres, apellidos,
			 fecha_nacimiento, sexo, telefono, email, direccion)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id`
	err = tx.QueryRow(ctx, insertPersona,
		p.Persona.TipoDocumento, p.Persona.NumeroDocumento, p.Persona.Nombres,
		p.Persona.Apellidos, p.Persona.FechaNacimiento, p.Persona.Sexo,
		p.Persona.Telefono, p.Persona.Email, p.Persona.Direccion,
	).Scan(&p.Persona.ID)
	if err != nil {
		return mapError(err)
	}

	const insertPaciente = `
		INSERT INTO pacientes
			(persona_id, grupo_sanguineo, factor_rh,
			 contacto_emergencia_nombre, contacto_emergencia_telefono, activo)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, numero_historia, created_at, updated_at`
	err = tx.QueryRow(ctx, insertPaciente,
		p.Persona.ID, p.GrupoSanguineo, p.FactorRH,
		p.ContactoEmergenciaNombre, p.ContactoEmergenciaTelefono, p.Activo,
	).Scan(&p.ID, &p.NumeroHistoria, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return mapError(err)
	}
	return mapError(tx.Commit(ctx))
}

func (s *PacienteStore) ObtenerPorID(ctx context.Context, id int64) (*entities.Paciente, error) {
	p, err := scanPaciente(s.db.QueryRow(ctx, pacienteSelect+` WHERE p.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, noEncontrado("paciente", id)
	}
	if err != nil {
		return nil, mapError(err)
	}
	return p, nil
}

func (s *PacienteStore) Listar(ctx context.Context, opts entities.ListOptions) ([]entities.Paciente, error) {
	q := pacienteSelect + `
		WHERE (
			$1 = ''
			OR pe.numero_documento ILIKE '%' || $1 || '%'
			OR pe.nombres ILIKE '%' || $1 || '%'
			OR pe.apellidos ILIKE '%' || $1 || '%'
			OR p.numero_historia ILIKE '%' || $1 || '%'
		)
		AND (NOT $2 OR p.activo)
		ORDER BY p.id DESC
		LIMIT $3 OFFSET $4`
	rows, err := s.db.Query(ctx, q, opts.Search, opts.SoloActivos, opts.Limit, opts.Offset)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	lista := make([]entities.Paciente, 0)
	for rows.Next() {
		p, err := scanPaciente(rows)
		if err != nil {
			return nil, mapError(err)
		}
		lista = append(lista, *p)
	}
	return lista, mapError(rows.Err())
}

func (s *PacienteStore) Actualizar(ctx context.Context, p *entities.Paciente) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return mapError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const updatePaciente = `
		UPDATE pacientes
		SET grupo_sanguineo = $2,
			factor_rh = $3,
			contacto_emergencia_nombre = $4,
			contacto_emergencia_telefono = $5
		WHERE id = $1
		RETURNING persona_id`
	var personaID int64
	err = tx.QueryRow(ctx, updatePaciente, p.ID,
		p.GrupoSanguineo, p.FactorRH,
		p.ContactoEmergenciaNombre, p.ContactoEmergenciaTelefono,
	).Scan(&personaID)
	if errors.Is(err, pgx.ErrNoRows) {
		return noEncontrado("paciente", p.ID)
	}
	if err != nil {
		return mapError(err)
	}

	const updatePersona = `
		UPDATE personas
		SET tipo_documento = $2,
			numero_documento = $3,
			nombres = $4,
			apellidos = $5,
			fecha_nacimiento = $6,
			sexo = $7,
			telefono = $8,
			email = $9,
			direccion = $10
		WHERE id = $1`
	_, err = tx.Exec(ctx, updatePersona, personaID,
		p.Persona.TipoDocumento, p.Persona.NumeroDocumento, p.Persona.Nombres,
		p.Persona.Apellidos, p.Persona.FechaNacimiento, p.Persona.Sexo,
		p.Persona.Telefono, p.Persona.Email, p.Persona.Direccion,
	)
	if err != nil {
		return mapError(err)
	}
	return mapError(tx.Commit(ctx))
}

// Eliminar es una baja lógica para preservar la historia clínica.
func (s *PacienteStore) Eliminar(ctx context.Context, id int64) error {
	tag, err := s.db.Exec(ctx, `UPDATE pacientes SET activo = false WHERE id = $1`, id)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return noEncontrado("paciente", id)
	}
	return nil
}

func scanPaciente(row rowScanner) (*entities.Paciente, error) {
	var p entities.Paciente
	err := row.Scan(
		&p.ID, &p.NumeroHistoria, &p.GrupoSanguineo, &p.FactorRH,
		&p.ContactoEmergenciaNombre, &p.ContactoEmergenciaTelefono,
		&p.Activo, &p.CreatedAt, &p.UpdatedAt,
		&p.Persona.ID, &p.Persona.TipoDocumento, &p.Persona.NumeroDocumento,
		&p.Persona.Nombres, &p.Persona.Apellidos, &p.Persona.FechaNacimiento,
		&p.Persona.Sexo, &p.Persona.Telefono, &p.Persona.Email, &p.Persona.Direccion,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}
