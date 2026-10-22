package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"regexp"

	"medilist/internal/service"
)

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{8,64}$`)

// RequestMeta viaja en el contexto y permite que el middleware de auth
// complete el usuario para los logs y la auditoría de errores.
type RequestMeta struct {
	ID       string
	Username string
}

type requestMetaContextKey struct{}
type errorServiceContextKey struct{}

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if !requestIDPattern.MatchString(id) {
			id = nuevoTraceID()
		}
		w.Header().Set("X-Request-ID", id)

		meta := &RequestMeta{ID: id}
		ctx := context.WithValue(r.Context(), requestMetaContextKey{}, meta)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func withErrorService(s *service.ErrorService, next http.Handler) http.Handler {
	if s == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), errorServiceContextKey{}, s)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requestMetaFromContext(ctx context.Context) *RequestMeta {
	meta, _ := ctx.Value(requestMetaContextKey{}).(*RequestMeta)
	return meta
}

func errorServiceFromContext(ctx context.Context) *service.ErrorService {
	svc, _ := ctx.Value(errorServiceContextKey{}).(*service.ErrorService)
	return svc
}

func requestIDFromContext(ctx context.Context) string {
	if meta := requestMetaFromContext(ctx); meta != nil {
		return meta.ID
	}
	return ""
}

func nuevoTraceID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "trace-sin-aleatoriedad"
	}
	return hex.EncodeToString(buf)
}
