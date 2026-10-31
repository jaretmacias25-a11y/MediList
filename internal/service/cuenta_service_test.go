package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"medilist/internal/service/entities"
)

type fakeCuentaStore struct {
	CuentaStore

	registrarPacienteFn  func(context.Context, *entities.Usuario, *entities.Paciente, *entities.TokenCuenta, *entities.Auditoria) error
	registrarPendienteFn func(context.Context, *entities.Usuario, int64, *entities.TokenCuenta, *entities.SolicitudVinculacion, *entities.Auditoria) error
	reemplazarTokenFn    func(context.Context, *entities.TokenCuenta) error
	verificarEmailFn     func(context.Context, string, time.Time, *entities.Auditoria) (*entities.Usuario, error)
	restablecerFn        func(context.Context, string, string, time.Time, *entities.Auditoria) (*entities.Usuario, error)
}

func (f *fakeCuentaStore) RegistrarPacienteConCuenta(
	ctx context.Context,
	usuario *entities.Usuario,
	paciente *entities.Paciente,
	token *entities.TokenCuenta,
	auditoria *entities.Auditoria,
) error {
	if f.registrarPacienteFn == nil {
		panic("RegistrarPacienteConCuenta no esperado")
	}
	return f.registrarPacienteFn(ctx, usuario, paciente, token, auditoria)
}

func (f *fakeCuentaStore) RegistrarCuentaPendienteVinculacion(
	ctx context.Context,
	usuario *entities.Usuario,
	pacienteID int64,
	token *entities.TokenCuenta,
	solicitud *entities.SolicitudVinculacion,
	auditoria *entities.Auditoria,
) error {
	if f.registrarPendienteFn == nil {
		panic("RegistrarCuentaPendienteVinculacion no esperado")
	}
	return f.registrarPendienteFn(ctx, usuario, pacienteID, token, solicitud, auditoria)
}

func (f *fakeCuentaStore) ReemplazarToken(ctx context.Context, token *entities.TokenCuenta) error {
	if f.reemplazarTokenFn == nil {
		panic("ReemplazarToken no esperado")
	}
	return f.reemplazarTokenFn(ctx, token)
}

func (f *fakeCuentaStore) VerificarEmail(
	ctx context.Context,
	tokenHash string,
	cuando time.Time,
	auditoria *entities.Auditoria,
) (*entities.Usuario, error) {
	if f.verificarEmailFn == nil {
		panic("VerificarEmail no esperado")
	}
	return f.verificarEmailFn(ctx, tokenHash, cuando, auditoria)
}

func (f *fakeCuentaStore) RestablecerPassword(
	ctx context.Context,
	tokenHash, passwordHash string,
	cuando time.Time,
	auditoria *entities.Auditoria,
) (*entities.Usuario, error) {
	if f.restablecerFn == nil {
		panic("RestablecerPassword no esperado")
	}
	return f.restablecerFn(ctx, tokenHash, passwordHash, cuando, auditoria)
}

type fakePacienteStoreCuenta struct {
	PacienteStore
	buscarFn func(context.Context, string, string) (*entities.Paciente, error)
}

func (f *fakePacienteStoreCuenta) BuscarPorDocumento(
	ctx context.Context,
	tipoDocumento, numeroDocumento string,
) (*entities.Paciente, error) {
	if f.buscarFn == nil {
		panic("BuscarPorDocumento no esperado")
	}
	return f.buscarFn(ctx, tipoDocumento, numeroDocumento)
}

type fakeUsuarioStoreCuenta struct {
	UsuarioStore
	buscarPorEmailFn func(context.Context, string) (*entities.Usuario, error)
}

func (f *fakeUsuarioStoreCuenta) BuscarPorEmail(ctx context.Context, email string) (*entities.Usuario, error) {
	if f.buscarPorEmailFn == nil {
		panic("BuscarPorEmail no esperado")
	}
	return f.buscarPorEmailFn(ctx, email)
}

