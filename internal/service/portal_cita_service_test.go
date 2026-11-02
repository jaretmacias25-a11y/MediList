package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"medilist/internal/service/entities"
)

type fakePortalCitaStore struct {
	CitaStore
	citas       map[int64]*entities.Cita
	creada      *entities.Cita
	actualizada *entities.Cita
	cancelada   *entities.Cita
	filtro      entities.CitaFiltro
	listado     []entities.Cita
}

func (f *fakePortalCitaStore) Crear(_ context.Context, c *entities.Cita) error {
	if f.citas == nil {
		f.citas = make(map[int64]*entities.Cita)
	}
	c.ID = 1
	c.Codigo = "CIT-000001"
	c.Version = 1
	f.creada = c
	f.citas[c.ID] = c
	return nil
}

func (f *fakePortalCitaStore) ObtenerPorID(_ context.Context, id int64) (*entities.Cita, error) {
	cita, ok := f.citas[id]
	if !ok {
		return nil, entities.ErrNotFound
	}
	return cita, nil
}

func (f *fakePortalCitaStore) Listar(
	_ context.Context,
	filtro entities.CitaFiltro,
) ([]entities.Cita, error) {
	f.filtro = filtro
	return f.listado, nil
}

func (f *fakePortalCitaStore) Actualizar(_ context.Context, c *entities.Cita) error {
	c.Version++
	f.actualizada = c
	f.citas[c.ID] = c
	return nil
}

func (f *fakePortalCitaStore) Cancelar(_ context.Context, c *entities.Cita) error {
	c.Version++
	c.Estado = entities.EstadoCitaCancelada
	f.cancelada = c
	f.citas[c.ID] = c
	return nil
}

type fakePacienteStorePortalCita struct {
	PacienteStore
	paciente *entities.Paciente
}

func (f *fakePacienteStorePortalCita) ObtenerPorID(
	_ context.Context, id int64,
) (*entities.Paciente, error) {
	if f.paciente == nil || f.paciente.ID != id {
		return nil, entities.ErrNotFound
	}
	return f.paciente, nil
}

type fakeMedicoStorePortalCita struct {
	MedicoStore
	medico *entities.Medico
}

func (f *fakeMedicoStorePortalCita) ObtenerPorID(
	_ context.Context, id int64,
) (*entities.Medico, error) {
	if f.medico == nil || f.medico.ID != id {
		return nil, entities.ErrNotFound
	}
	return f.medico, nil
}

type fakeSlotValidatorPortal struct {
	slot      *entities.SlotDisponible
	err       error
	medicoID  int64
	inicio    time.Time
	duracion  int
	consultas int
}

func (f *fakeSlotValidatorPortal) ValidarSlot(
	_ context.Context, medicoID int64, inicio time.Time, duracionMin int,
) (*entities.SlotDisponible, error) {
	f.consultas++
	f.medicoID = medicoID
	f.inicio = inicio
	f.duracion = duracionMin
	if f.err != nil {
		return nil, f.err
	}
	return f.slot, nil
}

func pacientePortalCita(usuarioID, pacienteID int64) *entities.Paciente {
	return &entities.Paciente{
		ID:        pacienteID,
		UsuarioID: &usuarioID,
		Activo:    true,
		Persona: entities.Persona{
			Nombres: "Paciente",
			Email:   "paciente@example.test",
		},
	}
}

func medicoPortalCita() *entities.Medico {
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
		},
	}
}

func actorPortalCita(usuarioID, pacienteID int64) *Actor {
	return &Actor{UsuarioID: usuarioID, PacienteID: &pacienteID}
}

func versionPortalCita(version int64) *int64 {
	return &version
}

func slotPortalCita(inicio time.Time) *entities.SlotDisponible {
	return &entities.SlotDisponible{
		Inicio:      inicio,
		Fin:         inicio.Add(30 * time.Minute),
		DuracionMin: 30,
	}
}

