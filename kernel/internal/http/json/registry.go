package jsontransport

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"as/kernel/internal/registry"
)

type RegistryAPI struct {
	RepoRoot string
}

func NewRegistryAPI(repoRoot string) *RegistryAPI {
	return &RegistryAPI{RepoRoot: repoRoot}
}

func (api *RegistryAPI) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/registry/catalog", api.handleCatalog)
	mux.HandleFunc("GET /v1/registry/objects", api.handleObjects)
	mux.HandleFunc("GET /v1/registry/object", api.handleObject)
	mux.HandleFunc("GET /v1/registry/schemas", api.handleSchemas)
	mux.HandleFunc("GET /v1/registry/schema", api.handleSchema)
}

func (api *RegistryAPI) handleCatalog(w http.ResponseWriter, r *http.Request) {
	catalog, err := registry.LoadCatalog(api.RepoRoot)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"mode":      "catalog_projection",
		"summary":   catalog.Summary,
		"objects":   summarizeObjects(catalog.Objects),
		"schemas":   summarizeSchemas(catalog.Schemas),
		"discovery": registryDiscoveryPaths(),
	})
}

func (api *RegistryAPI) handleObjects(w http.ResponseWriter, r *http.Request) {
	catalog, err := registry.LoadCatalog(api.RepoRoot)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	seedID := strings.TrimSpace(r.URL.Query().Get("seed_id"))
	schemaRef := strings.TrimSpace(r.URL.Query().Get("schema_ref"))
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	items := registry.FilterObjects(catalog.Objects, seedID, schemaRef, query)

	respondJSON(w, http.StatusOK, map[string]any{
		"mode":    "catalog_projection",
		"filters": map[string]string{"seed_id": seedID, "schema_ref": schemaRef, "q": query},
		"objects": summarizeObjects(items),
	})
}

func (api *RegistryAPI) handleObject(w http.ResponseWriter, r *http.Request) {
	catalog, err := registry.LoadCatalog(api.RepoRoot)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	seedID := strings.TrimSpace(r.URL.Query().Get("seed_id"))
	kind := strings.TrimSpace(r.URL.Query().Get("kind"))
	if seedID == "" || kind == "" {
		respondError(w, http.StatusBadRequest, errors.New("seed_id and kind are required"))
		return
	}

	item, ok := registry.GetObject(catalog, seedID, kind)
	if !ok {
		respondError(w, http.StatusNotFound, errors.New("object not found"))
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"mode":      "catalog_projection",
		"object":    detailObject(item),
		"discovery": registryDiscoveryPaths(),
	})
}

func (api *RegistryAPI) handleSchemas(w http.ResponseWriter, r *http.Request) {
	catalog, err := registry.LoadCatalog(api.RepoRoot)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	seedID := strings.TrimSpace(r.URL.Query().Get("seed_id"))
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	items := registry.FilterSchemas(catalog.Schemas, seedID, query)

	respondJSON(w, http.StatusOK, map[string]any{
		"mode":    "catalog_projection",
		"filters": map[string]string{"seed_id": seedID, "q": query},
		"schemas": summarizeSchemas(items),
	})
}

func (api *RegistryAPI) handleSchema(w http.ResponseWriter, r *http.Request) {
	catalog, err := registry.LoadCatalog(api.RepoRoot)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref == "" {
		respondError(w, http.StatusBadRequest, errors.New("ref is required"))
		return
	}

	item, ok := registry.GetSchema(catalog, ref)
	if !ok {
		respondError(w, http.StatusNotFound, errors.New("schema not found"))
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"mode":      "catalog_projection",
		"schema":    detailSchema(item),
		"discovery": registryDiscoveryPaths(),
	})
}

type registryObjectSummary struct {
	SeedID           string   `json:"seed_id"`
	Kind             string   `json:"kind"`
	Summary          string   `json:"summary"`
	SchemaRefs       []string `json:"schema_refs"`
	Capabilities     []string `json:"capabilities"`
	RealizationCount int      `json:"realization_count"`
	CommandCount     int      `json:"command_count"`
	ProjectionCount  int      `json:"projection_count"`
	Self             string   `json:"self"`
}

type registrySchemaSummary struct {
	Ref                string `json:"ref"`
	Path               string `json:"path"`
	Anchor             string `json:"anchor,omitempty"`
	ObjectUseCount     int    `json:"object_use_count"`
	CommandInputCount  int    `json:"command_input_count"`
	CommandResultCount int    `json:"command_result_count"`
	Self               string `json:"self"`
}

func summarizeObjects(items []registry.CatalogObject) []registryObjectSummary {
	out := make([]registryObjectSummary, 0, len(items))
	for _, item := range items {
		out = append(out, registryObjectSummary{
			SeedID:           item.SeedID,
			Kind:             item.Kind,
			Summary:          item.Summary,
			SchemaRefs:       append([]string(nil), item.SchemaRefs...),
			Capabilities:     append([]string(nil), item.Capabilities...),
			RealizationCount: len(item.Realizations),
			CommandCount:     len(item.Commands),
			ProjectionCount:  len(item.Projections),
			Self:             objectSelfPath(item.SeedID, item.Kind),
		})
	}
	return out
}

func summarizeSchemas(items []registry.CatalogSchema) []registrySchemaSummary {
	out := make([]registrySchemaSummary, 0, len(items))
	for _, item := range items {
		out = append(out, registrySchemaSummary{
			Ref:                item.Ref,
			Path:               item.Path,
			Anchor:             item.Anchor,
			ObjectUseCount:     len(item.ObjectUses),
			CommandInputCount:  len(item.CommandInputs),
			CommandResultCount: len(item.CommandResults),
			Self:               schemaSelfPath(item.Ref),
		})
	}
	return out
}

