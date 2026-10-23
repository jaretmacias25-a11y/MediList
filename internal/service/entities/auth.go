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

// Usuario representa una cuenta de acceso al sistema.
type Usuario struct {
	ID               int64
	Username         string
	Email            string
	PasswordHash     string
	Nombres          string
	Apellidos        string
	Activo           bool
	TokenVersion     int
	IntentosFallidos int
	BloqueadoHasta   *time.Time
	UltimoAcceso     *time.Time
	Roles            []string
	Permisos         []string
	Version          int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
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
