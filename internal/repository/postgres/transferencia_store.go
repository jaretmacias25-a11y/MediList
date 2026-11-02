package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"medilist/internal/service/entities"
)

// TransferenciaStore implementa service.TransferenciaStore sobre PostgreSQL.
type TransferenciaStore struct {
	db *pgxpool.Pool
}

// NewTransferenciaStore construye el almacén de transferencias internas.
func NewTransferenciaStore(db *pgxpool.Pool) *TransferenciaStore {
	return &TransferenciaStore{db: db}
}

const transferenciaSelect = `
	SELECT
		t.id, t.paciente_id,
		t.medico_anterior_id,
		COALESCE(ma_p.nombres || ' ' || ma_p.apellidos, '') AS medico_anterior_nombre,
		t.medico_nuevo_id,
		mnp.nombres || ' ' || mnp.apellidos AS medico_nuevo_nombre,
		t.motivo, t.creada_por_usuario_id,
		COALESCE(u.username, '') AS creada_por_username,
		t.created_at
	FROM transferencias_paciente t
	LEFT JOIN medicos ma ON ma.id = t.medico_anterior_id
	LEFT JOIN personas ma_p ON ma_p.id = ma.persona_id
	JOIN medicos mn ON mn.id = t.medico_nuevo_id
	JOIN personas mnp ON mnp.id = mn.persona_id
	LEFT JOIN usuarios u ON u.id = t.creada_por_usuario_id`

func (s *TransferenciaStore) Transferir(
	ctx context.Context,
	pacienteID, medicoNuevoID int64,
	motivo string,
	actorID int64,
	version int64,
) (*entities.TransferenciaPaciente, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var responsableID pgtype.Int8
	var actualVersion int64
	err = tx.QueryRow(ctx, `
		SELECT medico_responsable_id, version
		FROM pacientes
		WHERE id = $1
		FOR UPDATE`, pacienteID).Scan(&responsableID, &actualVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, noEncontrado("paciente", pacienteID)
	}
	if err != nil {
		return nil, mapError(err)
	}
	if version > 0 && version != actualVersion {
		return nil, conflictoConcurrencia("paciente", pacienteID)
	}
	if responsableID.Valid && responsableID.Int64 == medicoNuevoID {
		return nil, fmt.Errorf("%w: el paciente ya tiene asignado ese médico", entities.ErrConflict)
	}

	var medicoActivo bool
	err = tx.QueryRow(ctx, `
		SELECT activo FROM medicos WHERE id = $1 FOR SHARE`, medicoNuevoID).Scan(&medicoActivo)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, entities.NewValidationError("el médico destino %d no existe", medicoNuevoID)
	}
	if err != nil {
		return nil, mapError(err)
	}
	if !medicoActivo {
		return nil, entities.NewValidationError("el médico destino %d está inactivo", medicoNuevoID)
	}

	_, err = tx.Exec(ctx, `
		UPDATE pacientes
		SET medico_responsable_id = $2, version = version + 1
		WHERE id = $1`, pacienteID, medicoNuevoID)
	if err != nil {
		return nil, mapError(err)
	}

	var transferenciaID int64
	var creadaPor any
	if actorID > 0 {
		creadaPor = actorID
	}
	var anterior any
	if responsableID.Valid {
		anterior = responsableID.Int64
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO transferencias_paciente
			(paciente_id, medico_anterior_id, medico_nuevo_id, motivo, creada_por_usuario_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`,
		pacienteID, anterior, medicoNuevoID, motivo, creadaPor,
	).Scan(&transferenciaID)
	if err != nil {
		return nil, mapError(err)
	}

	transferencia, err := scanTransferencia(tx.QueryRow(
		ctx, transferenciaSelect+` WHERE t.id = $1`, transferenciaID,
	))
	if err != nil {
		return nil, mapError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, mapError(err)
	}
	return transferencia, nil
}

func (s *TransferenciaStore) ListarTransferencias(
	ctx context.Context,
	pacienteID int64,
) ([]entities.TransferenciaPaciente, error) {
	rows, err := s.db.Query(ctx, transferenciaSelect+`
		WHERE t.paciente_id = $1
		ORDER BY t.created_at DESC, t.id DESC`, pacienteID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	lista := make([]entities.TransferenciaPaciente, 0)
	for rows.Next() {
		t, err := scanTransferencia(rows)
		if err != nil {
			return nil, mapError(err)
		}
		lista = append(lista, *t)
	}
	return lista, mapError(rows.Err())
}

func scanTransferencia(row rowScanner) (*entities.TransferenciaPaciente, error) {
	var t entities.TransferenciaPaciente
	var anteriorID, creadaPorID pgtype.Int8
	err := row.Scan(
		&t.ID, &t.PacienteID,
		&anteriorID, &t.MedicoAnteriorNombre,
		&t.MedicoNuevoID, &t.MedicoNuevoNombre,
		&t.Motivo, &creadaPorID, &t.CreadaPorUsername, &t.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if anteriorID.Valid {
		id := anteriorID.Int64
		t.MedicoAnteriorID = &id
	}
	if creadaPorID.Valid {
		id := creadaPorID.Int64
		t.CreadaPorUsuarioID = &id
	}
	return &t, nil
}
