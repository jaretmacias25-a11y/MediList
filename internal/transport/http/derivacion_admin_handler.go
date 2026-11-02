package httpapi

import (
	"log/slog"
	"net/http"

	"medilist/internal/service"
)

// DerivacionAdminHandler expone la excepción administrativa de derivación.
type DerivacionAdminHandler struct {
	svc    *service.DerivacionAdminService
	logger *slog.Logger
}

// NewDerivacionAdminHandler construye el manejador.
func NewDerivacionAdminHandler(
	svc *service.DerivacionAdminService,
	logger *slog.Logger,
) *DerivacionAdminHandler {
	return &DerivacionAdminHandler{svc: svc, logger: logger}
}

func (h *DerivacionAdminHandler) Sobrescribir(w http.ResponseWriter, r *http.Request) {
	actor := actorFromContext(r.Context())
	if actor == nil {
		writeUnauthorized(w)
		return
	}
	citaID, err := pathID(r)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	var req sobrescribirDerivacionRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	excepcion, err := h.svc.Sobrescribir(r.Context(), actor, citaID, req.Motivo)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, nuevaExcepcionDerivacionResponse(*excepcion))
}
