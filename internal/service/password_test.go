package service

import (
	"strings"
	"testing"
)

func TestValidarPassword(t *testing.T) {
	validas := []string{
		"Admin12345!2026",
		"ClaveSegura#2026",
		"OtraClave$2026",
	}
	for _, password := range validas {
		if err := validarPassword(password); err != nil {
			t.Errorf("se esperaba contraseña válida %q, error: %v", password, err)
		}
	}

	invalidas := map[string]string{
		"corta":           "Ab1!",
		"sin_mayuscula":   "clave12345!",
		"sin_minuscula":   "CLAVE12345!",
		"sin_numero":      "ClaveSegura!",
		"sin_especial":    "ClaveSegura2026",
		"demasiado_larga": "Aa1!" + strings.Repeat("x", 70),
	}
	for nombre, password := range invalidas {
		if err := validarPassword(password); err == nil {
			t.Errorf("se esperaba error para %s", nombre)
		}
	}
}

func TestHashPassword(t *testing.T) {
	password := "ClaveSegura#2026"
	hash, err := hashearPassword(password, 10)
	if err != nil {
		t.Fatalf("no se pudo generar el hash: %v", err)
	}
	if hash == password {
		t.Fatal("el hash no debe ser igual a la contraseña")
	}
	if !verificarPassword(hash, password) {
		t.Fatal("la contraseña correcta debe verificarse")
	}
	if verificarPassword(hash, "OtraClave#2026") {
		t.Fatal("una contraseña incorrecta no debe verificarse")
	}
}
