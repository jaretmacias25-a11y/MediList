package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"medilist/internal/auth"
	"medilist/internal/observability"
	"medilist/internal/service/entities"
)

const (
	auditStatusOK           = 200
	auditStatusNoContent    = 204
	auditStatusUnauthorized = 401
	auditStatusLocked       = 423
)

// AuthConfig agrupa los parámetros de seguridad de autenticación.
type AuthConfig struct {
	JWTSecret           string
	JWTIssuer           string
	AccessTTL           time.Duration
	RefreshTTL          time.Duration
	BcryptCost          int
	MaxLoginAttempts    int
	LockoutDuration     time.Duration
	TermsVersion        string
	BootstrapAdminEmail string
}

// ClerkVerifier valida tokens de sesión externos (Clerk).
type ClerkVerifier interface {
	Verificar(ctx context.Context, token string) (*auth.IdentidadClerk, error)
}

// Sesion es el par de tokens devuelto al iniciar sesión o refrescar.
type Sesion struct {
	AccessToken  string
	RefreshToken string
	ExpiraEn     int64
	Usuario      *entities.Usuario
}

// RegistroClerkPendiente contiene los datos que la SPA puede mostrar cuando
// Clerk autenticó al usuario pero todavía no existe una cuenta local.
type RegistroClerkPendiente struct {
	Email     string
	Nombres   string
	Apellidos string
}

// ResultadoLoginClerk indica si se emitió una sesión o si el usuario debe
// completar sus datos antes de crear la cuenta local.
type ResultadoLoginClerk struct {
	Sesion    *Sesion
	Pendiente *RegistroClerkPendiente
}

// AuthService contiene la lógica de autenticación y sesiones.
type AuthService struct {
	usuarios            UsuarioStore
	refresh             RefreshTokenStore
	auditoria           AuditoriaStore
	tokens              *TokenManager
	clerk               ClerkVerifier
	termsVersion        string
	bootstrapAdminEmail string
	bcryptCost          int
	accessTTL           time.Duration
	refreshTTL          time.Duration
	maxIntentos         int
	bloqueo             time.Duration
}

// NewAuthService construye el servicio de autenticación. El verificador de
// Clerk es opcional: sin él, el inicio con Clerk devuelve 403.
func NewAuthService(
	cfg AuthConfig,
	usuarios UsuarioStore,
	refresh RefreshTokenStore,
	auditoria AuditoriaStore,
	clerk ...ClerkVerifier,
) *AuthService {
	termsVersion := strings.TrimSpace(cfg.TermsVersion)
	if termsVersion == "" {
		termsVersion = "2026-10"
	}
	var clerkVerifier ClerkVerifier
	if len(clerk) > 0 {
		clerkVerifier = clerk[0]
	}
	return &AuthService{
		usuarios:            usuarios,
		refresh:             refresh,
		auditoria:           auditoria,
		tokens:              NewTokenManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.AccessTTL),
		clerk:               clerkVerifier,
		termsVersion:        termsVersion,
		bootstrapAdminEmail: strings.ToLower(strings.TrimSpace(cfg.BootstrapAdminEmail)),
		bcryptCost:          cfg.BcryptCost,
		accessTTL:           cfg.AccessTTL,
		refreshTTL:          cfg.RefreshTTL,
		maxIntentos:         cfg.MaxLoginAttempts,
		bloqueo:             cfg.LockoutDuration,
	}
}

func credencialesInvalidas() error {
	return fmt.Errorf("%w: usuario o contraseña incorrectos", entities.ErrUnauthorized)
}

