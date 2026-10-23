package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"regexp"
	"strings"

	"medilist/internal/observability"
	"medilist/internal/service"
)

var (
	requestIDPattern   = regexp.MustCompile(`^[A-Za-z0-9._-]{8,64}$`)
	traceparentPattern = regexp.MustCompile(`(?i)^00-([0-9a-f]{32})-([0-9a-f]{16})-[0-9a-f]{2}$`)
)

// RequestMeta viaja en el contexto y permite que el middleware de auth
// complete el usuario para los logs y la auditoría de errores.
type RequestMeta struct {
	ID        string
	Username  string
	UsuarioID int64
}

type requestMetaContextKey struct{}
type errorServiceContextKey struct{}

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// W3C traceparent tiene prioridad para integrarse con tracing
		// distribuido. Si no viene o es inválido, se usa X-Request-ID.
		id := traceIDFromTraceparent(r.Header.Get("traceparent"))
		if id == "" {
			id = strings.TrimSpace(r.Header.Get("X-Request-ID"))
			if !requestIDPattern.MatchString(id) {
				id = nuevoTraceID()
			}
		}
		w.Header().Set("X-Request-ID", id)

		meta := &RequestMeta{ID: id}
		ctx := context.WithValue(r.Context(), requestMetaContextKey{}, meta)
		ctx = observability.WithTraceID(ctx, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// traceIDFromTraceparent extrae el trace-id de un encabezado W3C válido.
// Se rechazan versiones distintas de 00 y trace-id/parent-id en cero.
func traceIDFromTraceparent(value string) string {
	partes := traceparentPattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(partes) != 3 {
		return ""
	}
	if esTodoCero(partes[1]) || esTodoCero(partes[2]) {
		return ""
	}
	return strings.ToLower(partes[1])
}

func esTodoCero(valor string) bool {
	for _, caracter := range valor {
		if caracter != '0' {
			return false
		}
	}
	return true
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
