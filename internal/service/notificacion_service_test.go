package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"medilist/internal/service/entities"
)

type fakeNotificacionStore struct {
	NotificacionStore
	pendientes []entities.Notificacion
	enviadas   []int64
	fallidas   []int64
	maxFallos  int
	encoladas  []entities.Notificacion
	prefs      *entities.PreferenciasNotificacion
}

func (f *fakeNotificacionStore) Encolar(_ context.Context, n *entities.Notificacion) error {
	f.encoladas = append(f.encoladas, *n)
	return nil
}

func (f *fakeNotificacionStore) ReclamarPendientes(
	context.Context, time.Time, int,
) ([]entities.Notificacion, error) {
	return f.pendientes, nil
}

func (f *fakeNotificacionStore) MarcarEnviada(_ context.Context, id int64, _ time.Time) error {
	f.enviadas = append(f.enviadas, id)
	return nil
}

func (f *fakeNotificacionStore) MarcarFallo(
	_ context.Context, id int64, _ string, _ time.Time, maxIntentos int,
) error {
	f.fallidas = append(f.fallidas, id)
	f.maxFallos = maxIntentos
	return nil
}

func (f *fakeNotificacionStore) ObtenerPreferencias(
	context.Context, int64,
) (*entities.PreferenciasNotificacion, error) {
	if f.prefs != nil {
		return f.prefs, nil
	}
	return &entities.PreferenciasNotificacion{EmailHabilitado: true, RecordatoriosHabilitados: true}, nil
}

func (f *fakeNotificacionStore) ActualizarPreferencias(
	_ context.Context, p *entities.PreferenciasNotificacion,
) error {
	f.prefs = p
	return nil
}

type fakeNotificacionEmail struct {
	fallar   bool
	enviadas []entities.Notificacion
}

func (f *fakeNotificacionEmail) EnviarNotificacion(
	_ context.Context, n *entities.Notificacion,
) error {
	if f.fallar {
		return errors.New("proveedor caído")
	}
	f.enviadas = append(f.enviadas, *n)
	return nil
}

type fakeCitaStoreNotificacion struct {
	CitaStore
	listado []entities.Cita
}

func (f *fakeCitaStoreNotificacion) Listar(
	context.Context, entities.CitaFiltro,
) ([]entities.Cita, error) {
	return f.listado, nil
}

type fakePacienteStoreNotificacion struct {
	PacienteStore
	paciente *entities.Paciente
}

func (f *fakePacienteStoreNotificacion) ObtenerPorID(
	_ context.Context, id int64,
) (*entities.Paciente, error) {
	if f.paciente == nil || f.paciente.ID != id {
		return nil, entities.ErrNotFound
	}
	return f.paciente, nil
}

func pacienteNotificacionPrueba() *entities.Paciente {
	return &entities.Paciente{
		ID:     20,
		Activo: true,
		Persona: entities.Persona{
			Nombres: "Ana",
			Email:   "ana@example.test",
		},
	}
}

func citaNotificacionPrueba() *entities.Cita {
	return &entities.Cita{
		ID:                 5,
		Codigo:             "CIT-000005",
		PacienteID:         20,
		PacienteNombre:     "Ana Pérez",
		MedicoID:           7,
		MedicoNombre:       "Carlos Ruiz",
		EspecialidadNombre: "Medicina General",
		FechaHoraInicio:    time.Date(2026, 11, 9, 14, 0, 0, 0, time.UTC),
		Estado:             entities.EstadoCitaConfirmada,
	}
}

