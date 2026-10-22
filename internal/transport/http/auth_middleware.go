package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"medilist/internal/service"
	"medilist/internal/service/entities"
)

type claimsContextKey struct{}

var claimsKey claimsContextKey

// AuthMiddleware valida el access token y los permisos requeridos.
type AuthMiddleware struct {
	auth      *service.AuthService
	auditoria service.AuditoriaStore
	logger    *slog.Logger
}

// NewAuthMiddleware construye el middleware de autenticación.
func NewAuthMiddleware(auth *service.AuthService, auditoria service.AuditoriaStore, logger *slog.Logger) *AuthMiddleware {
	return &AuthMiddleware{auth: auth, auditoria: auditoria, logger: logger}
}

// Require exige un token válido y, si permission no está vacío, ese permiso.
func (m *AuthMiddleware) Require(permission string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := strings.TrimSpace(r.Header.Get("Authorization"))
		partes := strings.SplitN(header, " ", 2)
		if len(partes) != 2 || !strings.EqualFold(partes[0], "Bearer") || strings.TrimSpace(partes[1]) == "" {
			writeUnauthorized(w)
			return
		}

		claims, err := m.auth.ValidarToken(strings.TrimSpace(partes[1]))
		if err != nil {
			writeUnauthorized(w)
			return
		}

		if permission != "" && !tienePermiso(claims, permission) {
			m.registrarDenegacion(r, claims, permission)
			writeError(w, http.StatusForbidden, "forbidden", "no tienes permiso para realizar esta acción")
			return
		}

		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="medilist"`)
	writeError(w, http.StatusUnauthorized, "unauthorized", "token de acceso ausente, inválido o expirado")
}

func claimsFromContext(ctx context.Context) *service.Claims {
	claims, _ := ctx.Value(claimsKey).(*service.Claims)
	return claims
}

func tienePermiso(claims *service.Claims, permiso string) bool {
	for _, rol := range claims.Roles {
		if rol == "admin" {
			return true
		}
	}
	for _, p := range claims.Permisos {
		if p == permiso {
			return true
		}
	}
	return false
}

func (m *AuthMiddleware) registrarDenegacion(r *http.Request, claims *service.Claims, permiso string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	entrada := &entities.Auditoria{
		UsuarioID:  &claims.UsuarioID,
		Username:   claims.Username,
		Accion:     "acceso_denegado",
		Recurso:    permiso,
		Metodo:     r.Method,
		Ruta:       r.URL.Path,
		EstadoHTTP: http.StatusForbidden,
		IP:         clientIP(r),
		UserAgent:  r.UserAgent(),
	}
	if err := m.auditoria.Registrar(ctx, entrada); err != nil {
		m.logger.Error("no se pudo auditar el acceso denegado", "error", err)
	}
}
