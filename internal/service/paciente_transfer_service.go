package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"medilist/internal/service/entities"
)

// TransferirPacienteInput contiene el nuevo médico tratante y el motivo.
type TransferirPacienteInput struct {
	MedicoDestinoID int64
	Motivo          string
	Version         int64
}

// PacienteTransferService gestiona la asignación del médico responsable. Es
// una operación administrativa y no modifica las citas existentes.
type PacienteTransferService struct {
	pacientes      PacienteStore
	medicos        MedicoStore
	transferencias TransferenciaStore
}

// NewPacienteTransferService construye el servicio de transferencia interna.
func NewPacienteTransferService(
	pacientes PacienteStore,
	medicos MedicoStore,
	transferencias TransferenciaStore,
) *PacienteTransferService {
	return &PacienteTransferService{
		pacientes:      pacientes,
		medicos:        medicos,
		transferencias: transferencias,
	}
}

// Transferir reasigna el médico responsable con control de versión y auditoría
// en el historial. Solo admin/recepción tienen el permiso.
func (s *PacienteTransferService) Transferir(
	ctx context.Context,
	actor *Actor,
	pacienteID int64,
	in TransferirPacienteInput,
) (*entities.TransferenciaPaciente, error) {
	if err := validarActorTransferencia(actor); err != nil {
		return nil, err
	}
	if pacienteID <= 0 {
		return nil, entities.NewValidationError("id de paciente inválido")
	}
	if in.MedicoDestinoID <= 0 {
		return nil, entities.NewValidationError("medico_destino_id es obligatorio")
	}
	motivo := strings.TrimSpace(in.Motivo)
	if motivo == "" {
		return nil, entities.NewValidationError("motivo de la transferencia es obligatorio")
	}
	if utf8.RuneCountInString(motivo) > 1000 {
		return nil, entities.NewValidationError("motivo no puede superar los 1000 caracteres")
	}
	paciente, err := s.pacientes.ObtenerPorID(ctx, pacienteID)
	if err != nil {
		return nil, err
	}
	if !paciente.Activo {
		return nil, entities.NewValidationError("el paciente %d está inactivo", pacienteID)
	}
	if paciente.MedicoResponsableID != nil && *paciente.MedicoResponsableID == in.MedicoDestinoID {
		return nil, fmt.Errorf("%w: el paciente ya tiene asignado ese médico", entities.ErrConflict)
	}
	if actor.MedicoID != nil && *actor.MedicoID == in.MedicoDestinoID {
		return nil, fmt.Errorf("%w: un médico no puede transferirse un paciente a sí mismo", entities.ErrForbidden)
	}
	medico, err := s.medicos.ObtenerPorID(ctx, in.MedicoDestinoID)
	if err != nil {
		return nil, err
	}
	if !medico.Activo {
		return nil, entities.NewValidationError("el médico destino %d está inactivo", in.MedicoDestinoID)
	}
	version := in.Version
	if version == 0 {
		version = paciente.Version
	}
	if version != paciente.Version {
		return nil, fmt.Errorf(
			"%w: el paciente fue modificado por otra operación; vuelve a consultarlo e inténtalo de nuevo",
			entities.ErrConflict)
	}
	return s.transferencias.Transferir(
		ctx, pacienteID, in.MedicoDestinoID, motivo, actor.UsuarioID, version)
}

// ListarTransferencias devuelve el historial de una historia clínica.
func (s *PacienteTransferService) ListarTransferencias(
	ctx context.Context,
	actor *Actor,
	pacienteID int64,
) ([]entities.TransferenciaPaciente, error) {
	if err := validarActorTransferencia(actor); err != nil {
		return nil, err
	}
	if pacienteID <= 0 {
		return nil, entities.NewValidationError("id de paciente inválido")
	}
	if _, err := s.pacientes.ObtenerPorID(ctx, pacienteID); err != nil {
		return nil, err
	}
	return s.transferencias.ListarTransferencias(ctx, pacienteID)
}

func validarActorTransferencia(actor *Actor) error {
	if actor == nil {
		return fmt.Errorf("%w: actor no válido", entities.ErrForbidden)
	}
	if !actor.TienePermiso("pacientes:transferir") {
		return fmt.Errorf("%w: se requiere pacientes:transferir", entities.ErrForbidden)
	}
	return nil
}
