package entities

import "time"

// ExcepcionDerivacion registra una autorización administrativa excepcional
// para una cita de especialista que no tiene una derivación clínica asociada.
// No crea una derivación falsa: deja trazabilidad de la excepción.
type ExcepcionDerivacion struct {
	ID                     int64
	CitaID                 int64
	PacienteID             int64
	EspecialidadID         int64
	MedicoID               int64
	Motivo                 string
	AutorizadoPorUsuarioID *int64
	CreatedAt              time.Time
}
