package entities

import "time"

// TipoTokenCuenta identifica el propósito de un token de cuenta.
type TipoTokenCuenta string

const (
	TokenVerificacionEmail TipoTokenCuenta = "verificacion_email"
	TokenResetPassword     TipoTokenCuenta = "reset_password"
)

// TokenCuenta representa un token de un solo uso. Solo se persiste su hash.
type TokenCuenta struct {
	ID        int64
	UsuarioID int64
	Tipo      TipoTokenCuenta
	TokenHash string
	ExpiraEn  time.Time
	UsadoEn   *time.Time
	CreatedAt time.Time
}

// EstadoSolicitudVinculacion representa el ciclo de vida de una solicitud.
type EstadoSolicitudVinculacion string

const (
	SolicitudVinculacionPendiente EstadoSolicitudVinculacion = "pendiente"
	SolicitudVinculacionAprobada  EstadoSolicitudVinculacion = "aprobada"
	SolicitudVinculacionRechazada EstadoSolicitudVinculacion = "rechazada"
)

// Valido indica si el estado pertenece al catálogo permitido.
func (e EstadoSolicitudVinculacion) Valido() bool {
	switch e {
	case SolicitudVinculacionPendiente, SolicitudVinculacionAprobada, SolicitudVinculacionRechazada:
		return true
	default:
		return false
	}
}

// SolicitudVinculacion es la solicitud de una cuenta para vincularse con una
// historia clínica que ya existe sin usuario.
type SolicitudVinculacion struct {
	ID                   int64
	UsuarioID            int64
	Username             string
	Email                string
	PacienteID           int64
	NumeroHistoria       string
	Nombres              string
	Apellidos            string
	TipoDocumento        string
	NumeroDocumento      string
	Estado               EstadoSolicitudVinculacion
	Motivo               string
	ResueltaPorUsuarioID *int64
	ResueltaEn           *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// SolicitudVinculacionFiltro agrupa los filtros de consulta.
type SolicitudVinculacionFiltro struct {
	ListOptions
	Estado EstadoSolicitudVinculacion
}
