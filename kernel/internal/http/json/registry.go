package jsontransport

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"as/kernel/internal/registry"
)

type RegistryAPI struct {
	Reader registry.CatalogReader
}

func NewRegistryAPI(repoRoot string) *RegistryAPI {
	return NewRegistryCatalogAPI(registry.NewCatalogReader(repoRoot))
}

func NewRegistryCatalogAPI(reader registry.CatalogReader) *RegistryAPI {
	return &RegistryAPI{Reader: reader}
}

func (api *RegistryAPI) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/registry/catalog", api.handleCatalog)
	mux.HandleFunc("GET /v1/registry/realizations", api.handleRealizations)
	mux.HandleFunc("GET /v1/registry/realization", api.handleRealization)
	mux.HandleFunc("GET /v1/registry/commands", api.handleCommands)
	mux.HandleFunc("GET /v1/registry/command", api.handleCommand)
	mux.HandleFunc("GET /v1/registry/projections", api.handleProjections)
	mux.HandleFunc("GET /v1/registry/projection", api.handleProjection)
	mux.HandleFunc("GET /v1/registry/objects", api.handleObjects)
	mux.HandleFunc("GET /v1/registry/object", api.handleObject)
	mux.HandleFunc("GET /v1/registry/schemas", api.handleSchemas)
	mux.HandleFunc("GET /v1/registry/schema", api.handleSchema)
}

func (api *RegistryAPI) handleCatalog(w http.ResponseWriter, r *http.Request) {
	catalog, err := api.Reader.Catalog()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"mode":         "catalog_projection",
		"summary":      catalog.Summary,
		"realizations": summarizeRealizations(catalog.Realizations),
		"commands":     summarizeCommands(catalog.Commands),
		"projections":  summarizeProjections(catalog.Projections),
		"objects":      summarizeObjects(catalog.Objects),
		"schemas":      summarizeSchemas(catalog.Schemas),
		"discovery":    registryDiscoveryPaths(),
	})
}

func (api *RegistryAPI) handleRealizations(w http.ResponseWriter, r *http.Request) {
	seedID := strings.TrimSpace(r.URL.Query().Get("seed_id"))
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	items, err := api.Reader.ListRealizations(seedID, query)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"mode":         "catalog_projection",
		"filters":      map[string]string{"seed_id": seedID, "q": query},
		"realizations": summarizeRealizations(items),
	})
}

func (api *RegistryAPI) handleRealization(w http.ResponseWriter, r *http.Request) {
	reference := strings.TrimSpace(r.URL.Query().Get("reference"))
	if reference == "" {
		respondError(w, http.StatusBadRequest, errors.New("reference is required"))
		return
	}

	item, err := api.Reader.GetRealization(reference)
	if err != nil {
		if errors.Is(err, registry.ErrCatalogRealizationNotFound) {
			respondError(w, http.StatusNotFound, errors.New("realization not found"))
			return
		}
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"mode":        "catalog_projection",
		"realization": detailRealization(item),
		"discovery":   registryDiscoveryPaths(),
	})
}

func (api *RegistryAPI) handleCommands(w http.ResponseWriter, r *http.Request) {
	seedID := strings.TrimSpace(r.URL.Query().Get("seed_id"))
	reference := strings.TrimSpace(r.URL.Query().Get("reference"))
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	items, err := api.Reader.ListCommands(seedID, reference, query)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"mode":     "catalog_projection",
		"filters":  map[string]string{"seed_id": seedID, "reference": reference, "q": query},
		"commands": summarizeCommands(items),
	})
}

func (api *RegistryAPI) handleCommand(w http.ResponseWriter, r *http.Request) {
	reference := strings.TrimSpace(r.URL.Query().Get("reference"))
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if reference == "" || name == "" {
		respondError(w, http.StatusBadRequest, errors.New("reference and name are required"))
		return
	}

	item, err := api.Reader.GetCommand(reference, name)
	if err != nil {
		if errors.Is(err, registry.ErrCatalogCommandNotFound) {
			respondError(w, http.StatusNotFound, errors.New("command not found"))
			return
		}
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"mode":      "catalog_projection",
		"command":   detailCommand(item),
		"discovery": registryDiscoveryPaths(),
	})
}

