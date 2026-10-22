package httpapi

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"medilist/internal/service"
	"medilist/internal/service/entities"
)

// ErrorHandler expone los errores agregados para auditoría técnica.
type ErrorHandler struct {
	svc    *service.ErrorService
	logger *slog.Logger
}

// NewErrorHandler construye el manejador.
func NewErrorHandler(svc *service.ErrorService, logger *slog.Logger) *ErrorHandler {
	return &ErrorHandler{svc: svc, logger: logger}
}

func (h *ErrorHandler) Listar(w http.ResponseWriter, r *http.Request) {
	filtro := entities.ErrorFiltro{ListOptions: listOptions(r)}
	q := r.URL.Query()
	filtro.Nivel = strings.TrimSpace(q.Get("nivel"))
	filtro.Servicio = strings.TrimSpace(q.Get("servicio"))
	filtro.TraceID = strings.TrimSpace(q.Get("trace_id"))

	lista, err := h.svc.Listar(r.Context(), filtro)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	resp := make([]errorLogResponse, 0, len(lista))
	for _, e := range lista {
		resp = append(resp, nuevoErrorLogResponse(e))
	}
	writeList(w, resp, filtro.ListOptions, len(resp))
}

type errorLogResponse struct {
	ID         int64     `json:"id"`
	TraceID    string    `json:"trace_id"`
	Servicio   string    `json:"servicio"`
	Componente string    `json:"componente"`
	Nivel      string    `json:"nivel"`
	Metodo     string    `json:"metodo"`
	Ruta       string    `json:"ruta"`
	EstadoHTTP int       `json:"estado_http"`
	Codigo     string    `json:"codigo"`
	Mensaje    string    `json:"mensaje"`
	UsuarioID  *int64    `json:"usuario_id,omitempty"`
	Username   string    `json:"username"`
	IP         string    `json:"ip"`
	UserAgent  string    `json:"user_agent"`
	StackTrace string    `json:"stack_trace,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

func nuevoErrorLogResponse(e entities.ErrorAplicacion) errorLogResponse {
	return errorLogResponse{
		ID:         e.ID,
		TraceID:    e.TraceID,
		Servicio:   e.Servicio,
		Componente: e.Componente,
		Nivel:      e.Nivel,
		Metodo:     e.Metodo,
		Ruta:       e.Ruta,
		EstadoHTTP: e.EstadoHTTP,
		Codigo:     e.Codigo,
		Mensaje:    e.Mensaje,
		UsuarioID:  e.UsuarioID,
		Username:   e.Username,
		IP:         e.IP,
		UserAgent:  e.UserAgent,
		StackTrace: e.StackTrace,
		CreatedAt:  e.CreatedAt,
	}
}