// Login valida credenciales, aplica bloqueo por intentos fallidos y emite tokens.
func (s *AuthService) Login(ctx context.Context, username, password, ip, userAgent string) (*Sesion, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" || password == "" {
		return nil, credencialesInvalidas()
	}

	usuario, err := s.usuarios.BuscarPorUsername(ctx, username)
	if errors.Is(err, entities.ErrNotFound) {
		consumirTiempoPassword(password)
		s.auditar(ctx, nil, username, "login_fallido", ip, userAgent, auditStatusUnauthorized)
		return nil, credencialesInvalidas()
	}
	if err != nil {
		return nil, err
	}

	ahora := time.Now()
	if usuario.BloqueadoHasta != nil && usuario.BloqueadoHasta.After(ahora) {
		minutos := int(math.Ceil(time.Until(*usuario.BloqueadoHasta).Minutes()))
		if minutos < 1 {
			minutos = 1
		}
		s.auditar(ctx, usuario, usuario.Username, "login_bloqueado", ip, userAgent, auditStatusLocked)
		return nil, fmt.Errorf("%w: intenta de nuevo en %d minuto(s)", entities.ErrLocked, minutos)
	}

	if !usuario.Activo {
		consumirTiempoPassword(password)
		s.auditar(ctx, usuario, usuario.Username, "login_fallido", ip, userAgent, auditStatusUnauthorized)
		return nil, credencialesInvalidas()
	}

	if !verificarPassword(usuario.PasswordHash, password) {
		// El incremento se hace de forma atómica en la base de datos. Si se
		// hiciera con el valor leído antes del bcrypt, dos intentos
		// simultáneos podrían sobrescribirse y retrasar el bloqueo.
		_, bloqueadoHasta, err := s.usuarios.RegistrarIntentoFallido(
			ctx, usuario.ID, s.maxIntentos, s.bloqueo,
		)
		if err != nil {
			return nil, err
		}
		s.auditar(ctx, usuario, usuario.Username, "login_fallido", ip, userAgent, auditStatusUnauthorized)

		if bloqueadoHasta != nil {
			minutos := int(math.Ceil(time.Until(*bloqueadoHasta).Minutes()))
			if minutos < 1 {
				minutos = 1
			}
			return nil, fmt.Errorf("%w: demasiados intentos fallidos; intenta de nuevo en %d minuto(s)",
				entities.ErrLocked, minutos)
		}
		return nil, credencialesInvalidas()
	}

	// Solo las cuentas exclusivamente de paciente exigen verificación. Las
	// cuentas internas creadas antes de la Fase 2 quedan exentas.
	if usuario.RequiereVerificacionEmail() {
		s.auditar(ctx, usuario, usuario.Username, "login_email_no_verificado",
			ip, userAgent, auditStatusForbidden)
		return nil, fmt.Errorf("%w: verifica tu correo antes de iniciar sesión", entities.ErrEmailNoVerificado)
	}

	if err := s.usuarios.RegistrarLoginExitoso(ctx, usuario.ID); err != nil {
		return nil, err
	}

	sesion, err := s.crearSesion(ctx, usuario, ip, userAgent)
	if err != nil {
		return nil, err
	}
	s.auditar(ctx, usuario, usuario.Username, "login", ip, userAgent, auditStatusOK)
	return sesion, nil
}

// VerificarClerk valida un token de Clerk y devuelve la identidad externa.
func (s *AuthService) VerificarClerk(ctx context.Context, token string) (*auth.IdentidadClerk, error) {
	if s.clerk == nil {
		return nil, fmt.Errorf("%w: el inicio con Clerk no está configurado", entities.ErrForbidden)
	}
	identidad, err := s.clerk.Verificar(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("%w: token de Clerk inválido", entities.ErrUnauthorized)
	}
	return identidad, nil
}

// LoginClerk valida un token de Clerk. Si la cuenta local ya existe, emite una
// sesión; si no existe, devuelve los datos para que la SPA abra el formulario
// de registro. La cuenta local se crea únicamente al completar ese formulario.
func (s *AuthService) LoginClerk(ctx context.Context, token, ip, userAgent string) (*ResultadoLoginClerk, error) {
	identidad, err := s.VerificarClerk(ctx, token)
	if err != nil {
		if errors.Is(err, entities.ErrUnauthorized) {
			s.auditar(ctx, nil, "", "login_clerk_fallido", ip, userAgent, auditStatusUnauthorized)
		}
		return nil, err
	}

	usuario, err := s.usuarios.BuscarPorClerkID(ctx, identidad.Subject)
	if err != nil && !errors.Is(err, entities.ErrNotFound) {
		return nil, err
	}
	if errors.Is(err, entities.ErrNotFound) && identidad.Email != "" {
		usuario, err = s.usuarios.BuscarPorEmail(ctx, identidad.Email)
		if err != nil && !errors.Is(err, entities.ErrNotFound) {
			return nil, err
		}
	}

	if errors.Is(err, entities.ErrNotFound) || usuario == nil {
		return &ResultadoLoginClerk{
			Pendiente: &RegistroClerkPendiente{
				Email:     strings.ToLower(strings.TrimSpace(identidad.Email)),
				Nombres:   identidad.Nombres,
				Apellidos: identidad.Apellidos,
			},
		}, nil
	}

	// Vincula la identidad de Clerk y marca el correo como verificado.
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
	usuario, err = s.usuarios.ObtenerPorID(ctx, usuario.ID)
	if err != nil {
		return nil, err
	}

	if s.bootstrapAdminEmail != "" && strings.EqualFold(usuario.Email, s.bootstrapAdminEmail) && !tieneRol(usuario.Roles, "admin") {
		if err := s.usuarios.AsegurarRol(ctx, usuario.ID, "admin"); err != nil {
			return nil, err
		}
		usuario, err = s.usuarios.ObtenerPorID(ctx, usuario.ID)
		if err != nil {
			return nil, err
		}
	}

	if !usuario.Activo {
		return nil, fmt.Errorf("%w: la cuenta está inactiva", entities.ErrForbidden)
	}
	if usuario.BloqueadoHasta != nil && usuario.BloqueadoHasta.After(time.Now()) {
		return nil, fmt.Errorf("%w: cuenta bloqueada temporalmente", entities.ErrLocked)
	}

	sesion, err := s.crearSesion(ctx, usuario, ip, userAgent)
	if err != nil {
		return nil, err
	}
	s.auditar(ctx, usuario, usuario.Username, "login_clerk", ip, userAgent, auditStatusOK)
	return &ResultadoLoginClerk{Sesion: sesion}, nil
}

