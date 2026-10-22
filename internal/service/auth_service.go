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
	JWTSecret        string
	JWTIssuer        string
	AccessTTL        time.Duration
	RefreshTTL       time.Duration
	BcryptCost       int
	MaxLoginAttempts int
	LockoutDuration  time.Duration
}

// Sesion es el par de tokens devuelto al iniciar sesión o refrescar.
type Sesion struct {
	AccessToken  string
	RefreshToken string
	ExpiraEn     int64
	Usuario      *entities.Usuario
}

// AuthService contiene la lógica de autenticación y sesiones.
type AuthService struct {
	usuarios    UsuarioStore
	refresh     RefreshTokenStore
	auditoria   AuditoriaStore
	tokens      *TokenManager
	bcryptCost  int
	accessTTL   time.Duration
	refreshTTL  time.Duration
	maxIntentos int
	bloqueo     time.Duration
}

// NewAuthService construye el servicio de autenticación.
func NewAuthService(
	cfg AuthConfig,
	usuarios UsuarioStore,
	refresh RefreshTokenStore,
	auditoria AuditoriaStore,
) *AuthService {
	return &AuthService{
		usuarios:    usuarios,
		refresh:     refresh,
		auditoria:   auditoria,
		tokens:      NewTokenManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.AccessTTL),
		bcryptCost:  cfg.BcryptCost,
		accessTTL:   cfg.AccessTTL,
		refreshTTL:  cfg.RefreshTTL,
		maxIntentos: cfg.MaxLoginAttempts,
		bloqueo:     cfg.LockoutDuration,
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
		intentos := usuario.IntentosFallidos
		if usuario.BloqueadoHasta != nil && usuario.BloqueadoHasta.Before(ahora) {
			intentos = 0
		}
		intentos++

		var bloqueadoHasta *time.Time
		if intentos >= s.maxIntentos {
			t := ahora.Add(s.bloqueo)
			bloqueadoHasta = &t
		}
		if err := s.usuarios.RegistrarIntentoFallido(ctx, usuario.ID, intentos, bloqueadoHasta); err != nil {
			return nil, err
		}
		s.auditar(ctx, usuario, usuario.Username, "login_fallido", ip, userAgent, auditStatusUnauthorized)

		if bloqueadoHasta != nil {
			return nil, fmt.Errorf("%w: demasiados intentos fallidos; intenta de nuevo en %d minuto(s)",
				entities.ErrLocked, int(math.Ceil(s.bloqueo.Minutes())))
		}
		return nil, credencialesInvalidas()
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
		// Reutilización de un token ya rotado: posible robo. Se revocan todas las sesiones.
		_ = s.refresh.RevocarTodosPorUsuario(ctx, rt.UsuarioID)
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

	if err := s.refresh.Revocar(ctx, rt.ID); err != nil {
		return nil, err
	}
	sesion, err := s.crearSesion(ctx, usuario, ip, userAgent)
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
		UsuarioID: usuario.ID,
		TokenHash: refreshHash,
		ExpiraEn:  time.Now().Add(s.refreshTTL),
		IP:        ip,
		UserAgent: userAgent,
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

func (s *AuthService) auditar(ctx context.Context, usuario *entities.Usuario, username, accion, ip, userAgent string, estado int) {
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
	}
	_ = s.auditoria.Registrar(ctx, entrada)
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
