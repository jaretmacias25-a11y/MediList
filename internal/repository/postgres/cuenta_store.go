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

// CuentaStore implementa service.CuentaStore sobre PostgreSQL.
type CuentaStore struct {
	db *pgxpool.Pool
}

// NewCuentaStore construye el almacén.
func NewCuentaStore(db *pgxpool.Pool) *CuentaStore {
	return &CuentaStore{db: db}
}

const insertarUsuarioCuentaSQL = `
	INSERT INTO usuarios (
		username, email, password_hash, nombres, apellidos, activo,
		email_verificado_en, terminos_aceptados_en, version_terminos,
		debe_cambiar_password
	)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	RETURNING id, version, created_at, updated_at`

func insertarUsuarioCuenta(ctx context.Context, tx pgx.Tx, u *entities.Usuario) error {
	err := tx.QueryRow(ctx, insertarUsuarioCuentaSQL,
		u.Username, u.Email, u.PasswordHash, u.Nombres, u.Apellidos, u.Activo,
		u.EmailVerificadoEn, u.TerminosAceptadosEn, u.VersionTerminos,
		u.DebeCambiarPassword,
	).Scan(&u.ID, &u.Version, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return mapError(err)
	}
	return asignarRoles(ctx, tx, u.ID, u.Roles)
}

const insertarTokenCuentaSQL = `
	INSERT INTO tokens_cuenta (usuario_id, tipo, token_hash, expira_en)
	VALUES ($1, $2, $3, $4)
	RETURNING id, created_at`

func insertarTokenCuenta(ctx context.Context, tx pgx.Tx, token *entities.TokenCuenta) error {
	err := tx.QueryRow(ctx, insertarTokenCuentaSQL,
		token.UsuarioID, string(token.Tipo), token.TokenHash, token.ExpiraEn,
	).Scan(&token.ID, &token.CreatedAt)
	return mapError(err)
}

