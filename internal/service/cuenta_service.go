package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"medilist/internal/auth"
	"medilist/internal/observability"
	"medilist/internal/service/entities"
)

const (
	auditStatusCreated       = 201
	auditStatusAccepted      = 202
	auditStatusBadRequest    = 400
	auditStatusForbidden     = 403
	auditStatusConflict      = 409
	auditStatusInternal      = 500
	auditStatusNotAuthorized = 401
)

// CuentaConfig agrupa las políticas del registro y los tokens de cuenta.
type CuentaConfig struct {
	TermsVersion    string
	MinEdadCuenta   int
	BcryptCost      int
	VerificationTTL time.Duration
	ResetTTL        time.Duration
	LinkBaseURL     string
}

// RegistroPacienteInput contiene los datos del autorregistro público.
type RegistroPacienteInput struct {
	Username        string
	Email           string
	Password        string
	Nombres         string
	Apellidos       string
	TipoDocumento   string
	NumeroDocumento string
	FechaNacimiento time.Time
	Sexo            string
	Telefono        string
	Direccion       string
	AceptaTerminos  bool
	VersionTerminos string
}

// CompletarRegistroPacienteInput contiene los datos clínicos que faltan en
// una cuenta creada con Clerk.
type CompletarRegistroPacienteInput struct {
	Nombres         string
	Apellidos       string
	TipoDocumento   string
	NumeroDocumento string
	FechaNacimiento time.Time
	Sexo            string
	Telefono        string
	Direccion       string
}

// RegistroPacienteResultado indica si se creó un paciente nuevo o una cuenta
// pendiente de vinculación con una historia clínica existente.
type RegistroPacienteResultado struct {
	Usuario              *entities.Usuario
	Paciente             *entities.Paciente
	Solicitud            *entities.SolicitudVinculacion
	VinculacionPendiente bool
}

// CuentaService contiene los casos de uso de registro, verificación,
// recuperación y vinculación de cuentas.
type CuentaService struct {
	cfg            CuentaConfig
	cuentas        CuentaStore
	pacientes      PacienteStore
	usuarios       UsuarioStore
	email          EmailSender
	especialidades EspecialidadStore
	auditoria      AuditoriaStore
	logger         *slog.Logger
}

// NewCuentaService construye el servicio de cuentas.
func NewCuentaService(
	cfg CuentaConfig,
	cuentas CuentaStore,
	pacientes PacienteStore,
	usuarios UsuarioStore,
	email EmailSender,
	auditoria AuditoriaStore,
	logger *slog.Logger,
	especialidades ...EspecialidadStore,
) *CuentaService {
	if cfg.TermsVersion == "" {
		cfg.TermsVersion = "2026-10"
	}
	if cfg.MinEdadCuenta < 0 {
		cfg.MinEdadCuenta = 0
	}
	if cfg.BcryptCost <= 0 {
		cfg.BcryptCost = 12
	}
	if cfg.VerificationTTL <= 0 {
		cfg.VerificationTTL = 24 * time.Hour
	}
	if cfg.ResetTTL <= 0 {
		cfg.ResetTTL = time.Hour
	}
	if strings.TrimSpace(cfg.LinkBaseURL) == "" {
		cfg.LinkBaseURL = "http://localhost:3000"
	}
	if logger == nil {
		logger = slog.Default()
	}
	var especialidadStore EspecialidadStore
	if len(especialidades) > 0 {
		especialidadStore = especialidades[0]
	}
	return &CuentaService{
		cfg:            cfg,
		cuentas:        cuentas,
		pacientes:      pacientes,
		usuarios:       usuarios,
		email:          email,
		especialidades: especialidadStore,
		auditoria:      auditoria,
		logger:         logger,
	}
}

