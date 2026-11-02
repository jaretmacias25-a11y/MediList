package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RateLimitStore implementa ventanas fijas distribuidas sobre PostgreSQL.
type RateLimitStore struct {
	db *pgxpool.Pool
}

// NewRateLimitStore construye el almacén de rate limiting.
func NewRateLimitStore(db *pgxpool.Pool) *RateLimitStore {
	return &RateLimitStore{db: db}
}

func (s *RateLimitStore) Permitir(
	ctx context.Context,
	clave string,
	limite int,
	ventana time.Duration,
) (bool, time.Duration, error) {
	segundos := int(ventana / time.Second)
	if segundos < 1 {
		segundos = 1
	}
	var contador int
	var inicio time.Time
	err := s.db.QueryRow(ctx, `
		INSERT INTO rate_limit_buckets (clave, contador, ventana_inicio)
		VALUES ($1, 1, now())
		ON CONFLICT (clave) DO UPDATE
		SET contador = CASE
				WHEN rate_limit_buckets.ventana_inicio + ($2 * interval '1 second') <= now()
				THEN 1
				ELSE rate_limit_buckets.contador + 1
			END,
			ventana_inicio = CASE
				WHEN rate_limit_buckets.ventana_inicio + ($2 * interval '1 second') <= now()
				THEN now()
				ELSE rate_limit_buckets.ventana_inicio
			END,
			updated_at = now()
		RETURNING contador, ventana_inicio`,
		clave, segundos,
	).Scan(&contador, &inicio)
	if err != nil {
		return false, 0, mapError(err)
	}
	if contador > limite {
		espera := time.Until(inicio.Add(ventana))
		if espera < 0 {
			espera = 0
		}
		return false, espera, nil
	}
	return true, 0, nil
}