const insertarAuditoriaCuentaSQL = `
	INSERT INTO auditoria
		(usuario_id, username, accion, recurso, metodo, ruta, estado_http, ip, user_agent, trace_id)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

func insertarAuditoriaCuenta(ctx context.Context, tx pgx.Tx, a *entities.Auditoria) error {
	if a == nil {
		return nil
	}
	_, err := tx.Exec(ctx, insertarAuditoriaCuentaSQL,
		a.UsuarioID, a.Username, a.Accion, a.Recurso, a.Metodo, a.Ruta,
		a.EstadoHTTP, a.IP, a.UserAgent, a.TraceID,
	)
	return mapError(err)
}

// RegistrarPacienteConCuenta crea persona, usuario, rol, paciente, token y
// auditoría en una sola transacción. Si cualquier paso falla, no queda una
// historia clínica huérfana ni una cuenta a medio crear.
func (s *CuentaStore) RegistrarPacienteConCuenta(
	ctx context.Context,
	usuario *entities.Usuario,
	paciente *entities.Paciente,
	token *entities.TokenCuenta,
	auditoria *entities.Auditoria,
) error {
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
		paciente.Persona.TipoDocumento, paciente.Persona.NumeroDocumento,
		paciente.Persona.Nombres, paciente.Persona.Apellidos,
		paciente.Persona.FechaNacimiento, paciente.Persona.Sexo,
		paciente.Persona.Telefono, paciente.Persona.Email, paciente.Persona.Direccion,
	).Scan(&paciente.Persona.ID)
	if err != nil {
		return mapError(err)
	}

	if err := insertarUsuarioCuenta(ctx, tx, usuario); err != nil {
		return err
	}

	paciente.UsuarioID = &usuario.ID
	const insertPaciente = `
		INSERT INTO pacientes
			(persona_id, usuario_id, grupo_sanguineo, factor_rh,
			 contacto_emergencia_nombre, contacto_emergencia_telefono, activo)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, numero_historia, version, created_at, updated_at`
	err = tx.QueryRow(ctx, insertPaciente,
		paciente.Persona.ID, paciente.UsuarioID, paciente.GrupoSanguineo, paciente.FactorRH,
		paciente.ContactoEmergenciaNombre, paciente.ContactoEmergenciaTelefono, paciente.Activo,
	).Scan(&paciente.ID, &paciente.NumeroHistoria, &paciente.Version,
		&paciente.CreatedAt, &paciente.UpdatedAt)
	if err != nil {
		return mapError(err)
	}

	token.UsuarioID = usuario.ID
	if err := insertarTokenCuenta(ctx, tx, token); err != nil {
		return err
	}

	if auditoria != nil {
		auditoria.UsuarioID = &usuario.ID
		auditoria.Username = usuario.Username
	}
	if err := insertarAuditoriaCuenta(ctx, tx, auditoria); err != nil {
		return err
	}
	return mapError(tx.Commit(ctx))
}

// RegistrarCuentaPendienteVinculacion crea una cuenta y una solicitud cuando
// el documento ya tiene una historia clínica sin usuario. No crea personas ni
// pacientes adicionales.
func (s *CuentaStore) RegistrarCuentaPendienteVinculacion(
	ctx context.Context,
	usuario *entities.Usuario,
	pacienteID int64,
	token *entities.TokenCuenta,
	solicitud *entities.SolicitudVinculacion,
	auditoria *entities.Auditoria,
) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return mapError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var usuarioVinculado pgtype.Int8
	err = tx.QueryRow(ctx, `
		SELECT usuario_id
		FROM pacientes
		WHERE id = $1
		FOR UPDATE`, pacienteID).Scan(&usuarioVinculado)
	if errors.Is(err, pgx.ErrNoRows) {
		return noEncontrado("paciente", pacienteID)
	}
	if err != nil {
		return mapError(err)
	}
	if usuarioVinculado.Valid {
		return fmt.Errorf("%w: la historia clínica ya tiene una cuenta vinculada", entities.ErrConflict)
	}

	var existePendiente bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM solicitudes_vinculacion
			WHERE paciente_id = $1 AND estado = 'pendiente'
		)`, pacienteID).Scan(&existePendiente); err != nil {
		return mapError(err)
	}
	if existePendiente {
		return fmt.Errorf("%w: ya existe una solicitud de vinculación pendiente para esta historia clínica", entities.ErrConflict)
	}

	if err := insertarUsuarioCuenta(ctx, tx, usuario); err != nil {
		return err
	}

	token.UsuarioID = usuario.ID
	if err := insertarTokenCuenta(ctx, tx, token); err != nil {
		return err
	}

	solicitud.UsuarioID = usuario.ID
	solicitud.PacienteID = pacienteID
	solicitud.Estado = entities.SolicitudVinculacionPendiente
	solicitud.Username = usuario.Username
	solicitud.Email = usuario.Email
	solicitud.Nombres = usuario.Nombres
	solicitud.Apellidos = usuario.Apellidos

	const insertSolicitud = `
		INSERT INTO solicitudes_vinculacion
			(usuario_id, paciente_id, estado, motivo)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at`
	err = tx.QueryRow(ctx, insertSolicitud,
		solicitud.UsuarioID, solicitud.PacienteID, string(solicitud.Estado), solicitud.Motivo,
	).Scan(&solicitud.ID, &solicitud.CreatedAt, &solicitud.UpdatedAt)
	if err != nil {
		return mapError(err)
	}

	if auditoria != nil {
		auditoria.UsuarioID = &usuario.ID
		auditoria.Username = usuario.Username
	}
	if err := insertarAuditoriaCuenta(ctx, tx, auditoria); err != nil {
		return err
	}
	return mapError(tx.Commit(ctx))
}

