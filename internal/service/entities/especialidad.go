package entities

import "time"

// Especialidad es un catálogo de áreas médicas.
type Especialidad struct {
	ID                 int64
	Codigo             string
	Nombre             string
	Descripcion        string
	RequiereDerivacion bool
	Activa             bool
	Orden              int
	DuracionCitaMin    int
	Version            int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// AccesoDirecto indica si el paciente puede agendar esta especialidad sin
// una derivación previa.
func (e Especialidad) AccesoDirecto() bool {
	return e.Activa && !e.RequiereDerivacion
}
