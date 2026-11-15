package httpapi

import (
	"strings"
	"time"

	"medilist/internal/service"
	"medilist/internal/service/entities"
)

// ---------------------------------------------------------------------------
// Autenticación
// ---------------------------------------------------------------------------

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type refreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type clerkLoginRequest struct {
	Token string `json:"token"`
}

type clerkPendienteResponse struct {
	RegistroPendiente bool   `json:"registro_pendiente"`
	Email             string `json:"email"`
	Nombres           string `json:"nombres"`
	Apellidos         string `json:"apellidos"`
}

type clerkRegistroRequest struct {
	Token           string `json:"token"`
	Nombres         string `json:"nombres"`
	Apellidos       string `json:"apellidos"`
	TipoDocumento   string `json:"tipo_documento"`
	NumeroDocumento string `json:"numero_documento"`
	FechaNacimiento string `json:"fecha_nacimiento"`
	Sexo            string `json:"sexo"`
	Telefono        string `json:"telefono"`
	Direccion       string `json:"direccion"`
}

func (r clerkRegistroRequest) toInput() (service.CompletarRegistroPacienteInput, error) {
	var fecha time.Time
	if texto := strings.TrimSpace(r.FechaNacimiento); texto != "" {
		parsed, err := time.Parse("2006-01-02", texto)
		if err != nil {
			return service.CompletarRegistroPacienteInput{}, entities.NewValidationError(
				"fecha_nacimiento debe tener formato YYYY-MM-DD")
		}
		fecha = parsed
	}
	return service.CompletarRegistroPacienteInput{
		Nombres:         r.Nombres,
		Apellidos:       r.Apellidos,
		TipoDocumento:   r.TipoDocumento,
		NumeroDocumento: r.NumeroDocumento,
		FechaNacimiento: fecha,
		Sexo:            r.Sexo,
		Telefono:        r.Telefono,
		Direccion:       r.Direccion,
	}, nil
}

type cambiarPasswordRequest struct {
	PasswordActual string `json:"password_actual"`
	PasswordNueva  string `json:"password_nueva"`
}

type sesionResponse struct {
	AccessToken  string          `json:"access_token"`
	RefreshToken string          `json:"refresh_token"`
	TokenType    string          `json:"token_type"`
	ExpiresIn    int64           `json:"expires_in"`
	Usuario      usuarioResponse `json:"usuario"`
}

func nuevaSesionResponse(s service.Sesion) sesionResponse {
	return sesionResponse{
		AccessToken:  s.AccessToken,
		RefreshToken: s.RefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    s.ExpiraEn,
		Usuario:      nuevoUsuarioResponse(*s.Usuario),
	}
}

// ---------------------------------------------------------------------------
// Usuarios
// ---------------------------------------------------------------------------

