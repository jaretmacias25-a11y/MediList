package httpapi

import (
	"strings"
	"time"

	"medilist/internal/service"
	"medilist/internal/service/entities"
)

type notaCitaRequest struct {
	Motivo     string `json:"motivo"`
	Evaluacion string `json:"evaluacion"`
	Plan       string `json:"plan"`
	Version    int64  `json:"version"`
}

func (r notaCitaRequest) toService() service.NotaCitaInput {
	return service.NotaCitaInput{
		Motivo:     r.Motivo,
		Evaluacion: r.Evaluacion,
		Plan:       r.Plan,
		Version:    r.Version,
	}
}

type notaCitaResponse struct {
	ID         int64     `json:"id"`
	CitaID     int64     `json:"cita_id"`
	MedicoID   int64     `json:"medico_id"`
	Motivo     string    `json:"motivo"`
	Evaluacion string    `json:"evaluacion"`
	Plan       string    `json:"plan"`
	Version    int64     `json:"version"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func nuevaNotaCitaResponse(n entities.NotaCita) notaCitaResponse {
	return notaCitaResponse{
		ID:         n.ID,
		CitaID:     n.CitaID,
		MedicoID:   n.MedicoID,
		Motivo:     n.Motivo,
		Evaluacion: n.Evaluacion,
		Plan:       n.Plan,
		Version:    n.Version,
		CreatedAt:  n.CreatedAt,
		UpdatedAt:  n.UpdatedAt,
	}
}

type derivacionRequest struct {
	EspecialidadDestinoID int64  `json:"especialidad_destino_id"`
	MedicoDestinoID       *int64 `json:"medico_destino_id"`
	Motivo                string `json:"motivo"`
	Prioridad             string `json:"prioridad"`
}

func (r derivacionRequest) toService() service.DerivacionInput {
	return service.DerivacionInput{
		EspecialidadDestinoID: r.EspecialidadDestinoID,
		MedicoDestinoID:       r.MedicoDestinoID,
		Motivo:                r.Motivo,
		Prioridad:             entities.PrioridadDerivacion(strings.TrimSpace(r.Prioridad)),
	}
}

type derivacionResponse struct {
	ID                    int64     `json:"id"`
	PacienteID            int64     `json:"paciente_id"`
	PacienteNombre        string    `json:"paciente_nombre"`
	MedicoOrigenID        int64     `json:"medico_origen_id"`
	MedicoOrigenNombre    string    `json:"medico_origen_nombre"`
	CitaOrigenID          int64     `json:"cita_origen_id"`
	EspecialidadDestinoID int64     `json:"especialidad_destino_id"`
	EspecialidadDestino   string    `json:"especialidad_destino"`
	MedicoDestinoID       *int64    `json:"medico_destino_id,omitempty"`
	MedicoDestinoNombre   string    `json:"medico_destino_nombre,omitempty"`
	Motivo                string    `json:"motivo"`
	Prioridad             string    `json:"prioridad"`
	Estado                string    `json:"estado"`
	VigenteDesde          time.Time `json:"vigente_desde"`
	VigenteHasta          time.Time `json:"vigente_hasta"`
	MotivoCancelacion     string    `json:"motivo_cancelacion,omitempty"`
	Version               int64     `json:"version"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

func nuevaDerivacionResponse(d entities.Derivacion) derivacionResponse {
	return derivacionResponse{
		ID:                    d.ID,
		PacienteID:            d.PacienteID,
		PacienteNombre:        d.PacienteNombre,
		MedicoOrigenID:        d.MedicoOrigenID,
		MedicoOrigenNombre:    d.MedicoOrigenNombre,
		CitaOrigenID:          d.CitaOrigenID,
		EspecialidadDestinoID: d.EspecialidadDestinoID,
		EspecialidadDestino:   d.EspecialidadDestino,
		MedicoDestinoID:       d.MedicoDestinoID,
		MedicoDestinoNombre:   d.MedicoDestinoNombre,
		Motivo:                d.Motivo,
		Prioridad:             string(d.Prioridad),
		Estado:                string(d.Estado),
		VigenteDesde:          d.VigenteDesde,
		VigenteHasta:          d.VigenteHasta,
		MotivoCancelacion:     d.MotivoCancelacion,
		Version:               d.Version,
		CreatedAt:             d.CreatedAt,
		UpdatedAt:             d.UpdatedAt,
	}
}

type sobrescribirDerivacionRequest struct {
	Motivo string `json:"motivo"`
}

type excepcionDerivacionResponse struct {
	ID                     int64     `json:"id"`
	CitaID                 int64     `json:"cita_id"`
	PacienteID             int64     `json:"paciente_id"`
	EspecialidadID         int64     `json:"especialidad_id"`
	MedicoID               int64     `json:"medico_id"`
	Motivo                 string    `json:"motivo"`
	AutorizadoPorUsuarioID *int64    `json:"autorizado_por_usuario_id,omitempty"`
	CreatedAt              time.Time `json:"created_at"`
}

func nuevaExcepcionDerivacionResponse(
	e entities.ExcepcionDerivacion,
) excepcionDerivacionResponse {
	return excepcionDerivacionResponse{
		ID:                     e.ID,
		CitaID:                 e.CitaID,
		PacienteID:             e.PacienteID,
		EspecialidadID:         e.EspecialidadID,
		MedicoID:               e.MedicoID,
		Motivo:                 e.Motivo,
		AutorizadoPorUsuarioID: e.AutorizadoPorUsuarioID,
		CreatedAt:              e.CreatedAt,
	}
}

type transferirPacienteRequest struct {
	MedicoDestinoID int64  `json:"medico_destino_id"`
	Motivo          string `json:"motivo"`
	Version         int64  `json:"version"`
}

func (r transferirPacienteRequest) toService() service.TransferirPacienteInput {
	return service.TransferirPacienteInput{
		MedicoDestinoID: r.MedicoDestinoID,
		Motivo:          r.Motivo,
		Version:         r.Version,
	}
}

type transferenciaPacienteResponse struct {
	ID                   int64     `json:"id"`
	PacienteID           int64     `json:"paciente_id"`
	MedicoAnteriorID     *int64    `json:"medico_anterior_id,omitempty"`
	MedicoAnteriorNombre string    `json:"medico_anterior_nombre,omitempty"`
	MedicoNuevoID        int64     `json:"medico_nuevo_id"`
	MedicoNuevoNombre    string    `json:"medico_nuevo_nombre"`
	Motivo               string    `json:"motivo"`
	CreadaPorUsuarioID   *int64    `json:"creada_por_usuario_id,omitempty"`
	CreadaPorUsername    string    `json:"creada_por_username,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
}

func nuevaTransferenciaPacienteResponse(
	t entities.TransferenciaPaciente,
) transferenciaPacienteResponse {
	return transferenciaPacienteResponse{
		ID:                   t.ID,
		PacienteID:           t.PacienteID,
		MedicoAnteriorID:     t.MedicoAnteriorID,
		MedicoAnteriorNombre: t.MedicoAnteriorNombre,
		MedicoNuevoID:        t.MedicoNuevoID,
		MedicoNuevoNombre:    t.MedicoNuevoNombre,
		Motivo:               t.Motivo,
		CreadaPorUsuarioID:   t.CreadaPorUsuarioID,
		CreadaPorUsername:    t.CreadaPorUsername,
		CreatedAt:            t.CreatedAt,
	}
}
