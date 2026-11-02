package httpapi

import (
	"log/slog"
	"net/http"

	"medilist/internal/service"
	"medilist/internal/service/entities"
)

// MedicoClinicoHandler expone las acciones clínicas del médico sobre sus citas.
type MedicoClinicoHandler struct {
	svc    *service.MedicoClinicoService
	logger *slog.Logger
}

// NewMedicoClinicoHandler construye el manejador clínico del médico.
func NewMedicoClinicoHandler(
	svc *service.MedicoClinicoService,
	logger *slog.Logger,
) *MedicoClinicoHandler {
	return &MedicoClinicoHandler{svc: svc, logger: logger}
}

func (h *MedicoClinicoHandler) CambiarEstado(w http.ResponseWriter, r *http.Request) {
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
	var req cambiarEstadoRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	cita, err := h.svc.CambiarEstado(
		r.Context(), actor, citaID, entities.EstadoCita(req.Estado))
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, nuevaCitaResponse(*cita))
}

func (h *MedicoClinicoHandler) ObtenerNota(w http.ResponseWriter, r *http.Request) {
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
	nota, err := h.svc.ObtenerNota(r.Context(), actor, citaID)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, nuevaNotaCitaResponse(*nota))
}

func (h *MedicoClinicoHandler) GuardarNota(w http.ResponseWriter, r *http.Request) {
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
	var req notaCitaRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	nota, err := h.svc.GuardarNota(r.Context(), actor, citaID, req.toService())
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, nuevaNotaCitaResponse(*nota))
}

func (h *MedicoClinicoHandler) CrearDerivacion(w http.ResponseWriter, r *http.Request) {
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
	var req derivacionRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	derivacion, err := h.svc.CrearDerivacion(r.Context(), actor, citaID, req.toService())
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusCreated, nuevaDerivacionResponse(*derivacion))
}

func (h *MedicoClinicoHandler) ListarDerivaciones(w http.ResponseWriter, r *http.Request) {
	actor := actorFromContext(r.Context())
	if actor == nil {
		writeUnauthorized(w)
		return
	}
	opts := listOptions(r)
	lista, err := h.svc.ListarDerivaciones(r.Context(), actor, opts)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	resp := make([]derivacionResponse, 0, len(lista))
	for _, derivacion := range lista {
		resp = append(resp, nuevaDerivacionResponse(derivacion))
	}
	writeList(w, resp, opts, len(resp))
}
