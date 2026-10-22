package httpapi

import (
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	"medilist/internal/service"
)

// AuthHandler expone login, refresh, logout y perfil.
type AuthHandler struct {
	svc             *service.AuthService
	logger          *slog.Logger
	loginLimiter    *rateLimiter
	refreshLimiter  *rateLimiter
	passwordLimiter *rateLimiter
}

// NewAuthHandler construye el manejador de autenticación.
func NewAuthHandler(svc *service.AuthService, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{
		svc:             svc,
		logger:          logger,
		loginLimiter:    newRateLimiter(30, time.Minute),
		refreshLimiter:  newRateLimiter(60, time.Minute),
		passwordLimiter: newRateLimiter(5, time.Minute),
	}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	permitido, espera := h.loginLimiter.Allow(clientIP(r))
	if !permitido {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(espera.Seconds()))))
		writeError(w, http.StatusTooManyRequests, "rate_limited", "demasiados intentos; espera un momento")
		return
	}

	var req loginRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	sesion, err := h.svc.Login(r.Context(), req.Username, req.Password, clientIP(r), r.UserAgent())
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, nuevaSesionResponse(*sesion))
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	permitido, espera := h.refreshLimiter.Allow(clientIP(r))
	if !permitido {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(espera.Seconds()))))
		writeError(w, http.StatusTooManyRequests, "rate_limited", "demasiadas solicitudes; espera un momento")
		return
	}

	var req refreshTokenRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	sesion, err := h.svc.Refresh(r.Context(), req.RefreshToken, clientIP(r), r.UserAgent())
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, nuevaSesionResponse(*sesion))
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req refreshTokenRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	if err := h.svc.Logout(r.Context(), req.RefreshToken, clientIP(r), r.UserAgent()); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	if claims == nil {
		writeUnauthorized(w)
		return
	}
	usuario, err := h.svc.Me(r.Context(), claims.UsuarioID)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, nuevoUsuarioResponse(*usuario))
}

func (h *AuthHandler) CambiarPassword(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	if claims == nil {
		writeUnauthorized(w)
		return
	}
	permitido, espera := h.passwordLimiter.Allow(claims.Username)
	if !permitido {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(espera.Seconds()))))
		writeError(w, http.StatusTooManyRequests, "rate_limited", "demasiados intentos; espera un momento")
		return
	}

	var req cambiarPasswordRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	if err := h.svc.CambiarPassword(
		r.Context(), claims.UsuarioID, req.PasswordActual, req.PasswordNueva, clientIP(r), r.UserAgent(),
	); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
