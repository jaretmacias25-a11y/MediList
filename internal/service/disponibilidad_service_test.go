package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"medilist/internal/service/entities"
)

type fakeDisponibilidadStore struct {
	DisponibilidadStore
	horarios []entities.HorarioProfesional
	bloqueos []entities.BloqueoAgenda
	citas    []entities.IntervaloOcupado
}

func (f *fakeDisponibilidadStore) ListarHorariosVigentes(
	context.Context, int64, time.Time, time.Time,
) ([]entities.HorarioProfesional, error) {
	return f.horarios, nil
}

func (f *fakeDisponibilidadStore) ListarBloqueosRango(
	context.Context, int64, time.Time, time.Time,
) ([]entities.BloqueoAgenda, error) {
	return f.bloqueos, nil
}

func (f *fakeDisponibilidadStore) ListarCitasActivasRango(
	context.Context, int64, time.Time, time.Time,
) ([]entities.IntervaloOcupado, error) {
	return f.citas, nil
}

type fakeMedicoStoreDisponibilidad struct {
	MedicoStore
	medico *entities.Medico
}

func (f *fakeMedicoStoreDisponibilidad) ObtenerPorID(
	_ context.Context, id int64,
) (*entities.Medico, error) {
	if f.medico == nil || f.medico.ID != id {
		return nil, entities.ErrNotFound
	}
	return f.medico, nil
}

func ubicacionGuayaquil(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("America/Guayaquil")
	if err != nil {
		t.Fatalf("no se pudo cargar America/Guayaquil: %v", err)
	}
	return loc
}

func medicoDisponible() *entities.Medico {
	return &entities.Medico{
		ID:              7,
		EspecialidadID:  1,
		DuracionCitaMin: 30,
		Activo:          true,
		AceptaPortal:    true,
		Especialidad: entities.Especialidad{
			ID:     1,
			Codigo: "MED-GEN",
			Nombre: "Medicina General",
			Activa: true,
			Orden:  1,
		},
	}
}

func horarioLunes(desde, hasta time.Time) entities.HorarioProfesional {
	return entities.HorarioProfesional{
		ID:              10,
		MedicoID:        7,
		DiaSemana:       diaSemanaISO(desde),
		HoraInicio:      "09:00",
		HoraFin:         "10:00",
		DuracionSlotMin: 30,
		VigenteDesde:    time.Date(desde.Year(), desde.Month(), desde.Day(), 0, 0, 0, 0, time.UTC),
		VigenteHasta:    &hasta,
		Activo:          true,
		Version:         1,
	}
}

func nuevoServicioDisponibilidadPrueba(
	medico *entities.Medico,
	store *fakeDisponibilidadStore,
	ahora time.Time,
	minNotice time.Duration,
	maxDias int,
) *DisponibilidadService {
	return NewDisponibilidadService(
		&fakeMedicoStoreDisponibilidad{medico: medico},
		store,
		DisponibilidadConfig{
			Location:       ahora.Location(),
			MinNotice:      minNotice,
			MaxAdvanceDays: maxDias,
			Now:            func() time.Time { return ahora },
		},
	)
}

func TestDisponibilidadConvierteHoraLocalAUTC(t *testing.T) {
	loc := ubicacionGuayaquil(t)
	fecha := time.Date(2026, 11, 2, 0, 0, 0, 0, loc)
	ahora := time.Date(2026, 11, 2, 8, 0, 0, 0, loc)
	store := &fakeDisponibilidadStore{
		horarios: []entities.HorarioProfesional{
			horarioLunes(fecha, fecha.AddDate(0, 0, 30)),
		},
	}
	svc := nuevoServicioDisponibilidadPrueba(medicoDisponible(), store, ahora, 0, 90)

	disponibilidad, err := svc.Consultar(context.Background(), 7, "2026-11-02", "2026-11-02")
	if err != nil {
		t.Fatalf("consultar disponibilidad: %v", err)
	}
	if len(disponibilidad.Slots) != 2 {
		t.Fatalf("se esperaban 2 slots, obtenidos %d", len(disponibilidad.Slots))
	}
	esperado := time.Date(2026, 11, 2, 14, 0, 0, 0, time.UTC)
	if !disponibilidad.Slots[0].Inicio.Equal(esperado) {
		t.Fatalf("inicio esperado %s, obtenido %s", esperado, disponibilidad.Slots[0].Inicio)
	}
	if disponibilidad.ZonaHoraria != "America/Guayaquil" {
		t.Fatalf("zona horaria inesperada: %s", disponibilidad.ZonaHoraria)
	}
}

