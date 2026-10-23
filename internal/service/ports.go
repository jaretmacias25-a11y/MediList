package service

import (
	"context"
	"time"

	"medilist/internal/service/entities"
)

// EspecialidadStore es el puerto que debe implementar el repositorio.
type EspecialidadStore interface {
	Crear(ctx context.Context, e *entities.Especialidad) error
	ObtenerPorID(ctx context.Context, id int64) (*entities.Especialidad, error)
	Listar(ctx context.Context, opts entities.ListOptions) ([]entities.Especialidad, error)
	Actualizar(ctx context.Context, e *entities.Especialidad) error
	Eliminar(ctx context.Context, id int64) error
}

// PacienteStore es el puerto que debe implementar el repositorio.
type PacienteStore interface {
	Crear(ctx context.Context, p *entities.Paciente) error
	ObtenerPorID(ctx context.Context, id int64) (*entities.Paciente, error)
	Listar(ctx context.Context, opts entities.ListOptions) ([]entities.Paciente, error)
	Actualizar(ctx context.Context, p *entities.Paciente) error
	Eliminar(ctx context.Context, id int64) error
}

// MedicoStore es el puerto que debe implementar el repositorio.
type MedicoStore interface {
	Crear(ctx context.Context, m *entities.Medico) error
	ObtenerPorID(ctx context.Context, id int64) (*entities.Medico, error)
	Listar(ctx context.Context, opts entities.ListOptions) ([]entities.Medico, error)
	Actualizar(ctx context.Context, m *entities.Medico) error
	Eliminar(ctx context.Context, id int64) error
}

// CitaStore es el puerto que debe implementar el repositorio.
type CitaStore interface {
	Crear(ctx context.Context, c *entities.Cita) error
	ObtenerPorID(ctx context.Context, id int64) (*entities.Cita, error)
	Listar(ctx context.Context, filtro entities.CitaFiltro) ([]entities.Cita, error)
	Actualizar(ctx context.Context, c *entities.Cita) error
	ExisteSolape(ctx context.Context, medicoID int64, inicio time.Time, duracionMin int, excluirID int64) (bool, error)
}

// UsuarioStore es el puerto de persistencia de usuarios y sus roles.
type UsuarioStore interface {
	BuscarPorUsername(ctx context.Context, username string) (*entities.Usuario, error)
	ObtenerPorID(ctx context.Context, id int64) (*entities.Usuario, error)
	Listar(ctx context.Context, opts entities.ListOptions) ([]entities.Usuario, error)
	Crear(ctx context.Context, u *entities.Usuario) error
	Actualizar(ctx context.Context, u *entities.Usuario, passwordHash string, revocarSesiones bool) error
	RegistrarLoginExitoso(ctx context.Context, id int64) error
	RegistrarIntentoFallido(ctx context.Context, id int64, intentos int, bloqueadoHasta *time.Time) error
	RevocarSesiones(ctx context.Context, usuarioID int64) error
	Desbloquear(ctx context.Context, id int64) error
	SesionValida(ctx context.Context, usuarioID int64, tokenVersion int) (bool, error)
}

// RolStore es el puerto de consulta de roles y permisos.
type RolStore interface {
	Listar(ctx context.Context) ([]entities.Rol, error)
	ObtenerPorCodigos(ctx context.Context, codigos []string) ([]entities.Rol, error)
}

// RefreshTokenStore es el puerto de persistencia de refresh tokens.
type RefreshTokenStore interface {
	Crear(ctx context.Context, rt *entities.RefreshToken) error
	BuscarPorHash(ctx context.Context, hash string) (*entities.RefreshToken, error)
	Revocar(ctx context.Context, id int64) error
	RevocarTodosPorUsuario(ctx context.Context, usuarioID int64) error
}

// AuditoriaStore es el puerto de registro y consulta de auditoría.
type AuditoriaStore interface {
	Registrar(ctx context.Context, a *entities.Auditoria) error
	Listar(ctx context.Context, filtro entities.AuditoriaFiltro) ([]entities.Auditoria, error)
}

// ErrorStore es el puerto de registro y consulta de errores agregados.
type ErrorStore interface {
	Registrar(ctx context.Context, e *entities.ErrorAplicacion) error
	Listar(ctx context.Context, filtro entities.ErrorFiltro) ([]entities.ErrorAplicacion, error)
}

// RetentionStore elimina registros anteriores a una fecha de corte.
type RetentionStore interface {
	PurgarErrores(ctx context.Context, antes time.Time) (int64, error)
	PurgarAuditoria(ctx context.Context, antes time.Time) (int64, error)
}
