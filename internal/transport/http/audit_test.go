package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"medilist/internal/service"
	"medilist/internal/service/entities"
)

type fakeAuditoriaStore struct {
	registros []entities.Auditoria
}

func (f *fakeAuditoriaStore) Registrar(_ context.Context, a *entities.Auditoria) error {
	f.registros = append(f.registros, *a)
	return nil
}

func (f *fakeAuditoriaStore) Listar(context.Context, entities.AuditoriaFiltro) ([]entities.Auditoria, error) {
	return nil, nil
}

func TestAuditSensitiveReadsRegistraTraceID(t *testing.T) {
	store := &fakeAuditoriaStore{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := requestID(auditSensitiveReads(store, logger, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/errores", nil)
	req.Header.Set("X-Request-ID", "trace-auditoria-12345678")
	req = req.WithContext(context.WithValue(req.Context(), claimsKey, &service.Claims{
		UsuarioID: 9,
		Username:  "auditor",
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if len(store.registros) != 1 {
		t.Fatalf("se esperaba 1 registro de auditoría, obtenidos %d", len(store.registros))
	}
	registro := store.registros[0]
	if registro.TraceID != "trace-auditoria-12345678" {
		t.Fatalf("trace_id inesperado: %q", registro.TraceID)
	}
	if registro.Accion != "consultar" || registro.Recurso != "errores" {
		t.Fatalf("registro inesperado: %+v", registro)
	}
	if registro.EstadoHTTP != http.StatusOK {
		t.Fatalf("estado inesperado: %d", registro.EstadoHTTP)
	}
}

func TestAuditMutationsRegistraTraceID(t *testing.T) {
	store := &fakeAuditoriaStore{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := requestID(auditMutations(store, logger, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/especialidades", nil)
	req.Header.Set("X-Request-ID", "trace-mutacion-12345678")
	req = req.WithContext(context.WithValue(req.Context(), claimsKey, &service.Claims{
		UsuarioID: 2,
		Username:  "admin",
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if len(store.registros) != 1 {
		t.Fatalf("se esperaba 1 registro de auditoría, obtenidos %d", len(store.registros))
	}
	registro := store.registros[0]
	if registro.TraceID != "trace-mutacion-12345678" {
		t.Fatalf("trace_id inesperado: %q", registro.TraceID)
	}
	if registro.Accion != "crear" || registro.Recurso != "especialidades" {
		t.Fatalf("registro inesperado: %+v", registro)
	}
	if registro.EstadoHTTP != http.StatusCreated {
		t.Fatalf("estado inesperado: %d", registro.EstadoHTTP)
	}
}
