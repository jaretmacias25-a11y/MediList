package entities

import "time"

// TipoNotificacion identifica la plantilla usada por el worker.
type TipoNotificacion string

const (
	NotificacionVerificacionEmail   TipoNotificacion = "verificacion_email"
	NotificacionResetPassword       TipoNotificacion = "reset_password"
	NotificacionCitaCreada          TipoNotificacion = "cita_creada"
	NotificacionCitaConfirmada      TipoNotificacion = "cita_confirmada"
	NotificacionCitaCancelada       TipoNotificacion = "cita_cancelada"
	NotificacionCitaReprogramada    TipoNotificacion = "cita_reprogramada"
	NotificacionDerivacionEmitida   TipoNotificacion = "derivacion_emitida"
	NotificacionDerivacionReservada TipoNotificacion = "derivacion_reservada"
	NotificacionRecordatorio        TipoNotificacion = "recordatorio_cita"
)

// EstadoNotificacion representa el estado del outbox.
type EstadoNotificacion string

const (
	EstadoNotificacionPendiente EstadoNotificacion = "pendiente"
	EstadoNotificacionEnviando  EstadoNotificacion = "enviando"
	EstadoNotificacionEnviada   EstadoNotificacion = "enviada"
	EstadoNotificacionFallida   EstadoNotificacion = "fallida"
)

// Notificacion es una entrada del outbox. El cuerpo se renderiza antes de
// encolar para que el worker no dependa de datos clínicos cambiantes.
type Notificacion struct {
	ID             int64
	Tipo           TipoNotificacion
	UsuarioID      *int64
	Destinatario   string
	Nombre         string
	Asunto         string
	Cuerpo         string
	Estado         EstadoNotificacion
	Intentos       int
	ProximoIntento time.Time
	UltimoError    string
	DedupeKey      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	EnviadaEn      *time.Time
}

// PreferenciasNotificacion controla los correos de una cuenta.
type PreferenciasNotificacion struct {
	UsuarioID                int64
	EmailHabilitado          bool
	RecordatoriosHabilitados bool
	UpdatedAt                time.Time
}
