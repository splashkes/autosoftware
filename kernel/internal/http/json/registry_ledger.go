package jsontransport

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"as/kernel/internal/http/server"
	"as/kernel/internal/interactions"
)

type RegistryLedgerAPI struct {
	Service *interactions.RuntimeService
}

func NewRegistryLedgerAPI(service *interactions.RuntimeService) *RegistryLedgerAPI {
	return &RegistryLedgerAPI{Service: service}
}

func (api *RegistryLedgerAPI) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/registry/change-sets", api.handleListRegistryChangeSets)
	mux.HandleFunc("GET /v1/registry/change-sets/{change_set_id}", api.handleGetRegistryChangeSet)
	mux.HandleFunc("GET /v1/registry/rows", api.handleListRegistryRows)
	mux.HandleFunc("GET /v1/registry/rows/{row_id}", api.handleGetRegistryRow)
}

func (api *RegistryLedgerAPI) handleListRegistryChangeSets(w http.ResponseWriter, r *http.Request) {
	if api.Service == nil {
		respondError(w, http.StatusServiceUnavailable, server.ServiceUnavailable("runtime service unavailable"))
		return
	}
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	items, err := api.Service.ListRegistryChangeSets(r.Context(), strings.TrimSpace(r.URL.Query().Get("reference")), limit)
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"change_sets": items})
}

func (api *RegistryLedgerAPI) handleGetRegistryChangeSet(w http.ResponseWriter, r *http.Request) {
	if api.Service == nil {
		respondError(w, http.StatusServiceUnavailable, server.ServiceUnavailable("runtime service unavailable"))
		return
	}
	item, err := api.Service.GetRegistryChangeSet(r.Context(), r.PathValue("change_set_id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	respondJSON(w, http.StatusOK, item)
}

func (api *RegistryLedgerAPI) handleListRegistryRows(w http.ResponseWriter, r *http.Request) {
	if api.Service == nil {
		respondError(w, http.StatusServiceUnavailable, server.ServiceUnavailable("runtime service unavailable"))
		return
	}
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	after, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("after")), 10, 64)
	items, err := api.Service.ListRegistryRows(r.Context(), interactions.ListRegistryRowsInput{
		Reference:     strings.TrimSpace(r.URL.Query().Get("reference")),
		SeedID:        strings.TrimSpace(r.URL.Query().Get("seed_id")),
		RealizationID: strings.TrimSpace(r.URL.Query().Get("realization_id")),
		AfterRowID:    after,
		Limit:         limit,
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"rows": items})
}

func (api *RegistryLedgerAPI) handleGetRegistryRow(w http.ResponseWriter, r *http.Request) {
	if api.Service == nil {
		respondError(w, http.StatusServiceUnavailable, server.ServiceUnavailable("runtime service unavailable"))
		return
	}
	rowID, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("row_id")), 10, 64)
	if err != nil || rowID <= 0 {
		respondError(w, http.StatusBadRequest, errors.New("row_id must be a positive integer"))
		return
	}
	item, err := api.Service.GetRegistryRow(r.Context(), rowID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	respondJSON(w, http.StatusOK, item)
}
