package httpapi

import (
	"strings"
	"time"

	"medilist/internal/service/entities"
)

// ---------------------------------------------------------------------------
// Persona
// ---------------------------------------------------------------------------

type personaRequest struct {
	TipoDocumento   string `json:"tipo_documento"`
	NumeroDocumento string `json:"numero_documento"`
	Nombres         string `json:"nombres"`
	Apellidos       string `json:"apellidos"`
	FechaNacimiento string `json:"fecha_nacimiento"`
	Sexo            string `json:"sexo"`
	Telefono        string `json:"telefono"`
	Email           string `json:"email"`
	Direccion       string `json:"direccion"`
}

func (r personaRequest) toEntity() (entities.Persona, error) {
	var fecha time.Time
	if texto := strings.TrimSpace(r.FechaNacimiento); texto != "" {
		parsed, err := time.Parse("2006-01-02", texto)
		if err != nil {
			return entities.Persona{}, entities.NewValidationError(
				"persona.fecha_nacimiento debe tener formato YYYY-MM-DD")
		}
		fecha = parsed
	}
	return entities.Persona{
		TipoDocumento:   r.TipoDocumento,
		NumeroDocumento: r.NumeroDocumento,
		Nombres:         r.Nombres,
		Apellidos:       r.Apellidos,
		FechaNacimiento: fecha,
		Sexo:            r.Sexo,
		Telefono:        r.Telefono,
		Email:           r.Email,
		Direccion:       r.Direccion,
	}, nil
}

type personaResponse struct {
	ID              int64  `json:"id"`
	TipoDocumento   string `json:"tipo_documento"`
	NumeroDocumento string `json:"numero_documento"`
	Nombres         string `json:"nombres"`
	Apellidos       string `json:"apellidos"`
	FechaNacimiento string `json:"fecha_nacimiento"`
	Sexo            string `json:"sexo"`
	Telefono        string `json:"telefono"`
	Email           string `json:"email"`
	Direccion       string `json:"direccion"`
}

func nuevaPersonaResponse(p entities.Persona) personaResponse {
	return personaResponse{
		ID:              p.ID,
		TipoDocumento:   p.TipoDocumento,
		NumeroDocumento: p.NumeroDocumento,
		Nombres:         p.Nombres,
		Apellidos:       p.Apellidos,
		FechaNacimiento: p.FechaNacimiento.Format("2006-01-02"),
		Sexo:            p.Sexo,
		Telefono:        p.Telefono,
		Email:           p.Email,
		Direccion:       p.Direccion,
	}
}

// ---------------------------------------------------------------------------
// Especialidad
// ---------------------------------------------------------------------------

type especialidadRequest struct {
	Codigo      string `json:"codigo"`
	Nombre      string `json:"nombre"`
	Descripcion string `json:"descripcion"`
}

