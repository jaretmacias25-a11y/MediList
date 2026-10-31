package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func crearUsuarioPruebaActor(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	sufijo := sufijoPruebaCuenta()
	var usuarioID int64
	err := pool.QueryRow(ctx, `
		INSERT INTO usuarios
			(username, email, password_hash, nombres, apellidos, activo, email_verificado_en)
		VALUES ($1, $2, 'hash-de-prueba', 'Actor', 'Prueba', TRUE, now())
		RETURNING id`,
		"actor-"+sufijo, "actor-"+sufijo+"@example.test",
	).Scan(&usuarioID)
	if err != nil {
		t.Fatalf("crear usuario de prueba de actor: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _ = pool.Exec(ctx, `DELETE FROM usuarios WHERE id = $1`, usuarioID)
	})
	return usuarioID
}

func TestActorStoreResuelveVinculosActivosYOwnership(t *testing.T) {
	pool := poolIntegracion(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	usuarioID := crearUsuarioPruebaActor(t, pool)
	pacienteID := crearPacientePruebaDerivacion(t, pool)
	medicoID := crearMedicoPruebaDerivacion(t, pool, "MED-GEN")
	registrarLimpiezaCitasPruebaDerivacion(t, pool, pacienteID)

	if _, err := pool.Exec(ctx, `UPDATE pacientes SET usuario_id = $2 WHERE id = $1`, pacienteID, usuarioID); err != nil {
		t.Fatalf("vincular paciente: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE medicos SET usuario_id = $2 WHERE id = $1`, medicoID, usuarioID); err != nil {
		t.Fatalf("vincular médico: %v", err)
	}

	store := NewActorStore(pool)
	vinculos, err := store.ResolverVinculos(ctx, usuarioID)
	if err != nil {
		t.Fatalf("resolver vínculos: %v", err)
	}
	if vinculos.PacienteID == nil || *vinculos.PacienteID != pacienteID {
		t.Fatalf("paciente vinculado inesperado: %+v", vinculos.PacienteID)
	}
	if vinculos.MedicoID == nil || *vinculos.MedicoID != medicoID {
		t.Fatalf("médico vinculado inesperado: %+v", vinculos.MedicoID)
	}
	if !vinculos.EmailVerificado {
		t.Fatal("el usuario de prueba debe tener el email verificado")
	}

	citaID := crearCitaPruebaDerivacion(t, pool, pacienteID, medicoID, baseHoraPruebaDerivacion(5), "programada")
	pertenece, err := NewPacienteStore(pool).PerteneceAMedico(ctx, pacienteID, medicoID)
	if err != nil {
		t.Fatalf("verificar relación de atención: %v", err)
	}
	if !pertenece {
		t.Fatal("una cita programada debe crear relación de atención")
	}
	if _, err := pool.Exec(ctx, `UPDATE citas SET estado = 'cancelada' WHERE id = $1`, citaID); err != nil {
		t.Fatalf("cancelar cita de prueba: %v", err)
	}
	pertenece, err = NewPacienteStore(pool).PerteneceAMedico(ctx, pacienteID, medicoID)
	if err != nil {
		t.Fatalf("verificar relación tras cancelación: %v", err)
	}
	if pertenece {
		t.Fatal("una cita cancelada no debe habilitar el acceso al paciente")
	}

	if _, err := pool.Exec(ctx, `UPDATE pacientes SET activo = FALSE WHERE id = $1`, pacienteID); err != nil {
		t.Fatalf("desactivar paciente: %v", err)
	}
	vinculos, err = store.ResolverVinculos(ctx, usuarioID)
	if err != nil {
		t.Fatalf("resolver vínculos con paciente inactivo: %v", err)
	}
	if vinculos.PacienteID != nil {
		t.Fatalf("un paciente inactivo no debe resolverse como vínculo: %d", *vinculos.PacienteID)
	}
}

func TestMigracionRolMedicoSinPermisosGlobales(t *testing.T) {
	pool := poolIntegracion(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var globales int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM roles r
		JOIN rol_permisos rp ON rp.rol_id = r.id
		JOIN permisos p ON p.id = rp.permiso_id
		WHERE r.codigo = 'medico'
		  AND p.codigo IN ('pacientes:leer', 'citas:leer', 'citas:escribir')`,
	).Scan(&globales); err != nil {
		t.Fatalf("consultar permisos del rol médico: %v", err)
	}
	if globales != 0 {
		t.Fatalf("el rol médico conserva %d permisos globales", globales)
	}

	var propios int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM roles r
		JOIN rol_permisos rp ON rp.rol_id = r.id
		JOIN permisos p ON p.id = rp.permiso_id
		WHERE r.codigo = 'medico'
		  AND p.codigo IN (
			'medico:agenda:leer', 'medico:agenda:escribir',
			'pacientes:atendidos:leer', 'derivaciones:leer', 'derivaciones:escribir'
		  )`,
	).Scan(&propios); err != nil {
		t.Fatalf("consultar permisos propios del médico: %v", err)
	}
	if propios < 5 {
		t.Fatalf("faltan permisos propios del médico: %d", propios)
	}
}
