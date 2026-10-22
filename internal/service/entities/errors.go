package entities

import (
	"errors"
	"fmt"
)

var (
	// ErrNotFound indica que el recurso solicitado no existe.
	ErrNotFound = errors.New("recurso no encontrado")
	// ErrConflict indica una violación de unicidad o una regla de negocio.
	ErrConflict = errors.New("conflicto con datos existentes")
	// ErrValidation indica que los datos recibidos no son válidos.
	ErrValidation = errors.New("datos inválidos")
	// ErrUnauthorized indica credenciales inválidas o sesión no válida.
	ErrUnauthorized = errors.New("no autorizado")
	// ErrForbidden indica que el usuario no tiene permisos suficientes.
	ErrForbidden = errors.New("acceso denegado")
	// ErrLocked indica una cuenta bloqueada temporalmente.
	ErrLocked = errors.New("cuenta bloqueada temporalmente")
	// ErrRateLimited indica demasiadas solicitudes en poco tiempo.
	ErrRateLimited = errors.New("demasiadas solicitudes")
)

// ValidationError transporta un mensaje de validación legible.
type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string { return e.Message }

func (e ValidationError) Unwrap() error { return ErrValidation }

// NewValidationError construye un error de validación.
func NewValidationError(format string, args ...any) error {
	return ValidationError{Message: fmt.Sprintf(format, args...)}
}