func usernameDesdeEmail(email string) string {
	local := strings.ToLower(strings.TrimSpace(strings.Split(email, "@")[0]))
	var limpio strings.Builder
	for _, r := range local {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			limpio.WriteRune(r)
		}
	}
	base := strings.Trim(limpio.String(), "._-")
	if len(base) < 3 {
		base = "paciente"
	}
	if len(base) > 40 {
		base = strings.TrimRight(base[:40], "._-")
	}
	return base
}

func tieneRol(roles []string, codigo string) bool {
	for _, rol := range roles {
		if rol == codigo {
			return true
		}
	}
	return false
}

func esCuentaPaciente(usuario *entities.Usuario) bool {
	if usuario == nil || len(usuario.Roles) == 0 {
		return false
	}
	for _, rol := range usuario.Roles {
		if rol != "paciente" {
			return false
		}
	}
	return true
}

func (s *AuthService) crearUsuarioClerk(ctx context.Context, identidad *auth.IdentidadClerk) (*entities.Usuario, error) {
	nombres := identidad.Nombres
	if nombres == "" {
		nombres = "Paciente"
	}
	apellidos := identidad.Apellidos
	if apellidos == "" {
		apellidos = "Clerk"
	}
	email := strings.ToLower(strings.TrimSpace(identidad.Email))
	if email == "" {
		return nil, fmt.Errorf("%w: la cuenta de Clerk no tiene correo verificado", entities.ErrForbidden)
	}

	base := usernameDesdeEmail(email)
	username := base
	for intento := 2; ; intento++ {
		_, err := s.usuarios.BuscarPorUsername(ctx, username)
		if errors.Is(err, entities.ErrNotFound) {
			break
		}
		if err != nil {
			return nil, err
		}
		username = fmt.Sprintf("%s-%d", base, intento)
	}

	ahora := time.Now()
	usuario := &entities.Usuario{
		Username:            username,
		Email:               email,
		PasswordHash:        "",
		ClerkUserID:         identidad.Subject,
		Nombres:             nombres,
		Apellidos:           apellidos,
		Activo:              true,
		EmailVerificadoEn:   &ahora,
		TerminosAceptadosEn: &ahora,
		VersionTerminos:     s.termsVersion,
		Roles:               []string{"paciente"},
	}
	if err := s.usuarios.Crear(ctx, usuario); err != nil {
		return nil, err
	}
	return s.usuarios.ObtenerPorID(ctx, usuario.ID)
}

// Refresh rota el refresh token y emite un nuevo par de tokens.
func (s *AuthService) Refresh(ctx context.Context, refreshToken, ip, userAgent string) (*Sesion, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, fmt.Errorf("%w: refresh token requerido", entities.ErrUnauthorized)
	}

	rt, err := s.refresh.BuscarPorHash(ctx, hashRefreshToken(refreshToken))
	if errors.Is(err, entities.ErrNotFound) {
		return nil, fmt.Errorf("%w: refresh token inválido", entities.ErrUnauthorized)
	}
	if err != nil {
		return nil, err
	}

	if rt.RevocadoEn != nil {
		// Reutilización de un token ya rotado: posible robo. Se revocan los
		// refresh tokens y también los access tokens al incrementar la versión.
		_ = s.usuarios.RevocarSesiones(ctx, rt.UsuarioID)
		s.auditar(ctx, nil, "", "refresh_reutilizado", ip, userAgent, auditStatusUnauthorized)
		return nil, fmt.Errorf("%w: refresh token inválido", entities.ErrUnauthorized)
	}
	if !rt.ExpiraEn.After(time.Now()) {
		_ = s.refresh.Revocar(ctx, rt.ID)
		return nil, fmt.Errorf("%w: refresh token expirado", entities.ErrUnauthorized)
	}

	usuario, err := s.usuarios.ObtenerPorID(ctx, rt.UsuarioID)
	if err != nil {
		if errors.Is(err, entities.ErrNotFound) {
			return nil, fmt.Errorf("%w: usuario no válido", entities.ErrUnauthorized)
		}
		return nil, err
	}
	if !usuario.Activo {
		_ = s.refresh.Revocar(ctx, rt.ID)
		return nil, fmt.Errorf("%w: usuario inactivo", entities.ErrUnauthorized)
	}
	if usuario.RequiereVerificacionEmail() {
		_ = s.refresh.Revocar(ctx, rt.ID)
		s.auditar(ctx, usuario, usuario.Username, "refresh_email_no_verificado",
			ip, userAgent, auditStatusForbidden)
		return nil, fmt.Errorf("%w: verifica tu correo antes de continuar", entities.ErrEmailNoVerificado)
	}

	sesion, err := s.rotarSesion(ctx, usuario, rt, ip, userAgent)
	if err != nil {
		return nil, err
	}
	s.auditar(ctx, usuario, usuario.Username, "refresh", ip, userAgent, auditStatusOK)
	return sesion, nil
}

