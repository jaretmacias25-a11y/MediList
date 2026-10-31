package service

import (
	"context"
	"errors"
	"testing"

	"medilist/internal/service/entities"
)

type fakeCitaStorePortal struct {
	CitaStore
	filtro  entities.CitaFiltro
	citas   map[int64]*entities.Cita
	listado []entities.Cita
}

func (f *fakeCitaStorePortal) Listar(_ context.Context, filtro entities.CitaFiltro) ([]entities.Cita, error) {
	f.filtro = filtro
	return f.listado, nil
}

func (f *fakeCitaStorePortal) ObtenerPorID(_ context.Context, id int64) (*entities.Cita, error) {
	cita, ok := f.citas[id]
	if !ok {
		return nil, entities.ErrNotFound
	}
	return cita, nil
}

type fakePacienteStorePortal struct {
	PacienteStore
	relacion  bool
	paciente  *entities.Paciente
	consultas int
}

func (f *fakePacienteStorePortal) PerteneceAMedico(context.Context, int64, int64) (bool, error) {
	return f.relacion, nil
}

func (f *fakePacienteStorePortal) ObtenerPorID(_ context.Context, id int64) (*entities.Paciente, error) {
	f.consultas++
	if f.paciente == nil || f.paciente.ID != id {
		return nil, entities.ErrNotFound
	}
	return f.paciente, nil
}

func actorMedico(usuarioID, medicoID int64) *Actor {
	return &Actor{UsuarioID: usuarioID, Username: "medico", MedicoID: &medicoID}
}

func TestMedicoPortalAgendaFuerzaMedicoDelActor(t *testing.T) {
	store := &fakeCitaStorePortal{
		listado: []entities.Cita{{ID: 1, MedicoID: 7}},
	}
	svc := NewMedicoPortalService(store, &fakePacienteStorePortal{})

	lista, err := svc.Agenda(context.Background(), actorMedico(11, 7), entities.CitaFiltro{
		MedicoID: 99,
		Estado:   entities.EstadoCitaProgramada,
	})
	if err != nil {
		t.Fatalf("agenda: %v", err)
	}
	if len(lista) != 1 {
		t.Fatalf("se esperaba una cita, obtenidas %d", len(lista))
	}
	if store.filtro.MedicoID != 7 {
		t.Fatalf("el filtro debe forzar el médico del actor, obtenido %d", store.filtro.MedicoID)
	}
}

func TestMedicoPortalAgendaExigeVinculo(t *testing.T) {
	svc := NewMedicoPortalService(&fakeCitaStorePortal{}, &fakePacienteStorePortal{})

	_, err := svc.Agenda(context.Background(), &Actor{UsuarioID: 11}, entities.CitaFiltro{})
	if !errors.Is(err, entities.ErrForbidden) {
		t.Fatalf("se esperaba ErrForbidden sin vínculo, obtenido %v", err)
	}
}

func TestMedicoPortalNoDevuelveCitaDeOtroMedico(t *testing.T) {
	store := &fakeCitaStorePortal{
		citas: map[int64]*entities.Cita{
			5: {ID: 5, MedicoID: 8, PacienteID: 20},
		},
	}
	svc := NewMedicoPortalService(store, &fakePacienteStorePortal{})

	_, err := svc.ObtenerCita(context.Background(), actorMedico(11, 7), 5)
	if !errors.Is(err, entities.ErrForbidden) {
		t.Fatalf("se esperaba ErrForbidden para cita ajena, obtenido %v", err)
	}
}

func TestMedicoPortalPacienteSinRelacionNoSeConsulta(t *testing.T) {
	paciente := &entities.Paciente{ID: 20, Activo: true}
	pacientes := &fakePacienteStorePortal{relacion: false, paciente: paciente}
	svc := NewMedicoPortalService(&fakeCitaStorePortal{}, pacientes)

	_, err := svc.PacienteAtendido(context.Background(), actorMedico(11, 7), paciente.ID)
	if !errors.Is(err, entities.ErrForbidden) {
		t.Fatalf("se esperaba ErrForbidden sin relación, obtenido %v", err)
	}
	if pacientes.consultas != 0 {
		t.Fatal("no se debe consultar el paciente si no existe relación de atención")
	}
}

func TestMedicoPortalPacienteConRelacionSeDevuelve(t *testing.T) {
	paciente := &entities.Paciente{ID: 20, Activo: true}
	pacientes := &fakePacienteStorePortal{relacion: true, paciente: paciente}
	svc := NewMedicoPortalService(&fakeCitaStorePortal{}, pacientes)

	obtenido, err := svc.PacienteAtendido(context.Background(), actorMedico(11, 7), paciente.ID)
	if err != nil {
		t.Fatalf("paciente atendido: %v", err)
	}
	if obtenido.ID != paciente.ID {
		t.Fatalf("paciente inesperado: %d", obtenido.ID)
	}
}
