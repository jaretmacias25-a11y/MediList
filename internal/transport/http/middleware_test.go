package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"medilist/internal/service"
	"medilist/internal/service/entities"
)

type fakeErrorStore struct {
	registrados []entities.ErrorAplicacion
}

func (f *fakeErrorStore) Registrar(_ context.Context, e *entities.ErrorAplicacion) error {
	f.registrados = append(f.registrados, *e)
	return nil
}

func (f *fakeErrorStore) Listar(context.Context, entities.ErrorFiltro) ([]entities.ErrorAplicacion, error) {
	return nil, nil
}

func TestMiddlewareRegistraPanicConTraceID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := &fakeErrorStore{}
	errores := service.NewErrorService(store, "test-service", logger)

	handler := withMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("fallo de prueba")
	}), logger, errores)

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	req.Header.Set("X-Request-ID", "test-trace-12345678")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status esperado 500, obtenido %d", rec.Code)
	}
	if got := rec.Header().Get("X-Request-ID"); got != "test-trace-12345678" {
		t.Fatalf("X-Request-ID inesperado: %q", got)
	}
	if !strings.Contains(rec.Body.String(), `"request_id":"test-trace-12345678"`) {
		t.Fatalf("la respuesta no contiene el request_id: %s", rec.Body.String())
	}
	if len(store.registrados) != 1 {
		t.Fatalf("errores registrados esperados 1, obtenidos %d", len(store.registrados))
	}
	registrado := store.registrados[0]
	if registrado.TraceID != "test-trace-12345678" || registrado.Codigo != "panic" || registrado.EstadoHTTP != 500 {
		t.Fatalf("error registrado inesperado: %+v", registrado)
	}
	if registrado.Servicio != "test-service" {
		t.Fatalf("servicio inesperado: %q", registrado.Servicio)
	}
}
