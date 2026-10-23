package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

type fakeRetentionStore struct {
	errores        int64
	auditoria      int64
	errErrores     error
	errAuditoria   error
	antesErrores   time.Time
	antesAuditoria time.Time
	llamoErrores   bool
	llamoAuditoria bool
}

func (f *fakeRetentionStore) PurgarErrores(_ context.Context, antes time.Time) (int64, error) {
	f.llamoErrores = true
	f.antesErrores = antes
	return f.errores, f.errErrores
}

func (f *fakeRetentionStore) PurgarAuditoria(_ context.Context, antes time.Time) (int64, error) {
	f.llamoAuditoria = true
	f.antesAuditoria = antes
	return f.auditoria, f.errAuditoria
}

func TestRetentionServicePurgaErroresYAuditoria(t *testing.T) {
	var salida bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&salida, nil))
	store := &fakeRetentionStore{errores: 3, auditoria: 5}
	svc := NewRetentionService(store, RetentionConfig{
		Interval:       24 * time.Hour,
		ErrorRetention: 90 * 24 * time.Hour,
		AuditRetention: 365 * 24 * time.Hour,
	}, logger)
	ahora := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	svc.ahora = func() time.Time { return ahora }

	svc.Ejecutar(context.Background())

	if !store.llamoErrores || !store.llamoAuditoria {
		t.Fatal("se debieron purgar ambas tablas")
	}
	if !store.antesErrores.Equal(ahora.Add(-90 * 24 * time.Hour)) {
		t.Fatalf("corte de errores inesperado: %s", store.antesErrores)
	}
	if !store.antesAuditoria.Equal(ahora.Add(-365 * 24 * time.Hour)) {
		t.Fatalf("corte de auditoría inesperado: %s", store.antesAuditoria)
	}

	eventos := leerEventosRetencion(t, salida.String())
	if len(eventos) != 2 {
		t.Fatalf("se esperaban 2 eventos de retención, obtenidos %d", len(eventos))
	}
	if eventos[0]["tabla"] != "errores_aplicacion" || eventos[0]["eliminados"] != float64(3) {
		t.Fatalf("evento de errores inesperado: %+v", eventos[0])
	}
	if eventos[1]["tabla"] != "auditoria" || eventos[1]["eliminados"] != float64(5) {
		t.Fatalf("evento de auditoría inesperado: %+v", eventos[1])
	}
}

func TestRetentionServiceContinuaSiFallaUnaTabla(t *testing.T) {
	var salida bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&salida, nil))
	store := &fakeRetentionStore{
		errErrores: errors.New("tabla bloqueada"),
		auditoria:  2,
	}
	svc := NewRetentionService(store, RetentionConfig{
		ErrorRetention: 90 * 24 * time.Hour,
		AuditRetention: 365 * 24 * time.Hour,
	}, logger)

	svc.Ejecutar(context.Background())

	if !store.llamoAuditoria {
		t.Fatal("la falla de errores no debe impedir la purga de auditoría")
	}
	eventos := leerEventosRetencion(t, salida.String())
	if len(eventos) != 2 {
		t.Fatalf("se esperaban 2 eventos, obtenidos %d", len(eventos))
	}
	if eventos[0]["msg"] != "no se pudo aplicar la retención de errores" {
		t.Fatalf("no se registró la falla de errores: %+v", eventos[0])
	}
	if eventos[1]["msg"] != "retencion_aplicada" {
		t.Fatalf("no se registró la purga de auditoría: %+v", eventos[1])
	}
}

func TestRetentionServiceOmiteTablaDeshabilitada(t *testing.T) {
	var salida bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&salida, nil))
	store := &fakeRetentionStore{auditoria: 1}
	svc := NewRetentionService(store, RetentionConfig{
		ErrorRetention: 0,
		AuditRetention: 30 * 24 * time.Hour,
	}, logger)

	svc.Ejecutar(context.Background())

	if store.llamoErrores {
		t.Fatal("la retención de errores está deshabilitada")
	}
	if !store.llamoAuditoria {
		t.Fatal("la retención de auditoría debe ejecutarse")
	}
}

func leerEventosRetencion(t *testing.T, salida string) []map[string]any {
	t.Helper()
	var eventos []map[string]any
	for _, linea := range strings.Split(strings.TrimSpace(salida), "\n") {
		if strings.TrimSpace(linea) == "" {
			continue
		}
		var evento map[string]any
		if err := json.Unmarshal([]byte(linea), &evento); err != nil {
			t.Fatalf("log no es JSON válido: %v\n%s", err, linea)
		}
		eventos = append(eventos, evento)
	}
	return eventos
}