func TestNotificacionServiceConstruyeCitaYDerivacion(t *testing.T) {
	svc := NewNotificacionService(
		&fakeNotificacionStore{}, &fakeNotificacionEmail{},
		&fakeCitaStoreNotificacion{}, &fakePacienteStoreNotificacion{paciente: pacienteNotificacionPrueba()},
		nil, NotificacionConfig{},
	)
	cita := citaNotificacionPrueba()
	paciente := pacienteNotificacionPrueba()

	notificacion := svc.CitaCreada(cita, paciente)
	if notificacion == nil || notificacion.Destinatario != "ana@example.test" ||
		notificacion.DedupeKey != "cita_creada:5" {
		t.Fatalf("notificación de cita inesperada: %+v", notificacion)
	}
	if notificacion.Asunto == "" || notificacion.Cuerpo == "" {
		t.Fatalf("plantilla de cita vacía: %+v", notificacion)
	}

	derivacion := &entities.Derivacion{
		ID: 9, EspecialidadDestino: "Otorrinolaringología",
		VigenteHasta: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
	}
	d := svc.DerivacionEmitida(derivacion, paciente)
	if d == nil || d.DedupeKey != "derivacion_emitida:9" || d.Tipo != entities.NotificacionDerivacionEmitida {
		t.Fatalf("notificación de derivación inesperada: %+v", d)
	}
}

func TestNotificacionServiceProcesaExitoYFallo(t *testing.T) {
	store := &fakeNotificacionStore{pendientes: []entities.Notificacion{
		{ID: 1, Tipo: entities.NotificacionCitaCreada, Destinatario: "a@example.test"},
	}}
	email := &fakeNotificacionEmail{}
	svc := NewNotificacionService(
		store, email, nil, nil, nil,
		NotificacionConfig{MaxAttempts: 5, BackoffBase: time.Second},
	)
	if err := svc.Procesar(context.Background()); err != nil {
		t.Fatalf("procesar outbox: %v", err)
	}
	if len(email.enviadas) != 1 || len(store.enviadas) != 1 {
		t.Fatalf("envío exitoso inesperado: email=%d store=%d", len(email.enviadas), len(store.enviadas))
	}

	store.pendientes = []entities.Notificacion{{ID: 2}}
	email.fallar = true
	if err := svc.Procesar(context.Background()); err != nil {
		t.Fatalf("procesar fallo: %v", err)
	}
	if len(store.fallidas) != 1 || store.maxFallos != 5 {
		t.Fatalf("fallo no registrado: %+v", store)
	}
}

func TestNotificacionServicePreferencias(t *testing.T) {
	store := &fakeNotificacionStore{}
	svc := NewNotificacionService(store, &fakeNotificacionEmail{}, nil, nil, nil, NotificacionConfig{})
	actor := &Actor{UsuarioID: 10}

	p, err := svc.ObtenerPreferencias(context.Background(), actor)
	if err != nil {
		t.Fatalf("obtener preferencias: %v", err)
	}
	if !p.EmailHabilitado || !p.RecordatoriosHabilitados {
		t.Fatalf("preferencias por defecto inesperadas: %+v", p)
	}

	p, err = svc.ActualizarPreferencias(context.Background(), actor, false, false)
	if err != nil {
		t.Fatalf("actualizar preferencias: %v", err)
	}
	if p.UsuarioID != 10 || p.EmailHabilitado || p.RecordatoriosHabilitados {
		t.Fatalf("preferencias actualizadas inesperadas: %+v", p)
	}
}

func TestNotificacionServiceEncolaRecordatorio(t *testing.T) {
	ahora := time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)
	cita := *citaNotificacionPrueba()
	cita.FechaHoraInicio = ahora.Add(3 * time.Hour)
	citas := &fakeCitaStoreNotificacion{listado: []entities.Cita{cita}}
	pacientes := &fakePacienteStoreNotificacion{paciente: pacienteNotificacionPrueba()}
	store := &fakeNotificacionStore{}
	svc := NewNotificacionService(
		store, &fakeNotificacionEmail{}, citas, pacientes, nil,
		NotificacionConfig{
			ReminderHours: []int{24, 2},
			Now:           func() time.Time { return ahora },
		},
	)
	if err := svc.ProcesarRecordatorios(context.Background()); err != nil {
		t.Fatalf("procesar recordatorios: %v", err)
	}
	if len(store.encoladas) != 1 || store.encoladas[0].DedupeKey != "recordatorio:24h:5" {
		t.Fatalf("recordatorio inesperado: %+v", store.encoladas)
	}
}
