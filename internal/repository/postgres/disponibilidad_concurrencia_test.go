package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// poolIntegracion abre la base indicada por TEST_DATABASE_URL. La prueba se
// omite en la suite normal para no exigir PostgreSQL en cada `go test ./...`.
func poolIntegracion(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL no definido; prueba de concurrencia omitida")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("no se pudo crear el pool de integración: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("no se pudo conectar a TEST_DATABASE_URL: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func crearMedicoPrueba(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	sufijo := fmt.Sprintf("%d", time.Now().UnixNano())

	var personaID int64
	err := pool.QueryRow(ctx, `
		INSERT INTO personas
			(tipo_documento, numero_documento, nombres, apellidos, fecha_nacimiento, sexo)
		VALUES ('cedula', $1, 'Prueba', 'Disponibilidad', '1980-01-01', 'otro')
		RETURNING id`, "TEST-DISP-"+sufijo).Scan(&personaID)
	if err != nil {
		t.Fatalf("no se pudo crear la persona de prueba: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _ = pool.Exec(ctx, `DELETE FROM personas WHERE id = $1`, personaID)
	})

	var especialidadID int64
	err = pool.QueryRow(ctx, `
		SELECT id FROM especialidades WHERE codigo = 'MED-GEN'`).Scan(&especialidadID)
	if err != nil {
		t.Fatalf("no se encontró la especialidad MED-GEN: %v", err)
	}

	var medicoID int64
	err = pool.QueryRow(ctx, `
		INSERT INTO medicos (persona_id, especialidad_id, numero_licencia, tipo_profesional)
		VALUES ($1, $2, $3, 'medico')
		RETURNING id`, personaID, especialidadID, "TEST-DISP-"+sufijo).Scan(&medicoID)
	if err != nil {
		t.Fatalf("no se pudo crear el médico de prueba: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _ = pool.Exec(ctx, `DELETE FROM medicos WHERE id = $1`, medicoID)
	})

	return medicoID
}

func insertarHorario(
	ctx context.Context,
	tx pgx.Tx,
	medicoID int64,
	dia int16,
	horaInicio, horaFin, vigenteDesde string,
	vigenteHasta any,
	activo bool,
) (int64, error) {
	const q = `
		INSERT INTO horarios_profesional
			(medico_id, dia_semana, hora_inicio, hora_fin,
			 vigente_desde, vigente_hasta, activo)
		VALUES ($1, $2, $3::time, $4::time, $5::date, $6::date, $7)
		RETURNING id`
	var id int64
	err := tx.QueryRow(ctx, q,
		medicoID, dia, horaInicio, horaFin, vigenteDesde, vigenteHasta, activo,
	).Scan(&id)
	return id, err
}

func insertarBloqueo(
	ctx context.Context,
	tx pgx.Tx,
	medicoID int64,
	inicio, fin time.Time,
	motivo string,
) (int64, error) {
	const q = `
		INSERT INTO bloqueos_agenda (medico_id, inicio, fin, motivo)
		VALUES ($1, $2, $3, $4)
		RETURNING id`
	var id int64
	err := tx.QueryRow(ctx, q, medicoID, inicio, fin, motivo).Scan(&id)
	return id, err
}

func esConflictoExclusion(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23P01"
}

func TestHorarioConcurrenteNoPermiteSolape(t *testing.T) {
	pool := poolIntegracion(t)
	medicoID := crearMedicoPrueba(t, pool)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	tx1, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("no se pudo iniciar la primera transacción: %v", err)
	}
	defer func() { _ = tx1.Rollback(ctx) }()

	if _, err := insertarHorario(ctx, tx1, medicoID, 1, "09:00", "12:00", "2026-01-01", nil, true); err != nil {
		t.Fatalf("no se pudo insertar el primer horario: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		tx2, err := pool.Begin(ctx)
		if err != nil {
			errCh <- err
			return
		}
		defer func() { _ = tx2.Rollback(ctx) }()

		_, err = insertarHorario(ctx, tx2, medicoID, 1, "10:00", "13:00", "2026-01-01", nil, true)
		errCh <- err
	}()

	// La segunda inserción debe quedar esperando la resolución de la primera.
	// Si el planificador la retrasa hasta después del commit, la exclusión
	// sigue rechazándola por el registro ya confirmado.
	time.Sleep(300 * time.Millisecond)
	if err := tx1.Commit(ctx); err != nil {
		t.Fatalf("no se pudo confirmar el primer horario: %v", err)
	}

	if err := <-errCh; !esConflictoExclusion(err) {
		t.Fatalf("se esperaba conflicto de exclusión para el segundo horario, se obtuvo: %v", err)
	}

	var cantidad int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM horarios_profesional WHERE medico_id = $1`,
		medicoID,
	).Scan(&cantidad); err != nil {
		t.Fatalf("no se pudo contar los horarios: %v", err)
	}
	if cantidad != 1 {
		t.Fatalf("se esperaba un solo horario confirmado, se encontraron %d", cantidad)
	}
}

func TestBloqueoConcurrenteNoPermiteSolape(t *testing.T) {
	pool := poolIntegracion(t)
	medicoID := crearMedicoPrueba(t, pool)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	inicio := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	tx1, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("no se pudo iniciar la primera transacción: %v", err)
	}
	defer func() { _ = tx1.Rollback(ctx) }()

	if _, err := insertarBloqueo(ctx, tx1, medicoID, inicio, inicio.Add(2*time.Hour), "primero"); err != nil {
		t.Fatalf("no se pudo insertar el primer bloqueo: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		tx2, err := pool.Begin(ctx)
		if err != nil {
			errCh <- err
			return
		}
		defer func() { _ = tx2.Rollback(ctx) }()

		_, err = insertarBloqueo(ctx, tx2, medicoID, inicio.Add(time.Hour), inicio.Add(3*time.Hour), "segundo")
		errCh <- err
	}()

	time.Sleep(300 * time.Millisecond)
	if err := tx1.Commit(ctx); err != nil {
		t.Fatalf("no se pudo confirmar el primer bloqueo: %v", err)
	}

	if err := <-errCh; !esConflictoExclusion(err) {
		t.Fatalf("se esperaba conflicto de exclusión para el segundo bloqueo, se obtuvo: %v", err)
	}

	var cantidad int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM bloqueos_agenda WHERE medico_id = $1`,
		medicoID,
	).Scan(&cantidad); err != nil {
		t.Fatalf("no se pudo contar los bloqueos: %v", err)
	}
	if cantidad != 1 {
		t.Fatalf("se esperaba un solo bloqueo confirmado, se encontraron %d", cantidad)
	}
}

