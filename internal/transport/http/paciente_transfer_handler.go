package httpapi

import (
	"log/slog"
	"net/http"

	"medilist/internal/service"
)

// PacienteTransferHandler expone la transferencia interna de médico tratante.
type PacienteTransferHandler struct {
	svc    *service.PacienteTransferService
	logger *slog.Logger
}

// NewPacienteTransferHandler construye el manejador de transferencias.
func NewPacienteTransferHandler(
	svc *service.PacienteTransferService,
	logger *slog.Logger,
) *PacienteTransferHandler {
	return &PacienteTransferHandler{svc: svc, logger: logger}
}

func (h *PacienteTransferHandler) Transferir(w http.ResponseWriter, r *http.Request) {
	actor := actorFromContext(r.Context())
	if actor == nil {
		writeUnauthorized(w)
		return
	}
	pacienteID, err := pathID(r)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	var req transferirPacienteRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	transferencia, err := h.svc.Transferir(
		r.Context(), actor, pacienteID, req.toService())
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	writeData(w, http.StatusOK, nuevaTransferenciaPacienteResponse(*transferencia))
}

func (h *PacienteTransferHandler) ListarTransferencias(
	w http.ResponseWriter,
	r *http.Request,
) {
	actor := actorFromContext(r.Context())
	if actor == nil {
		writeUnauthorized(w)
		return
	}
	pacienteID, err := pathID(r)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	lista, err := h.svc.ListarTransferencias(r.Context(), actor, pacienteID)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	resp := make([]transferenciaPacienteResponse, 0, len(lista))
	for _, transferencia := range lista {
		resp = append(resp, nuevaTransferenciaPacienteResponse(transferencia))
	}
	writeCollection(w, resp, len(resp))
}
