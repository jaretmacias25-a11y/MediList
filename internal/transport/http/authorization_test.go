package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"medilist/internal/service"
)

func TestSoloPersonalAdministrativoBloqueaMedico(t *testing.T) {
	handler := soloPersonalAdministrativo(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	casos := []struct {
		nombre string
		actor  *service.Actor
		estado int
	}{
		{"médico", &service.Actor{UsuarioID: 1, Roles: []string{"medico"}}, http.StatusForbidden},
		{"recepción", &service.Actor{UsuarioID: 2, Roles: []string{"recepcion"}}, http.StatusNoContent},
		{"admin", &service.Actor{UsuarioID: 3, Roles: []string{"admin"}}, http.StatusNoContent},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/pacientes", nil)
			req = req.WithContext(context.WithValue(req.Context(), actorKey, caso.actor))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != caso.estado {
				t.Fatalf("estado esperado %d, obtenido %d", caso.estado, rec.Code)
			}
		})
	}
}

func TestAuditDataAccessMarcaIntentoDenegado(t *testing.T) {
	store := &fakeAuditoriaStore{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := auditDataAccess(store, logger, "consultar_agenda_propia", "agenda_medico",
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeError(w, http.StatusForbidden, "forbidden", "cita ajena")
		}))

	actor := &service.Actor{UsuarioID: 7, Username: "medico.a", Roles: []string{"medico"}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/medico/agenda", nil)
	req = req.WithContext(context.WithValue(req.Context(), actorKey, actor))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if len(store.registros) != 1 {
		t.Fatalf("se esperaba 1 registro de auditoría, obtenidos %d", len(store.registros))
	}
	registro := store.registros[0]
	if registro.Accion != "consultar_agenda_propia_denegado" {
		t.Fatalf("acción inesperada: %q", registro.Accion)
	}
	if registro.EstadoHTTP != http.StatusForbidden || registro.Recurso != "agenda_medico" {
		t.Fatalf("registro inesperado: %+v", registro)
	}
}

func TestAuditDataAccessRegistraConsultaAutorizada(t *testing.T) {
	store := &fakeAuditoriaStore{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := auditDataAccess(store, logger, "consultar_perfil_propio", "perfil_paciente",
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

	actor := &service.Actor{UsuarioID: 8, Username: "paciente.a", Roles: []string{"paciente"}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/perfil", nil)
	req = req.WithContext(context.WithValue(req.Context(), actorKey, actor))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if len(store.registros) != 1 {
		t.Fatalf("se esperaba 1 registro de auditoría, obtenidos %d", len(store.registros))
	}
	if store.registros[0].Accion != "consultar_perfil_propio" {
		t.Fatalf("acción inesperada: %q", store.registros[0].Accion)
	}
}
