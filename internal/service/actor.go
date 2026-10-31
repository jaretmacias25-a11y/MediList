package service

import (
	"fmt"

	"medilist/internal/service/entities"
)

// Actor es el contexto de autorización resuelto para una solicitud autenticada.
// Los IDs de paciente y médico no se aceptan desde el cliente: se obtienen de
// los vínculos de la cuenta en la base de datos.
type Actor struct {
	UsuarioID       int64
	Username        string
	Email           string
	Roles           []string
	Permisos        []string
	PacienteID      *int64
	MedicoID        *int64
	EmailVerificado bool
}

// TieneRol indica si el actor posee un rol determinado.
func (a Actor) TieneRol(codigo string) bool {
	for _, rol := range a.Roles {
		if rol == codigo {
			return true
		}
	}
	return false
}

// TienePermiso replica la regla del middleware: el administrador tiene todos
// los permisos y el resto requiere el permiso explícito.
func (a Actor) TienePermiso(codigo string) bool {
	if a.TieneRol("admin") {
		return true
	}
	for _, permiso := range a.Permisos {
		if permiso == codigo {
			return true
		}
	}
	return false
}

// PuedeGestionarDatosGlobales limita los endpoints administrativos de
// pacientes y citas al personal autorizado. Un médico con un token antiguo que
// todavía contenga permisos globales no puede usarlos para ver datos ajenos.
func (a Actor) PuedeGestionarDatosGlobales() bool {
	return a.TieneRol("admin") || a.TieneRol("recepcion") || a.TieneRol("enfermeria")
}

// PacienteVinculado devuelve el ID de paciente del actor o un error de dominio
// si la cuenta no tiene un perfil de paciente activo vinculado.
func (a Actor) PacienteVinculado() (int64, error) {
	if a.PacienteID == nil || *a.PacienteID <= 0 {
		return 0, fmt.Errorf("%w: la cuenta no tiene un perfil de paciente vinculado", entities.ErrForbidden)
	}
	return *a.PacienteID, nil
}

// MedicoVinculado devuelve el ID de médico del actor o un error de dominio si
// la cuenta no tiene un perfil médico activo vinculado.
func (a Actor) MedicoVinculado() (int64, error) {
	if a.MedicoID == nil || *a.MedicoID <= 0 {
		return 0, fmt.Errorf("%w: la cuenta no tiene un perfil médico vinculado", entities.ErrForbidden)
	}
	return *a.MedicoID, nil
}
