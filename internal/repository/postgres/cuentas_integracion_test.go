package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"medilist/internal/service/entities"
)

var secuenciaPruebaCuenta atomic.Int64

func sufijoPruebaCuenta() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), secuenciaPruebaCuenta.Add(1))
}

func hashPruebaCuenta(texto string) string {
	suma := sha256.Sum256([]byte(texto))
	return hex.EncodeToString(suma[:])
}

func usuarioCuentaPrueba(sufijo string) *entities.Usuario {
	ahora := time.Now()
	return &entities.Usuario{
		Username:            "cu-" + sufijo,
		Email:               "cuenta-" + sufijo + "@example.test",
		PasswordHash:        "hash-de-prueba",
		Nombres:             "Cuenta",
		Apellidos:           "Prueba",
		Activo:              true,
		TerminosAceptadosEn: &ahora,
		VersionTerminos:     "test",
		Roles:               []string{"paciente"},
	}
}

func pacienteCuentaPrueba(sufijo string) *entities.Paciente {
	return &entities.Paciente{
		Persona: entities.Persona{
			TipoDocumento:   "CEDULA",
			NumeroDocumento: "DOC-" + sufijo,
			Nombres:         "Paciente",
			Apellidos:       "Prueba",
			FechaNacimiento: time.Date(1990, 1, 15, 0, 0, 0, 0, time.UTC),
			Sexo:            "otro",
			Email:           "paciente-" + sufijo + "@example.test",
		},
		Activo: true,
	}
}

func tokenCuentaPrueba(usuarioID int64, tipo entities.TipoTokenCuenta, expira time.Time, sufijo string) *entities.TokenCuenta {
	return &entities.TokenCuenta{
		UsuarioID: usuarioID,
		Tipo:      tipo,
		TokenHash: hashPruebaCuenta("token-" + sufijo),
		ExpiraEn:  expira,
	}
}

