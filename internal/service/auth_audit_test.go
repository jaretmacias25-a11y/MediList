package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"medilist/internal/observability"
	"medilist/internal/service/entities"
)

type fakeAuditoriaAuth struct {
	registros []entities.Auditoria
}

func (f *fakeAuditoriaAuth) Registrar(_ context.Context, a *entities.Auditoria) error {
	f.registros = append(f.registros, *a)
	return nil
}

func (f *fakeAuditoriaAuth) Listar(context.Context, entities.AuditoriaFiltro) ([]entities.Auditoria, error) {
	return nil, nil
}

func TestAuthServiceAuditaTraceIDDesdeContexto(t *testing.T) {
	store := &fakeAuditoriaAuth{}
	svc := NewAuthService(AuthConfig{
		JWTSecret: strings.Repeat("s", 64),
		JWTIssuer: "medilist",
		AccessTTL: time.Minute,
	}, nil, nil, store)

	ctx := observability.WithTraceID(context.Background(), "trace-login-12345678")
	svc.auditar(ctx, nil, "ana", "login_fallido", "127.0.0.1", "test", 401)

	if len(store.registros) != 1 {
		t.Fatalf("se esperaba 1 registro, obtenidos %d", len(store.registros))
	}
	if store.registros[0].TraceID != "trace-login-12345678" {
		t.Fatalf("trace_id inesperado: %q", store.registros[0].TraceID)
	}
}