func TestDisponibilidadRestaBloqueosYCitasActivas(t *testing.T) {
	loc := ubicacionGuayaquil(t)
	fecha := time.Date(2026, 11, 2, 0, 0, 0, 0, loc)
	ahora := time.Date(2026, 11, 2, 8, 0, 0, 0, loc)
	store := &fakeDisponibilidadStore{
		horarios: []entities.HorarioProfesional{
			horarioLunes(fecha, fecha.AddDate(0, 0, 30)),
		},
		bloqueos: []entities.BloqueoAgenda{
			{
				Inicio: time.Date(2026, 11, 2, 14, 30, 0, 0, time.UTC),
				Fin:    time.Date(2026, 11, 2, 15, 0, 0, 0, time.UTC),
			},
		},
		citas: []entities.IntervaloOcupado{
			{
				Inicio: time.Date(2026, 11, 2, 14, 0, 0, 0, time.UTC),
				Fin:    time.Date(2026, 11, 2, 14, 30, 0, 0, time.UTC),
			},
		},
	}
	svc := nuevoServicioDisponibilidadPrueba(medicoDisponible(), store, ahora, 0, 90)

	disponibilidad, err := svc.Consultar(context.Background(), 7, "2026-11-02", "2026-11-02")
	if err != nil {
		t.Fatalf("consultar disponibilidad: %v", err)
	}
	if len(disponibilidad.Slots) != 0 {
		t.Fatalf("todos los slots deben estar ocupados, obtenidos %d", len(disponibilidad.Slots))
	}
}

func TestDisponibilidadRespetaAntelacionMinima(t *testing.T) {
	loc := ubicacionGuayaquil(t)
	fecha := time.Date(2026, 11, 2, 0, 0, 0, 0, loc)
	ahora := time.Date(2026, 11, 2, 8, 45, 0, 0, loc)
	horario := horarioLunes(fecha, fecha.AddDate(0, 0, 30))
	horario.HoraFin = "12:00"
	store := &fakeDisponibilidadStore{
		horarios: []entities.HorarioProfesional{horario},
	}
	svc := nuevoServicioDisponibilidadPrueba(medicoDisponible(), store, ahora, 2*time.Hour, 90)

	disponibilidad, err := svc.Consultar(context.Background(), 7, "2026-11-02", "2026-11-02")
	if err != nil {
		t.Fatalf("consultar disponibilidad: %v", err)
	}
	if len(disponibilidad.Slots) != 2 {
		t.Fatalf("se esperaban 2 slots después de la antelación, obtenidos %d", len(disponibilidad.Slots))
	}
	esperado := time.Date(2026, 11, 2, 16, 0, 0, 0, time.UTC) // 11:00 en Guayaquil
	if !disponibilidad.Slots[0].Inicio.Equal(esperado) {
		t.Fatalf("primer slot esperado %s, obtenido %s", esperado, disponibilidad.Slots[0].Inicio)
	}
}

func TestDisponibilidadRechazaRangoFueraDelHorizonte(t *testing.T) {
	loc := ubicacionGuayaquil(t)
	ahora := time.Date(2026, 11, 2, 8, 0, 0, 0, loc)
	store := &fakeDisponibilidadStore{}
	svc := nuevoServicioDisponibilidadPrueba(medicoDisponible(), store, ahora, 0, 3)

	if _, err := svc.Consultar(context.Background(), 7, "2026-11-05", "2026-11-05"); err == nil {
		t.Fatal("un rango fuera del horizonte debe rechazarse")
	}
	if _, err := svc.Consultar(context.Background(), 7, "2026-11-01", "2026-11-01"); err == nil {
		t.Fatal("un rango pasado debe rechazarse")
	}
}

