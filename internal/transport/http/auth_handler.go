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
	cuentas         *service.CuentaService
	logger          *slog.Logger
	loginLimiter    *rateLimiter
	refreshLimiter  *rateLimiter
	passwordLimiter *rateLimiter
}

// NewAuthHandler construye el manejador de autenticación.
func NewAuthHandler(svc *service.AuthService, logger *slog.Logger, cuentas ...*service.CuentaService) *AuthHandler {
	h := &AuthHandler{
		svc:             svc,
		logger:          logger,
		loginLimiter:    newRateLimiter(30, time.Minute),
		refreshLimiter:  newRateLimiter(60, time.Minute),
		passwordLimiter: newRateLimiter(5, time.Minute),
	}
	if len(cuentas) > 0 {
		h.cuentas = cuentas[0]
	}
	return h
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

// ClerkLogin intercambia un token de sesión de Clerk por una sesión local.
func (h *AuthHandler) ClerkLogin(w http.ResponseWriter, r *http.Request) {
	permitido, espera := h.loginLimiter.Allow(clientIP(r))
	if !permitido {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(espera.Seconds()))))
		writeError(w, http.StatusTooManyRequests, "rate_limited", "demasiados intentos; espera un momento")
		return
	}

	var req clerkLoginRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	resultado, err := h.svc.LoginClerk(r.Context(), req.Token, clientIP(r), r.UserAgent())
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	if resultado.Pendiente != nil {
		writeData(w, http.StatusOK, clerkPendienteResponse{
			RegistroPendiente: true,
			Email:             resultado.Pendiente.Email,
			Nombres:           resultado.Pendiente.Nombres,
			Apellidos:         resultado.Pendiente.Apellidos,
		})
		return
	}
	writeData(w, http.StatusOK, nuevaSesionResponse(*resultado.Sesion))
}

// RegistrarClerk crea la cuenta local después de que Clerk autenticó al
// usuario y este completó sus datos clínicos.
func (h *AuthHandler) RegistrarClerk(w http.ResponseWriter, r *http.Request) {
	if h.cuentas == nil {
		writeError(w, http.StatusServiceUnavailable, "clerk_not_configured",
			"el registro con Clerk no está configurado")
		return
	}
	permitido, espera := h.loginLimiter.Allow(clientIP(r))
	if !permitido {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(espera.Seconds()))))
		writeError(w, http.StatusTooManyRequests, "rate_limited", "demasiados intentos; espera un momento")
		return
	}

	var req clerkRegistroRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	identidad, err := h.svc.VerificarClerk(r.Context(), req.Token)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	in, err := req.toInput()
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	if _, err := h.cuentas.CompletarRegistroClerk(
		r.Context(), identidad, in, clientIP(r), r.UserAgent(),
	); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}

	resultado, err := h.svc.LoginClerk(r.Context(), req.Token, clientIP(r), r.UserAgent())
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	if resultado.Sesion == nil {
		writeError(w, http.StatusConflict, "registro_pendiente",
			"la cuenta se creó pero aún requiere completar la vinculación")
		return
	}
	writeData(w, http.StatusCreated, nuevaSesionResponse(*resultado.Sesion))
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
	writeData(w, http.StatusOK, nuevoMeResponse(*usuario, actorFromContext(r.Context())))
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
