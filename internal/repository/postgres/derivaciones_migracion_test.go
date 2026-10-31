package postgres

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var secuenciaPruebaDerivacion atomic.Int64

func sufijoPruebaDerivacion() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), secuenciaPruebaDerivacion.Add(1))
}

func crearPacientePruebaDerivacion(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	sufijo := sufijoPruebaDerivacion()

	var personaID int64
	err := pool.QueryRow(ctx, `
		INSERT INTO personas
			(tipo_documento, numero_documento, nombres, apellidos, fecha_nacimiento, sexo)
		VALUES ('cedula', $1, 'Prueba', 'Derivación', '1990-01-01', 'otro')
		RETURNING id`, "TD-P-"+sufijo).Scan(&personaID)
	if err != nil {
		t.Fatalf("no se pudo crear la persona del paciente de prueba: %v", err)
	}

	var pacienteID int64
	err = pool.QueryRow(ctx, `
		INSERT INTO pacientes (persona_id)
		VALUES ($1)
		RETURNING id`, personaID).Scan(&pacienteID)
	if err != nil {
		t.Fatalf("no se pudo crear el paciente de prueba: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _ = pool.Exec(ctx, `DELETE FROM pacientes WHERE id = $1`, pacienteID)
		_, _ = pool.Exec(ctx, `DELETE FROM personas WHERE id = $1`, personaID)
	})

	return pacienteID
}

func crearMedicoPruebaDerivacion(t *testing.T, pool *pgxpool.Pool, codigoEspecialidad string) int64 {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	sufijo := sufijoPruebaDerivacion()

	var personaID int64
	err := pool.QueryRow(ctx, `
		INSERT INTO personas
			(tipo_documento, numero_documento, nombres, apellidos, fecha_nacimiento, sexo)
		VALUES ('cedula', $1, 'Prueba', 'Médico derivación', '1980-01-01', 'otro')
		RETURNING id`, "TD-M-"+sufijo).Scan(&personaID)
	if err != nil {
		t.Fatalf("no se pudo crear la persona del médico de prueba: %v", err)
	}

	var especialidadID int64
	err = pool.QueryRow(ctx, `
		SELECT id FROM especialidades WHERE codigo = $1`, codigoEspecialidad).Scan(&especialidadID)
	if err != nil {
		t.Fatalf("no se encontró la especialidad %s: %v", codigoEspecialidad, err)
	}

	var medicoID int64
	err = pool.QueryRow(ctx, `
		INSERT INTO medicos (persona_id, especialidad_id, numero_licencia, tipo_profesional)
		VALUES ($1, $2, $3, 'medico')
		RETURNING id`, personaID, especialidadID, "TD-L-"+sufijo).Scan(&medicoID)
	if err != nil {
		t.Fatalf("no se pudo crear el médico de prueba: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _ = pool.Exec(ctx, `DELETE FROM medicos WHERE id = $1`, medicoID)
		_, _ = pool.Exec(ctx, `DELETE FROM personas WHERE id = $1`, personaID)
	})

	return medicoID
}

func especialidadIDPruebaDerivacion(t *testing.T, pool *pgxpool.Pool, codigo string) int64 {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var id int64
	if err := pool.QueryRow(ctx,
		`SELECT id FROM especialidades WHERE codigo = $1`, codigo,
	).Scan(&id); err != nil {
		t.Fatalf("no se encontró la especialidad %s: %v", codigo, err)
	}
	return id
}

// registrarLimpiezaCitasPruebaDerivacion debe registrarse después de crear los
// pacientes y médicos para que, por orden LIFO, las citas y derivaciones se
// eliminen antes de los perfiles que referencian.
func registrarLimpiezaCitasPruebaDerivacion(t *testing.T, pool *pgxpool.Pool, pacienteID int64) {
	t.Helper()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _ = pool.Exec(ctx, `DELETE FROM derivaciones WHERE paciente_id = $1`, pacienteID)
		_, _ = pool.Exec(ctx, `DELETE FROM citas WHERE paciente_id = $1`, pacienteID)
	})
}

func baseHoraPruebaDerivacion(indice int) time.Time {
	return time.Now().UTC().
		Add(72 * time.Hour).
		Truncate(time.Minute).
		Add(time.Duration(indice) * time.Hour)
}

