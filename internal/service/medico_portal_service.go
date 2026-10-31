package service

import (
	"context"
	"fmt"

	"medilist/internal/service/entities"
)

// MedicoPortalService implementa la agenda y el acceso a pacientes propios del
// médico. Todas las consultas fuerzan el MedicoID del Actor: el cliente nunca
// puede elegir el propietario mediante un parámetro.
type MedicoPortalService struct {
	citas     CitaStore
	pacientes PacienteStore
}

// NewMedicoPortalService construye el servicio del portal médico.
func NewMedicoPortalService(citas CitaStore, pacientes PacienteStore) *MedicoPortalService {
	return &MedicoPortalService{citas: citas, pacientes: pacientes}
}

// Agenda lista únicamente las citas del médico autenticado.
func (s *MedicoPortalService) Agenda(
	ctx context.Context,
	actor *Actor,
	filtro entities.CitaFiltro,
) ([]entities.Cita, error) {
	if actor == nil {
		return nil, fmt.Errorf("%w: actor no válido", entities.ErrForbidden)
	}
	medicoID, err := actor.MedicoVinculado()
	if err != nil {
		return nil, err
	}

	filtro, err = prepararFiltroCitas(filtro)
	if err != nil {
		return nil, err
	}
	// Se sobrescribe cualquier medico_id que llegue en la query. El dueño se
	// decide exclusivamente por el vínculo de la cuenta.
	filtro.MedicoID = medicoID
	return s.citas.Listar(ctx, filtro)
}

// ObtenerCita devuelve una cita solo si pertenece al médico autenticado.
func (s *MedicoPortalService) ObtenerCita(
	ctx context.Context,
	actor *Actor,
	citaID int64,
) (*entities.Cita, error) {
	if actor == nil {
		return nil, fmt.Errorf("%w: actor no válido", entities.ErrForbidden)
	}
	medicoID, err := actor.MedicoVinculado()
	if err != nil {
		return nil, err
	}
	if citaID <= 0 {
		return nil, entities.NewValidationError("id de cita inválido")
	}

	cita, err := s.citas.ObtenerPorID(ctx, citaID)
	if err != nil {
		return nil, err
	}
	if cita.MedicoID != medicoID {
		return nil, fmt.Errorf("%w: la cita no pertenece al médico autenticado", entities.ErrForbidden)
	}
	return cita, nil
}

// PacienteAtendido devuelve un paciente solo cuando tiene una relación de
// atención vigente o atendida con el médico autenticado.
func (s *MedicoPortalService) PacienteAtendido(
	ctx context.Context,
	actor *Actor,
	pacienteID int64,
) (*entities.Paciente, error) {
	if actor == nil {
		return nil, fmt.Errorf("%w: actor no válido", entities.ErrForbidden)
	}
	medicoID, err := actor.MedicoVinculado()
	if err != nil {
		return nil, err
	}
	if pacienteID <= 0 {
		return nil, entities.NewValidationError("id de paciente inválido")
	}

	pertenece, err := s.pacientes.PerteneceAMedico(ctx, pacienteID, medicoID)
	if err != nil {
		return nil, err
	}
	if !pertenece {
		return nil, fmt.Errorf("%w: el paciente no tiene relación de atención con el médico autenticado", entities.ErrForbidden)
	}

	paciente, err := s.pacientes.ObtenerPorID(ctx, pacienteID)
	if err != nil {
		return nil, err
	}
	if !paciente.Activo {
		return nil, fmt.Errorf("%w: el paciente no está activo", entities.ErrForbidden)
	}
	return paciente, nil
}
