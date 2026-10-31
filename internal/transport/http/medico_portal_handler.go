package httpapi

import (
	"log/slog"
	"net/http"

	"medilist/internal/service"
)

// MedicoPortalHandler expone la agenda y los pacientes propios del médico.
type MedicoPortalHandler struct {
	svc    *service.MedicoPortalService
	logger *slog.Logger
}

// NewMedicoPortalHandler construye el manejador del portal médico.
func NewMedicoPortalHandler(svc *service.MedicoPortalService, logger *slog.Logger) *MedicoPortalHandler {
	return &MedicoPortalHandler{svc: svc, logger: logger}
}

func (h *MedicoPortalHandler) Agenda(w http.ResponseWriter, r *http.Request) {
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
	// El servicio vuelve a forzar el médico del actor. Se limpia aquí para que
	// quede explícito que la query nunca decide el propietario.
	filtro.MedicoID = 0
	filtro.ListOptions = filtro.ListOptions.Normalized()

	lista, err := h.svc.Agenda(r.Context(), actor, filtro)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	resp := make([]citaResponse, 0, len(lista))
	for _, c := range lista {
		resp = append(resp, nuevaCitaResponse(c))
	}
	writeList(w, resp, filtro.ListOptions, len(resp))
}

func (h *MedicoPortalHandler) ObtenerCita(w http.ResponseWriter, r *http.Request) {
	actor := actorFromContext(r.Context())
	if actor == nil {
		writeUnauthorized(w)
		return
	}
	id, err := pathID(r)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	cita, err := h.svc.ObtenerCita(r.Context(), actor, id)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, nuevaCitaResponse(*cita))
}

func (h *MedicoPortalHandler) ObtenerPaciente(w http.ResponseWriter, r *http.Request) {
	actor := actorFromContext(r.Context())
	if actor == nil {
		writeUnauthorized(w)
		return
	}
	id, err := pathID(r)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	paciente, err := h.svc.PacienteAtendido(r.Context(), actor, id)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, nuevoPacienteResponse(*paciente))
}