// RegistrarPaciente crea una cuenta y, si el documento no existe, también la
// persona y la historia clínica. Si el documento ya existe sin cuenta, crea
// una solicitud de vinculación en lugar de duplicar el paciente.
func (s *CuentaService) RegistrarPaciente(
	ctx context.Context,
	in RegistroPacienteInput,
	ip, userAgent string,
) (*RegistroPacienteResultado, error) {
	usuario, paciente, err := s.validarRegistro(in)
	if err != nil {
		s.auditar(ctx, nil, strings.ToLower(strings.TrimSpace(in.Username)), "registro_fallido",
			ip, userAgent, auditStatusBadRequest)
		return nil, err
	}

	hash, err := hashearPassword(strings.TrimSpace(in.Password), s.cfg.BcryptCost)
	if err != nil {
		return nil, err
	}
	usuario.PasswordHash = hash

	token, tokenPlano, err := s.nuevoToken(0, entities.TokenVerificacionEmail, s.cfg.VerificationTTL)
	if err != nil {
		return nil, err
	}
	auditoria := s.auditoriaCuenta("auth", "registro", "POST", "/api/v1/auth/register", ip, userAgent, auditStatusCreated)

	existente, err := s.pacientes.BuscarPorDocumento(ctx,
		paciente.Persona.TipoDocumento, paciente.Persona.NumeroDocumento)
	switch {
	case err == nil:
		if existente.UsuarioID != nil {
			s.auditar(ctx, nil, usuario.Username, "registro_documento_con_cuenta",
				ip, userAgent, auditStatusConflict)
			return nil, fmt.Errorf("%w: el documento ya tiene una cuenta vinculada; usa la recuperación de contraseña o contacta a recepción", entities.ErrConflict)
		}

		solicitud := &entities.SolicitudVinculacion{}
		auditoria.EstadoHTTP = auditStatusAccepted
		err = s.cuentas.RegistrarCuentaPendienteVinculacion(
			ctx, usuario, existente.ID, token, solicitud, auditoria,
		)
		if err != nil {
			s.auditar(ctx, nil, usuario.Username, "registro_pendiente_fallido",
				ip, userAgent, auditStatusConflict)
			return nil, err
		}
		s.enviarVerificacion(ctx, usuario, tokenPlano, token.ExpiraEn)
		observability.IncRegistro()
		return &RegistroPacienteResultado{
			Usuario: usuario, Solicitud: solicitud, VinculacionPendiente: true,
		}, nil

	case errors.Is(err, entities.ErrNotFound):
		err = s.cuentas.RegistrarPacienteConCuenta(ctx, usuario, paciente, token, auditoria)
		if err != nil {
			s.auditar(ctx, nil, usuario.Username, "registro_fallido", ip, userAgent, auditStatusConflict)
			return nil, err
		}
		s.enviarVerificacion(ctx, usuario, tokenPlano, token.ExpiraEn)
		observability.IncRegistro()
		return &RegistroPacienteResultado{Usuario: usuario, Paciente: paciente}, nil

	default:
		return nil, err
	}
}

// CompletarRegistroPaciente permite que una cuenta autenticada sin historia
// clínica (por ejemplo, creada con Clerk) complete sus datos demográficos.
// Si el documento ya existe sin cuenta, se crea una solicitud de vinculación.
func (s *CuentaService) CompletarRegistroPaciente(
	ctx context.Context,
	usuarioID int64,
	in CompletarRegistroPacienteInput,
	ip, userAgent string,
) (*RegistroPacienteResultado, error) {
	usuario, err := s.usuarios.ObtenerPorID(ctx, usuarioID)
	if err != nil {
		return nil, err
	}
	if !usuario.Activo {
		return nil, fmt.Errorf("%w: la cuenta está inactiva", entities.ErrForbidden)
	}
	if !esCuentaPaciente(usuario) {
		return nil, fmt.Errorf("%w: solo las cuentas de paciente pueden completar este registro", entities.ErrForbidden)
	}

	paciente, err := s.validarPacienteCompletar(in, usuario.Email)
	if err != nil {
		s.auditar(ctx, usuario, usuario.Username, "completar_registro_fallido", ip, userAgent, estadoAuditoriaErrorCuenta(err))
		return nil, err
	}

	auditoria := s.auditoriaCuenta(
		"cuentas", "completar_registro", "POST",
		"/api/v1/portal/cuenta/completar-registro", ip, userAgent, auditStatusCreated,
	)
	creado, solicitud, err := s.cuentas.CompletarRegistroPaciente(
		ctx, usuario, paciente, "Registro completado desde una cuenta de Clerk", auditoria,
	)
	if err != nil {
		s.auditar(ctx, usuario, usuario.Username, "completar_registro_fallido", ip, userAgent, estadoAuditoriaErrorCuenta(err))
		return nil, err
	}
	if solicitud != nil {
		auditoria.EstadoHTTP = auditStatusAccepted
		return &RegistroPacienteResultado{
			Usuario:              usuario,
			Solicitud:            solicitud,
			VinculacionPendiente: true,
		}, nil
	}
	return &RegistroPacienteResultado{Usuario: usuario, Paciente: creado}, nil
}