func TestDisponibilidadRespetaVigenciaDelHorario(t *testing.T) {
	loc := ubicacionGuayaquil(t)
	fecha := time.Date(2026, 11, 2, 0, 0, 0, 0, loc)
	ahora := time.Date(2026, 11, 2, 8, 0, 0, 0, loc)
	horario := horarioLunes(fecha, fecha)
	horario.VigenteHasta = &fecha
	store := &fakeDisponibilidadStore{
		horarios: []entities.HorarioProfesional{horario},
	}
	svc := nuevoServicioDisponibilidadPrueba(medicoDisponible(), store, ahora, 0, 90)

	// El horario termina el mismo día; el lunes siguiente ya no debe generar slots.
	disponibilidad, err := svc.Consultar(context.Background(), 7, "2026-11-09", "2026-11-09")
	if err != nil {
		t.Fatalf("consultar disponibilidad: %v", err)
	}
	if len(disponibilidad.Slots) != 0 {
		t.Fatalf("no debe haber slots fuera de vigencia, obtenidos %d", len(disponibilidad.Slots))
	}
}

func TestValidarSlotExigeUnSlotGenerado(t *testing.T) {
	loc := ubicacionGuayaquil(t)
	fecha := time.Date(2026, 11, 2, 0, 0, 0, 0, loc)
	ahora := time.Date(2026, 11, 2, 8, 0, 0, 0, loc)
	store := &fakeDisponibilidadStore{
		horarios: []entities.HorarioProfesional{
			horarioLunes(fecha, fecha.AddDate(0, 0, 30)),
		},
	}
	svc := nuevoServicioDisponibilidadPrueba(medicoDisponible(), store, ahora, 0, 90)

	inicio := time.Date(2026, 11, 2, 14, 0, 0, 0, time.UTC)
	slot, err := svc.ValidarSlot(context.Background(), 7, inicio, 30)
	if err != nil {
		t.Fatalf("validar slot: %v", err)
	}
	if !slot.Inicio.Equal(inicio) {
		t.Fatalf("slot inesperado: %s", slot.Inicio)
	}

	if _, err := svc.ValidarSlot(context.Background(), 7, inicio.Add(15*time.Minute), 30); !errors.Is(err, entities.ErrConflict) {
		t.Fatalf("un inicio no alineado debe rechazarse, obtenido %v", err)
	}
	if _, err := svc.ValidarSlot(context.Background(), 7, inicio, 45); !errors.Is(err, entities.ErrConflict) {
		t.Fatalf("una duración distinta debe rechazarse, obtenido %v", err)
	}

	store.bloqueos = []entities.BloqueoAgenda{{
		Inicio: inicio,
		Fin:    inicio.Add(30 * time.Minute),
	}}
	if _, err := svc.ValidarSlot(context.Background(), 7, inicio, 30); !errors.Is(err, entities.ErrConflict) {
		t.Fatalf("un slot bloqueado debe rechazarse, obtenido %v", err)
	}
}

func TestDisponibilidadNoExponeMedicoInactivoODePortalCerrado(t *testing.T) {
	loc := ubicacionGuayaquil(t)
	ahora := time.Date(2026, 11, 2, 8, 0, 0, 0, loc)
	casos := []struct {
		nombre string
		cambio func(*entities.Medico)
	}{
		{"inactivo", func(m *entities.Medico) { m.Activo = false }},
		{"sin portal", func(m *entities.Medico) { m.AceptaPortal = false }},
		{"especialidad inactiva", func(m *entities.Medico) { m.Especialidad.Activa = false }},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			medico := medicoDisponible()
			caso.cambio(medico)
			svc := nuevoServicioDisponibilidadPrueba(
				medico, &fakeDisponibilidadStore{}, ahora, 0, 90,
			)
			_, err := svc.Consultar(context.Background(), 7, "2026-11-02", "2026-11-02")
			if !errors.Is(err, entities.ErrNotFound) {
				t.Fatalf("se esperaba ErrNotFound, obtenido %v", err)
			}
		})
	}
}
