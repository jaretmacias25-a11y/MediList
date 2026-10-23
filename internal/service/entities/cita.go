package entities

import "time"

// TipoCita identifica la modalidad de la atención.
type TipoCita string

const (
	TipoCitaPrimeraVez   TipoCita = "primera_vez"
	TipoCitaControl      TipoCita = "control"
	TipoCitaUrgencia     TipoCita = "urgencia"
	TipoCitaTelemedicina TipoCita = "telemedicina"
)

// Valido indica si el tipo está permitido.
func (t TipoCita) Valido() bool {
	switch t {
	case TipoCitaPrimeraVez, TipoCitaControl, TipoCitaUrgencia, TipoCitaTelemedicina:
		return true
	default:
		return false
	}
}

// EstadoCita representa el ciclo de vida de una cita.
type EstadoCita string

const (
	EstadoCitaProgramada   EstadoCita = "programada"
	EstadoCitaConfirmada   EstadoCita = "confirmada"
	EstadoCitaAtendida     EstadoCita = "atendida"
	EstadoCitaNoAsistio    EstadoCita = "no_asistio"
	EstadoCitaCancelada    EstadoCita = "cancelada"
	EstadoCitaReprogramada EstadoCita = "reprogramada"
)

// Valido indica si el estado existe.
func (e EstadoCita) Valido() bool {
	switch e {
	case EstadoCitaProgramada, EstadoCitaConfirmada, EstadoCitaAtendida,
		EstadoCitaNoAsistio, EstadoCitaCancelada, EstadoCitaReprogramada:
		return true
	default:
		return false
	}
}

// EsTerminal indica si el estado ya no admite modificaciones.
func (e EstadoCita) EsTerminal() bool {
	switch e {
	case EstadoCitaAtendida, EstadoCitaNoAsistio, EstadoCitaCancelada, EstadoCitaReprogramada:
		return true
	default:
		return false
	}
}

// PuedePasarA define las transiciones permitidas del ciclo de vida.
func (e EstadoCita) PuedePasarA(nuevo EstadoCita) bool {
	switch e {
	case EstadoCitaProgramada:
		return nuevo == EstadoCitaConfirmada ||
			nuevo == EstadoCitaCancelada ||
			nuevo == EstadoCitaReprogramada
	case EstadoCitaConfirmada:
		return nuevo == EstadoCitaAtendida ||
			nuevo == EstadoCitaNoAsistio ||
			nuevo == EstadoCitaCancelada ||
			nuevo == EstadoCitaReprogramada
	default:
		return false
	}
}

// Cita es la reserva de atención de un paciente con un médico.
// PacienteNombre, MedicoNombre y EspecialidadNombre son datos de lectura
// que el almacén completa mediante JOIN.
type Cita struct {
	ID                 int64
	Codigo             string
	PacienteID         int64
	PacienteNombre     string
	MedicoID           int64
	MedicoNombre       string
	EspecialidadNombre string
	FechaHoraInicio    time.Time
	DuracionMin        int
	Tipo               TipoCita
	Motivo             string
	Estado             EstadoCita
	Version            int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// CitaFiltro agrupa los filtros de listado de citas.
type CitaFiltro struct {
	ListOptions
	PacienteID int64
	MedicoID   int64
	Estado     EstadoCita
	Desde      *time.Time
	Hasta      *time.Time
}
