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
	"medilist/internal/notificaciones"
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
	clinicLocation, err := cfg.ClinicLocation()
	if err != nil {
		fmt.Fprintf(os.Stderr, "zona horaria de la clínica inválida: %v\n", err)
		os.Exit(1)
	}
	if err := httpapi.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		fmt.Fprintf(os.Stderr, "configuración de proxies de confianza inválida: %v\n", err)
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
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL, postgres.PoolOptions{
		MaxConns:         cfg.DBMaxConns,
		MinConns:         cfg.DBMinConns,
		StatementTimeout: cfg.DBStatementTimeout,
		LockTimeout:      cfg.DBLockTimeout,
		IdleTxTimeout:    cfg.DBIdleTxTimeout,
		ApplicationName:  cfg.ServiceName,
	})
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
	notaCitaStore := postgres.NewNotaCitaStore(pool)
	derivacionStore := postgres.NewDerivacionStore(pool)
	transferenciaStore := postgres.NewTransferenciaStore(pool)
	excepcionDerivacionStore := postgres.NewExcepcionDerivacionStore(pool)
	notificacionStore := postgres.NewNotificacionStore(pool)
	rateLimitStore := postgres.NewRateLimitStore(pool)
	agendaStore := postgres.NewAgendaStore(pool)
	catalogoStore := postgres.NewCatalogoStore(pool)
	usuarioStore := postgres.NewUsuarioStore(pool)
	rolStore := postgres.NewRolStore(pool)
	refreshTokenStore := postgres.NewRefreshTokenStore(pool)
	auditoriaStore := postgres.NewAuditoriaStore(pool)
	errorStore := postgres.NewErrorStore(pool)
	cuentaStore := postgres.NewCuentaStore(pool)
	actorStore := postgres.NewActorStore(pool)

	var emailSender service.EmailSender
	var notificationSender service.NotificacionEmailSender
	switch cfg.EmailProvider {
	case "smtp":
		smtpSender, err := notificaciones.NewSMTPEmailSender(
			cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPassword,
			cfg.EmailFrom, logger,
		)
		if err != nil {
			logger.Error("no se pudo inicializar SMTP", "error", err)
			os.Exit(1)
		}
		emailSender = smtpSender
		notificationSender = smtpSender
	case "outbox":
		devSender, err := notificaciones.NewDevEmailSender(
			cfg.EmailOutboxDir, cfg.EmailFrom, logger,
		)
		if err != nil {
			logger.Error("no se pudo inicializar el buzón de correo", "error", err)
			os.Exit(1)
		}
		emailSender = devSender
		notificationSender = devSender
		if cfg.Env == "production" {
			logger.Warn("EMAIL_PROVIDER=outbox es solo para desarrollo; configura SMTP antes de producción")
		}
	default:
		logger.Error("EMAIL_PROVIDER no soportado", "provider", cfg.EmailProvider)
		os.Exit(1)
	}

	// Servicios (lógica de negocio)
	especialidadService := service.NewEspecialidadService(especialidadStore)
	pacienteService := service.NewPacienteService(pacienteStore)
	medicoService := service.NewMedicoService(medicoStore, especialidadStore)
	citaService := service.NewCitaService(citaStore, pacienteStore, medicoStore)
	portalPacienteService := service.NewPortalPacienteService(pacienteStore)
	medicoPortalService := service.NewMedicoPortalService(citaStore, pacienteStore)
	agendaService := service.NewAgendaService(
		medicoStore, agendaStore, agendaStore,
		service.AgendaConfig{Location: clinicLocation},
	)
	disponibilidadService := service.NewDisponibilidadService(
		medicoStore, agendaStore,
		service.DisponibilidadConfig{
			Location:       clinicLocation,
			MinNotice:      time.Duration(cfg.PortalMinNoticeHours) * time.Hour,
			MaxAdvanceDays: cfg.PortalMaxAdvanceDays,
		},
	)
	catalogoService := service.NewCatalogoService(catalogoStore)
	portalCitaService := service.NewPortalCitaService(
		citaStore, pacienteStore, medicoStore, derivacionStore, disponibilidadService,
		service.PortalCitaConfig{
			CancelCutoff: time.Duration(cfg.PortalCancelCutoffHours) * time.Hour,
			AutoConfirm:  cfg.AutoConfirmPortalBookings,
		},
		emailSender,
	)
	medicoClinicoService := service.NewMedicoClinicoService(
		citaStore, notaCitaStore, derivacionStore, medicoStore, especialidadStore,
		service.MedicoClinicoConfig{ReferralValidDays: cfg.ReferralValidDays},
	)
	pacienteTransferService := service.NewPacienteTransferService(
		pacienteStore, medicoStore, transferenciaStore,
	)
	derivacionAdminService := service.NewDerivacionAdminService(
		citaStore, medicoStore, excepcionDerivacionStore,
	)
	var rateLimitService *service.RateLimitService
	if cfg.RateLimitEnabled {
		rateLimitService = service.NewRateLimitService(rateLimitStore)
	}
	notificacionService := service.NewNotificacionService(
		notificacionStore, notificationSender, citaStore, pacienteStore, logger,
		service.NotificacionConfig{
			Interval:      time.Duration(cfg.NotificationIntervalSeconds) * time.Second,
			BatchSize:     cfg.NotificationBatchSize,
			MaxAttempts:   cfg.NotificationMaxAttempts,
			BackoffBase:   time.Duration(cfg.NotificationBackoffSeconds) * time.Second,
			ReminderHours: cfg.NotificationReminderHours,
		},
	)
	citaService.SetNotificaciones(notificacionService)
	portalCitaService.SetNotificaciones(notificacionService)
	medicoClinicoService.SetNotificaciones(notificacionService, pacienteStore)

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
	cuentaService := service.NewCuentaService(service.CuentaConfig{
		TermsVersion:    cfg.TermsVersion,
		MinEdadCuenta:   cfg.MinEdadCuenta,
		BcryptCost:      cfg.BcryptCost,
		VerificationTTL: cfg.EmailVerificationTTL,
		ResetTTL:        cfg.PasswordResetTTL,
		LinkBaseURL:     cfg.EmailLinkBaseURL,
	}, cuentaStore, pacienteStore, usuarioStore, emailSender, auditoriaStore, logger)
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

	notificacionCtx, cancelNotificacion := context.WithCancel(ctx)
	defer cancelNotificacion()
	go notificacionService.Run(notificacionCtx)

	// Transporte (HTTP)
	router := httpapi.NewRouter(httpapi.Dependencies{
		Especialidades:     especialidadService,
		Pacientes:          pacienteService,
		Medicos:            medicoService,
		Citas:              citaService,
		Auth:               authService,
		Usuarios:           usuarioService,
		Cuentas:            cuentaService,
		PortalPaciente:     portalPacienteService,
		MedicoPortal:       medicoPortalService,
		Agenda:             agendaService,
		Catalogo:           catalogoService,
		Disponibilidad:     disponibilidadService,
		PortalCitas:        portalCitaService,
		MedicoClinico:      medicoClinicoService,
		PacienteTransfer:   pacienteTransferService,
		DerivacionAdmin:    derivacionAdminService,
		Notificaciones:     notificacionService,
		RateLimiter:        rateLimitService,
		CORSAllowedOrigins: cfg.CORSAllowedOrigins,
		HSTSEnabled:        cfg.HSTSEnabled,
		MetricsEnabled:     cfg.MetricsEnabled,
		Auditoria:          auditoriaService,
		AuditoriaStore:     auditoriaStore,
		ActorStore:         actorStore,
		Errores:            errorService,
		DB:                 pool,
		Logger:             logger,
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
