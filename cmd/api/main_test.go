package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestErrorLogDelServidorUsaJSONEstructurado(t *testing.T) {
	var salida bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&salida, nil)).With("service", "medilist-api")

	serverLog := slog.NewLogLogger(logger.Handler(), slog.LevelError)
	serverLog.Print("http: error interno de prueba")

	var entrada map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(salida.String())), &entrada); err != nil {
		t.Fatalf("el log del servidor no es JSON válido: %v\n%s", err, salida.String())
	}
	if entrada["service"] != "medilist-api" {
		t.Fatalf("falta el atributo service: %+v", entrada)
	}
	if entrada["level"] != "ERROR" {
		t.Fatalf("nivel inesperado: %+v", entrada)
	}
	if msg, _ := entrada["msg"].(string); !strings.Contains(msg, "http: error interno de prueba") {
		t.Fatalf("mensaje inesperado: %+v", entrada)
	}
}
