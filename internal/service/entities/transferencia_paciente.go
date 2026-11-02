package entities

import "time"

// TransferenciaPaciente registra un cambio interno de médico tratante. Es un
// historial inmutable: no se actualiza ni se elimina desde la API.
type TransferenciaPaciente struct {
	ID                   int64
	PacienteID           int64
	MedicoAnteriorID     *int64
	MedicoAnteriorNombre string
	MedicoNuevoID        int64
	MedicoNuevoNombre    string
	Motivo               string
	CreadaPorUsuarioID   *int64
	CreadaPorUsername    string
	CreatedAt            time.Time
}
