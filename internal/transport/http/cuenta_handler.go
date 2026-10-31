package httpapi

import (
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"medilist/internal/service"
	"medilist/internal/service/entities"
)

// CuentaHandler expone registro, verificación, recuperación y vinculación.
type CuentaHandler struct {
	svc             *service.CuentaService
	logger          *slog.Logger
	registerLimiter *rateLimiter
	verifyLimiter   *rateLimiter
	resendLimiter   *rateLimiter
	recoveryLimiter *rateLimiter
	resetLimiter    *rateLimiter
}

// NewCuentaHandler construye el manejador de cuentas.
func NewCuentaHandler(svc *service.CuentaService, logger *slog.Logger) *CuentaHandler {
	return &CuentaHandler{
		svc:             svc,
		logger:          logger,
		registerLimiter: newRateLimiter(5, time.Hour),
		verifyLimiter:   newRateLimiter(30, time.Minute),
		resendLimiter:   newRateLimiter(3, 15*time.Minute),
		recoveryLimiter: newRateLimiter(5, 15*time.Minute),
		resetLimiter:    newRateLimiter(10, 15*time.Minute),
	}
}

func (h *CuentaHandler) Registrar(w http.ResponseWriter, r *http.Request) {
	if !h.permitir(w, h.registerLimiter, "registro|"+clientIP(r)) {
		return
	}
	var req registroPacienteRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	in, err := req.toInput()
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	resultado, err := h.svc.RegistrarPaciente(r.Context(), in, clientIP(r), r.UserAgent())
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}

	resp := registroPacienteResponse{
		UsuarioID:            resultado.Usuario.ID,
		Email:                resultado.Usuario.Email,
		EmailVerificado:      resultado.Usuario.EmailVerificado(),
		VinculacionPendiente: resultado.VinculacionPendiente,
	}
	if resultado.Paciente != nil {
		id := resultado.Paciente.ID
		resp.PacienteID = &id
	}
	if resultado.Solicitud != nil {
		id := resultado.Solicitud.ID
		resp.SolicitudID = &id
	}
	estado := http.StatusCreated
	resp.Mensaje = "Revisa tu correo para verificar la cuenta."
	if resultado.VinculacionPendiente {
		estado = http.StatusAccepted
		resp.Mensaje = "Revisa tu correo para verificar la cuenta. Recepción revisará la vinculación con tu historia clínica."
	}
	writeData(w, estado, resp)
}

func (h *CuentaHandler) VerificarEmail(w http.ResponseWriter, r *http.Request) {
	if !h.permitir(w, h.verifyLimiter, "verificar|"+clientIP(r)) {
		return
	}
	var req verificarEmailRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	usuario, err := h.svc.VerificarEmail(r.Context(), req.Token, clientIP(r), r.UserAgent())
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, verificarEmailResponse{
		EmailVerificado: usuario.EmailVerificado(),
		Mensaje:         "Correo verificado. Ya puedes iniciar sesión.",
	})
}

func (h *CuentaHandler) ReenviarVerificacion(w http.ResponseWriter, r *http.Request) {
	var req emailCuentaRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	email, err := normalizarEmailCuenta(req.Email)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	if !h.permitir(w, h.resendLimiter, "reenvio|"+clientIP(r)+"|"+email) {
		return
	}
	if err := h.svc.ReenviarVerificacion(r.Context(), email, clientIP(r), r.UserAgent()); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusAccepted, mensajeResponse{
		Mensaje: "Si la cuenta existe y aún no está verificada, enviamos un nuevo enlace de verificación.",
	})
}

func (h *CuentaHandler) SolicitarResetPassword(w http.ResponseWriter, r *http.Request) {
	var req emailCuentaRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	email, err := normalizarEmailCuenta(req.Email)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	if !h.permitir(w, h.recoveryLimiter, "recuperar|"+clientIP(r)+"|"+email) {
		return
	}
	if err := h.svc.SolicitarResetPassword(r.Context(), email, clientIP(r), r.UserAgent()); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusAccepted, mensajeResponse{
		Mensaje: "Si la cuenta existe, enviamos un enlace para restablecer la contraseña.",
	})
}

