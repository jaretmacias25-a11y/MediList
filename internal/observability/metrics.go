package observability

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

var (
	registros        atomic.Uint64
	verificaciones   atomic.Uint64
	reservas         atomic.Uint64
	conflictosSlot   atomic.Uint64
	derivaciones     atomic.Uint64
	conversiones     atomic.Uint64
	cancelaciones    atomic.Uint64
	fallosEmail      atomic.Uint64
	notificacionesOK atomic.Uint64
)

func IncRegistro()       { registros.Add(1) }
func IncVerificacion()   { verificaciones.Add(1) }
func IncReserva()        { reservas.Add(1) }
func IncConflictoSlot()  { conflictosSlot.Add(1) }
func IncDerivacion()     { derivaciones.Add(1) }
func IncConversion()     { conversiones.Add(1) }
func IncCancelacion()    { cancelaciones.Add(1) }
func IncFalloEmail()     { fallosEmail.Add(1) }
func IncNotificacionOK() { notificacionesOK.Add(1) }

// MetricsHandler expone contadores Prometheus sin etiquetas de usuario.
func MetricsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = fmt.Fprintf(w, `# HELP medilist_registros_total Registros de paciente completados.
# TYPE medilist_registros_total counter
medilist_registros_total %d
# HELP medilist_verificaciones_total Correos de cuenta verificados.
# TYPE medilist_verificaciones_total counter
medilist_verificaciones_total %d
# HELP medilist_reservas_total Citas creadas.
# TYPE medilist_reservas_total counter
medilist_reservas_total %d
# HELP medilist_conflictos_slot_total Reservas rechazadas por conflicto de slot.
# TYPE medilist_conflictos_slot_total counter
medilist_conflictos_slot_total %d
# HELP medilist_derivaciones_total Derivaciones emitidas.
# TYPE medilist_derivaciones_total counter
medilist_derivaciones_total %d
# HELP medilist_conversiones_total Reservas de especialista creadas desde una derivación.
# TYPE medilist_conversiones_total counter
medilist_conversiones_total %d
# HELP medilist_cancelaciones_total Citas canceladas por portal.
# TYPE medilist_cancelaciones_total counter
medilist_cancelaciones_total %d
# HELP medilist_fallos_email_total Fallos de envío de notificaciones.
# TYPE medilist_fallos_email_total counter
medilist_fallos_email_total %d
# HELP medilist_notificaciones_enviadas_total Notificaciones enviadas correctamente.
# TYPE medilist_notificaciones_enviadas_total counter
medilist_notificaciones_enviadas_total %d
`,
			registros.Load(), verificaciones.Load(), reservas.Load(),
			conflictosSlot.Load(), derivaciones.Load(), conversiones.Load(),
			cancelaciones.Load(), fallosEmail.Load(), notificacionesOK.Load(),
		)
	}
}
