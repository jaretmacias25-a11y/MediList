package httpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"medilist/internal/service"
	"medilist/internal/service/entities"
)

func TestPerfilRechazaIDsDePropietarioEnBody(t *testing.T) {
	cuerpos := []string{
		`{"paciente_id":2,"telefono":"0999999999","version":1}`,
		`{"medico_id":7,"telefono":"0999999999","version":1}`,
	}
	for _, cuerpo := range cuerpos {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/portal/perfil", strings.NewReader(cuerpo))
		rec := httptest.NewRecorder()
		var dst actualizarPerfilRequest
		err := decodeJSON(rec, req, &dst)
		if !errors.Is(err, entities.ErrValidation) {
			t.Fatalf("se esperaba ErrValidation para body %s, obtenido %v", cuerpo, err)
		}
	}
}

func TestPerfilAceptaSoloDatosDeContacto(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/v1/portal/perfil", strings.NewReader(
		`{"telefono":"0999999999","direccion":"Quito","contacto_emergencia_nombre":"Luis","contacto_emergencia_telefono":"0988888888","version":3}`))
	rec := httptest.NewRecorder()
	var dst actualizarPerfilRequest
	if err := decodeJSON(rec, req, &dst); err != nil {
		t.Fatalf("body de perfil válido rechazado: %v", err)
	}
	if dst.Version != 3 || dst.Telefono != "0999999999" {
		t.Fatalf("body decodificado inesperado: %+v", dst)
	}
}

func TestMeResponseIncluyeVinculosDelActor(t *testing.T) {
	pacienteID := int64(10)
	medicoID := int64(20)
	resp := nuevoMeResponse(entities.Usuario{ID: 7, Username: "cuenta"}, &service.Actor{
		UsuarioID:  7,
		PacienteID: &pacienteID,
		MedicoID:   &medicoID,
	})
	if resp.PacienteID == nil || *resp.PacienteID != pacienteID {
		t.Fatalf("paciente_id inesperado: %+v", resp.PacienteID)
	}
	if resp.MedicoID == nil || *resp.MedicoID != medicoID {
		t.Fatalf("medico_id inesperado: %+v", resp.MedicoID)
	}
}
