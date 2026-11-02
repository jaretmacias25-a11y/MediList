package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"medilist/internal/observability"
	"medilist/internal/service"
)

// Dependencies agrupa los servicios que consume la capa de transporte.
type Dependencies struct {
	Especialidades     *service.EspecialidadService
	Pacientes          *service.PacienteService
	Medicos            *service.MedicoService
	Citas              *service.CitaService
	Auth               *service.AuthService
	Usuarios           *service.UsuarioService
	Cuentas            *service.CuentaService
	PortalPaciente     *service.PortalPacienteService
	MedicoPortal       *service.MedicoPortalService
	Agenda             *service.AgendaService
	Catalogo           *service.CatalogoService
	Disponibilidad     *service.DisponibilidadService
	PortalCitas        *service.PortalCitaService
	MedicoClinico      *service.MedicoClinicoService
	PacienteTransfer   *service.PacienteTransferService
	DerivacionAdmin    *service.DerivacionAdminService
	Notificaciones     *service.NotificacionService
	Auditoria          *service.AuditoriaService
	AuditoriaStore     service.AuditoriaStore
	ActorStore         service.ActorStore
	Errores            *service.ErrorService
	RateLimiter        *service.RateLimitService
	CORSAllowedOrigins []string
	HSTSEnabled        bool
	MetricsEnabled     bool
	DB                 *pgxpool.Pool
	Logger             *slog.Logger
}

