package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"medilist/internal/service/entities"
)

type fakeErrorStorePersistencia struct {
	fallo   error
	recibio bool
}

func (f *fakeErrorStorePersistencia) Registrar(context.Context, *entities.ErrorAplicacion) error {
	f.recibio = true
	return f.fallo
}

func (f *fakeErrorStorePersistencia) Listar(context.Context, entities.ErrorFiltro) ([]entities.ErrorAplicacion, error) {
	return nil, nil
}

func TestErrorServiceNoFallaSiNoPuedePersistir(t *testing.T) {
	var salida bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&salida, nil))
	store := &fakeErrorStorePersistencia{fallo: errors.New("base no disponible")}
	svc := NewErrorService(store, "test-service", "test-instance", logger)

	svc.Registrar(context.Background(), &entities.ErrorAplicacion{
		TraceID:    "trace-12345678",
		Componente: "http",
		Nivel:      "error",
		Codigo:     "internal_error",
		Mensaje:    "fallo de prueba",
		StackTrace: "stack de prueba",
	})

	if !store.recibio {
		t.Fatal("el servicio debió intentar persistir el error")
	}
	var encontrados []map[string]any
	for _, linea := range strings.Split(strings.TrimSpace(salida.String()), "\n") {
		var entrada map[string]any
		if err := json.Unmarshal([]byte(linea), &entrada); err != nil {
			t.Fatalf("log no es JSON válido: %v", err)
		}
		encontrados = append(encontrados, entrada)
	}
	if len(encontrados) != 2 {
		t.Fatalf("se esperaban 2 logs (error y fallo de persistencia), obtenidos %d", len(encontrados))
	}
	if encontrados[0]["msg"] != "error_aplicacion" {
		t.Fatalf("primer log inesperado: %+v", encontrados[0])
	}
	if encontrados[0]["servicio"] != "test-service" || encontrados[0]["instancia"] != "test-instance" {
		t.Fatalf("faltan servicio/instancia en la agregación: %+v", encontrados[0])
	}
	if encontrados[0]["stack_trace"] != "stack de prueba" {
		t.Fatalf("falta el stack trace: %+v", encontrados[0])
	}
	if encontrados[1]["msg"] != "no se pudo persistir el error agregado" {
		t.Fatalf("no se registró la falla de persistencia: %+v", encontrados[1])
	}
}
