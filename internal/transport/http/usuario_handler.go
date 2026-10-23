package httpapi

import (
	"log/slog"
	"net/http"

	"medilist/internal/service"
)

// UsuarioHandler administra cuentas y roles (solo usuarios autorizados).
type UsuarioHandler struct {
	svc    *service.UsuarioService
	logger *slog.Logger
}

// NewUsuarioHandler construye el manejador.
func NewUsuarioHandler(svc *service.UsuarioService, logger *slog.Logger) *UsuarioHandler {
	return &UsuarioHandler{svc: svc, logger: logger}
}

func (h *UsuarioHandler) Listar(w http.ResponseWriter, r *http.Request) {
	opts := listOptions(r)
	lista, err := h.svc.Listar(r.Context(), opts)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	resp := make([]usuarioResponse, 0, len(lista))
	for _, u := range lista {
		resp = append(resp, nuevoUsuarioResponse(u))
	}
	writeList(w, resp, opts, len(resp))
}

func (h *UsuarioHandler) Crear(w http.ResponseWriter, r *http.Request) {
	var req crearUsuarioRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	usuario, err := h.svc.Crear(r.Context(), service.CrearUsuarioInput{
		Username:  req.Username,
		Email:     req.Email,
		Nombres:   req.Nombres,
		Apellidos: req.Apellidos,
		Password:  req.Password,
		Roles:     req.Roles,
	})
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusCreated, nuevoUsuarioResponse(*usuario))
}

func (h *UsuarioHandler) Obtener(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	usuario, err := h.svc.Obtener(r.Context(), id)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, nuevoUsuarioResponse(*usuario))
}

func (h *UsuarioHandler) Actualizar(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	claims := claimsFromContext(r.Context())
	if claims == nil {
		writeUnauthorized(w)
		return
	}
	var req actualizarUsuarioRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	usuario, err := h.svc.Actualizar(r.Context(), id, service.ActualizarUsuarioInput{
		Email:     req.Email,
		Nombres:   req.Nombres,
		Apellidos: req.Apellidos,
		Activo:    req.Activo,
		Roles:     req.Roles,
		Password:  req.Password,
		Version:   req.Version,
	}, claims.UsuarioID)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, nuevoUsuarioResponse(*usuario))
}

func (h *UsuarioHandler) Desbloquear(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	if err := h.svc.Desbloquear(r.Context(), id); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *UsuarioHandler) RevocarSesiones(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	if err := h.svc.RevocarSesiones(r.Context(), id); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *UsuarioHandler) ListarRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.svc.ListarRoles(r.Context())
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	resp := make([]rolResponse, 0, len(roles))
	for _, rol := range roles {
		resp = append(resp, nuevoRolResponse(rol))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": resp})
}
