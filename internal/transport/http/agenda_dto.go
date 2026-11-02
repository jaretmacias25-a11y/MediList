package httpapi

import (
	"strings"
	"time"

	"medilist/internal/service"
	"medilist/internal/service/entities"
)

// ---------------------------------------------------------------------------
// Horarios y bloqueos administrativos
// ---------------------------------------------------------------------------

type horarioRequest struct {
	DiaSemana       int     `json:"dia_semana"`
	HoraInicio      string  `json:"hora_inicio"`
	HoraFin         string  `json:"hora_fin"`
	DuracionSlotMin int     `json:"duracion_slot_min"`
	VigenteDesde    *string `json:"vigente_desde"`
	VigenteHasta    *string `json:"vigente_hasta"`
	Activo          *bool   `json:"activo"`
	Version         int64   `json:"version"`
}

func (r horarioRequest) toService() service.HorarioInput {
	return service.HorarioInput{
		DiaSemana:       r.DiaSemana,
		HoraInicio:      r.HoraInicio,
		HoraFin:         r.HoraFin,
		DuracionSlotMin: r.DuracionSlotMin,
		VigenteDesde:    r.VigenteDesde,
		VigenteHasta:    r.VigenteHasta,
		Activo:          r.Activo,
		Version:         r.Version,
	}
}

type horarioResponse struct {
	ID              int64     `json:"id"`
	MedicoID        int64     `json:"medico_id"`
	DiaSemana       int       `json:"dia_semana"`
	HoraInicio      string    `json:"hora_inicio"`
	HoraFin         string    `json:"hora_fin"`
	DuracionSlotMin int       `json:"duracion_slot_min"`
	VigenteDesde    string    `json:"vigente_desde"`
	VigenteHasta    *string   `json:"vigente_hasta"`
	Activo          bool      `json:"activo"`
	Version         int64     `json:"version"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func nuevoHorarioResponse(h entities.HorarioProfesional) horarioResponse {
	resp := horarioResponse{
		ID:              h.ID,
		MedicoID:        h.MedicoID,
		DiaSemana:       h.DiaSemana,
		HoraInicio:      h.HoraInicio,
		HoraFin:         h.HoraFin,
		DuracionSlotMin: h.DuracionSlotMin,
		VigenteDesde:    h.VigenteDesde.Format("2006-01-02"),
		Activo:          h.Activo,
		Version:         h.Version,
		CreatedAt:       h.CreatedAt,
		UpdatedAt:       h.UpdatedAt,
	}
	if h.VigenteHasta != nil {
		hasta := h.VigenteHasta.Format("2006-01-02")
		resp.VigenteHasta = &hasta
	}
	return resp
}

type bloqueoRequest struct {
	Inicio string `json:"inicio"`
	Fin    string `json:"fin"`
	Motivo string `json:"motivo"`
}

func (r bloqueoRequest) toService() (service.BloqueoInput, error) {
	inicio, err := time.Parse(time.RFC3339, strings.TrimSpace(r.Inicio))
	if err != nil {
		return service.BloqueoInput{}, entities.NewValidationError(
			"inicio debe tener formato RFC3339 con zona horaria")
	}
	fin, err := time.Parse(time.RFC3339, strings.TrimSpace(r.Fin))
	if err != nil {
		return service.BloqueoInput{}, entities.NewValidationError(
			"fin debe tener formato RFC3339 con zona horaria")
	}
	return service.BloqueoInput{
		Inicio: inicio,
		Fin:    fin,
		Motivo: r.Motivo,
	}, nil
}

type bloqueoResponse struct {
	ID        int64     `json:"id"`
	MedicoID  int64     `json:"medico_id"`
	Inicio    time.Time `json:"inicio"`
	Fin       time.Time `json:"fin"`
	Motivo    string    `json:"motivo"`
	CreadoPor *int64    `json:"creado_por,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func nuevoBloqueoResponse(b entities.BloqueoAgenda) bloqueoResponse {
	return bloqueoResponse{
		ID:        b.ID,
		MedicoID:  b.MedicoID,
		Inicio:    b.Inicio.UTC(),
		Fin:       b.Fin.UTC(),
		Motivo:    b.Motivo,
		CreadoPor: b.CreadoPor,
		CreatedAt: b.CreatedAt,
		UpdatedAt: b.UpdatedAt,
	}
}

// ---------------------------------------------------------------------------
// Catálogo y disponibilidad del portal
// ---------------------------------------------------------------------------

type especialidadPortalResponse struct {
	ID                 int64  `json:"id"`
	Codigo             string `json:"codigo"`
	Nombre             string `json:"nombre"`
	Descripcion        string `json:"descripcion"`
	RequiereDerivacion bool   `json:"requiere_derivacion"`
	DuracionCitaMin    int    `json:"duracion_cita_min"`
}

func nuevaEspecialidadPortalResponse(e entities.Especialidad) especialidadPortalResponse {
	return especialidadPortalResponse{
		ID:                 e.ID,
		Codigo:             e.Codigo,
		Nombre:             e.Nombre,
		Descripcion:        e.Descripcion,
		RequiereDerivacion: e.RequiereDerivacion,
		DuracionCitaMin:    e.DuracionCitaMin,
	}
}

type medicoPortalResponse struct {
	ID              int64                      `json:"id"`
	Nombres         string                     `json:"nombres"`
	Apellidos       string                     `json:"apellidos"`
	Especialidad    especialidadPortalResponse `json:"especialidad"`
	DuracionCitaMin int                        `json:"duracion_cita_min"`
	Bio             string                     `json:"bio"`
	FotoURL         string                     `json:"foto_url"`
}

func nuevoMedicoPortalResponse(m entities.Medico) medicoPortalResponse {
	return medicoPortalResponse{
		ID:              m.ID,
		Nombres:         m.Persona.Nombres,
		Apellidos:       m.Persona.Apellidos,
		Especialidad:    nuevaEspecialidadPortalResponse(m.Especialidad),
		DuracionCitaMin: m.DuracionCitaMin,
		Bio:             m.Bio,
		FotoURL:         m.FotoURL,
	}
}

type slotResponse struct {
	Inicio      string `json:"inicio"`
	Fin         string `json:"fin"`
	DuracionMin int    `json:"duracion_min"`
}

type disponibilidadResponse struct {
	MedicoID       int64          `json:"medico_id"`
	EspecialidadID int64          `json:"especialidad_id"`
	ZonaHoraria    string         `json:"zona_horaria"`
	Desde          string         `json:"desde"`
	Hasta          string         `json:"hasta"`
	Slots          []slotResponse `json:"slots"`
}

func nuevaDisponibilidadResponse(d entities.Disponibilidad) disponibilidadResponse {
	slots := make([]slotResponse, 0, len(d.Slots))
	for _, slot := range d.Slots {
		slots = append(slots, slotResponse{
			Inicio:      slot.Inicio.UTC().Format(time.RFC3339),
			Fin:         slot.Fin.UTC().Format(time.RFC3339),
			DuracionMin: slot.DuracionMin,
		})
	}
	return disponibilidadResponse{
		MedicoID:       d.MedicoID,
		EspecialidadID: d.EspecialidadID,
		ZonaHoraria:    d.ZonaHoraria,
		Desde:          d.Desde,
		Hasta:          d.Hasta,
		Slots:          slots,
	}
}
