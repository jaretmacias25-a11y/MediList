package entities

import "time"

// NotaCita es la nota mínima de la consulta. Solo puede escribirla el médico
// de la cita y existe como máximo una por cita.
type NotaCita struct {
	ID         int64
	CitaID     int64
	MedicoID   int64
	Motivo     string
	Evaluacion string
	Plan       string
	Version    int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