// ConvertirPacienteMedicoInput contiene los datos profesionales para convertir
// una cuenta de paciente en médico reutilizando su persona.
type ConvertirPacienteMedicoInput struct {
	EspecialidadID  int64
	NumeroLicencia  string
	TipoProfesional string
	DuracionCitaMin *int
	Bio             *string
	FotoURL         *string
	AceptaPortal    *bool
}

func (s *CuentaService) ConvertirPacienteEnMedico(
	ctx context.Context,
	usuarioID int64,
	in ConvertirPacienteMedicoInput,
	actorID int64,
	actorUsername, ip, userAgent string,
) (*entities.Medico, error) {
	if usuarioID <= 0 {
		return nil, entities.NewValidationError("usuario_id es obligatorio")
	}
	if s.especialidades == nil {
		return nil, fmt.Errorf("no está disponible el catálogo de especialidades")
	}
	especialidad, err := s.especialidades.ObtenerPorID(ctx, in.EspecialidadID)
	if err != nil {
		if errors.Is(err, entities.ErrNotFound) {
			return nil, entities.NewValidationError("la especialidad %d no existe", in.EspecialidadID)
		}
		return nil, err
	}
	paciente, err := s.pacientes.BuscarPorUsuarioID(ctx, usuarioID)
	if err != nil {
		if errors.Is(err, entities.ErrNotFound) {
			return nil, entities.NewValidationError(
				"el usuario no tiene una historia clínica de paciente activa")
		}
		return nil, err
	}

	medico := &entities.Medico{
		UsuarioID:       &usuarioID,
		Persona:         paciente.Persona,
		EspecialidadID:  in.EspecialidadID,
		Especialidad:    *especialidad,
		NumeroLicencia:  in.NumeroLicencia,
		TipoProfesional: in.TipoProfesional,
		DuracionCitaMin: 30,
		AceptaPortal:    true,
		Activo:          true,
	}
	if in.DuracionCitaMin != nil {
		medico.DuracionCitaMin = *in.DuracionCitaMin
	}
	if in.Bio != nil {
		medico.Bio = *in.Bio
	}
	if in.FotoURL != nil {
		medico.FotoURL = *in.FotoURL
	}
	if in.AceptaPortal != nil {
		medico.AceptaPortal = *in.AceptaPortal
	}
	normalizarMedico(medico)
	if err := validarMedico(*medico); err != nil {
		return nil, err
	}

	auditoria := s.auditoriaCuenta(
		"cuentas", "convertir_medico", "POST",
		"/api/v1/cuentas/convertir-paciente-medico", ip, userAgent, auditStatusOK,
	)
	auditoria.UsuarioID = &actorID
	auditoria.Username = usernameAuditoria(actorUsername)
	creado, err := s.cuentas.ConvertirPacienteEnMedico(
		ctx, usuarioID, medico, "medico", auditoria,
	)
	if err != nil {
		s.auditarActor(ctx, actorID, actorUsername, "convertir_medico_fallido",
			"/api/v1/cuentas/convertir-paciente-medico", ip, userAgent,
			estadoAuditoriaErrorCuenta(err))
		return nil, err
	}
	return creado, nil
}

