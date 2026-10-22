package httpapi

import (
	"log/slog"
	"net/http"

	"medilist/internal/service"
	"medilist/internal/service/entities"
)

// EspecialidadHandler expone los endpoints de especialidades.
type EspecialidadHandler struct {
	svc    *service.EspecialidadService
	logger *slog.Logger
}

// NewEspecialidadHandler construye el manejador.
func NewEspecialidadHandler(svc *service.EspecialidadService, logger *slog.Logger) *EspecialidadHandler {
	return &EspecialidadHandler{svc: svc, logger: logger}
}

func (h *EspecialidadHandler) Crear(w http.ResponseWriter, r *http.Request) {
	var req especialidadRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, h.logger, err)
		return
	}
	e, err := h.svc.Crear(r.Context(), entities.Especialidad{
		Codigo:      req.Codigo,
		Nombre:      req.Nombre,
		Descripcion: req.Descripcion,
	})
	if err != nil {
		writeServiceError(w, h.logger, err)
		return
	}
	writeData(w, http.StatusCreated, nuevaEspecialidadResponse(*e))
}

func (h *EspecialidadHandler) Listar(w http.ResponseWriter, r *http.Request) {
	opts := listOptions(r)
	lista, err := h.svc.Listar(r.Context(), opts)
	if err != nil {
		writeServiceError(w, h.logger, err)
		return
	}
	resp := make([]especialidadResponse, 0, len(lista))
	for _, e := range lista {
		resp = append(resp, nuevaEspecialidadResponse(e))
	}
	writeList(w, resp, opts, len(resp))
}

func (h *EspecialidadHandler) Obtener(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeServiceError(w, h.logger, err)
		return
	}
	e, err := h.svc.Obtener(r.Context(), id)
	if err != nil {
		writeServiceError(w, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, nuevaEspecialidadResponse(*e))
}

func (h *EspecialidadHandler) Actualizar(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeServiceError(w, h.logger, err)
		return
	}
	var req especialidadRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, h.logger, err)
		return
	}
	e, err := h.svc.Actualizar(r.Context(), id, entities.Especialidad{
		Codigo:      req.Codigo,
		Nombre:      req.Nombre,
		Descripcion: req.Descripcion,
	})
	if err != nil {
		writeServiceError(w, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, nuevaEspecialidadResponse(*e))
}

func (h *EspecialidadHandler) Eliminar(w http.ResponseWriter, r *http.Request) {
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
