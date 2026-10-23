package service

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"medilist/internal/service/entities"
)

// ErrorService registra errores estructurados y los persiste para su consulta.
// El log estructurado va a stdout y la fila a PostgreSQL, de modo que un
// agregador de logs (Loki, ELK, CloudWatch) pueda correlacionar por trace_id.
type ErrorService struct {
	store     ErrorStore
	servicio  string
	instancia string
	logger    *slog.Logger
}

// NewErrorService construye el servicio de errores agregados.
func NewErrorService(store ErrorStore, servicio, instancia string, logger *slog.Logger) *ErrorService {
	return &ErrorService{store: store, servicio: servicio, instancia: instancia, logger: logger}
}

// Registrar guarda el error en stdout y en la base. Es best-effort: un fallo
// al persistir no debe romper la respuesta original. No usa el contexto de la
// solicitud para no perder el registro si el cliente cancela.
func (s *ErrorService) Registrar(_ context.Context, e *entities.ErrorAplicacion) {
	if e == nil {
		return
	}
	if e.Servicio == "" {
		e.Servicio = s.servicio
	}
	if e.Instancia == "" {
		e.Instancia = s.instancia
	}
	if e.Nivel == "" {
		e.Nivel = "error"
	}
	if e.TraceID == "" {
		e.TraceID = "sin-trace"
	}

	var usuarioID int64
	if e.UsuarioID != nil {
		usuarioID = *e.UsuarioID
	}
	args := []any{
		"trace_id", e.TraceID,
		"servicio", e.Servicio,
		"instancia", e.Instancia,
		"nivel", e.Nivel,
		"componente", e.Componente,
		"metodo", e.Metodo,
		"ruta", e.Ruta,
		"estado_http", e.EstadoHTTP,
		"codigo", e.Codigo,
		"mensaje", e.Mensaje,
		"usuario_id", usuarioID,
		"usuario", e.Username,
		"ip", e.IP,
		"user_agent", e.UserAgent,
		"stack_trace", e.StackTrace,
	}
	if e.Nivel == "warning" {
		s.logger.Warn("error_aplicacion", args...)
	} else {
		s.logger.Error("error_aplicacion", args...)
	}

	persistCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.store.Registrar(persistCtx, e); err != nil {
		s.logger.Error("no se pudo persistir el error agregado",
			"trace_id", e.TraceID, "error", err)
	}
}

func (s *ErrorService) Listar(ctx context.Context, filtro entities.ErrorFiltro) ([]entities.ErrorAplicacion, error) {
	filtro.ListOptions = filtro.ListOptions.Normalized()
	filtro.Search = strings.TrimSpace(filtro.Search)
	filtro.Nivel = strings.ToLower(strings.TrimSpace(filtro.Nivel))
	filtro.Servicio = strings.TrimSpace(filtro.Servicio)
	filtro.Instancia = strings.TrimSpace(filtro.Instancia)
	filtro.TraceID = strings.TrimSpace(filtro.TraceID)
	return s.store.Listar(ctx, filtro)
}