// CompletarRegistroClerk crea la cuenta local solo después de que el usuario
// autenticado con Clerk completa sus datos clínicos. Si ya existe una cuenta
// local sin historia clínica, completa esa cuenta en lugar de duplicarla.
func (s *CuentaService) CompletarRegistroClerk(
	ctx context.Context,
	identidad *auth.IdentidadClerk,
	in CompletarRegistroPacienteInput,
	ip, userAgent string,
) (*RegistroPacienteResultado, error) {
	if identidad == nil || strings.TrimSpace(identidad.Subject) == "" {
		return nil, entities.NewValidationError("la identidad de Clerk es obligatoria")
	}
	email := strings.ToLower(strings.TrimSpace(identidad.Email))
	if email == "" {
		return nil, entities.NewValidationError("la cuenta de Clerk no tiene correo verificado")
	}

	usuario, err := s.usuarios.BuscarPorClerkID(ctx, identidad.Subject)
	if err != nil && !errors.Is(err, entities.ErrNotFound) {
		return nil, err
	}
	if errors.Is(err, entities.ErrNotFound) {
		usuario, err = s.usuarios.BuscarPorEmail(ctx, email)
		if err != nil && !errors.Is(err, entities.ErrNotFound) {
			return nil, err
		}
	}

	paciente, err := s.validarPacienteCompletar(in, email)
	if err != nil {
		s.auditar(ctx, usuario, email, "registro_clerk_fallido", ip, userAgent, estadoAuditoriaErrorCuenta(err))
		return nil, err
	}

	auditoria := s.auditoriaCuenta(
		"auth", "registro_clerk", "POST",
		"/api/v1/auth/clerk/registro", ip, userAgent, auditStatusCreated,
	)

	if usuario != nil {
		if !usuario.Activo {
			return nil, fmt.Errorf("%w: la cuenta está inactiva", entities.ErrForbidden)
		}
		if !esCuentaPaciente(usuario) {
			return nil, fmt.Errorf("%w: solo las cuentas de paciente pueden completar este registro", entities.ErrForbidden)
		}
		if _, err := s.pacientes.BuscarPorUsuarioID(ctx, usuario.ID); err == nil {
			return nil, fmt.Errorf("%w: la cuenta ya tiene una historia clínica vinculada", entities.ErrConflict)
		} else if !errors.Is(err, entities.ErrNotFound) {
			return nil, err
		}
		if usuario.ClerkUserID == "" {
			if err := s.usuarios.VincularClerkID(ctx, usuario.ID, identidad.Subject); err != nil {
				return nil, err
			}
		}
		if usuario.EmailVerificadoEn == nil {
			if err := s.usuarios.MarcarEmailVerificado(ctx, usuario.ID, time.Now()); err != nil {
				return nil, err
			}
		}

		creado, solicitud, err := s.cuentas.CompletarRegistroPaciente(
			ctx, usuario, paciente, "Registro completado con Clerk", auditoria,
		)
		if err != nil {
			s.auditar(ctx, usuario, usuario.Username, "registro_clerk_fallido", ip, userAgent, estadoAuditoriaErrorCuenta(err))
			return nil, err
		}
		if solicitud != nil {
			return &RegistroPacienteResultado{
				Usuario:              usuario,
				Solicitud:            solicitud,
				VinculacionPendiente: true,
			}, nil
		}
		return &RegistroPacienteResultado{Usuario: usuario, Paciente: creado}, nil
	}

	nombres := strings.TrimSpace(in.Nombres)
	apellidos := strings.TrimSpace(in.Apellidos)
	username := usernameDesdeEmail(email)
	for intento := 2; ; intento++ {
		_, err := s.usuarios.BuscarPorUsername(ctx, username)
		if errors.Is(err, entities.ErrNotFound) {
			break
		}
		if err != nil {
			return nil, err
		}
		username = fmt.Sprintf("%s-%d", usernameDesdeEmail(email), intento)
	}

	ahora := time.Now()
	usuario = &entities.Usuario{
		Username:            username,
		Email:               email,
		ClerkUserID:         identidad.Subject,
		Nombres:             nombres,
		Apellidos:           apellidos,
		Activo:              true,
		EmailVerificadoEn:   &ahora,
		TerminosAceptadosEn: &ahora,
		VersionTerminos:     s.cfg.TermsVersion,
		Roles:               []string{"paciente"},
	}
	creado, solicitud, err := s.cuentas.RegistrarCuentaClerk(
		ctx, usuario, paciente, "Registro creado con Clerk", auditoria,
	)
	if err != nil {
		s.auditar(ctx, nil, email, "registro_clerk_fallido", ip, userAgent, estadoAuditoriaErrorCuenta(err))
		return nil, err
	}
	if solicitud != nil {
		return &RegistroPacienteResultado{
			Usuario:              usuario,
			Solicitud:            solicitud,
			VinculacionPendiente: true,
		}, nil
	}
	return &RegistroPacienteResultado{Usuario: usuario, Paciente: creado}, nil
}

func (s *CuentaService) validarPacienteCompletar(
	in CompletarRegistroPacienteInput,
	email string,
) (*entities.Paciente, error) {
	nombres := strings.TrimSpace(in.Nombres)
	apellidos := strings.TrimSpace(in.Apellidos)
	if nombres == "" || apellidos == "" {
		return nil, entities.NewValidationError("nombres y apellidos son obligatorios")
	}
	if len(nombres) > 120 || len(apellidos) > 120 {
		return nil, entities.NewValidationError("nombres y apellidos no pueden superar los 120 caracteres")
	}

	paciente := &entities.Paciente{
		Persona: entities.Persona{
			TipoDocumento:   in.TipoDocumento,
			NumeroDocumento: in.NumeroDocumento,
			Nombres:         nombres,
			Apellidos:       apellidos,
			FechaNacimiento: in.FechaNacimiento,
			Sexo:            in.Sexo,
			Telefono:        in.Telefono,
			Email:           email,
			Direccion:       in.Direccion,
		},
		Activo: true,
	}
	normalizarPaciente(paciente)
	if err := validarPaciente(*paciente); err != nil {
		return nil, err
	}
	edad := edadEnAnios(paciente.Persona.FechaNacimiento, time.Now())
	if edad > 120 {
		return nil, entities.NewValidationError("la fecha de nacimiento no es válida")
	}
	if s.cfg.MinEdadCuenta > 0 && edad < s.cfg.MinEdadCuenta {
		return nil, entities.NewValidationError(
			"debes tener al menos %d años para crear una cuenta; los pacientes menores requieren un tutor o representante",
			s.cfg.MinEdadCuenta)
	}
	return paciente, nil
}

