package entities

import "time"

// Rol agrupa un conjunto de permisos.
type Rol struct {
	ID          int64
	Codigo      string
	Nombre      string
	Descripcion string
	Permisos    []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// VinculosUsuario contiene los perfiles clínicos asociados a una cuenta.
// Se resuelven desde pacientes.usuario_id y medicos.usuario_id. Un vínculo a
// un perfil inactivo no se considera válido para el portal.
type VinculosUsuario struct {
	Email           string
	EmailVerificado bool
	PacienteID      *int64
	MedicoID        *int64
}

// Usuario representa una cuenta de acceso al sistema.
type Usuario struct {
	ID                  int64
	Username            string
	Email               string
	PasswordHash        string
	Nombres             string
	Apellidos           string
	Activo              bool
	TokenVersion        int
	IntentosFallidos    int
	BloqueadoHasta      *time.Time
	UltimoAcceso        *time.Time
	EmailVerificadoEn   *time.Time
	TerminosAceptadosEn *time.Time
	VersionTerminos     string
	DebeCambiarPassword bool
	Roles               []string
	Permisos            []string
	Version             int64
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// EmailVerificado indica si la cuenta completó la verificación de email.
func (u Usuario) EmailVerificado() bool {
	return u.EmailVerificadoEn != nil
}

// RequiereVerificacionEmail indica si la cuenta es exclusivamente de paciente
// y todavía no verificó su correo. Las cuentas internas existentes quedan
// exentas para no bloquear el acceso administrativo durante la transición.
func (u Usuario) RequiereVerificacionEmail() bool {
	if u.EmailVerificado() || len(u.Roles) == 0 {
		return false
	}
	for _, rol := range u.Roles {
		if rol != "paciente" {
			return false
		}
	}
	return true
}

// RefreshToken representa un token de refresco almacenado de forma hasheada.
type RefreshToken struct {
	ID           int64
	UsuarioID    int64
	TokenHash    string
	TokenVersion int
	ExpiraEn     time.Time
	RevocadoEn   *time.Time
	IP           string
	UserAgent    string
	CreatedAt    time.Time
}

// Auditoria registra una acción relevante para la trazabilidad del sistema.
type Auditoria struct {
	ID         int64
	UsuarioID  *int64
	Username   string
	Accion     string
	Recurso    string
	Metodo     string
	Ruta       string
	EstadoHTTP int
	IP         string
	UserAgent  string
	TraceID    string
	CreatedAt  time.Time
}

// AuditoriaFiltro agrupa los filtros de consulta de auditoría.
type AuditoriaFiltro struct {
	ListOptions
	UsuarioID int64
	Accion    string
	TraceID   string
}
