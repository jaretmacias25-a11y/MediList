package service

import (
	"context"
	"errors"
	"testing"

	"medilist/internal/service/entities"
)

type fakePortalPacienteStore struct {
	PacienteStore
	paciente     *entities.Paciente
	actualizados int
}

func (f *fakePortalPacienteStore) ObtenerPorID(_ context.Context, id int64) (*entities.Paciente, error) {
	if f.paciente == nil || f.paciente.ID != id {
		return nil, entities.ErrNotFound
	}
	return f.paciente, nil
}

func (f *fakePortalPacienteStore) Actualizar(_ context.Context, p *entities.Paciente) error {
	p.Version++
	f.paciente = p
	f.actualizados++
	return nil
}

func actorPaciente(usuarioID, pacienteID int64) *Actor {
	return &Actor{UsuarioID: usuarioID, Username: "paciente", PacienteID: &pacienteID}
}

func pacientePortalPrueba(usuarioID int64) *entities.Paciente {
	return &entities.Paciente{
		ID:        10,
		UsuarioID: &usuarioID,
		Persona: entities.Persona{
			ID:              3,
			TipoDocumento:   "CEDULA",
			NumeroDocumento: "0102030405",
			Nombres:         "Ana",
			Apellidos:       "Pérez",
			Email:           "ana@example.test",
		},
		NumeroHistoria:             "HC-000010",
		ContactoEmergenciaNombre:   "Luis",
		ContactoEmergenciaTelefono: "0999999999",
		Activo:                     true,
		Version:                    2,
	}
}

func TestPortalPacientePerfilExigeVinculo(t *testing.T) {
	svc := NewPortalPacienteService(&fakePortalPacienteStore{})

	_, err := svc.Perfil(context.Background(), &Actor{UsuarioID: 1})
	if !errors.Is(err, entities.ErrForbidden) {
		t.Fatalf("se esperaba ErrForbidden sin vínculo, obtenido %v", err)
	}
}

func TestPortalPacientePerfilNoDevuelvePerfilDeOtroUsuario(t *testing.T) {
	otroUsuarioID := int64(99)
	paciente := pacientePortalPrueba(otroUsuarioID)
	svc := NewPortalPacienteService(&fakePortalPacienteStore{paciente: paciente})

	_, err := svc.Perfil(context.Background(), actorPaciente(1, paciente.ID))
	if !errors.Is(err, entities.ErrForbidden) {
		t.Fatalf("se esperaba ErrForbidden para un paciente ajeno, obtenido %v", err)
	}
}

func TestPortalPacienteActualizarContactoConservaIdentidad(t *testing.T) {
	usuarioID := int64(1)
	paciente := pacientePortalPrueba(usuarioID)
	store := &fakePortalPacienteStore{paciente: paciente}
	svc := NewPortalPacienteService(store)

	actualizado, err := svc.ActualizarContacto(context.Background(), actorPaciente(usuarioID, paciente.ID), DatosContactoPaciente{
		Telefono:                   " 0987654321 ",
		Direccion:                  " Nueva dirección ",
		ContactoEmergenciaNombre:   " María ",
		ContactoEmergenciaTelefono: " 0977777777 ",
		Version:                    paciente.Version,
	})
	if err != nil {
		t.Fatalf("actualizar contacto: %v", err)
	}
	if store.actualizados != 1 {
		t.Fatalf("se esperaba una escritura, obtenidas %d", store.actualizados)
	}
	if actualizado.Persona.NumeroDocumento != "0102030405" ||
		actualizado.Persona.Nombres != "Ana" ||
		actualizado.Persona.Email != "ana@example.test" {
		t.Fatalf("el portal no debe modificar la identidad: %+v", actualizado.Persona)
	}
	if actualizado.Persona.Telefono != "0987654321" ||
		actualizado.Persona.Direccion != "Nueva dirección" ||
		actualizado.ContactoEmergenciaNombre != "María" ||
		actualizado.ContactoEmergenciaTelefono != "0977777777" {
		t.Fatalf("contacto no actualizado correctamente: %+v", actualizado)
	}
}

func TestPortalPacienteActualizarContactoRechazaVersionVieja(t *testing.T) {
	usuarioID := int64(1)
	paciente := pacientePortalPrueba(usuarioID)
	store := &fakePortalPacienteStore{paciente: paciente}
	svc := NewPortalPacienteService(store)

	_, err := svc.ActualizarContacto(context.Background(), actorPaciente(usuarioID, paciente.ID), DatosContactoPaciente{
		Version: paciente.Version - 1,
	})
	if !errors.Is(err, entities.ErrConflict) {
		t.Fatalf("se esperaba ErrConflict por versión vieja, obtenido %v", err)
	}
	if store.actualizados != 0 {
		t.Fatal("no se debe escribir cuando la versión no coincide")
	}
}

func TestPortalPacienteActualizarContactoValidaLongitudes(t *testing.T) {
	usuarioID := int64(1)
	paciente := pacientePortalPrueba(usuarioID)
	svc := NewPortalPacienteService(&fakePortalPacienteStore{paciente: paciente})

	_, err := svc.ActualizarContacto(context.Background(), actorPaciente(usuarioID, paciente.ID), DatosContactoPaciente{
		Telefono: "1234567890123456789012345678901",
	})
	if !errors.Is(err, entities.ErrValidation) {
		t.Fatalf("se esperaba ErrValidation por teléfono largo, obtenido %v", err)
	}
}
