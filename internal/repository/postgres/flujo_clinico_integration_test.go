package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"medilist/internal/service/entities"
)

func TestNotaCitaStoreCreaYActualizaConVersion(t *testing.T) {
	pool := poolIntegracion(t)
	pacienteID := crearPacientePruebaDerivacion(t, pool)
	medicoID := crearMedicoPruebaDerivacion(t, pool, "MED-GEN")
	registrarLimpiezaCitasPruebaDerivacion(t, pool, pacienteID)
	citaID := crearCitaPruebaDerivacion(
		t, pool, pacienteID, medicoID, baseHoraPruebaDerivacion(0), "atendida",
	)
	store := NewNotaCitaStore(pool)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	nota := &entities.NotaCita{
		CitaID:     citaID,
		MedicoID:   medicoID,
		Motivo:     "Dolor",
		Evaluacion: "Leve",
		Plan:       "Reposo",
	}
	if err := store.Crear(ctx, nota); err != nil {
		t.Fatalf("crear nota: %v", err)
	}
	if nota.ID == 0 || nota.Version != 1 {
		t.Fatalf("nota creada sin identidad/versión: %+v", nota)
	}
	obtenida, err := store.ObtenerPorCita(ctx, citaID)
	if err != nil {
		t.Fatalf("obtener nota: %v", err)
	}
	if obtenida.Motivo != "Dolor" || obtenida.MedicoID != medicoID {
		t.Fatalf("nota inesperada: %+v", obtenida)
	}

	obtenida.Evaluacion = "Mejor"
	if err := store.Actualizar(ctx, obtenida); err != nil {
		t.Fatalf("actualizar nota: %v", err)
	}
	if obtenida.Version != 2 {
		t.Fatalf("se esperaba versión 2, obtenida %d", obtenida.Version)
	}
	obtenida.Version = 1
	if err := store.Actualizar(ctx, obtenida); !errors.Is(err, entities.ErrConflict) {
		t.Fatalf("se esperaba conflicto de versión, obtenido %v", err)
	}
}

func TestDerivacionStoreCreaYListaPorMedicoOrigen(t *testing.T) {
	pool := poolIntegracion(t)
	pacienteID := crearPacientePruebaDerivacion(t, pool)
	medicoGenID := crearMedicoPruebaDerivacion(t, pool, "MED-GEN")
	medicoOtorrinoID := crearMedicoPruebaDerivacion(t, pool, "OTORRINO")
	registrarLimpiezaCitasPruebaDerivacion(t, pool, pacienteID)
	citaID := crearCitaPruebaDerivacion(
		t, pool, pacienteID, medicoGenID, baseHoraPruebaDerivacion(1), "atendida",
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
		MedicoDestinoID:       &medicoOtorrinoID,
		Motivo:                "Evaluación especializada",
		Prioridad:             entities.PrioridadDerivacionNormal,
		Estado:                entities.EstadoDerivacionPendiente,
		VigenteDesde:          ahora,
		VigenteHasta:          ahora.AddDate(0, 0, 90),
	}
	if err := store.Crear(ctx, derivacion); err != nil {
		t.Fatalf("crear derivación: %v", err)
	}
	if derivacion.ID == 0 || derivacion.Version != 1 {
		t.Fatalf("derivación creada sin identidad/versión: %+v", derivacion)
	}
	obtenida, err := store.ObtenerPorID(ctx, derivacion.ID)
	if err != nil {
		t.Fatalf("obtener derivación: %v", err)
	}
	if obtenida.Estado != entities.EstadoDerivacionPendiente ||
		obtenida.EspecialidadDestino == "" ||
		obtenida.MedicoDestinoNombre == "" {
		t.Fatalf("derivación inesperada: %+v", obtenida)
	}

	lista, err := store.ListarPorMedicoOrigen(ctx, medicoGenID, entities.ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("listar derivaciones: %v", err)
	}
	if len(lista) != 1 || lista[0].ID != derivacion.ID {
		t.Fatalf("lista de derivaciones inesperada: %+v", lista)
	}
}