func rateLimit(
	limiter *service.RateLimitService,
	logger *slog.Logger,
	key func(*http.Request) string,
	limit int,
	window time.Duration,
	next http.Handler,
) http.Handler {
	if limiter == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		permitido, espera, err := limiter.Permitir(r.Context(), key(r), limit, window)
		if err != nil {
			logger.Error("no se pudo aplicar rate limiting distribuido", "error", err)
			next.ServeHTTP(w, r)
			return
		}
		if !permitido {
			segundos := int(espera.Seconds())
			if segundos < 1 {
				segundos = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(segundos))
			writeError(w, http.StatusTooManyRequests, "rate_limited", "demasiadas solicitudes; espera un momento")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func rateKeyIP(prefijo string) func(*http.Request) string {
	return func(r *http.Request) string { return prefijo + ":" + clientIP(r) }
}

func rateKeyActor(prefijo string) func(*http.Request) string {
	return func(r *http.Request) string {
		if actor := actorFromContext(r.Context()); actor != nil {
			return fmt.Sprintf("%s:actor:%d", prefijo, actor.UsuarioID)
		}
		return prefijo + ":" + clientIP(r)
	}
}

// NewRouter construye el enrutador HTTP con todas las dependencias inyectadas.
func NewRouter(d Dependencies) http.Handler {
	mux := http.NewServeMux()
	auth := NewAuthMiddleware(d.Auth, d.AuditoriaStore, d.Logger, d.ActorStore)

	// Públicas
	mux.HandleFunc("GET /health", healthHandler(d.DB))
	if d.MetricsEnabled {
		mux.HandleFunc("GET /metrics", observability.MetricsHandler())
	}

	ah := NewAuthHandler(d.Auth, d.Logger)
	mux.Handle("POST /api/v1/auth/login", rateLimit(
		d.RateLimiter, d.Logger, rateKeyIP("login"), 30, time.Minute, http.HandlerFunc(ah.Login)))
	mux.Handle("POST /api/v1/auth/refresh", rateLimit(
		d.RateLimiter, d.Logger, rateKeyIP("refresh"), 60, time.Minute, http.HandlerFunc(ah.Refresh)))
	mux.HandleFunc("POST /api/v1/auth/logout", ah.Logout)
	mux.Handle("GET /api/v1/auth/me", auth.Require("", http.HandlerFunc(ah.Me)))
	mux.Handle("POST /api/v1/auth/password", auth.Require("", http.HandlerFunc(ah.CambiarPassword)))

	cuh := NewCuentaHandler(d.Cuentas, d.Logger)
	mux.Handle("POST /api/v1/auth/register", rateLimit(
		d.RateLimiter, d.Logger, rateKeyIP("register"), 5, time.Hour, http.HandlerFunc(cuh.Registrar)))
	mux.Handle("POST /api/v1/auth/verify-email", rateLimit(
		d.RateLimiter, d.Logger, rateKeyIP("verify"), 30, time.Minute, http.HandlerFunc(cuh.VerificarEmail)))
	mux.Handle("POST /api/v1/auth/resend-verification", rateLimit(
		d.RateLimiter, d.Logger, rateKeyIP("resend"), 3, 15*time.Minute, http.HandlerFunc(cuh.ReenviarVerificacion)))
	mux.Handle("POST /api/v1/auth/forgot-password", rateLimit(
		d.RateLimiter, d.Logger, rateKeyIP("forgot"), 5, 15*time.Minute, http.HandlerFunc(cuh.SolicitarResetPassword)))
	mux.Handle("POST /api/v1/auth/reset-password", rateLimit(
		d.RateLimiter, d.Logger, rateKeyIP("reset"), 10, 15*time.Minute, http.HandlerFunc(cuh.RestablecerPassword)))

	protegido := func(pattern, permiso string, handler http.HandlerFunc) {
		mux.Handle(pattern, auth.Require(permiso, handler))
	}
	escritura := func(pattern, permiso string, handler http.HandlerFunc) {
		mux.Handle(pattern, auth.Require(permiso,
			auditMutations(d.AuditoriaStore, d.Logger, handler)))
	}

	eh := NewEspecialidadHandler(d.Especialidades, d.Logger)
	protegido("GET /api/v1/especialidades", "especialidades:leer", eh.Listar)
	protegido("GET /api/v1/especialidades/{id}", "especialidades:leer", eh.Obtener)
	escritura("POST /api/v1/especialidades", "especialidades:escribir", eh.Crear)
	escritura("PUT /api/v1/especialidades/{id}", "especialidades:escribir", eh.Actualizar)
	escritura("DELETE /api/v1/especialidades/{id}", "especialidades:escribir", eh.Eliminar)

	ph := NewPacienteHandler(d.Pacientes, d.Logger)
	staffRead := func(pattern, permiso, accion, recurso string, handler http.HandlerFunc) {
		mux.Handle(pattern, auth.Require(permiso,
			auditDataAccess(d.AuditoriaStore, d.Logger, accion, recurso,
				soloPersonalAdministrativo(handler))))
	}
	staffWrite := func(pattern, permiso string, handler http.HandlerFunc) {
		mux.Handle(pattern, auth.Require(permiso,
			auditMutations(d.AuditoriaStore, d.Logger,
				soloPersonalAdministrativo(handler))))
	}
	staffRead("GET /api/v1/pacientes", "pacientes:leer", "consultar_pacientes", "pacientes", ph.Listar)
	staffRead("GET /api/v1/pacientes/{id}", "pacientes:leer", "consultar_paciente", "pacientes", ph.Obtener)
	staffWrite("POST /api/v1/pacientes", "pacientes:escribir", ph.Crear)
	staffWrite("PUT /api/v1/pacientes/{id}", "pacientes:escribir", ph.Actualizar)
	staffWrite("DELETE /api/v1/pacientes/{id}", "pacientes:escribir", ph.Eliminar)

	pth := NewPacienteTransferHandler(d.PacienteTransfer, d.Logger)
	staffWrite("POST /api/v1/pacientes/{id}/transferir", "pacientes:transferir", pth.Transferir)
	staffRead("GET /api/v1/pacientes/{id}/transferencias", "pacientes:transferir",
		"consultar_transferencias", "transferencias_paciente", pth.ListarTransferencias)

	mh := NewMedicoHandler(d.Medicos, d.Logger)
	protegido("GET /api/v1/medicos", "medicos:leer", mh.Listar)
	protegido("GET /api/v1/medicos/{id}", "medicos:leer", mh.Obtener)
	escritura("POST /api/v1/medicos", "medicos:escribir", mh.Crear)
	escritura("PUT /api/v1/medicos/{id}", "medicos:escribir", mh.Actualizar)
	escritura("DELETE /api/v1/medicos/{id}", "medicos:escribir", mh.Eliminar)

	agh := NewAgendaHandler(d.Agenda, d.Logger)
	staffRead("GET /api/v1/medicos/{id}/horarios", "agenda:gestionar", "consultar_horarios", "horarios_profesional", agh.ListarHorarios)
	staffWrite("POST /api/v1/medicos/{id}/horarios", "agenda:gestionar", agh.CrearHorario)
	staffWrite("PUT /api/v1/medicos/{id}/horarios/{horarioId}", "agenda:gestionar", agh.ActualizarHorario)
	staffWrite("DELETE /api/v1/medicos/{id}/horarios/{horarioId}", "agenda:gestionar", agh.DesactivarHorario)
	staffRead("GET /api/v1/medicos/{id}/bloqueos", "agenda:gestionar", "consultar_bloqueos", "bloqueos_agenda", agh.ListarBloqueos)
	staffWrite("POST /api/v1/medicos/{id}/bloqueos", "agenda:gestionar", agh.CrearBloqueo)
	staffWrite("PUT /api/v1/medicos/{id}/bloqueos/{bloqueoId}", "agenda:gestionar", agh.ActualizarBloqueo)
	staffWrite("DELETE /api/v1/medicos/{id}/bloqueos/{bloqueoId}", "agenda:gestionar", agh.EliminarBloqueo)

	ch := NewCitaHandler(d.Citas, d.Logger)
	staffRead("GET /api/v1/citas", "citas:leer", "consultar_citas", "citas", ch.Listar)
	staffRead("GET /api/v1/citas/{id}", "citas:leer", "consultar_cita", "citas", ch.Obtener)
	staffWrite("POST /api/v1/citas", "citas:escribir", ch.Crear)
	staffWrite("PUT /api/v1/citas/{id}", "citas:escribir", ch.Actualizar)
	staffWrite("PATCH /api/v1/citas/{id}/estado", "citas:escribir", ch.CambiarEstado)
	staffWrite("DELETE /api/v1/citas/{id}", "citas:escribir", ch.Eliminar)

	dah := NewDerivacionAdminHandler(d.DerivacionAdmin, d.Logger)
	mux.Handle("POST /api/v1/citas/{id}/sobrescribir-derivacion", auth.Require("derivaciones:sobrescribir",
		auditMutations(d.AuditoriaStore, d.Logger,
			soloAdmin(http.HandlerFunc(dah.Sobrescribir)))))

	portalHandler := NewPortalHandler(d.PortalPaciente, d.Logger)
	mux.Handle("GET /api/v1/portal/perfil", auth.Require("portal:perfil:leer",
		auditDataAccess(d.AuditoriaStore, d.Logger, "consultar_perfil_propio", "perfil_paciente",
			http.HandlerFunc(portalHandler.Perfil))))
	mux.Handle("PUT /api/v1/portal/perfil", auth.Require("portal:perfil:escribir",
		auditMutations(d.AuditoriaStore, d.Logger, http.HandlerFunc(portalHandler.ActualizarPerfil))))

	nh := NewNotificacionHandler(d.Notificaciones, d.Logger)
	mux.Handle("GET /api/v1/portal/preferencias-notificacion", auth.Require("portal:perfil:leer",
		auditDataAccess(d.AuditoriaStore, d.Logger, "consultar_preferencias_notificacion", "preferencias_notificacion",
			http.HandlerFunc(nh.ObtenerPreferencias))))
	mux.Handle("PUT /api/v1/portal/preferencias-notificacion", auth.Require("portal:perfil:escribir",
		auditMutations(d.AuditoriaStore, d.Logger, http.HandlerFunc(nh.ActualizarPreferencias))))

	catalogoHandler := NewCatalogoHandler(d.Catalogo, d.Disponibilidad, d.Logger)
	protegido("GET /api/v1/portal/especialidades", "catalogo:leer", catalogoHandler.Especialidades)
	protegido("GET /api/v1/portal/medicos", "catalogo:leer", catalogoHandler.Medicos)
	protegido("GET /api/v1/portal/disponibilidad", "portal:citas:leer", catalogoHandler.Disponibilidad)

	portalCitaHandler := NewPortalCitaHandler(d.PortalCitas, d.Logger)
	mux.Handle("POST /api/v1/portal/citas", auth.Require("portal:citas:escribir",
		rateLimit(d.RateLimiter, d.Logger, rateKeyActor("booking"), 20, time.Hour,
			auditMutations(d.AuditoriaStore, d.Logger, http.HandlerFunc(portalCitaHandler.Crear)))))
	mux.Handle("GET /api/v1/portal/citas", auth.Require("portal:citas:leer",
		auditDataAccess(d.AuditoriaStore, d.Logger, "consultar_citas_propias", "citas",
			http.HandlerFunc(portalCitaHandler.Listar))))
	mux.Handle("GET /api/v1/portal/citas/{id}", auth.Require("portal:citas:leer",
		auditDataAccess(d.AuditoriaStore, d.Logger, "consultar_cita_propia", "citas",
			http.HandlerFunc(portalCitaHandler.Obtener))))
	mux.Handle("PUT /api/v1/portal/citas/{id}", auth.Require("portal:citas:escribir",
		auditMutations(d.AuditoriaStore, d.Logger, http.HandlerFunc(portalCitaHandler.Reprogramar))))
	mux.Handle("POST /api/v1/portal/citas/{id}/cancelar", auth.Require("portal:citas:escribir",
		auditMutations(d.AuditoriaStore, d.Logger, http.HandlerFunc(portalCitaHandler.Cancelar))))
	mux.Handle("GET /api/v1/portal/derivaciones", auth.Require("portal:derivaciones:leer",
		auditDataAccess(d.AuditoriaStore, d.Logger, "consultar_derivaciones_propias", "derivaciones",
			http.HandlerFunc(portalCitaHandler.ListarDerivaciones))))
	mux.Handle("GET /api/v1/portal/derivaciones/{id}", auth.Require("portal:derivaciones:leer",
		auditDataAccess(d.AuditoriaStore, d.Logger, "consultar_derivacion_propia", "derivaciones",
			http.HandlerFunc(portalCitaHandler.ObtenerDerivacion))))

	medicoPortalHandler := NewMedicoPortalHandler(d.MedicoPortal, d.Logger)
	mux.Handle("GET /api/v1/medico/agenda", auth.Require("medico:agenda:leer",
		auditDataAccess(d.AuditoriaStore, d.Logger, "consultar_agenda_propia", "agenda_medico",
			http.HandlerFunc(medicoPortalHandler.Agenda))))
	mux.Handle("GET /api/v1/medico/citas/{id}", auth.Require("medico:agenda:leer",
		auditDataAccess(d.AuditoriaStore, d.Logger, "consultar_cita_propia", "citas",
			http.HandlerFunc(medicoPortalHandler.ObtenerCita))))
	mux.Handle("GET /api/v1/medico/pacientes/{id}", auth.Require("pacientes:atendidos:leer",
		auditDataAccess(d.AuditoriaStore, d.Logger, "consultar_paciente_atendido", "pacientes",
			http.HandlerFunc(medicoPortalHandler.ObtenerPaciente))))

	mch := NewMedicoClinicoHandler(d.MedicoClinico, d.Logger)
	mux.Handle("PATCH /api/v1/medico/citas/{id}/estado", auth.Require("medico:agenda:escribir",
		auditMutations(d.AuditoriaStore, d.Logger, http.HandlerFunc(mch.CambiarEstado))))
	mux.Handle("GET /api/v1/medico/citas/{id}/nota", auth.Require("medico:nota:leer",
		auditDataAccess(d.AuditoriaStore, d.Logger, "consultar_nota_propia", "notas_cita",
			http.HandlerFunc(mch.ObtenerNota))))
	mux.Handle("POST /api/v1/medico/citas/{id}/nota", auth.Require("medico:nota:escribir",
		auditMutations(d.AuditoriaStore, d.Logger, http.HandlerFunc(mch.GuardarNota))))
	mux.Handle("PUT /api/v1/medico/citas/{id}/nota", auth.Require("medico:nota:escribir",
		auditMutations(d.AuditoriaStore, d.Logger, http.HandlerFunc(mch.GuardarNota))))
	mux.Handle("POST /api/v1/medico/citas/{id}/derivaciones", auth.Require("derivaciones:escribir",
		auditMutations(d.AuditoriaStore, d.Logger, http.HandlerFunc(mch.CrearDerivacion))))
	mux.Handle("GET /api/v1/medico/derivaciones", auth.Require("derivaciones:leer",
		auditDataAccess(d.AuditoriaStore, d.Logger, "consultar_derivaciones_propias", "derivaciones",
			http.HandlerFunc(mch.ListarDerivaciones))))

	uh := NewUsuarioHandler(d.Usuarios, d.Logger)
	protegido("GET /api/v1/usuarios", "usuarios:leer", uh.Listar)
	protegido("GET /api/v1/usuarios/{id}", "usuarios:leer", uh.Obtener)
	protegido("GET /api/v1/roles", "roles:leer", uh.ListarRoles)
	escritura("POST /api/v1/usuarios", "usuarios:escribir", uh.Crear)
	escritura("PUT /api/v1/usuarios/{id}", "usuarios:escribir", uh.Actualizar)
	escritura("POST /api/v1/usuarios/{id}/desbloquear", "usuarios:escribir", uh.Desbloquear)
	escritura("POST /api/v1/usuarios/{id}/revocar-sesiones", "usuarios:escribir", uh.RevocarSesiones)

	escritura("POST /api/v1/cuentas/vinculaciones/{id}/aprobar", "cuentas:vincular", cuh.AprobarSolicitud)
	escritura("POST /api/v1/cuentas/vinculaciones/{id}/rechazar", "cuentas:vincular", cuh.RechazarSolicitud)
	escritura("POST /api/v1/cuentas/vincular-paciente", "cuentas:vincular", cuh.VincularPaciente)
	escritura("POST /api/v1/cuentas/vincular-medico", "usuarios:escribir", cuh.VincularMedico)

	consultaAuditada := func(pattern, permiso string, handler http.HandlerFunc) {
		mux.Handle(pattern, auth.Require(permiso,
			auditSensitiveReads(d.AuditoriaStore, d.Logger, handler)))
	}

	consultaAuditada("GET /api/v1/cuentas/vinculaciones", "cuentas:vincular", cuh.ListarSolicitudes)

	audh := NewAuditoriaHandler(d.Auditoria, d.Logger)
	consultaAuditada("GET /api/v1/auditoria", "auditoria:leer", audh.Listar)

	erh := NewErrorHandler(d.Errores, d.Logger)
	consultaAuditada("GET /api/v1/errores", "errores:leer", erh.Listar)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "ruta no encontrada")
	})

	return withMiddleware(mux, d.Logger, d.Errores, d.CORSAllowedOrigins, d.HSTSEnabled)
}

func healthHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status":   "degraded",
				"database": "down",
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"status":   "ok",
			"database": "up",
		})
	}
}
