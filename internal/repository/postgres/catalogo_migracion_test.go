package postgres

import (
	"context"
	"testing"
	"time"
)

func TestMigracionPediatriaAccesoDirecto(t *testing.T) {
	pool := poolIntegracion(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var pediatriaDirecta, otorrinoConDerivacion bool
	if err := pool.QueryRow(ctx, `
		SELECT requiere_derivacion = FALSE
		FROM especialidades
		WHERE codigo = 'PED'`).Scan(&pediatriaDirecta); err != nil {
		t.Fatalf("no se pudo consultar Pediatría: %v", err)
	}
	if !pediatriaDirecta {
		t.Fatal("Pediatría debe ser de acceso directo según la decisión de Fase 0")
	}

	if err := pool.QueryRow(ctx, `
		SELECT requiere_derivacion = TRUE
		FROM especialidades
		WHERE codigo = 'OTORRINO'`).Scan(&otorrinoConDerivacion); err != nil {
		t.Fatalf("no se pudo consultar Otorrinolaringología: %v", err)
	}
	if !otorrinoConDerivacion {
		t.Fatal("Otorrinolaringología debe seguir exigiendo derivación")
	}
}
