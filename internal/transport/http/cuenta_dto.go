package httpapi

import (
	"strings"
	"time"

	"medilist/internal/service"
	"medilist/internal/service/entities"
)

// ---------------------------------------------------------------------------
// Registro y recuperación
// ---------------------------------------------------------------------------

type registroPacienteRequest struct {
	Username        string `json:"username"`
	Email           string `json:"email"`
	Password        string `json:"password"`
	Nombres         string `json:"nombres"`
	Apellidos       string `json:"apellidos"`
	TipoDocumento   string `json:"tipo_documento"`
	NumeroDocumento string `json:"numero_documento"`
	FechaNacimiento string `json:"fecha_nacimiento"`
	Sexo            string `json:"sexo"`
	Telefono        string `json:"telefono"`
	Direccion       string `json:"direccion"`
	AceptaTerminos  bool   `json:"acepta_terminos"`
	VersionTerminos string `json:"version_terminos"`
}

func (r registroPacienteRequest) toInput() (service.RegistroPacienteInput, error) {
	var fecha time.Time
	if texto := strings.TrimSpace(r.FechaNacimiento); texto != "" {
		parsed, err := time.Parse("2006-01-02", texto)
		if err != nil {
			return service.RegistroPacienteInput{}, entities.NewValidationError(
				"fecha_nacimiento debe tener formato YYYY-MM-DD")
		}
		fecha = parsed
	}
	return service.RegistroPacienteInput{
		Username:        r.Username,
		Email:           r.Email,
		Password:        r.Password,
		Nombres:         r.Nombres,
		Apellidos:       r.Apellidos,
		TipoDocumento:   r.TipoDocumento,
		NumeroDocumento: r.NumeroDocumento,
		FechaNacimiento: fecha,
		Sexo:            r.Sexo,
		Telefono:        r.Telefono,
		Direccion:       r.Direccion,
		AceptaTerminos:  r.AceptaTerminos,
		VersionTerminos: r.VersionTerminos,
	}, nil
}

type registroPacienteResponse struct {
	UsuarioID            int64  `json:"usuario_id"`
	PacienteID           *int64 `json:"paciente_id,omitempty"`
	SolicitudID          *int64 `json:"solicitud_id,omitempty"`
	Email                string `json:"email"`
	EmailVerificado      bool   `json:"email_verificado"`
	VinculacionPendiente bool   `json:"vinculacion_pendiente"`
	Mensaje              string `json:"mensaje"`
}

type verificarEmailRequest struct {
	Token string `json:"token"`
}

type emailCuentaRequest struct {
	Email string `json:"email"`
}

type resetPasswordCuentaRequest struct {
	Token         string `json:"token"`
	PasswordNueva string `json:"password_nueva"`
}

type mensajeResponse struct {
	Mensaje string `json:"mensaje"`
}

type verificarEmailResponse struct {
	EmailVerificado bool   `json:"email_verificado"`
	Mensaje         string `json:"mensaje"`
}

// ---------------------------------------------------------------------------
// Vinculación de cuentas
// ---------------------------------------------------------------------------

type vincularPacienteRequest struct {
	PacienteID  int64  `json:"paciente_id"`
	UsuarioID   int64  `json:"usuario_id"`
	SolicitudID *int64 `json:"solicitud_id,omitempty"`
}

type vincularMedicoRequest struct {
	MedicoID  int64 `json:"medico_id"`
	UsuarioID int64 `json:"usuario_id"`
}

type rechazarSolicitudRequest struct {
	Motivo string `json:"motivo"`
}

type solicitudVinculacionResponse struct {
	ID                   int64      `json:"id"`
	UsuarioID            int64      `json:"usuario_id"`
	Username             string     `json:"username"`
	Email                string     `json:"email"`
	PacienteID           int64      `json:"paciente_id"`
	NumeroHistoria       string     `json:"numero_historia"`
	Nombres              string     `json:"nombres"`
	Apellidos            string     `json:"apellidos"`
	TipoDocumento        string     `json:"tipo_documento"`
	NumeroDocumento      string     `json:"numero_documento"`
	Estado               string     `json:"estado"`
	Motivo               string     `json:"motivo"`
	ResueltaPorUsuarioID *int64     `json:"resuelta_por_usuario_id,omitempty"`
	ResueltaEn           *time.Time `json:"resuelta_en,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

func nuevaSolicitudVinculacionResponse(s entities.SolicitudVinculacion) solicitudVinculacionResponse {
	return solicitudVinculacionResponse{
		ID:                   s.ID,
		UsuarioID:            s.UsuarioID,
		Username:             s.Username,
		Email:                s.Email,
		PacienteID:           s.PacienteID,
		NumeroHistoria:       s.NumeroHistoria,
		Nombres:              s.Nombres,
		Apellidos:            s.Apellidos,
		TipoDocumento:        s.TipoDocumento,
		NumeroDocumento:      s.NumeroDocumento,
		Estado:               string(s.Estado),
		Motivo:               s.Motivo,
		ResueltaPorUsuarioID: s.ResueltaPorUsuarioID,
		ResueltaEn:           s.ResueltaEn,
		CreatedAt:            s.CreatedAt,
		UpdatedAt:            s.UpdatedAt,
	}
}