// Logout revoca el refresh token si existe. Es idempotente.
func (s *AuthService) Logout(ctx context.Context, refreshToken, ip, userAgent string) error {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil
	}
	rt, err := s.refresh.BuscarPorHash(ctx, hashRefreshToken(refreshToken))
	if errors.Is(err, entities.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if rt.RevocadoEn == nil {
		if err := s.refresh.Revocar(ctx, rt.ID); err != nil {
			return err
		}
	}
	s.auditar(ctx, nil, "", "logout", ip, userAgent, auditStatusNoContent)
	return nil
}

// Me devuelve el usuario autenticado con sus roles y permisos actuales.
func (s *AuthService) Me(ctx context.Context, userID int64) (*entities.Usuario, error) {
	usuario, err := s.usuarios.ObtenerPorID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !usuario.Activo {
		return nil, fmt.Errorf("%w: usuario inactivo", entities.ErrUnauthorized)
	}
	return usuario, nil
}

// ValidarToken valida firma y expiración, pero no comprueba revocación.
// Para autenticar solicitudes usa ValidarSesion.
func (s *AuthService) ValidarToken(token string) (*Claims, error) {
	return s.tokens.Validar(token)
}

// ValidarSesion valida el token y comprueba contra la base de datos que el
// usuario siga activo y que su versión de sesión no haya sido revocada.
func (s *AuthService) ValidarSesion(ctx context.Context, token string) (*Claims, error) {
	claims, err := s.tokens.Validar(token)
	if err != nil {
		return nil, err
	}
	valida, err := s.usuarios.SesionValida(ctx, claims.UsuarioID, claims.TokenVersion)
	if err != nil {
		return nil, err
	}
	if !valida {
		return nil, fmt.Errorf("%w: sesión revocada", entities.ErrUnauthorized)
	}
	return claims, nil
}

// CambiarPassword permite que un usuario autenticado cambie su contraseña.
// Verifica la contraseña actual y revoca todas las sesiones de refresco.
func (s *AuthService) CambiarPassword(ctx context.Context, userID int64, actual, nueva, ip, userAgent string) error {
	usuario, err := s.usuarios.ObtenerPorID(ctx, userID)
	if err != nil {
		return err
	}
	if !usuario.Activo {
		return fmt.Errorf("%w: usuario inactivo", entities.ErrUnauthorized)
	}
	if !verificarPassword(usuario.PasswordHash, actual) {
		s.auditar(ctx, usuario, usuario.Username, "cambio_password_fallido", ip, userAgent, auditStatusUnauthorized)
		return fmt.Errorf("%w: la contraseña actual no es correcta", entities.ErrUnauthorized)
	}
	if err := validarPassword(nueva); err != nil {
		return err
	}
	if actual == nueva {
		return entities.NewValidationError("la nueva contraseña debe ser diferente a la actual")
	}
	hash, err := hashearPassword(nueva, s.bcryptCost)
	if err != nil {
		return err
	}
	u := &entities.Usuario{
		ID:        usuario.ID,
		Username:  usuario.Username,
		Email:     usuario.Email,
		Nombres:   usuario.Nombres,
		Apellidos: usuario.Apellidos,
		Activo:    usuario.Activo,
		Roles:     usuario.Roles,
		Version:   usuario.Version,
	}
	if err := s.usuarios.Actualizar(ctx, u, hash, true); err != nil {
		return err
	}
	s.auditar(ctx, usuario, usuario.Username, "cambio_password", ip, userAgent, auditStatusNoContent)
	return nil
}

