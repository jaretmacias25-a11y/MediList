package postgres

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// filaFalsa permite probar los scan sin una conexión real a PostgreSQL.
// Asigna cada valor al destino si el tipo coincide exactamente.
type filaFalsa struct {
	valores []any
}

func (f filaFalsa) Scan(dest ...any) error {
	if len(dest) != len(f.valores) {
		return fmt.Errorf("se esperaban %d destinos, se recibieron %d", len(f.valores), len(dest))
	}
	for i, d := range dest {
		dv := reflect.ValueOf(d)
		if dv.Kind() != reflect.Pointer || dv.IsNil() {
			return fmt.Errorf("el destino %d no es un puntero válido", i)
		}
		sv := reflect.ValueOf(f.valores[i])
		if !sv.IsValid() || !sv.Type().AssignableTo(dv.Elem().Type()) {
			return fmt.Errorf(
				"el valor %d de tipo %T no se puede asignar a %s",
				i, f.valores[i], dv.Elem().Type(),
			)
		}
		dv.Elem().Set(sv)
	}
	return nil
}

func TestScanUsuarioIncluyeCamposDeCuenta(t *testing.T) {
	bloqueado := time.Date(2026, 10, 24, 8, 0, 0, 0, time.UTC)
	verificado := time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)
	terminos := time.Date(2026, 10, 24, 9, 5, 0, 0, time.UTC)
	creado := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	actualizado := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)

	u, err := scanUsuario(filaFalsa{valores: []any{
		int64(7), "juan", "juan@example.com", "hash", "Juan", "Pérez",
		true, 3, 1,
		pgtype.Timestamptz{Time: bloqueado, Valid: true},
		pgtype.Timestamptz{},
		pgtype.Timestamptz{Time: verificado, Valid: true},
		pgtype.Timestamptz{Time: terminos, Valid: true},
		"2026-10", true,
		int64(4), creado, actualizado,
		[]string{"paciente"}, []string{"portal:perfil:leer"},
	}})
	if err != nil {
		t.Fatalf("scanUsuario devolvió error: %v", err)
	}
	if u.ID != 7 || u.Username != "juan" || !u.Activo {
		t.Fatalf("campos básicos mal mapeados: %+v", u)
	}
	if u.BloqueadoHasta == nil || !u.BloqueadoHasta.Equal(bloqueado) {
		t.Fatalf("bloqueado_hasta mal mapeado: %+v", u.BloqueadoHasta)
	}
	if u.UltimoAcceso != nil {
		t.Fatalf("ultimo_acceso debía ser nil: %+v", u.UltimoAcceso)
	}
	if u.EmailVerificadoEn == nil || !u.EmailVerificadoEn.Equal(verificado) {
		t.Fatalf("email_verificado_en mal mapeado: %+v", u.EmailVerificadoEn)
	}
	if !u.EmailVerificado() {
		t.Fatal("EmailVerificado debía ser true")
	}
	if u.TerminosAceptadosEn == nil || !u.TerminosAceptadosEn.Equal(terminos) {
		t.Fatalf("terminos_aceptados_en mal mapeado: %+v", u.TerminosAceptadosEn)
	}
	if u.VersionTerminos != "2026-10" || !u.DebeCambiarPassword {
		t.Fatalf("campos de cuenta mal mapeados: %+v", u)
	}
	if u.Version != 4 || !u.CreatedAt.Equal(creado) || !u.UpdatedAt.Equal(actualizado) {
		t.Fatalf("versión/fechas mal mapeadas: %+v", u)
	}
}

func valoresPaciente(usuarioID pgtype.Int8) []any {
	return []any{
		int64(11), usuarioID, pgtype.Int8{}, "HC-000011", "O", "+",
		"Contacto", "0999999999",
		true, int64(2),
		time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC),
		int64(21), "cedula", "0102030405", "Ana", "Gómez",
		time.Date(1990, 5, 12, 0, 0, 0, 0, time.UTC), "femenino",
		"0999999999", "ana@example.com", "Av. Principal",
	}
}

func TestScanPacienteVinculaUsuario(t *testing.T) {
	t.Run("con usuario", func(t *testing.T) {
		p, err := scanPaciente(filaFalsa{valores: valoresPaciente(pgtype.Int8{Int64: 7, Valid: true})})
		if err != nil {
			t.Fatalf("scanPaciente devolvió error: %v", err)
		}
		if p.UsuarioID == nil || *p.UsuarioID != 7 {
			t.Fatalf("usuario_id mal mapeado: %+v", p.UsuarioID)
		}
		if p.ID != 11 || p.Persona.ID != 21 || p.NumeroHistoria != "HC-000011" {
			t.Fatalf("campos del paciente mal mapeados: %+v", p)
		}
	})

	t.Run("sin usuario", func(t *testing.T) {
		p, err := scanPaciente(filaFalsa{valores: valoresPaciente(pgtype.Int8{})})
		if err != nil {
			t.Fatalf("scanPaciente devolvió error: %v", err)
		}
		if p.UsuarioID != nil {
			t.Fatalf("usuario_id debía ser nil: %+v", p.UsuarioID)
		}
	})
}

func TestScanMedicoIncluyePerfilYEspecialidad(t *testing.T) {
	creado := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	actualizado := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	m, err := scanMedico(filaFalsa{valores: []any{
		int64(31), pgtype.Int8{Int64: 7, Valid: true}, int64(5), "LIC-001", "medico",
		45, "Cardiólogo", "https://example.com/foto.jpg", true,
		true, int64(3), creado, actualizado,
		int64(41), "cedula", "0102030406", "Carlos", "Ruiz",
		time.Date(1980, 3, 20, 0, 0, 0, 0, time.UTC), "masculino",
		"0999999998", "carlos@example.com", "Av. Secundaria",
		int64(9), "CARD", "Cardiología", "Sistema cardiovascular",
		true, true, 60, 30, int64(4), creado, actualizado,
	}})
	if err != nil {
		t.Fatalf("scanMedico devolvió error: %v", err)
	}
	if m.UsuarioID == nil || *m.UsuarioID != 7 {
		t.Fatalf("usuario_id mal mapeado: %+v", m.UsuarioID)
	}
	if m.DuracionCitaMin != 45 || m.Bio != "Cardiólogo" || !m.AceptaPortal {
		t.Fatalf("perfil del médico mal mapeado: %+v", m)
	}
	if m.Especialidad.ID != 9 || m.Especialidad.RequiereDerivacion != true ||
		!m.Especialidad.Activa || m.Especialidad.Orden != 60 ||
		m.Especialidad.DuracionCitaMin != 30 || m.Especialidad.Version != 4 {
		t.Fatalf("especialidad anidada mal mapeada: %+v", m.Especialidad)
	}
}

func TestScanEspecialidadIncluyeAcceso(t *testing.T) {
	creado := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	actualizado := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	e, err := scanEspecialidad(filaFalsa{valores: []any{
		int64(9), "MED-GEN", "Medicina General", "Atención integral",
		false, true, 10, 45, int64(3), creado, actualizado,
	}})
	if err != nil {
		t.Fatalf("scanEspecialidad devolvió error: %v", err)
	}
	if e.RequiereDerivacion || !e.Activa || e.Orden != 10 || e.DuracionCitaMin != 45 {
		t.Fatalf("flags de especialidad mal mapeados: %+v", e)
	}
	if !e.AccesoDirecto() {
		t.Fatal("AccesoDirecto debía ser true")
	}
}