// ReemplazarToken invalida los tokens activos del mismo tipo y guarda uno
// nuevo. Así un reenvío no deja varios enlaces válidos simultáneamente.
func (s *CuentaStore) ReemplazarToken(ctx context.Context, token *entities.TokenCuenta) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return mapError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		UPDATE tokens_cuenta
		SET usado_en = now()
		WHERE usuario_id = $1 AND tipo = $2 AND usado_en IS NULL`,
		token.UsuarioID, string(token.Tipo),
	); err != nil {
		return mapError(err)
	}
	if err := insertarTokenCuenta(ctx, tx, token); err != nil {
		return err
	}
	return mapError(tx.Commit(ctx))
}

const usuarioCuentaPorIDSQL = usuarioSelect + ` WHERE u.id = $1 GROUP BY u.id`

func usuarioCuentaPorID(ctx context.Context, tx pgx.Tx, id int64) (*entities.Usuario, error) {
	u, err := scanUsuario(tx.QueryRow(ctx, usuarioCuentaPorIDSQL, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, noEncontrado("usuario", id)
	}
	if err != nil {
		return nil, mapError(err)
	}
	return u, nil
}

// VerificarEmail consume el token y marca el correo como verificado dentro de
// una transacción. Un segundo intento con el mismo token no encuentra fila
// porque el token ya quedó usado.
func (s *CuentaStore) VerificarEmail(
	ctx context.Context,
	tokenHash string,
	cuando time.Time,
	auditoria *entities.Auditoria,
) (*entities.Usuario, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var usuarioID int64
	err = tx.QueryRow(ctx, `
		UPDATE tokens_cuenta
		SET usado_en = $2
		WHERE token_hash = $1
		  AND tipo = 'verificacion_email'
		  AND usado_en IS NULL
		  AND expira_en > $2
		RETURNING usuario_id`, tokenHash, cuando).Scan(&usuarioID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, entities.NewValidationError("el token de verificación es inválido, ya fue usado o expiró")
	}
	if err != nil {
		return nil, mapError(err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE usuarios
		SET email_verificado_en = COALESCE(email_verificado_en, $2),
			version = version + 1
		WHERE id = $1`, usuarioID, cuando); err != nil {
		return nil, mapError(err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE tokens_cuenta
		SET usado_en = $2
		WHERE usuario_id = $1
		  AND tipo = 'verificacion_email'
		  AND usado_en IS NULL`, usuarioID, cuando); err != nil {
		return nil, mapError(err)
	}

	usuario, err := usuarioCuentaPorID(ctx, tx, usuarioID)
	if err != nil {
		return nil, err
	}
	if auditoria != nil {
		auditoria.UsuarioID = &usuario.ID
		auditoria.Username = usuario.Username
	}
	if err := insertarAuditoriaCuenta(ctx, tx, auditoria); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, mapError(err)
	}
	return usuario, nil
}

// RestablecerPassword consume el token, cambia la contraseña, verifica el
// correo si aún no lo estaba y revoca todas las sesiones del usuario.
func (s *CuentaStore) RestablecerPassword(
	ctx context.Context,
	tokenHash, passwordHash string,
	cuando time.Time,
	auditoria *entities.Auditoria,
) (*entities.Usuario, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var usuarioID int64
	err = tx.QueryRow(ctx, `
		UPDATE tokens_cuenta
		SET usado_en = $2
		WHERE token_hash = $1
		  AND tipo = 'reset_password'
		  AND usado_en IS NULL
		  AND expira_en > $2
		RETURNING usuario_id`, tokenHash, cuando).Scan(&usuarioID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, entities.NewValidationError("el token de recuperación es inválido, ya fue usado o expiró")
	}
	if err != nil {
		return nil, mapError(err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE usuarios
		SET password_hash = $2,
			email_verificado_en = COALESCE(email_verificado_en, $3),
			token_version = token_version + 1,
			intentos_fallidos = 0,
			bloqueado_hasta = NULL,
			debe_cambiar_password = FALSE,
			version = version + 1
		WHERE id = $1`, usuarioID, passwordHash, cuando); err != nil {
		return nil, mapError(err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE refresh_tokens
		SET revocado_en = $2
		WHERE usuario_id = $1 AND revocado_en IS NULL`, usuarioID, cuando); err != nil {
		return nil, mapError(err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE tokens_cuenta
		SET usado_en = $2
		WHERE usuario_id = $1
		  AND tipo = 'reset_password'
		  AND usado_en IS NULL`, usuarioID, cuando); err != nil {
		return nil, mapError(err)
	}

	usuario, err := usuarioCuentaPorID(ctx, tx, usuarioID)
	if err != nil {
		return nil, err
	}
	if auditoria != nil {
		auditoria.UsuarioID = &usuario.ID
		auditoria.Username = usuario.Username
	}
	if err := insertarAuditoriaCuenta(ctx, tx, auditoria); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, mapError(err)
	}
	return usuario, nil
}

const solicitudVinculacionSelect = `
	SELECT
		s.id, s.usuario_id, u.username, u.email,
		s.paciente_id, p.numero_historia,
		pe.nombres, pe.apellidos, pe.tipo_documento, pe.numero_documento,
		s.estado, s.motivo, s.resuelta_por_usuario_id, s.resuelta_en,
		s.created_at, s.updated_at
	FROM solicitudes_vinculacion s
	JOIN usuarios u ON u.id = s.usuario_id
	JOIN pacientes p ON p.id = s.paciente_id
	JOIN personas pe ON pe.id = p.persona_id`

