package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"medilist/internal/service/entities"
)

type fakeMedicoStoreAgenda struct {
	MedicoStore
	medico *entities.Medico
}

func (f *fakeMedicoStoreAgenda) ObtenerPorID(
	_ context.Context, id int64,
) (*entities.Medico, error) {
	if f.medico == nil || f.medico.ID != id {
		return nil, entities.ErrNotFound
	}
	return f.medico, nil
}

type fakeHorarioStoreAgenda struct {
	HorarioStore
	solapa      bool
	horarios    map[int64]*entities.HorarioProfesional
	creado      *entities.HorarioProfesional
	actualizado *entities.HorarioProfesional
	desactivado bool
}

func (f *fakeHorarioStoreAgenda) ListarHorariosPorMedico(
	context.Context, int64,
) ([]entities.HorarioProfesional, error) {
	return nil, nil
}

func (f *fakeHorarioStoreAgenda) ObtenerHorarioPorID(
	_ context.Context, _, id int64,
) (*entities.HorarioProfesional, error) {
	horario, ok := f.horarios[id]
	if !ok {
		return nil, entities.ErrNotFound
	}
	return horario, nil
}

func (f *fakeHorarioStoreAgenda) CrearHorario(
	_ context.Context, h *entities.HorarioProfesional,
) error {
	h.ID = 100
	h.Version = 1
	f.creado = h
	return nil
}

func (f *fakeHorarioStoreAgenda) ActualizarHorario(
	_ context.Context, h *entities.HorarioProfesional,
) error {
	f.actualizado = h
	return nil
}

func (f *fakeHorarioStoreAgenda) DesactivarHorario(context.Context, int64, int64) error {
	f.desactivado = true
	return nil
}

func (f *fakeHorarioStoreAgenda) ExisteSolapamientoHorario(
	context.Context, int64, int, string, string, time.Time, *time.Time, int64,
) (bool, error) {
	return f.solapa, nil
}

type fakeBloqueoStoreAgenda struct {
	BloqueoStore
	solapa      bool
	creado      *entities.BloqueoAgenda
	actualizado *entities.BloqueoAgenda
	eliminado   bool
}

func (f *fakeBloqueoStoreAgenda) ListarBloqueosRango(
	context.Context, int64, time.Time, time.Time,
) ([]entities.BloqueoAgenda, error) {
	return nil, nil
}

func (f *fakeBloqueoStoreAgenda) CrearBloqueo(
	_ context.Context, b *entities.BloqueoAgenda,
) error {
	b.ID = 200
	f.creado = b
	return nil
}

func (f *fakeBloqueoStoreAgenda) ActualizarBloqueo(
	_ context.Context, b *entities.BloqueoAgenda,
) error {
	f.actualizado = b
	return nil
}

func (f *fakeBloqueoStoreAgenda) EliminarBloqueo(context.Context, int64, int64) error {
	f.eliminado = true
	return nil
}

func (f *fakeBloqueoStoreAgenda) ExisteSolapamientoBloqueo(
	context.Context, int64, time.Time, time.Time, int64,
) (bool, error) {
	return f.solapa, nil
}

func nuevoServicioAgendaPrueba(
	medico *entities.Medico,
	horarios *fakeHorarioStoreAgenda,
	bloqueos *fakeBloqueoStoreAgenda,
) *AgendaService {
	loc := time.FixedZone("America/Guayaquil", -5*60*60)
	return NewAgendaService(
		&fakeMedicoStoreAgenda{medico: medico},
		horarios,
		bloqueos,
		AgendaConfig{
			Location: loc,
			Now:      func() time.Time { return time.Date(2026, 11, 2, 8, 0, 0, 0, loc) },
		},
	)
}

func medicoAgendaPrueba() *entities.Medico {
	return &entities.Medico{ID: 7, Activo: true}
}

func textoPuntero(texto string) *string {
	return &texto
}

func TestAgendaCrearHorarioValido(t *testing.T) {
	horarios := &fakeHorarioStoreAgenda{}
	svc := nuevoServicioAgendaPrueba(medicoAgendaPrueba(), horarios, &fakeBloqueoStoreAgenda{})

	horario, err := svc.CrearHorario(context.Background(), 7, HorarioInput{
		DiaSemana:       1,
		HoraInicio:      "09:00",
		HoraFin:         "10:00",
		DuracionSlotMin: 30,
		VigenteDesde:    textoPuntero("2026-11-02"),
	})
	if err != nil {
		t.Fatalf("crear horario: %v", err)
	}
	if horarios.creado == nil {
		t.Fatal("el horario no se persistió")
	}
	if horario.HoraInicio != "09:00" || horario.HoraFin != "10:00" {
		t.Fatalf("horas inesperadas: %s - %s", horario.HoraInicio, horario.HoraFin)
	}
	if !horario.Activo {
		t.Fatal("un horario nuevo debe estar activo por defecto")
	}
}

