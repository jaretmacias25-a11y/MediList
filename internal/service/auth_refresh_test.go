package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"medilist/internal/service/entities"
)

type fakeUsuarioRefresh struct {
	UsuarioStore
	mu           sync.Mutex
	usuario      entities.Usuario
	revocaciones int
}

func (f *fakeUsuarioRefresh) ObtenerPorID(context.Context, int64) (*entities.Usuario, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	copia := f.usuario
	return &copia, nil
}

func (f *fakeUsuarioRefresh) RevocarSesiones(context.Context, int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.revocaciones++
	f.usuario.TokenVersion++
	return nil
}

type fakeRefreshStore struct {
	RefreshTokenStore
	mu         sync.Mutex
	token      entities.RefreshToken
	rotaciones int
	revocados  int
}

func (f *fakeRefreshStore) BuscarPorHash(context.Context, string) (*entities.RefreshToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	copia := f.token
	return &copia, nil
}

func (f *fakeRefreshStore) Rotar(_ context.Context, id int64, nuevo *entities.RefreshToken) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.token.ID != id || f.token.RevocadoEn != nil || !f.token.ExpiraEn.After(time.Now()) {
		return false, nil
	}
	ahora := time.Now()
	f.token.RevocadoEn = &ahora
	nuevo.ID = f.token.ID + 1
	f.token = *nuevo
	f.rotaciones++
	return true, nil
}

func (f *fakeRefreshStore) Revocar(context.Context, int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.revocados++
	ahora := time.Now()
	f.token.RevocadoEn = &ahora
	return nil
}

func (f *fakeRefreshStore) RevocarTodosPorUsuario(context.Context, int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.revocados++
	ahora := time.Now()
	f.token.RevocadoEn = &ahora
	return nil
}

func TestRefreshConcurrenteSoloRotaUnaVez(t *testing.T) {
	cfg := AuthConfig{
		JWTSecret:  "secreto-de-prueba-con-mas-de-32-caracteres-para-jwt",
		JWTIssuer:  "medilist",
		AccessTTL:  time.Minute,
		RefreshTTL: time.Hour,
		BcryptCost: 10,
	}
	usuarios := &fakeUsuarioRefresh{
		usuario: entities.Usuario{
			ID:           5,
			Username:     "ana",
			Activo:       true,
			TokenVersion: 0,
		},
	}
	refresh := &fakeRefreshStore{
		token: entities.RefreshToken{
			ID:           10,
			UsuarioID:    5,
			TokenHash:    hashRefreshToken("token-compartido"),
			TokenVersion: 0,
			ExpiraEn:     time.Now().Add(time.Hour),
		},
	}
	auditoria := &fakeAuditoriaAuth{}
	svc := NewAuthService(cfg, usuarios, refresh, auditoria)

	inicio := make(chan struct{})
	var wg sync.WaitGroup
	var mu sync.Mutex
	exitos := 0
	noAutorizados := 0

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-inicio
			_, err := svc.Refresh(context.Background(), "token-compartido", "127.0.0.1", "test")
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				exitos++
			case errors.Is(err, entities.ErrUnauthorized):
				noAutorizados++
			default:
				t.Errorf("error inesperado: %v", err)
			}
		}()
	}
	close(inicio)
	wg.Wait()

	if exitos != 1 {
		t.Fatalf("se esperaba exactamente 1 refresh exitoso, obtenidos %d", exitos)
	}
	if noAutorizados != 1 {
		t.Fatalf("se esperaba 1 refresh rechazado, obtenidos %d", noAutorizados)
	}
	if refresh.rotaciones != 1 {
		t.Fatalf("se esperaba 1 rotación, obtenidas %d", refresh.rotaciones)
	}
	if usuarios.revocaciones == 0 {
		t.Fatal("la reutilización debe revocar las sesiones del usuario")
	}
}