func nuevoServicioPortalCitaPrueba(
	citas *fakePortalCitaStore,
	paciente *entities.Paciente,
	medico *entities.Medico,
	slots *fakeSlotValidatorPortal,
	ahora time.Time,
	autoConfirm bool,
) *PortalCitaService {
	return NewPortalCitaService(
		citas,
		&fakePacienteStorePortalCita{paciente: paciente},
		&fakeMedicoStorePortalCita{medico: medico},
		slots,
		PortalCitaConfig{
			CancelCutoff: 24 * time.Hour,
			AutoConfirm:  autoConfirm,
			Now:          func() time.Time { return ahora },
		},
	)
}

func TestPortalCitaCrearFuerzaPacienteCanalYDuracion(t *testing.T) {
	ahora := time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)
	inicio := time.Date(2026, 11, 9, 14, 0, 0, 0, time.UTC)
	citas := &fakePortalCitaStore{}
	slots := &fakeSlotValidatorPortal{slot: slotPortalCita(inicio)}
	svc := nuevoServicioPortalCitaPrueba(
		citas, pacientePortalCita(10, 20), medicoPortalCita(), slots, ahora, true,
	)

	cita, err := svc.Crear(context.Background(), actorPortalCita(10, 20), PortalCitaInput{
		MedicoID:        7,
		FechaHoraInicio: inicio,
		Motivo:          "Dolor lumbar",
	})
	if err != nil {
		t.Fatalf("crear cita de portal: %v", err)
	}
	if cita.PacienteID != 20 || citas.creada.PacienteID != 20 {
		t.Fatalf("el paciente debe salir del actor, obtenido %d", citas.creada.PacienteID)
	}
	if citas.creada.Canal != entities.CanalCitaPortalPaciente {
		t.Fatalf("canal inesperado: %s", citas.creada.Canal)
	}
	if citas.creada.CreadaPorUsuarioID == nil || *citas.creada.CreadaPorUsuarioID != 10 {
		t.Fatalf("creada_por_usuario_id inesperado: %v", citas.creada.CreadaPorUsuarioID)
	}
	if citas.creada.Estado != entities.EstadoCitaConfirmada {
		t.Fatalf("estado inesperado: %s", citas.creada.Estado)
	}
	if citas.creada.DuracionMin != 30 {
		t.Fatalf("duración inesperada: %d", citas.creada.DuracionMin)
	}
	if citas.creada.Tipo != entities.TipoCitaPrimeraVez {
		t.Fatalf("tipo inesperado: %s", citas.creada.Tipo)
	}
	if slots.medicoID != 7 || !slots.inicio.Equal(inicio) {
		t.Fatalf("validación de slot inesperada: medico=%d inicio=%s", slots.medicoID, slots.inicio)
	}
}

func TestPortalCitaCrearEscribeConfirmacionBasica(t *testing.T) {
	ahora := time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)
	inicio := time.Date(2026, 11, 9, 14, 0, 0, 0, time.UTC)
	citas := &fakePortalCitaStore{}
	emails := &fakeEmailSenderCuenta{}
	svc := NewPortalCitaService(
		citas,
		&fakePacienteStorePortalCita{paciente: pacientePortalCita(10, 20)},
		&fakeMedicoStorePortalCita{medico: medicoPortalCita()},
		&fakeSlotValidatorPortal{slot: slotPortalCita(inicio)},
		PortalCitaConfig{Now: func() time.Time { return ahora }, AutoConfirm: true},
		emails,
	)

	_, err := svc.Crear(context.Background(), actorPortalCita(10, 20), PortalCitaInput{
		MedicoID:        7,
		FechaHoraInicio: inicio,
	})
	if err != nil {
		t.Fatalf("crear cita: %v", err)
	}
	if len(emails.enviados) != 1 || emails.enviados[0].tipo != "cita" {
		t.Fatalf("se esperaba una confirmación de cita, obtenidos %+v", emails.enviados)
	}
	if emails.enviados[0].destino != "paciente@example.test" {
		t.Fatalf("destino de confirmación inesperado: %s", emails.enviados[0].destino)
	}
}

