package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"medilist/internal/config"
	"medilist/internal/repository/postgres"
	"medilist/internal/service"
	httpapi "medilist/internal/transport/http"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel(),
	}))

	if err := cfg.Validate(); err != nil {
		logger.Error("configuración de seguridad inválida", "error", err)
		os.Exit(1)
	}
	logger.Info("iniciando MediList API", "env", cfg.Env)

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
		DB:             pool,
		Logger:         logger,
	})

	server := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           router,
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