type usuarioResponse struct {
	ID                  int64      `json:"id"`
	Username            string     `json:"username"`
	Email               string     `json:"email"`
	Nombres             string     `json:"nombres"`
	Apellidos           string     `json:"apellidos"`
	Activo              bool       `json:"activo"`
	IntentosFallidos    int        `json:"intentos_fallidos"`
	BloqueadoHasta      *time.Time `json:"bloqueado_hasta,omitempty"`
	UltimoAcceso        *time.Time `json:"ultimo_acceso,omitempty"`
	EmailVerificado     bool       `json:"email_verificado"`
	EmailVerificadoEn   *time.Time `json:"email_verificado_en,omitempty"`
	TerminosAceptadosEn *time.Time `json:"terminos_aceptados_en,omitempty"`
	VersionTerminos     string     `json:"version_terminos,omitempty"`
	Roles               []string   `json:"roles"`
	Permisos            []string   `json:"permisos"`
	Version             int64      `json:"version"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// meResponse extiende el usuario con los perfiles clínicos vinculados. Estos
// IDs son de solo lectura: el cliente no puede enviarlos para elegir dueño.
type meResponse struct {
	usuarioResponse
	PacienteID                *int64 `json:"paciente_id,omitempty"`
	MedicoID                  *int64 `json:"medico_id,omitempty"`
	RequiereCompletarRegistro bool   `json:"requiere_completar_registro"`
}

func nuevoMeResponse(u entities.Usuario, actor *service.Actor) meResponse {
	resp := meResponse{usuarioResponse: nuevoUsuarioResponse(u)}
	if actor != nil {
		resp.PacienteID = actor.PacienteID
		resp.MedicoID = actor.MedicoID
		if u.ClerkUserID != "" && actor.PacienteID == nil && !actor.SolicitudPendiente && soloRolPaciente(u.Roles) {
			resp.RequiereCompletarRegistro = true
		}
	}
	return resp
}

func soloRolPaciente(roles []string) bool {
	if len(roles) == 0 {
		return false
	}
	for _, rol := range roles {
		if rol != "paciente" {
			return false
		}
	}
	return true
}

func nuevoUsuarioResponse(u entities.Usuario) usuarioResponse {
	roles := u.Roles
	if roles == nil {
		roles = []string{}
	}
	permisos := u.Permisos
	if permisos == nil {
		permisos = []string{}
	}
	return usuarioResponse{
		ID:                  u.ID,
		Username:            u.Username,
		Email:               u.Email,
		Nombres:             u.Nombres,
		Apellidos:           u.Apellidos,
		Activo:              u.Activo,
		IntentosFallidos:    u.IntentosFallidos,
		BloqueadoHasta:      u.BloqueadoHasta,
		UltimoAcceso:        u.UltimoAcceso,
		EmailVerificado:     u.EmailVerificado(),
		EmailVerificadoEn:   u.EmailVerificadoEn,
		TerminosAceptadosEn: u.TerminosAceptadosEn,
		VersionTerminos:     u.VersionTerminos,
		Roles:               roles,
		Permisos:            permisos,
		Version:             u.Version,
		CreatedAt:           u.CreatedAt,
		UpdatedAt:           u.UpdatedAt,
	}
}

type crearUsuarioRequest struct {
	Username  string   `json:"username"`
	Email     string   `json:"email"`
	Nombres   string   `json:"nombres"`
	Apellidos string   `json:"apellidos"`
	Roles     []string `json:"roles"`
}

type actualizarUsuarioRequest struct {
	Email     string   `json:"email"`
	Nombres   string   `json:"nombres"`
	Apellidos string   `json:"apellidos"`
	Activo    *bool    `json:"activo"`
	Roles     []string `json:"roles"`
	Version   int64    `json:"version"`
}

// ---------------------------------------------------------------------------
// Roles
// ---------------------------------------------------------------------------

type rolResponse struct {
	ID          int64    `json:"id"`
	Codigo      string   `json:"codigo"`
	Nombre      string   `json:"nombre"`
	Descripcion string   `json:"descripcion"`
	Permisos    []string `json:"permisos"`
}

func nuevoRolResponse(r entities.Rol) rolResponse {
	permisos := r.Permisos
	if permisos == nil {
		permisos = []string{}
	}
	return rolResponse{
		ID:          r.ID,
		Codigo:      r.Codigo,
		Nombre:      r.Nombre,
		Descripcion: r.Descripcion,
		Permisos:    permisos,
	}
}

// ---------------------------------------------------------------------------
// Auditoría
// ---------------------------------------------------------------------------

type auditoriaResponse struct {
	ID         int64     `json:"id"`
	UsuarioID  *int64    `json:"usuario_id,omitempty"`
	Username   string    `json:"username"`
	Accion     string    `json:"accion"`
	Recurso    string    `json:"recurso"`
	Metodo     string    `json:"metodo"`
	Ruta       string    `json:"ruta"`
	EstadoHTTP int       `json:"estado_http"`
	IP         string    `json:"ip"`
	UserAgent  string    `json:"user_agent"`
	TraceID    string    `json:"trace_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

func nuevaAuditoriaResponse(a entities.Auditoria) auditoriaResponse {
	return auditoriaResponse{
		ID:         a.ID,
		UsuarioID:  a.UsuarioID,
		Username:   a.Username,
		Accion:     a.Accion,
		Recurso:    a.Recurso,
		Metodo:     a.Metodo,
		Ruta:       a.Ruta,
		EstadoHTTP: a.EstadoHTTP,
		IP:         a.IP,
		UserAgent:  a.UserAgent,
		TraceID:    a.TraceID,
		CreatedAt:  a.CreatedAt,
	}
}
