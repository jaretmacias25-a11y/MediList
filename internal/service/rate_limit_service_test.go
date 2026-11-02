package service

import (
	"context"
	"testing"
	"time"
)

type fakeRateLimitStore struct {
	RateLimitStore
	clave    string
	limite   int
	ventana  time.Duration
	permitir bool
}

func (f *fakeRateLimitStore) Permitir(
	_ context.Context,
	clave string,
	limite int,
	ventana time.Duration,
) (bool, time.Duration, error) {
	f.clave = clave
	f.limite = limite
	f.ventana = ventana
	return f.permitir, 0, nil
}

func TestRateLimitServiceDelegaEnStore(t *testing.T) {
	store := &fakeRateLimitStore{permitir: true}
	svc := NewRateLimitService(store)
	permitido, _, err := svc.Permitir(context.Background(), "login:1.2.3.4", 5, time.Minute)
	if err != nil || !permitido {
		t.Fatalf("resultado inesperado: %v %v", permitido, err)
	}
	if store.clave != "login:1.2.3.4" || store.limite != 5 || store.ventana != time.Minute {
		t.Fatalf("delegación inesperada: %+v", store)
	}
}
