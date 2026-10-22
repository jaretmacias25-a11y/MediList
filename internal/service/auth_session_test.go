package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"medilist/internal/service/entities"
)

type fakeUsuarioStoreSesion struct {
	UsuarioStore
	valida bool
	err    error
}

func (f fakeUsuarioStoreSesion) SesionValida(context.Context, int64, int) (bool, error) {
	return f.valida, f.err
}

func TestValidarSesionCompruebaVersionEnBaseDeDatos(t *testing.T) {
	cfg := AuthConfig{
		JWTSecret: strings.Repeat("s", 64),
		JWTIssuer: "medilist",
		AccessTTL: time.Minute,
	}
	usuario := &entities.Usuario{ID: 5, Username: "ana", TokenVersion: 2}

	valido := NewAuthService(cfg, fakeUsuarioStoreSesion{valida: true}, nil, nil)
	token, _, err := valido.tokens.Generar(usuario)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := valido.ValidarSesion(context.Background(), token)
	if err != nil {
		t.Fatalf("la sesión debería ser válida: %v", err)
	}
	if claims.TokenVersion != usuario.TokenVersion {
		t.Fatalf("versión inesperada: %d", claims.TokenVersion)
	}

	revocado := NewAuthService(cfg, fakeUsuarioStoreSesion{valida: false}, nil, nil)
	if _, err := revocado.ValidarSesion(context.Background(), token); !errors.Is(err, entities.ErrUnauthorized) {
		t.Fatalf("se esperaba ErrUnauthorized, se obtuvo: %v", err)
	}
}

func TestValidarSesionPropagaErrorDeBaseDeDatos(t *testing.T) {
	cfg := AuthConfig{
		JWTSecret: strings.Repeat("s", 64),
		JWTIssuer: "medilist",
		AccessTTL: time.Minute,
	}
	fallo := errors.New("base de datos no disponible")
	svc := NewAuthService(cfg, fakeUsuarioStoreSesion{err: fallo}, nil, nil)
	token, _, err := svc.tokens.Generar(&entities.Usuario{ID: 1, Username: "ana"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := svc.ValidarSesion(context.Background(), token); !errors.Is(err, fallo) {
		t.Fatalf("se esperaba el error de base de datos, se obtuvo: %v", err)
	}
}
