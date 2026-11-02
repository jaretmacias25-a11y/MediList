package httpapi

import (
	"log/slog"
	"net/http"

	"medilist/internal/service"
)

// PortalCitaHandler expone la agenda propia del paciente. El paciente se
// resuelve desde el Actor; el body nunca decide paciente_id.
type PortalCitaHandler struct {
	svc    *service.PortalCitaService
	logger *slog.Logger
}

// NewPortalCitaHandler construye el manejador del portal de citas.
func NewPortalCitaHandler(svc *service.PortalCitaService, logger *slog.Logger) *PortalCitaHandler {
	return &PortalCitaHandler{svc: svc, logger: logger}
}

func (h *PortalCitaHandler) Crear(w http.ResponseWriter, r *http.Request) {
	actor := actorFromContext(r.Context())
	if actor == nil {
		writeUnauthorized(w)
		return
	}
	var req portalCitaRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	in, err := req.toService()
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	cita, err := h.svc.Crear(r.Context(), actor, in)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusCreated, nuevaCitaResponse(*cita))
}

func (h *PortalCitaHandler) Listar(w http.ResponseWriter, r *http.Request) {
	actor := actorFromContext(r.Context())
	if actor == nil {
		writeUnauthorized(w)
		return
	}
	filtro, err := citaFiltro(r)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	lista, err := h.svc.Listar(r.Context(), actor, filtro)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	resp := make([]citaResponse, 0, len(lista))
	for _, cita := range lista {
		resp = append(resp, nuevaCitaResponse(cita))
	}
	writeList(w, resp, filtro.ListOptions, len(resp))
}

func (h *PortalCitaHandler) Obtener(w http.ResponseWriter, r *http.Request) {
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
	cita, err := h.svc.Obtener(r.Context(), actor, citaID)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, nuevaCitaResponse(*cita))
}

func (h *PortalCitaHandler) Reprogramar(w http.ResponseWriter, r *http.Request) {
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
	var req portalCitaReprogramarRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	in, err := req.toService()
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	cita, err := h.svc.Reprogramar(r.Context(), actor, citaID, in)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, nuevaCitaResponse(*cita))
}

func (h *PortalCitaHandler) Cancelar(w http.ResponseWriter, r *http.Request) {
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
	var req cancelarCitaRequest
	if err := decodeOptionalJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	cita, err := h.svc.Cancelar(r.Context(), actor, citaID, req.Motivo)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, nuevaCitaResponse(*cita))
}
