package config

import (
	"testing"
	"time"
)

func configValidaParaPruebas() Config {
	return Config{
		Env:                "production",
		HTTPPort:           "8080",
		DatabaseURL:        "postgres://localhost/medilist",
		DBMaxConns:         20,
		DBMinConns:         2,
		DBStatementTimeout: 30 * time.Second,
		DBLockTimeout:      5 * time.Second,
		DBIdleTxTimeout:    30 * time.Second,
		LogFormat:          "json",
		ServiceName:        "medilist-api",
		InstanceID:         "test-instance",
		JWTSecret:          "secreto-de-prueba-con-mas-de-32-caracteres",
		JWTIssuer:          "medilist",
		AccessTokenTTL:     15 * time.Minute,
		RefreshTokenTTL:    7 * 24 * time.Hour,
		BcryptCost:         12,
		MaxLoginAttempts:   5,
		LockoutDuration:    15 * time.Minute,

		RetentionInterval: 24 * time.Hour,
		ErrorRetention:    90 * 24 * time.Hour,
		AuditRetention:    365 * 24 * time.Hour,

		EmailProvider:        "outbox",
		EmailFrom:            "MediList <no-reply@medilist.local>",
		EmailOutboxDir:       "outbox/emails",
		EmailLinkBaseURL:     "http://localhost:3000",
		EmailVerificationTTL: 24 * time.Hour,
		PasswordResetTTL:     time.Hour,
		SMTPHost:             "",
		SMTPPort:             587,
		TermsVersion:         "2026-10",
		MinEdadCuenta:        18,

		ClinicTimezone:            "America/Guayaquil",
		PortalMinNoticeHours:      2,
		PortalMaxAdvanceDays:      90,
		PortalCancelCutoffHours:   24,
		AutoConfirmPortalBookings: true,
		ReferralValidDays:         90,
		PortalPacienteEnabled:     true,
		DerivacionesEnabled:       true,
		TransferenciasEnabled:     true,

		NotificationIntervalSeconds: 30,
		NotificationBatchSize:       50,
		NotificationMaxAttempts:     5,
		NotificationBackoffSeconds:  30,
		NotificationReminderHours:   []int{24, 2},

		CORSAllowedOrigins: []string{"http://localhost:3000"},
		HSTSEnabled:        true,
		MetricsEnabled:     true,
		RateLimitEnabled:   true,
	}
}

func TestValidateRechazaInstanciaYRetencionInvalidas(t *testing.T) {
	cfg := configValidaParaPruebas()
	cfg.InstanceID = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("la instancia no puede estar vacía")
	}

	cfg = configValidaParaPruebas()
	cfg.ErrorRetention = 24 * time.Hour
	if err := cfg.Validate(); err == nil {
		t.Fatal("la retención de errores menor a 7 días debe rechazarse")
	}

	cfg = configValidaParaPruebas()
	cfg.AuditRetention = 10 * 24 * time.Hour
	if err := cfg.Validate(); err == nil {
		t.Fatal("la retención de auditoría menor a 30 días debe rechazarse")
	}
}

func TestLoadApagaFeatureFlagsEnProduccion(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("PORTAL_PACIENTE_ENABLED", "")
	t.Setenv("DERIVACIONES_ENABLED", "")
	t.Setenv("TRANSFERENCIAS_PACIENTE_ENABLED", "")
	cfg := Load()
	if cfg.PortalPacienteEnabled || cfg.DerivacionesEnabled || cfg.TransferenciasEnabled {
		t.Fatalf("las flags deben iniciar apagadas en producción: %+v", cfg)
	}
}

func TestValidateRechazaPoolYTimeoutsInvalidos(t *testing.T) {
	cfg := configValidaParaPruebas()
	cfg.DBMaxConns = 1
	if err := cfg.Validate(); err == nil {
		t.Fatal("DB_MAX_CONNS menor a 2 debe rechazarse")
	}

	cfg = configValidaParaPruebas()
	cfg.DBMinConns = cfg.DBMaxConns + 1
	if err := cfg.Validate(); err == nil {
		t.Fatal("DB_MIN_CONNS mayor que DB_MAX_CONNS debe rechazarse")
	}

	cfg = configValidaParaPruebas()
	cfg.DBLockTimeout = 2 * time.Minute
	if err := cfg.Validate(); err == nil {
		t.Fatal("DB_LOCK_TIMEOUT_MS fuera de rango debe rechazarse")
	}
}

