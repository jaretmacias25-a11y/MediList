package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
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

// decodeOptionalJSON acepta un cuerpo ausente o vacío y, si existe, exige un
// único objeto JSON sin campos desconocidos.
func decodeOptionalJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return entities.NewValidationError("JSON inválido: %v", err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return entities.NewValidationError("el cuerpo debe contener un único objeto JSON")
	}
	return nil
}

func pathID(r *http.Request) (int64, error) {
	return pathInt(r, "id")
}

func pathInt(r *http.Request, nombre string) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue(nombre), 10, 64)
	if err != nil || id <= 0 {
		return 0, entities.NewValidationError("el %s de la ruta no es válido", nombre)
	}
	return id, nil
}

func queryInt64(r *http.Request, nombre string) (int64, error) {
	texto := strings.TrimSpace(r.URL.Query().Get(nombre))
	if texto == "" {
		return 0, nil
	}
	id, err := strconv.ParseInt(texto, 10, 64)
	if err != nil || id <= 0 {
		return 0, entities.NewValidationError("%s no es válido", nombre)
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

var trustedProxyNets []*net.IPNet

// SetTrustedProxies configura las redes cuyos X-Forwarded-For son confiables.
// Debe llamarse una vez al arrancar, antes de aceptar solicitudes.
func SetTrustedProxies(valores []string) error {
	redes := make([]*net.IPNet, 0, len(valores))
	for _, valor := range valores {
		if strings.Contains(valor, "/") {
			_, red, err := net.ParseCIDR(valor)
			if err != nil {
				return fmt.Errorf("CIDR de proxy inválido %q: %w", valor, err)
			}
			redes = append(redes, red)
			continue
		}
		ip := net.ParseIP(valor)
		if ip == nil {
			return fmt.Errorf("IP de proxy inválida %q", valor)
		}
		bits := 128
		if ip.To4() != nil {
			ip = ip.To4()
			bits = 32
		}
		redes = append(redes, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
	}
	trustedProxyNets = redes
	return nil
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil || !esProxyConfiable(ip) {
		return host
	}
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		partes := strings.Split(forwarded, ",")
		for i := len(partes) - 1; i >= 0; i-- {
			candidato := net.ParseIP(strings.TrimSpace(partes[i]))
			if candidato == nil || esProxyConfiable(candidato) {
				continue
			}
			return candidato.String()
		}
	}
	if real := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); real != nil {
		return real.String()
	}
	return host
}

func esProxyConfiable(ip net.IP) bool {
	for _, red := range trustedProxyNets {
		if red.Contains(ip) {
			return true
		}
	}
	return false
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