func (s *CuentaStore) ListarSolicitudesVinculacion(
	ctx context.Context,
	filtro entities.SolicitudVinculacionFiltro,
) ([]entities.SolicitudVinculacion, error) {
	q := solicitudVinculacionSelect + `
		WHERE ($1 = '' OR s.estado = $1)
		  AND (
			$2 = ''
			OR u.username ILIKE '%' || $2 || '%'
			OR u.email ILIKE '%' || $2 || '%'
			OR pe.numero_documento ILIKE '%' || $2 || '%'
			OR pe.nombres ILIKE '%' || $2 || '%'
			OR pe.apellidos ILIKE '%' || $2 || '%'
			OR p.numero_historia ILIKE '%' || $2 || '%'
		  )
		ORDER BY s.created_at DESC
		LIMIT $3 OFFSET $4`
	rows, err := s.db.Query(ctx, q,
		string(filtro.Estado), filtro.Search, filtro.Limit, filtro.Offset,
	)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	lista := make([]entities.SolicitudVinculacion, 0)
	for rows.Next() {
		solicitud, err := scanSolicitudVinculacion(rows)
		if err != nil {
			return nil, mapError(err)
		}
		lista = append(lista, *solicitud)
	}
	return lista, mapError(rows.Err())
}

func scanSolicitudVinculacion(row rowScanner) (*entities.SolicitudVinculacion, error) {
	var s entities.SolicitudVinculacion
	var estado string
	var resueltaPor pgtype.Int8
	var resueltaEn pgtype.Timestamptz
	err := row.Scan(
		&s.ID, &s.UsuarioID, &s.Username, &s.Email,
		&s.PacienteID, &s.NumeroHistoria,
		&s.Nombres, &s.Apellidos, &s.TipoDocumento, &s.NumeroDocumento,
		&estado, &s.Motivo, &resueltaPor, &resueltaEn,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	s.Estado = entities.EstadoSolicitudVinculacion(estado)
	if resueltaPor.Valid {
		id := resueltaPor.Int64
		s.ResueltaPorUsuarioID = &id
	}
	if resueltaEn.Valid {
		t := resueltaEn.Time
		s.ResueltaEn = &t
	}
	return &s, nil
}

func solicitudVinculacionPorID(ctx context.Context, tx pgx.Tx, id int64) (*entities.SolicitudVinculacion, error) {
	solicitud, err := scanSolicitudVinculacion(
		tx.QueryRow(ctx, solicitudVinculacionSelect+` WHERE s.id = $1`, id),
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, noEncontrado("solicitud de vinculación", id)
	}
	if err != nil {
		return nil, mapError(err)
	}
	return solicitud, nil
}

// AprobarSolicitudVinculacion vincula la cuenta con la historia clínica y
// resuelve la solicitud en la misma transacción.
func (s *CuentaStore) AprobarSolicitudVinculacion(
	ctx context.Context,
	solicitudID, actorID int64,
	cuando time.Time,
) (*entities.SolicitudVinculacion, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var pacienteID, usuarioID int64
	var estado string
	err = tx.QueryRow(ctx, `
		SELECT paciente_id, usuario_id, estado
		FROM solicitudes_vinculacion
		WHERE id = $1
		FOR UPDATE`, solicitudID).Scan(&pacienteID, &usuarioID, &estado)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, noEncontrado("solicitud de vinculación", solicitudID)
	}
	if err != nil {
		return nil, mapError(err)
	}
	if estado != string(entities.SolicitudVinculacionPendiente) {
		return nil, fmt.Errorf("%w: la solicitud ya fue resuelta", entities.ErrConflict)
	}

	if err := vincularPacienteTx(ctx, tx, pacienteID, usuarioID, &solicitudID, actorID, cuando, "Solicitud aprobada"); err != nil {
		return nil, err
	}
	solicitud, err := solicitudVinculacionPorID(ctx, tx, solicitudID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, mapError(err)
	}
	return solicitud, nil
}

