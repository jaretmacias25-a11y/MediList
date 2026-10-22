package entities

import "time"

// Medico es un profesional de la salud habilitado para atender citas.
type Medico struct {
	ID              int64
	Persona         Persona
	EspecialidadID  int64
	Especialidad    Especialidad
	NumeroLicencia  string
	TipoProfesional string
	Activo          bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
