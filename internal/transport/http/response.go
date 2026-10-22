package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"medilist/internal/service/entities"
)

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error apiError `json:"error"`
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
	writeJSON(w, status, errorResponse{Error: apiError{Code: code, Message: message}})
}

// writeServiceError traduce errores de dominio a códigos HTTP.
func writeServiceError(w http.ResponseWriter, logger *slog.Logger, err error) {
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
		logger.Error("error interno", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "error interno del servidor")
	}
}