func TestDerivacionStoreMarcaVencidasYListaPorPaciente(t *testing.T) {
	pool := poolIntegracion(t)
	pacienteID := crearPacientePruebaDerivacion(t, pool)
	medicoGenID := crearMedicoPruebaDerivacion(t, pool, "MED-GEN")
	registrarLimpiezaCitasPruebaDerivacion(t, pool, pacienteID)
	citaID := crearCitaPruebaDerivacion(
		t, pool, pacienteID, medicoGenID, baseHoraPruebaDerivacion(5), "atendida",
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
		Motivo:                "Derivación vencida",
		Prioridad:             entities.PrioridadDerivacionNormal,
		Estado:                entities.EstadoDerivacionPendiente,
		VigenteDesde:          ahora.Add(-48 * time.Hour),
		VigenteHasta:          ahora.Add(-24 * time.Hour),
	}
	if err := store.Crear(ctx, derivacion); err != nil {
		t.Fatalf("crear derivación vencida: %v", err)
	}
	if err := store.MarcarVencidas(ctx, ahora); err != nil {
		t.Fatalf("marcar vencidas: %v", err)
	}
	vencida, err := store.ObtenerPorID(ctx, derivacion.ID)
	if err != nil {
		t.Fatalf("obtener derivación vencida: %v", err)
	}
	if vencida.Estado != entities.EstadoDerivacionVencida {
		t.Fatalf("estado inesperado: %s", vencida.Estado)
	}
	lista, err := store.ListarPorPaciente(ctx, pacienteID, entities.ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("listar derivaciones por paciente: %v", err)
	}
	if len(lista) != 1 || lista[0].ID != derivacion.ID {
		t.Fatalf("lista por paciente inesperada: %+v", lista)
	}
}

func TestCitaStoreReservaYConsumeDerivacion(t *testing.T) {
	pool := poolIntegracion(t)
	pacienteID := crearPacientePruebaDerivacion(t, pool)
	medicoGenID := crearMedicoPruebaDerivacion(t, pool, "MED-GEN")
	medicoOrlID := crearMedicoPruebaDerivacion(t, pool, "OTORRINO")
	registrarLimpiezaCitasPruebaDerivacion(t, pool, pacienteID)
	citaOrigenID := crearCitaPruebaDerivacion(
		t, pool, pacienteID, medicoGenID, baseHoraPruebaDerivacion(2), "atendida",
	)
	especialidadID := especialidadIDPruebaDerivacion(t, pool, "OTORRINO")
	derivacionStore := NewDerivacionStore(pool)
	citaStore := NewCitaStore(pool)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	ahora := time.Now().UTC().Truncate(time.Second)
	derivacion := &entities.Derivacion{
		PacienteID:            pacienteID,
		MedicoOrigenID:        medicoGenID,
		CitaOrigenID:          citaOrigenID,
		EspecialidadDestinoID: especialidadID,
		MedicoDestinoID:       &medicoOrlID,
		Motivo:                "Reserva de especialista",
		Prioridad:             entities.PrioridadDerivacionNormal,
		Estado:                entities.EstadoDerivacionPendiente,
		VigenteDesde:          ahora,
		VigenteHasta:          ahora.AddDate(0, 0, 90),
	}
	if err := derivacionStore.Crear(ctx, derivacion); err != nil {
		t.Fatalf("crear derivación: %v", err)
	}

	cita := &entities.Cita{
		PacienteID:      pacienteID,
		MedicoID:        medicoOrlID,
		FechaHoraInicio: baseHoraPruebaDerivacion(10),
		DuracionMin:     30,
		Tipo:            entities.TipoCitaPrimeraVez,
		Estado:          entities.EstadoCitaProgramada,
		Canal:           entities.CanalCitaPortalPaciente,
	}
	if err := citaStore.CrearConDerivacion(ctx, cita, derivacion.ID); err != nil {
		t.Fatalf("crear cita con derivación: %v", err)
	}
	agendada, err := derivacionStore.ObtenerPorID(ctx, derivacion.ID)
	if err != nil {
		t.Fatalf("obtener derivación agendada: %v", err)
	}
	if agendada.Estado != entities.EstadoDerivacionAgendada {
		t.Fatalf("estado de derivación inesperado: %s", agendada.Estado)
	}

	segunda := &entities.Cita{
		PacienteID:      pacienteID,
		MedicoID:        medicoOrlID,
		FechaHoraInicio: baseHoraPruebaDerivacion(11),
		DuracionMin:     30,
		Tipo:            entities.TipoCitaPrimeraVez,
		Estado:          entities.EstadoCitaProgramada,
		Canal:           entities.CanalCitaPortalPaciente,
	}
	if err := citaStore.CrearConDerivacion(ctx, segunda, derivacion.ID); !errors.Is(err, entities.ErrConflict) {
		t.Fatalf("la derivación ya agendada debía rechazar otra cita; se obtuvo %v", err)
	}

	cita.Estado = entities.EstadoCitaAtendida
	if err := citaStore.CambiarEstadoConDerivacion(
		ctx, cita, entities.EstadoDerivacionAtendida, "",
	); err != nil {
		t.Fatalf("marcar cita y derivación atendidas: %v", err)
	}
	atendida, err := derivacionStore.ObtenerPorID(ctx, derivacion.ID)
	if err != nil {
		t.Fatalf("obtener derivación atendida: %v", err)
	}
	if atendida.Estado != entities.EstadoDerivacionAtendida {
		t.Fatalf("estado de derivación atendida inesperado: %s", atendida.Estado)
	}
}

