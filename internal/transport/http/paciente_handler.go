package httpapi

import (
	"log/slog"
	"net/http"

	"medilist/internal/service"
)

// PacienteHandler expone los endpoints de pacientes.
type PacienteHandler struct {
	svc    *service.PacienteService
	logger *slog.Logger
}

// NewPacienteHandler construye el manejador.
func NewPacienteHandler(svc *service.PacienteService, logger *slog.Logger) *PacienteHandler {
	return &PacienteHandler{svc: svc, logger: logger}
}

func (h *PacienteHandler) Crear(w http.ResponseWriter, r *http.Request) {
	var req pacienteRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, h.logger, err)
		return
	}
	in, err := req.toEntity()
	if err != nil {
		writeServiceError(w, h.logger, err)
		return
	}
	p, err := h.svc.Crear(r.Context(), in)
	if err != nil {
		writeServiceError(w, h.logger, err)
		return
	}
	writeData(w, http.StatusCreated, nuevoPacienteResponse(*p))
}

func (h *PacienteHandler) Listar(w http.ResponseWriter, r *http.Request) {
	opts := listOptions(r)
	lista, err := h.svc.Listar(r.Context(), opts)
	if err != nil {
		writeServiceError(w, h.logger, err)
		return
	}
	resp := make([]pacienteResponse, 0, len(lista))
	for _, p := range lista {
		resp = append(resp, nuevoPacienteResponse(p))
	}
	writeList(w, resp, opts, len(resp))
}

func (h *PacienteHandler) Obtener(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeServiceError(w, h.logger, err)
		return
	}
	p, err := h.svc.Obtener(r.Context(), id)
	if err != nil {
		writeServiceError(w, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, nuevoPacienteResponse(*p))
}

func (h *PacienteHandler) Actualizar(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeServiceError(w, h.logger, err)
		return
	}
	var req pacienteRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, h.logger, err)
		return
	}
	in, err := req.toEntity()
	if err != nil {
		writeServiceError(w, h.logger, err)
		return
	}
	p, err := h.svc.Actualizar(r.Context(), id, in)
	if err != nil {
		writeServiceError(w, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, nuevoPacienteResponse(*p))
}

func (h *PacienteHandler) Eliminar(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeServiceError(w, h.logger, err)
		return
	}
	if err := h.svc.Eliminar(r.Context(), id); err != nil {
		writeServiceError(w, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