type correoCuentaPrueba struct {
	destino string
	nombre  string
	enlace  string
	expira  time.Time
	tipo    string
}

type fakeAuditoriaStoreCuenta struct {
	AuditoriaStore
	entradas []entities.Auditoria
}

func (f *fakeAuditoriaStoreCuenta) Registrar(_ context.Context, a *entities.Auditoria) error {
	f.entradas = append(f.entradas, *a)
	return nil
}

type fakeEmailSenderCuenta struct {
	enviados []correoCuentaPrueba
}

func (f *fakeEmailSenderCuenta) EnviarVerificacion(
	_ context.Context,
	destino, nombre, enlace string,
	expira time.Time,
) error {
	f.enviados = append(f.enviados, correoCuentaPrueba{
		destino: destino, nombre: nombre, enlace: enlace, expira: expira, tipo: "verificacion",
	})
	return nil
}

func (f *fakeEmailSenderCuenta) EnviarResetPassword(
	_ context.Context,
	destino, nombre, enlace string,
	expira time.Time,
) error {
	f.enviados = append(f.enviados, correoCuentaPrueba{
		destino: destino, nombre: nombre, enlace: enlace, expira: expira, tipo: "reset",
	})
	return nil
}

func configCuentaPrueba() CuentaConfig {
	return CuentaConfig{
		TermsVersion:    "2026-10",
		MinEdadCuenta:   18,
		BcryptCost:      10,
		VerificationTTL: time.Hour,
		ResetTTL:        time.Hour,
		LinkBaseURL:     "http://localhost:3000",
	}
}

func registroCuentaPrueba() RegistroPacienteInput {
	return RegistroPacienteInput{
		Username:        "nueva.cuenta",
		Email:           "nueva@example.test",
		Password:        "ClaveSegura123!",
		Nombres:         "Nueva",
		Apellidos:       "Cuenta",
		TipoDocumento:   "CEDULA",
		NumeroDocumento: "DOC-123456",
		FechaNacimiento: time.Now().AddDate(-30, 0, 0),
		Sexo:            "otro",
		Telefono:        "0999999999",
		Direccion:       "Calle de prueba",
		AceptaTerminos:  true,
		VersionTerminos: "2026-10",
	}
}

func TestCuentaServiceRegistrarPacienteNuevoEnviaVerificacion(t *testing.T) {
	var usuarioRegistrado *entities.Usuario
	var pacienteRegistrado *entities.Paciente
	var tokenRegistrado *entities.TokenCuenta
	var auditoriaRegistrada *entities.Auditoria

	cuentas := &fakeCuentaStore{
		registrarPacienteFn: func(
			_ context.Context,
			usuario *entities.Usuario,
			paciente *entities.Paciente,
			token *entities.TokenCuenta,
			auditoria *entities.Auditoria,
		) error {
			usuario.ID = 10
			paciente.ID = 20
			token.ID = 30
			token.UsuarioID = usuario.ID
			usuarioRegistrado = usuario
			pacienteRegistrado = paciente
			tokenRegistrado = token
			auditoriaRegistrada = auditoria
			return nil
		},
	}
	pacientes := &fakePacienteStoreCuenta{
		buscarFn: func(context.Context, string, string) (*entities.Paciente, error) {
			return nil, entities.ErrNotFound
		},
	}
	emails := &fakeEmailSenderCuenta{}
	svc := NewCuentaService(configCuentaPrueba(), cuentas, pacientes,
		&fakeUsuarioStoreCuenta{}, emails, nil, nil)

	resultado, err := svc.RegistrarPaciente(
		context.Background(), registroCuentaPrueba(), "127.0.0.1", "prueba",
	)
	if err != nil {
		t.Fatalf("registro: %v", err)
	}
	if resultado.VinculacionPendiente {
		t.Fatal("un documento nuevo no debe quedar pendiente de vinculación")
	}
	if resultado.Usuario.ID != 10 || resultado.Paciente.ID != 20 {
		t.Fatalf("identificadores inesperados: usuario=%d paciente=%d",
			resultado.Usuario.ID, resultado.Paciente.ID)
	}
	if usuarioRegistrado == nil || pacienteRegistrado == nil || tokenRegistrado == nil {
		t.Fatal("el store debe recibir usuario, paciente y token")
	}
	if auditoriaRegistrada == nil || auditoriaRegistrada.EstadoHTTP != 201 {
		t.Fatalf("auditoría de registro inesperada: %+v", auditoriaRegistrada)
	}
	if usuarioRegistrado.PasswordHash == "" || usuarioRegistrado.PasswordHash == "ClaveSegura123!" {
		t.Fatal("la contraseña debe guardarse hasheada")
	}
	if len(usuarioRegistrado.Roles) != 1 || usuarioRegistrado.Roles[0] != "paciente" {
		t.Fatalf("roles inesperados: %v", usuarioRegistrado.Roles)
	}
	if usuarioRegistrado.TerminosAceptadosEn == nil {
		t.Fatal("el registro debe guardar la aceptación de términos")
	}
	if pacienteRegistrado.Persona.Email != "nueva@example.test" {
		t.Fatalf("email de persona inesperado: %q", pacienteRegistrado.Persona.Email)
	}
	if len(emails.enviados) != 1 {
		t.Fatalf("se esperaba un correo de verificación, obtenidos %d", len(emails.enviados))
	}
	if emails.enviados[0].tipo != "verificacion" || emails.enviados[0].destino != "nueva@example.test" {
		t.Fatalf("correo inesperado: %+v", emails.enviados[0])
	}
	if !strings.Contains(emails.enviados[0].enlace, "token=") {
		t.Fatal("el correo debe contener el enlace con token")
	}
}

