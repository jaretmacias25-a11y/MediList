package httpapi

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"medilist/internal/service"
	"medilist/internal/service/entities"
)

// AuditoriaHandler expone la bitácora de seguridad.
type AuditoriaHandler struct {
	svc    *service.AuditoriaService
	logger *slog.Logger
}

// NewAuditoriaHandler construye el manejador.
func NewAuditoriaHandler(svc *service.AuditoriaService, logger *slog.Logger) *AuditoriaHandler {
	return &AuditoriaHandler{svc: svc, logger: logger}
}

func (h *AuditoriaHandler) Listar(w http.ResponseWriter, r *http.Request) {
	filtro := entities.AuditoriaFiltro{ListOptions: listOptions(r)}
	q := r.URL.Query()
	if v := strings.TrimSpace(q.Get("usuario_id")); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "validation_error", "usuario_id no es válido")
			return
		}
		filtro.UsuarioID = id
	}
	filtro.Accion = strings.TrimSpace(q.Get("accion"))

	lista, err := h.svc.Listar(r.Context(), filtro)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	resp := make([]auditoriaResponse, 0, len(lista))
	for _, a := range lista {
		resp = append(resp, nuevaAuditoriaResponse(a))
	}
	writeList(w, resp, filtro.ListOptions, len(resp))
}
