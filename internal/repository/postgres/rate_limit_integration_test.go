package postgres

import (
	"context"
	"testing"
	"time"
)

func TestRateLimitStoreComparteVentana(t *testing.T) {
	pool := poolIntegracion(t)
	store := NewRateLimitStore(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	clave := "test-rate-limit-" + time.Now().Format("150405.000000000")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _ = pool.Exec(ctx, `DELETE FROM rate_limit_buckets WHERE clave = $1`, clave)
	})

	for i := 0; i < 2; i++ {
		permitido, _, err := store.Permitir(ctx, clave, 2, time.Minute)
		if err != nil || !permitido {
			t.Fatalf("solicitud %d debía permitirse: %v %v", i+1, permitido, err)
		}
	}
	permitido, espera, err := store.Permitir(ctx, clave, 2, time.Minute)
	if err != nil {
		t.Fatalf("tercera solicitud: %v", err)
	}
	if permitido || espera <= 0 {
		t.Fatalf("la tercera solicitud debía rechazarse con espera: permitido=%v espera=%s", permitido, espera)
	}
}
