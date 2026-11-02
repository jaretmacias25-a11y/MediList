package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
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

func withMiddleware(
	next http.Handler,
	logger *slog.Logger,
	errores *service.ErrorService,
	corsOrigins []string,
	hsts bool,
) http.Handler {
	// CORS queda fuera de auth para responder preflight sin token. requestID
	// sigue siendo el primer contexto de trazabilidad de la solicitud real.
	handler := requestID(withErrorService(errores, logRequests(logger,
		recoverPanic(logger, securityHeaders(hsts, next)))))
	return corsMiddleware(corsOrigins, handler)
}

func securityHeaders(hsts bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-site")
		w.Header().Set("X-Permitted-Cross-Domain-Policies", "none")
		if hsts {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		// Los datos clínicos y los tokens no deben quedar en cachés intermedias.
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		next.ServeHTTP(w, r)
	})
}

func corsMiddleware(allowedOrigins []string, next http.Handler) http.Handler {
	if len(allowedOrigins) == 0 {
		return next
	}
	permitidos := make(map[string]struct{}, len(allowedOrigins))
	wildcard := false
	for _, origen := range allowedOrigins {
		if origen == "*" {
			wildcard = true
			continue
		}
		permitidos[origen] = struct{}{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origen := strings.TrimSpace(r.Header.Get("Origin"))
		if origen == "" {
			next.ServeHTTP(w, r)
			return
		}
		_, permitido := permitidos[origen]
		if !permitido && !wildcard {
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		if wildcard {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", origen)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID, traceparent")
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
		w.Header().Set("Access-Control-Max-Age", "600")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
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
