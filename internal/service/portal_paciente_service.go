package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"medilist/internal/service/entities"
)

// DatosContactoPaciente contiene los únicos campos que el paciente puede
// modificar desde el portal. Documento, nombres, fecha de nacimiento, sexo,
// email de la cuenta y datos clínicos quedan fuera de este contrato.
type DatosContactoPaciente struct {
	Telefono                   string
	Direccion                  string
	ContactoEmergenciaNombre   string
	ContactoEmergenciaTelefono string
	Version                    int64
}

// PortalPacienteService implementa el perfil propio del paciente.
type PortalPacienteService struct {
	pacientes PacienteStore
}

// NewPortalPacienteService construye el servicio del portal del paciente.
func NewPortalPacienteService(pacientes PacienteStore) *PortalPacienteService {
	return &PortalPacienteService{pacientes: pacientes}
}

// Perfil devuelve únicamente la historia vinculada al actor autenticado.
func (s *PortalPacienteService) Perfil(ctx context.Context, actor *Actor) (*entities.Paciente, error) {
	if actor == nil {
		return nil, fmt.Errorf("%w: actor no válido", entities.ErrForbidden)
	}
	pacienteID, err := actor.PacienteVinculado()
	if err != nil {
		return nil, err
	}
	return s.pacientePropio(ctx, actor, pacienteID)
}

// ActualizarContacto modifica el contacto del paciente propio con control de
// versión. No acepta el ID del paciente desde el cliente.
func (s *PortalPacienteService) ActualizarContacto(
	ctx context.Context,
	actor *Actor,
	datos DatosContactoPaciente,
) (*entities.Paciente, error) {
	if actor == nil {
		return nil, fmt.Errorf("%w: actor no válido", entities.ErrForbidden)
	}
	pacienteID, err := actor.PacienteVinculado()
	if err != nil {
		return nil, err
	}
	actual, err := s.pacientePropio(ctx, actor, pacienteID)
	if err != nil {
		return nil, err
	}
	if datos.Version > 0 && datos.Version != actual.Version {
		return nil, fmt.Errorf(
			"%w: el perfil fue modificado por otra operación; vuelve a consultarlo e inténtalo de nuevo",
			entities.ErrConflict)
	}

	actual.Persona.Telefono = strings.TrimSpace(datos.Telefono)
	actual.Persona.Direccion = strings.TrimSpace(datos.Direccion)
	actual.ContactoEmergenciaNombre = strings.TrimSpace(datos.ContactoEmergenciaNombre)
	actual.ContactoEmergenciaTelefono = strings.TrimSpace(datos.ContactoEmergenciaTelefono)
	if err := validarContactoPaciente(*actual); err != nil {
		return nil, err
	}
	if err := s.pacientes.Actualizar(ctx, actual); err != nil {
		return nil, err
	}
	return s.pacientes.ObtenerPorID(ctx, pacienteID)
}

func (s *PortalPacienteService) pacientePropio(
	ctx context.Context,
	actor *Actor,
	pacienteID int64,
) (*entities.Paciente, error) {
	paciente, err := s.pacientes.ObtenerPorID(ctx, pacienteID)
	if err != nil {
		return nil, err
	}
	if !paciente.Activo ||
		paciente.UsuarioID == nil ||
		*paciente.UsuarioID != actor.UsuarioID {
		return nil, fmt.Errorf("%w: el perfil no pertenece a la cuenta autenticada", entities.ErrForbidden)
	}
	return paciente, nil
}

func validarContactoPaciente(p entities.Paciente) error {
	if utf8.RuneCountInString(p.Persona.Telefono) > 30 {
		return entities.NewValidationError("telefono no puede superar los 30 caracteres")
	}
	if utf8.RuneCountInString(p.Persona.Direccion) > 255 {
		return entities.NewValidationError("direccion no puede superar los 255 caracteres")
	}
	if utf8.RuneCountInString(p.ContactoEmergenciaNombre) > 160 {
		return entities.NewValidationError("contacto_emergencia_nombre no puede superar los 160 caracteres")
	}
	if utf8.RuneCountInString(p.ContactoEmergenciaTelefono) > 30 {
		return entities.NewValidationError("contacto_emergencia_telefono no puede superar los 30 caracteres")
	}
	return nil
}
