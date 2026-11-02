package service

import (
	"context"
	"errors"
	"testing"

	"medilist/internal/service/entities"
)

type fakeExcepcionDerivacionStore struct {
	ExcepcionDerivacionStore
	porCita map[int64]*entities.ExcepcionDerivacion
	creada  *entities.ExcepcionDerivacion
}

func (f *fakeExcepcionDerivacionStore) Crear(
	_ context.Context, e *entities.ExcepcionDerivacion,
) error {
	e.ID = 90
	f.creada = e
	if f.porCita == nil {
		f.porCita = make(map[int64]*entities.ExcepcionDerivacion)
	}
	f.porCita[e.CitaID] = e
	return nil
}

func (f *fakeExcepcionDerivacionStore) ObtenerPorCita(
	_ context.Context, citaID int64,
) (*entities.ExcepcionDerivacion, error) {
	e, ok := f.porCita[citaID]
	if !ok {
		return nil, entities.ErrNotFound
	}
	return e, nil
}

func TestDerivacionAdminSobrescribeSoloAdmin(t *testing.T) {
	citas := &fakeCitaStoreClinico{citas: map[int64]*entities.Cita{
		5: citaClinicaPrueba(5, 7, 20, entities.EstadoCitaProgramada),
	}}
	medicos := &fakeMedicoStoreClinico{medicos: map[int64]*entities.Medico{
		7: {
			ID: 7, Activo: true, EspecialidadID: 9,
			Especialidad: entities.Especialidad{
				ID: 9, Codigo: "OTORRINO", Activa: true, RequiereDerivacion: true,
			},
		},
	}}
	excepciones := &fakeExcepcionDerivacionStore{}
	svc := NewDerivacionAdminService(citas, medicos, excepciones)

	noAdmin := &Actor{UsuarioID: 10, Roles: []string{"recepcion"},
		Permisos: []string{"derivaciones:sobrescribir"}}
	if _, err := svc.Sobrescribir(context.Background(), noAdmin, 5, "Urgencia"); !errors.Is(err, entities.ErrForbidden) {
		t.Fatalf("se esperaba ErrForbidden para no admin, obtenido %v", err)
	}

	admin := &Actor{UsuarioID: 1, Roles: []string{"admin"}}
	excepcion, err := svc.Sobrescribir(context.Background(), admin, 5, "Urgencia operativa")
	if err != nil {
		t.Fatalf("sobrescribir: %v", err)
	}
	if excepcion.ID == 0 || excepcion.CitaID != 5 || excepcion.Motivo != "Urgencia operativa" {
		t.Fatalf("excepción inesperada: %+v", excepcion)
	}
	if excepcion.AutorizadoPorUsuarioID == nil || *excepcion.AutorizadoPorUsuarioID != 1 {
		t.Fatalf("autorizador inesperado: %v", excepcion.AutorizadoPorUsuarioID)
	}
}

func TestDerivacionAdminRechazaCitaDirectaOConDerivacion(t *testing.T) {
	citas := &fakeCitaStoreClinico{citas: map[int64]*entities.Cita{
		5: citaClinicaPrueba(5, 7, 20, entities.EstadoCitaProgramada),
		6: func() *entities.Cita {
			c := citaClinicaPrueba(6, 7, 20, entities.EstadoCitaProgramada)
			id := int64(80)
			c.DerivacionID = &id
			return c
		}(),
	}}
	medicos := &fakeMedicoStoreClinico{medicos: map[int64]*entities.Medico{
		7: {
			ID: 7, Activo: true, EspecialidadID: 1,
			Especialidad: entities.Especialidad{
				ID: 1, Codigo: "MED-GEN", Activa: true, RequiereDerivacion: false,
			},
		},
	}}
	svc := NewDerivacionAdminService(citas, medicos, &fakeExcepcionDerivacionStore{})
	admin := &Actor{UsuarioID: 1, Roles: []string{"admin"}}

	if _, err := svc.Sobrescribir(context.Background(), admin, 5, "No aplica"); !errors.Is(err, entities.ErrValidation) {
		t.Fatalf("se esperaba validación para especialidad directa, obtenido %v", err)
	}

	// Cambiar el médico a una especialidad con derivación para probar la cita
	// que ya tiene derivación.
	medicos.medicos[7] = &entities.Medico{
		ID: 7, Activo: true, EspecialidadID: 9,
		Especialidad: entities.Especialidad{
			ID: 9, Codigo: "OTORRINO", Activa: true, RequiereDerivacion: true,
		},
	}
	if _, err := svc.Sobrescribir(context.Background(), admin, 6, "Ya tiene"); !errors.Is(err, entities.ErrConflict) {
		t.Fatalf("se esperaba conflicto por derivación existente, obtenido %v", err)
	}
}
