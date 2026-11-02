package httpapi

import (
	"log/slog"
	"net/http"

	"medilist/internal/service"
)

// AgendaHandler expone el CRUD administrativo de horarios y bloqueos.
type AgendaHandler struct {
	svc    *service.AgendaService
	logger *slog.Logger
}

// NewAgendaHandler construye el manejador de agenda.
func NewAgendaHandler(svc *service.AgendaService, logger *slog.Logger) *AgendaHandler {
	return &AgendaHandler{svc: svc, logger: logger}
}

func (h *AgendaHandler) ListarHorarios(w http.ResponseWriter, r *http.Request) {
	medicoID, err := pathID(r)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	lista, err := h.svc.ListarHorarios(r.Context(), medicoID)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	resp := make([]horarioResponse, 0, len(lista))
	for _, horario := range lista {
		resp = append(resp, nuevoHorarioResponse(horario))
	}
	writeCollection(w, resp, len(resp))
}

func (h *AgendaHandler) CrearHorario(w http.ResponseWriter, r *http.Request) {
	medicoID, err := pathID(r)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	var req horarioRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	horario, err := h.svc.CrearHorario(r.Context(), medicoID, req.toService())
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusCreated, nuevoHorarioResponse(*horario))
}

func (h *AgendaHandler) ActualizarHorario(w http.ResponseWriter, r *http.Request) {
	medicoID, err := pathID(r)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	horarioID, err := pathInt(r, "horarioId")
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	var req horarioRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	horario, err := h.svc.ActualizarHorario(r.Context(), medicoID, horarioID, req.toService())
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, nuevoHorarioResponse(*horario))
}

func (h *AgendaHandler) DesactivarHorario(w http.ResponseWriter, r *http.Request) {
	medicoID, err := pathID(r)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	horarioID, err := pathInt(r, "horarioId")
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	if err := h.svc.DesactivarHorario(r.Context(), medicoID, horarioID); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AgendaHandler) ListarBloqueos(w http.ResponseWriter, r *http.Request) {
	medicoID, err := pathID(r)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	lista, err := h.svc.ListarBloqueos(
		r.Context(), medicoID,
		r.URL.Query().Get("desde"), r.URL.Query().Get("hasta"),
	)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	resp := make([]bloqueoResponse, 0, len(lista))
	for _, bloqueo := range lista {
		resp = append(resp, nuevoBloqueoResponse(bloqueo))
	}
	writeCollection(w, resp, len(resp))
}

func (h *AgendaHandler) CrearBloqueo(w http.ResponseWriter, r *http.Request) {
	medicoID, err := pathID(r)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	actor := actorFromContext(r.Context())
	if actor == nil {
		writeUnauthorized(w)
		return
	}
	var req bloqueoRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	in, err := req.toService()
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	bloqueo, err := h.svc.CrearBloqueo(r.Context(), medicoID, actor, in)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusCreated, nuevoBloqueoResponse(*bloqueo))
}

func (h *AgendaHandler) ActualizarBloqueo(w http.ResponseWriter, r *http.Request) {
	medicoID, err := pathID(r)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	bloqueoID, err := pathInt(r, "bloqueoId")
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	var req bloqueoRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	in, err := req.toService()
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	bloqueo, err := h.svc.ActualizarBloqueo(r.Context(), medicoID, bloqueoID, in)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, nuevoBloqueoResponse(*bloqueo))
}

func (h *AgendaHandler) EliminarBloqueo(w http.ResponseWriter, r *http.Request) {
	medicoID, err := pathID(r)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	bloqueoID, err := pathInt(r, "bloqueoId")
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	if err := h.svc.EliminarBloqueo(r.Context(), medicoID, bloqueoID); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
