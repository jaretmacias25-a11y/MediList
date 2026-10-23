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
