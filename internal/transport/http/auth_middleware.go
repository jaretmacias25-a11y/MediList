package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"medilist/internal/observability"
	"medilist/internal/service"
	"medilist/internal/service/entities"
)

type claimsContextKey struct{}
type actorContextKey struct{}

var (
	claimsKey claimsContextKey
	actorKey  actorContextKey
)

// AuthMiddleware valida el access token, resuelve el Actor y exige permisos.
type AuthMiddleware struct {
	auth       *service.AuthService
	actorStore service.ActorStore
	auditoria  service.AuditoriaStore
	logger     *slog.Logger
}

// NewAuthMiddleware construye el middleware de autenticación. El ActorStore es
// opcional para facilitar pruebas aisladas del middleware; en producción el
// router siempre lo inyecta.
func NewAuthMiddleware(
	auth *service.AuthService,
	auditoria service.AuditoriaStore,
	logger *slog.Logger,
	actorStore ...service.ActorStore,
) *AuthMiddleware {
	var store service.ActorStore
	if len(actorStore) > 0 {
		store = actorStore[0]
	}
	return &AuthMiddleware{auth: auth, actorStore: store, auditoria: auditoria, logger: logger}
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

		claims, err := m.auth.ValidarSesion(r.Context(), strings.TrimSpace(partes[1]))
		if err != nil {
			if errors.Is(err, entities.ErrUnauthorized) {
				writeUnauthorized(w)
				return
			}
			m.logger.Error("no se pudo validar la sesión", "error", err)
			writeServiceError(w, r, m.logger, err)
			return
		}
		if meta := requestMetaFromContext(r.Context()); meta != nil {
			meta.Username = claims.Username
			meta.UsuarioID = claims.UsuarioID
		}

		actor := &service.Actor{
			UsuarioID: claims.UsuarioID,
			Username:  claims.Username,
			Roles:     claims.Roles,
			Permisos:  claims.Permisos,
		}
		if m.actorStore != nil {
			vinculos, err := m.actorStore.ResolverVinculos(r.Context(), claims.UsuarioID)
			if err != nil {
				if errors.Is(err, entities.ErrNotFound) {
					writeUnauthorized(w)
					return
				}
				m.logger.Error("no se pudieron resolver los vínculos de la cuenta", "error", err)
				writeServiceError(w, r, m.logger, err)
				return
			}
			actor.Email = vinculos.Email
			actor.EmailVerificado = vinculos.EmailVerificado
			actor.PacienteID = vinculos.PacienteID
			actor.MedicoID = vinculos.MedicoID
		}

		if permission != "" && !tienePermiso(claims, permission) {
			m.registrarDenegacion(r, claims, permission)
			writeError(w, http.StatusForbidden, "forbidden", "no tienes permiso para realizar esta acción")
			return
		}

		ctx := context.WithValue(r.Context(), claimsKey, claims)
		ctx = context.WithValue(ctx, actorKey, actor)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// soloPersonalAdministrativo evita que un médico use los endpoints globales
// de pacientes o citas. No basta con retirar permisos de la base: un access
// token emitido antes de la migración conserva los permisos antiguos hasta
// expirar. Esta guarda se evalúa por roles, no por permisos embebidos.
func soloPersonalAdministrativo(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor := actorFromContext(r.Context())
		if actor == nil {
			writeUnauthorized(w)
			return
		}
		if !actor.PuedeGestionarDatosGlobales() {
			writeError(w, http.StatusForbidden, "forbidden",
				"este endpoint es administrativo; usa los endpoints de tu portal")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func soloAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor := actorFromContext(r.Context())
		if actor == nil {
			writeUnauthorized(w)
			return
		}
		if !actor.TieneRol("admin") {
			writeError(w, http.StatusForbidden, "forbidden",
				"esta operación excepcional requiere rol admin")
			return
		}
		next.ServeHTTP(w, r)
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

// actorFromContext devuelve el Actor resuelto por el middleware de auth.
func actorFromContext(ctx context.Context) *service.Actor {
	actor, _ := ctx.Value(actorKey).(*service.Actor)
	return actor
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
		TraceID:    observability.TraceID(r.Context()),
	}
	if err := m.auditoria.Registrar(ctx, entrada); err != nil {
		m.logger.Error("no se pudo auditar el acceso denegado", "error", err)
	}
}