func crearCitaPruebaDerivacion(
	t *testing.T,
	pool *pgxpool.Pool,
	pacienteID, medicoID int64,
	inicio time.Time,
	estado string,
) int64 {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var id int64
	err := pool.QueryRow(ctx, `
		INSERT INTO citas
			(paciente_id, medico_id, fecha_hora_inicio, duracion_min, estado, motivo)
		VALUES ($1, $2, $3, 30, $4, 'prueba de derivación')
		RETURNING id`, pacienteID, medicoID, inicio, estado).Scan(&id)
	if err != nil {
		t.Fatalf("no se pudo crear la cita de prueba: %v", err)
	}
	return id
}

const insertarCitaConDerivacionSQL = `
	INSERT INTO citas
		(paciente_id, medico_id, fecha_hora_inicio, duracion_min, estado, derivacion_id, motivo)
	VALUES ($1, $2, $3, 30, $4, $5, 'prueba de derivación')
	RETURNING id`

func insertarCitaConDerivacionPrueba(
	ctx context.Context,
	pool *pgxpool.Pool,
	pacienteID, medicoID int64,
	inicio time.Time,
	estado string,
	derivacionID int64,
) error {
	var id int64
	return pool.QueryRow(ctx, insertarCitaConDerivacionSQL,
		pacienteID, medicoID, inicio, estado, derivacionID,
	).Scan(&id)
}

func crearCitaConDerivacionPrueba(
	t *testing.T,
	pool *pgxpool.Pool,
	pacienteID, medicoID int64,
	inicio time.Time,
	estado string,
	derivacionID int64,
) int64 {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := insertarCitaConDerivacionPrueba(
		ctx, pool, pacienteID, medicoID, inicio, estado, derivacionID,
	); err != nil {
		t.Fatalf("no se pudo crear la cita con derivación: %v", err)
	}

	var id int64
	if err := pool.QueryRow(ctx,
		`SELECT id FROM citas WHERE paciente_id = $1 AND medico_id = $2 AND fecha_hora_inicio = $3`,
		pacienteID, medicoID, inicio,
	).Scan(&id); err != nil {
		t.Fatalf("no se pudo recuperar la cita con derivación: %v", err)
	}
	return id
}

const insertarDerivacionSQL = `
	INSERT INTO derivaciones
		(paciente_id, medico_origen_id, cita_origen_id,
		 especialidad_destino_id, medico_destino_id, motivo, vigente_hasta)
	VALUES ($1, $2, $3, $4, $5, 'Derivación de prueba', now() + interval '90 days')
	RETURNING id`

func insertarDerivacionPrueba(
	ctx context.Context,
	pool *pgxpool.Pool,
	pacienteID, medicoOrigenID, citaOrigenID, especialidadDestinoID int64,
	medicoDestinoID any,
) error {
	var id int64
	return pool.QueryRow(ctx, insertarDerivacionSQL,
		pacienteID, medicoOrigenID, citaOrigenID, especialidadDestinoID, medicoDestinoID,
	).Scan(&id)
}

func crearDerivacionPrueba(
	t *testing.T,
	pool *pgxpool.Pool,
	pacienteID, medicoOrigenID, citaOrigenID, especialidadDestinoID int64,
	medicoDestinoID any,
) int64 {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := insertarDerivacionPrueba(
		ctx, pool, pacienteID, medicoOrigenID, citaOrigenID, especialidadDestinoID, medicoDestinoID,
	); err != nil {
		t.Fatalf("no se pudo crear la derivación de prueba: %v", err)
	}

	var id int64
	if err := pool.QueryRow(ctx, `
		SELECT id
		FROM derivaciones
		WHERE paciente_id = $1
		  AND cita_origen_id = $2
		  AND especialidad_destino_id = $3
		ORDER BY id DESC
		LIMIT 1`, pacienteID, citaOrigenID, especialidadDestinoID,
	).Scan(&id); err != nil {
		t.Fatalf("no se pudo recuperar la derivación de prueba: %v", err)
	}
	return id
}

func esErrorValidacion(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23514"
}

