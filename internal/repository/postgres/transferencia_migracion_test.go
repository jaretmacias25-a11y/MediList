package postgres

import (
	"context"
	"testing"
	"time"
)

func TestMigracionTransferenciaInterna(t *testing.T) {
	pool := poolIntegracion(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var columna string
	if err := pool.QueryRow(ctx, `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name = 'pacientes'
		  AND column_name = 'medico_responsable_id'`).Scan(&columna); err != nil {
		t.Fatalf("pacientes.medico_responsable_id no existe: %v", err)
	}

	var tabla string
	if err := pool.QueryRow(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
		  AND table_name = 'transferencias_paciente'`).Scan(&tabla); err != nil {
		t.Fatalf("transferencias_paciente no existe: %v", err)
	}

	var permiso int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM permisos
		WHERE codigo = 'pacientes:transferir'`).Scan(&permiso); err != nil {
		t.Fatalf("no se pudo consultar el permiso: %v", err)
	}
	if permiso != 1 {
		t.Fatalf("se esperaba el permiso pacientes:transferir, encontrados %d", permiso)
	}

	var roles int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM rol_permisos rp
		JOIN roles r ON r.id = rp.rol_id
		JOIN permisos p ON p.id = rp.permiso_id
		WHERE p.codigo = 'pacientes:transferir'
		  AND r.codigo IN ('admin', 'recepcion')`).Scan(&roles); err != nil {
		t.Fatalf("no se pudo consultar la asignación del permiso: %v", err)
	}
	if roles != 2 {
		t.Fatalf("el permiso debe estar asignado a admin y recepción; asignaciones: %d", roles)
	}
}
