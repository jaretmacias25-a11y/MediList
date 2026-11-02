package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"medilist/internal/service/entities"
)

func TestCitaStorePersisteMetadataDePortalYCancela(t *testing.T) {
	pool := poolIntegracion(t)
	medicoID := crearMedicoPrueba(t, pool)
	pacienteID := crearPacientePruebaDerivacion(t, pool)
	store := NewCitaStore(pool)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var usuarioID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM usuarios ORDER BY id LIMIT 1`).Scan(&usuarioID); err != nil {
		t.Fatalf("no se encontró un usuario para creada_por: %v", err)
	}

	inicio := time.Date(2026, 11, 9, 14, 0, 0, 0, time.UTC)
	cita := &entities.Cita{
		PacienteID:         pacienteID,
		MedicoID:           medicoID,
		FechaHoraInicio:    inicio,
		DuracionMin:        30,
		Tipo:               entities.TipoCitaPrimeraVez,
		Motivo:             "Prueba de portal",
		Estado:             entities.EstadoCitaProgramada,
		Canal:              entities.CanalCitaPortalPaciente,
		CreadaPorUsuarioID: &usuarioID,
	}
	if err := store.Crear(ctx, cita); err != nil {
		t.Fatalf("crear cita de portal: %v", err)
	}
	if cita.ID == 0 || cita.Codigo == "" {
		t.Fatalf("cita creada sin identidad: %+v", cita)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _ = pool.Exec(ctx, `DELETE FROM citas WHERE id = $1`, cita.ID)
	})

	obtenida, err := store.ObtenerPorID(ctx, cita.ID)
	if err != nil {
		t.Fatalf("obtener cita: %v", err)
	}
	if obtenida.Canal != entities.CanalCitaPortalPaciente {
		t.Fatalf("canal inesperado: %s", obtenida.Canal)
	}
	if obtenida.CreadaPorUsuarioID == nil || *obtenida.CreadaPorUsuarioID != usuarioID {
		t.Fatalf("creada_por inesperado: %v", obtenida.CreadaPorUsuarioID)
	}
	if obtenida.DerivacionID != nil {
		t.Fatalf("una cita directa no debe tener derivación: %v", obtenida.DerivacionID)
	}

	versionAnterior := obtenida.Version
	obtenida.MotivoCancelacion = "cancelacion_paciente"
	if err := store.Cancelar(ctx, obtenida); err != nil {
		t.Fatalf("cancelar cita: %v", err)
	}
	if obtenida.Version != versionAnterior+1 {
		t.Fatalf("versión esperada %d, obtenida %d", versionAnterior+1, obtenida.Version)
	}
	cancelada, err := store.ObtenerPorID(ctx, cita.ID)
	if err != nil {
		t.Fatalf("obtener cita cancelada: %v", err)
	}
	if cancelada.Estado != entities.EstadoCitaCancelada {
		t.Fatalf("estado inesperado: %s", cancelada.Estado)
	}
	if cancelada.MotivoCancelacion != "cancelacion_paciente" {
		t.Fatalf("motivo de cancelación inesperado: %s", cancelada.MotivoCancelacion)
	}
}

func TestCitaStoreConcurrenteSoloPermiteUnSlot(t *testing.T) {
	pool := poolIntegracion(t)
	medicoID := crearMedicoPrueba(t, pool)
	pacienteA := crearPacientePruebaDerivacion(t, pool)
	pacienteB := crearPacientePruebaDerivacion(t, pool)
	store := NewCitaStore(pool)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _ = pool.Exec(ctx, `DELETE FROM citas WHERE medico_id = $1`, medicoID)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	inicio := time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC)
	crear := func(pacienteID int64) error {
		cita := &entities.Cita{
			PacienteID:      pacienteID,
			MedicoID:        medicoID,
			FechaHoraInicio: inicio,
			DuracionMin:     30,
			Tipo:            entities.TipoCitaPrimeraVez,
			Estado:          entities.EstadoCitaProgramada,
			Canal:           entities.CanalCitaPortalPaciente,
		}
		return store.Crear(ctx, cita)
	}

	resultados := make(chan error, 2)
	go func() { resultados <- crear(pacienteA) }()
	go func() { resultados <- crear(pacienteB) }()

	var exitos, conflictos int
	for i := 0; i < 2; i++ {
		err := <-resultados
		switch {
		case err == nil:
			exitos++
		case errors.Is(err, entities.ErrConflict):
			conflictos++
		default:
			t.Fatalf("error inesperado en la carrera: %v", err)
		}
	}
	if exitos != 1 || conflictos != 1 {
		t.Fatalf("se esperaba 1 éxito y 1 conflicto, obtenidos %d/%d", exitos, conflictos)
	}

	var cantidad int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM citas WHERE medico_id = $1`, medicoID,
	).Scan(&cantidad); err != nil {
		t.Fatalf("contar citas: %v", err)
	}
	if cantidad != 1 {
		t.Fatalf("se esperaba una sola cita, obtenidas %d", cantidad)
	}
}