// VerificarEmail consume un token de verificación de un solo uso.
func (s *CuentaService) VerificarEmail(
	ctx context.Context,
	token, ip, userAgent string,
) (*entities.Usuario, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, entities.NewValidationError("token es obligatorio")
	}
	auditoria := s.auditoriaCuenta("auth", "verificacion_email", "POST", "/api/v1/auth/verify-email", ip, userAgent, auditStatusOK)
	usuario, err := s.cuentas.VerificarEmail(ctx, hashTokenCuenta(token), time.Now(), auditoria)
	if err != nil {
		s.auditar(ctx, nil, "", "verificacion_email_fallida", ip, userAgent, estadoAuditoriaErrorCuenta(err))
		return nil, err
	}
	observability.IncVerificacion()
	return usuario, nil
}

// ReenviarVerificacion responde siempre de forma genérica para no filtrar si
// una dirección está registrada. Solo envía correo a cuentas activas y no
// verificadas.
func (s *CuentaService) ReenviarVerificacion(
	ctx context.Context,
	email, ip, userAgent string,
) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if err := validarEmail(email); err != nil {
		return err
	}

	usuario, err := s.usuarios.BuscarPorEmail(ctx, email)
	if errors.Is(err, entities.ErrNotFound) {
		s.auditar(ctx, nil, "", "reenvio_verificacion_sin_cuenta", ip, userAgent, auditStatusAccepted)
		return nil
	}
	if err != nil {
		return err
	}
	if !usuario.Activo || usuario.EmailVerificado() {
		s.auditar(ctx, usuario, usuario.Username, "reenvio_verificacion_omitido", ip, userAgent, auditStatusAccepted)
		return nil
	}

	token, tokenPlano, err := s.nuevoToken(usuario.ID, entities.TokenVerificacionEmail, s.cfg.VerificationTTL)
	if err != nil {
		return err
	}
	if err := s.cuentas.ReemplazarToken(ctx, token); err != nil {
		return err
	}
	s.enviarVerificacion(ctx, usuario, tokenPlano, token.ExpiraEn)
	s.auditar(ctx, usuario, usuario.Username, "reenvio_verificacion", ip, userAgent, auditStatusAccepted)
	return nil
}

// SolicitarResetPassword responde de forma genérica y solo envía un enlace si
// la cuenta existe y está activa.
func (s *CuentaService) SolicitarResetPassword(
	ctx context.Context,
	email, ip, userAgent string,
) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if err := validarEmail(email); err != nil {
		return err
	}

	usuario, err := s.usuarios.BuscarPorEmail(ctx, email)
	if errors.Is(err, entities.ErrNotFound) {
		s.auditar(ctx, nil, "", "solicitud_reset_sin_cuenta", ip, userAgent, auditStatusAccepted)
		return nil
	}
	if err != nil {
		return err
	}
	if !usuario.Activo {
		s.auditar(ctx, usuario, usuario.Username, "solicitud_reset_cuenta_inactiva", ip, userAgent, auditStatusAccepted)
		return nil
	}

	token, tokenPlano, err := s.nuevoToken(usuario.ID, entities.TokenResetPassword, s.cfg.ResetTTL)
	if err != nil {
		return err
	}
	if err := s.cuentas.ReemplazarToken(ctx, token); err != nil {
		return err
	}
	if s.email != nil {
		enlace := s.enlaceCuenta("/restablecer-password", tokenPlano)
		if err := s.email.EnviarResetPassword(ctx, usuario.Email, usuario.Nombres, enlace, token.ExpiraEn); err != nil {
			s.logger.Error("no se pudo enviar el correo de recuperación",
				"usuario_id", usuario.ID, "error", err)
		}
	}
	s.auditar(ctx, usuario, usuario.Username, "solicitud_reset_password", ip, userAgent, auditStatusAccepted)
	return nil
}

