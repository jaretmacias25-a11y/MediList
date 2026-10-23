package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"medilist/internal/observability"
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
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	store := &fakeErrorStore{}
	errores := service.NewErrorService(store, "test-service", "test-instance", logger)

	handler := withMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("fallo de prueba")
	}), logger, errores)

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	req.Header.Set("X-Request-ID", "test-trace-12345678")
	req = req.WithContext(context.WithValue(req.Context(), claimsKey, &service.Claims{
		UsuarioID: 7,
		Username:  "ana",
	}))
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
	if registrado.Servicio != "test-service" || registrado.Instancia != "test-instance" {
		t.Fatalf("servicio/instancia inesperados: %+v", registrado)
	}

	requestLog, errorLog := buscarLogs(t, logs.String())
	if requestLog["trace_id"] != "test-trace-12345678" {
		t.Fatalf("trace_id ausente en el log de solicitud: %+v", requestLog)
	}
	if requestLog["status"] != float64(500) {
		t.Fatalf("status inesperado en el log de solicitud: %+v", requestLog)
	}
	if errorLog["trace_id"] != "test-trace-12345678" {
		t.Fatalf("trace_id ausente en el log de error: %+v", errorLog)
	}
	if errorLog["codigo"] != "panic" || errorLog["servicio"] != "test-service" {
		t.Fatalf("log de error inesperado: %+v", errorLog)
	}
	if errorLog["instancia"] != "test-instance" {
		t.Fatalf("instancia ausente en el log de error: %+v", errorLog)
	}
	if errorLog["usuario"] != "ana" || errorLog["usuario_id"] != float64(7) {
		t.Fatalf("usuario ausente en el log de error: %+v", errorLog)
	}
	if stack, _ := errorLog["stack_trace"].(string); stack == "" {
		t.Fatalf("stack_trace ausente en el log de error: %+v", errorLog)
	}
}

func TestRequestIDReemplazaValorInvalido(t *testing.T) {
	handler := requestID(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "corto")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	id := rec.Header().Get("X-Request-ID")
	if id == "corto" {
		t.Fatal("no se debe confiar en un X-Request-ID inválido")
	}
	if !requestIDPattern.MatchString(id) {
		t.Fatalf("se generó un request ID inválido: %q", id)
	}
}

func TestRequestIDPriorizaTraceparentW3C(t *testing.T) {
	traceID := "4bf92f3577b34da6a3ce929d0e0e4736"
	var traceEnContexto string
	handler := requestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceEnContexto = observability.TraceID(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("traceparent", "00-4BF92F3577B34DA6A3CE929D0E0E4736-00f067aa0ba902b7-01")
	req.Header.Set("X-Request-ID", "otro-identificador")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-ID"); got != traceID {
		t.Fatalf("se esperaba el trace-id W3C %q, obtenido %q", traceID, got)
	}
	if traceEnContexto != traceID {
		t.Fatalf("trace_id ausente en el contexto: %q", traceEnContexto)
	}
}

func TestRequestIDIgnoraTraceparentInvalido(t *testing.T) {
	handler := requestID(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("traceparent", "00-00000000000000000000000000000000-00f067aa0ba902b7-01")
	req.Header.Set("X-Request-ID", "fallback-12345678")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-ID"); got != "fallback-12345678" {
		t.Fatalf("se esperaba el fallback X-Request-ID, obtenido %q", got)
	}
}

func buscarLogs(t *testing.T, salida string) (map[string]any, map[string]any) {
	t.Helper()
	var requestLog, errorLog map[string]any
	for _, linea := range strings.Split(strings.TrimSpace(salida), "\n") {
		if strings.TrimSpace(linea) == "" {
			continue
		}
		var entrada map[string]any
		if err := json.Unmarshal([]byte(linea), &entrada); err != nil {
			t.Fatalf("log no es JSON válido: %v\n%s", err, linea)
		}
		switch entrada["msg"] {
		case "solicitud HTTP":
			requestLog = entrada
		case "error_aplicacion":
			errorLog = entrada
		}
	}
	if requestLog == nil {
		t.Fatalf("no se encontró el log de solicitud:\n%s", salida)
	}
	if errorLog == nil {
		t.Fatalf("no se encontró el log de error:\n%s", salida)
	}
	return requestLog, errorLog
}