func TestCuentaServiceRegistrarDocumentoExistenteCreaSolicitud(t *testing.T) {
	var auditoriaRegistrada *entities.Auditoria
	cuentas := &fakeCuentaStore{
		registrarPendienteFn: func(
			_ context.Context,
			usuario *entities.Usuario,
			pacienteID int64,
			token *entities.TokenCuenta,
			solicitud *entities.SolicitudVinculacion,
			auditoria *entities.Auditoria,
		) error {
			if pacienteID != 9 {
				t.Fatalf("paciente_id inesperado: %d", pacienteID)
			}
			usuario.ID = 11
			token.ID = 31
			solicitud.ID = 41
			solicitud.Estado = entities.SolicitudVinculacionPendiente
			auditoriaRegistrada = auditoria
			return nil
		},
	}
	pacientes := &fakePacienteStoreCuenta{
		buscarFn: func(context.Context, string, string) (*entities.Paciente, error) {
			return &entities.Paciente{ID: 9}, nil
		},
	}
	emails := &fakeEmailSenderCuenta{}
	svc := NewCuentaService(configCuentaPrueba(), cuentas, pacientes,
		&fakeUsuarioStoreCuenta{}, emails, nil, nil)

	resultado, err := svc.RegistrarPaciente(
		context.Background(), registroCuentaPrueba(), "127.0.0.1", "prueba",
	)
	if err != nil {
		t.Fatalf("registro de documento existente: %v", err)
	}
	if !resultado.VinculacionPendiente {
		t.Fatal("debe quedar pendiente de vinculación")
	}
	if resultado.Paciente != nil {
		t.Fatal("no debe devolver un paciente nuevo")
	}
	if resultado.Solicitud == nil || resultado.Solicitud.ID != 41 {
		t.Fatalf("solicitud inesperada: %+v", resultado.Solicitud)
	}
	if len(emails.enviados) != 1 {
		t.Fatalf("se esperaba el correo de verificación, obtenidos %d", len(emails.enviados))
	}
	if auditoriaRegistrada == nil || auditoriaRegistrada.EstadoHTTP != 202 {
		t.Fatalf("auditoría de vinculación pendiente inesperada: %+v", auditoriaRegistrada)
	}
}