func TestHorariosCompatiblesSePermiten(t *testing.T) {
	pool := poolIntegracion(t)
	medicoID := crearMedicoPrueba(t, pool)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("no se pudo iniciar la transacción: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// La vigencia base termina en 2026 para poder probar una vigencia futura
	// disjunta. Un NULL en vigente_hasta significa vigencia indefinida.
	if _, err := insertarHorario(ctx, tx, medicoID, 1, "09:00", "12:00", "2026-01-01", "2026-12-31", true); err != nil {
		t.Fatalf("no se pudo insertar el horario base: %v", err)
	}

	casos := []struct {
		nombre      string
		dia         int16
		inicio, fin string
		desde       string
		hasta       any
		activo      bool
	}{
		{"adyacente", 1, "12:00", "13:00", "2026-01-01", "2026-12-31", true},
		{"otro día", 2, "09:00", "12:00", "2026-01-01", "2026-12-31", true},
		{"vigencia futura", 1, "09:00", "12:00", "2027-01-01", nil, true},
		{"vigencia pasada", 1, "09:00", "12:00", "2025-01-01", "2025-12-31", true},
		{"inactivo", 1, "10:00", "11:00", "2026-01-01", nil, false},
	}
	for _, caso := range casos {
		if _, err := insertarHorario(
			ctx, tx, medicoID, caso.dia, caso.inicio, caso.fin,
			caso.desde, caso.hasta, caso.activo,
		); err != nil {
			t.Fatalf("el horario %q debía permitirse: %v", caso.nombre, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("no se pudo confirmar los horarios compatibles: %v", err)
	}

	var cantidad int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM horarios_profesional WHERE medico_id = $1`,
		medicoID,
	).Scan(&cantidad); err != nil {
		t.Fatalf("no se pudo contar los horarios: %v", err)
	}
	if cantidad != len(casos)+1 {
		t.Fatalf("se esperaban %d horarios, se encontraron %d", len(casos)+1, cantidad)
	}
}

func TestBloqueosCompatiblesSePermiten(t *testing.T) {
	pool := poolIntegracion(t)
	medicoID := crearMedicoPrueba(t, pool)
	otroMedicoID := crearMedicoPrueba(t, pool)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	inicio := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("no se pudo iniciar la transacción: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := insertarBloqueo(ctx, tx, medicoID, inicio, inicio.Add(2*time.Hour), "base"); err != nil {
		t.Fatalf("no se pudo insertar el bloqueo base: %v", err)
	}
	if _, err := insertarBloqueo(ctx, tx, medicoID, inicio.Add(2*time.Hour), inicio.Add(4*time.Hour), "adyacente"); err != nil {
		t.Fatalf("el bloqueo adyacente debía permitirse: %v", err)
	}
	if _, err := insertarBloqueo(ctx, tx, otroMedicoID, inicio, inicio.Add(2*time.Hour), "otro médico"); err != nil {
		t.Fatalf("el bloqueo de otro médico debía permitirse: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("no se pudieron confirmar los bloqueos compatibles: %v", err)
	}

	var cantidad int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM bloqueos_agenda WHERE medico_id IN ($1, $2)`,
		medicoID, otroMedicoID,
	).Scan(&cantidad); err != nil {
		t.Fatalf("no se pudo contar los bloqueos: %v", err)
	}
	if cantidad != 3 {
		t.Fatalf("se esperaban 3 bloqueos, se encontraron %d", cantidad)
	}
}
