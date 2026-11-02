package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"medilist/internal/service"
)

type preferenciasNotificacionRequest struct {
	EmailHabilitado          bool `json:"email_habilitado"`
	RecordatoriosHabilitados bool `json:"recordatorios_habilitados"`
}

type preferenciasNotificacionResponse struct {
	UsuarioID                int64     `json:"usuario_id"`
	EmailHabilitado          bool      `json:"email_habilitado"`
	RecordatoriosHabilitados bool      `json:"recordatorios_habilitados"`
	UpdatedAt                time.Time `json:"updated_at"`
}

// NotificacionHandler expone las preferencias de correo de la cuenta.
type NotificacionHandler struct {
	svc    *service.NotificacionService
	logger *slog.Logger
}

func NewNotificacionHandler(
	svc *service.NotificacionService,
	logger *slog.Logger,
) *NotificacionHandler {
	return &NotificacionHandler{svc: svc, logger: logger}
}

func (h *NotificacionHandler) ObtenerPreferencias(w http.ResponseWriter, r *http.Request) {
	actor := actorFromContext(r.Context())
	if actor == nil {
		writeUnauthorized(w)
		return
	}
	p, err := h.svc.ObtenerPreferencias(r.Context(), actor)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, preferenciasNotificacionResponse{
		UsuarioID: p.UsuarioID, EmailHabilitado: p.EmailHabilitado,
		RecordatoriosHabilitados: p.RecordatoriosHabilitados, UpdatedAt: p.UpdatedAt,
	})
}

func (h *NotificacionHandler) ActualizarPreferencias(w http.ResponseWriter, r *http.Request) {
	actor := actorFromContext(r.Context())
	if actor == nil {
		writeUnauthorized(w)
		return
	}
	var req preferenciasNotificacionRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	p, err := h.svc.ActualizarPreferencias(
		r.Context(), actor, req.EmailHabilitado, req.RecordatoriosHabilitados)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, preferenciasNotificacionResponse{
		UsuarioID: p.UsuarioID, EmailHabilitado: p.EmailHabilitado,
		RecordatoriosHabilitados: p.RecordatoriosHabilitados, UpdatedAt: p.UpdatedAt,
	})
}