func TestUsernameAuditoriaSeLimitaPorCaracteres(t *testing.T) {
	recortado := usernameAuditoria(strings.Repeat("á", 60))
	if len([]rune(recortado)) != 50 {
		t.Fatalf("se esperaban 50 caracteres, obtenidos %d", len([]rune(recortado)))
	}
}

func TestCuentaServiceAuditaTokensInvalidos(t *testing.T) {
	auditoria := &fakeAuditoriaStoreCuenta{}
	cuentas := &fakeCuentaStore{
		verificarEmailFn: func(context.Context, string, time.Time, *entities.Auditoria) (*entities.Usuario, error) {
			return nil, entities.NewValidationError("token inválido")
		},
		restablecerFn: func(context.Context, string, string, time.Time, *entities.Auditoria) (*entities.Usuario, error) {
			return nil, entities.NewValidationError("token inválido")
		},
	}
	svc := NewCuentaService(configCuentaPrueba(), cuentas, &fakePacienteStoreCuenta{},
		&fakeUsuarioStoreCuenta{}, nil, auditoria, nil)

	if _, err := svc.VerificarEmail(context.Background(), "token-invalido", "127.0.0.1", "prueba"); err == nil {
		t.Fatal("se esperaba error de verificación")
	}
	if err := svc.RestablecerPassword(
		context.Background(), "token-invalido", "OtraClaveSegura456!", "127.0.0.1", "prueba",
	); err == nil {
		t.Fatal("se esperaba error de reset")
	}

	if len(auditoria.entradas) != 2 {
		t.Fatalf("se esperaban 2 eventos de auditoría, obtenidos %d", len(auditoria.entradas))
	}
	for i, accion := range []string{"verificacion_email_fallida", "reset_password_fallido"} {
		if auditoria.entradas[i].Accion != accion || auditoria.entradas[i].EstadoHTTP != 400 {
			t.Fatalf("evento %d inesperado: %+v", i, auditoria.entradas[i])
		}
	}
}

func TestCuentaServiceRegistrarDocumentoConCuentaEsConflicto(t *testing.T) {
	usuarioID := int64(77)
	cuentas := &fakeCuentaStore{
		registrarPacienteFn: func(
			context.Context, *entities.Usuario, *entities.Paciente,
			*entities.TokenCuenta, *entities.Auditoria,
		) error {
			t.Fatal("no debe intentar crear un paciente si el documento ya tiene cuenta")
			return nil
		},
	}
	pacientes := &fakePacienteStoreCuenta{
		buscarFn: func(context.Context, string, string) (*entities.Paciente, error) {
			return &entities.Paciente{ID: 9, UsuarioID: &usuarioID}, nil
		},
	}
	svc := NewCuentaService(configCuentaPrueba(), cuentas, pacientes,
		&fakeUsuarioStoreCuenta{}, &fakeEmailSenderCuenta{}, nil, nil)

	_, err := svc.RegistrarPaciente(
		context.Background(), registroCuentaPrueba(), "127.0.0.1", "prueba",
	)
	if !errors.Is(err, entities.ErrConflict) {
		t.Fatalf("se esperaba conflicto, obtenido: %v", err)
	}
}

func TestCuentaServiceValidacionesDeRegistro(t *testing.T) {
	casos := []struct {
		nombre  string
		cambiar func(*RegistroPacienteInput)
	}{
		{"sin terminos", func(in *RegistroPacienteInput) { in.AceptaTerminos = false }},
		{"version de terminos", func(in *RegistroPacienteInput) { in.VersionTerminos = "otra" }},
		{"menor de edad", func(in *RegistroPacienteInput) { in.FechaNacimiento = time.Now().AddDate(-10, 0, 0) }},
		{"fecha futura", func(in *RegistroPacienteInput) { in.FechaNacimiento = time.Now().AddDate(1, 0, 0) }},
		{"password corta", func(in *RegistroPacienteInput) { in.Password = "corta" }},
		{"sin documento", func(in *RegistroPacienteInput) { in.NumeroDocumento = "" }},
		{"sexo invalido", func(in *RegistroPacienteInput) { in.Sexo = "desconocido" }},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			in := registroCuentaPrueba()
			caso.cambiar(&in)
			svc := NewCuentaService(configCuentaPrueba(), &fakeCuentaStore{},
				&fakePacienteStoreCuenta{}, &fakeUsuarioStoreCuenta{},
				&fakeEmailSenderCuenta{}, nil, nil)
			_, err := svc.RegistrarPaciente(context.Background(), in, "127.0.0.1", "prueba")
			if !errors.Is(err, entities.ErrValidation) {
				t.Fatalf("se esperaba validación, obtenido: %v", err)
			}
		})
	}
}

