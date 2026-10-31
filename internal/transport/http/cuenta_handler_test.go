package httpapi

import (
	"strings"
	"testing"
)

func TestNormalizarEmailCuenta(t *testing.T) {
	email, err := normalizarEmailCuenta("  Nueva@Example.Test ")
	if err != nil {
		t.Fatalf("email válido rechazado: %v", err)
	}
	if email != "nueva@example.test" {
		t.Fatalf("email normalizado inesperado: %q", email)
	}

	if _, err := normalizarEmailCuenta("   "); err == nil {
		t.Fatal("el email vacío debe rechazarse")
	}
	if _, err := normalizarEmailCuenta(strings.Repeat("a", 150) + "@example.test"); err == nil {
		t.Fatal("un email mayor a 160 caracteres debe rechazarse antes del rate limiter")
	}
}
