package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"medilist/internal/observability"
	"medilist/internal/service"
	"medilist/internal/service/entities"
)

// auditMutations registra en la bitácora toda operación de escritura.
// Debe ejecutarse después del middleware de autenticación para conocer al usuario.
func auditMutations(store service.AuditoriaStore, logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)

		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			return
		}
		claims := claimsFromContext(r.Context())
		if claims == nil {
			return
		}
		estado := rec.status
		if estado == 0 {
			estado = http.StatusOK
		}

		entrada := &entities.Auditoria{
			UsuarioID:  &claims.UsuarioID,
			Username:   claims.Username,
			Accion:     accionDeSolicitud(r),
			Recurso:    recursoDeRuta(r.URL.Path),
			Metodo:     r.Method,
			Ruta:       r.URL.Path,
			EstadoHTTP: estado,
			IP:         clientIP(r),
			UserAgent:  r.UserAgent(),
			TraceID:    observability.TraceID(r.Context()),
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := store.Registrar(ctx, entrada); err != nil {
			logger.Error("no se pudo registrar la auditoría", "error", err)
		}
	})
}

// auditSensitiveReads deja evidencia de quién consultó la bitácora o los
// errores agregados. Son operaciones de solo lectura, pero exponen
// información sensible para la supervisión del sistema.
func auditSensitiveReads(store service.AuditoriaStore, logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)

		claims := claimsFromContext(r.Context())
		if claims == nil {
			return
		}
		estado := rec.status
		if estado == 0 {
			estado = http.StatusOK
		}

		entrada := &entities.Auditoria{
			UsuarioID:  &claims.UsuarioID,
			Username:   claims.Username,
			Accion:     "consultar",
			Recurso:    recursoDeRuta(r.URL.Path),
			Metodo:     r.Method,
			Ruta:       r.URL.Path,
			EstadoHTTP: estado,
			IP:         clientIP(r),
			UserAgent:  r.UserAgent(),
			TraceID:    observability.TraceID(r.Context()),
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := store.Registrar(ctx, entrada); err != nil {
			logger.Error("no se pudo registrar la consulta auditada", "error", err)
		}
	})
}

func accionDeSolicitud(r *http.Request) string {
	switch {
	case strings.HasSuffix(r.URL.Path, "/revocar-sesiones"):
		return "revocar_sesiones"
	case strings.HasSuffix(r.URL.Path, "/desbloquear"):
		return "desbloquear"
	case strings.HasSuffix(r.URL.Path, "/estado"):
		return "cambiar_estado"
	}
	return accionDeMetodo(r.Method)
}

func accionDeMetodo(method string) string {
	switch method {
	case http.MethodPost:
		return "crear"
	case http.MethodPut, http.MethodPatch:
		return "actualizar"
	case http.MethodDelete:
		return "eliminar"
	default:
		return strings.ToLower(method)
	}
}

func recursoDeRuta(path string) string {
	path = strings.TrimPrefix(path, "/api/v1/")
	if path == "" {
		return ""
	}
	partes := strings.Split(path, "/")
	if len(partes) > 0 {
		return partes[0]
	}
	return ""
}
