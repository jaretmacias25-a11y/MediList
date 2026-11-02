package config

import (
	"bufio"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

const devJWTSecret = "dev-only-insecure-secret-change-me-please-0123456789"

// Config contiene la configuración de la aplicación.
type Config struct {
	Env                string
	HTTPPort           string
	DatabaseURL        string
	DBMaxConns         int
	DBMinConns         int
	DBStatementTimeout time.Duration
	DBLockTimeout      time.Duration
	DBIdleTxTimeout    time.Duration
	LogFormat          string
	LogFile            string
	LogFileMaxMB       int
	LogFileBackups     int
	ServiceName        string
	InstanceID         string

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

	EmailProvider        string
	EmailFrom            string
	EmailOutboxDir       string
	EmailLinkBaseURL     string
	EmailVerificationTTL time.Duration
	PasswordResetTTL     time.Duration
	TermsVersion         string
	MinEdadCuenta        int

	ClinicTimezone       string
	PortalMinNoticeHours int
	PortalMaxAdvanceDays int
}

// Load lee .env (sin sobrescribir variables del sistema) y las variables de entorno.
func Load() Config {
	_ = loadDotEnv(".env")

	env := getenv("APP_ENV", "development")
	return Config{
		Env:                env,
		HTTPPort:           strings.TrimPrefix(getenv("HTTP_PORT", "8080"), ":"),
		DatabaseURL:        getenv("DATABASE_URL", "postgres://postgres:CAMBIAR_PASSWORD@localhost:5432/medilist?sslmode=disable"),
		DBMaxConns:         getenvInt("DB_MAX_CONNS", 20),
		DBMinConns:         getenvInt("DB_MIN_CONNS", 2),
		DBStatementTimeout: time.Duration(getenvInt("DB_STATEMENT_TIMEOUT_MS", 30000)) * time.Millisecond,
		DBLockTimeout:      time.Duration(getenvInt("DB_LOCK_TIMEOUT_MS", 5000)) * time.Millisecond,
		DBIdleTxTimeout:    time.Duration(getenvInt("DB_IDLE_TX_TIMEOUT_MS", 30000)) * time.Millisecond,
		LogFormat:          strings.ToLower(getenv("LOG_FORMAT", "text")),
		LogFile:            strings.TrimSpace(getenv("LOG_FILE", "")),
		LogFileMaxMB:       getenvInt("LOG_FILE_MAX_MB", 100),
		LogFileBackups:     getenvInt("LOG_FILE_BACKUPS", 5),
		ServiceName:        strings.TrimSpace(getenv("APP_SERVICE_NAME", "medilist-api")),
		InstanceID:         strings.TrimSpace(getenv("APP_INSTANCE_ID", defaultInstanceID())),
		JWTSecret:          getenv("JWT_SECRET", devJWTSecret),
		JWTIssuer:          getenv("JWT_ISSUER", "medilist"),
		AccessTokenTTL:     time.Duration(getenvInt("ACCESS_TOKEN_MINUTES", 15)) * time.Minute,
		RefreshTokenTTL:    time.Duration(getenvInt("REFRESH_TOKEN_HOURS", 168)) * time.Hour,
		BcryptCost:         getenvInt("BCRYPT_COST", 12),
		MaxLoginAttempts:   getenvInt("MAX_LOGIN_ATTEMPTS", 5),
		LockoutDuration:    time.Duration(getenvInt("LOCK_MINUTES", 15)) * time.Minute,

		RetentionInterval: time.Duration(getenvInt("RETENTION_INTERVAL_HOURS", 24)) * time.Hour,
		ErrorRetention:    time.Duration(getenvInt("ERROR_RETENTION_DAYS", 90)) * 24 * time.Hour,
		AuditRetention:    time.Duration(getenvInt("AUDIT_RETENTION_DAYS", 365)) * 24 * time.Hour,

		EmailProvider:        strings.ToLower(strings.TrimSpace(getenv("EMAIL_PROVIDER", "outbox"))),
		EmailFrom:            strings.TrimSpace(getenv("EMAIL_FROM", "MediList <no-reply@medilist.local>")),
		EmailOutboxDir:       strings.TrimSpace(getenv("EMAIL_OUTBOX_DIR", "outbox/emails")),
		EmailLinkBaseURL:     strings.TrimSpace(getenv("EMAIL_LINK_BASE_URL", "http://localhost:3000")),
		EmailVerificationTTL: time.Duration(getenvInt("EMAIL_VERIFICATION_HOURS", 24)) * time.Hour,
		PasswordResetTTL:     time.Duration(getenvInt("PASSWORD_RESET_MINUTES", 60)) * time.Minute,
		TermsVersion:         strings.TrimSpace(getenv("TERMS_VERSION", "2026-10")),
		MinEdadCuenta:        getenvInt("MIN_EDAD_CUENTA", 18),

		ClinicTimezone:       strings.TrimSpace(getenv("CLINIC_TIMEZONE", "America/Guayaquil")),
		PortalMinNoticeHours: getenvInt("PORTAL_MIN_NOTICE_HOURS", 2),
		PortalMaxAdvanceDays: getenvInt("PORTAL_MAX_ADVANCE_DAYS", 90),
	}
}

// Validate comprueba que la configuración de seguridad sea segura.
func (c Config) Validate() error {
	if c.DBMaxConns < 2 || c.DBMaxConns > 200 {
		return fmt.Errorf("DB_MAX_CONNS debe estar entre 2 y 200")
	}
	if c.DBMinConns < 0 || c.DBMinConns > c.DBMaxConns {
		return fmt.Errorf("DB_MIN_CONNS debe estar entre 0 y DB_MAX_CONNS")
	}
	if c.DBStatementTimeout < 0 || c.DBStatementTimeout > 5*time.Minute {
		return fmt.Errorf("DB_STATEMENT_TIMEOUT_MS debe estar entre 0 y 300000")
	}
	if c.DBLockTimeout < 0 || c.DBLockTimeout > time.Minute {
		return fmt.Errorf("DB_LOCK_TIMEOUT_MS debe estar entre 0 y 60000")
	}
	if c.DBIdleTxTimeout < 0 || c.DBIdleTxTimeout > 5*time.Minute {
		return fmt.Errorf("DB_IDLE_TX_TIMEOUT_MS debe estar entre 0 y 300000")
	}
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
	if c.EmailProvider != "outbox" {
		return fmt.Errorf("EMAIL_PROVIDER debe ser outbox; el proveedor real aún no está implementado")
	}
	if c.EmailFrom == "" || len(c.EmailFrom) > 200 {
		return fmt.Errorf("EMAIL_FROM debe tener entre 1 y 200 caracteres")
	}
	if c.EmailOutboxDir == "" || len(c.EmailOutboxDir) > 260 {
		return fmt.Errorf("EMAIL_OUTBOX_DIR debe tener entre 1 y 260 caracteres")
	}
	enlace, err := url.Parse(c.EmailLinkBaseURL)
	if err != nil || enlace.Host == "" || (enlace.Scheme != "http" && enlace.Scheme != "https") {
		return fmt.Errorf("EMAIL_LINK_BASE_URL debe ser una URL http(s) absoluta")
	}
	if c.EmailVerificationTTL < 15*time.Minute || c.EmailVerificationTTL > 30*24*time.Hour {
		return fmt.Errorf("EMAIL_VERIFICATION_HOURS debe estar entre 1 y 720")
	}
	if c.PasswordResetTTL < 5*time.Minute || c.PasswordResetTTL > 24*time.Hour {
		return fmt.Errorf("PASSWORD_RESET_MINUTES debe estar entre 5 y 1440")
	}
	if c.TermsVersion == "" || len(c.TermsVersion) > 20 {
		return fmt.Errorf("TERMS_VERSION debe tener entre 1 y 20 caracteres")
	}
	if c.MinEdadCuenta < 0 || c.MinEdadCuenta > 120 {
		return fmt.Errorf("MIN_EDAD_CUENTA debe estar entre 0 y 120")
	}
	if c.ClinicTimezone == "" {
		return fmt.Errorf("CLINIC_TIMEZONE no puede estar vacía")
	}
	if _, err := time.LoadLocation(c.ClinicTimezone); err != nil {
		return fmt.Errorf("CLINIC_TIMEZONE no es una zona IANA válida: %w", err)
	}
	if c.PortalMinNoticeHours < 0 || c.PortalMinNoticeHours > 168 {
		return fmt.Errorf("PORTAL_MIN_NOTICE_HOURS debe estar entre 0 y 168")
	}
	if c.PortalMaxAdvanceDays < 1 || c.PortalMaxAdvanceDays > 365 {
		return fmt.Errorf("PORTAL_MAX_ADVANCE_DAYS debe estar entre 1 y 365")
	}
	return nil
}

// ClinicLocation devuelve la zona horaria validada de la clínica.
func (c Config) ClinicLocation() (*time.Location, error) {
	return time.LoadLocation(c.ClinicTimezone)
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