func (s *AuthService) crearSesion(ctx context.Context, usuario *entities.Usuario, ip, userAgent string) (*Sesion, error) {
	accessToken, _, err := s.tokens.Generar(usuario)
	if err != nil {
		return nil, err
	}
	refreshPlano, refreshHash, err := generarRefreshToken()
	if err != nil {
		return nil, err
	}
	rt := &entities.RefreshToken{
		UsuarioID:    usuario.ID,
		TokenHash:    refreshHash,
		TokenVersion: usuario.TokenVersion,
		ExpiraEn:     time.Now().Add(s.refreshTTL),
		IP:           ip,
		UserAgent:    userAgent,
	}
	if err := s.refresh.Crear(ctx, rt); err != nil {
		return nil, err
	}
	return &Sesion{
		AccessToken:  accessToken,
		RefreshToken: refreshPlano,
		ExpiraEn:     int64(s.accessTTL.Seconds()),
		Usuario:      usuario,
	}, nil
}

// rotarSesion consume el refresh token anterior y crea el siguiente dentro de
// una sola transacción. Si dos solicitudes usan el mismo token a la vez, solo
// una puede rotarlo; la otra se trata como reutilización y revoca la sesión.
func (s *AuthService) rotarSesion(
	ctx context.Context,
	usuario *entities.Usuario,
	anterior *entities.RefreshToken,
	ip, userAgent string,
) (*Sesion, error) {
	accessToken, _, err := s.tokens.Generar(usuario)
	if err != nil {
		return nil, err
	}
	refreshPlano, refreshHash, err := generarRefreshToken()
	if err != nil {
		return nil, err
	}
	nuevo := &entities.RefreshToken{
		UsuarioID:    usuario.ID,
		TokenHash:    refreshHash,
		TokenVersion: usuario.TokenVersion,
		ExpiraEn:     time.Now().Add(s.refreshTTL),
		IP:           ip,
		UserAgent:    userAgent,
	}
	rotado, err := s.refresh.Rotar(ctx, anterior.ID, nuevo)
	if err != nil {
		return nil, err
	}
	if !rotado {
		// El token ya fue consumido, expiró o fue revocado entre la lectura y
		// la rotación. Se asume reutilización y se cierran todas las sesiones,
		// incluidos los access tokens vigentes.
		_ = s.usuarios.RevocarSesiones(ctx, usuario.ID)
		s.auditar(ctx, nil, "", "refresh_reutilizado", ip, userAgent, auditStatusUnauthorized)
		return nil, fmt.Errorf("%w: refresh token inválido", entities.ErrUnauthorized)
	}
	return &Sesion{
		AccessToken:  accessToken,
		RefreshToken: refreshPlano,
		ExpiraEn:     int64(s.accessTTL.Seconds()),
		Usuario:      usuario,
	}, nil
}

func (s *AuthService) auditar(ctx context.Context, usuario *entities.Usuario, username, accion, ip, userAgent string, estado int) {
	if s.auditoria == nil {
		return
	}
	var usuarioID *int64
	if usuario != nil {
		id := usuario.ID
		usuarioID = &id
		username = usuario.Username
	}
	entrada := &entities.Auditoria{
		UsuarioID:  usuarioID,
		Username:   username,
		Accion:     accion,
		Recurso:    "auth",
		Metodo:     "POST",
		Ruta:       rutaAuditoriaAuth(accion),
		EstadoHTTP: estado,
		IP:         ip,
		UserAgent:  userAgent,
		TraceID:    observability.TraceID(ctx),
	}
	// La bitácora de seguridad no debe perderse si el cliente cancela.
	auditCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	_ = s.auditoria.Registrar(auditCtx, entrada)
}

func rutaAuditoriaAuth(accion string) string {
	switch {
	case strings.HasPrefix(accion, "login"):
		return "/api/v1/auth/login"
	case strings.HasPrefix(accion, "refresh"):
		return "/api/v1/auth/refresh"
	case strings.HasPrefix(accion, "logout"):
		return "/api/v1/auth/logout"
	case strings.HasPrefix(accion, "cambio_password"):
		return "/api/v1/auth/password"
	default:
		return "/api/v1/auth/" + accion
	}
}

func generarRefreshToken() (plano, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generar refresh token: %w", err)
	}
	plano = base64.RawURLEncoding.EncodeToString(buf)
	return plano, hashRefreshToken(plano), nil
}

func hashRefreshToken(token string) string {
	suma := sha256.Sum256([]byte(token))
	return hex.EncodeToString(suma[:])
}
