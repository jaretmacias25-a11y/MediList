package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"medilist/internal/service/entities"
)

func TestTokenManagerGenerarValidar(t *testing.T) {
	manager := NewTokenManager(strings.Repeat("s", 64), "medilist", time.Minute)
	usuario := &entities.Usuario{
		ID:           7,
		Username:     "ana",
		TokenVersion: 4,
		Roles:        []string{"medico"},
		Permisos:     []string{"citas:leer"},
	}

	token, expira, err := manager.Generar(usuario)
	if err != nil {
		t.Fatalf("no se pudo generar el token: %v", err)
	}
	if token == "" {
		t.Fatal("el token no debe estar vacío")
	}
	if !expira.After(time.Now()) {
		t.Fatal("la expiración debe estar en el futuro")
	}

	claims, err := manager.Validar(token)
	if err != nil {
		t.Fatalf("el token debe ser válido: %v", err)
	}
	if claims.UsuarioID != usuario.ID || claims.Username != usuario.Username {
		t.Fatalf("claims inesperados: %+v", claims)
	}
	if claims.TokenVersion != usuario.TokenVersion {
		t.Fatalf("versión de token inesperada: %d", claims.TokenVersion)
	}
	if len(claims.Roles) != 1 || claims.Roles[0] != "medico" {
		t.Fatalf("roles inesperados: %v", claims.Roles)
	}
}

func TestTokenManagerRechazaTokenAlterado(t *testing.T) {
	manager := NewTokenManager(strings.Repeat("s", 64), "medilist", time.Minute)
	token, _, err := manager.Generar(&entities.Usuario{ID: 1, Username: "ana"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := manager.Validar(token + "x"); !errors.Is(err, entities.ErrUnauthorized) {
		t.Fatalf("se esperaba ErrUnauthorized, se obtuvo: %v", err)
	}
}

func TestTokenManagerRechazaOtroSecretoEmisorOExpiracion(t *testing.T) {
	secreto := strings.Repeat("s", 64)
	manager := NewTokenManager(secreto, "medilist", time.Minute)
	token, _, err := manager.Generar(&entities.Usuario{ID: 1, Username: "ana"})
	if err != nil {
		t.Fatal(err)
	}

	casos := []struct {
		nombre  string
		manager *TokenManager
	}{
		{"otro secreto", NewTokenManager(strings.Repeat("x", 64), "medilist", time.Minute)},
		{"otro emisor", NewTokenManager(secreto, "otro", time.Minute)},
	}
	for _, caso := range casos {
		if _, err := caso.manager.Validar(token); !errors.Is(err, entities.ErrUnauthorized) {
			t.Errorf("%s: se esperaba ErrUnauthorized, se obtuvo: %v", caso.nombre, err)
		}
	}

	expiradoManager := NewTokenManager(secreto, "medilist", -time.Minute)
	expirado, _, err := expiradoManager.Generar(&entities.Usuario{ID: 1, Username: "ana"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := expiradoManager.Validar(expirado); !errors.Is(err, entities.ErrUnauthorized) {
		t.Fatalf("expirado: se esperaba ErrUnauthorized, se obtuvo: %v", err)
	}
}