func TestCuentaStoreRegistroVerificaTokenUnSoloUso(t *testing.T) {
	pool := poolIntegracion(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store := NewCuentaStore(pool)
	sufijo := sufijoPruebaCuenta()
	usuario := usuarioCuentaPrueba(sufijo)
	paciente := pacienteCuentaPrueba(sufijo)
	token := tokenCuentaPrueba(0, entities.TokenVerificacionEmail, time.Now().Add(time.Hour), sufijo+"-verificacion")

	if err := store.RegistrarPacienteConCuenta(ctx, usuario, paciente, token, nil); err != nil {
		t.Fatalf("registro transaccional: %v", err)
	}
	if usuario.ID == 0 || paciente.ID == 0 || token.ID == 0 {
		t.Fatalf("el registro no devolvió identificadores: usuario=%d paciente=%d token=%d",
			usuario.ID, paciente.ID, token.ID)
	}

	var vinculado bool
	if err := pool.QueryRow(ctx, `
		SELECT usuario_id = $2
		FROM pacientes
		WHERE id = $1`, paciente.ID, usuario.ID).Scan(&vinculado); err != nil {
		t.Fatalf("consultar vínculo: %v", err)
	}
	if !vinculado {
		t.Fatal("el paciente debe quedar vinculado a la cuenta en la misma transacción")
	}

	verificado, err := store.VerificarEmail(ctx, token.TokenHash, time.Now(), nil)
	if err != nil {
		t.Fatalf("verificar token: %v", err)
	}
	if !verificado.EmailVerificado() {
		t.Fatal("el usuario debe quedar con email verificado")
	}

	if _, err := store.VerificarEmail(ctx, token.TokenHash, time.Now(), nil); err == nil {
		t.Fatal("un token de verificación no puede reutilizarse")
	}

	var usado bool
	if err := pool.QueryRow(ctx, `
		SELECT usado_en IS NOT NULL
		FROM tokens_cuenta
		WHERE id = $1`, token.ID).Scan(&usado); err != nil {
		t.Fatalf("consultar uso del token: %v", err)
	}
	if !usado {
		t.Fatal("el token debe quedar marcado como usado")
	}
}

func TestCuentaStoreDocumentoDuplicadoNoDuplicaPaciente(t *testing.T) {
	pool := poolIntegracion(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store := NewCuentaStore(pool)
	sufijo := sufijoPruebaCuenta()
	usuario1 := usuarioCuentaPrueba(sufijo + "-a")
	paciente1 := pacienteCuentaPrueba(sufijo)
	token1 := tokenCuentaPrueba(0, entities.TokenVerificacionEmail, time.Now().Add(time.Hour), sufijo+"-a")
	if err := store.RegistrarPacienteConCuenta(ctx, usuario1, paciente1, token1, nil); err != nil {
		t.Fatalf("primer registro: %v", err)
	}

	usuario2 := usuarioCuentaPrueba(sufijo + "-b")
	paciente2 := pacienteCuentaPrueba(sufijo)
	paciente2.Persona.Nombres = "Otra Persona"
	paciente2.Persona.Email = "otra-" + sufijo + "@example.test"
	token2 := tokenCuentaPrueba(0, entities.TokenVerificacionEmail, time.Now().Add(time.Hour), sufijo+"-b")

	err := store.RegistrarPacienteConCuenta(ctx, usuario2, paciente2, token2, nil)
	if !errors.Is(err, entities.ErrConflict) {
		t.Fatalf("se esperaba conflicto por documento duplicado, obtenido: %v", err)
	}

	var personas, pacientes int
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM personas WHERE numero_documento = $1),
			(SELECT count(*) FROM pacientes p
			 JOIN personas pe ON pe.id = p.persona_id
			 WHERE pe.numero_documento = $1)`,
		paciente1.Persona.NumeroDocumento,
	).Scan(&personas, &pacientes); err != nil {
		t.Fatalf("contar datos duplicados: %v", err)
	}
	if personas != 1 || pacientes != 1 {
		t.Fatalf("el documento debe tener una sola persona y un solo paciente: personas=%d pacientes=%d",
			personas, pacientes)
	}
}

func TestCuentaStoreRegistroConcurrenteMismoDocumento(t *testing.T) {
	pool := poolIntegracion(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store := NewCuentaStore(pool)
	sufijo := sufijoPruebaCuenta()
	documento := "CONC-" + sufijo

	resultados := make(chan error, 2)
	inicio := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(indice int) {
			defer wg.Done()
			usuario := usuarioCuentaPrueba(fmt.Sprintf("%s-%d", sufijo, indice))
			paciente := pacienteCuentaPrueba(fmt.Sprintf("%s-%d", sufijo, indice))
			paciente.Persona.NumeroDocumento = documento
			token := tokenCuentaPrueba(0, entities.TokenVerificacionEmail,
				time.Now().Add(time.Hour), fmt.Sprintf("%s-%d", sufijo, indice))

			<-inicio
			resultados <- store.RegistrarPacienteConCuenta(ctx, usuario, paciente, token, nil)
		}(i)
	}
	close(inicio)
	wg.Wait()
	close(resultados)

	exitos, conflictos := 0, 0
	for err := range resultados {
		switch {
		case err == nil:
			exitos++
		case errors.Is(err, entities.ErrConflict):
			conflictos++
		default:
			t.Fatalf("error inesperado en registro concurrente: %v", err)
		}
	}
	if exitos != 1 || conflictos != 1 {
		t.Fatalf("se esperaba un registro exitoso y un conflicto: exitos=%d conflictos=%d", exitos, conflictos)
	}

	var personas, pacientes int
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM personas WHERE numero_documento = $1),
			(SELECT count(*) FROM pacientes p
			 JOIN personas pe ON pe.id = p.persona_id
			 WHERE pe.numero_documento = $1)`, documento,
	).Scan(&personas, &pacientes); err != nil {
		t.Fatalf("contar carrera: %v", err)
	}
	if personas != 1 || pacientes != 1 {
		t.Fatalf("la carrera no debe duplicar personas ni pacientes: personas=%d pacientes=%d",
			personas, pacientes)
	}
}

