package entities

import "time"

// Especialidad es un catálogo de áreas médicas.
type Especialidad struct {
	ID          int64
	Codigo      string
	Nombre      string
	Descripcion string
	Version     int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