// RestablecerPassword valida la nueva clave y consume el token de recuperación.
func (s *CuentaService) RestablecerPassword(
	ctx context.Context,
	token, nuevaPassword, ip, userAgent string,
) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return entities.NewValidationError("token es obligatorio")
	}
	if err := validarPassword(nuevaPassword); err != nil {
		return err
	}
	hash, err := hashearPassword(nuevaPassword, s.cfg.BcryptCost)
	if err != nil {
		return err
	}
	auditoria := s.auditoriaCuenta("auth", "reset_password", "POST", "/api/v1/auth/reset-password", ip, userAgent, auditStatusNoContent)
	if _, err := s.cuentas.RestablecerPassword(ctx, hashTokenCuenta(token), hash, time.Now(), auditoria); err != nil {
		s.auditar(ctx, nil, "", "reset_password_fallido", ip, userAgent, estadoAuditoriaErrorCuenta(err))
		return err
	}
	return nil
}

// estadoAuditoriaErrorCuenta clasifica el fallo para la bitácora sin exponer
// detalles internos al cliente.
func estadoAuditoriaErrorCuenta(err error) int {
	if errors.Is(err, entities.ErrValidation) {
		return auditStatusBadRequest
	}
	return auditStatusInternal
}

// ListarSolicitudesVinculacion consulta las solicitudes para recepción.
func (s *CuentaService) ListarSolicitudesVinculacion(
	ctx context.Context,
	filtro entities.SolicitudVinculacionFiltro,
) ([]entities.SolicitudVinculacion, error) {
	filtro.ListOptions = filtro.ListOptions.Normalized()
	filtro.Search = strings.TrimSpace(filtro.Search)
	if filtro.Estado != "" && !filtro.Estado.Valido() {
		return nil, entities.NewValidationError("estado de solicitud no válido")
	}
	return s.cuentas.ListarSolicitudesVinculacion(ctx, filtro)
}

// AprobarSolicitudVinculacion vincula la cuenta pendiente con su historia.
func (s *CuentaService) AprobarSolicitudVinculacion(
	ctx context.Context,
	solicitudID, actorID int64,
	actorUsername, ip, userAgent string,
) (*entities.SolicitudVinculacion, error) {
	if solicitudID <= 0 {
		return nil, entities.NewValidationError("id de solicitud inválido")
	}
	solicitud, err := s.cuentas.AprobarSolicitudVinculacion(ctx, solicitudID, actorID, time.Now())
	if err != nil {
		return nil, err
	}
	s.auditarActor(ctx, actorID, actorUsername, "vinculacion_cuenta_aprobada",
		"/api/v1/cuentas/vinculaciones", ip, userAgent, auditStatusOK)
	return solicitud, nil
}

// RechazarSolicitudVinculacion deja constancia y desactiva la cuenta pendiente.
func (s *CuentaService) RechazarSolicitudVinculacion(
	ctx context.Context,
	solicitudID, actorID int64,
	actorUsername, motivo, ip, userAgent string,
) (*entities.SolicitudVinculacion, error) {
	if solicitudID <= 0 {
		return nil, entities.NewValidationError("id de solicitud inválido")
	}
	motivo = strings.TrimSpace(motivo)
	if motivo == "" {
		return nil, entities.NewValidationError("el motivo del rechazo es obligatorio")
	}
	if len(motivo) > 1000 {
		return nil, entities.NewValidationError("el motivo no puede superar los 1000 caracteres")
	}
	solicitud, err := s.cuentas.RechazarSolicitudVinculacion(
		ctx, solicitudID, actorID, motivo, time.Now(),
	)
	if err != nil {
		return nil, err
	}
	s.auditarActor(ctx, actorID, actorUsername, "vinculacion_cuenta_rechazada",
		"/api/v1/cuentas/vinculaciones", ip, userAgent, auditStatusOK)
	return solicitud, nil
}

// VincularPaciente permite una vinculación directa desde recepción.
func (s *CuentaService) VincularPaciente(
	ctx context.Context,
	pacienteID, usuarioID int64,
	solicitudID *int64,
	actorID int64,
	actorUsername, ip, userAgent string,
) error {
	if pacienteID <= 0 || usuarioID <= 0 {
		return entities.NewValidationError("paciente_id y usuario_id son obligatorios")
	}
	if solicitudID != nil && *solicitudID <= 0 {
		return entities.NewValidationError("solicitud_id no es válido")
	}
	if err := s.cuentas.VincularPaciente(
		ctx, pacienteID, usuarioID, solicitudID, actorID, time.Now(),
	); err != nil {
		return err
	}
	s.auditarActor(ctx, actorID, actorUsername, "vinculacion_paciente",
		"/api/v1/cuentas/vincular-paciente", ip, userAgent, auditStatusOK)
	return nil
}

