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
		c.canal, c.creada_por_usuario_id, c.derivacion_id, c.motivo_cancelacion,
		c.version, c.created_at, c.updated_at
	FROM citas c
	JOIN pacientes p ON p.id = c.paciente_id
	JOIN personas pe ON pe.id = p.persona_id
	JOIN medicos m ON m.id = c.medico_id
	JOIN personas me ON me.id = m.persona_id
	JOIN especialidades e ON e.id = m.especialidad_id`

func (s *CitaStore) Crear(ctx context.Context, c *entities.Cita) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return mapError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := crearCitaTx(ctx, tx, c); err != nil {
		return err
	}
	return mapError(tx.Commit(ctx))
}

// CrearConDerivacion bloquea la derivación, valida su disponibilidad e inserta
// la cita y el consumo de la derivación en la misma transacción.
func (s *CitaStore) CrearConDerivacion(
	ctx context.Context,
	c *entities.Cita,
	derivacionID int64,
) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return mapError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var pacienteID, especialidadID int64
	var estado string
	var medicoDestino pgtype.Int8
	var vigente bool
	err = tx.QueryRow(ctx, `
		SELECT paciente_id, especialidad_destino_id, estado,
		       medico_destino_id, vigente_hasta > now()
		FROM derivaciones
		WHERE id = $1
		FOR UPDATE`, derivacionID).Scan(
		&pacienteID, &especialidadID, &estado, &medicoDestino, &vigente,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return noEncontrado("derivación", derivacionID)
	}
	if err != nil {
		return mapError(err)
	}
	if pacienteID != c.PacienteID {
		return entities.NewValidationError(
			"la derivación no pertenece al paciente de la cita")
	}
	if estado != string(entities.EstadoDerivacionPendiente) {
		return fmt.Errorf(
			"%w: la derivación no está pendiente (estado: %s)", entities.ErrConflict, estado)
	}
	if !vigente {
		return fmt.Errorf("%w: la derivación está vencida", entities.ErrConflict)
	}
	if medicoDestino.Valid && medicoDestino.Int64 != c.MedicoID {
		return fmt.Errorf(
			"%w: la derivación está dirigida a otro médico", entities.ErrConflict)
	}
	var medicoEspecialidad int64
	err = tx.QueryRow(ctx, `
		SELECT especialidad_id FROM medicos WHERE id = $1`, c.MedicoID,
	).Scan(&medicoEspecialidad)
	if errors.Is(err, pgx.ErrNoRows) {
		return entities.NewValidationError("el médico de la cita no existe")
	}
	if err != nil {
		return mapError(err)
	}
	if medicoEspecialidad != especialidadID {
		return entities.NewValidationError(
			"el médico no pertenece a la especialidad de la derivación")
	}

	c.DerivacionID = &derivacionID
	if err := crearCitaTx(ctx, tx, c); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE derivaciones
		SET estado = 'agendada', version = version + 1
		WHERE id = $1 AND estado = 'pendiente'`, derivacionID)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: la derivación fue consumida por otra operación", entities.ErrConflict)
	}
	return mapError(tx.Commit(ctx))
}

