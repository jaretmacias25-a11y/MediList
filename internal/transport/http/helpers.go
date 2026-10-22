package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"medilist/internal/service/entities"
)

const maxBodyBytes = 1 << 20 // 1 MiB

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return entities.NewValidationError("el cuerpo JSON es obligatorio")
		}
		return entities.NewValidationError("JSON inválido: %v", err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return entities.NewValidationError("el cuerpo debe contener un único objeto JSON")
	}
	return nil
}

func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, entities.NewValidationError("el id de la ruta no es válido")
	}
	return id, nil
}

func listOptions(r *http.Request) entities.ListOptions {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	return entities.ListOptions{
		Limit:       limit,
		Offset:      offset,
		Search:      strings.TrimSpace(q.Get("search")),
		SoloActivos: strings.EqualFold(q.Get("activos"), "true"),
	}.Normalized()
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func citaFiltro(r *http.Request) (entities.CitaFiltro, error) {
	filtro := entities.CitaFiltro{ListOptions: listOptions(r)}
	q := r.URL.Query()

	if v := strings.TrimSpace(q.Get("paciente_id")); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			return filtro, entities.NewValidationError("paciente_id no es válido")
		}
		filtro.PacienteID = id
	}
	if v := strings.TrimSpace(q.Get("medico_id")); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			return filtro, entities.NewValidationError("medico_id no es válido")
		}
		filtro.MedicoID = id
	}
	if v := strings.TrimSpace(q.Get("estado")); v != "" {
		filtro.Estado = entities.EstadoCita(v)
	}
	if v := strings.TrimSpace(q.Get("desde")); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return filtro, entities.NewValidationError("desde debe tener formato RFC3339")
		}
		filtro.Desde = &t
	}
	if v := strings.TrimSpace(q.Get("hasta")); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return filtro, entities.NewValidationError("hasta debe tener formato RFC3339")
		}
		filtro.Hasta = &t
	}
	return filtro, nil
}
