package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"medilist/internal/config"
	"medilist/internal/observability"
	"medilist/internal/repository/postgres"
	"medilist/internal/service"
	httpapi "medilist/internal/transport/http"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "configuración de seguridad inválida: %v\n", err)
		os.Exit(1)
	}

	logOutput := io.Writer(os.Stdout)
	var logFile *observability.RotatingFileWriter
	if cfg.LogFile != "" {
		writer, err := observability.NewRotatingFileWriter(
			cfg.LogFile,
			int64(cfg.LogFileMaxMB)*1024*1024,
			cfg.LogFileBackups,
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "no se pudo abrir LOG_FILE: %v\n", err)
			os.Exit(1)
		}
		logFile = writer
		defer logFile.Close()
		logOutput = io.MultiWriter(os.Stdout, logFile)
	}

	opts := &slog.HandlerOptions{Level: cfg.LogLevel()}
	var handler slog.Handler
	if cfg.LogFormat == "json" {
		handler = slog.NewJSONHandler(logOutput, opts)
	} else {
		handler = slog.NewTextHandler(logOutput, opts)
	}
	logger := slog.New(handler).With(
		"service", cfg.ServiceName,
		"env", cfg.Env,
		"instance", cfg.InstanceID,
	)

	logger.Info("iniciando MediList API")

	ctx := context.Background()

	// Repositorio (PostgreSQL)
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("no se pudo conectar a PostgreSQL", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Almacenes
	especialidadStore := postgres.NewEspecialidadStore(pool)
	pacienteStore := postgres.NewPacienteStore(pool)
	medicoStore := postgres.NewMedicoStore(pool)
	citaStore := postgres.NewCitaStore(pool)
	usuarioStore := postgres.NewUsuarioStore(pool)
	rolStore := postgres.NewRolStore(pool)
	refreshTokenStore := postgres.NewRefreshTokenStore(pool)
	auditoriaStore := postgres.NewAuditoriaStore(pool)
	errorStore := postgres.NewErrorStore(pool)

	// Servicios (lógica de negocio)
	especialidadService := service.NewEspecialidadService(especialidadStore)
	pacienteService := service.NewPacienteService(pacienteStore)
	medicoService := service.NewMedicoService(medicoStore, especialidadStore)
	citaService := service.NewCitaService(citaStore, pacienteStore, medicoStore)

	authService := service.NewAuthService(service.AuthConfig{
		JWTSecret:        cfg.JWTSecret,
		JWTIssuer:        cfg.JWTIssuer,
		AccessTTL:        cfg.AccessTokenTTL,
		RefreshTTL:       cfg.RefreshTokenTTL,
		BcryptCost:       cfg.BcryptCost,
		MaxLoginAttempts: cfg.MaxLoginAttempts,
		LockoutDuration:  cfg.LockoutDuration,
	}, usuarioStore, refreshTokenStore, auditoriaStore)
	usuarioService := service.NewUsuarioService(usuarioStore, rolStore, cfg.BcryptCost)
	auditoriaService := service.NewAuditoriaService(auditoriaStore)
	errorService := service.NewErrorService(errorStore, cfg.ServiceName, cfg.InstanceID, logger)
	retentionService := service.NewRetentionService(
		postgres.NewRetentionStore(pool),
		service.RetentionConfig{
			Interval:       cfg.RetentionInterval,
			ErrorRetention: cfg.ErrorRetention,
			AuditRetention: cfg.AuditRetention,
		},
		logger,
	)
	retentionCtx, cancelRetention := context.WithCancel(ctx)
	defer cancelRetention()
	go retentionService.Run(retentionCtx)

	// Transporte (HTTP)
	router := httpapi.NewRouter(httpapi.Dependencies{
		Especialidades: especialidadService,
		Pacientes:      pacienteService,
		Medicos:        medicoService,
		Citas:          citaService,
		Auth:           authService,
		Usuarios:       usuarioService,
		Auditoria:      auditoriaService,
		AuditoriaStore: auditoriaStore,
		Errores:        errorService,
		DB:             pool,
		Logger:         logger,
	})

	server := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           router,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("servidor HTTP escuchando", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	stopCtx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	select {
	case err := <-serverErrors:
		logger.Error("el servidor HTTP falló", "error", err)
		os.Exit(1)
	case <-stopCtx.Done():
		logger.Info("deteniendo servidor HTTP")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("apagado no controlado", "error", err)
	}
	logger.Info("MediList API detenida")
}
