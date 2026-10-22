package entities

import "time"

// ErrorAplicacion representa un error estructurado listo para agregación de logs.
type ErrorAplicacion struct {
	ID         int64
	TraceID    string
	Servicio   string
	Componente string
	Nivel      string
	Metodo     string
	Ruta       string
	EstadoHTTP int
	Codigo     string
	Mensaje    string
	UsuarioID  *int64
	Username   string
	IP         string
	UserAgent  string
	StackTrace string
	CreatedAt  time.Time
}

// ErrorFiltro agrupa los filtros de consulta de errores.
type ErrorFiltro struct {
	ListOptions
	Nivel    string
	Servicio string
	TraceID  string
}
