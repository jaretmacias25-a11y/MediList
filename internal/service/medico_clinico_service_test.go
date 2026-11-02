package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"medilist/internal/service/entities"
)

type fakeCitaStoreClinico struct {
	CitaStore
	citas       map[int64]*entities.Cita
	actualizada *entities.Cita
}

func (f *fakeCitaStoreClinico) ObtenerPorID(
	_ context.Context, id int64,
) (*entities.Cita, error) {
	cita, ok := f.citas[id]
	if !ok {
		return nil, entities.ErrNotFound
	}
	return cita, nil
}

func (f *fakeCitaStoreClinico) Actualizar(
	_ context.Context, c *entities.Cita,
) error {
	c.Version++
	f.actualizada = c
	f.citas[c.ID] = c
	return nil
}

type fakeNotaCitaStore struct {
	NotaCitaStore
	notas map[int64]*entities.NotaCita
}

func (f *fakeNotaCitaStore) ObtenerPorCita(
	_ context.Context, citaID int64,
) (*entities.NotaCita, error) {
	nota, ok := f.notas[citaID]
	if !ok {
		return nil, entities.ErrNotFound
	}
	return nota, nil
}

func (f *fakeNotaCitaStore) Crear(_ context.Context, n *entities.NotaCita) error {
	if f.notas == nil {
		f.notas = make(map[int64]*entities.NotaCita)
	}
	n.ID = 50
	n.Version = 1
	f.notas[n.CitaID] = n
	return nil
}

func (f *fakeNotaCitaStore) Actualizar(_ context.Context, n *entities.NotaCita) error {
	n.Version++
	f.notas[n.CitaID] = n
	return nil
}

type fakeDerivacionStore struct {
	DerivacionStore
	creada       *entities.Derivacion
	porID        map[int64]*entities.Derivacion
	listado      []entities.Derivacion
	medicoFiltro int64
}

func (f *fakeDerivacionStore) Crear(_ context.Context, d *entities.Derivacion) error {
	d.ID = 80
	d.Version = 1
	f.creada = d
	if f.porID == nil {
		f.porID = make(map[int64]*entities.Derivacion)
	}
	f.porID[d.ID] = d
	return nil
}

func (f *fakeDerivacionStore) ObtenerPorID(
	_ context.Context, id int64,
) (*entities.Derivacion, error) {
	d, ok := f.porID[id]
	if !ok {
		return nil, entities.ErrNotFound
	}
	return d, nil
}

func (f *fakeDerivacionStore) ListarPorMedicoOrigen(
	_ context.Context, medicoID int64, _ entities.ListOptions,
) ([]entities.Derivacion, error) {
	f.medicoFiltro = medicoID
	return f.listado, nil
}

func (f *fakeDerivacionStore) ListarPorPaciente(
	_ context.Context, _ int64, _ entities.ListOptions,
) ([]entities.Derivacion, error) {
	return f.listado, nil
}

func (f *fakeDerivacionStore) MarcarVencidas(context.Context, time.Time) error {
	return nil
}

type fakeMedicoStoreClinico struct {
	MedicoStore
	medicos map[int64]*entities.Medico
}

func (f *fakeMedicoStoreClinico) ObtenerPorID(
	_ context.Context, id int64,
) (*entities.Medico, error) {
	m, ok := f.medicos[id]
	if !ok {
		return nil, entities.ErrNotFound
	}
	return m, nil
}

type fakeEspecialidadStoreClinico struct {
	EspecialidadStore
	especialidades map[int64]*entities.Especialidad
}

func (f *fakeEspecialidadStoreClinico) ObtenerPorID(
	_ context.Context, id int64,
) (*entities.Especialidad, error) {
	e, ok := f.especialidades[id]
	if !ok {
		return nil, entities.ErrNotFound
	}
	return e, nil
}

func int64Ptr(valor int64) *int64 {
	return &valor
}

func medicoClinicoPrueba(id int64, codigo string) *entities.Medico {
	return &entities.Medico{
		ID:             id,
		Activo:         true,
		EspecialidadID: 1,
		Especialidad: entities.Especialidad{
			ID:     1,
			Codigo: codigo,
			Nombre: codigo,
			Activa: true,
		},
	}
}

func citaClinicaPrueba(id, medicoID, pacienteID int64, estado entities.EstadoCita) *entities.Cita {
	return &entities.Cita{
		ID:         id,
		PacienteID: pacienteID,
		MedicoID:   medicoID,
		Estado:     estado,
		Version:    2,
	}
}

func nuevoServicioClinicoPrueba(
	citas *fakeCitaStoreClinico,
	notas *fakeNotaCitaStore,
	derivaciones *fakeDerivacionStore,
	medicos *fakeMedicoStoreClinico,
	especialidades *fakeEspecialidadStoreClinico,
) *MedicoClinicoService {
	ahora := time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)
	return NewMedicoClinicoService(
		citas, notas, derivaciones, medicos, especialidades,
		MedicoClinicoConfig{
			ReferralValidDays: 90,
			Now:               func() time.Time { return ahora },
		},
	)
}

