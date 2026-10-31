package httpapi

import (
	"log/slog"
	"net/http"

	"medilist/internal/service"
)

// PortalHandler expone los datos propios del paciente autenticado.
type PortalHandler struct {
	svc    *service.PortalPacienteService
	logger *slog.Logger
}

// NewPortalHandler construye el manejador del portal del paciente.
func NewPortalHandler(svc *service.PortalPacienteService, logger *slog.Logger) *PortalHandler {
	return &PortalHandler{svc: svc, logger: logger}
}

func (h *PortalHandler) Perfil(w http.ResponseWriter, r *http.Request) {
	actor := actorFromContext(r.Context())
	if actor == nil {
		writeUnauthorized(w)
		return
	}
	paciente, err := h.svc.Perfil(r.Context(), actor)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, nuevoPerfilPacienteResponse(*paciente, actor))
}

func (h *PortalHandler) ActualizarPerfil(w http.ResponseWriter, r *http.Request) {
	actor := actorFromContext(r.Context())
	if actor == nil {
		writeUnauthorized(w)
		return
	}

	var req actualizarPerfilRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	paciente, err := h.svc.ActualizarContacto(r.Context(), actor, req.toService())
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, nuevoPerfilPacienteResponse(*paciente, actor))
}