func TestCitaStoreCancelarConDerivacionLiberaOConsume(t *testing.T) {
	pool := poolIntegracion(t)
	pacienteID := crearPacientePruebaDerivacion(t, pool)
	medicoGenID := crearMedicoPruebaDerivacion(t, pool, "MED-GEN")
	medicoOrlID := crearMedicoPruebaDerivacion(t, pool, "OTORRINO")
	registrarLimpiezaCitasPruebaDerivacion(t, pool, pacienteID)
	citaOrigenID := crearCitaPruebaDerivacion(
		t, pool, pacienteID, medicoGenID, baseHoraPruebaDerivacion(3), "atendida",
	)
	especialidadID := especialidadIDPruebaDerivacion(t, pool, "OTORRINO")
	derivacionStore := NewDerivacionStore(pool)
	citaStore := NewCitaStore(pool)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	ahora := time.Now().UTC().Truncate(time.Second)
	derivacion := &entities.Derivacion{
		PacienteID:            pacienteID,
		MedicoOrigenID:        medicoGenID,
		CitaOrigenID:          citaOrigenID,
		EspecialidadDestinoID: especialidadID,
		MedicoDestinoID:       &medicoOrlID,
		Motivo:                "Cancelación de especialista",
		Prioridad:             entities.PrioridadDerivacionNormal,
		Estado:                entities.EstadoDerivacionPendiente,
		VigenteDesde:          ahora,
		VigenteHasta:          ahora.AddDate(0, 0, 90),
	}
	if err := derivacionStore.Crear(ctx, derivacion); err != nil {
		t.Fatalf("crear derivación: %v", err)
	}

	cita := &entities.Cita{
		PacienteID:        pacienteID,
		MedicoID:          medicoOrlID,
		FechaHoraInicio:   baseHoraPruebaDerivacion(20),
		DuracionMin:       30,
		Tipo:              entities.TipoCitaPrimeraVez,
		Estado:            entities.EstadoCitaProgramada,
		Canal:             entities.CanalCitaPortalPaciente,
		MotivoCancelacion: "cancelacion_paciente",
	}
	if err := citaStore.CrearConDerivacion(ctx, cita, derivacion.ID); err != nil {
		t.Fatalf("crear cita con derivación: %v", err)
	}
	if err := citaStore.CancelarConDerivacion(ctx, cita, true, ""); err != nil {
		t.Fatalf("cancelar y liberar derivación: %v", err)
	}
	liberada, err := derivacionStore.ObtenerPorID(ctx, derivacion.ID)
	if err != nil {
		t.Fatalf("obtener derivación liberada: %v", err)
	}
	if liberada.Estado != entities.EstadoDerivacionPendiente {
		t.Fatalf("se esperaba derivación pendiente, obtenida %s", liberada.Estado)
	}

	nuevaCita := &entities.Cita{
		PacienteID:        pacienteID,
		MedicoID:          medicoOrlID,
		FechaHoraInicio:   baseHoraPruebaDerivacion(21),
		DuracionMin:       30,
		Tipo:              entities.TipoCitaPrimeraVez,
		Estado:            entities.EstadoCitaProgramada,
		Canal:             entities.CanalCitaPortalPaciente,
		MotivoCancelacion: "cancelacion_tardia",
	}
	if err := citaStore.CrearConDerivacion(ctx, nuevaCita, derivacion.ID); err != nil {
		t.Fatalf("reutilizar derivación liberada: %v", err)
	}
	if err := citaStore.CancelarConDerivacion(ctx, nuevaCita, false, "cancelacion_tardia"); err != nil {
		t.Fatalf("cancelar y consumir derivación: %v", err)
	}
	consumida, err := derivacionStore.ObtenerPorID(ctx, derivacion.ID)
	if err != nil {
		t.Fatalf("obtener derivación consumida: %v", err)
	}
	if consumida.Estado != entities.EstadoDerivacionCancelada ||
		consumida.MotivoCancelacion != "cancelacion_tardia" {
		t.Fatalf("derivación consumida inesperada: %+v", consumida)
	}
}

