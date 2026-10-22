package httpapi

import (
	"log/slog"
	"net/http"

	"medilist/internal/service"
)

// MedicoHandler expone los endpoints de médicos.
type MedicoHandler struct {
	svc    *service.MedicoService
	logger *slog.Logger
}

// NewMedicoHandler construye el manejador.
func NewMedicoHandler(svc *service.MedicoService, logger *slog.Logger) *MedicoHandler {
	return &MedicoHandler{svc: svc, logger: logger}
}

func (h *MedicoHandler) Crear(w http.ResponseWriter, r *http.Request) {
	var req medicoRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	in, err := req.toEntity()
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	m, err := h.svc.Crear(r.Context(), in)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusCreated, nuevoMedicoResponse(*m))
}

func (h *MedicoHandler) Listar(w http.ResponseWriter, r *http.Request) {
	opts := listOptions(r)
	lista, err := h.svc.Listar(r.Context(), opts)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	resp := make([]medicoResponse, 0, len(lista))
	for _, m := range lista {
		resp = append(resp, nuevoMedicoResponse(m))
	}
	writeList(w, resp, opts, len(resp))
}

func (h *MedicoHandler) Obtener(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	m, err := h.svc.Obtener(r.Context(), id)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, nuevoMedicoResponse(*m))
}

func (h *MedicoHandler) Actualizar(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	var req medicoRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	in, err := req.toEntity()
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	m, err := h.svc.Actualizar(r.Context(), id, in)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, nuevoMedicoResponse(*m))
}

func (h *MedicoHandler) Eliminar(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	if err := h.svc.Eliminar(r.Context(), id); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