func TestMedicoClinicoCambiarEstadoSoloCitaPropia(t *testing.T) {
	citas := &fakeCitaStoreClinico{citas: map[int64]*entities.Cita{
		5: citaClinicaPrueba(5, 7, 20, entities.EstadoCitaProgramada),
		6: citaClinicaPrueba(6, 8, 20, entities.EstadoCitaProgramada),
	}}
	svc := nuevoServicioClinicoPrueba(
		citas, &fakeNotaCitaStore{}, &fakeDerivacionStore{},
		&fakeMedicoStoreClinico{}, &fakeEspecialidadStoreClinico{},
	)
	actor := &Actor{UsuarioID: 10, MedicoID: int64Ptr(7)}

	cita, err := svc.CambiarEstado(context.Background(), actor, 5, entities.EstadoCitaConfirmada)
	if err != nil {
		t.Fatalf("cambiar estado: %v", err)
	}
	if cita.Estado != entities.EstadoCitaConfirmada {
		t.Fatalf("estado inesperado: %s", cita.Estado)
	}
	if _, err := svc.CambiarEstado(context.Background(), actor, 6, entities.EstadoCitaConfirmada); !errors.Is(err, entities.ErrForbidden) {
		t.Fatalf("se esperaba ErrForbidden para cita ajena, obtenido %v", err)
	}
}

func TestMedicoClinicoNotaCreaYActualizaConVersion(t *testing.T) {
	citas := &fakeCitaStoreClinico{citas: map[int64]*entities.Cita{
		5: citaClinicaPrueba(5, 7, 20, entities.EstadoCitaAtendida),
	}}
	notas := &fakeNotaCitaStore{}
	svc := nuevoServicioClinicoPrueba(
		citas, notas, &fakeDerivacionStore{},
		&fakeMedicoStoreClinico{}, &fakeEspecialidadStoreClinico{},
	)
	actor := &Actor{UsuarioID: 10, MedicoID: int64Ptr(7)}

	nota, err := svc.GuardarNota(context.Background(), actor, 5, NotaCitaInput{
		Motivo: "Dolor", Evaluacion: "Leve", Plan: "Reposo",
	})
	if err != nil {
		t.Fatalf("crear nota: %v", err)
	}
	if nota.ID == 0 || nota.MedicoID != 7 || nota.Version != 1 {
		t.Fatalf("nota creada inesperada: %+v", nota)
	}

	nota, err = svc.GuardarNota(context.Background(), actor, 5, NotaCitaInput{
		Motivo: "Dolor", Evaluacion: "Mejor", Plan: "Alta", Version: 1,
	})
	if err != nil {
		t.Fatalf("actualizar nota: %v", err)
	}
	if nota.Version != 2 || nota.Plan != "Alta" {
		t.Fatalf("nota actualizada inesperada: %+v", nota)
	}
	if _, err := svc.GuardarNota(context.Background(), actor, 5, NotaCitaInput{
		Motivo: "Dolor", Version: 1,
	}); !errors.Is(err, entities.ErrConflict) {
		t.Fatalf("se esperaba conflicto de versión, obtenido %v", err)
	}
}

func TestMedicoClinicoNotaExigeCitaAtendida(t *testing.T) {
	citas := &fakeCitaStoreClinico{citas: map[int64]*entities.Cita{
		5: citaClinicaPrueba(5, 7, 20, entities.EstadoCitaProgramada),
	}}
	svc := nuevoServicioClinicoPrueba(
		citas, &fakeNotaCitaStore{}, &fakeDerivacionStore{},
		&fakeMedicoStoreClinico{}, &fakeEspecialidadStoreClinico{},
	)
	_, err := svc.GuardarNota(context.Background(), &Actor{UsuarioID: 10, MedicoID: int64Ptr(7)}, 5,
		NotaCitaInput{Motivo: "Dolor"})
	if !errors.Is(err, entities.ErrValidation) {
		t.Fatalf("se esperaba ErrValidation, obtenido %v", err)
	}
}