func (api *RegistryAPI) handleProjections(w http.ResponseWriter, r *http.Request) {
	seedID := strings.TrimSpace(r.URL.Query().Get("seed_id"))
	reference := strings.TrimSpace(r.URL.Query().Get("reference"))
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	items, err := api.Reader.ListProjections(seedID, reference, query)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"mode":        "catalog_projection",
		"filters":     map[string]string{"seed_id": seedID, "reference": reference, "q": query},
		"projections": summarizeProjections(items),
	})
}

func (api *RegistryAPI) handleProjection(w http.ResponseWriter, r *http.Request) {
	reference := strings.TrimSpace(r.URL.Query().Get("reference"))
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if reference == "" || name == "" {
		respondError(w, http.StatusBadRequest, errors.New("reference and name are required"))
		return
	}

	item, err := api.Reader.GetProjection(reference, name)
	if err != nil {
		if errors.Is(err, registry.ErrCatalogProjectionNotFound) {
			respondError(w, http.StatusNotFound, errors.New("projection not found"))
			return
		}
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"mode":       "catalog_projection",
		"projection": detailProjection(item),
		"discovery":  registryDiscoveryPaths(),
	})
}

func (api *RegistryAPI) handleObjects(w http.ResponseWriter, r *http.Request) {
	seedID := strings.TrimSpace(r.URL.Query().Get("seed_id"))
	schemaRef := strings.TrimSpace(r.URL.Query().Get("schema_ref"))
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	items, err := api.Reader.ListObjects(seedID, schemaRef, query)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"mode":    "catalog_projection",
		"filters": map[string]string{"seed_id": seedID, "schema_ref": schemaRef, "q": query},
		"objects": summarizeObjects(items),
	})
}

func (api *RegistryAPI) handleObject(w http.ResponseWriter, r *http.Request) {
	seedID := strings.TrimSpace(r.URL.Query().Get("seed_id"))
	kind := strings.TrimSpace(r.URL.Query().Get("kind"))
	if seedID == "" || kind == "" {
		respondError(w, http.StatusBadRequest, errors.New("seed_id and kind are required"))
		return
	}

	item, err := api.Reader.GetObject(seedID, kind)
	if err != nil {
		if errors.Is(err, registry.ErrCatalogObjectNotFound) {
			respondError(w, http.StatusNotFound, errors.New("object not found"))
			return
		}
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"mode":      "catalog_projection",
		"object":    detailObject(item),
		"discovery": registryDiscoveryPaths(),
	})
}

func (api *RegistryAPI) handleSchemas(w http.ResponseWriter, r *http.Request) {
	seedID := strings.TrimSpace(r.URL.Query().Get("seed_id"))
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	items, err := api.Reader.ListSchemas(seedID, query)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"mode":    "catalog_projection",
		"filters": map[string]string{"seed_id": seedID, "q": query},
		"schemas": summarizeSchemas(items),
	})
}

func (api *RegistryAPI) handleSchema(w http.ResponseWriter, r *http.Request) {
	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref == "" {
		respondError(w, http.StatusBadRequest, errors.New("ref is required"))
		return
	}

	item, err := api.Reader.GetSchema(ref)
	if err != nil {
		if errors.Is(err, registry.ErrCatalogSchemaNotFound) {
			respondError(w, http.StatusNotFound, errors.New("schema not found"))
			return
		}
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"mode":      "catalog_projection",
		"schema":    detailSchema(item),
		"discovery": registryDiscoveryPaths(),
	})
}

type registryRealizationSummary struct {
	Reference       string   `json:"reference"`
	SeedID          string   `json:"seed_id"`
	RealizationID   string   `json:"realization_id"`
	ApproachID      string   `json:"approach_id,omitempty"`
	Summary         string   `json:"summary"`
	Status          string   `json:"status"`
	SurfaceKind     string   `json:"surface_kind"`
	ObjectKinds     []string `json:"object_kinds"`
	CommandCount    int      `json:"command_count"`
	ProjectionCount int      `json:"projection_count"`
	Self            string   `json:"self"`
	CanonicalURL    string   `json:"canonical_url"`
	PermalinkURL    string   `json:"permalink_url"`
	ContentHash     string   `json:"content_hash"`
}

