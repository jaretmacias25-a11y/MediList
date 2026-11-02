package service

import (
	"context"
	"time"
)

// RateLimitStore implementa una ventana fija atómica sobre PostgreSQL.
type RateLimitStore interface {
	Permitir(
		ctx context.Context,
		clave string,
		limite int,
		ventana time.Duration,
	) (bool, time.Duration, error)
}

// RateLimitService comparte el límite entre todas las réplicas de la API.
type RateLimitService struct {
	store RateLimitStore
}

// NewRateLimitService construye el servicio distribuido.
func NewRateLimitService(store RateLimitStore) *RateLimitService {
	return &RateLimitService{store: store}
}

// Permitir indica si la clave puede continuar y cuánto falta para reiniciar.
func (s *RateLimitService) Permitir(
	ctx context.Context,
	clave string,
	limite int,
	ventana time.Duration,
) (bool, time.Duration, error) {
	if limite <= 0 {
		return true, 0, nil
	}
	if ventana <= 0 {
		ventana = time.Minute
	}
	return s.store.Permitir(ctx, clave, limite, ventana)
}
