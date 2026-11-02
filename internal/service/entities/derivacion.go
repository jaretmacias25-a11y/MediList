package entities

import "time"

// EstadoDerivacion representa el ciclo de vida de una derivación clínica.
type EstadoDerivacion string

const (
	EstadoDerivacionPendiente EstadoDerivacion = "pendiente"
	EstadoDerivacionAgendada  EstadoDerivacion = "agendada"
	EstadoDerivacionAtendida  EstadoDerivacion = "atendida"
	EstadoDerivacionVencida   EstadoDerivacion = "vencida"
	EstadoDerivacionCancelada EstadoDerivacion = "cancelada"
)

// Valido indica si el estado existe.
func (e EstadoDerivacion) Valido() bool {
	switch e {
	case EstadoDerivacionPendiente, EstadoDerivacionAgendada,
		EstadoDerivacionAtendida, EstadoDerivacionVencida, EstadoDerivacionCancelada:
		return true
	default:
		return false
	}
}

// PrioridadDerivacion ordena la atención clínica de una derivación.
type PrioridadDerivacion string

const (
	PrioridadDerivacionNormal     PrioridadDerivacion = "normal"
	PrioridadDerivacionPreferente PrioridadDerivacion = "preferente"
	PrioridadDerivacionUrgente    PrioridadDerivacion = "urgente"
)

// Valida indica si la prioridad existe.
func (p PrioridadDerivacion) Valida() bool {
	switch p {
	case PrioridadDerivacionNormal, PrioridadDerivacionPreferente, PrioridadDerivacionUrgente:
		return true
	default:
		return false
	}
}

// Derivacion es la autorización clínica para que un paciente sea atendido por
// una especialidad que requiere derivación. Los campos de nombre son datos de
// lectura completados por el store.
type Derivacion struct {
	ID                    int64
	PacienteID            int64
	PacienteNombre        string
	MedicoOrigenID        int64
	MedicoOrigenNombre    string
	CitaOrigenID          int64
	EspecialidadDestinoID int64
	EspecialidadDestino   string
	MedicoDestinoID       *int64
	MedicoDestinoNombre   string
	Motivo                string
	Prioridad             PrioridadDerivacion
	Estado                EstadoDerivacion
	VigenteDesde          time.Time
	VigenteHasta          time.Time
	MotivoCancelacion     string
	Version               int64
	CreatedAt             time.Time
	UpdatedAt             time.Time

	// NotificacionPendiente se inserta en el mismo outbox/transacción que la
	// derivación.
	NotificacionPendiente *Notificacion
}

// Vigente indica si la derivación puede usarse en este instante.
func (d Derivacion) Vigente(ahora time.Time) bool {
	return d.Estado == EstadoDerivacionPendiente &&
		!d.VigenteHasta.IsZero() &&
		d.VigenteHasta.After(ahora)
}
