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
	Env         string
	HTTPPort    string
	DatabaseURL string
	LogFormat   string
	ServiceName string

	JWTSecret string
	JWTIssuer string

	AccessTokenTTL   time.Duration
	RefreshTokenTTL  time.Duration
	BcryptCost       int
	MaxLoginAttempts int
	LockoutDuration  time.Duration
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
		ServiceName:      getenv("APP_SERVICE_NAME", "medilist-api"),
		JWTSecret:        getenv("JWT_SECRET", devJWTSecret),
		JWTIssuer:        getenv("JWT_ISSUER", "medilist"),
		AccessTokenTTL:   time.Duration(getenvInt("ACCESS_TOKEN_MINUTES", 15)) * time.Minute,
		RefreshTokenTTL:  time.Duration(getenvInt("REFRESH_TOKEN_HOURS", 168)) * time.Hour,
		BcryptCost:       getenvInt("BCRYPT_COST", 12),
		MaxLoginAttempts: getenvInt("MAX_LOGIN_ATTEMPTS", 5),
		LockoutDuration:  time.Duration(getenvInt("LOCK_MINUTES", 15)) * time.Minute,
	}
}

// Validate comprueba que la configuración de seguridad sea segura.
func (c Config) Validate() error {
	if c.LogFormat != "text" && c.LogFormat != "json" {
		return fmt.Errorf("LOG_FORMAT debe ser text o json")
	}
	if strings.TrimSpace(c.ServiceName) == "" {
		return fmt.Errorf("APP_SERVICE_NAME no puede estar vacío")
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