func crearCitaTx(ctx context.Context, tx pgx.Tx, c *entities.Cita) error {
	// Se bloquean las filas de los participantes para que una desactivación
	// concurrente no pueda colarse entre la validación y el INSERT. La
	// restricción de exclusión sigue siendo la última línea de defensa para
	// el solapamiento de horarios.
	if err := validarParticipantesActivos(ctx, tx, c.PacienteID, c.MedicoID); err != nil {
		return err
	}

	canal := string(c.Canal)
	if canal == "" {
		canal = string(entities.CanalCitaRecepcion)
	}
	var creadaPor, derivacion any
	if c.CreadaPorUsuarioID != nil {
		creadaPor = *c.CreadaPorUsuarioID
	}
	if c.DerivacionID != nil {
		derivacion = *c.DerivacionID
	}
	const q = `
		INSERT INTO citas
			(paciente_id, medico_id, fecha_hora_inicio, duracion_min, tipo, motivo, estado,
			 canal, creada_por_usuario_id, derivacion_id, motivo_cancelacion)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, codigo, version, created_at, updated_at`
	err := tx.QueryRow(ctx, q,
		c.PacienteID, c.MedicoID, c.FechaHoraInicio, c.DuracionMin,
		string(c.Tipo), c.Motivo, string(c.Estado), canal, creadaPor, derivacion,
		c.MotivoCancelacion,
	).Scan(&c.ID, &c.Codigo, &c.Version, &c.CreatedAt, &c.UpdatedAt)
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

// Cancelar cambia el estado y registra el motivo dentro de la misma escritura
// optimista. Solo compite con citas activas.
func (s *CitaStore) Cancelar(ctx context.Context, c *entities.Cita) error {
	const q = `
		UPDATE citas
		SET estado = 'cancelada',
			motivo_cancelacion = $2,
			version = version + 1
		WHERE id = $1 AND version = $3
		  AND estado IN ('programada', 'confirmada')
		RETURNING version, updated_at`
	err := s.db.QueryRow(ctx, q, c.ID, c.MotivoCancelacion, c.Version).
		Scan(&c.Version, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return conflictoConcurrencia("cita", c.ID)
	}
	return mapError(err)
}

// CancelarConDerivacion cancela la cita y libera o consume la derivación en
// la misma transacción, según la política de cancelación.
func (s *CitaStore) CancelarConDerivacion(
	ctx context.Context,
	c *entities.Cita,
	liberar bool,
	motivoDerivacion string,
) error {
	if c.DerivacionID == nil {
		return entities.NewValidationError("la cita no tiene una derivación asociada")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return mapError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	err = tx.QueryRow(ctx, `
		UPDATE citas
		SET estado = 'cancelada',
			motivo_cancelacion = $2,
			version = version + 1
		WHERE id = $1 AND version = $3
		  AND estado IN ('programada', 'confirmada')
		RETURNING version, updated_at`,
		c.ID, c.MotivoCancelacion, c.Version,
	).Scan(&c.Version, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return conflictoConcurrencia("cita", c.ID)
	}
	if err != nil {
		return mapError(err)
	}

	if liberar {
		tag, err := tx.Exec(ctx, `
			UPDATE derivaciones
			SET estado = 'pendiente', motivo_cancelacion = '', version = version + 1
			WHERE id = $1
			  AND estado IN ('pendiente', 'agendada')
			  AND vigente_hasta > now()`, *c.DerivacionID)
		if err != nil {
			return mapError(err)
		}
		if tag.RowsAffected() == 0 {
			_, err = tx.Exec(ctx, `
				UPDATE derivaciones
				SET estado = 'vencida', version = version + 1
				WHERE id = $1
				  AND estado IN ('pendiente', 'agendada')
				  AND vigente_hasta <= now()`, *c.DerivacionID)
			if err != nil {
				return mapError(err)
			}
		}
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE derivaciones
			SET estado = 'cancelada',
				motivo_cancelacion = $2,
				version = version + 1
			WHERE id = $1
			  AND estado IN ('pendiente', 'agendada')`,
			*c.DerivacionID, motivoDerivacion)
		if err != nil {
			return mapError(err)
		}
	}
	return mapError(tx.Commit(ctx))
}

// CambiarEstadoConDerivacion actualiza la cita y el estado de la derivación
// (atendida o consumida por no-show) de forma atómica.
func (s *CitaStore) CambiarEstadoConDerivacion(
	ctx context.Context,
	c *entities.Cita,
	estadoDerivacion entities.EstadoDerivacion,
	motivoDerivacion string,
) error {
	if c.DerivacionID == nil {
		return entities.NewValidationError("la cita no tiene una derivación asociada")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return mapError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	err = tx.QueryRow(ctx, `
		UPDATE citas
		SET estado = $2, version = version + 1
		WHERE id = $1 AND version = $3
		  AND estado IN ('programada', 'confirmada')
		RETURNING version, updated_at`,
		c.ID, string(c.Estado), c.Version,
	).Scan(&c.Version, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return conflictoConcurrencia("cita", c.ID)
	}
	if err != nil {
		return mapError(err)
	}

	tag, err := tx.Exec(ctx, `
		UPDATE derivaciones
		SET estado = $2, motivo_cancelacion = $3, version = version + 1
		WHERE id = $1
		  AND estado IN ('pendiente', 'agendada')`,
		*c.DerivacionID, string(estadoDerivacion), motivoDerivacion)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: la derivación ya no está disponible", entities.ErrConflict)
	}
	return mapError(tx.Commit(ctx))
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
			estado = $8,
			version = version + 1
		WHERE id = $1 AND version = $9
		RETURNING version, updated_at`
	err := s.db.QueryRow(ctx, q,
		c.ID, c.PacienteID, c.MedicoID, c.FechaHoraInicio, c.DuracionMin,
		string(c.Tipo), c.Motivo, string(c.Estado), c.Version,
	).Scan(&c.Version, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return conflictoConcurrencia("cita", c.ID)
	}
	return mapError(err)
}

// ExisteSolape detecta citas activas del mismo médico que se cruzan en el tiempo.
// La condición clásica de solape es: inicioA < finB && finA > inicioB.
func (s *CitaStore) ExisteSolape(ctx context.Context, medicoID int64, inicio time.Time, duracionMin int, excluirID int64) (bool, error) {
	return s.existeSolape(ctx, "medico_id", medicoID, inicio, duracionMin, excluirID)
}

// ExisteSolapePaciente detecta que el paciente ya tiene otra cita activa en
// el mismo rango. Evita que una persona quede citada con dos médicos a la vez.
func (s *CitaStore) ExisteSolapePaciente(ctx context.Context, pacienteID int64, inicio time.Time, duracionMin int, excluirID int64) (bool, error) {
	return s.existeSolape(ctx, "paciente_id", pacienteID, inicio, duracionMin, excluirID)
}

func (s *CitaStore) existeSolape(
	ctx context.Context,
	columna string,
	recursoID int64,
	inicio time.Time,
	duracionMin int,
	excluirID int64,
) (bool, error) {
	switch columna {
	case "medico_id", "paciente_id":
	default:
		return false, fmt.Errorf("columna de solape no válida: %s", columna)
	}

	q := fmt.Sprintf(`
		SELECT EXISTS (
			SELECT 1
			FROM citas
			WHERE %s = $1
			  AND id <> $4
			  AND estado IN ('programada', 'confirmada')
			  AND fecha_hora_inicio < $3
			  AND cita_fin(fecha_hora_inicio, duracion_min) > $2
		)`, columna)
	fin := inicio.Add(time.Duration(duracionMin) * time.Minute)
	var existe bool
	err := s.db.QueryRow(ctx, q, recursoID, inicio, fin, excluirID).Scan(&existe)
	return existe, mapError(err)
}

func validarParticipantesActivos(ctx context.Context, tx pgx.Tx, pacienteID, medicoID int64) error {
	var activo bool
	err := tx.QueryRow(ctx, `
		SELECT activo FROM pacientes WHERE id = $1 FOR SHARE`, pacienteID).Scan(&activo)
	if errors.Is(err, pgx.ErrNoRows) {
		return entities.NewValidationError("el paciente %d no existe", pacienteID)
	}
	if err != nil {
		return mapError(err)
	}
	if !activo {
		return entities.NewValidationError("el paciente %d está inactivo", pacienteID)
	}

	err = tx.QueryRow(ctx, `
		SELECT activo FROM medicos WHERE id = $1 FOR SHARE`, medicoID).Scan(&activo)
	if errors.Is(err, pgx.ErrNoRows) {
		return entities.NewValidationError("el médico %d no existe", medicoID)
	}
	if err != nil {
		return mapError(err)
	}
	if !activo {
		return entities.NewValidationError("el médico %d está inactivo", medicoID)
	}
	return nil
}

func scanCita(row rowScanner) (*entities.Cita, error) {
	var c entities.Cita
	var tipo, estado, canal string
	var creadaPor, derivacion pgtype.Int8
	err := row.Scan(
		&c.ID, &c.Codigo, &c.PacienteID, &c.PacienteNombre,
		&c.MedicoID, &c.MedicoNombre, &c.EspecialidadNombre,
		&c.FechaHoraInicio, &c.DuracionMin, &tipo, &c.Motivo, &estado,
		&canal, &creadaPor, &derivacion, &c.MotivoCancelacion,
		&c.Version, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	c.Tipo = entities.TipoCita(tipo)
	c.Estado = entities.EstadoCita(estado)
	c.Canal = entities.CanalCita(canal)
	if creadaPor.Valid {
		id := creadaPor.Int64
		c.CreadaPorUsuarioID = &id
	}
	if derivacion.Valid {
		id := derivacion.Int64
		c.DerivacionID = &id
	}
	return &c, nil
}
