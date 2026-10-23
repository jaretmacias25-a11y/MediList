package service

import (
	"context"
	"log/slog"
	"time"
)

// RetentionConfig define cada cuánto y hasta cuándo se conservan los datos.
// Un valor de retención en cero deshabilita la purga de esa tabla.
type RetentionConfig struct {
	Interval       time.Duration
	ErrorRetention time.Duration
	AuditRetention time.Duration
}

// RetentionService aplica la política de retención de observabilidad.
type RetentionService struct {
	store  RetentionStore
	cfg    RetentionConfig
	logger *slog.Logger
	ahora  func() time.Time
}

// NewRetentionService construye el servicio de retención.
func NewRetentionService(store RetentionStore, cfg RetentionConfig, logger *slog.Logger) *RetentionService {
	return &RetentionService{
		store:  store,
		cfg:    cfg,
		logger: logger,
		ahora:  time.Now,
	}
}

// Run ejecuta la retención al arrancar y luego en cada intervalo.
func (s *RetentionService) Run(ctx context.Context) {
	if s.cfg.Interval <= 0 {
		return
	}
	if s.cfg.ErrorRetention <= 0 && s.cfg.AuditRetention <= 0 {
		s.logger.Info("retención de observabilidad deshabilitada")
		return
	}
	select {
	case <-ctx.Done():
		return
	default:
	}

	s.Ejecutar(ctx)
	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.Ejecutar(ctx)
		}
	}
}

// Ejecutar aplica una pasada de retención. Es idempotente y no aborta la
// segunda tabla si falla la primera.
func (s *RetentionService) Ejecutar(ctx context.Context) {
	ahora := s.ahora()

	if s.cfg.ErrorRetention > 0 {
		antes := ahora.Add(-s.cfg.ErrorRetention)
		cantidad, err := s.store.PurgarErrores(ctx, antes)
		if err != nil {
			s.logger.Error("no se pudo aplicar la retención de errores",
				"tabla", "errores_aplicacion", "antes", antes, "error", err)
		} else {
			s.logger.Info("retencion_aplicada",
				"tabla", "errores_aplicacion", "eliminados", cantidad, "antes", antes)
		}
	}

	if s.cfg.AuditRetention > 0 {
		antes := ahora.Add(-s.cfg.AuditRetention)
		cantidad, err := s.store.PurgarAuditoria(ctx, antes)
		if err != nil {
			s.logger.Error("no se pudo aplicar la retención de auditoría",
				"tabla", "auditoria", "antes", antes, "error", err)
		} else {
			s.logger.Info("retencion_aplicada",
				"tabla", "auditoria", "eliminados", cantidad, "antes", antes)
		}
	}
}
