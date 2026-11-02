package service

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"medilist/internal/service/entities"
)

// AgendaConfig agrupa la zona horaria de la clínica y el reloj inyectable.
type AgendaConfig struct {
	Location *time.Location
	Now      func() time.Time
}

// HorarioInput es el contrato de creación/actualización de un horario. Los
// punteros permiten distinguir "no enviado" de "enviado con valor vacío" en
// las actualizaciones.
type HorarioInput struct {
	DiaSemana       int
	HoraInicio      string
	HoraFin         string
	DuracionSlotMin int
	VigenteDesde    *string
	VigenteHasta    *string
	Activo          *bool
	Version         int64
}

// BloqueoInput es el contrato de creación de un bloqueo puntual.
type BloqueoInput struct {
	Inicio time.Time
	Fin    time.Time
	Motivo string
}

// AgendaService gestiona horarios recurrentes y bloqueos puntuales. El
// catálogo de médicos es la fuente de verdad para validar el profesional.
type AgendaService struct {
	medicos  MedicoStore
	horarios HorarioStore
	bloqueos BloqueoStore
	location *time.Location
	now      func() time.Time
}

// NewAgendaService construye el servicio administrativo de agenda.
func NewAgendaService(
	medicos MedicoStore,
	horarios HorarioStore,
	bloqueos BloqueoStore,
	cfg AgendaConfig,
) *AgendaService {
	loc := cfg.Location
	if loc == nil {
		loc = time.Local
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &AgendaService{
		medicos:  medicos,
		horarios: horarios,
		bloqueos: bloqueos,
		location: loc,
		now:      now,
	}
}

// ListarHorarios devuelve todos los horarios del médico, incluidos los
// inactivos, para que administración pueda revisar el historial operativo.
func (s *AgendaService) ListarHorarios(
	ctx context.Context,
	medicoID int64,
) ([]entities.HorarioProfesional, error) {
	if err := s.validarMedico(ctx, medicoID, false); err != nil {
		return nil, err
	}
	return s.horarios.ListarHorariosPorMedico(ctx, medicoID)
}

// CrearHorario valida el rango, la vigencia y el solape antes de persistir.
func (s *AgendaService) CrearHorario(
	ctx context.Context,
	medicoID int64,
	in HorarioInput,
) (*entities.HorarioProfesional, error) {
	if err := s.validarMedico(ctx, medicoID, true); err != nil {
		return nil, err
	}
	h, err := s.prepararHorario(ctx, medicoID, nil, in)
	if err != nil {
		return nil, err
	}
	if err := s.horarios.CrearHorario(ctx, h); err != nil {
		return nil, err
	}
	return h, nil
}

// ActualizarHorario aplica un PUT con control de versión y valida de nuevo el
// solape con el resto de la agenda.
func (s *AgendaService) ActualizarHorario(
	ctx context.Context,
	medicoID, horarioID int64,
	in HorarioInput,
) (*entities.HorarioProfesional, error) {
	if horarioID <= 0 {
		return nil, entities.NewValidationError("id de horario inválido")
	}
	if err := s.validarMedico(ctx, medicoID, true); err != nil {
		return nil, err
	}
	actual, err := s.horarios.ObtenerHorarioPorID(ctx, medicoID, horarioID)
	if err != nil {
		return nil, err
	}
	if in.Version > 0 && in.Version != actual.Version {
		return nil, fmt.Errorf(
			"%w: el horario fue modificado por otra operación; vuelve a consultarlo e inténtalo de nuevo",
			entities.ErrConflict)
	}
	h, err := s.prepararHorario(ctx, medicoID, actual, in)
	if err != nil {
		return nil, err
	}
	if err := s.horarios.ActualizarHorario(ctx, h); err != nil {
		return nil, err
	}
	return s.horarios.ObtenerHorarioPorID(ctx, medicoID, horarioID)
}

// DesactivarHorario realiza una baja lógica para no perder el historial ni las
// citas que ya fueron reservadas con ese horario.
func (s *AgendaService) DesactivarHorario(
	ctx context.Context,
	medicoID, horarioID int64,
) error {
	if horarioID <= 0 {
		return entities.NewValidationError("id de horario inválido")
	}
	if err := s.validarMedico(ctx, medicoID, false); err != nil {
		return err
	}
	return s.horarios.DesactivarHorario(ctx, medicoID, horarioID)
}

// ListarBloqueos devuelve los bloqueos que se cruzan con el rango local
// solicitado. Sin parámetros usa los próximos 30 días.
func (s *AgendaService) ListarBloqueos(
	ctx context.Context,
	medicoID int64,
	desdeTexto, hastaTexto string,
) ([]entities.BloqueoAgenda, error) {
	if err := s.validarMedico(ctx, medicoID, false); err != nil {
		return nil, err
	}
	desde, hasta, err := s.rangoFechas(desdeTexto, hastaTexto)
	if err != nil {
		return nil, err
	}
	inicio := time.Date(desde.Year(), desde.Month(), desde.Day(), 0, 0, 0, 0, s.location)
	fin := time.Date(hasta.Year(), hasta.Month(), hasta.Day(), 0, 0, 0, 0, s.location).AddDate(0, 0, 1)
	return s.bloqueos.ListarBloqueosRango(ctx, medicoID, inicio.UTC(), fin.UTC())
}

// CrearBloqueo valida el motivo y el rango, y guarda el bloqueo en UTC.
func (s *AgendaService) CrearBloqueo(
	ctx context.Context,
	medicoID int64,
	actor *Actor,
	in BloqueoInput,
) (*entities.BloqueoAgenda, error) {
	if err := s.validarMedico(ctx, medicoID, true); err != nil {
		return nil, err
	}
	inicio, fin, motivo, err := validarDatosBloqueo(in)
	if err != nil {
		return nil, err
	}
	solapa, err := s.bloqueos.ExisteSolapamientoBloqueo(ctx, medicoID, inicio, fin, 0)
	if err != nil {
		return nil, err
	}
	if solapa {
		return nil, fmt.Errorf("%w: el médico ya tiene un bloqueo que se cruza con ese rango", entities.ErrConflict)
	}

	b := &entities.BloqueoAgenda{
		MedicoID: medicoID,
		Inicio:   inicio,
		Fin:      fin,
		Motivo:   motivo,
	}
	if actor != nil && actor.UsuarioID > 0 {
		creadoPor := actor.UsuarioID
		b.CreadoPor = &creadoPor
	}
	if err := s.bloqueos.CrearBloqueo(ctx, b); err != nil {
		return nil, err
	}
	return b, nil
}

// ActualizarBloqueo reemplaza el rango y el motivo, excluyendo el propio
// bloqueo de la comprobación de solape.
func (s *AgendaService) ActualizarBloqueo(
	ctx context.Context,
	medicoID, bloqueoID int64,
	in BloqueoInput,
) (*entities.BloqueoAgenda, error) {
	if bloqueoID <= 0 {
		return nil, entities.NewValidationError("id de bloqueo inválido")
	}
	if err := s.validarMedico(ctx, medicoID, true); err != nil {
		return nil, err
	}
	inicio, fin, motivo, err := validarDatosBloqueo(in)
	if err != nil {
		return nil, err
	}
	solapa, err := s.bloqueos.ExisteSolapamientoBloqueo(ctx, medicoID, inicio, fin, bloqueoID)
	if err != nil {
		return nil, err
	}
	if solapa {
		return nil, fmt.Errorf("%w: el médico ya tiene otro bloqueo que se cruza con ese rango", entities.ErrConflict)
	}

	b := &entities.BloqueoAgenda{
		ID:       bloqueoID,
		MedicoID: medicoID,
		Inicio:   inicio,
		Fin:      fin,
		Motivo:   motivo,
	}
	if err := s.bloqueos.ActualizarBloqueo(ctx, b); err != nil {
		return nil, err
	}
	return b, nil
}

// EliminarBloqueo elimina el bloqueo solo si pertenece al médico indicado.
func (s *AgendaService) EliminarBloqueo(
	ctx context.Context,
	medicoID, bloqueoID int64,
) error {
	if bloqueoID <= 0 {
		return entities.NewValidationError("id de bloqueo inválido")
	}
	if err := s.validarMedico(ctx, medicoID, false); err != nil {
		return err
	}
	return s.bloqueos.EliminarBloqueo(ctx, medicoID, bloqueoID)
}

func validarDatosBloqueo(in BloqueoInput) (time.Time, time.Time, string, error) {
	if in.Inicio.IsZero() || in.Fin.IsZero() {
		return time.Time{}, time.Time{}, "", entities.NewValidationError(
			"inicio y fin del bloqueo son obligatorios")
	}
	inicio := in.Inicio.UTC()
	fin := in.Fin.UTC()
	if !fin.After(inicio) {
		return time.Time{}, time.Time{}, "", entities.NewValidationError(
			"el fin del bloqueo debe ser posterior al inicio")
	}
	if fin.Sub(inicio) > 365*24*time.Hour {
		return time.Time{}, time.Time{}, "", entities.NewValidationError(
			"el bloqueo no puede superar 365 días")
	}
	motivo := strings.TrimSpace(in.Motivo)
	if motivo == "" {
		return time.Time{}, time.Time{}, "", entities.NewValidationError(
			"motivo del bloqueo es obligatorio")
	}
	if utf8.RuneCountInString(motivo) > 500 {
		return time.Time{}, time.Time{}, "", entities.NewValidationError(
			"motivo no puede superar los 500 caracteres")
	}
	return inicio, fin, motivo, nil
}

func (s *AgendaService) prepararHorario(
	ctx context.Context,
	medicoID int64,
	base *entities.HorarioProfesional,
	in HorarioInput,
) (*entities.HorarioProfesional, error) {
	h := &entities.HorarioProfesional{MedicoID: medicoID}
	if base != nil {
		*h = *base
	}

	if in.DiaSemana != 0 || base == nil {
		h.DiaSemana = in.DiaSemana
	}
	if strings.TrimSpace(in.HoraInicio) != "" || base == nil {
		h.HoraInicio = strings.TrimSpace(in.HoraInicio)
	}
	if strings.TrimSpace(in.HoraFin) != "" || base == nil {
		h.HoraFin = strings.TrimSpace(in.HoraFin)
	}
	if in.DuracionSlotMin != 0 || base == nil {
		h.DuracionSlotMin = in.DuracionSlotMin
	}
	if h.DuracionSlotMin == 0 {
		h.DuracionSlotMin = 30
	}

	if in.VigenteDesde != nil {
		if strings.TrimSpace(*in.VigenteDesde) == "" {
			return nil, entities.NewValidationError("vigente_desde no puede estar vacío")
		}
		fecha, err := fechaISO(*in.VigenteDesde)
		if err != nil {
			return nil, err
		}
		h.VigenteDesde = fecha
	} else if base == nil {
		hoy := s.now().In(s.location)
		h.VigenteDesde = time.Date(hoy.Year(), hoy.Month(), hoy.Day(), 0, 0, 0, 0, time.UTC)
	}

	if in.VigenteHasta != nil {
		if strings.TrimSpace(*in.VigenteHasta) == "" {
			h.VigenteHasta = nil
		} else {
			fecha, err := fechaISO(*in.VigenteHasta)
			if err != nil {
				return nil, err
			}
			h.VigenteHasta = &fecha
		}
	}

	if in.Activo != nil {
		h.Activo = *in.Activo
	} else if base == nil {
		h.Activo = true
	}

	if h.DiaSemana < 1 || h.DiaSemana > 7 {
		return nil, entities.NewValidationError("dia_semana debe estar entre 1 (lunes) y 7 (domingo)")
	}
	inicio, err := horaLocal(h.HoraInicio)
	if err != nil {
		return nil, err
	}
	fin, err := horaLocal(h.HoraFin)
	if err != nil {
		return nil, err
	}
	if !fin.After(inicio) {
		return nil, entities.NewValidationError("hora_fin debe ser posterior a hora_inicio")
	}
	if h.DuracionSlotMin < 5 || h.DuracionSlotMin > 240 {
		return nil, entities.NewValidationError("duracion_slot_min debe estar entre 5 y 240 minutos")
	}
	if int(fin.Sub(inicio).Minutes()) < h.DuracionSlotMin {
		return nil, entities.NewValidationError("el rango del horario no alcanza para un slot completo")
	}
	if h.VigenteDesde.IsZero() {
		return nil, entities.NewValidationError("vigente_desde es obligatorio")
	}
	if h.VigenteHasta != nil && h.VigenteHasta.Before(h.VigenteDesde) {
		return nil, entities.NewValidationError("vigente_hasta no puede ser anterior a vigente_desde")
	}
	h.HoraInicio = inicio.Format("15:04")
	h.HoraFin = fin.Format("15:04")

	if h.Activo {
		excluirID := int64(0)
		if base != nil {
			excluirID = base.ID
		}
		solapa, err := s.horarios.ExisteSolapamientoHorario(
			ctx, medicoID, h.DiaSemana, h.HoraInicio, h.HoraFin,
			h.VigenteDesde, h.VigenteHasta, excluirID,
		)
		if err != nil {
			return nil, err
		}
		if solapa {
			return nil, fmt.Errorf(
				"%w: el horario se solapa con otro horario activo del médico",
				entities.ErrConflict)
		}
	}
	return h, nil
}

func (s *AgendaService) validarMedico(
	ctx context.Context,
	medicoID int64,
	requiereActivo bool,
) error {
	if medicoID <= 0 {
		return entities.NewValidationError("id de médico inválido")
	}
	medico, err := s.medicos.ObtenerPorID(ctx, medicoID)
	if err != nil {
		return err
	}
	if requiereActivo && !medico.Activo {
		return entities.NewValidationError("el médico %d está inactivo", medicoID)
	}
	return nil
}

func (s *AgendaService) rangoFechas(desdeTexto, hastaTexto string) (time.Time, time.Time, error) {
	hoy := s.now().In(s.location)
	hoyUTC := time.Date(hoy.Year(), hoy.Month(), hoy.Day(), 0, 0, 0, 0, time.UTC)

	desde := hoyUTC
	if strings.TrimSpace(desdeTexto) != "" {
		fecha, err := fechaISO(desdeTexto)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		desde = fecha
	}
	hasta := desde.AddDate(0, 0, 29)
	if strings.TrimSpace(hastaTexto) != "" {
		fecha, err := fechaISO(hastaTexto)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		hasta = fecha
	}
	if hasta.Before(desde) {
		return time.Time{}, time.Time{}, entities.NewValidationError("el rango es inválido: hasta es anterior a desde")
	}
	if hasta.Sub(desde) > 90*24*time.Hour {
		return time.Time{}, time.Time{}, entities.NewValidationError("el rango no puede superar 90 días")
	}
	return desde, hasta, nil
}

func fechaISO(texto string) (time.Time, error) {
	fecha, err := time.Parse("2006-01-02", strings.TrimSpace(texto))
	if err != nil {
		return time.Time{}, entities.NewValidationError("la fecha debe tener formato YYYY-MM-DD")
	}
	return fecha, nil
}

func horaLocal(texto string) (time.Time, error) {
	hora, err := time.Parse("15:04", strings.TrimSpace(texto))
	if err != nil {
		return time.Time{}, entities.NewValidationError("la hora debe tener formato HH:MM")
	}
	return hora, nil
}
