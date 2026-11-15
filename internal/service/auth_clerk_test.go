package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"medilist/internal/auth"
	"medilist/internal/service/entities"
)

type fakeClerkVerifier struct {
	identidad *auth.IdentidadClerk
	err       error
}

func (f fakeClerkVerifier) Verificar(context.Context, string) (*auth.IdentidadClerk, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.identidad, nil
}

type fakeUsuarioClerk struct {
	UsuarioStore
	porEmail map[string]*entities.Usuario
	porClerk map[string]*entities.Usuario
	creados  int
}

func (f *fakeUsuarioClerk) BuscarPorUsername(context.Context, string) (*entities.Usuario, error) {
	return nil, entities.ErrNotFound
}

func (f *fakeUsuarioClerk) BuscarPorEmail(_ context.Context, email string) (*entities.Usuario, error) {
	if u, ok := f.porEmail[strings.ToLower(email)]; ok {
		return u, nil
	}
	return nil, entities.ErrNotFound
}

func (f *fakeUsuarioClerk) BuscarPorClerkID(_ context.Context, id string) (*entities.Usuario, error) {
	if u, ok := f.porClerk[id]; ok {
		return u, nil
	}
	return nil, entities.ErrNotFound
}

func (f *fakeUsuarioClerk) Crear(_ context.Context, u *entities.Usuario) error {
	f.creados++
	u.ID = int64(100 + f.creados)
	f.porEmail[strings.ToLower(u.Email)] = u
	f.porClerk[u.ClerkUserID] = u
	return nil
}

func (f *fakeUsuarioClerk) ObtenerPorID(_ context.Context, id int64) (*entities.Usuario, error) {
	for _, u := range f.porEmail {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, entities.ErrNotFound
}

func (f *fakeUsuarioClerk) MarcarEmailVerificado(_ context.Context, id int64, cuando time.Time) error {
	for _, u := range f.porEmail {
		if u.ID == id && u.EmailVerificadoEn == nil {
			u.EmailVerificadoEn = &cuando
		}
	}
	return nil
}

func (f *fakeUsuarioClerk) VincularClerkID(_ context.Context, id int64, clerkID string) error {
	for _, u := range f.porEmail {
		if u.ID == id {
			u.ClerkUserID = clerkID
			f.porClerk[clerkID] = u
			return nil
		}
	}
	return entities.ErrNotFound
}

type fakeRefreshClerk struct {
	RefreshTokenStore
	creados int
}

func (f *fakeRefreshClerk) Crear(context.Context, *entities.RefreshToken) error {
	f.creados++
	return nil
}

func TestLoginClerkSinCuentaDevuelveRegistroPendiente(t *testing.T) {
	store := &fakeUsuarioClerk{
		porEmail: map[string]*entities.Usuario{},
		porClerk: map[string]*entities.Usuario{},
	}
	refresh := &fakeRefreshClerk{}
	svc := NewAuthService(AuthConfig{
		JWTSecret:    strings.Repeat("s", 64),
		JWTIssuer:    "medilist",
		AccessTTL:    time.Minute,
		RefreshTTL:   time.Hour,
		TermsVersion: "2026-10",
	}, store, refresh, nil, fakeClerkVerifier{identidad: &auth.IdentidadClerk{
		Subject:   "user_123",
		Email:     "Paciente@Example.com",
		Nombres:   "Ana",
		Apellidos: "Pérez",
	}})

	resultado, err := svc.LoginClerk(context.Background(), "token", "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("LoginClerk devolvió error: %v", err)
	}
	if resultado.Pendiente == nil || resultado.Sesion != nil {
		t.Fatal("una cuenta inexistente debe devolver registro pendiente")
	}
	if resultado.Pendiente.Email != "paciente@example.com" {
		t.Fatalf("email no normalizado: %q", resultado.Pendiente.Email)
	}
	if store.creados != 0 || refresh.creados != 0 {
		t.Fatalf("no debe crear cuenta ni sesión antes de completar datos: usuarios=%d refresh=%d", store.creados, refresh.creados)
	}
}

func TestLoginClerkVinculaCuentaExistentePorEmail(t *testing.T) {
	existente := &entities.Usuario{
		ID:       7,
		Username: "ana",
		Email:    "ana@example.com",
		Activo:   true,
		Roles:    []string{"paciente"},
	}
	store := &fakeUsuarioClerk{
		porEmail: map[string]*entities.Usuario{"ana@example.com": existente},
		porClerk: map[string]*entities.Usuario{},
	}
	refresh := &fakeRefreshClerk{}
	svc := NewAuthService(AuthConfig{
		JWTSecret:  strings.Repeat("s", 64),
		JWTIssuer:  "medilist",
		AccessTTL:  time.Minute,
		RefreshTTL: time.Hour,
	}, store, refresh, nil, fakeClerkVerifier{identidad: &auth.IdentidadClerk{
		Subject: "user_456",
		Email:   "ana@example.com",
	}})

	resultado, err := svc.LoginClerk(context.Background(), "token", "", "")
	if err != nil {
		t.Fatalf("LoginClerk devolvió error: %v", err)
	}
	if resultado.Sesion == nil || resultado.Sesion.AccessToken == "" || resultado.Sesion.RefreshToken == "" {
		t.Fatal("la cuenta existente debe recibir una sesión")
	}
	if store.creados != 0 {
		t.Fatal("no debe crear una cuenta nueva si el correo ya existe")
	}
	if existente.ClerkUserID != "user_456" {
		t.Fatalf("clerk_user_id no vinculado: %q", existente.ClerkUserID)
	}
	if existente.EmailVerificadoEn == nil {
		t.Fatal("el correo debe marcarse como verificado")
	}
}

func TestLoginClerkVinculaCuentaInternaPorEmail(t *testing.T) {
	admin := &entities.Usuario{
		ID:       9,
		Username: "admin",
		Email:    "admin@example.com",
		Activo:   true,
		Roles:    []string{"admin"},
	}
	store := &fakeUsuarioClerk{
		porEmail: map[string]*entities.Usuario{"admin@example.com": admin},
		porClerk: map[string]*entities.Usuario{},
	}
	svc := NewAuthService(AuthConfig{
		JWTSecret:  strings.Repeat("s", 64),
		JWTIssuer:  "medilist",
		AccessTTL:  time.Minute,
		RefreshTTL: time.Hour,
	}, store, &fakeRefreshClerk{}, nil, fakeClerkVerifier{identidad: &auth.IdentidadClerk{
		Subject: "user_789",
		Email:   "admin@example.com",
	}})

	resultado, err := svc.LoginClerk(context.Background(), "token", "", "")
	if err != nil {
		t.Fatalf("LoginClerk devolvió error: %v", err)
	}
	if resultado.Sesion == nil {
		t.Fatal("la cuenta interna debe recibir una sesión")
	}
	if admin.ClerkUserID != "user_789" {
		t.Fatal("la cuenta interna debe vincularse con Clerk por correo verificado")
	}
}

func TestLoginClerkSinConfiguracionDevuelveForbidden(t *testing.T) {
	svc := NewAuthService(AuthConfig{
		JWTSecret: strings.Repeat("s", 64),
		JWTIssuer: "medilist",
		AccessTTL: time.Minute,
	}, &fakeUsuarioClerk{}, nil, nil)

	if _, err := svc.LoginClerk(context.Background(), "token", "", ""); !errors.Is(err, entities.ErrForbidden) {
		t.Fatalf("se esperaba ErrForbidden, se obtuvo: %v", err)
	}
}