func (h *CuentaHandler) RestablecerPassword(w http.ResponseWriter, r *http.Request) {
	if !h.permitir(w, h.resetLimiter, "reset|"+clientIP(r)) {
		return
	}
	var req resetPasswordCuentaRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	if err := h.svc.RestablecerPassword(
		r.Context(), req.Token, req.PasswordNueva, clientIP(r), r.UserAgent(),
	); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *CuentaHandler) ListarSolicitudes(w http.ResponseWriter, r *http.Request) {
	filtro, err := filtroSolicitudes(r)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	lista, err := h.svc.ListarSolicitudesVinculacion(r.Context(), filtro)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	resp := make([]solicitudVinculacionResponse, 0, len(lista))
	for _, solicitud := range lista {
		resp = append(resp, nuevaSolicitudVinculacionResponse(solicitud))
	}
	writeList(w, resp, filtro.ListOptions, len(resp))
}

func (h *CuentaHandler) AprobarSolicitud(w http.ResponseWriter, r *http.Request) {
	solicitudID, err := pathID(r)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	claims := claimsFromContext(r.Context())
	if claims == nil {
		writeUnauthorized(w)
		return
	}
	if _, err := h.svc.AprobarSolicitudVinculacion(
		r.Context(), solicitudID, claims.UsuarioID, claims.Username,
		clientIP(r), r.UserAgent(),
	); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *CuentaHandler) RechazarSolicitud(w http.ResponseWriter, r *http.Request) {
	solicitudID, err := pathID(r)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	claims := claimsFromContext(r.Context())
	if claims == nil {
		writeUnauthorized(w)
		return
	}
	var req rechazarSolicitudRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	if _, err := h.svc.RechazarSolicitudVinculacion(
		r.Context(), solicitudID, claims.UsuarioID, claims.Username, req.Motivo,
		clientIP(r), r.UserAgent(),
	); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *CuentaHandler) VincularPaciente(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	if claims == nil {
		writeUnauthorized(w)
		return
	}
	var req vincularPacienteRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	if err := h.svc.VincularPaciente(
		r.Context(), req.PacienteID, req.UsuarioID, req.SolicitudID,
		claims.UsuarioID, claims.Username, clientIP(r), r.UserAgent(),
	); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *CuentaHandler) VincularMedico(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	if claims == nil {
		writeUnauthorized(w)
		return
	}
	var req vincularMedicoRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	if err := h.svc.VincularMedico(
		r.Context(), req.MedicoID, req.UsuarioID, claims.UsuarioID, claims.Username,
		clientIP(r), r.UserAgent(),
	); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *CuentaHandler) permitir(w http.ResponseWriter, limiter *rateLimiter, clave string) bool {
	permitido, espera := limiter.Allow(clave)
	if permitido {
		return true
	}
	w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(espera.Seconds()))))
	writeError(w, http.StatusTooManyRequests, "rate_limited", "demasiados intentos; espera un momento")
	return false
}

func filtroSolicitudes(r *http.Request) (entities.SolicitudVinculacionFiltro, error) {
	filtro := entities.SolicitudVinculacionFiltro{ListOptions: listOptions(r)}
	estado := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("estado")))
	if estado == "" {
		estado = string(entities.SolicitudVinculacionPendiente)
	}
	if estado != "todas" {
		filtro.Estado = entities.EstadoSolicitudVinculacion(estado)
		if !filtro.Estado.Valido() {
			return filtro, entities.NewValidationError("estado debe ser pendiente, aprobada, rechazada o todas")
		}
	}
	return filtro, nil
}

// normalizarEmailCuenta acota el email antes de usarlo como clave del rate
// limiter; de lo contrario un atacante podría crear claves de memoria muy
// grandes con correos únicos y no válidos.
func normalizarEmailCuenta(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "", entities.NewValidationError("email es obligatorio")
	}
	if len(email) > 160 {
		return "", entities.NewValidationError("email no puede superar los 160 caracteres")
	}
	return email, nil
}
