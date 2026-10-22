package httpapi

import (
	"log/slog"
	"net/http"

	"medilist/internal/service"
	"medilist/internal/service/entities"
)

// CitaHandler expone los endpoints de la agenda de citas.
type CitaHandler struct {
	svc    *service.CitaService
	logger *slog.Logger
}

// NewCitaHandler construye el manejador.
func NewCitaHandler(svc *service.CitaService, logger *slog.Logger) *CitaHandler {
	return &CitaHandler{svc: svc, logger: logger}
}

func (h *CitaHandler) Crear(w http.ResponseWriter, r *http.Request) {
	var req citaRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	in, err := req.toEntity()
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	c, err := h.svc.Crear(r.Context(), in)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusCreated, nuevaCitaResponse(*c))
}

func (h *CitaHandler) Listar(w http.ResponseWriter, r *http.Request) {
	filtro, err := citaFiltro(r)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	lista, err := h.svc.Listar(r.Context(), filtro)
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

func (h *CitaHandler) Obtener(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	c, err := h.svc.Obtener(r.Context(), id)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, nuevaCitaResponse(*c))
}

func (h *CitaHandler) Actualizar(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	var req citaRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	in, err := req.toEntity()
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	c, err := h.svc.Actualizar(r.Context(), id, in)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, nuevaCitaResponse(*c))
}

func (h *CitaHandler) CambiarEstado(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	var req cambiarEstadoRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	c, err := h.svc.CambiarEstado(r.Context(), id, entities.EstadoCita(req.Estado))
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, nuevaCitaResponse(*c))
}

func (h *CitaHandler) Eliminar(w http.ResponseWriter, r *http.Request) {
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
