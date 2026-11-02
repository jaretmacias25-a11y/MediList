package postgres

import (
	"context"
	"testing"
	"time"

	"medilist/internal/service/entities"
)

func TestNotificacionStorePreferencias(t *testing.T) {
	pool := poolIntegracion(t)
	store := NewNotificacionStore(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var usuarioID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM usuarios ORDER BY id LIMIT 1`).Scan(&usuarioID); err != nil {
		t.Fatalf("no se encontró usuario: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _ = pool.Exec(ctx, `DELETE FROM preferencias_notificacion WHERE usuario_id = $1`, usuarioID)
	})

	prefs, err := store.ObtenerPreferencias(ctx, usuarioID)
	if err != nil {
		t.Fatalf("obtener preferencias: %v", err)
	}
	if !prefs.EmailHabilitado || !prefs.RecordatoriosHabilitados {
		t.Fatalf("preferencias por defecto inesperadas: %+v", prefs)
	}
	prefs.EmailHabilitado = false
	prefs.RecordatoriosHabilitados = false
	if err := store.ActualizarPreferencias(ctx, prefs); err != nil {
		t.Fatalf("actualizar preferencias: %v", err)
	}
	actualizadas, err := store.ObtenerPreferencias(ctx, usuarioID)
	if err != nil {
		t.Fatalf("releer preferencias: %v", err)
	}
	if actualizadas.EmailHabilitado || actualizadas.RecordatoriosHabilitados {
		t.Fatalf("preferencias no persistidas: %+v", actualizadas)
	}
}

func TestNotificacionOutboxSeEncolaConLaDerivacion(t *testing.T) {
	pool := poolIntegracion(t)
	pacienteID := crearPacientePruebaDerivacion(t, pool)
	medicoGenID := crearMedicoPruebaDerivacion(t, pool, "MED-GEN")
	medicoOrlID := crearMedicoPruebaDerivacion(t, pool, "OTORRINO")
	registrarLimpiezaCitasPruebaDerivacion(t, pool, pacienteID)
	citaID := crearCitaPruebaDerivacion(
		t, pool, pacienteID, medicoGenID, baseHoraPruebaDerivacion(6), "atendida",
	)
	especialidadID := especialidadIDPruebaDerivacion(t, pool, "OTORRINO")
	store := NewDerivacionStore(pool)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ahora := time.Now().UTC().Truncate(time.Second)
	derivacion := &entities.Derivacion{
		PacienteID:            pacienteID,
		MedicoOrigenID:        medicoGenID,
		CitaOrigenID:          citaID,
		EspecialidadDestinoID: especialidadID,
		MedicoDestinoID:       &medicoOrlID,
		Motivo:                "Notificación de derivación",
		Prioridad:             entities.PrioridadDerivacionNormal,
		Estado:                entities.EstadoDerivacionPendiente,
		VigenteDesde:          ahora,
		VigenteHasta:          ahora.AddDate(0, 0, 90),
		NotificacionPendiente: &entities.Notificacion{
			Tipo:         entities.NotificacionDerivacionEmitida,
			Destinatario: "paciente@example.test",
			Nombre:       "Paciente",
			Asunto:       "Derivación emitida",
			Cuerpo:       "Nueva derivación",
			DedupeKey:    "integration:derivacion_emitida",
		},
	}
	if err := store.Crear(ctx, derivacion); err != nil {
		t.Fatalf("crear derivación con outbox: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _ = pool.Exec(ctx, `DELETE FROM notificaciones_outbox WHERE dedupe_key = $1`, "integration:derivacion_emitida")
	})
	var estado string
	if err := pool.QueryRow(ctx, `
		SELECT estado FROM notificaciones_outbox
		WHERE dedupe_key = 'integration:derivacion_emitida'`).Scan(&estado); err != nil {
		t.Fatalf("la notificación no se encoló con la derivación: %v", err)
	}
	if estado != "pendiente" {
		t.Fatalf("estado inicial inesperado: %s", estado)
	}
}

func TestNotificacionOutboxSeEncolaConLaCitaYSeProcesa(t *testing.T) {
	pool := poolIntegracion(t)
	pacienteID := crearPacientePruebaDerivacion(t, pool)
	medicoID := crearMedicoPrueba(t, pool)
	citaStore := NewCitaStore(pool)
	notificacionStore := NewNotificacionStore(pool)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cita := &entities.Cita{
		PacienteID:      pacienteID,
		MedicoID:        medicoID,
		FechaHoraInicio: time.Now().UTC().Add(72 * time.Hour).Truncate(time.Minute),
		DuracionMin:     30,
		Tipo:            entities.TipoCitaPrimeraVez,
		Estado:          entities.EstadoCitaProgramada,
		Canal:           entities.CanalCitaPortalPaciente,
		NotificacionPendiente: &entities.Notificacion{
			Tipo:         entities.NotificacionCitaCreada,
			Destinatario: "paciente@example.test",
			Nombre:       "Paciente",
			Asunto:       "Cita creada",
			Cuerpo:       "Tu cita fue creada",
			DedupeKey:    "integration:cita_creada",
		},
	}
	if err := citaStore.Crear(ctx, cita); err != nil {
		t.Fatalf("crear cita con outbox: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _ = pool.Exec(ctx, `DELETE FROM notificaciones_outbox WHERE dedupe_key = $1`, "integration:cita_creada")
		_, _ = pool.Exec(ctx, `DELETE FROM citas WHERE id = $1`, cita.ID)
	})

	var estado string
	if err := pool.QueryRow(ctx, `
		SELECT estado FROM notificaciones_outbox
		WHERE dedupe_key = 'integration:cita_creada'`).Scan(&estado); err != nil {
		t.Fatalf("la notificación no se encoló con la cita: %v", err)
	}
	if estado != "pendiente" {
		t.Fatalf("estado inicial inesperado: %s", estado)
	}

	lista, err := notificacionStore.ReclamarPendientes(ctx, time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("reclamar pendientes: %v", err)
	}
	if len(lista) != 1 || lista[0].DedupeKey != "integration:cita_creada" {
		t.Fatalf("outbox reclamado inesperado: %+v", lista)
	}
	if err := notificacionStore.MarcarEnviada(ctx, lista[0].ID, time.Now().UTC()); err != nil {
		t.Fatalf("marcar enviada: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT estado FROM notificaciones_outbox
		WHERE id = $1`, lista[0].ID).Scan(&estado); err != nil {
		t.Fatalf("consultar estado final: %v", err)
	}
	if estado != "enviada" {
		t.Fatalf("estado final inesperado: %s", estado)
	}
}