// RechazarSolicitudVinculacion deja constancia del rechazo y desactiva la
// cuenta pendiente para que no quede operativa sin perfil clínico.
func (s *CuentaStore) RechazarSolicitudVinculacion(
	ctx context.Context,
	solicitudID, actorID int64,
	motivo string,
	cuando time.Time,
) (*entities.SolicitudVinculacion, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var usuarioID int64
	var estado string
	err = tx.QueryRow(ctx, `
		SELECT usuario_id, estado
		FROM solicitudes_vinculacion
		WHERE id = $1
		FOR UPDATE`, solicitudID).Scan(&usuarioID, &estado)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, noEncontrado("solicitud de vinculación", solicitudID)
	}
	if err != nil {
		return nil, mapError(err)
	}
	if estado != string(entities.SolicitudVinculacionPendiente) {
		return nil, fmt.Errorf("%w: la solicitud ya fue resuelta", entities.ErrConflict)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE solicitudes_vinculacion
		SET estado = 'rechazada',
			motivo = $2,
			resuelta_por_usuario_id = $3,
			resuelta_en = $4
		WHERE id = $1`, solicitudID, motivo, nullIfZero(actorID), cuando); err != nil {
		return nil, mapError(err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE usuarios
		SET activo = false,
			token_version = token_version + 1,
			version = version + 1
		WHERE id = $1`, usuarioID); err != nil {
		return nil, mapError(err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE refresh_tokens
		SET revocado_en = $2
		WHERE usuario_id = $1 AND revocado_en IS NULL`, usuarioID, cuando); err != nil {
		return nil, mapError(err)
	}

	solicitud, err := solicitudVinculacionPorID(ctx, tx, solicitudID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, mapError(err)
	}
	return solicitud, nil
}

// VincularPaciente permite a recepción/administración vincular una cuenta ya
// creada con una historia clínica existente.
func (s *CuentaStore) VincularPaciente(
	ctx context.Context,
	pacienteID, usuarioID int64,
	solicitudID *int64,
	actorID int64,
	cuando time.Time,
) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return mapError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := vincularPacienteTx(
		ctx, tx, pacienteID, usuarioID, solicitudID, actorID, cuando, "Vinculación directa",
	); err != nil {
		return err
	}
	return mapError(tx.Commit(ctx))
}

// VincularMedico asocia una cuenta con un perfil profesional existente. Solo
// se permite una cuenta por médico y una cuenta por usuario.
func (s *CuentaStore) VincularMedico(
	ctx context.Context,
	medicoID, usuarioID int64,
	cuando time.Time,
) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return mapError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var vinculado pgtype.Int8
	err = tx.QueryRow(ctx, `
		SELECT usuario_id
		FROM medicos
		WHERE id = $1
		FOR UPDATE`, medicoID).Scan(&vinculado)
	if errors.Is(err, pgx.ErrNoRows) {
		return noEncontrado("médico", medicoID)
	}
	if err != nil {
		return mapError(err)
	}
	if vinculado.Valid {
		if vinculado.Int64 == usuarioID {
			return mapError(tx.Commit(ctx))
		}
		return fmt.Errorf("%w: el médico ya tiene otra cuenta vinculada", entities.ErrConflict)
	}

	var activo bool
	err = tx.QueryRow(ctx, `
		SELECT activo
		FROM usuarios
		WHERE id = $1
		FOR SHARE`, usuarioID).Scan(&activo)
	if errors.Is(err, pgx.ErrNoRows) {
		return noEncontrado("usuario", usuarioID)
	}
	if err != nil {
		return mapError(err)
	}
	if !activo {
		return entities.NewValidationError("la cuenta está inactiva")
	}

	var tieneRol bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM usuario_roles ur
			JOIN roles r ON r.id = ur.rol_id
			WHERE ur.usuario_id = $1 AND r.codigo = 'medico'
		)`, usuarioID).Scan(&tieneRol); err != nil {
		return mapError(err)
	}
	if !tieneRol {
		return entities.NewValidationError("la cuenta debe tener el rol medico")
	}

	var vinculadoOtro bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM medicos WHERE usuario_id = $1 AND id <> $2
		)`, usuarioID, medicoID).Scan(&vinculadoOtro); err != nil {
		return mapError(err)
	}
	if vinculadoOtro {
		return fmt.Errorf("%w: la cuenta ya está vinculada a otro médico", entities.ErrConflict)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE medicos
		SET usuario_id = $2, version = version + 1
		WHERE id = $1`, medicoID, usuarioID); err != nil {
		return mapError(err)
	}
	return mapError(tx.Commit(ctx))
}

