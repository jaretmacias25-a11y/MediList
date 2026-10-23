package entities

import "time"

// Persona agrupa los datos de identidad comunes a pacientes y profesionales.
type Persona struct {
	ID              int64
	TipoDocumento   string
	NumeroDocumento string
	Nombres         string
	Apellidos       string
	FechaNacimiento time.Time
	Sexo            string
	Telefono        string
	Email           string
	Direccion       string
}

// NombreCompleto devuelve nombres y apellidos unidos.
func (p Persona) NombreCompleto() string {
	return p.Nombres + " " + p.Apellidos
}

// Paciente es el perfil clínico-administrativo de una persona.
type Paciente struct {
	ID                         int64
	Persona                    Persona
	NumeroHistoria             string
	GrupoSanguineo             string
	FactorRH                   string
	ContactoEmergenciaNombre   string
	ContactoEmergenciaTelefono string
	Activo                     bool
	Version                    int64
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
}
