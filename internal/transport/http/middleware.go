package httpapi

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
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

func withMiddleware(next http.Handler, logger *slog.Logger) http.Handler {
	return recoverPanic(logger, logRequests(logger, securityHeaders(next)))
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
		logger.Info("solicitud HTTP",
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", r.RemoteAddr,
		)
	})
}

func recoverPanic(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error("panic recuperado",
					"error", rec,
					"path", r.URL.Path,
					"stack", string(debug.Stack()),
				)
				writeError(w, http.StatusInternalServerError, "internal_error", "error interno del servidor")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