func TestMedicoClinicoCrearDerivacionValida(t *testing.T) {
	citas := &fakeCitaStoreClinico{citas: map[int64]*entities.Cita{
		5: citaClinicaPrueba(5, 7, 20, entities.EstadoCitaAtendida),
		6: citaClinicaPrueba(6, 8, 20, entities.EstadoCitaAtendida),
	}}
	notas := &fakeNotaCitaStore{}
	derivaciones := &fakeDerivacionStore{}
	medicos := &fakeMedicoStoreClinico{medicos: map[int64]*entities.Medico{
		7: medicoClinicoPrueba(7, "MED-GEN"),
		8: {ID: 8, Activo: true, EspecialidadID: 9, Especialidad: entities.Especialidad{ID: 9, Codigo: "OTORRINO", Activa: true}},
	}}
	especialidades := &fakeEspecialidadStoreClinico{especialidades: map[int64]*entities.Especialidad{
		9: {ID: 9, Codigo: "OTORRINO", Nombre: "Otorrinolaringología", Activa: true, RequiereDerivacion: true},
	}}
	svc := nuevoServicioClinicoPrueba(citas, notas, derivaciones, medicos, especialidades)
	actor := &Actor{UsuarioID: 10, MedicoID: int64Ptr(7)}
	destinoID := int64(8)

	derivacion, err := svc.CrearDerivacion(context.Background(), actor, 5, DerivacionInput{
		EspecialidadDestinoID: 9,
		MedicoDestinoID:       &destinoID,
		Motivo:                "Evaluación especializada",
	})
	if err != nil {
		t.Fatalf("crear derivación: %v", err)
	}
	if derivacion.PacienteID != 20 || derivacion.MedicoOrigenID != 7 ||
		derivacion.CitaOrigenID != 5 || derivacion.Estado != entities.EstadoDerivacionPendiente {
		t.Fatalf("derivación inesperada: %+v", derivacion)
	}
	if derivacion.VigenteHasta.Sub(derivacion.VigenteDesde) != 90*24*time.Hour {
		t.Fatalf("vigencia inesperada: %s", derivacion.VigenteHasta.Sub(derivacion.VigenteDesde))
	}
	if derivacion.MedicoDestinoID == nil || *derivacion.MedicoDestinoID != 8 {
		t.Fatalf("médico destino inesperado: %v", derivacion.MedicoDestinoID)
	}

	if _, err := svc.CrearDerivacion(context.Background(), actor, 6, DerivacionInput{
		EspecialidadDestinoID: 9, Motivo: "Ajena",
	}); !errors.Is(err, entities.ErrForbidden) {
		t.Fatalf("se esperaba ErrForbidden para cita ajena, obtenido %v", err)
	}
}

func TestMedicoClinicoCrearDerivacionRechazaOrigenYDestinoInvalidos(t *testing.T) {
	citas := &fakeCitaStoreClinico{citas: map[int64]*entities.Cita{
		5: citaClinicaPrueba(5, 7, 20, entities.EstadoCitaProgramada),
		6: citaClinicaPrueba(6, 7, 20, entities.EstadoCitaAtendida),
	}}
	medicos := &fakeMedicoStoreClinico{medicos: map[int64]*entities.Medico{
		7: medicoClinicoPrueba(7, "CARD"),
	}}
	especialidades := &fakeEspecialidadStoreClinico{especialidades: map[int64]*entities.Especialidad{
		1: {ID: 1, Codigo: "MED-GEN", Activa: true, RequiereDerivacion: false},
		9: {ID: 9, Codigo: "OTORRINO", Activa: true, RequiereDerivacion: true},
	}}
	svc := nuevoServicioClinicoPrueba(citas, &fakeNotaCitaStore{}, &fakeDerivacionStore{}, medicos, especialidades)
	actor := &Actor{UsuarioID: 10, MedicoID: int64Ptr(7)}

	if _, err := svc.CrearDerivacion(context.Background(), actor, 5, DerivacionInput{
		EspecialidadDestinoID: 9, Motivo: "No atendida",
	}); !errors.Is(err, entities.ErrValidation) {
		t.Fatalf("se esperaba validación por cita no atendida, obtenido %v", err)
	}
	if _, err := svc.CrearDerivacion(context.Background(), actor, 6, DerivacionInput{
		EspecialidadDestinoID: 9, Motivo: "Origen no MED-GEN",
	}); !errors.Is(err, entities.ErrValidation) {
		t.Fatalf("se esperaba validación por origen no MED-GEN, obtenido %v", err)
	}

	medicos.medicos[7] = medicoClinicoPrueba(7, "MED-GEN")
	if _, err := svc.CrearDerivacion(context.Background(), actor, 6, DerivacionInput{
		EspecialidadDestinoID: 1, Motivo: "Destino directo",
	}); !errors.Is(err, entities.ErrValidation) {
		t.Fatalf("se esperaba validación por destino directo, obtenido %v", err)
	}
}

func TestMedicoClinicoListarDerivacionesFuerzaMedico(t *testing.T) {
	derivaciones := &fakeDerivacionStore{listado: []entities.Derivacion{{ID: 1}}}
	svc := nuevoServicioClinicoPrueba(
		&fakeCitaStoreClinico{}, &fakeNotaCitaStore{}, derivaciones,
		&fakeMedicoStoreClinico{}, &fakeEspecialidadStoreClinico{},
	)
	_, err := svc.ListarDerivaciones(context.Background(), &Actor{UsuarioID: 10, MedicoID: int64Ptr(7)}, entities.ListOptions{})
	if err != nil {
		t.Fatalf("listar derivaciones: %v", err)
	}
	if derivaciones.medicoFiltro != 7 {
		t.Fatalf("se esperaba filtro por médico 7, obtenido %d", derivaciones.medicoFiltro)
	}
}