func detailObject(item registry.CatalogObject) map[string]any {
	realizations := make([]map[string]any, 0, len(item.Realizations))
	for _, entry := range item.Realizations {
		realizations = append(realizations, map[string]any{
			"reference":      entry.Reference,
			"seed_id":        entry.SeedID,
			"realization_id": entry.RealizationID,
			"approach_id":    entry.ApproachID,
			"summary":        entry.Summary,
			"status":         entry.Status,
			"surface_kind":   entry.SurfaceKind,
			"contract_file":  entry.ContractFile,
			"schema_ref":     entry.SchemaRef,
			"capabilities":   entry.Capabilities,
			"contract":       "/v1/contracts/" + entry.SeedID + "/" + entry.RealizationID,
		})
	}

	commands := make([]map[string]any, 0, len(item.Commands))
	for _, entry := range item.Commands {
		commands = append(commands, map[string]any{
			"reference":         entry.Reference,
			"seed_id":           entry.SeedID,
			"realization_id":    entry.RealizationID,
			"name":              entry.Name,
			"summary":           entry.Summary,
			"path":              entry.Path,
			"auth_modes":        entry.AuthModes,
			"capabilities":      entry.Capabilities,
			"idempotency":       entry.Idempotency,
			"input_schema_ref":  entry.InputSchemaRef,
			"result_schema_ref": entry.ResultSchemaRef,
			"projection":        entry.Projection,
			"consistency":       entry.Consistency,
			"contract_file":     entry.ContractFile,
			"contract":          "/v1/contracts/" + entry.SeedID + "/" + entry.RealizationID,
		})
	}

	projections := make([]map[string]any, 0, len(item.Projections))
	for _, entry := range item.Projections {
		projections = append(projections, map[string]any{
			"reference":      entry.Reference,
			"seed_id":        entry.SeedID,
			"realization_id": entry.RealizationID,
			"name":           entry.Name,
			"summary":        entry.Summary,
			"path":           entry.Path,
			"capabilities":   entry.Capabilities,
			"freshness":      entry.Freshness,
			"contract_file":  entry.ContractFile,
			"contract":       "/v1/contracts/" + entry.SeedID + "/" + entry.RealizationID,
		})
	}

	schemaLinks := make([]map[string]string, 0, len(item.SchemaRefs))
	for _, ref := range item.SchemaRefs {
		schemaLinks = append(schemaLinks, map[string]string{
			"ref":  ref,
			"self": schemaSelfPath(ref),
		})
	}

	return map[string]any{
		"seed_id":      item.SeedID,
		"kind":         item.Kind,
		"summary":      item.Summary,
		"capabilities": item.Capabilities,
		"schema_refs":  item.SchemaRefs,
		"schemas":      schemaLinks,
		"realizations": realizations,
		"commands":     commands,
		"projections":  projections,
		"self":         objectSelfPath(item.SeedID, item.Kind),
	}
}

func detailSchema(item registry.CatalogSchema) map[string]any {
	objectUses := make([]map[string]any, 0, len(item.ObjectUses))
	for _, use := range item.ObjectUses {
		objectUses = append(objectUses, map[string]any{
			"reference":      use.Reference,
			"seed_id":        use.SeedID,
			"realization_id": use.RealizationID,
			"kind":           use.Kind,
			"summary":        use.Summary,
			"contract_file":  use.ContractFile,
			"object":         objectSelfPath(use.SeedID, use.Kind),
			"contract":       "/v1/contracts/" + use.SeedID + "/" + use.RealizationID,
		})
	}

	commandInputs := make([]map[string]any, 0, len(item.CommandInputs))
	for _, use := range item.CommandInputs {
		commandInputs = append(commandInputs, detailSchemaCommandUse(use))
	}

	commandResults := make([]map[string]any, 0, len(item.CommandResults))
	for _, use := range item.CommandResults {
		commandResults = append(commandResults, detailSchemaCommandUse(use))
	}

	return map[string]any{
		"ref":             item.Ref,
		"path":            item.Path,
		"anchor":          item.Anchor,
		"object_uses":     objectUses,
		"command_inputs":  commandInputs,
		"command_results": commandResults,
		"self":            schemaSelfPath(item.Ref),
	}
}

func detailSchemaCommandUse(use registry.CatalogSchemaCommandUse) map[string]any {
	return map[string]any{
		"reference":      use.Reference,
		"seed_id":        use.SeedID,
		"realization_id": use.RealizationID,
		"name":           use.Name,
		"summary":        use.Summary,
		"path":           use.Path,
		"contract_file":  use.ContractFile,
		"contract":       "/v1/contracts/" + use.SeedID + "/" + use.RealizationID,
	}
}

func registryDiscoveryPaths() map[string]string {
	return map[string]string{
		"catalog":   "/v1/registry/catalog",
		"objects":   "/v1/registry/objects",
		"object":    "/v1/registry/object?seed_id={seed_id}&kind={kind}",
		"schemas":   "/v1/registry/schemas",
		"schema":    "/v1/registry/schema?ref={ref}",
		"contracts": "/v1/contracts",
	}
}

func objectSelfPath(seedID, kind string) string {
	return "/v1/registry/object?seed_id=" + url.QueryEscape(seedID) + "&kind=" + url.QueryEscape(kind)
}

func schemaSelfPath(ref string) string {
	return "/v1/registry/schema?ref=" + url.QueryEscape(ref)
}