func TestAgendaCrearHorarioRechazaSolape(t *testing.T) {
	horarios := &fakeHorarioStoreAgenda{solapa: true}
	svc := nuevoServicioAgendaPrueba(medicoAgendaPrueba(), horarios, &fakeBloqueoStoreAgenda{})

	_, err := svc.CrearHorario(context.Background(), 7, HorarioInput{
		DiaSemana:       1,
		HoraInicio:      "09:00",
		HoraFin:         "10:00",
		DuracionSlotMin: 30,
		VigenteDesde:    textoPuntero("2026-11-02"),
	})
	if !errors.Is(err, entities.ErrConflict) {
		t.Fatalf("se esperaba ErrConflict, obtenido %v", err)
	}
	if horarios.creado != nil {
		t.Fatal("no se debe persistir un horario que se solapa")
	}
}

func TestAgendaCrearHorarioRechazaRangoInvalido(t *testing.T) {
	horarios := &fakeHorarioStoreAgenda{}
	svc := nuevoServicioAgendaPrueba(medicoAgendaPrueba(), horarios, &fakeBloqueoStoreAgenda{})

	_, err := svc.CrearHorario(context.Background(), 7, HorarioInput{
		DiaSemana:       1,
		HoraInicio:      "10:00",
		HoraFin:         "09:00",
		DuracionSlotMin: 30,
		VigenteDesde:    textoPuntero("2026-11-02"),
	})
	if !errors.Is(err, entities.ErrValidation) {
		t.Fatalf("se esperaba ErrValidation, obtenido %v", err)
	}
}

func TestAgendaActualizarHorarioExigeVersionVigente(t *testing.T) {
	horarios := &fakeHorarioStoreAgenda{
		horarios: map[int64]*entities.HorarioProfesional{
			10: {
				ID:              10,
				MedicoID:        7,
				DiaSemana:       1,
				HoraInicio:      "09:00",
				HoraFin:         "10:00",
				DuracionSlotMin: 30,
				VigenteDesde:    time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
				Activo:          true,
				Version:         3,
			},
		},
	}
	svc := nuevoServicioAgendaPrueba(medicoAgendaPrueba(), horarios, &fakeBloqueoStoreAgenda{})

	_, err := svc.ActualizarHorario(context.Background(), 7, 10, HorarioInput{
		Version: 2,
	})
	if !errors.Is(err, entities.ErrConflict) {
		t.Fatalf("se esperaba ErrConflict por versión, obtenido %v", err)
	}
	if horarios.actualizado != nil {
		t.Fatal("no se debe actualizar con una versión obsoleta")
	}
}

func TestAgendaActualizarBloqueoValidaSolape(t *testing.T) {
	inicio := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	fin := inicio.Add(2 * time.Hour)
	bloqueos := &fakeBloqueoStoreAgenda{solapa: true}
	svc := nuevoServicioAgendaPrueba(medicoAgendaPrueba(), &fakeHorarioStoreAgenda{}, bloqueos)

	_, err := svc.ActualizarBloqueo(context.Background(), 7, 200, BloqueoInput{
		Inicio: inicio,
		Fin:    fin,
		Motivo: "Reprogramación",
	})
	if !errors.Is(err, entities.ErrConflict) {
		t.Fatalf("se esperaba ErrConflict, obtenido %v", err)
	}
	if bloqueos.actualizado != nil {
		t.Fatal("no se debe actualizar un bloqueo que se solapa")
	}

	bloqueos.solapa = false
	bloqueo, err := svc.ActualizarBloqueo(context.Background(), 7, 200, BloqueoInput{
		Inicio: inicio,
		Fin:    fin,
		Motivo: "Reprogramación",
	})
	if err != nil {
		t.Fatalf("actualizar bloqueo: %v", err)
	}
	if bloqueo.ID != 200 || bloqueo.Motivo != "Reprogramación" {
		t.Fatalf("bloqueo actualizado inesperado: %+v", bloqueo)
	}
}

func TestAgendaCrearBloqueoValidaMotivoYSolape(t *testing.T) {
	inicio := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	fin := inicio.Add(2 * time.Hour)
	bloqueos := &fakeBloqueoStoreAgenda{}
	svc := nuevoServicioAgendaPrueba(medicoAgendaPrueba(), &fakeHorarioStoreAgenda{}, bloqueos)

	if _, err := svc.CrearBloqueo(context.Background(), 7, &Actor{UsuarioID: 9}, BloqueoInput{
		Inicio: inicio,
		Fin:    fin,
	}); !errors.Is(err, entities.ErrValidation) {
		t.Fatalf("se esperaba ErrValidation por motivo vacío, obtenido %v", err)
	}

	bloqueos.solapa = true
	if _, err := svc.CrearBloqueo(context.Background(), 7, &Actor{UsuarioID: 9}, BloqueoInput{
		Inicio: inicio,
		Fin:    fin,
		Motivo: "Capacitación",
	}); !errors.Is(err, entities.ErrConflict) {
		t.Fatalf("se esperaba ErrConflict por solape, obtenido %v", err)
	}

	bloqueos.solapa = false
	bloqueo, err := svc.CrearBloqueo(context.Background(), 7, &Actor{UsuarioID: 9}, BloqueoInput{
		Inicio: inicio,
		Fin:    fin,
		Motivo: "Capacitación",
	})
	if err != nil {
		t.Fatalf("crear bloqueo: %v", err)
	}
	if bloqueo.CreadoPor == nil || *bloqueo.CreadoPor != 9 {
		t.Fatalf("creado_por inesperado: %v", bloqueo.CreadoPor)
	}
}
