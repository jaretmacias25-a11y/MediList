package config

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

const devJWTSecret = "dev-only-insecure-secret-change-me-please-0123456789"

// Config contiene la configuración de la aplicación.
type Config struct {
	Env            string
	HTTPPort       string
	DatabaseURL    string
	LogFormat      string
	LogFile        string
	LogFileMaxMB   int
	LogFileBackups int
	ServiceName    string
	InstanceID     string

	JWTSecret string
	JWTIssuer string

	AccessTokenTTL   time.Duration
	RefreshTokenTTL  time.Duration
	BcryptCost       int
	MaxLoginAttempts int
	LockoutDuration  time.Duration

	RetentionInterval time.Duration
	ErrorRetention    time.Duration
	AuditRetention    time.Duration
}

// Load lee .env (sin sobrescribir variables del sistema) y las variables de entorno.
func Load() Config {
	_ = loadDotEnv(".env")

	env := getenv("APP_ENV", "development")
	return Config{
		Env:              env,
		HTTPPort:         strings.TrimPrefix(getenv("HTTP_PORT", "8080"), ":"),
		DatabaseURL:      getenv("DATABASE_URL", "postgres://postgres:CAMBIAR_PASSWORD@localhost:5432/medilist?sslmode=disable"),
		LogFormat:        strings.ToLower(getenv("LOG_FORMAT", "text")),
		LogFile:          strings.TrimSpace(getenv("LOG_FILE", "")),
		LogFileMaxMB:     getenvInt("LOG_FILE_MAX_MB", 100),
		LogFileBackups:   getenvInt("LOG_FILE_BACKUPS", 5),
		ServiceName:      strings.TrimSpace(getenv("APP_SERVICE_NAME", "medilist-api")),
		InstanceID:       strings.TrimSpace(getenv("APP_INSTANCE_ID", defaultInstanceID())),
		JWTSecret:        getenv("JWT_SECRET", devJWTSecret),
		JWTIssuer:        getenv("JWT_ISSUER", "medilist"),
		AccessTokenTTL:   time.Duration(getenvInt("ACCESS_TOKEN_MINUTES", 15)) * time.Minute,
		RefreshTokenTTL:  time.Duration(getenvInt("REFRESH_TOKEN_HOURS", 168)) * time.Hour,
		BcryptCost:       getenvInt("BCRYPT_COST", 12),
		MaxLoginAttempts: getenvInt("MAX_LOGIN_ATTEMPTS", 5),
		LockoutDuration:  time.Duration(getenvInt("LOCK_MINUTES", 15)) * time.Minute,

		RetentionInterval: time.Duration(getenvInt("RETENTION_INTERVAL_HOURS", 24)) * time.Hour,
		ErrorRetention:    time.Duration(getenvInt("ERROR_RETENTION_DAYS", 90)) * 24 * time.Hour,
		AuditRetention:    time.Duration(getenvInt("AUDIT_RETENTION_DAYS", 365)) * 24 * time.Hour,
	}
}

// Validate comprueba que la configuración de seguridad sea segura.
func (c Config) Validate() error {
	if c.LogFormat != "text" && c.LogFormat != "json" {
		return fmt.Errorf("LOG_FORMAT debe ser text o json")
	}
	if c.Env == "production" && c.LogFormat != "json" {
		return fmt.Errorf("LOG_FORMAT debe ser json en producción para la agregación de logs")
	}
	if c.LogFile != "" {
		if c.LogFormat != "json" {
			return fmt.Errorf("LOG_FILE requiere LOG_FORMAT=json para la agregación")
		}
		if c.LogFileMaxMB < 1 || c.LogFileMaxMB > 1024 {
			return fmt.Errorf("LOG_FILE_MAX_MB debe estar entre 1 y 1024")
		}
		if c.LogFileBackups < 1 || c.LogFileBackups > 20 {
			return fmt.Errorf("LOG_FILE_BACKUPS debe estar entre 1 y 20")
		}
	}
	if c.ServiceName == "" {
		return fmt.Errorf("APP_SERVICE_NAME no puede estar vacío")
	}
	if len(c.ServiceName) > 80 {
		return fmt.Errorf("APP_SERVICE_NAME no puede superar 80 caracteres")
	}
	if c.InstanceID == "" {
		return fmt.Errorf("APP_INSTANCE_ID no puede estar vacío")
	}
	if len(c.InstanceID) > 120 {
		return fmt.Errorf("APP_INSTANCE_ID no puede superar 120 caracteres")
	}
	if c.RetentionInterval < time.Hour || c.RetentionInterval > 7*24*time.Hour {
		return fmt.Errorf("RETENTION_INTERVAL_HOURS debe estar entre 1 y 168")
	}
	if c.ErrorRetention < 0 || (c.ErrorRetention > 0 && c.ErrorRetention < 7*24*time.Hour) {
		return fmt.Errorf("ERROR_RETENTION_DAYS debe ser 0 o al menos 7")
	}
	if c.ErrorRetention > 3650*24*time.Hour {
		return fmt.Errorf("ERROR_RETENTION_DAYS no puede superar 3650")
	}
	if c.AuditRetention < 0 || (c.AuditRetention > 0 && c.AuditRetention < 30*24*time.Hour) {
		return fmt.Errorf("AUDIT_RETENTION_DAYS debe ser 0 o al menos 30")
	}
	if c.AuditRetention > 3650*24*time.Hour {
		return fmt.Errorf("AUDIT_RETENTION_DAYS no puede superar 3650")
	}
	if len(c.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET debe tener al menos 32 caracteres")
	}
	if c.Env == "production" && c.JWTSecret == devJWTSecret {
		return fmt.Errorf("JWT_SECRET de desarrollo no permitido en producción")
	}
	if c.AccessTokenTTL <= 0 || c.AccessTokenTTL > time.Hour {
		return fmt.Errorf("ACCESS_TOKEN_MINUTES debe estar entre 1 y 60")
	}
	if c.RefreshTokenTTL <= c.AccessTokenTTL {
		return fmt.Errorf("REFRESH_TOKEN_HOURS debe ser mayor que el token de acceso")
	}
	if c.BcryptCost < 10 || c.BcryptCost > 14 {
		return fmt.Errorf("BCRYPT_COST debe estar entre 10 y 14")
	}
	if c.MaxLoginAttempts < 3 || c.MaxLoginAttempts > 20 {
		return fmt.Errorf("MAX_LOGIN_ATTEMPTS debe estar entre 3 y 20")
	}
	if c.LockoutDuration < time.Minute {
		return fmt.Errorf("LOCK_MINUTES debe ser al menos 1")
	}
	return nil
}

func (c Config) LogLevel() slog.Level {
	if c.Env == "production" {
		return slog.LevelInfo
	}
	return slog.LevelDebug
}

func defaultInstanceID() string {
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		return "desconocido"
	}
	return strings.TrimSpace(hostname)
}

func getenv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

// loadDotEnv implementa un lector mínimo de archivos .env.
func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, value)
		}
	}
	return scanner.Err()
}