func TestPortalCitaCrearRechazaEspecialidadConDerivacion(t *testing.T) {
	ahora := time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)
	inicio := time.Date(2026, 11, 9, 14, 0, 0, 0, time.UTC)
	medico := medicoPortalCita()
	medico.Especialidad.RequiereDerivacion = true
	citas := &fakePortalCitaStore{}
	svc := nuevoServicioPortalCitaPrueba(
		citas, pacientePortalCita(10, 20), medico,
		&fakeSlotValidatorPortal{slot: slotPortalCita(inicio)}, ahora, true,
	)

	_, err := svc.Crear(context.Background(), actorPortalCita(10, 20), PortalCitaInput{
		MedicoID:        7,
		FechaHoraInicio: inicio,
	})
	if !errors.Is(err, entities.ErrRequiereDerivacion) {
		t.Fatalf("se esperaba ErrRequiereDerivacion, obtenido %v", err)
	}
	if citas.creada != nil {
		t.Fatal("no se debe crear una cita de especialista sin derivación")
	}
}

func TestPortalCitaCrearRechazaMedicoSinPortal(t *testing.T) {
	ahora := time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)
	inicio := time.Date(2026, 11, 9, 14, 0, 0, 0, time.UTC)
	medico := medicoPortalCita()
	medico.AceptaPortal = false
	citas := &fakePortalCitaStore{}
	svc := nuevoServicioPortalCitaPrueba(
		citas, pacientePortalCita(10, 20), medico,
		&fakeSlotValidatorPortal{slot: slotPortalCita(inicio)}, ahora, true,
	)

	_, err := svc.Crear(context.Background(), actorPortalCita(10, 20), PortalCitaInput{
		MedicoID:        7,
		FechaHoraInicio: inicio,
	})
	if !errors.Is(err, entities.ErrNotFound) {
		t.Fatalf("se esperaba ErrNotFound, obtenido %v", err)
	}
}

func TestPortalCitaListarFuerzaPacienteDelActor(t *testing.T) {
	citas := &fakePortalCitaStore{listado: []entities.Cita{{ID: 1, PacienteID: 20}}}
	svc := nuevoServicioPortalCitaPrueba(
		citas, pacientePortalCita(10, 20), medicoPortalCita(),
		&fakeSlotValidatorPortal{}, time.Now(), true,
	)

	_, err := svc.Listar(context.Background(), actorPortalCita(10, 20), entities.CitaFiltro{
		PacienteID: 999,
	})
	if err != nil {
		t.Fatalf("listar citas: %v", err)
	}
	if citas.filtro.PacienteID != 20 {
		t.Fatalf("el filtro debe forzar el paciente del actor, obtenido %d", citas.filtro.PacienteID)
	}
}

func TestPortalCitaObtenerRechazaCitaAjena(t *testing.T) {
	citas := &fakePortalCitaStore{
		citas: map[int64]*entities.Cita{
			5: {ID: 5, PacienteID: 99},
		},
	}
	svc := nuevoServicioPortalCitaPrueba(
		citas, pacientePortalCita(10, 20), medicoPortalCita(),
		&fakeSlotValidatorPortal{}, time.Now(), true,
	)

	_, err := svc.Obtener(context.Background(), actorPortalCita(10, 20), 5)
	if !errors.Is(err, entities.ErrForbidden) {
		t.Fatalf("se esperaba ErrForbidden, obtenido %v", err)
	}
}