func esConflictoUnico(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func TestDerivacionValidaYRechazaOrigenInvalido(t *testing.T) {
	pool := poolIntegracion(t)

	pacienteID := crearPacientePruebaDerivacion(t, pool)
	otroPacienteID := crearPacientePruebaDerivacion(t, pool)
	medicoGenID := crearMedicoPruebaDerivacion(t, pool, "MED-GEN")
	medicoOtorrinoID := crearMedicoPruebaDerivacion(t, pool, "OTORRINO")
	registrarLimpiezaCitasPruebaDerivacion(t, pool, pacienteID)
	registrarLimpiezaCitasPruebaDerivacion(t, pool, otroPacienteID)

	especialidadOtorrinoID := especialidadIDPruebaDerivacion(t, pool, "OTORRINO")
	especialidadMedGenID := especialidadIDPruebaDerivacion(t, pool, "MED-GEN")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	citaOrigenID := crearCitaPruebaDerivacion(t, pool, pacienteID, medicoGenID, baseHoraPruebaDerivacion(0), "programada")
	if _, err := pool.Exec(ctx,
		`UPDATE citas SET estado = 'atendida' WHERE id = $1`, citaOrigenID,
	); err != nil {
		t.Fatalf("no se pudo marcar la cita de origen como atendida: %v", err)
	}

	citaNoAtendidaID := crearCitaPruebaDerivacion(t, pool, pacienteID, medicoGenID, baseHoraPruebaDerivacion(1), "programada")

	derivacionID := crearDerivacionPrueba(
		t, pool, pacienteID, medicoGenID, citaOrigenID,
		especialidadOtorrinoID, medicoOtorrinoID,
	)

	var estado string
	var version int64
	if err := pool.QueryRow(ctx,
		`SELECT estado, version FROM derivaciones WHERE id = $1`, derivacionID,
	).Scan(&estado, &version); err != nil {
		t.Fatalf("no se pudo consultar la derivación creada: %v", err)
	}
	if estado != "pendiente" || version != 1 {
		t.Fatalf("la derivación debía iniciar pendiente y versión 1; se obtuvo estado=%q versión=%d", estado, version)
	}

	casos := []struct {
		nombre          string
		pacienteID      int64
		medicoOrigenID  int64
		citaOrigenID    int64
		especialidadID  int64
		medicoDestinoID any
	}{
		{
			nombre:          "cita de origen no atendida",
			pacienteID:      pacienteID,
			medicoOrigenID:  medicoGenID,
			citaOrigenID:    citaNoAtendidaID,
			especialidadID:  especialidadOtorrinoID,
			medicoDestinoID: nil,
		},
		{
			nombre:          "paciente distinto al de la cita",
			pacienteID:      otroPacienteID,
			medicoOrigenID:  medicoGenID,
			citaOrigenID:    citaOrigenID,
			especialidadID:  especialidadOtorrinoID,
			medicoDestinoID: nil,
		},
		{
			nombre:          "médico de origen distinto al de la cita",
			pacienteID:      pacienteID,
			medicoOrigenID:  medicoOtorrinoID,
			citaOrigenID:    citaOrigenID,
			especialidadID:  especialidadOtorrinoID,
			medicoDestinoID: nil,
		},
		{
			nombre:          "destino que no requiere derivación",
			pacienteID:      pacienteID,
			medicoOrigenID:  medicoGenID,
			citaOrigenID:    citaOrigenID,
			especialidadID:  especialidadMedGenID,
			medicoDestinoID: nil,
		},
		{
			nombre:          "médico destino de otra especialidad",
			pacienteID:      pacienteID,
			medicoOrigenID:  medicoGenID,
			citaOrigenID:    citaOrigenID,
			especialidadID:  especialidadOtorrinoID,
			medicoDestinoID: medicoGenID,
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			err := insertarDerivacionPrueba(
				ctx, pool,
				caso.pacienteID, caso.medicoOrigenID, caso.citaOrigenID,
				caso.especialidadID, caso.medicoDestinoID,
			)
			if !esErrorValidacion(err) {
				t.Fatalf("se esperaba error de validación (23514), se obtuvo: %v", err)
			}
		})
	}
}

