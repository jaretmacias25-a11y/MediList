package postgres

import (
	"context"
	"testing"
	"time"

	"medilist/internal/service/entities"
)

func TestAgendaStoreCRUDYConsultasDeDisponibilidad(t *testing.T) {
	pool := poolIntegracion(t)
	medicoID := crearMedicoPrueba(t, pool)
	store := NewAgendaStore(pool)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	desde := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	horario := &entities.HorarioProfesional{
		MedicoID:        medicoID,
		DiaSemana:       1,
		HoraInicio:      "09:00",
		HoraFin:         "10:00",
		DuracionSlotMin: 30,
		VigenteDesde:    desde,
		Activo:          true,
	}
	if err := store.CrearHorario(ctx, horario); err != nil {
		t.Fatalf("crear horario: %v", err)
	}
	if horario.ID == 0 || horario.Version != 1 {
		t.Fatalf("horario creado sin identidad/versión: %+v", horario)
	}

	lista, err := store.ListarHorariosPorMedico(ctx, medicoID)
	if err != nil {
		t.Fatalf("listar horarios: %v", err)
	}
	if len(lista) != 1 {
		t.Fatalf("se esperaba un horario, obtenidos %d", len(lista))
	}

	vigentes, err := store.ListarHorariosVigentes(ctx, medicoID, desde, desde)
	if err != nil {
		t.Fatalf("listar horarios vigentes: %v", err)
	}
	if len(vigentes) != 1 {
		t.Fatalf("se esperaba un horario vigente, obtenidos %d", len(vigentes))
	}

	solapa, err := store.ExisteSolapamientoHorario(
		ctx, medicoID, 1, "09:30", "10:30", desde, nil, 0,
	)
	if err != nil {
		t.Fatalf("comprobar solape de horario: %v", err)
	}
	if !solapa {
		t.Fatal("se esperaba solape de horario")
	}
	solapa, err = store.ExisteSolapamientoHorario(
		ctx, medicoID, 2, "09:30", "10:30", desde, nil, 0,
	)
	if err != nil {
		t.Fatalf("comprobar horario sin solape: %v", err)
	}
	if solapa {
		t.Fatal("otro día no debe considerarse solape")
	}

	vigenteHasta := desde.AddDate(0, 0, 30)
	horario.HoraFin = "11:00"
	horario.VigenteHasta = &vigenteHasta
	if err := store.ActualizarHorario(ctx, horario); err != nil {
		t.Fatalf("actualizar horario: %v", err)
	}
	if horario.Version != 2 {
		t.Fatalf("se esperaba versión 2, obtenida %d", horario.Version)
	}
	obtenido, err := store.ObtenerHorarioPorID(ctx, medicoID, horario.ID)
	if err != nil {
		t.Fatalf("obtener horario: %v", err)
	}
	if obtenido.HoraFin != "11:00" {
		t.Fatalf("hora fin no actualizada: %s", obtenido.HoraFin)
	}
	if obtenido.VigenteHasta == nil || !obtenido.VigenteHasta.Equal(vigenteHasta) {
		t.Fatalf("vigente_hasta no actualizada: %v", obtenido.VigenteHasta)
	}

	var usuarioID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM usuarios ORDER BY id LIMIT 1`).Scan(&usuarioID); err != nil {
		t.Fatalf("no se encontró un usuario para creado_por: %v", err)
	}
	bloqueo := &entities.BloqueoAgenda{
		MedicoID:  medicoID,
		Inicio:    time.Date(2026, 11, 2, 14, 30, 0, 0, time.UTC),
		Fin:       time.Date(2026, 11, 2, 15, 0, 0, 0, time.UTC),
		Motivo:    "Prueba de integración",
		CreadoPor: &usuarioID,
	}
	if err := store.CrearBloqueo(ctx, bloqueo); err != nil {
		t.Fatalf("crear bloqueo: %v", err)
	}
	bloqueos, err := store.ListarBloqueosRango(
		ctx, medicoID,
		time.Date(2026, 11, 2, 14, 0, 0, 0, time.UTC),
		time.Date(2026, 11, 2, 16, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("listar bloqueos: %v", err)
	}
	if len(bloqueos) != 1 {
		t.Fatalf("se esperaba un bloqueo, obtenidos %d", len(bloqueos))
	}
	solapaBloqueo, err := store.ExisteSolapamientoBloqueo(
		ctx, medicoID,
		time.Date(2026, 11, 2, 14, 45, 0, 0, time.UTC),
		time.Date(2026, 11, 2, 15, 15, 0, 0, time.UTC),
		0,
	)
	if err != nil {
		t.Fatalf("comprobar solape de bloqueo: %v", err)
	}
	if !solapaBloqueo {
		t.Fatal("se esperaba solape de bloqueo")
	}

	bloqueo.Inicio = time.Date(2026, 11, 2, 15, 0, 0, 0, time.UTC)
	bloqueo.Fin = time.Date(2026, 11, 2, 15, 30, 0, 0, time.UTC)
	bloqueo.Motivo = "Bloqueo actualizado"
	if err := store.ActualizarBloqueo(ctx, bloqueo); err != nil {
		t.Fatalf("actualizar bloqueo: %v", err)
	}
	if bloqueo.CreadoPor == nil || *bloqueo.CreadoPor != usuarioID {
		t.Fatalf("creado_por no preservado al actualizar: %v", bloqueo.CreadoPor)
	}

	pacienteID := crearPacientePruebaDerivacion(t, pool)
	citaInicio := time.Date(2026, 11, 9, 14, 0, 0, 0, time.UTC)
	var citaID int64
	err = pool.QueryRow(ctx, `
		INSERT INTO citas
			(paciente_id, medico_id, fecha_hora_inicio, duracion_min, tipo, motivo, estado)
		VALUES ($1, $2, $3, 30, 'primera_vez', 'Prueba de integración', 'programada')
		RETURNING id`,
		pacienteID, medicoID, citaInicio,
	).Scan(&citaID)
	if err != nil {
		t.Fatalf("crear cita de prueba: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _ = pool.Exec(ctx, `DELETE FROM citas WHERE id = $1`, citaID)
	})

	citas, err := store.ListarCitasActivasRango(
		ctx, medicoID, citaInicio, citaInicio.Add(2*time.Hour),
	)
	if err != nil {
		t.Fatalf("listar citas activas: %v", err)
	}
	if len(citas) != 1 {
		t.Fatalf("se esperaba una cita activa, obtenidas %d", len(citas))
	}

	if _, err := pool.Exec(ctx, `UPDATE citas SET estado = 'cancelada' WHERE id = $1`, citaID); err != nil {
		t.Fatalf("cancelar cita de prueba: %v", err)
	}
	citas, err = store.ListarCitasActivasRango(
		ctx, medicoID, citaInicio, citaInicio.Add(2*time.Hour),
	)
	if err != nil {
		t.Fatalf("listar citas activas después de cancelar: %v", err)
	}
	if len(citas) != 0 {
		t.Fatalf("una cita cancelada no debe ocupar agenda, obtenidas %d", len(citas))
	}

	if err := store.EliminarBloqueo(ctx, medicoID, bloqueo.ID); err != nil {
		t.Fatalf("eliminar bloqueo: %v", err)
	}
	if err := store.DesactivarHorario(ctx, medicoID, horario.ID); err != nil {
		t.Fatalf("desactivar horario: %v", err)
	}
	vigentes, err = store.ListarHorariosVigentes(ctx, medicoID, desde, desde)
	if err != nil {
		t.Fatalf("listar horarios vigentes después de desactivar: %v", err)
	}
	if len(vigentes) != 0 {
		t.Fatalf("un horario inactivo no debe ser vigente, obtenidos %d", len(vigentes))
	}
}