func TestExcepcionDerivacionStoreRegistraAutorizacion(t *testing.T) {
	pool := poolIntegracion(t)
	pacienteID := crearPacientePruebaDerivacion(t, pool)
	medicoID := crearMedicoPruebaDerivacion(t, pool, "OTORRINO")
	registrarLimpiezaCitasPruebaDerivacion(t, pool, pacienteID)
	citaID := crearCitaPruebaDerivacion(
		t, pool, pacienteID, medicoID, baseHoraPruebaDerivacion(4), "programada",
	)
	store := NewExcepcionDerivacionStore(pool)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var usuarioID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM usuarios ORDER BY id LIMIT 1`).Scan(&usuarioID); err != nil {
		t.Fatalf("no se encontró usuario autorizador: %v", err)
	}
	especialidadID := especialidadIDPruebaDerivacion(t, pool, "OTORRINO")
	excepcion := &entities.ExcepcionDerivacion{
		CitaID:                 citaID,
		PacienteID:             pacienteID,
		EspecialidadID:         especialidadID,
		MedicoID:               medicoID,
		Motivo:                 "Urgencia operativa",
		AutorizadoPorUsuarioID: &usuarioID,
	}
	if err := store.Crear(ctx, excepcion); err != nil {
		t.Fatalf("crear excepción: %v", err)
	}
	obtenida, err := store.ObtenerPorCita(ctx, citaID)
	if err != nil {
		t.Fatalf("obtener excepción: %v", err)
	}
	if obtenida.ID != excepcion.ID || obtenida.Motivo != "Urgencia operativa" ||
		obtenida.AutorizadoPorUsuarioID == nil || *obtenida.AutorizadoPorUsuarioID != usuarioID {
		t.Fatalf("excepción inesperada: %+v", obtenida)
	}
}

func TestTransferenciaStoreCambiaResponsableYHistorial(t *testing.T) {
	pool := poolIntegracion(t)
	medicoA := crearMedicoPrueba(t, pool)
	medicoB := crearMedicoPrueba(t, pool)
	pacienteID := crearPacientePruebaDerivacion(t, pool)
	pacientes := NewPacienteStore(pool)
	transferencias := NewTransferenciaStore(pool)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	paciente, err := pacientes.ObtenerPorID(ctx, pacienteID)
	if err != nil {
		t.Fatalf("obtener paciente: %v", err)
	}
	if paciente.MedicoResponsableID != nil {
		t.Fatalf("el paciente no debía tener responsable inicial: %v", paciente.MedicoResponsableID)
	}

	transferencia, err := transferencias.Transferir(
		ctx, pacienteID, medicoA, "Asignación inicial", 1, paciente.Version,
	)
	if err != nil {
		t.Fatalf("transferir a médico A: %v", err)
	}
	if transferencia.MedicoNuevoID != medicoA || transferencia.MedicoAnteriorID != nil {
		t.Fatalf("transferencia inicial inesperada: %+v", transferencia)
	}

	paciente, err = pacientes.ObtenerPorID(ctx, pacienteID)
	if err != nil {
		t.Fatalf("obtener paciente tras transferencia: %v", err)
	}
	if paciente.MedicoResponsableID == nil || *paciente.MedicoResponsableID != medicoA {
		t.Fatalf("responsable A no persistido: %v", paciente.MedicoResponsableID)
	}

	segunda, err := transferencias.Transferir(
		ctx, pacienteID, medicoB, "Cambio de tratante", 1, paciente.Version,
	)
	if err != nil {
		t.Fatalf("transferir a médico B: %v", err)
	}
	if segunda.MedicoAnteriorID == nil || *segunda.MedicoAnteriorID != medicoA {
		t.Fatalf("médico anterior inesperado: %+v", segunda)
	}

	historial, err := transferencias.ListarTransferencias(ctx, pacienteID)
	if err != nil {
		t.Fatalf("listar historial: %v", err)
	}
	if len(historial) != 2 {
		t.Fatalf("se esperaban 2 transferencias, obtenidas %d", len(historial))
	}

	perteneceA, err := pacientes.PerteneceAMedico(ctx, pacienteID, medicoA)
	if err != nil {
		t.Fatalf("comprobar pertenencia A: %v", err)
	}
	perteneceB, err := pacientes.PerteneceAMedico(ctx, pacienteID, medicoB)
	if err != nil {
		t.Fatalf("comprobar pertenencia B: %v", err)
	}
	if perteneceA || !perteneceB {
		t.Fatalf("ownership por asignación inesperado: A=%v B=%v", perteneceA, perteneceB)
	}
}
