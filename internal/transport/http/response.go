package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"

	"medilist/internal/service/entities"
)

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error     apiError `json:"error"`
	RequestID string   `json:"request_id,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if payload != nil {
		_ = json.NewEncoder(w).Encode(payload)
	}
}

func writeData(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, map[string]any{"data": data})
}

func writeList(w http.ResponseWriter, data any, opts entities.ListOptions, count int) {
	writeJSON(w, http.StatusOK, map[string]any{
		"data": data,
		"meta": map[string]any{
			"limit":  opts.Limit,
			"offset": opts.Offset,
			"count":  count,
		},
	})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	requestID := w.Header().Get("X-Request-ID")
	writeJSON(w, status, errorResponse{
		Error:     apiError{Code: code, Message: message},
		RequestID: requestID,
	})
}

// writeServiceError traduce errores de dominio a códigos HTTP y registra los
// errores internos (5xx) en la agregación de logs.
func writeServiceError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, entities.ErrValidation):
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
	case errors.Is(err, entities.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, entities.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", err.Error())
	case errors.Is(err, entities.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
	case errors.Is(err, entities.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", err.Error())
	case errors.Is(err, entities.ErrLocked):
		writeError(w, http.StatusLocked, "locked", err.Error())
	case errors.Is(err, entities.ErrRateLimited):
		writeError(w, http.StatusTooManyRequests, "rate_limited", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "error interno del servidor")
		reportarError(r, logger, http.StatusInternalServerError, "internal_error",
			"error interno del servidor", err, string(debug.Stack()))
	}
}