func vincularPacienteTx(
	ctx context.Context,
	tx pgx.Tx,
	pacienteID, usuarioID int64,
	solicitudID *int64,
	actorID int64,
	cuando time.Time,
	motivoAprobacion string,
) error {
	var vinculado pgtype.Int8
	err := tx.QueryRow(ctx, `
		SELECT usuario_id
		FROM pacientes
		WHERE id = $1
		FOR UPDATE`, pacienteID).Scan(&vinculado)
	if errors.Is(err, pgx.ErrNoRows) {
		return noEncontrado("paciente", pacienteID)
	}
	if err != nil {
		return mapError(err)
	}
	if vinculado.Valid && vinculado.Int64 != usuarioID {
		return fmt.Errorf("%w: la historia clínica ya tiene otra cuenta vinculada", entities.ErrConflict)
	}

	if !vinculado.Valid {
		var activo bool
		err := tx.QueryRow(ctx, `
			SELECT activo
			FROM usuarios
			WHERE id = $1
			FOR SHARE`, usuarioID).Scan(&activo)
		if errors.Is(err, pgx.ErrNoRows) {
			return noEncontrado("usuario", usuarioID)
		}
		if err != nil {
			return mapError(err)
		}
		if !activo {
			return entities.NewValidationError("la cuenta está inactiva")
		}

		var tieneRol bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM usuario_roles ur
				JOIN roles r ON r.id = ur.rol_id
				WHERE ur.usuario_id = $1 AND r.codigo = 'paciente'
			)`, usuarioID).Scan(&tieneRol); err != nil {
			return mapError(err)
		}
		if !tieneRol {
			return entities.NewValidationError("la cuenta debe tener el rol paciente")
		}

		var vinculadoOtro bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM pacientes WHERE usuario_id = $1 AND id <> $2
			)`, usuarioID, pacienteID).Scan(&vinculadoOtro); err != nil {
			return mapError(err)
		}
		if vinculadoOtro {
			return fmt.Errorf("%w: la cuenta ya está vinculada a otro paciente", entities.ErrConflict)
		}

		if _, err := tx.Exec(ctx, `
			UPDATE pacientes
			SET usuario_id = $2, version = version + 1
			WHERE id = $1`, pacienteID, usuarioID); err != nil {
			return mapError(err)
		}
	}

	if solicitudID != nil {
		var solicitudPaciente, solicitudUsuario int64
		var estado string
		err := tx.QueryRow(ctx, `
			SELECT paciente_id, usuario_id, estado
			FROM solicitudes_vinculacion
			WHERE id = $1
			FOR UPDATE`, *solicitudID).Scan(&solicitudPaciente, &solicitudUsuario, &estado)
		if errors.Is(err, pgx.ErrNoRows) {
			return noEncontrado("solicitud de vinculación", *solicitudID)
		}
		if err != nil {
			return mapError(err)
		}
		if solicitudPaciente != pacienteID || solicitudUsuario != usuarioID {
			return entities.NewValidationError("la solicitud no corresponde al paciente y la cuenta indicados")
		}
		if estado != string(entities.SolicitudVinculacionPendiente) {
			return fmt.Errorf("%w: la solicitud ya fue resuelta", entities.ErrConflict)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE solicitudes_vinculacion
			SET estado = 'aprobada',
				motivo = $2,
				resuelta_por_usuario_id = $3,
				resuelta_en = $4
			WHERE id = $1`, *solicitudID, motivoAprobacion, nullIfZero(actorID), cuando); err != nil {
			return mapError(err)
		}
		return nil
	}

	var solicitudPendienteID, solicitudPendienteUsuario int64
	err = tx.QueryRow(ctx, `
		SELECT id, usuario_id
		FROM solicitudes_vinculacion
		WHERE paciente_id = $1 AND estado = 'pendiente'
		FOR UPDATE`, pacienteID).Scan(&solicitudPendienteID, &solicitudPendienteUsuario)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return mapError(err)
	}
	if solicitudPendienteUsuario != usuarioID {
		return fmt.Errorf("%w: existe una solicitud pendiente para otra cuenta; resuélvela primero", entities.ErrConflict)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE solicitudes_vinculacion
		SET estado = 'aprobada',
			motivo = $2,
			resuelta_por_usuario_id = $3,
			resuelta_en = $4
		WHERE id = $1`, solicitudPendienteID, motivoAprobacion, nullIfZero(actorID), cuando); err != nil {
		return mapError(err)
	}
	return nil
}

func nullIfZero(id int64) any {
	if id <= 0 {
		return nil
	}
	return id
}