func TestCuentaServiceReenviarVerificacionNoRevelaCuenta(t *testing.T) {
	usuarios := &fakeUsuarioStoreCuenta{
		buscarPorEmailFn: func(context.Context, string) (*entities.Usuario, error) {
			return nil, entities.ErrNotFound
		},
	}
	emails := &fakeEmailSenderCuenta{}
	svc := NewCuentaService(configCuentaPrueba(), &fakeCuentaStore{}, &fakePacienteStoreCuenta{},
		usuarios, emails, nil, nil)

	if err := svc.ReenviarVerificacion(
		context.Background(), "noexiste@example.test", "127.0.0.1", "prueba",
	); err != nil {
		t.Fatalf("el reenvío no debe revelar si la cuenta existe: %v", err)
	}
	if len(emails.enviados) != 0 {
		t.Fatal("no debe enviarse correo para una cuenta inexistente")
	}
}

func TestCuentaServiceSolicitarResetEnviaTokenYCorreo(t *testing.T) {
	var tokenCreado *entities.TokenCuenta
	cuentas := &fakeCuentaStore{
		reemplazarTokenFn: func(_ context.Context, token *entities.TokenCuenta) error {
			token.ID = 51
			tokenCreado = token
			return nil
		},
	}
	usuarios := &fakeUsuarioStoreCuenta{
		buscarPorEmailFn: func(context.Context, string) (*entities.Usuario, error) {
			return &entities.Usuario{
				ID: 5, Username: "ana", Email: "ana@example.test",
				Nombres: "Ana", Activo: true,
			}, nil
		},
	}
	emails := &fakeEmailSenderCuenta{}
	svc := NewCuentaService(configCuentaPrueba(), cuentas, &fakePacienteStoreCuenta{},
		usuarios, emails, nil, nil)

	if err := svc.SolicitarResetPassword(
		context.Background(), "ana@example.test", "127.0.0.1", "prueba",
	); err != nil {
		t.Fatalf("solicitar reset: %v", err)
	}
	if tokenCreado == nil || tokenCreado.Tipo != entities.TokenResetPassword {
		t.Fatalf("token de reset inesperado: %+v", tokenCreado)
	}
	if len(emails.enviados) != 1 || emails.enviados[0].tipo != "reset" {
		t.Fatalf("correo de reset inesperado: %+v", emails.enviados)
	}
	if emails.enviados[0].destino != "ana@example.test" {
		t.Fatalf("destino inesperado: %q", emails.enviados[0].destino)
	}
}

func TestCuentaServiceVerificarYResetValidanEntrada(t *testing.T) {
	svc := NewCuentaService(configCuentaPrueba(), &fakeCuentaStore{}, &fakePacienteStoreCuenta{},
		&fakeUsuarioStoreCuenta{}, &fakeEmailSenderCuenta{}, nil, nil)

	if _, err := svc.VerificarEmail(context.Background(), "", "127.0.0.1", "prueba"); !errors.Is(err, entities.ErrValidation) {
		t.Fatalf("token vacío debe ser validación: %v", err)
	}
	if err := svc.RestablecerPassword(
		context.Background(), "token", "corta", "127.0.0.1", "prueba",
	); !errors.Is(err, entities.ErrValidation) {
		t.Fatalf("password corta debe ser validación: %v", err)
	}
}
