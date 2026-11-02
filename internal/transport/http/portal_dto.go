package httpapi

import (
	"strings"
	"time"

	"medilist/internal/service"
	"medilist/internal/service/entities"
)

type actualizarPerfilRequest struct {
	Telefono                   string `json:"telefono"`
	Direccion                  string `json:"direccion"`
	ContactoEmergenciaNombre   string `json:"contacto_emergencia_nombre"`
	ContactoEmergenciaTelefono string `json:"contacto_emergencia_telefono"`
	Version                    int64  `json:"version"`
}

func (r actualizarPerfilRequest) toService() service.DatosContactoPaciente {
	return service.DatosContactoPaciente{
		Telefono:                   r.Telefono,
		Direccion:                  r.Direccion,
		ContactoEmergenciaNombre:   r.ContactoEmergenciaNombre,
		ContactoEmergenciaTelefono: r.ContactoEmergenciaTelefono,
		Version:                    r.Version,
	}
}

type portalCitaRequest struct {
	MedicoID        int64  `json:"medico_id"`
	FechaHoraInicio string `json:"fecha_hora_inicio"`
	Tipo            string `json:"tipo"`
	Motivo          string `json:"motivo"`
	DerivacionID    *int64 `json:"derivacion_id"`
}

func (r portalCitaRequest) toService() (service.PortalCitaInput, error) {
	fecha, err := fechaHoraRFC3339(r.FechaHoraInicio)
	if err != nil {
		return service.PortalCitaInput{}, err
	}
	return service.PortalCitaInput{
		MedicoID:        r.MedicoID,
		FechaHoraInicio: fecha,
		Tipo:            entities.TipoCita(strings.TrimSpace(r.Tipo)),
		Motivo:          r.Motivo,
		DerivacionID:    r.DerivacionID,
	}, nil
}

type portalCitaReprogramarRequest struct {
	FechaHoraInicio string  `json:"fecha_hora_inicio"`
	Motivo          *string `json:"motivo"`
	Version         *int64  `json:"version"`
}

func (r portalCitaReprogramarRequest) toService() (service.PortalCitaReprogramarInput, error) {
	fecha, err := fechaHoraRFC3339(r.FechaHoraInicio)
	if err != nil {
		return service.PortalCitaReprogramarInput{}, err
	}
	return service.PortalCitaReprogramarInput{
		FechaHoraInicio: fecha,
		Motivo:          r.Motivo,
		Version:         r.Version,
	}, nil
}

type cancelarCitaRequest struct {
	Motivo string `json:"motivo"`
}

func fechaHoraRFC3339(texto string) (time.Time, error) {
	texto = strings.TrimSpace(texto)
	if texto == "" {
		return time.Time{}, entities.NewValidationError(
			"fecha_hora_inicio es obligatoria (formato RFC3339)")
	}
	fecha, err := time.Parse(time.RFC3339, texto)
	if err != nil {
		return time.Time{}, entities.NewValidationError(
			"fecha_hora_inicio debe tener formato RFC3339, por ejemplo 2026-11-05T09:00:00-05:00")
	}
	return fecha, nil
}

type cuentaPerfilResponse struct {
	UsuarioID       int64  `json:"usuario_id"`
	Username        string `json:"username"`
	Email           string `json:"email"`
	EmailVerificado bool   `json:"email_verificado"`
	PacienteID      *int64 `json:"paciente_id,omitempty"`
	MedicoID        *int64 `json:"medico_id,omitempty"`
}

type perfilPacienteResponse struct {
	Paciente pacienteResponse     `json:"paciente"`
	Cuenta   cuentaPerfilResponse `json:"cuenta"`
}

func nuevoPerfilPacienteResponse(p entities.Paciente, actor *service.Actor) perfilPacienteResponse {
	resp := perfilPacienteResponse{
		Paciente: nuevoPacienteResponse(p),
	}
	if actor != nil {
		resp.Cuenta = cuentaPerfilResponse{
			UsuarioID:       actor.UsuarioID,
			Username:        actor.Username,
			Email:           actor.Email,
			EmailVerificado: actor.EmailVerificado,
			PacienteID:      actor.PacienteID,
			MedicoID:        actor.MedicoID,
		}
	}
	return resp
}
