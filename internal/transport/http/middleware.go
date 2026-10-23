package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"medilist/internal/service"
)

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(status int) {
	if r.status != 0 {
		return
	}
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(b)
}

func (r *responseRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func withMiddleware(next http.Handler, logger *slog.Logger, errores *service.ErrorService) http.Handler {
	// requestID es el más externo para que el trace_id esté disponible también
	// durante la recuperación de pánicos y el log de acceso. logRequests queda
	// fuera de recoverPanic para registrar también las solicitudes que fallan.
	return requestID(withErrorService(errores, logRequests(logger, recoverPanic(logger, securityHeaders(next)))))
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		// Los datos clínicos y los tokens no deben quedar en cachés intermedias.
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		next.ServeHTTP(w, r)
	})
}

func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}
		traceID := ""
		username := ""
		var usuarioID int64
		if meta := requestMetaFromContext(r.Context()); meta != nil {
			traceID = meta.ID
			username = meta.Username
			usuarioID = meta.UsuarioID
		}
		logger.Info("solicitud HTTP",
			"trace_id", traceID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", r.RemoteAddr,
			"usuario", username,
			"usuario_id", usuarioID,
		)
	})
}

func recoverPanic(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "error interno del servidor")
				reportarError(r, logger, http.StatusInternalServerError, "panic",
					"pánico recuperado", fmt.Errorf("%v", rec), string(debug.Stack()))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
