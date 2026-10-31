package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"medilist/internal/service"
)

// Dependencies agrupa los servicios que consume la capa de transporte.
type Dependencies struct {
	Especialidades *service.EspecialidadService
	Pacientes      *service.PacienteService
	Medicos        *service.MedicoService
	Citas          *service.CitaService
	Auth           *service.AuthService
	Usuarios       *service.UsuarioService
	Cuentas        *service.CuentaService
	PortalPaciente *service.PortalPacienteService
	MedicoPortal   *service.MedicoPortalService
	Auditoria      *service.AuditoriaService
	AuditoriaStore service.AuditoriaStore
	ActorStore     service.ActorStore
	Errores        *service.ErrorService
	DB             *pgxpool.Pool
	Logger         *slog.Logger
}

// NewRouter construye el enrutador HTTP con todas las dependencias inyectadas.
func NewRouter(d Dependencies) http.Handler {
	mux := http.NewServeMux()
	auth := NewAuthMiddleware(d.Auth, d.AuditoriaStore, d.Logger, d.ActorStore)

	// Públicas
	mux.HandleFunc("GET /health", healthHandler(d.DB))

	ah := NewAuthHandler(d.Auth, d.Logger)
	mux.HandleFunc("POST /api/v1/auth/login", ah.Login)
	mux.HandleFunc("POST /api/v1/auth/refresh", ah.Refresh)
	mux.HandleFunc("POST /api/v1/auth/logout", ah.Logout)
	mux.Handle("GET /api/v1/auth/me", auth.Require("", http.HandlerFunc(ah.Me)))
	mux.Handle("POST /api/v1/auth/password", auth.Require("", http.HandlerFunc(ah.CambiarPassword)))

	cuh := NewCuentaHandler(d.Cuentas, d.Logger)
	mux.HandleFunc("POST /api/v1/auth/register", cuh.Registrar)
	mux.HandleFunc("POST /api/v1/auth/verify-email", cuh.VerificarEmail)
	mux.HandleFunc("POST /api/v1/auth/resend-verification", cuh.ReenviarVerificacion)
	mux.HandleFunc("POST /api/v1/auth/forgot-password", cuh.SolicitarResetPassword)
	mux.HandleFunc("POST /api/v1/auth/reset-password", cuh.RestablecerPassword)

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

	mh := NewMedicoHandler(d.Medicos, d.Logger)
	protegido("GET /api/v1/medicos", "medicos:leer", mh.Listar)
	protegido("GET /api/v1/medicos/{id}", "medicos:leer", mh.Obtener)
	escritura("POST /api/v1/medicos", "medicos:escribir", mh.Crear)
	escritura("PUT /api/v1/medicos/{id}", "medicos:escribir", mh.Actualizar)
	escritura("DELETE /api/v1/medicos/{id}", "medicos:escribir", mh.Eliminar)

	ch := NewCitaHandler(d.Citas, d.Logger)
	staffRead("GET /api/v1/citas", "citas:leer", "consultar_citas", "citas", ch.Listar)
	staffRead("GET /api/v1/citas/{id}", "citas:leer", "consultar_cita", "citas", ch.Obtener)
	staffWrite("POST /api/v1/citas", "citas:escribir", ch.Crear)
	staffWrite("PUT /api/v1/citas/{id}", "citas:escribir", ch.Actualizar)
	staffWrite("PATCH /api/v1/citas/{id}/estado", "citas:escribir", ch.CambiarEstado)
	staffWrite("DELETE /api/v1/citas/{id}", "citas:escribir", ch.Eliminar)

	portalHandler := NewPortalHandler(d.PortalPaciente, d.Logger)
	mux.Handle("GET /api/v1/portal/perfil", auth.Require("portal:perfil:leer",
		auditDataAccess(d.AuditoriaStore, d.Logger, "consultar_perfil_propio", "perfil_paciente",
			http.HandlerFunc(portalHandler.Perfil))))
	mux.Handle("PUT /api/v1/portal/perfil", auth.Require("portal:perfil:escribir",
		auditMutations(d.AuditoriaStore, d.Logger, http.HandlerFunc(portalHandler.ActualizarPerfil))))

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

	return withMiddleware(mux, d.Logger, d.Errores)
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
