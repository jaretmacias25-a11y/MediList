package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"medilist/internal/service/entities"
)

type fakeUsuarioStoreLoginCuenta struct {
	UsuarioStore
	usuario  *entities.Usuario
	intentos int
	exitos   int
}

func (f *fakeUsuarioStoreLoginCuenta) BuscarPorUsername(context.Context, string) (*entities.Usuario, error) {
	return f.usuario, nil
}

func (f *fakeUsuarioStoreLoginCuenta) RegistrarIntentoFallido(
	context.Context, int64, int, time.Duration,
) (int, *time.Time, error) {
	f.intentos++
	return f.intentos, nil, nil
}

func (f *fakeUsuarioStoreLoginCuenta) RegistrarLoginExitoso(context.Context, int64) error {
	f.exitos++
	return nil
}

func TestLoginPacienteSinEmailVerificadoNoEmiteSesion(t *testing.T) {
	hash, err := hashearPassword("ClaveSegura123!", 10)
	if err != nil {
		t.Fatal(err)
	}
	usuario := &entities.Usuario{
		ID:           10,
		Username:     "paciente",
		Email:        "paciente@example.test",
		PasswordHash: hash,
		Activo:       true,
		Roles:        []string{"paciente"},
	}
	store := &fakeUsuarioStoreLoginCuenta{usuario: usuario}
	auditoria := &fakeAuditoriaAuth{}
	svc := NewAuthService(AuthConfig{
		JWTSecret:        strings.Repeat("s", 64),
		JWTIssuer:        "medilist",
		AccessTTL:        time.Minute,
		RefreshTTL:       time.Hour,
		BcryptCost:       10,
		MaxLoginAttempts: 5,
		LockoutDuration:  time.Minute,
	}, store, nil, auditoria)

	_, err = svc.Login(context.Background(), "paciente", "ClaveSegura123!", "127.0.0.1", "prueba")
	if !errors.Is(err, entities.ErrEmailNoVerificado) {
		t.Fatalf("se esperaba email no verificado, obtenido: %v", err)
	}
	if store.exitos != 0 {
		t.Fatal("no debe registrar un login exitoso sin verificar el correo")
	}
	if store.intentos != 0 {
		t.Fatal("una contraseña correcta no debe contar como intento fallido")
	}
	if len(auditoria.registros) != 1 || auditoria.registros[0].Accion != "login_email_no_verificado" {
		t.Fatalf("auditoría inesperada: %+v", auditoria.registros)
	}
}

func TestLoginConPasswordIncorrectaNoRevelaEstadoDeVerificacion(t *testing.T) {
	hash, err := hashearPassword("ClaveSegura123!", 10)
	if err != nil {
		t.Fatal(err)
	}
	usuario := &entities.Usuario{
		ID:           10,
		Username:     "paciente",
		Email:        "paciente@example.test",
		PasswordHash: hash,
		Activo:       true,
		Roles:        []string{"paciente"},
	}
	store := &fakeUsuarioStoreLoginCuenta{usuario: usuario}
	svc := NewAuthService(AuthConfig{
		JWTSecret:        strings.Repeat("s", 64),
		JWTIssuer:        "medilist",
		AccessTTL:        time.Minute,
		RefreshTTL:       time.Hour,
		BcryptCost:       10,
		MaxLoginAttempts: 5,
		LockoutDuration:  time.Minute,
	}, store, nil, nil)

	_, err = svc.Login(context.Background(), "paciente", "PasswordIncorrecta1!", "127.0.0.1", "prueba")
	if !errors.Is(err, entities.ErrUnauthorized) {
		t.Fatalf("se esperaba credenciales inválidas, obtenido: %v", err)
	}
	if errors.Is(err, entities.ErrEmailNoVerificado) {
		t.Fatal("no debe revelarse el estado de verificación con una contraseña incorrecta")
	}
	if store.intentos != 1 {
		t.Fatalf("se esperaba un intento fallido, obtenidos %d", store.intentos)
	}
}
