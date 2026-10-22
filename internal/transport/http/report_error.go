package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"

	"medilist/internal/service/entities"
)

// reportarError envía el error al servicio de agregación (stdout + PostgreSQL)
// usando el trace_id y el usuario del contexto de la solicitud.
func reportarError(r *http.Request, logger *slog.Logger, status int, codigo, mensaje string, err error, stack string) {
	entrada := &entities.ErrorAplicacion{
		TraceID:    requestIDFromContext(r.Context()),
		Componente: "http",
		Nivel:      "error",
		Metodo:     r.Method,
		Ruta:       r.URL.Path,
		EstadoHTTP: status,
		Codigo:     codigo,
		Mensaje:    mensaje,
		IP:         clientIP(r),
		UserAgent:  r.UserAgent(),
		StackTrace: stack,
	}
	if meta := requestMetaFromContext(r.Context()); meta != nil {
		entrada.Username = meta.Username
	}
	if claims := claimsFromContext(r.Context()); claims != nil {
		id := claims.UsuarioID
		entrada.UsuarioID = &id
		entrada.Username = claims.Username
	}
	if err != nil {
		if entrada.Mensaje == "" {
			entrada.Mensaje = err.Error()
		} else {
			entrada.Mensaje = fmt.Sprintf("%s: %v", entrada.Mensaje, err)
		}
	}

	if svc := errorServiceFromContext(r.Context()); svc != nil {
		svc.Registrar(r.Context(), entrada)
		return
	}
	logger.Error("error_aplicacion",
		"trace_id", entrada.TraceID,
		"componente", entrada.Componente,
		"metodo", entrada.Metodo,
		"ruta", entrada.Ruta,
		"estado_http", entrada.EstadoHTTP,
		"codigo", entrada.Codigo,
		"mensaje", entrada.Mensaje,
		"error", err,
	)
}
