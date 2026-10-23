package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"medilist/internal/service/entities"
)

// MedicoStore implementa service.MedicoStore sobre PostgreSQL.
type MedicoStore struct {
	db *pgxpool.Pool
}

// NewMedicoStore construye el almacén.
func NewMedicoStore(db *pgxpool.Pool) *MedicoStore {
	return &MedicoStore{db: db}
}

const medicoSelect = `
	SELECT
		m.id, m.especialidad_id, m.numero_licencia, m.tipo_profesional,
		m.activo, m.version, m.created_at, m.updated_at,
		pe.id, pe.tipo_documento, pe.numero_documento, pe.nombres, pe.apellidos,
		pe.fecha_nacimiento, pe.sexo, pe.telefono, pe.email, pe.direccion,
		e.id, e.codigo, e.nombre, e.descripcion, e.created_at, e.updated_at
	FROM medicos m
	JOIN personas pe ON pe.id = m.persona_id
	JOIN especialidades e ON e.id = m.especialidad_id`

func (s *MedicoStore) Crear(ctx context.Context, m *entities.Medico) error {
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
		m.Persona.TipoDocumento, m.Persona.NumeroDocumento, m.Persona.Nombres,
		m.Persona.Apellidos, m.Persona.FechaNacimiento, m.Persona.Sexo,
		m.Persona.Telefono, m.Persona.Email, m.Persona.Direccion,
	).Scan(&m.Persona.ID)
	if err != nil {
		return mapError(err)
	}

	const insertMedico = `
		INSERT INTO medicos
			(persona_id, especialidad_id, numero_licencia, tipo_profesional, activo)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, version, created_at, updated_at`
	err = tx.QueryRow(ctx, insertMedico,
		m.Persona.ID, m.EspecialidadID, m.NumeroLicencia, m.TipoProfesional, m.Activo,
	).Scan(&m.ID, &m.Version, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return mapError(err)
	}
	return mapError(tx.Commit(ctx))
}

func (s *MedicoStore) ObtenerPorID(ctx context.Context, id int64) (*entities.Medico, error) {
	m, err := scanMedico(s.db.QueryRow(ctx, medicoSelect+` WHERE m.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, noEncontrado("médico", id)
	}
	if err != nil {
		return nil, mapError(err)
	}
	return m, nil
}

func (s *MedicoStore) Listar(ctx context.Context, opts entities.ListOptions) ([]entities.Medico, error) {
	q := medicoSelect + `
		WHERE (
			$1 = ''
			OR pe.numero_documento ILIKE '%' || $1 || '%'
			OR pe.nombres ILIKE '%' || $1 || '%'
			OR pe.apellidos ILIKE '%' || $1 || '%'
			OR m.numero_licencia ILIKE '%' || $1 || '%'
			OR e.nombre ILIKE '%' || $1 || '%'
		)
		AND (NOT $2 OR m.activo)
		ORDER BY pe.apellidos ASC, pe.nombres ASC
		LIMIT $3 OFFSET $4`
	rows, err := s.db.Query(ctx, q, opts.Search, opts.SoloActivos, opts.Limit, opts.Offset)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	lista := make([]entities.Medico, 0)
	for rows.Next() {
		m, err := scanMedico(rows)
		if err != nil {
			return nil, mapError(err)
		}
		lista = append(lista, *m)
	}
	return lista, mapError(rows.Err())
}

func (s *MedicoStore) Actualizar(ctx context.Context, m *entities.Medico) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return mapError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const updateMedico = `
		UPDATE medicos
		SET especialidad_id = $2,
			numero_licencia = $3,
			tipo_profesional = $4,
			version = version + 1
		WHERE id = $1 AND version = $5
		RETURNING persona_id`
	var personaID int64
	err = tx.QueryRow(ctx, updateMedico, m.ID,
		m.EspecialidadID, m.NumeroLicencia, m.TipoProfesional, m.Version,
	).Scan(&personaID)
	if errors.Is(err, pgx.ErrNoRows) {
		return conflictoConcurrencia("médico", m.ID)
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
		m.Persona.TipoDocumento, m.Persona.NumeroDocumento, m.Persona.Nombres,
		m.Persona.Apellidos, m.Persona.FechaNacimiento, m.Persona.Sexo,
		m.Persona.Telefono, m.Persona.Email, m.Persona.Direccion,
	)
	if err != nil {
		return mapError(err)
	}
	return mapError(tx.Commit(ctx))
}

// Eliminar es una baja lógica para preservar las citas históricas.
func (s *MedicoStore) Eliminar(ctx context.Context, id int64) error {
	tag, err := s.db.Exec(ctx, `
		UPDATE medicos
		SET activo = false, version = version + 1
		WHERE id = $1`, id)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return noEncontrado("médico", id)
	}
	return nil
}

func scanMedico(row rowScanner) (*entities.Medico, error) {
	var m entities.Medico
	err := row.Scan(
		&m.ID, &m.EspecialidadID, &m.NumeroLicencia, &m.TipoProfesional,
		&m.Activo, &m.Version, &m.CreatedAt, &m.UpdatedAt,
		&m.Persona.ID, &m.Persona.TipoDocumento, &m.Persona.NumeroDocumento,
		&m.Persona.Nombres, &m.Persona.Apellidos, &m.Persona.FechaNacimiento,
		&m.Persona.Sexo, &m.Persona.Telefono, &m.Persona.Email, &m.Persona.Direccion,
		&m.Especialidad.ID, &m.Especialidad.Codigo, &m.Especialidad.Nombre,
		&m.Especialidad.Descripcion, &m.Especialidad.CreatedAt, &m.Especialidad.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &m, nil
}