func TestPortalCitaReprogramarExigeVersionYConservaMedico(t *testing.T) {
	ahora := time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)
	inicio := time.Date(2026, 11, 10, 15, 0, 0, 0, time.UTC)
	cita := &entities.Cita{
		ID:              5,
		PacienteID:      20,
		MedicoID:        7,
		FechaHoraInicio: time.Date(2026, 11, 9, 14, 0, 0, 0, time.UTC),
		DuracionMin:     30,
		Tipo:            entities.TipoCitaControl,
		Estado:          entities.EstadoCitaProgramada,
		Canal:           entities.CanalCitaPortalPaciente,
		Version:         3,
	}
	citas := &fakePortalCitaStore{citas: map[int64]*entities.Cita{5: cita}}
	slots := &fakeSlotValidatorPortal{slot: slotPortalCita(inicio)}
	svc := nuevoServicioPortalCitaPrueba(
		citas, pacientePortalCita(10, 20), medicoPortalCita(), slots, ahora, true,
	)

	_, err := svc.Reprogramar(context.Background(), actorPortalCita(10, 20), 5,
		PortalCitaReprogramarInput{FechaHoraInicio: inicio, Version: versionPortalCita(2)})
	if !errors.Is(err, entities.ErrConflict) {
		t.Fatalf("se esperaba ErrConflict por versión, obtenido %v", err)
	}

	actualizada, err := svc.Reprogramar(context.Background(), actorPortalCita(10, 20), 5,
		PortalCitaReprogramarInput{FechaHoraInicio: inicio, Version: versionPortalCita(3)})
	if err != nil {
		t.Fatalf("reprogramar: %v", err)
	}
	if actualizada.PacienteID != 20 || actualizada.MedicoID != 7 {
		t.Fatalf("no se deben cambiar paciente/médico: %+v", actualizada)
	}
	if !actualizada.FechaHoraInicio.Equal(inicio) {
		t.Fatalf("fecha no actualizada: %s", actualizada.FechaHoraInicio)
	}
	if actualizada.Version != 4 {
		t.Fatalf("versión esperada 4, obtenida %d", actualizada.Version)
	}
}

func TestPortalCitaCancelarAntesDelCorte(t *testing.T) {
	ahora := time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)
	cita := &entities.Cita{
		ID:              5,
		PacienteID:      20,
		MedicoID:        7,
		FechaHoraInicio: ahora.Add(48 * time.Hour),
		Estado:          entities.EstadoCitaProgramada,
		Canal:           entities.CanalCitaPortalPaciente,
		Version:         2,
	}
	citas := &fakePortalCitaStore{citas: map[int64]*entities.Cita{5: cita}}
	svc := nuevoServicioPortalCitaPrueba(
		citas, pacientePortalCita(10, 20), medicoPortalCita(),
		&fakeSlotValidatorPortal{}, ahora, true,
	)

	cancelada, err := svc.Cancelar(context.Background(), actorPortalCita(10, 20), 5, "")
	if err != nil {
		t.Fatalf("cancelar: %v", err)
	}
	if cancelada.Estado != entities.EstadoCitaCancelada {
		t.Fatalf("estado inesperado: %s", cancelada.Estado)
	}
	if cancelada.MotivoCancelacion != "cancelacion_paciente" {
		t.Fatalf("motivo inesperado: %s", cancelada.MotivoCancelacion)
	}
}

func TestPortalCitaCancelarDespuesDelCorte(t *testing.T) {
	ahora := time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)
	cita := &entities.Cita{
		ID:              5,
		PacienteID:      20,
		MedicoID:        7,
		FechaHoraInicio: ahora.Add(2 * time.Hour),
		Estado:          entities.EstadoCitaProgramada,
		Version:         2,
	}
	citas := &fakePortalCitaStore{citas: map[int64]*entities.Cita{5: cita}}
	svc := nuevoServicioPortalCitaPrueba(
		citas, pacientePortalCita(10, 20), medicoPortalCita(),
		&fakeSlotValidatorPortal{}, ahora, true,
	)

	_, err := svc.Cancelar(context.Background(), actorPortalCita(10, 20), 5, "Tarde")
	if !errors.Is(err, entities.ErrConflict) {
		t.Fatalf("se esperaba ErrConflict por corte, obtenido %v", err)
	}
	if citas.cancelada != nil {
		t.Fatal("no se debe cancelar fuera del plazo")
	}
}