// VincularMedico asocia una cuenta con un perfil médico existente.
func (s *CuentaService) VincularMedico(
	ctx context.Context,
	medicoID, usuarioID, actorID int64,
	actorUsername, ip, userAgent string,
) error {
	if medicoID <= 0 || usuarioID <= 0 {
		return entities.NewValidationError("medico_id y usuario_id son obligatorios")
	}
	if err := s.cuentas.VincularMedico(ctx, medicoID, usuarioID, time.Now()); err != nil {
		return err
	}
	s.auditarActor(ctx, actorID, actorUsername, "vinculacion_medico",
		"/api/v1/cuentas/vincular-medico", ip, userAgent, auditStatusOK)
	return nil
}

func (s *CuentaService) validarRegistro(
	in RegistroPacienteInput,
) (*entities.Usuario, *entities.Paciente, error) {
	username := strings.ToLower(strings.TrimSpace(in.Username))
	if !usernamePattern.MatchString(username) {
		return nil, nil, entities.NewValidationError(
			"username debe tener entre 3 y 50 caracteres y solo usar letras, números, punto, guion o guion bajo")
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if err := validarEmail(email); err != nil {
		return nil, nil, err
	}
	if len(email) > 160 {
		return nil, nil, entities.NewValidationError("email no puede superar los 160 caracteres")
	}
	nombres := strings.TrimSpace(in.Nombres)
	apellidos := strings.TrimSpace(in.Apellidos)
	if nombres == "" || apellidos == "" {
		return nil, nil, entities.NewValidationError("nombres y apellidos son obligatorios")
	}
	if len(nombres) > 120 || len(apellidos) > 120 {
		return nil, nil, entities.NewValidationError("nombres y apellidos no pueden superar los 120 caracteres")
	}
	if err := validarPassword(strings.TrimSpace(in.Password)); err != nil {
		return nil, nil, err
	}
	if !in.AceptaTerminos {
		return nil, nil, entities.NewValidationError("debes aceptar los términos y condiciones")
	}
	if strings.TrimSpace(in.VersionTerminos) != s.cfg.TermsVersion {
		return nil, nil, entities.NewValidationError(
			"la versión de términos aceptada no corresponde a la versión vigente")
	}

	paciente := &entities.Paciente{
		Persona: entities.Persona{
			TipoDocumento:   in.TipoDocumento,
			NumeroDocumento: in.NumeroDocumento,
			Nombres:         nombres,
			Apellidos:       apellidos,
			FechaNacimiento: in.FechaNacimiento,
			Sexo:            in.Sexo,
			Telefono:        in.Telefono,
			Email:           email,
			Direccion:       in.Direccion,
		},
		Activo: true,
	}
	normalizarPaciente(paciente)
	if err := validarPaciente(*paciente); err != nil {
		return nil, nil, err
	}
	edad := edadEnAnios(paciente.Persona.FechaNacimiento, time.Now())
	if edad > 120 {
		return nil, nil, entities.NewValidationError("la fecha de nacimiento no es válida")
	}
	if s.cfg.MinEdadCuenta > 0 && edad < s.cfg.MinEdadCuenta {
		return nil, nil, entities.NewValidationError(
			"debes tener al menos %d años para crear una cuenta; los pacientes menores requieren un tutor o representante",
			s.cfg.MinEdadCuenta)
	}

	ahora := time.Now()
	usuario := &entities.Usuario{
		Username:            username,
		Email:               email,
		Nombres:             nombres,
		Apellidos:           apellidos,
		Activo:              true,
		TerminosAceptadosEn: &ahora,
		VersionTerminos:     s.cfg.TermsVersion,
		Roles:               []string{"paciente"},
	}
	return usuario, paciente, nil
}

func (s *CuentaService) nuevoToken(
	usuarioID int64,
	tipo entities.TipoTokenCuenta,
	ttl time.Duration,
) (*entities.TokenCuenta, string, error) {
	plano, hash, err := generarTokenCuenta()
	if err != nil {
		return nil, "", err
	}
	return &entities.TokenCuenta{
		UsuarioID: usuarioID,
		Tipo:      tipo,
		TokenHash: hash,
		ExpiraEn:  time.Now().Add(ttl),
	}, plano, nil
}

func (s *CuentaService) enviarVerificacion(
	ctx context.Context,
	usuario *entities.Usuario,
	token string,
	expira time.Time,
) {
	if s.email == nil {
		return
	}
	enlace := s.enlaceCuenta("/verificar-email", token)
	if err := s.email.EnviarVerificacion(ctx, usuario.Email, usuario.Nombres, enlace, expira); err != nil {
		s.logger.Error("no se pudo enviar el correo de verificación",
			"usuario_id", usuario.ID, "error", err)
	}
}

func (s *CuentaService) enlaceCuenta(ruta, token string) string {
	base := strings.TrimRight(strings.TrimSpace(s.cfg.LinkBaseURL), "/")
	return base + ruta + "?token=" + url.QueryEscape(token)
}

func (s *CuentaService) auditoriaCuenta(
	recurso, accion, metodo, ruta, ip, userAgent string,
	estado int,
) *entities.Auditoria {
	return &entities.Auditoria{
		Accion:     accion,
		Recurso:    recurso,
		Metodo:     metodo,
		Ruta:       ruta,
		EstadoHTTP: estado,
		IP:         ip,
		UserAgent:  userAgent,
	}
}

func (s *CuentaService) auditar(
	ctx context.Context,
	usuario *entities.Usuario,
	username, accion, ip, userAgent string,
	estado int,
) {
	if s.auditoria == nil {
		return
	}
	var usuarioID *int64
	if usuario != nil {
		id := usuario.ID
		usuarioID = &id
	}
	entrada := s.auditoriaCuenta("auth", accion, "POST", rutaAuditoriaCuenta(accion), ip, userAgent, estado)
	entrada.UsuarioID = usuarioID
	entrada.Username = usernameAuditoria(username)
	entrada.TraceID = observability.TraceID(ctx)
	auditCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if err := s.auditoria.Registrar(auditCtx, entrada); err != nil {
		s.logger.Error("no se pudo auditar la cuenta", "accion", accion, "error", err)
	}
}

func (s *CuentaService) auditarActor(
	ctx context.Context,
	actorID int64,
	actorUsername, accion, ruta, ip, userAgent string,
	estado int,
) {
	if s.auditoria == nil {
		return
	}
	var usuarioID *int64
	if actorID > 0 {
		id := actorID
		usuarioID = &id
	}
	entrada := s.auditoriaCuenta("cuentas", accion, "POST", ruta, ip, userAgent, estado)
	entrada.UsuarioID = usuarioID
	entrada.Username = usernameAuditoria(actorUsername)
	entrada.TraceID = observability.TraceID(ctx)
	auditCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if err := s.auditoria.Registrar(auditCtx, entrada); err != nil {
		s.logger.Error("no se pudo auditar la vinculación", "accion", accion, "error", err)
	}
}

// usernameAuditoria limita el valor a la columna auditoria.username (50
// caracteres) y evita que un registro fallido con un username arbitrario
// haga fallar la propia auditoría.
func usernameAuditoria(username string) string {
	runas := []rune(strings.TrimSpace(username))
	if len(runas) > 50 {
		runas = runas[:50]
	}
	return string(runas)
}

func rutaAuditoriaCuenta(accion string) string {
	switch {
	case strings.HasPrefix(accion, "registro"):
		return "/api/v1/auth/register"
	case strings.HasPrefix(accion, "verificacion"):
		return "/api/v1/auth/verify-email"
	case strings.HasPrefix(accion, "reenvio"):
		return "/api/v1/auth/resend-verification"
	case strings.HasPrefix(accion, "solicitud_reset"):
		return "/api/v1/auth/forgot-password"
	case strings.HasPrefix(accion, "reset_password"):
		return "/api/v1/auth/reset-password"
	case strings.HasPrefix(accion, "completar_registro"):
		return "/api/v1/portal/cuenta/completar-registro"
	default:
		return "/api/v1/cuentas"
	}
}

func edadEnAnios(fecha, ahora time.Time) int {
	anios := ahora.Year() - fecha.Year()
	if ahora.Month() < fecha.Month() ||
		(ahora.Month() == fecha.Month() && ahora.Day() < fecha.Day()) {
		anios--
	}
	return anios
}

func generarTokenCuenta() (plano, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generar token de cuenta: %w", err)
	}
	plano = base64.RawURLEncoding.EncodeToString(buf)
	return plano, hashTokenCuenta(plano), nil
}

func hashTokenCuenta(token string) string {
	suma := sha256.Sum256([]byte(token))
	return hex.EncodeToString(suma[:])
}
