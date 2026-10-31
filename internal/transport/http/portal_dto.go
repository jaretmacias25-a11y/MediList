package httpapi

import (
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
