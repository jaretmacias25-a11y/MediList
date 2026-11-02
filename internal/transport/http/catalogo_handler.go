package httpapi

import (
	"log/slog"
	"net/http"

	"medilist/internal/service"
	"medilist/internal/service/entities"
)

// CatalogoHandler expone el catálogo y la disponibilidad visibles para el
// paciente autenticado. Nunca recibe paciente_id ni medico_id desde el cuerpo.
type CatalogoHandler struct {
	catalogo       *service.CatalogoService
	disponibilidad *service.DisponibilidadService
	logger         *slog.Logger
}

// NewCatalogoHandler construye el manejador del catálogo del portal.
func NewCatalogoHandler(
	catalogo *service.CatalogoService,
	disponibilidad *service.DisponibilidadService,
	logger *slog.Logger,
) *CatalogoHandler {
	return &CatalogoHandler{
		catalogo:       catalogo,
		disponibilidad: disponibilidad,
		logger:         logger,
	}
}

func (h *CatalogoHandler) Especialidades(w http.ResponseWriter, r *http.Request) {
	lista, err := h.catalogo.Especialidades(r.Context())
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	resp := make([]especialidadPortalResponse, 0, len(lista))
	for _, especialidad := range lista {
		resp = append(resp, nuevaEspecialidadPortalResponse(especialidad))
	}
	writeCollection(w, resp, len(resp))
}

func (h *CatalogoHandler) Medicos(w http.ResponseWriter, r *http.Request) {
	especialidadID, err := queryInt64(r, "especialidad_id")
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	opts := listOptions(r)
	lista, err := h.catalogo.Medicos(r.Context(), especialidadID, opts)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	resp := make([]medicoPortalResponse, 0, len(lista))
	for _, medico := range lista {
		resp = append(resp, nuevoMedicoPortalResponse(medico))
	}
	writeList(w, resp, opts, len(resp))
}

func (h *CatalogoHandler) Disponibilidad(w http.ResponseWriter, r *http.Request) {
	medicoID, err := queryInt64(r, "medico_id")
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	if medicoID == 0 {
		writeServiceError(w, r, h.logger, entities.NewValidationError("medico_id es obligatorio"))
		return
	}
	disponibilidad, err := h.disponibilidad.Consultar(
		r.Context(), medicoID,
		r.URL.Query().Get("desde"), r.URL.Query().Get("hasta"),
	)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, nuevaDisponibilidadResponse(*disponibilidad))
}