func TestCuentaStoreTokenVencidoYReutilizado(t *testing.T) {
	pool := poolIntegracion(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store := NewCuentaStore(pool)
	sufijo := sufijoPruebaCuenta()
	usuario := usuarioCuentaPrueba(sufijo)
	paciente := pacienteCuentaPrueba(sufijo)
	vencido := tokenCuentaPrueba(0, entities.TokenVerificacionEmail, time.Now().Add(-time.Minute), sufijo+"-vencido")

	if err := store.RegistrarPacienteConCuenta(ctx, usuario, paciente, vencido, nil); err != nil {
		t.Fatalf("registro con token vencido: %v", err)
	}
	if _, err := store.VerificarEmail(ctx, vencido.TokenHash, time.Now(), nil); err == nil {
		t.Fatal("un token vencido no debe aceptarse")
	}

	var verificado bool
	if err := pool.QueryRow(ctx, `
		SELECT email_verificado_en IS NOT NULL
		FROM usuarios
		WHERE id = $1`, usuario.ID).Scan(&verificado); err != nil {
		t.Fatalf("consultar verificación: %v", err)
	}
	if verificado {
		t.Fatal("un token vencido no debe marcar el email como verificado")
	}

	nuevo := tokenCuentaPrueba(usuario.ID, entities.TokenVerificacionEmail,
		time.Now().Add(time.Hour), sufijo+"-nuevo")
	if err := store.ReemplazarToken(ctx, nuevo); err != nil {
		t.Fatalf("reemplazar token: %v", err)
	}
	if _, err := store.VerificarEmail(ctx, nuevo.TokenHash, time.Now(), nil); err != nil {
		t.Fatalf("verificar token nuevo: %v", err)
	}
	if _, err := store.VerificarEmail(ctx, nuevo.TokenHash, time.Now(), nil); err == nil {
		t.Fatal("un token nuevo tampoco puede reutilizarse")
	}
}

func TestCuentaStoreResetPasswordRevocaSesiones(t *testing.T) {
	pool := poolIntegracion(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store := NewCuentaStore(pool)
	sufijo := sufijoPruebaCuenta()
	usuario := usuarioCuentaPrueba(sufijo)
	paciente := pacienteCuentaPrueba(sufijo)
	tokenVerificacion := tokenCuentaPrueba(0, entities.TokenVerificacionEmail,
		time.Now().Add(time.Hour), sufijo+"-ver")
	if err := store.RegistrarPacienteConCuenta(ctx, usuario, paciente, tokenVerificacion, nil); err != nil {
		t.Fatalf("registrar usuario: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO refresh_tokens (usuario_id, token_hash, expira_en, token_version)
		VALUES ($1, $2, $3, 1)`,
		usuario.ID, hashPruebaCuenta("refresh-"+sufijo), time.Now().Add(24*time.Hour),
	); err != nil {
		t.Fatalf("crear refresh token de prueba: %v", err)
	}

	reset := tokenCuentaPrueba(usuario.ID, entities.TokenResetPassword,
		time.Now().Add(time.Hour), sufijo+"-reset")
	if err := store.ReemplazarToken(ctx, reset); err != nil {
		t.Fatalf("crear token de reset: %v", err)
	}

	actualizado, err := store.RestablecerPassword(ctx, reset.TokenHash, "nuevo-hash", time.Now(), nil)
	if err != nil {
		t.Fatalf("restablecer contraseña: %v", err)
	}
	if actualizado.PasswordHash != "nuevo-hash" {
		t.Fatalf("hash de contraseña inesperado: %q", actualizado.PasswordHash)
	}
	if !actualizado.EmailVerificado() {
		t.Fatal("el reset debe verificar el correo si aún no lo estaba")
	}
	if actualizado.TokenVersion < 1 {
		t.Fatalf("el reset debe incrementar token_version: %d", actualizado.TokenVersion)
	}

	var revocado bool
	if err := pool.QueryRow(ctx, `
		SELECT revocado_en IS NOT NULL
		FROM refresh_tokens
		WHERE usuario_id = $1`, usuario.ID).Scan(&revocado); err != nil {
		t.Fatalf("consultar revocación: %v", err)
	}
	if !revocado {
		t.Fatal("el reset debe revocar los refresh tokens activos")
	}

	if _, err := store.RestablecerPassword(ctx, reset.TokenHash, "otro-hash", time.Now(), nil); err == nil {
		t.Fatal("un token de reset ya usado no puede reutilizarse")
	}
}

func TestCuentaStoreSolicitudVinculacionAprobarYListar(t *testing.T) {
	pool := poolIntegracion(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pacienteStore := NewPacienteStore(pool)
	cuentaStore := NewCuentaStore(pool)
	sufijo := sufijoPruebaCuenta()
	legacy := pacienteCuentaPrueba(sufijo)
	if err := pacienteStore.Crear(ctx, legacy); err != nil {
		t.Fatalf("crear paciente histórico: %v", err)
	}

	usuario := usuarioCuentaPrueba(sufijo)
	token := tokenCuentaPrueba(0, entities.TokenVerificacionEmail,
		time.Now().Add(time.Hour), sufijo+"-pendiente")
	solicitud := &entities.SolicitudVinculacion{}
	if err := cuentaStore.RegistrarCuentaPendienteVinculacion(
		ctx, usuario, legacy.ID, token, solicitud, nil,
	); err != nil {
		t.Fatalf("crear solicitud de vinculación: %v", err)
	}

	var personas, pacientes int
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM personas WHERE numero_documento = $1),
			(SELECT count(*) FROM pacientes p
			 JOIN personas pe ON pe.id = p.persona_id
			 WHERE pe.numero_documento = $1)`, legacy.Persona.NumeroDocumento,
	).Scan(&personas, &pacientes); err != nil {
		t.Fatalf("contar historia existente: %v", err)
	}
	if personas != 1 || pacientes != 1 {
		t.Fatalf("la vinculación no debe duplicar la historia: personas=%d pacientes=%d",
			personas, pacientes)
	}

	lista, err := cuentaStore.ListarSolicitudesVinculacion(ctx, entities.SolicitudVinculacionFiltro{
		ListOptions: entities.ListOptions{Limit: 20, Search: legacy.Persona.NumeroDocumento},
		Estado:      entities.SolicitudVinculacionPendiente,
	})
	if err != nil {
		t.Fatalf("listar solicitudes: %v", err)
	}
	if len(lista) != 1 || lista[0].ID != solicitud.ID {
		t.Fatalf("se esperaba la solicitud %d en la lista, obtenidas %d", solicitud.ID, len(lista))
	}

	aprobada, err := cuentaStore.AprobarSolicitudVinculacion(ctx, solicitud.ID, 0, time.Now())
	if err != nil {
		t.Fatalf("aprobar solicitud: %v", err)
	}
	if aprobada.Estado != entities.SolicitudVinculacionAprobada {
		t.Fatalf("estado inesperado: %s", aprobada.Estado)
	}

	var vinculado bool
	if err := pool.QueryRow(ctx, `
		SELECT usuario_id = $2
		FROM pacientes
		WHERE id = $1`, legacy.ID, usuario.ID).Scan(&vinculado); err != nil {
		t.Fatalf("consultar vínculo aprobado: %v", err)
	}
	if !vinculado {
		t.Fatal("la aprobación debe vincular la cuenta con la historia clínica")
	}
}

func TestCuentaStoreRechazarSolicitudDesactivaCuenta(t *testing.T) {
	pool := poolIntegracion(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pacienteStore := NewPacienteStore(pool)
	cuentaStore := NewCuentaStore(pool)
	sufijo := sufijoPruebaCuenta()
	legacy := pacienteCuentaPrueba(sufijo)
	if err := pacienteStore.Crear(ctx, legacy); err != nil {
		t.Fatalf("crear paciente histórico: %v", err)
	}

	usuario := usuarioCuentaPrueba(sufijo)
	token := tokenCuentaPrueba(0, entities.TokenVerificacionEmail,
		time.Now().Add(time.Hour), sufijo+"-rechazo")
	solicitud := &entities.SolicitudVinculacion{}
	if err := cuentaStore.RegistrarCuentaPendienteVinculacion(
		ctx, usuario, legacy.ID, token, solicitud, nil,
	); err != nil {
		t.Fatalf("crear solicitud: %v", err)
	}

	rechazada, err := cuentaStore.RechazarSolicitudVinculacion(
		ctx, solicitud.ID, 0, "documento no coincide", time.Now(),
	)
	if err != nil {
		t.Fatalf("rechazar solicitud: %v", err)
	}
	if rechazada.Estado != entities.SolicitudVinculacionRechazada {
		t.Fatalf("estado inesperado: %s", rechazada.Estado)
	}

	var activo bool
	if err := pool.QueryRow(ctx, `SELECT activo FROM usuarios WHERE id = $1`, usuario.ID).Scan(&activo); err != nil {
		t.Fatalf("consultar usuario: %v", err)
	}
	if activo {
		t.Fatal("la cuenta pendiente rechazada debe quedar inactiva")
	}
}

func TestCuentaStoreVincularMedico(t *testing.T) {
	pool := poolIntegracion(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sufijo := sufijoPruebaCuenta()
	var especialidadID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM especialidades WHERE codigo = 'MED-GEN'`).Scan(&especialidadID); err != nil {
		t.Fatalf("buscar especialidad base: %v", err)
	}

	medicoStore := NewMedicoStore(pool)
	medico := &entities.Medico{
		Persona: entities.Persona{
			TipoDocumento:   "CEDULA",
			NumeroDocumento: "MED-" + sufijo,
			Nombres:         "Médico",
			Apellidos:       "Prueba",
			FechaNacimiento: time.Date(1980, 2, 2, 0, 0, 0, 0, time.UTC),
			Sexo:            "otro",
		},
		EspecialidadID:  especialidadID,
		NumeroLicencia:  "LIC-" + sufijo,
		TipoProfesional: "medico",
		DuracionCitaMin: 30,
		Activo:          true,
	}
	if err := medicoStore.Crear(ctx, medico); err != nil {
		t.Fatalf("crear médico: %v", err)
	}

	usuarioStore := NewUsuarioStore(pool)
	usuario := usuarioCuentaPrueba(sufijo + "-medico")
	usuario.Roles = []string{"medico"}
	if err := usuarioStore.Crear(ctx, usuario); err != nil {
		t.Fatalf("crear usuario médico: %v", err)
	}

	cuentaStore := NewCuentaStore(pool)
	if err := cuentaStore.VincularMedico(ctx, medico.ID, usuario.ID, time.Now()); err != nil {
		t.Fatalf("vincular médico: %v", err)
	}

	var vinculado bool
	if err := pool.QueryRow(ctx, `
		SELECT usuario_id = $2
		FROM medicos
		WHERE id = $1`, medico.ID, usuario.ID).Scan(&vinculado); err != nil {
		t.Fatalf("consultar vínculo de médico: %v", err)
	}
	if !vinculado {
		t.Fatal("el médico debe quedar vinculado a la cuenta")
	}
}