func TestValidateExigeJSONEnProduccion(t *testing.T) {
	cfg := configValidaParaPruebas()
	cfg.LogFormat = "text"
	if err := cfg.Validate(); err == nil {
		t.Fatal("producción debe rechazar logs de texto")
	}

	cfg.LogFormat = "json"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("la configuración JSON debería ser válida: %v", err)
	}
}

func TestValidateConfiguracionDeCuentas(t *testing.T) {
	cfg := configValidaParaPruebas()
	cfg.EmailLinkBaseURL = "localhost:3000"
	if err := cfg.Validate(); err == nil {
		t.Fatal("el enlace de correo debe ser una URL absoluta")
	}

	cfg = configValidaParaPruebas()
	cfg.PasswordResetTTL = 25 * time.Hour
	if err := cfg.Validate(); err == nil {
		t.Fatal("PASSWORD_RESET_MINUTES fuera de rango debe rechazarse")
	}

	cfg = configValidaParaPruebas()
	cfg.MinEdadCuenta = 130
	if err := cfg.Validate(); err == nil {
		t.Fatal("MIN_EDAD_CUENTA fuera de rango debe rechazarse")
	}
}

func TestValidateZonaHorariaYHorizonteDelPortal(t *testing.T) {
	cfg := configValidaParaPruebas()
	cfg.ClinicTimezone = "Zona/NoExiste"
	if err := cfg.Validate(); err == nil {
		t.Fatal("una zona horaria IANA inválida debe rechazarse")
	}

	cfg = configValidaParaPruebas()
	cfg.PortalMaxAdvanceDays = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("el horizonte del portal debe ser al menos un día")
	}

	cfg = configValidaParaPruebas()
	cfg.PortalMinNoticeHours = 200
	if err := cfg.Validate(); err == nil {
		t.Fatal("la antelación mínima fuera de rango debe rechazarse")
	}

	cfg = configValidaParaPruebas()
	cfg.PortalCancelCutoffHours = 721
	if err := cfg.Validate(); err == nil {
		t.Fatal("el corte de cancelación fuera de rango debe rechazarse")
	}

	cfg = configValidaParaPruebas()
	cfg.ReferralValidDays = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("la vigencia de derivación debe ser al menos un día")
	}

	cfg = configValidaParaPruebas()
	cfg.NotificationReminderHours = []int{24, 0}
	if err := cfg.Validate(); err == nil {
		t.Fatal("los recordatorios deben tener horas positivas")
	}

	cfg = configValidaParaPruebas()
	cfg.EmailProvider = "smtp"
	if err := cfg.Validate(); err == nil {
		t.Fatal("SMTP requiere host")
	}
	cfg.SMTPHost = "smtp.example.test"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("SMTP con host válido debería aceptarse: %v", err)
	}

	cfg = configValidaParaPruebas()
	cfg.CORSAllowedOrigins = []string{"*"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("CORS no debe permitir '*' en producción")
	}

	cfg = configValidaParaPruebas()
	cfg.CORSAllowedOrigins = []string{"https://app.example.test/ruta"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("CORS debe rechazar orígenes con ruta")
	}

	cfg = configValidaParaPruebas()
	cfg.TrustedProxies = []string{"no-es-una-ip"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("TRUSTED_PROXIES inválido debe rechazarse")
	}
}

func TestValidateArchivoDeLogs(t *testing.T) {
	cfg := configValidaParaPruebas()
	cfg.Env = "development"
	cfg.LogFile = "logs/medilist.jsonl"
	cfg.LogFormat = "text"
	if err := cfg.Validate(); err == nil {
		t.Fatal("LOG_FILE debe exigir LOG_FORMAT=json")
	}

	cfg.LogFormat = "json"
	cfg.LogFileMaxMB = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("LOG_FILE_MAX_MB inválido debe rechazarse")
	}

	cfg.LogFileMaxMB = 100
	cfg.LogFileBackups = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("LOG_FILE_BACKUPS inválido debe rechazarse")
	}

	cfg.LogFileBackups = 5
	if err := cfg.Validate(); err != nil {
		t.Fatalf("la configuración de archivo de logs debería ser válida: %v", err)
	}
}