type especialidadResponse struct {
	ID          int64     `json:"id"`
	Codigo      string    `json:"codigo"`
	Nombre      string    `json:"nombre"`
	Descripcion string    `json:"descripcion"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func nuevaEspecialidadResponse(e entities.Especialidad) especialidadResponse {
	return especialidadResponse{
		ID:          e.ID,
		Codigo:      e.Codigo,
		Nombre:      e.Nombre,
		Descripcion: e.Descripcion,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

// ---------------------------------------------------------------------------
// Paciente
// ---------------------------------------------------------------------------

type pacienteRequest struct {
	Persona                    personaRequest `json:"persona"`
	GrupoSanguineo             string         `json:"grupo_sanguineo"`
	FactorRH                   string         `json:"factor_rh"`
	ContactoEmergenciaNombre   string         `json:"contacto_emergencia_nombre"`
	ContactoEmergenciaTelefono string         `json:"contacto_emergencia_telefono"`
}

func (r pacienteRequest) toEntity() (entities.Paciente, error) {
	persona, err := r.Persona.toEntity()
	if err != nil {
		return entities.Paciente{}, err
	}
	return entities.Paciente{
		Persona:                    persona,
		GrupoSanguineo:             r.GrupoSanguineo,
		FactorRH:                   r.FactorRH,
		ContactoEmergenciaNombre:   r.ContactoEmergenciaNombre,
		ContactoEmergenciaTelefono: r.ContactoEmergenciaTelefono,
	}, nil
}

type pacienteResponse struct {
	ID                         int64           `json:"id"`
	Persona                    personaResponse `json:"persona"`
	NumeroHistoria             string          `json:"numero_historia"`
	GrupoSanguineo             string          `json:"grupo_sanguineo"`
	FactorRH                   string          `json:"factor_rh"`
	ContactoEmergenciaNombre   string          `json:"contacto_emergencia_nombre"`
	ContactoEmergenciaTelefono string          `json:"contacto_emergencia_telefono"`
	Activo                     bool            `json:"activo"`
	CreatedAt                  time.Time       `json:"created_at"`
	UpdatedAt                  time.Time       `json:"updated_at"`
}

func nuevoPacienteResponse(p entities.Paciente) pacienteResponse {
	return pacienteResponse{
		ID:                         p.ID,
		Persona:                    nuevaPersonaResponse(p.Persona),
		NumeroHistoria:             p.NumeroHistoria,
		GrupoSanguineo:             p.GrupoSanguineo,
		FactorRH:                   p.FactorRH,
		ContactoEmergenciaNombre:   p.ContactoEmergenciaNombre,
		ContactoEmergenciaTelefono: p.ContactoEmergenciaTelefono,
		Activo:                     p.Activo,
		CreatedAt:                  p.CreatedAt,
		UpdatedAt:                  p.UpdatedAt,
	}
}

// ---------------------------------------------------------------------------
// Médico
// ---------------------------------------------------------------------------

type medicoRequest struct {
	Persona         personaRequest `json:"persona"`
	EspecialidadID  int64          `json:"especialidad_id"`
	NumeroLicencia  string         `json:"numero_licencia"`
	TipoProfesional string         `json:"tipo_profesional"`
}

func (r medicoRequest) toEntity() (entities.Medico, error) {
	persona, err := r.Persona.toEntity()
	if err != nil {
		return entities.Medico{}, err
	}
	return entities.Medico{
		Persona:         persona,
		EspecialidadID:  r.EspecialidadID,
		NumeroLicencia:  r.NumeroLicencia,
		TipoProfesional: r.TipoProfesional,
	}, nil
}

type medicoResponse struct {
	ID              int64                `json:"id"`
	Persona         personaResponse      `json:"persona"`
	Especialidad    especialidadResponse `json:"especialidad"`
	NumeroLicencia  string               `json:"numero_licencia"`
	TipoProfesional string               `json:"tipo_profesional"`
	Activo          bool                 `json:"activo"`
	CreatedAt       time.Time            `json:"created_at"`
	UpdatedAt       time.Time            `json:"updated_at"`
}

func nuevoMedicoResponse(m entities.Medico) medicoResponse {
	return medicoResponse{
		ID:              m.ID,
		Persona:         nuevaPersonaResponse(m.Persona),
		Especialidad:    nuevaEspecialidadResponse(m.Especialidad),
		NumeroLicencia:  m.NumeroLicencia,
		TipoProfesional: m.TipoProfesional,
		Activo:          m.Activo,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
}

// ---------------------------------------------------------------------------
// Cita
// ---------------------------------------------------------------------------

type citaRequest struct {
	PacienteID      int64  `json:"paciente_id"`
	MedicoID        int64  `json:"medico_id"`
	FechaHoraInicio string `json:"fecha_hora_inicio"`
	DuracionMin     int    `json:"duracion_min"`
	Tipo            string `json:"tipo"`
	Motivo          string `json:"motivo"`
	Estado          string `json:"estado"`
}

func (r citaRequest) toEntity() (entities.Cita, error) {
	var fecha time.Time
	if texto := strings.TrimSpace(r.FechaHoraInicio); texto != "" {
		parsed, err := time.Parse(time.RFC3339, texto)
		if err != nil {
			return entities.Cita{}, entities.NewValidationError(
				"fecha_hora_inicio debe tener formato RFC3339, por ejemplo 2026-11-05T09:00:00-05:00")
		}
		fecha = parsed
	}
	return entities.Cita{
		PacienteID:      r.PacienteID,
		MedicoID:        r.MedicoID,
		FechaHoraInicio: fecha,
		DuracionMin:     r.DuracionMin,
		Tipo:            entities.TipoCita(strings.TrimSpace(r.Tipo)),
		Motivo:          r.Motivo,
		Estado:          entities.EstadoCita(strings.TrimSpace(r.Estado)),
	}, nil
}

type citaResponse struct {
	ID                 int64     `json:"id"`
	Codigo             string    `json:"codigo"`
	PacienteID         int64     `json:"paciente_id"`
	PacienteNombre     string    `json:"paciente_nombre"`
	MedicoID           int64     `json:"medico_id"`
	MedicoNombre       string    `json:"medico_nombre"`
	EspecialidadNombre string    `json:"especialidad_nombre"`
	FechaHoraInicio    time.Time `json:"fecha_hora_inicio"`
	DuracionMin        int       `json:"duracion_min"`
	Tipo               string    `json:"tipo"`
	Motivo             string    `json:"motivo"`
	Estado             string    `json:"estado"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func nuevaCitaResponse(c entities.Cita) citaResponse {
	return citaResponse{
		ID:                 c.ID,
		Codigo:             c.Codigo,
		PacienteID:         c.PacienteID,
		PacienteNombre:     c.PacienteNombre,
		MedicoID:           c.MedicoID,
		MedicoNombre:       c.MedicoNombre,
		EspecialidadNombre: c.EspecialidadNombre,
		FechaHoraInicio:    c.FechaHoraInicio,
		DuracionMin:        c.DuracionMin,
		Tipo:               string(c.Tipo),
		Motivo:             c.Motivo,
		Estado:             string(c.Estado),
		CreatedAt:          c.CreatedAt,
		UpdatedAt:          c.UpdatedAt,
	}
}

type cambiarEstadoRequest struct {
	Estado string `json:"estado"`
}