type registryCommandSummary struct {
	Reference       string `json:"reference"`
	SeedID          string `json:"seed_id"`
	RealizationID   string `json:"realization_id"`
	Name            string `json:"name"`
	Path            string `json:"path"`
	InputSchemaRef  string `json:"input_schema_ref"`
	ResultSchemaRef string `json:"result_schema_ref"`
	Self            string `json:"self"`
	CanonicalURL    string `json:"canonical_url"`
	PermalinkURL    string `json:"permalink_url"`
	ContentHash     string `json:"content_hash"`
}

type registryProjectionSummary struct {
	Reference     string `json:"reference"`
	SeedID        string `json:"seed_id"`
	RealizationID string `json:"realization_id"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	Freshness     string `json:"freshness"`
	Self          string `json:"self"`
	CanonicalURL  string `json:"canonical_url"`
	PermalinkURL  string `json:"permalink_url"`
	ContentHash   string `json:"content_hash"`
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
	CanonicalURL     string   `json:"canonical_url"`
	PermalinkURL     string   `json:"permalink_url"`
	ContentHash      string   `json:"content_hash"`
}

type registrySchemaSummary struct {
	Ref                string `json:"ref"`
	Path               string `json:"path"`
	Anchor             string `json:"anchor,omitempty"`
	ObjectUseCount     int    `json:"object_use_count"`
	CommandInputCount  int    `json:"command_input_count"`
	CommandResultCount int    `json:"command_result_count"`
	Self               string `json:"self"`
	CanonicalURL       string `json:"canonical_url"`
	PermalinkURL       string `json:"permalink_url"`
	ContentHash        string `json:"content_hash"`
}

func summarizeRealizations(items []registry.CatalogRealization) []registryRealizationSummary {
	out := make([]registryRealizationSummary, 0, len(items))
	for _, item := range items {
		locator := realizationLocator(item)
		out = append(out, registryRealizationSummary{
			Reference:       item.Reference,
			SeedID:          item.SeedID,
			RealizationID:   item.RealizationID,
			ApproachID:      item.ApproachID,
			Summary:         item.Summary,
			Status:          item.Status,
			SurfaceKind:     item.SurfaceKind,
			ObjectKinds:     append([]string(nil), item.ObjectKinds...),
			CommandCount:    len(item.CommandNames),
			ProjectionCount: len(item.Projections),
			Self:            realizationSelfPath(item.Reference),
			CanonicalURL:    locator.CanonicalURL,
			PermalinkURL:    locator.PermalinkURL,
			ContentHash:     locator.ContentHash,
		})
	}
	return out
}

func summarizeCommands(items []registry.CatalogCommand) []registryCommandSummary {
	out := make([]registryCommandSummary, 0, len(items))
	for _, item := range items {
		locator := commandLocator(item)
		out = append(out, registryCommandSummary{
			Reference:       item.Reference,
			SeedID:          item.SeedID,
			RealizationID:   item.RealizationID,
			Name:            item.Name,
			Path:            item.Path,
			InputSchemaRef:  item.InputSchemaRef,
			ResultSchemaRef: item.ResultSchemaRef,
			Self:            commandSelfPath(item.Reference, item.Name),
			CanonicalURL:    locator.CanonicalURL,
			PermalinkURL:    locator.PermalinkURL,
			ContentHash:     locator.ContentHash,
		})
	}
	return out
}

func summarizeProjections(items []registry.CatalogProjection) []registryProjectionSummary {
	out := make([]registryProjectionSummary, 0, len(items))
	for _, item := range items {
		locator := projectionLocator(item)
		out = append(out, registryProjectionSummary{
			Reference:     item.Reference,
			SeedID:        item.SeedID,
			RealizationID: item.RealizationID,
			Name:          item.Name,
			Path:          item.Path,
			Freshness:     item.Freshness,
			Self:          projectionSelfPath(item.Reference, item.Name),
			CanonicalURL:  locator.CanonicalURL,
			PermalinkURL:  locator.PermalinkURL,
			ContentHash:   locator.ContentHash,
		})
	}
	return out
}

func summarizeObjects(items []registry.CatalogObject) []registryObjectSummary {
	out := make([]registryObjectSummary, 0, len(items))
	for _, item := range items {
		locator := objectLocator(item)
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
			CanonicalURL:     locator.CanonicalURL,
			PermalinkURL:     locator.PermalinkURL,
			ContentHash:      locator.ContentHash,
		})
	}
	return out
}

func summarizeSchemas(items []registry.CatalogSchema) []registrySchemaSummary {
	out := make([]registrySchemaSummary, 0, len(items))
	for _, item := range items {
		locator := schemaLocator(item)
		out = append(out, registrySchemaSummary{
			Ref:                item.Ref,
			Path:               item.Path,
			Anchor:             item.Anchor,
			ObjectUseCount:     len(item.ObjectUses),
			CommandInputCount:  len(item.CommandInputs),
			CommandResultCount: len(item.CommandResults),
			Self:               schemaSelfPath(item.Ref),
			CanonicalURL:       locator.CanonicalURL,
			PermalinkURL:       locator.PermalinkURL,
			ContentHash:        locator.ContentHash,
		})
	}
	return out
}

func detailRealization(item registry.CatalogRealization) map[string]any {
	locator := realizationLocator(item)
	objectLinks := make([]map[string]string, 0, len(item.ObjectKinds))
	for _, kind := range item.ObjectKinds {
		objectLinks = append(objectLinks, map[string]string{
			"kind": kind,
			"self": objectSelfPath(item.SeedID, kind),
		})
	}

	commandLinks := make([]map[string]string, 0, len(item.CommandNames))
	for _, name := range item.CommandNames {
		commandLinks = append(commandLinks, map[string]string{
			"name": name,
			"self": commandSelfPath(item.Reference, name),
		})
	}

	projectionLinks := make([]map[string]string, 0, len(item.Projections))
	for _, name := range item.Projections {
		projectionLinks = append(projectionLinks, map[string]string{
			"name": name,
			"self": projectionSelfPath(item.Reference, name),
		})
	}

	return map[string]any{
		"reference":      item.Reference,
		"seed_id":        item.SeedID,
		"realization_id": item.RealizationID,
		"approach_id":    item.ApproachID,
		"summary":        item.Summary,
		"status":         item.Status,
		"surface_kind":   item.SurfaceKind,
		"contract_file":  item.ContractFile,
		"auth_modes":     item.AuthModes,
		"capabilities":   item.Capabilities,
		"object_kinds":   item.ObjectKinds,
		"objects":        objectLinks,
		"commands":       commandLinks,
		"projections":    projectionLinks,
		"contract":       "/v1/contracts/" + item.SeedID + "/" + item.RealizationID,
		"self":           realizationSelfPath(item.Reference),
		"canonical_url":  locator.CanonicalURL,
		"permalink_url":  locator.PermalinkURL,
		"content_hash":   locator.ContentHash,
	}
}

func detailCommand(item registry.CatalogCommand) map[string]any {
	locator := commandLocator(item)
	inputSchema := ""
	if item.InputSchemaRef != "" {
		inputSchema = schemaSelfPath(item.InputSchemaRef)
	}
	resultSchema := ""
	if item.ResultSchemaRef != "" {
		resultSchema = schemaSelfPath(item.ResultSchemaRef)
	}
	projection := ""
	if item.Projection != "" {
		projection = projectionSelfPath(item.Reference, item.Projection)
	}

	return map[string]any{
		"reference":         item.Reference,
		"seed_id":           item.SeedID,
		"realization_id":    item.RealizationID,
		"name":              item.Name,
		"summary":           item.Summary,
		"path":              item.Path,
		"auth_modes":        item.AuthModes,
		"capabilities":      item.Capabilities,
		"idempotency":       item.Idempotency,
		"input_schema_ref":  item.InputSchemaRef,
		"result_schema_ref": item.ResultSchemaRef,
		"input_schema":      inputSchema,
		"result_schema":     resultSchema,
		"projection":        item.Projection,
		"projection_self":   projection,
		"consistency":       item.Consistency,
		"contract_file":     item.ContractFile,
		"contract":          "/v1/contracts/" + item.SeedID + "/" + item.RealizationID,
		"self":              commandSelfPath(item.Reference, item.Name),
		"canonical_url":     locator.CanonicalURL,
		"permalink_url":     locator.PermalinkURL,
		"content_hash":      locator.ContentHash,
	}
}

func detailProjection(item registry.CatalogProjection) map[string]any {
	locator := projectionLocator(item)
	return map[string]any{
		"reference":      item.Reference,
		"seed_id":        item.SeedID,
		"realization_id": item.RealizationID,
		"name":           item.Name,
		"summary":        item.Summary,
		"path":           item.Path,
		"capabilities":   item.Capabilities,
		"freshness":      item.Freshness,
		"contract_file":  item.ContractFile,
		"contract":       "/v1/contracts/" + item.SeedID + "/" + item.RealizationID,
		"self":           projectionSelfPath(item.Reference, item.Name),
		"canonical_url":  locator.CanonicalURL,
		"permalink_url":  locator.PermalinkURL,
		"content_hash":   locator.ContentHash,
	}
}

func detailObject(item registry.CatalogObject) map[string]any {
	locator := objectLocator(item)
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
			"realization":    realizationSelfPath(entry.Reference),
		})
	}

	commands := make([]map[string]any, 0, len(item.Commands))
	for _, entry := range item.Commands {
		command := detailCommand(entry)
		commands = append(commands, command)
	}

	projections := make([]map[string]any, 0, len(item.Projections))
	for _, entry := range item.Projections {
		projection := detailProjection(entry)
		projections = append(projections, projection)
	}

	schemaLinks := make([]map[string]string, 0, len(item.SchemaRefs))
	for _, ref := range item.SchemaRefs {
		schemaLinks = append(schemaLinks, map[string]string{
			"ref":  ref,
			"self": schemaSelfPath(ref),
		})
	}

	return map[string]any{
		"seed_id":       item.SeedID,
		"kind":          item.Kind,
		"summary":       item.Summary,
		"capabilities":  item.Capabilities,
		"schema_refs":   item.SchemaRefs,
		"schemas":       schemaLinks,
		"realizations":  realizations,
		"commands":      commands,
		"projections":   projections,
		"self":          objectSelfPath(item.SeedID, item.Kind),
		"canonical_url": locator.CanonicalURL,
		"permalink_url": locator.PermalinkURL,
		"content_hash":  locator.ContentHash,
	}
}

func detailSchema(item registry.CatalogSchema) map[string]any {
	locator := schemaLocator(item)
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
			"realization":    realizationSelfPath(use.Reference),
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
		"canonical_url":   locator.CanonicalURL,
		"permalink_url":   locator.PermalinkURL,
		"content_hash":    locator.ContentHash,
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
		"command":        commandSelfPath(use.Reference, use.Name),
		"realization":    realizationSelfPath(use.Reference),
		"contract":       "/v1/contracts/" + use.SeedID + "/" + use.RealizationID,
	}
}

type registryResourceLocator struct {
	CanonicalURL string
	PermalinkURL string
	ContentHash  string
}

func realizationLocator(item registry.CatalogRealization) registryResourceLocator {
	return resourceLocator("realization", browseRealizationPath(item.Reference), item)
}

func commandLocator(item registry.CatalogCommand) registryResourceLocator {
	return resourceLocator("command", browseCommandPath(item.Reference, item.Name), item)
}

func projectionLocator(item registry.CatalogProjection) registryResourceLocator {
	return resourceLocator("projection", browseProjectionPath(item.Reference, item.Name), item)
}

func objectLocator(item registry.CatalogObject) registryResourceLocator {
	return resourceLocator("object", browseObjectPath(item.SeedID, item.Kind), item)
}

func schemaLocator(item registry.CatalogSchema) registryResourceLocator {
	return resourceLocator("schema", browseSchemaPath(item.Ref), item)
}

func resourceLocator(kind, canonicalURL string, payload any) registryResourceLocator {
	contentHash := resourceContentHash(kind, payload)
	return registryResourceLocator{
		CanonicalURL: canonicalURL,
		PermalinkURL: permalinkBrowsePath(canonicalURL, contentHash),
		ContentHash:  contentHash,
	}
}

func resourceContentHash(kind string, payload any) string {
	raw, err := json.Marshal(struct {
		Kind    string `json:"kind"`
		Payload any    `json:"payload"`
	}{
		Kind:    strings.TrimSpace(kind),
		Payload: payload,
	})
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func registryDiscoveryPaths() map[string]string {
	return map[string]string{
		"catalog":      "/v1/registry/catalog",
		"realizations": "/v1/registry/realizations",
		"realization":  "/v1/registry/realization?reference={reference}",
		"commands":     "/v1/registry/commands",
		"command":      "/v1/registry/command?reference={reference}&name={name}",
		"projections":  "/v1/registry/projections",
		"projection":   "/v1/registry/projection?reference={reference}&name={name}",
		"objects":      "/v1/registry/objects",
		"object":       "/v1/registry/object?seed_id={seed_id}&kind={kind}",
		"schemas":      "/v1/registry/schemas",
		"schema":       "/v1/registry/schema?ref={ref}",
		"contracts":    "/v1/contracts",
	}
}

func realizationSelfPath(reference string) string {
	return "/v1/registry/realization?reference=" + url.QueryEscape(reference)
}

func browseRealizationPath(reference string) string {
	seedID, realizationID, ok := splitBrowseReference(reference)
	if !ok {
		return "/contracts/" + url.PathEscape(reference)
	}
	return "/contracts/" + url.PathEscape(seedID) + "/" + url.PathEscape(realizationID)
}

func commandSelfPath(reference, name string) string {
	return "/v1/registry/command?reference=" + url.QueryEscape(reference) + "&name=" + url.QueryEscape(name)
}

func browseCommandPath(reference, name string) string {
	seedID, realizationID, ok := splitBrowseReference(reference)
	if !ok {
		return "/actions/" + url.PathEscape(reference) + "/" + url.PathEscape(name)
	}
	return "/actions/" + url.PathEscape(seedID) + "/" + url.PathEscape(realizationID) + "/" + url.PathEscape(name)
}

func projectionSelfPath(reference, name string) string {
	return "/v1/registry/projection?reference=" + url.QueryEscape(reference) + "&name=" + url.QueryEscape(name)
}

func browseProjectionPath(reference, name string) string {
	seedID, realizationID, ok := splitBrowseReference(reference)
	if !ok {
		return "/read-models/" + url.PathEscape(reference) + "/" + url.PathEscape(name)
	}
	return "/read-models/" + url.PathEscape(seedID) + "/" + url.PathEscape(realizationID) + "/" + url.PathEscape(name)
}

func objectSelfPath(seedID, kind string) string {
	return "/v1/registry/object?seed_id=" + url.QueryEscape(seedID) + "&kind=" + url.QueryEscape(kind)
}

func browseObjectPath(seedID, kind string) string {
	return "/objects/" + url.PathEscape(seedID) + "/" + url.PathEscape(kind)
}

func schemaSelfPath(ref string) string {
	return "/v1/registry/schema?ref=" + url.QueryEscape(ref)
}

func browseSchemaPath(ref string) string {
	return "/schemas/detail?ref=" + url.QueryEscape(ref)
}

func permalinkBrowsePath(canonicalURL, contentHash string) string {
	if strings.TrimSpace(canonicalURL) == "" || strings.TrimSpace(contentHash) == "" {
		return ""
	}
	return "/@sha256-" + contentHash + canonicalURL
}

func splitBrowseReference(reference string) (string, string, bool) {
	reference = strings.Trim(strings.TrimSpace(reference), "/")
	if reference == "" {
		return "", "", false
	}
	parts := strings.Split(reference, "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), true
}