func TestDerivacionUnaSolaCitaActiva(t *testing.T) {
	pool := poolIntegracion(t)

	pacienteID := crearPacientePruebaDerivacion(t, pool)
	medicoGenID := crearMedicoPruebaDerivacion(t, pool, "MED-GEN")
	medicoOtorrinoID := crearMedicoPruebaDerivacion(t, pool, "OTORRINO")
	registrarLimpiezaCitasPruebaDerivacion(t, pool, pacienteID)

	especialidadOtorrinoID := especialidadIDPruebaDerivacion(t, pool, "OTORRINO")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	citaOrigenID := crearCitaPruebaDerivacion(t, pool, pacienteID, medicoGenID, baseHoraPruebaDerivacion(0), "programada")
	if _, err := pool.Exec(ctx,
		`UPDATE citas SET estado = 'atendida' WHERE id = $1`, citaOrigenID,
	); err != nil {
		t.Fatalf("no se pudo marcar la cita de origen como atendida: %v", err)
	}

	derivacionID := crearDerivacionPrueba(
		t, pool, pacienteID, medicoGenID, citaOrigenID,
		especialidadOtorrinoID, nil,
	)

	citaActivaID := crearCitaConDerivacionPrueba(
		t, pool, pacienteID, medicoOtorrinoID,
		baseHoraPruebaDerivacion(10), "programada", derivacionID,
	)

	if _, err := pool.Exec(ctx,
		`UPDATE derivaciones SET estado = 'agendada' WHERE id = $1`, derivacionID,
	); err != nil {
		t.Fatalf("no se pudo marcar la derivación como agendada: %v", err)
	}

	if err := insertarCitaConDerivacionPrueba(
		ctx, pool, pacienteID, medicoOtorrinoID,
		baseHoraPruebaDerivacion(11), "programada", derivacionID,
	); !esConflictoUnico(err) {
		t.Fatalf("la segunda cita activa debía fallar con conflicto único (23505), se obtuvo: %v", err)
	}

	if _, err := pool.Exec(ctx,
		`UPDATE citas SET estado = 'cancelada' WHERE id = $1`, citaActivaID,
	); err != nil {
		t.Fatalf("no se pudo cancelar la primera cita: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE derivaciones SET estado = 'pendiente' WHERE id = $1`, derivacionID,
	); err != nil {
		t.Fatalf("no se pudo liberar la derivación: %v", err)
	}

	citaNuevaID := crearCitaConDerivacionPrueba(
		t, pool, pacienteID, medicoOtorrinoID,
		baseHoraPruebaDerivacion(11), "programada", derivacionID,
	)

	// Una cita atendida sigue consumiendo la derivación. La reprogramación de
	// estado no vuelve a disparar la validación de vínculo, pero el índice
	// parcial sí debe seguir bloqueando una segunda cita activa o atendida.
	if _, err := pool.Exec(ctx,
		`UPDATE citas SET estado = 'atendida' WHERE id = $1`, citaNuevaID,
	); err != nil {
		t.Fatalf("no se pudo marcar la nueva cita como atendida: %v", err)
	}
	if err := insertarCitaConDerivacionPrueba(
		ctx, pool, pacienteID, medicoOtorrinoID,
		baseHoraPruebaDerivacion(12), "programada", derivacionID,
	); !esConflictoUnico(err) {
		t.Fatalf("una cita atendida debía seguir consumiendo la derivación; se obtuvo: %v", err)
	}

	var cantidad int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM citas
		WHERE derivacion_id = $1
		  AND estado IN ('programada', 'confirmada', 'atendida')`,
		derivacionID,
	).Scan(&cantidad); err != nil {
		t.Fatalf("no se pudo contar las citas que consumen la derivación: %v", err)
	}
	if cantidad != 1 {
		t.Fatalf("se esperaba exactamente una cita consumiendo la derivación, se encontraron %d", cantidad)
	}
}

func TestDerivacionCitaValidaPacienteEspecialidadYEstado(t *testing.T) {
	pool := poolIntegracion(t)

	pacienteID := crearPacientePruebaDerivacion(t, pool)
	otroPacienteID := crearPacientePruebaDerivacion(t, pool)
	medicoGenID := crearMedicoPruebaDerivacion(t, pool, "MED-GEN")
	medicoOtorrinoID := crearMedicoPruebaDerivacion(t, pool, "OTORRINO")
	registrarLimpiezaCitasPruebaDerivacion(t, pool, pacienteID)
	registrarLimpiezaCitasPruebaDerivacion(t, pool, otroPacienteID)

	especialidadOtorrinoID := especialidadIDPruebaDerivacion(t, pool, "OTORRINO")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	citaOrigenID := crearCitaPruebaDerivacion(t, pool, pacienteID, medicoGenID, baseHoraPruebaDerivacion(0), "programada")
	if _, err := pool.Exec(ctx,
		`UPDATE citas SET estado = 'atendida' WHERE id = $1`, citaOrigenID,
	); err != nil {
		t.Fatalf("no se pudo marcar la cita de origen como atendida: %v", err)
	}

	derivacionID := crearDerivacionPrueba(
		t, pool, pacienteID, medicoGenID, citaOrigenID,
		especialidadOtorrinoID, nil,
	)

	if err := insertarCitaConDerivacionPrueba(
		ctx, pool, otroPacienteID, medicoOtorrinoID,
		baseHoraPruebaDerivacion(20), "programada", derivacionID,
	); !esErrorValidacion(err) {
		t.Fatalf("una cita de otro paciente debía rechazarse (23514), se obtuvo: %v", err)
	}

	if err := insertarCitaConDerivacionPrueba(
		ctx, pool, pacienteID, medicoGenID,
		baseHoraPruebaDerivacion(21), "programada", derivacionID,
	); !esErrorValidacion(err) {
		t.Fatalf("un médico de otra especialidad debía rechazarse (23514), se obtuvo: %v", err)
	}

	if _, err := pool.Exec(ctx,
		`UPDATE derivaciones SET estado = 'cancelada' WHERE id = $1`, derivacionID,
	); err != nil {
		t.Fatalf("no se pudo cancelar la derivación: %v", err)
	}
	if err := insertarCitaConDerivacionPrueba(
		ctx, pool, pacienteID, medicoOtorrinoID,
		baseHoraPruebaDerivacion(22), "programada", derivacionID,
	); !esErrorValidacion(err) {
		t.Fatalf("una derivación cancelada debía rechazarse (23514), se obtuvo: %v", err)
	}

	derivacionVencidaID := crearDerivacionPrueba(
		t, pool, pacienteID, medicoGenID, citaOrigenID,
		especialidadOtorrinoID, nil,
	)
	if _, err := pool.Exec(ctx, `
		UPDATE derivaciones
		SET vigente_desde = now() - interval '2 days',
		    vigente_hasta = now() - interval '1 day'
		WHERE id = $1`, derivacionVencidaID,
	); err != nil {
		t.Fatalf("no se pudo vencer la derivación de prueba: %v", err)
	}
	if err := insertarCitaConDerivacionPrueba(
		ctx, pool, pacienteID, medicoOtorrinoID,
		baseHoraPruebaDerivacion(23), "programada", derivacionVencidaID,
	); !esErrorValidacion(err) {
		t.Fatalf("una derivación vencida debía rechazarse (23514), se obtuvo: %v", err)
	}
}

func TestDerivacionConcurrenteSoloPermiteUnaCita(t *testing.T) {
	pool := poolIntegracion(t)

	pacienteID := crearPacientePruebaDerivacion(t, pool)
	medicoGenID := crearMedicoPruebaDerivacion(t, pool, "MED-GEN")
	medicoOtorrinoID := crearMedicoPruebaDerivacion(t, pool, "OTORRINO")
	registrarLimpiezaCitasPruebaDerivacion(t, pool, pacienteID)

	especialidadOtorrinoID := especialidadIDPruebaDerivacion(t, pool, "OTORRINO")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	citaOrigenID := crearCitaPruebaDerivacion(t, pool, pacienteID, medicoGenID, baseHoraPruebaDerivacion(0), "programada")
	if _, err := pool.Exec(ctx,
		`UPDATE citas SET estado = 'atendida' WHERE id = $1`, citaOrigenID,
	); err != nil {
		t.Fatalf("no se pudo marcar la cita de origen como atendida: %v", err)
	}

	derivacionID := crearDerivacionPrueba(
		t, pool, pacienteID, medicoGenID, citaOrigenID,
		especialidadOtorrinoID, nil,
	)

	tx1, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("no se pudo iniciar la primera transacción: %v", err)
	}
	defer func() { _ = tx1.Rollback(ctx) }()

	var citaID int64
	if err := tx1.QueryRow(ctx, insertarCitaConDerivacionSQL,
		pacienteID, medicoOtorrinoID, baseHoraPruebaDerivacion(50), "programada", derivacionID,
	).Scan(&citaID); err != nil {
		t.Fatalf("no se pudo insertar la primera cita: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		tx2, err := pool.Begin(ctx)
		if err != nil {
			errCh <- err
			return
		}
		defer func() { _ = tx2.Rollback(ctx) }()

		var id int64
		err = tx2.QueryRow(ctx, insertarCitaConDerivacionSQL,
			pacienteID, medicoOtorrinoID, baseHoraPruebaDerivacion(51), "programada", derivacionID,
		).Scan(&id)
		errCh <- err
	}()

	// La segunda inserción debe esperar a que la primera transacción resuelva.
	time.Sleep(300 * time.Millisecond)
	if err := tx1.Commit(ctx); err != nil {
		t.Fatalf("no se pudo confirmar la primera cita: %v", err)
	}

	if err := <-errCh; !esConflictoUnico(err) {
		t.Fatalf("la segunda reserva debía fallar con conflicto único (23505), se obtuvo: %v", err)
	}

	var cantidad int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM citas
		WHERE derivacion_id = $1
		  AND estado IN ('programada', 'confirmada', 'atendida')`,
		derivacionID,
	).Scan(&cantidad); err != nil {
		t.Fatalf("no se pudo contar las citas confirmadas: %v", err)
	}
	if cantidad != 1 {
		t.Fatalf("se esperaba exactamente una cita confirmada, se encontraron %d", cantidad)
	}
}

