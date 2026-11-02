package entities

import "time"

// HorarioProfesional representa la disponibilidad semanal recurrente de un
// médico. HoraInicio y HoraFin son horas locales de la clínica en formato
// HH:MM; las fechas de vigencia son fechas sin zona horaria.
type HorarioProfesional struct {
	ID              int64
	MedicoID        int64
	DiaSemana       int
	HoraInicio      string
	HoraFin         string
	DuracionSlotMin int
	VigenteDesde    time.Time
	VigenteHasta    *time.Time
	Activo          bool
	Version         int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// BloqueoAgenda representa una ausencia puntual de un médico. Los instantes se
// guardan en UTC.
type BloqueoAgenda struct {
	ID        int64
	MedicoID  int64
	Inicio    time.Time
	Fin       time.Time
	Motivo    string
	CreadoPor *int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// IntervaloOcupado es un rango ya comprometido de la agenda (cita activa o
// bloqueo). Se usa para restar disponibilidad sin cargar datos clínicos.
type IntervaloOcupado struct {
	Inicio time.Time
	Fin    time.Time
}

// SlotDisponible es un bloque reservable calculado por el servicio. Los
// instantes se exponen en UTC y la duración corresponde al slot.
type SlotDisponible struct {
	Inicio      time.Time
	Fin         time.Time
	DuracionMin int
}

// Disponibilidad es la respuesta del portal para un médico y un rango local.
type Disponibilidad struct {
	MedicoID       int64
	EspecialidadID int64
	ZonaHoraria    string
	Desde          string
	Hasta          string
	Slots          []SlotDisponible
}
