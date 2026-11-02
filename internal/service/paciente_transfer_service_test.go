package service

import (
	"context"
	"errors"
	"testing"

	"medilist/internal/service/entities"
)

type fakePacienteStoreTransfer struct {
	PacienteStore
	paciente *entities.Paciente
}

func (f *fakePacienteStoreTransfer) ObtenerPorID(
	_ context.Context, id int64,
) (*entities.Paciente, error) {
	if f.paciente == nil || f.paciente.ID != id {
		return nil, entities.ErrNotFound
	}
	return f.paciente, nil
}

type fakeMedicoStoreTransfer struct {
	MedicoStore
	medicos map[int64]*entities.Medico
}

func (f *fakeMedicoStoreTransfer) ObtenerPorID(
	_ context.Context, id int64,
) (*entities.Medico, error) {
	m, ok := f.medicos[id]
	if !ok {
		return nil, entities.ErrNotFound
	}
	return m, nil
}

type fakeTransferenciaStore struct {
	TransferenciaStore
	pacienteID      int64
	medicoNuevoID   int64
	motivo          string
	actorID         int64
	version         int64
	transferencia   *entities.TransferenciaPaciente
	listado         []entities.TransferenciaPaciente
	pacienteListado int64
}

func (f *fakeTransferenciaStore) Transferir(
	_ context.Context,
	pacienteID, medicoNuevoID int64,
	motivo string,
	actorID int64,
	version int64,
) (*entities.TransferenciaPaciente, error) {
	f.pacienteID = pacienteID
	f.medicoNuevoID = medicoNuevoID
	f.motivo = motivo
	f.actorID = actorID
	f.version = version
	if f.transferencia == nil {
		f.transferencia = &entities.TransferenciaPaciente{
			ID:            1,
			PacienteID:    pacienteID,
			MedicoNuevoID: medicoNuevoID,
			Motivo:        motivo,
		}
	}
	return f.transferencia, nil
}

func (f *fakeTransferenciaStore) ListarTransferencias(
	_ context.Context, pacienteID int64,
) ([]entities.TransferenciaPaciente, error) {
	f.pacienteListado = pacienteID
	return f.listado, nil
}

func actorTransferencia(usuarioID int64, medicoID *int64, permiso bool) *Actor {
	actor := &Actor{UsuarioID: usuarioID, MedicoID: medicoID}
	if permiso {
		actor.Permisos = []string{"pacientes:transferir"}
	}
	return actor
}

func pacienteTransferenciaPrueba() *entities.Paciente {
	responsable := int64(7)
	return &entities.Paciente{
		ID:                  20,
		MedicoResponsableID: &responsable,
		Activo:              true,
		Version:             3,
	}
}

func TestPacienteTransferServiceTransfiereConVersion(t *testing.T) {
	pacientes := &fakePacienteStoreTransfer{paciente: pacienteTransferenciaPrueba()}
	medicos := &fakeMedicoStoreTransfer{medicos: map[int64]*entities.Medico{
		8: {ID: 8, Activo: true},
	}}
	transferencias := &fakeTransferenciaStore{}
	svc := NewPacienteTransferService(pacientes, medicos, transferencias)
	actor := actorTransferencia(10, nil, true)

	transferencia, err := svc.Transferir(context.Background(), actor, 20, TransferirPacienteInput{
		MedicoDestinoID: 8,
		Motivo:          "Cambio de médico tratante",
		Version:         3,
	})
	if err != nil {
		t.Fatalf("transferir: %v", err)
	}
	if transferencia.ID == 0 || transferencias.medicoNuevoID != 8 ||
		transferencias.actorID != 10 || transferencias.version != 3 {
		t.Fatalf("argumentos de transferencia inesperados: %+v", transferencias)
	}
}

func TestPacienteTransferServiceRechazaSinPermiso(t *testing.T) {
	svc := NewPacienteTransferService(
		&fakePacienteStoreTransfer{paciente: pacienteTransferenciaPrueba()},
		&fakeMedicoStoreTransfer{}, &fakeTransferenciaStore{},
	)
	_, err := svc.Transferir(context.Background(), actorTransferencia(10, nil, false), 20, TransferirPacienteInput{
		MedicoDestinoID: 8, Motivo: "x",
	})
	if !errors.Is(err, entities.ErrForbidden) {
		t.Fatalf("se esperaba ErrForbidden, obtenido %v", err)
	}
}

func TestPacienteTransferServiceRechazaAutoTransferencia(t *testing.T) {
	svc := NewPacienteTransferService(
		&fakePacienteStoreTransfer{paciente: pacienteTransferenciaPrueba()},
		&fakeMedicoStoreTransfer{medicos: map[int64]*entities.Medico{
			8: {ID: 8, Activo: true},
		}},
		&fakeTransferenciaStore{},
	)
	_, err := svc.Transferir(context.Background(), actorTransferencia(10, int64Ptr(8), true), 20,
		TransferirPacienteInput{MedicoDestinoID: 8, Motivo: "auto"})
	if !errors.Is(err, entities.ErrForbidden) {
		t.Fatalf("se esperaba ErrForbidden, obtenido %v", err)
	}
}

func TestPacienteTransferServiceRechazaMismoResponsableYVersion(t *testing.T) {
	paciente := pacienteTransferenciaPrueba()
	medicos := &fakeMedicoStoreTransfer{medicos: map[int64]*entities.Medico{
		7: {ID: 7, Activo: true},
		8: {ID: 8, Activo: true},
	}}
	svc := NewPacienteTransferService(
		&fakePacienteStoreTransfer{paciente: paciente}, medicos, &fakeTransferenciaStore{},
	)
	actor := actorTransferencia(10, nil, true)

	if _, err := svc.Transferir(context.Background(), actor, 20, TransferirPacienteInput{
		MedicoDestinoID: 7, Motivo: "mismo",
	}); !errors.Is(err, entities.ErrConflict) {
		t.Fatalf("se esperaba conflicto por mismo responsable, obtenido %v", err)
	}
	if _, err := svc.Transferir(context.Background(), actor, 20, TransferirPacienteInput{
		MedicoDestinoID: 8, Motivo: "versión", Version: 2,
	}); !errors.Is(err, entities.ErrConflict) {
		t.Fatalf("se esperaba conflicto de versión, obtenido %v", err)
	}
}

func TestPacienteTransferServiceListaHistorial(t *testing.T) {
	transferencias := &fakeTransferenciaStore{
		listado: []entities.TransferenciaPaciente{{ID: 1, PacienteID: 20}},
	}
	svc := NewPacienteTransferService(
		&fakePacienteStoreTransfer{paciente: pacienteTransferenciaPrueba()},
		&fakeMedicoStoreTransfer{}, transferencias,
	)
	lista, err := svc.ListarTransferencias(
		context.Background(), actorTransferencia(10, nil, true), 20)
	if err != nil {
		t.Fatalf("listar transferencias: %v", err)
	}
	if len(lista) != 1 || transferencias.pacienteListado != 20 {
		t.Fatalf("historial inesperado: %+v", transferencias)
	}
}