func TestNotaCitaSoloMedicoDeLaCita(t *testing.T) {
	pool := poolIntegracion(t)

	pacienteID := crearPacientePruebaDerivacion(t, pool)
	medicoGenID := crearMedicoPruebaDerivacion(t, pool, "MED-GEN")
	medicoOtorrinoID := crearMedicoPruebaDerivacion(t, pool, "OTORRINO")
	registrarLimpiezaCitasPruebaDerivacion(t, pool, pacienteID)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	citaID := crearCitaPruebaDerivacion(
		t, pool, pacienteID, medicoOtorrinoID,
		baseHoraPruebaDerivacion(30), "atendida",
	)

	var notaID int64
	err := pool.QueryRow(ctx, `
		INSERT INTO notas_cita (cita_id, medico_id, motivo, evaluacion, plan)
		VALUES ($1, $2, 'motivo', 'evaluación', 'plan')
		RETURNING id`, citaID, medicoGenID).Scan(&notaID)
	if !esErrorValidacion(err) {
		t.Fatalf("una nota de otro médico debía rechazarse (23514), se obtuvo: %v", err)
	}

	if err := pool.QueryRow(ctx, `
		INSERT INTO notas_cita (cita_id, medico_id, motivo, evaluacion, plan)
		VALUES ($1, $2, 'motivo', 'evaluación', 'plan')
		RETURNING id`, citaID, medicoOtorrinoID).Scan(&notaID); err != nil {
		t.Fatalf("la nota del médico de la cita debía crearse: %v", err)
	}

	err = pool.QueryRow(ctx, `
		INSERT INTO notas_cita (cita_id, medico_id, motivo, evaluacion, plan)
		VALUES ($1, $2, 'duplicada', '', '')
		RETURNING id`, citaID, medicoOtorrinoID).Scan(&notaID)
	if !esConflictoUnico(err) {
		t.Fatalf("una segunda nota para la misma cita debía fallar (23505), se obtuvo: %v", err)
	}

	var cantidad int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM notas_cita WHERE cita_id = $1`, citaID,
	).Scan(&cantidad); err != nil {
		t.Fatalf("no se pudo contar las notas: %v", err)
	}
	if cantidad != 1 {
		t.Fatalf("se esperaba una sola nota, se encontraron %d", cantidad)
	}
}

func TestCitasMetadataPorDefecto(t *testing.T) {
	pool := poolIntegracion(t)

	pacienteID := crearPacientePruebaDerivacion(t, pool)
	medicoGenID := crearMedicoPruebaDerivacion(t, pool, "MED-GEN")
	registrarLimpiezaCitasPruebaDerivacion(t, pool, pacienteID)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	citaID := crearCitaPruebaDerivacion(
		t, pool, pacienteID, medicoGenID,
		baseHoraPruebaDerivacion(40), "programada",
	)

	var canal, motivoCancelacion string
	var creadaPorUsuarioID, derivacionID int64
	if err := pool.QueryRow(ctx, `
		SELECT canal,
		       motivo_cancelacion,
		       COALESCE(creada_por_usuario_id, 0),
		       COALESCE(derivacion_id, 0)
		FROM citas
		WHERE id = $1`, citaID,
	).Scan(&canal, &motivoCancelacion, &creadaPorUsuarioID, &derivacionID); err != nil {
		t.Fatalf("no se pudo consultar la metadata de la cita: %v", err)
	}

	if canal != "recepcion" {
		t.Fatalf("el canal por defecto debía ser recepcion, se obtuvo %q", canal)
	}
	if motivoCancelacion != "" {
		t.Fatalf("el motivo de cancelación por defecto debía ser vacío, se obtuvo %q", motivoCancelacion)
	}
	if creadaPorUsuarioID != 0 {
		t.Fatalf("creada_por_usuario_id debía quedar NULL, se obtuvo %d", creadaPorUsuarioID)
	}
	if derivacionID != 0 {
		t.Fatalf("derivacion_id debía quedar NULL, se obtuvo %d", derivacionID)
	}
}
