package service

import (
	"errors"
	"testing"

	"medilist/internal/service/entities"
)

func TestActorPermisosYAlcanceGlobal(t *testing.T) {
	admin := Actor{Roles: []string{"admin"}}
	if !admin.TienePermiso("cualquier:permiso") {
		t.Fatal("el administrador debe tener todos los permisos")
	}
	if !admin.PuedeGestionarDatosGlobales() {
		t.Fatal("el administrador debe gestionar datos globales")
	}

	medico := Actor{
		Roles:    []string{"medico"},
		Permisos: []string{"medico:agenda:leer"},
	}
	if !medico.TienePermiso("medico:agenda:leer") {
		t.Fatal("el médico debe conservar su permiso propio")
	}
	if medico.TienePermiso("citas:leer") {
		t.Fatal("el médico no debe tener permisos globales de citas")
	}
	if medico.PuedeGestionarDatosGlobales() {
		t.Fatal("el médico no debe acceder a los endpoints administrativos")
	}

	recepcion := Actor{Roles: []string{"recepcion"}}
	if !recepcion.PuedeGestionarDatosGlobales() {
		t.Fatal("recepción debe gestionar datos globales")
	}
}

func TestActorExigeVinculosClinicos(t *testing.T) {
	actor := Actor{UsuarioID: 3}
	if _, err := actor.PacienteVinculado(); !errors.Is(err, entities.ErrForbidden) {
		t.Fatalf("se esperaba ErrForbidden sin paciente, obtenido %v", err)
	}
	if _, err := actor.MedicoVinculado(); !errors.Is(err, entities.ErrForbidden) {
		t.Fatalf("se esperaba ErrForbidden sin médico, obtenido %v", err)
	}

	pacienteID := int64(10)
	medicoID := int64(20)
	actor.PacienteID = &pacienteID
	actor.MedicoID = &medicoID
	if id, err := actor.PacienteVinculado(); err != nil || id != pacienteID {
		t.Fatalf("paciente vinculado inesperado: id=%d err=%v", id, err)
	}
	if id, err := actor.MedicoVinculado(); err != nil || id != medicoID {
		t.Fatalf("médico vinculado inesperado: id=%d err=%v", id, err)
	}
}
