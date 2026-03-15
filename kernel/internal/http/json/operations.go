package jsontransport

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"as/kernel/internal/interactions"
)

type OperationsAPI struct {
	Service *interactions.RuntimeService
}

func NewOperationsAPI(service *interactions.RuntimeService) *OperationsAPI {
	return &OperationsAPI{Service: service}
}

func (api *OperationsAPI) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/projections/runtime/process-samples", api.handleProcessSamples)
	mux.HandleFunc("GET /v1/projections/runtime/service-events", api.handleServiceEvents)
	mux.HandleFunc("GET /v1/projections/runtime/realization-suspensions", api.handleRealizationSuspensions)
}

func (api *OperationsAPI) handleProcessSamples(w http.ResponseWriter, r *http.Request) {
	if api.Service == nil {
		respondError(w, http.StatusServiceUnavailable, errors.New("runtime service unavailable"))
		return
	}
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	items, err := api.Service.ListProcessSamples(r.Context(), interactions.ListProcessSamplesInput{
		ScopeKind:   strings.TrimSpace(r.URL.Query().Get("scope_kind")),
		ServiceName: strings.TrimSpace(r.URL.Query().Get("service_name")),
		ExecutionID: strings.TrimSpace(r.URL.Query().Get("execution_id")),
		Reference:   strings.TrimSpace(r.URL.Query().Get("reference")),
		Limit:       limit,
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"samples": items})
}

func (api *OperationsAPI) handleServiceEvents(w http.ResponseWriter, r *http.Request) {
	if api.Service == nil {
		respondError(w, http.StatusServiceUnavailable, errors.New("runtime service unavailable"))
		return
	}
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	items, err := api.Service.ListServiceEvents(r.Context(), interactions.ListServiceEventsInput{
		ServiceName: strings.TrimSpace(r.URL.Query().Get("service_name")),
		EventName:   strings.TrimSpace(r.URL.Query().Get("event_name")),
		Limit:       limit,
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"events": items})
}

func (api *OperationsAPI) handleRealizationSuspensions(w http.ResponseWriter, r *http.Request) {
	if api.Service == nil {
		respondError(w, http.StatusServiceUnavailable, errors.New("runtime service unavailable"))
		return
	}
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	activeOnly := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("active_only")), "true") ||
		strings.TrimSpace(r.URL.Query().Get("active_only")) == "1"
	items, err := api.Service.ListRealizationSuspensions(r.Context(), interactions.ListRealizationSuspensionsInput{
		Reference:   strings.TrimSpace(r.URL.Query().Get("reference")),
		ExecutionID: strings.TrimSpace(r.URL.Query().Get("execution_id")),
		ActiveOnly:  activeOnly,
		Limit:       limit,
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"suspensions": items})
}
