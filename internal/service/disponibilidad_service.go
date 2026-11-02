package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"medilist/internal/service/entities"
)

// DisponibilidadConfig define la zona horaria, la antelación mínima y el
// horizonte máximo de reserva del portal.
type DisponibilidadConfig struct {
	Location       *time.Location
	MinNotice      time.Duration
	MaxAdvanceDays int
	Now            func() time.Time
}

// DisponibilidadService calcula slots reservables a partir de los horarios
// recurrentes, los bloqueos y las citas activas.
type DisponibilidadService struct {
	medicos        MedicoStore
	store          DisponibilidadStore
	location       *time.Location
	minNotice      time.Duration
	maxAdvanceDays int
	now            func() time.Time
}

// NewDisponibilidadService construye el servicio de disponibilidad.
func NewDisponibilidadService(
	medicos MedicoStore,
	store DisponibilidadStore,
	cfg DisponibilidadConfig,
) *DisponibilidadService {
	loc := cfg.Location
	if loc == nil {
		loc = time.Local
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	minNotice := cfg.MinNotice
	if minNotice < 0 {
		minNotice = 0
	}
	maxDias := cfg.MaxAdvanceDays
	if maxDias <= 0 {
		maxDias = 90
	}
	return &DisponibilidadService{
		medicos:        medicos,
		store:          store,
		location:       loc,
		minNotice:      minNotice,
		maxAdvanceDays: maxDias,
		now:            now,
	}
}

// Consultar devuelve los slots de un médico para un rango de fechas local.
// Si no se envían fechas, usa desde hoy hasta dentro de 6 días.
func (s *DisponibilidadService) Consultar(
	ctx context.Context,
	medicoID int64,
	desdeTexto, hastaTexto string,
) (*entities.Disponibilidad, error) {
	if medicoID <= 0 {
		return nil, entities.NewValidationError("medico_id es obligatorio")
	}
	medico, err := s.medicos.ObtenerPorID(ctx, medicoID)
	if err != nil {
		return nil, err
	}
	if !medico.Activo || !medico.AceptaPortal || !medico.Especialidad.Activa {
		return nil, fmt.Errorf("%w: el médico no está disponible para reservas del portal", entities.ErrNotFound)
	}

	ahora := s.now().In(s.location)
	hoy := time.Date(ahora.Year(), ahora.Month(), ahora.Day(), 0, 0, 0, 0, s.location)
	desde := hoy
	if desdeTexto != "" {
		fecha, err := fechaISO(desdeTexto)
		if err != nil {
			return nil, err
		}
		desde = time.Date(fecha.Year(), fecha.Month(), fecha.Day(), 0, 0, 0, 0, s.location)
	}
	hasta := desde.AddDate(0, 0, 6)
	if hastaTexto != "" {
		fecha, err := fechaISO(hastaTexto)
		if err != nil {
			return nil, err
		}
		hasta = time.Date(fecha.Year(), fecha.Month(), fecha.Day(), 0, 0, 0, 0, s.location)
	}
	if hasta.Before(desde) {
		return nil, entities.NewValidationError("el rango es inválido: hasta es anterior a desde")
	}
	if hasta.Before(hoy) {
		return nil, entities.NewValidationError("el rango no puede estar completamente en el pasado")
	}
	if desde.Before(hoy) {
		desde = hoy
	}
	limite := hoy.AddDate(0, 0, s.maxAdvanceDays-1)
	if hasta.After(limite) {
		return nil, entities.NewValidationError(
			"el rango supera el máximo de %d días de anticipación", s.maxAdvanceDays)
	}
	limiteSlots := limite.AddDate(0, 0, 1)

	desdeDB := time.Date(desde.Year(), desde.Month(), desde.Day(), 0, 0, 0, 0, time.UTC)
	hastaDB := time.Date(hasta.Year(), hasta.Month(), hasta.Day(), 0, 0, 0, 0, time.UTC)
	inicioRango := time.Date(desde.Year(), desde.Month(), desde.Day(), 0, 0, 0, 0, s.location)
	finRango := time.Date(hasta.Year(), hasta.Month(), hasta.Day(), 0, 0, 0, 0, s.location).AddDate(0, 0, 1)

	horarios, err := s.store.ListarHorariosVigentes(ctx, medicoID, desdeDB, hastaDB)
	if err != nil {
		return nil, err
	}
	bloqueos, err := s.store.ListarBloqueosRango(ctx, medicoID, inicioRango.UTC(), finRango.UTC())
	if err != nil {
		return nil, err
	}
	citas, err := s.store.ListarCitasActivasRango(ctx, medicoID, inicioRango.UTC(), finRango.UTC())
	if err != nil {
		return nil, err
	}

	earliest := ahora.Add(s.minNotice)
	slots := make([]entities.SlotDisponible, 0)
	vistos := make(map[int64]struct{})
	for fecha := desde; !fecha.After(hasta); fecha = fecha.AddDate(0, 0, 1) {
		dia := diaSemanaISO(fecha)
		for _, h := range horarios {
			if h.DiaSemana != dia || !horarioVigenteEnFecha(h, fecha) {
				continue
			}
			horaInicio, err := time.Parse("15:04", h.HoraInicio)
			if err != nil {
				return nil, entities.NewValidationError(
					"el horario %d tiene una hora de inicio inválida", h.ID)
			}
			horaFin, err := time.Parse("15:04", h.HoraFin)
			if err != nil {
				return nil, entities.NewValidationError(
					"el horario %d tiene una hora de fin inválida", h.ID)
			}
			inicioHorario := time.Date(
				fecha.Year(), fecha.Month(), fecha.Day(),
				horaInicio.Hour(), horaInicio.Minute(), 0, 0, s.location,
			)
			finHorario := time.Date(
				fecha.Year(), fecha.Month(), fecha.Day(),
				horaFin.Hour(), horaFin.Minute(), 0, 0, s.location,
			)
			duracion := h.DuracionSlotMin
			if duracion <= 0 {
				duracion = medico.DuracionCitaMin
			}
			if duracion <= 0 {
				duracion = 30
			}
			for slot := inicioHorario; !slot.Add(time.Duration(duracion) * time.Minute).After(finHorario); slot = slot.Add(time.Duration(duracion) * time.Minute) {
				finSlot := slot.Add(time.Duration(duracion) * time.Minute)
				if slot.Before(earliest) || !slot.Before(limiteSlots) {
					continue
				}
				if intervaloOcupado(slot, finSlot, bloqueos, citas) {
					continue
				}
				clave := slot.UTC().Unix()
				if _, existe := vistos[clave]; existe {
					continue
				}
				vistos[clave] = struct{}{}
				slots = append(slots, entities.SlotDisponible{
					Inicio:      slot.UTC(),
					Fin:         finSlot.UTC(),
					DuracionMin: duracion,
				})
			}
		}
	}
	sort.Slice(slots, func(i, j int) bool {
		return slots[i].Inicio.Before(slots[j].Inicio)
	})

	return &entities.Disponibilidad{
		MedicoID:       medicoID,
		EspecialidadID: medico.EspecialidadID,
		ZonaHoraria:    s.location.String(),
		Desde:          desde.Format("2006-01-02"),
		Hasta:          hasta.Format("2006-01-02"),
		Slots:          slots,
	}, nil
}

// ValidarSlot comprueba que un instante concreto corresponda a un slot que la
// API mostraría al paciente. Es la validación que usará la reserva de la Fase
// 5 antes de insertar la cita; no acepta inicios arbitrarios ni duraciones
// distintas a las del horario.
func (s *DisponibilidadService) ValidarSlot(
	ctx context.Context,
	medicoID int64,
	inicio time.Time,
	duracionMin int,
) (*entities.SlotDisponible, error) {
	if inicio.IsZero() {
		return nil, entities.NewValidationError("el inicio del slot es obligatorio")
	}
	local := inicio.In(s.location)
	fecha := local.Format("2006-01-02")
	disponibilidad, err := s.Consultar(ctx, medicoID, fecha, fecha)
	if err != nil {
		return nil, err
	}
	for _, slot := range disponibilidad.Slots {
		if !slot.Inicio.Equal(inicio.UTC()) {
			continue
		}
		if duracionMin > 0 && slot.DuracionMin != duracionMin {
			return nil, fmt.Errorf(
				"%w: la duración no coincide con el slot disponible", entities.ErrConflict)
		}
		copia := slot
		return &copia, nil
	}
	return nil, fmt.Errorf(
		"%w: el slot no está disponible, está bloqueado o ya fue ocupado", entities.ErrConflict)
}

func diaSemanaISO(fecha time.Time) int {
	dia := int(fecha.Weekday())
	if dia == 0 {
		return 7
	}
	return dia
}

func horarioVigenteEnFecha(h entities.HorarioProfesional, fecha time.Time) bool {
	clave := fecha.Format("2006-01-02")
	if clave < h.VigenteDesde.Format("2006-01-02") {
		return false
	}
	if h.VigenteHasta != nil && clave > h.VigenteHasta.Format("2006-01-02") {
		return false
	}
	return true
}

func intervaloOcupado(
	inicio, fin time.Time,
	bloqueos []entities.BloqueoAgenda,
	citas []entities.IntervaloOcupado,
) bool {
	for _, b := range bloqueos {
		if inicio.Before(b.Fin) && fin.After(b.Inicio) {
			return true
		}
	}
	for _, c := range citas {
		if inicio.Before(c.Fin) && fin.After(c.Inicio) {
			return true
		}
	}
	return false
}
