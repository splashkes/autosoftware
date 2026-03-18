package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed assets/style.css
var styleCSS []byte

// --- Registry API response types ---

type CatalogResponse struct {
	Summary      CatalogSummary       `json:"summary"`
	Realizations []RealizationSummary `json:"realizations"`
	Commands     []CommandSummary     `json:"commands"`
	Projections  []ProjectionSummary  `json:"projections"`
	Objects      []ObjectSummary      `json:"objects"`
	Schemas      []SchemaSummary      `json:"schemas"`
	Discovery    map[string]string    `json:"discovery"`
}

type CatalogSummary struct {
	Realizations int `json:"realizations"`
	Contracts    int `json:"contracts"`
	Objects      int `json:"objects"`
	Schemas      int `json:"schemas"`
	Commands     int `json:"commands"`
	Projections  int `json:"projections"`
}

type RealizationSummary struct {
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
	SchemaCount     int      `json:"schema_count,omitempty"`
	Self            string   `json:"self"`
}

type RealizationDetail struct {
	Reference     string          `json:"reference"`
	SeedID        string          `json:"seed_id"`
	RealizationID string          `json:"realization_id"`
	ApproachID    string          `json:"approach_id,omitempty"`
	Summary       string          `json:"summary"`
	Status        string          `json:"status"`
	SurfaceKind   string          `json:"surface_kind"`
	ContractFile  string          `json:"contract_file"`
	AuthModes     []string        `json:"auth_modes"`
	Capabilities  []string        `json:"capabilities"`
	ObjectKinds   []string        `json:"object_kinds"`
	Objects       []ResourceLink  `json:"objects"`
	Relations     []GraphRelation `json:"relations,omitempty"`
	Commands      []ResourceLink  `json:"commands"`
	Projections   []ResourceLink  `json:"projections"`
	Contract      string          `json:"contract"`
	Self          string          `json:"self"`
	CanonicalURL  string          `json:"canonical_url"`
	PermalinkURL  string          `json:"permalink_url"`
	ContentHash   string          `json:"content_hash"`
}

type CommandSummary struct {
	Reference       string `json:"reference"`
	SeedID          string `json:"seed_id"`
	RealizationID   string `json:"realization_id"`
	Name            string `json:"name"`
	Summary         string `json:"summary,omitempty"`
	Path            string `json:"path"`
	InputSchemaRef  string `json:"input_schema_ref"`
	ResultSchemaRef string `json:"result_schema_ref"`
	Projection      string `json:"projection,omitempty"`
	Self            string `json:"self"`
}

type CommandDetail struct {
	Reference       string   `json:"reference"`
	SeedID          string   `json:"seed_id"`
	RealizationID   string   `json:"realization_id"`
	Name            string   `json:"name"`
	Summary         string   `json:"summary"`
	Path            string   `json:"path"`
	AuthModes       []string `json:"auth_modes"`
	Capabilities    []string `json:"capabilities"`
	Idempotency     string   `json:"idempotency"`
	InputSchemaRef  string   `json:"input_schema_ref"`
	ResultSchemaRef string   `json:"result_schema_ref"`
	InputSchema     string   `json:"input_schema"`
	ResultSchema    string   `json:"result_schema"`
	Projection      string   `json:"projection"`
	ProjectionSelf  string   `json:"projection_self"`
	Consistency     string   `json:"consistency"`
	ContractFile    string   `json:"contract_file"`
	Contract        string   `json:"contract"`
	Self            string   `json:"self"`
	CanonicalURL    string   `json:"canonical_url"`
	PermalinkURL    string   `json:"permalink_url"`
	ContentHash     string   `json:"content_hash"`
}

type ProjectionSummary struct {
	Reference     string   `json:"reference"`
	SeedID        string   `json:"seed_id"`
	RealizationID string   `json:"realization_id"`
	Name          string   `json:"name"`
	Summary       string   `json:"summary,omitempty"`
	Path          string   `json:"path"`
	AuthModes     []string `json:"auth_modes"`
	Freshness     string   `json:"freshness"`
	Self          string   `json:"self"`
}

type ProjectionDetail struct {
	Reference     string     `json:"reference"`
	SeedID        string     `json:"seed_id"`
	RealizationID string     `json:"realization_id"`
	Name          string     `json:"name"`
	Summary       string     `json:"summary"`
	Path          string     `json:"path"`
	AuthModes     []string   `json:"auth_modes"`
	Capabilities  []string   `json:"capabilities"`
	Freshness     string     `json:"freshness"`
	DataViews     []DataView `json:"data_views,omitempty"`
	ContractFile  string     `json:"contract_file"`
	Contract      string     `json:"contract"`
	Self          string     `json:"self"`
	CanonicalURL  string     `json:"canonical_url"`
	PermalinkURL  string     `json:"permalink_url"`
	ContentHash   string     `json:"content_hash"`
}

type ObjectSummary struct {
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

type ObjectDetail struct {
	SeedID            string              `json:"seed_id"`
	Kind              string              `json:"kind"`
	Summary           string              `json:"summary"`
	Capabilities      []string            `json:"capabilities"`
	SchemaRefs        []string            `json:"schema_refs"`
	DataLayout        DataLayout          `json:"data_layout,omitempty"`
	OutgoingRelations []GraphRelation     `json:"outgoing_relations,omitempty"`
	IncomingRelations []GraphRelation     `json:"incoming_relations,omitempty"`
	Schemas           []ResourceLink      `json:"schemas"`
	Realizations      []ObjectRealization `json:"realizations"`
	Commands          []CommandDetail     `json:"commands"`
	Projections       []ProjectionDetail  `json:"projections"`
	Self              string              `json:"self"`
	CanonicalURL      string              `json:"canonical_url"`
	PermalinkURL      string              `json:"permalink_url"`
	ContentHash       string              `json:"content_hash"`
}

type ObjectRealization struct {
	Reference     string   `json:"reference"`
	SeedID        string   `json:"seed_id"`
	RealizationID string   `json:"realization_id"`
	ApproachID    string   `json:"approach_id,omitempty"`
	Summary       string   `json:"summary"`
	Status        string   `json:"status"`
	SurfaceKind   string   `json:"surface_kind"`
	ContractFile  string   `json:"contract_file"`
	SchemaRef     string   `json:"schema_ref"`
	Capabilities  []string `json:"capabilities"`
	Contract      string   `json:"contract"`
	Realization   string   `json:"realization"`
}

type DataLayout struct {
	SharedMetadata DataSection `json:"shared_metadata,omitempty"`
	PublicPayload  DataSection `json:"public_payload,omitempty"`
	PrivatePayload DataSection `json:"private_payload,omitempty"`
	RuntimeOnly    DataSection `json:"runtime_only,omitempty"`
}

type DataSection struct {
	Summary string      `json:"summary,omitempty"`
	Fields  []DataField `json:"fields,omitempty"`
}

type DataField struct {
	Name    string `json:"name"`
	Type    string `json:"type,omitempty"`
	Summary string `json:"summary,omitempty"`
}

type DataView struct {
	AuthModes []string `json:"auth_modes"`
	Sections  []string `json:"sections"`
	Summary   string   `json:"summary,omitempty"`
}

type GraphRelation struct {
	Reference     string         `json:"reference"`
	SeedID        string         `json:"seed_id"`
	RealizationID string         `json:"realization_id"`
	Kind          string         `json:"kind"`
	Summary       string         `json:"summary"`
	FromKinds     []string       `json:"from_kinds"`
	ToKinds       []string       `json:"to_kinds"`
	Cardinality   string         `json:"cardinality"`
	Visibility    string         `json:"visibility"`
	SchemaRef     string         `json:"schema_ref"`
	Schema        string         `json:"schema,omitempty"`
	Capabilities  []string       `json:"capabilities"`
	Attributes    []DataField    `json:"attributes,omitempty"`
	FromObjects   []ResourceLink `json:"from_objects"`
	ToObjects     []ResourceLink `json:"to_objects"`
	Contract      string         `json:"contract"`
}

type SchemaSummary struct {
	Ref                string `json:"ref"`
	Path               string `json:"path"`
	Anchor             string `json:"anchor,omitempty"`
	ObjectUseCount     int    `json:"object_use_count"`
	CommandInputCount  int    `json:"command_input_count"`
	CommandResultCount int    `json:"command_result_count"`
	Self               string `json:"self"`
}

type SchemaDetail struct {
	Ref            string             `json:"ref"`
	Path           string             `json:"path"`
	Anchor         string             `json:"anchor,omitempty"`
	ObjectUses     []SchemaObjectUse  `json:"object_uses"`
	CommandInputs  []SchemaCommandUse `json:"command_inputs"`
	CommandResults []SchemaCommandUse `json:"command_results"`
	Self           string             `json:"self"`
	CanonicalURL   string             `json:"canonical_url"`
	PermalinkURL   string             `json:"permalink_url"`
	ContentHash    string             `json:"content_hash"`
}

type HashLookupDetail struct {
	ContentHash  string `json:"content_hash"`
	ResourceKind string `json:"resource_kind"`
	CanonicalURL string `json:"canonical_url"`
	PermalinkURL string `json:"permalink_url"`
	RedirectURL  string `json:"redirect_url"`
}

type SchemaObjectUse struct {
	Reference     string `json:"reference"`
	SeedID        string `json:"seed_id"`
	RealizationID string `json:"realization_id"`
	Kind          string `json:"kind"`
	Summary       string `json:"summary"`
	ContractFile  string `json:"contract_file"`
	Object        string `json:"object"`
	Realization   string `json:"realization"`
	Contract      string `json:"contract"`
}

type SchemaCommandUse struct {
	Reference     string `json:"reference"`
	SeedID        string `json:"seed_id"`
	RealizationID string `json:"realization_id"`
	Name          string `json:"name"`
	Summary       string `json:"summary"`
	Path          string `json:"path"`
	ContractFile  string `json:"contract_file"`
	Command       string `json:"command"`
	Realization   string `json:"realization"`
	Contract      string `json:"contract"`
}

type ResourceLink struct {
	Name string `json:"name,omitempty"`
	Kind string `json:"kind,omitempty"`
	Ref  string `json:"ref,omitempty"`
	Self string `json:"self"`
}

type SystemView struct {
	SeedID             string
	Title              string
	Summary            string
	Statuses           []string
	SurfaceKinds       []string
	RealizationCount   int
	ObjectCount        int
	ActionCount        int
	ReadModelCount     int
	SchemaCount        int
	IsRegistryInternal bool
	Realizations       []RealizationSummary
	Objects            []ObjectSummary
	Commands           []CommandSummary
	Projections        []ProjectionSummary
	Schemas            []SchemaSummary
}

type ObjectRelationBrowseContext struct {
	SeedID          string
	Kind            string
	Direction       string
	DirectionLabel  string
	RelationKind    string
	MatchedKinds    []string
	MatchedCount    int
	ClearFiltersURL template.URL
}

// --- Registry client ---

type RegistryClient struct {
	baseURL string
	http    *http.Client
}

func NewRegistryClient(baseURL string) *RegistryClient {
	return &RegistryClient{
		baseURL: normalizeRegistryBaseURL(baseURL),
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

func normalizeRegistryBaseURL(baseURL string) string {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.TrimRight(trimmed, "/")
	}
	for _, suffix := range []string{"/v1/registry/catalog", "/v1/registry"} {
		if strings.HasSuffix(parsed.Path, suffix) {
			parsed.Path = strings.TrimSuffix(parsed.Path, suffix)
			break
		}
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/")
}

func (c *RegistryClient) get(path string, out any) error {
	resp, err := c.http.Get(c.baseURL + path)
	if err != nil {
		return fmt.Errorf("registry request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("registry returned %d for %s", resp.StatusCode, path)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *RegistryClient) Catalog() (*CatalogResponse, error) {
	var resp CatalogResponse
	if err := c.get("/v1/registry/catalog", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RegistryClient) Realizations(seedID, q string) ([]RealizationSummary, error) {
	params := buildQuery("seed_id", seedID, "q", q)
	var resp struct {
		Realizations []RealizationSummary `json:"realizations"`
	}
	if err := c.get("/v1/registry/realizations"+params, &resp); err != nil {
		return nil, err
	}
	return resp.Realizations, nil
}

func (c *RegistryClient) Realization(reference string) (*RealizationDetail, error) {
	var resp struct {
		Realization RealizationDetail `json:"realization"`
	}
	if err := c.get("/v1/registry/realization?reference="+url.QueryEscape(reference), &resp); err != nil {
		return nil, err
	}
	return &resp.Realization, nil
}

func (c *RegistryClient) Commands(seedID, reference, q string) ([]CommandSummary, error) {
	params := buildQuery("seed_id", seedID, "reference", reference, "q", q)
	var resp struct {
		Commands []CommandSummary `json:"commands"`
	}
	if err := c.get("/v1/registry/commands"+params, &resp); err != nil {
		return nil, err
	}
	return resp.Commands, nil
}

func (c *RegistryClient) Command(reference, name string) (*CommandDetail, error) {
	var resp struct {
		Command CommandDetail `json:"command"`
	}
	path := "/v1/registry/command?reference=" + url.QueryEscape(reference) + "&name=" + url.QueryEscape(name)
	if err := c.get(path, &resp); err != nil {
		return nil, err
	}
	return &resp.Command, nil
}

func (c *RegistryClient) Projections(seedID, reference, q string) ([]ProjectionSummary, error) {
	params := buildQuery("seed_id", seedID, "reference", reference, "q", q)
	var resp struct {
		Projections []ProjectionSummary `json:"projections"`
	}
	if err := c.get("/v1/registry/projections"+params, &resp); err != nil {
		return nil, err
	}
	return resp.Projections, nil
}

func (c *RegistryClient) Projection(reference, name string) (*ProjectionDetail, error) {
	var resp struct {
		Projection ProjectionDetail `json:"projection"`
	}
	path := "/v1/registry/projection?reference=" + url.QueryEscape(reference) + "&name=" + url.QueryEscape(name)
	if err := c.get(path, &resp); err != nil {
		return nil, err
	}
	return &resp.Projection, nil
}

func (c *RegistryClient) Objects(seedID, schemaRef, q string) ([]ObjectSummary, error) {
	params := buildQuery("seed_id", seedID, "schema_ref", schemaRef, "q", q)
	var resp struct {
		Objects []ObjectSummary `json:"objects"`
	}
	if err := c.get("/v1/registry/objects"+params, &resp); err != nil {
		return nil, err
	}
	return resp.Objects, nil
}

func (c *RegistryClient) Object(seedID, kind string) (*ObjectDetail, error) {
	var resp struct {
		Object ObjectDetail `json:"object"`
	}
	path := "/v1/registry/object?seed_id=" + url.QueryEscape(seedID) + "&kind=" + url.QueryEscape(kind)
	if err := c.get(path, &resp); err != nil {
		return nil, err
	}
	return &resp.Object, nil
}

func (c *RegistryClient) Schemas(seedID, q string) ([]SchemaSummary, error) {
	params := buildQuery("seed_id", seedID, "q", q)
	var resp struct {
		Schemas []SchemaSummary `json:"schemas"`
	}
	if err := c.get("/v1/registry/schemas"+params, &resp); err != nil {
		return nil, err
	}
	return resp.Schemas, nil
}

func (c *RegistryClient) Schema(ref string) (*SchemaDetail, error) {
	var resp struct {
		Schema SchemaDetail `json:"schema"`
	}
	if err := c.get("/v1/registry/schema?ref="+url.QueryEscape(ref), &resp); err != nil {
		return nil, err
	}
	return &resp.Schema, nil
}

func (c *RegistryClient) Lookup(contentHash string) (*HashLookupDetail, error) {
	var resp struct {
		Lookup HashLookupDetail `json:"lookup"`
	}
	if err := c.get("/v1/registry/lookup?sha256="+url.QueryEscape(contentHash), &resp); err != nil {
		return nil, err
	}
	return &resp.Lookup, nil
}

func buildQuery(pairs ...string) string {
	v := url.Values{}
	for i := 0; i+1 < len(pairs); i += 2 {
		if pairs[i+1] != "" {
			v.Set(pairs[i], pairs[i+1])
		}
	}
	if len(v) == 0 {
		return ""
	}
	return "?" + v.Encode()
}

// --- App ---

type App struct {
	registry *RegistryClient
	tmpl     map[string]*template.Template
}

const (
	repoBlobBaseURL       = "https://github.com/splashkes/autosoftware/blob/main/"
	publicRegistryBaseURL = "https://registry.autosoftware.app"
	publicRegistryHost    = "registry.autosoftware.app"
	publicAPIBaseURL      = "https://registry.autosoftware.app"
	legacyRegistryPathRoot = "/registry/reading-room"
	legacyRegistryPathPrefix = legacyRegistryPathRoot + "/"
)

func (app *App) loadTemplates() {
	funcMap := template.FuncMap{
		"join": strings.Join,
		"pathEscape": func(s string) string {
			return url.PathEscape(s)
		},
		"systemPath":         browseSystemPath,
		"objectPath":         browseObjectPath,
		"objectsPath":        browseObjectsPath,
		"relatedObjectsPath": browseRelatedObjectsPath,
		"realizationPath":    browseRealizationPath,
		"commandPath":        browseCommandPath,
		"projectionPath":     browseProjectionPath,
		"schemaPath":         browseSchemaPath,
		"hrefURL":            trustedURL,
		"apiURL":             trustedAPIURL,
		"repoSourceURL":      repoSourceURL,
		"queryEscape": func(s string) string {
			return url.QueryEscape(s)
		},
		"plural": func(n int, singular, plural string) string {
			if n == 1 {
				return singular
			}
			return plural
		},
		"add": func(a, b int) int {
			return a + b
		},
		"layoutSectionTitle": layoutSectionTitle,
		"layoutShape":        layoutShape,
		"hasDataLayout":      hasDataLayout,
	}

	pages := []string{
		"home",
		"systems", "system_detail", "registry_internals",
		"realizations", "realization_detail",
		"commands", "command_detail",
		"projections", "projection_detail",
		"objects", "object_detail",
		"schemas", "schema_detail",
	}

	app.tmpl = make(map[string]*template.Template, len(pages))
	for _, p := range pages {
		t := template.Must(
			template.New("").Funcs(funcMap).ParseFS(templateFS,
				"templates/base.html",
				"templates/"+p+".html",
			),
		)
		app.tmpl[p] = t
	}
}

func (app *App) render(w http.ResponseWriter, page string, data map[string]any) {
	t, ok := app.tmpl[page]
	if !ok {
		http.Error(w, "template not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("template error: %v", err)
	}
}

func (app *App) renderError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<!doctype html><html><head><title>Error</title>
<link rel="stylesheet" href="/assets/style.css"></head>
<body><div class="container"><h1>%d</h1><p>%s</p><p><a href="%s">Back to home</a></p></div></body></html>`, status, template.HTMLEscapeString(msg), canonicalRegistryURL("/"))
}

type permalinkContextKey string

const requestedPermalinkContextKey permalinkContextKey = "requested_permalink_hash"

func (app *App) canonicalHostMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if shouldRedirectToCanonicalRegistryHost(r) {
			http.Redirect(w, r, canonicalRegistryURL(canonicalRequestTargetPath(r)), http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (app *App) permalinkMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hash, trimmed, ok := trimPermalinkRequest(r)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		ctx := context.WithValue(trimmed.Context(), requestedPermalinkContextKey, hash)
		next.ServeHTTP(w, trimmed.WithContext(ctx))
	})
}

func trimPermalinkRequest(r *http.Request) (string, *http.Request, bool) {
	if r == nil || r.URL == nil {
		return "", r, false
	}
	path := strings.TrimSpace(r.URL.Path)
	if !strings.HasPrefix(path, "/@") {
		return "", r, false
	}
	rest := strings.TrimPrefix(path, "/")
	segment, _, ok := strings.Cut(rest, "/")
	if !ok {
		return "", r, false
	}
	hash := strings.TrimPrefix(segment, "@")
	if !isSHA256Hex(hash) {
		return "", r, false
	}
	prefix := "/" + segment
	r2 := r.Clone(r.Context())
	r2.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)
	if r2.URL.Path == "" {
		r2.URL.Path = "/"
	}
	if r.URL.RawPath != "" {
		r2.URL.RawPath = strings.TrimPrefix(r.URL.RawPath, prefix)
		if r2.URL.RawPath == "" {
			r2.URL.RawPath = "/"
		}
	}
	escapedPath := r2.URL.EscapedPath()
	if escapedPath == "" {
		escapedPath = r2.URL.Path
	}
	if !isPermalinkablePath(escapedPath) {
		return "", r, false
	}
	return hash, r2, true
}

func isPermalinkablePath(path string) bool {
	switch {
	case strings.HasPrefix(path, "/contracts/"):
		return path != "/contracts/"
	case strings.HasPrefix(path, "/realizations/"):
		return path != "/realizations/"
	case strings.HasPrefix(path, "/actions/"):
		return path != "/actions/"
	case strings.HasPrefix(path, "/commands/"):
		return path != "/commands/"
	case strings.HasPrefix(path, "/read-models/"):
		return path != "/read-models/"
	case strings.HasPrefix(path, "/projections/"):
		return path != "/projections/"
	case strings.HasPrefix(path, "/objects/"):
		return path != "/objects/"
	case strings.HasPrefix(path, "/schemas/"):
		return path != "/schemas/"
	case strings.HasPrefix(path, "/schemas/detail"):
		return true
	default:
		return false
	}
}

func isSHA256Hex(value string) bool {
	return registryIsSHA256Hex(value)
}

func registryIsSHA256Hex(value string) bool {
	if len(strings.TrimSpace(value)) != 64 {
		return false
	}
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

func requestedPermalinkHash(r *http.Request) string {
	if r == nil {
		return ""
	}
	value, _ := r.Context().Value(requestedPermalinkContextKey).(string)
	return strings.TrimSpace(value)
}

func matchesRequestedPermalink(r *http.Request, contentHash string) bool {
	requested := requestedPermalinkHash(r)
	if requested == "" {
		return true
	}
	return strings.EqualFold(requested, strings.TrimSpace(contentHash))
}

func withResourceIdentity(data map[string]any, canonicalURL, permalinkURL, contentHash string) map[string]any {
	data["CanonicalURL"] = canonicalURL
	data["PermalinkURL"] = permalinkURL
	data["ContentHash"] = contentHash
	return data
}

func setResourceIdentityHeaders(w http.ResponseWriter, canonicalURL, permalinkURL, contentHash string) {
	if w == nil {
		return
	}
	if hash := strings.TrimSpace(contentHash); hash != "" {
		w.Header().Set("ETag", `"sha256-`+hash+`"`)
	}
	if canonical := strings.TrimSpace(canonicalURL); canonical != "" {
		w.Header().Add("Link", "<"+canonical+">; rel=\"canonical\"")
	}
	if permalink := strings.TrimSpace(permalinkURL); permalink != "" {
		w.Header().Add("Link", "<"+permalink+">; rel=\"alternate\"")
	}
}

func parseSinglePathParam(r *http.Request, prefix string) (string, bool) {
	if r == nil || prefix == "" {
		return "", false
	}
	path := r.URL.EscapedPath()
	if path == "" {
		path = r.URL.Path
	}
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	path = strings.TrimPrefix(path, prefix)
	path = strings.TrimSuffix(path, "/")
	if path == "" || strings.Contains(path, "/") {
		return "", false
	}
	value, err := url.PathUnescape(path)
	if err != nil || value == "" {
		return "", false
	}
	return value, true
}

func parseReferencePathParam(r *http.Request, prefix string) (string, bool) {
	if r == nil || prefix == "" {
		return "", false
	}
	rest := r.URL.EscapedPath()
	if rest == "" {
		rest = r.URL.Path
	}
	if !strings.HasPrefix(rest, prefix) {
		return "", false
	}
	rest = strings.TrimPrefix(rest, prefix)
	rest = strings.TrimSuffix(rest, "/")
	parts := strings.Split(rest, "/")
	switch len(parts) {
	case 1:
		value, err := url.PathUnescape(parts[0])
		if err != nil || value == "" {
			return "", false
		}
		return value, true
	case 2:
		left, err1 := url.PathUnescape(parts[0])
		right, err2 := url.PathUnescape(parts[1])
		if err1 != nil || err2 != nil || left == "" || right == "" {
			return "", false
		}
		return left + "/" + right, true
	default:
		return "", false
	}
}

func parsePairPathParam(r *http.Request, prefix string) (string, string, bool) {
	if r == nil || prefix == "" {
		return "", "", false
	}
	rest := r.URL.EscapedPath()
	if rest == "" {
		rest = r.URL.Path
	}
	if !strings.HasPrefix(rest, prefix) {
		return "", "", false
	}
	rest = strings.TrimPrefix(rest, prefix)
	rest = strings.TrimSuffix(rest, "/")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	left, err1 := url.PathUnescape(parts[0])
	right, err2 := url.PathUnescape(parts[1])
	if err1 != nil || err2 != nil || left == "" || right == "" {
		return "", "", false
	}
	return left, right, true
}

func parseReferenceNamePathParam(r *http.Request, prefix string) (string, string, bool) {
	if r == nil || prefix == "" {
		return "", "", false
	}
	rest := r.URL.EscapedPath()
	if rest == "" {
		rest = r.URL.Path
	}
	if !strings.HasPrefix(rest, prefix) {
		return "", "", false
	}
	rest = strings.TrimPrefix(rest, prefix)
	rest = strings.TrimSuffix(rest, "/")
	parts := strings.Split(rest, "/")
	switch len(parts) {
	case 2:
		reference, err1 := url.PathUnescape(parts[0])
		name, err2 := url.PathUnescape(parts[1])
		if err1 != nil || err2 != nil || reference == "" || name == "" {
			return "", "", false
		}
		return reference, name, true
	case 3:
		seedID, err1 := url.PathUnescape(parts[0])
		realizationID, err2 := url.PathUnescape(parts[1])
		name, err3 := url.PathUnescape(parts[2])
		if err1 != nil || err2 != nil || err3 != nil || seedID == "" || realizationID == "" || name == "" {
			return "", "", false
		}
		return seedID + "/" + realizationID, name, true
	default:
		return "", "", false
	}
}

func browseRealizationPath(reference string) string {
	seedID, realizationID, ok := splitBrowseReference(reference)
	if !ok {
		return "/contracts/" + url.PathEscape(reference)
	}
	return "/contracts/" + url.PathEscape(seedID) + "/" + url.PathEscape(realizationID)
}

func browseCommandPath(reference, name string) string {
	seedID, realizationID, ok := splitBrowseReference(reference)
	if !ok {
		return "/actions/" + url.PathEscape(reference) + "/" + url.PathEscape(name)
	}
	return "/actions/" + url.PathEscape(seedID) + "/" + url.PathEscape(realizationID) + "/" + url.PathEscape(name)
}

func browseProjectionPath(reference, name string) string {
	seedID, realizationID, ok := splitBrowseReference(reference)
	if !ok {
		return "/read-models/" + url.PathEscape(reference) + "/" + url.PathEscape(name)
	}
	return "/read-models/" + url.PathEscape(seedID) + "/" + url.PathEscape(realizationID) + "/" + url.PathEscape(name)
}

func browseObjectsPath(seedID, schemaRef, q, relatedSeedID, relatedKind, relationDirection, relationKind string) template.URL {
	params := url.Values{}
	if seedID = strings.TrimSpace(seedID); seedID != "" {
		params.Set("seed_id", seedID)
	}
	if schemaRef = strings.TrimSpace(schemaRef); schemaRef != "" {
		params.Set("schema_ref", schemaRef)
	}
	if q = strings.TrimSpace(q); q != "" {
		params.Set("q", q)
	}
	if relatedSeedID = strings.TrimSpace(relatedSeedID); relatedSeedID != "" {
		params.Set("related_seed_id", relatedSeedID)
	}
	if relatedKind = strings.TrimSpace(relatedKind); relatedKind != "" {
		params.Set("related_kind", relatedKind)
	}
	if relationDirection = normalizeRelationDirection(relationDirection); relationDirection != "" {
		params.Set("relation_direction", relationDirection)
	}
	if relationKind = strings.TrimSpace(relationKind); relationKind != "" {
		params.Set("relation_kind", relationKind)
	}
	if len(params) == 0 {
		return trustedURL("/objects")
	}
	return trustedURL("/objects?" + params.Encode())
}

func browseRelatedObjectsPath(seedID, kind, direction, relationKind string) template.URL {
	seedID = strings.TrimSpace(seedID)
	return browseObjectsPath(seedID, "", "", seedID, strings.TrimSpace(kind), direction, relationKind)
}

func browseSystemPath(seedID string) template.URL {
	seedID = strings.TrimSpace(seedID)
	if seedID == "" {
		return trustedURL("/systems")
	}
	return trustedURL("/systems/" + url.PathEscape(seedID))
}

func browseSchemaPath(ref string) template.URL {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return trustedURL("/schemas")
	}
	pathPart, anchorPart, _ := strings.Cut(ref, "#")
	pathPart = strings.Trim(pathPart, "/")
	segments := strings.Split(pathPart, "/")
	escapedSegments := make([]string, 0, len(segments))
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		escapedSegments = append(escapedSegments, url.PathEscape(segment))
	}
	browsePath := "/schemas/" + strings.Join(escapedSegments, "/")
	if anchorPart = strings.TrimSpace(anchorPart); anchorPart != "" {
		browsePath += "/anchors/" + url.PathEscape(anchorPart)
	}
	return trustedURL(browsePath)
}

func permalinkResolvePath(canonicalURL, contentHash string) string {
	contentHash = strings.ToLower(strings.TrimSpace(contentHash))
	canonicalPath := registryPath(canonicalURL)
	if canonicalPath == "" || contentHash == "" {
		return ""
	}
	return canonicalRegistryURL("/@" + contentHash + canonicalPath)
}

func trustedURL(value string) template.URL {
	return template.URL(canonicalRegistryURL(value))
}

func trustedAPIURL(value string) template.URL {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if parsed, err := url.Parse(value); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		return template.URL(parsed.String())
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return template.URL(publicAPIBaseURL + value)
}

func repoSourceURL(path string) template.URL {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	filePath, fragment, _ := strings.Cut(path, "#")
	filePath = strings.TrimPrefix(strings.TrimSpace(filePath), "/")
	if filePath == "" {
		return ""
	}
	link := repoBlobBaseURL + filePath
	if fragment = strings.TrimSpace(fragment); fragment != "" {
		link += "#" + fragment
	}
	return trustedURL(link)
}

func canonicalRegistryURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return publicRegistryBaseURL
	}
	if parsed, err := url.Parse(value); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		return parsed.String()
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return publicRegistryBaseURL + value
}

func registryPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err == nil && parsed.Scheme != "" && parsed.Host != "" {
		path := parsed.EscapedPath()
		if path == "" {
			path = parsed.Path
		}
		if path == "" {
			return ""
		}
		if parsed.RawQuery != "" {
			path += "?" + parsed.RawQuery
		}
		return path
	}
	if !strings.HasPrefix(value, "/") {
		return ""
	}
	return value
}

func requestHost(r *http.Request) string {
	if r == nil {
		return ""
	}
	host := strings.TrimSpace(r.Host)
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}
	return strings.ToLower(strings.TrimSpace(host))
}

func isLocalRequestHost(host string) bool {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "", "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func shouldRedirectToCanonicalRegistryHost(r *http.Request) bool {
	host := requestHost(r)
	if isLocalRequestHost(host) {
		return false
	}
	if host == publicRegistryHost {
		return false
	}
	return host == "autosoftware.app" || strings.HasSuffix(host, ".autosoftware.app")
}

func requestTargetPath(r *http.Request) string {
	if r == nil || r.URL == nil {
		return "/"
	}
	path := r.URL.EscapedPath()
	if path == "" {
		path = r.URL.Path
	}
	if path == "" {
		path = "/"
	}
	if r.URL.RawQuery != "" {
		path += "?" + r.URL.RawQuery
	}
	return path
}

func canonicalRequestTargetPath(r *http.Request) string {
	if r == nil || r.URL == nil {
		return "/"
	}
	path := r.URL.EscapedPath()
	if path == "" {
		path = r.URL.Path
	}
	switch {
	case path == "" || path == legacyRegistryPathRoot:
		path = "/"
	case strings.HasPrefix(path, legacyRegistryPathPrefix):
		path = "/" + strings.TrimPrefix(path, legacyRegistryPathPrefix)
	}
	if r.URL.RawQuery != "" {
		path += "?" + r.URL.RawQuery
	}
	return path
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

func layoutSectionTitle(name string) string {
	switch strings.TrimSpace(name) {
	case "shared_metadata":
		return "Shared Metadata"
	case "public_payload":
		return "Public Payload"
	case "private_payload":
		return "Private Payload"
	case "runtime_only":
		return "Runtime-Only"
	default:
		return strings.ReplaceAll(strings.TrimSpace(name), "_", " ")
	}
}

func browseObjectPath(seedID, kind string) string {
	return "/objects/" + url.PathEscape(seedID) + "/" + url.PathEscape(kind)
}

func hasDataLayout(layout DataLayout) bool {
	return len(layout.SharedMetadata.Fields) > 0 ||
		len(layout.PublicPayload.Fields) > 0 ||
		len(layout.PrivatePayload.Fields) > 0 ||
		len(layout.RuntimeOnly.Fields) > 0 ||
		strings.TrimSpace(layout.SharedMetadata.Summary) != "" ||
		strings.TrimSpace(layout.PublicPayload.Summary) != "" ||
		strings.TrimSpace(layout.PrivatePayload.Summary) != "" ||
		strings.TrimSpace(layout.RuntimeOnly.Summary) != ""
}

func layoutShape(layout DataLayout) string {
	sections := []struct {
		name    string
		section DataSection
	}{
		{name: "shared_metadata", section: layout.SharedMetadata},
		{name: "public_payload", section: layout.PublicPayload},
		{name: "private_payload", section: layout.PrivatePayload},
		{name: "runtime_only", section: layout.RuntimeOnly},
	}
	lines := []string{"{"}
	added := 0
	for _, item := range sections {
		if len(item.section.Fields) == 0 {
			continue
		}
		lines = append(lines, fmt.Sprintf("  %q: {", item.name))
		for _, field := range item.section.Fields {
			valueType := strings.TrimSpace(field.Type)
			if valueType == "" {
				valueType = "value"
			}
			lines = append(lines, fmt.Sprintf("    %q: %q,", field.Name, valueType))
		}
		lines = append(lines, "  },")
		added++
	}
	if added == 0 {
		lines = append(lines, "}")
		return strings.Join(lines, "\n")
	}
	lines = append(lines, "}")
	return strings.Join(lines, "\n")
}

func parseSchemaBrowseRef(r *http.Request) (string, bool) {
	if r == nil || r.URL == nil {
		return "", false
	}
	rest := r.URL.EscapedPath()
	if rest == "" {
		rest = r.URL.Path
	}
	if !strings.HasPrefix(rest, "/schemas/") {
		return "", false
	}
	rest = strings.Trim(strings.TrimPrefix(rest, "/schemas/"), "/")
	if rest == "" || rest == "detail" {
		return "", false
	}

	pathPart := rest
	anchorPart := ""
	if before, after, ok := strings.Cut(rest, "/anchors/"); ok {
		pathPart = before
		anchorPart = after
	}

	var pathSegments []string
	for _, segment := range strings.Split(pathPart, "/") {
		if strings.TrimSpace(segment) == "" {
			continue
		}
		decoded, err := url.PathUnescape(segment)
		if err != nil || strings.TrimSpace(decoded) == "" {
			return "", false
		}
		pathSegments = append(pathSegments, decoded)
	}
	if len(pathSegments) == 0 {
		return "", false
	}

	ref := strings.Join(pathSegments, "/")
	if anchorPart = strings.TrimSpace(anchorPart); anchorPart != "" {
		decodedAnchor, err := url.PathUnescape(anchorPart)
		if err != nil || strings.TrimSpace(decodedAnchor) == "" {
			return "", false
		}
		ref += "#" + decodedAnchor
	}
	return ref, true
}

// --- Handlers ---

func (app *App) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (app *App) handleHome(w http.ResponseWriter, r *http.Request) {
	catalog, err := app.registry.Catalog()
	if err != nil {
		app.renderError(w, http.StatusBadGateway, "Could not reach the registry API.")
		log.Printf("catalog error: %v", err)
		return
	}
	systems := buildSystemViewsFromCatalog(catalog)
	domainSystems, internalSystems := splitSystems(systems)
	app.render(w, "home", map[string]any{
		"Title":           "Ledger Reading Room",
		"Nav":             "home",
		"Summary":         catalog.Summary,
		"Discovery":       catalog.Discovery,
		"Systems":         systems,
		"DomainSystems":   domainSystems,
		"InternalSystems": internalSystems,
	})
}

func (app *App) handleSystems(w http.ResponseWriter, r *http.Request) {
	catalog, err := app.registry.Catalog()
	if err != nil {
		app.renderError(w, http.StatusBadGateway, "Could not fetch systems.")
		log.Printf("systems error: %v", err)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	systems := filterSystems(buildSystemViewsFromCatalog(catalog), q)
	app.render(w, "systems", map[string]any{
		"Title":    "Systems",
		"Nav":      "systems",
		"Systems":  systems,
		"Query":    q,
		"APIRoute": "/v1/registry/catalog",
	})
}

func (app *App) handleSystemDetail(w http.ResponseWriter, r *http.Request) {
	seedID, ok := parseSinglePathParam(r, "/systems/")
	if !ok {
		app.renderError(w, http.StatusNotFound, "System not found.")
		return
	}
	realizations, err := app.registry.Realizations(seedID, "")
	if err != nil {
		app.renderError(w, http.StatusBadGateway, "Could not fetch system realizations.")
		log.Printf("system realizations error: %v", err)
		return
	}
	objects, err := app.registry.Objects(seedID, "", "")
	if err != nil {
		app.renderError(w, http.StatusBadGateway, "Could not fetch objects.")
		log.Printf("system objects error: %v", err)
		return
	}
	commands, err := app.registry.Commands(seedID, "", "")
	if err != nil {
		app.renderError(w, http.StatusBadGateway, "Could not fetch actions.")
		log.Printf("system commands error: %v", err)
		return
	}
	projections, err := app.registry.Projections(seedID, "", "")
	if err != nil {
		app.renderError(w, http.StatusBadGateway, "Could not fetch read models.")
		log.Printf("system projections error: %v", err)
		return
	}
	schemas, err := app.registry.Schemas(seedID, "")
	if err != nil {
		app.renderError(w, http.StatusBadGateway, "Could not fetch schemas.")
		log.Printf("system schemas error: %v", err)
		return
	}
	if len(realizations) == 0 && len(objects) == 0 && len(commands) == 0 && len(projections) == 0 && len(schemas) == 0 {
		app.renderError(w, http.StatusNotFound, "System not found.")
		return
	}
	system := buildSystemView(seedID, realizations, objects, app.hydrateCommandSummaries(commands), app.hydrateProjectionSummaries(projections), schemas)
	app.render(w, "system_detail", map[string]any{
		"Title":    system.Title,
		"Nav":      "systems",
		"System":   system,
		"APIRoute": "/v1/registry/catalog",
	})
}

func (app *App) handleRealizations(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	items, err := app.registry.Realizations("", q)
	if err != nil {
		app.renderError(w, http.StatusBadGateway, "Could not fetch realizations.")
		log.Printf("realizations error: %v", err)
		return
	}
	schemas, err := app.registry.Schemas("", "")
	if err == nil {
		counts := schemaCountBySeed(schemas)
		for i := range items {
			items[i].SchemaCount = counts[items[i].SeedID]
		}
	} else {
		log.Printf("realization schema count error: %v", err)
	}
	app.render(w, "realizations", map[string]any{
		"Title":        "Realizations",
		"Nav":          "contracts",
		"Realizations": items,
		"Query":        q,
		"APIRoute":     "/v1/registry/realizations",
	})
}

func (app *App) handleRealizationDetail(w http.ResponseWriter, r *http.Request) {
	reference, ok := parseReferencePathParam(r, "/realizations/")
	if !ok {
		reference, ok = parseReferencePathParam(r, "/contracts/")
	}
	if !ok {
		app.renderError(w, http.StatusNotFound, "Realization not found.")
		return
	}
	item, err := app.registry.Realization(reference)
	if err != nil {
		app.renderError(w, http.StatusNotFound, "Realization not found.")
		log.Printf("realization detail error: %v", err)
		return
	}
	schemas, err := app.registry.Schemas(item.SeedID, "")
	if err != nil {
		log.Printf("realization schemas error: %v", err)
	}
	if !matchesRequestedPermalink(r, item.ContentHash) {
		app.renderError(w, http.StatusNotFound, "This realization permalink does not match the current accepted registry state.")
		return
	}
	setResourceIdentityHeaders(w, item.CanonicalURL, item.PermalinkURL, item.ContentHash)
	app.render(w, "realization_detail", withResourceIdentity(map[string]any{
		"Title":       item.RealizationID,
		"Nav":         "contracts",
		"Realization": item,
		"Schemas":     schemas,
		"SourcePath":  item.ContractFile,
		"APIRoute":    item.Self,
	}, item.CanonicalURL, item.PermalinkURL, item.ContentHash))
}

func (app *App) handleCommands(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	seedID := r.URL.Query().Get("seed_id")
	reference := r.URL.Query().Get("reference")
	items, err := app.registry.Commands(seedID, reference, q)
	if err != nil {
		app.renderError(w, http.StatusBadGateway, "Could not fetch commands.")
		log.Printf("commands error: %v", err)
		return
	}
	items = app.hydrateCommandSummaries(items)
	app.render(w, "commands", map[string]any{
		"Title":    "Actions",
		"Nav":      "actions",
		"Commands": items,
		"Query":    q,
		"SeedID":   seedID,
		"APIRoute": "/v1/registry/commands",
	})
}

func (app *App) handleCommandDetail(w http.ResponseWriter, r *http.Request) {
	reference, name, ok := parseReferenceNamePathParam(r, "/actions/")
	if !ok {
		reference, name, ok = parseReferenceNamePathParam(r, "/commands/")
	}
	if !ok {
		app.renderError(w, http.StatusNotFound, "Command not found.")
		return
	}
	item, err := app.registry.Command(reference, name)
	if err != nil {
		app.renderError(w, http.StatusNotFound, "Command not found.")
		log.Printf("command detail error: %v", err)
		return
	}
	if !matchesRequestedPermalink(r, item.ContentHash) {
		app.renderError(w, http.StatusNotFound, "This action permalink does not match the current accepted registry state.")
		return
	}
	setResourceIdentityHeaders(w, item.CanonicalURL, item.PermalinkURL, item.ContentHash)
	app.render(w, "command_detail", withResourceIdentity(map[string]any{
		"Title":    item.Name,
		"Nav":      "actions",
		"Command":  item,
		"SourcePath": item.ContractFile,
		"APIRoute": item.Self,
	}, item.CanonicalURL, item.PermalinkURL, item.ContentHash))
}

func (app *App) handleProjections(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	seedID := r.URL.Query().Get("seed_id")
	reference := r.URL.Query().Get("reference")
	items, err := app.registry.Projections(seedID, reference, q)
	if err != nil {
		app.renderError(w, http.StatusBadGateway, "Could not fetch projections.")
		log.Printf("projections error: %v", err)
		return
	}
	items = app.hydrateProjectionSummaries(items)
	app.render(w, "projections", map[string]any{
		"Title":       "Read Models",
		"Nav":         "read-models",
		"Projections": items,
		"Query":       q,
		"SeedID":      seedID,
		"APIRoute":    "/v1/registry/projections",
	})
}

func (app *App) handleProjectionDetail(w http.ResponseWriter, r *http.Request) {
	reference, name, ok := parseReferenceNamePathParam(r, "/read-models/")
	if !ok {
		reference, name, ok = parseReferenceNamePathParam(r, "/projections/")
	}
	if !ok {
		app.renderError(w, http.StatusNotFound, "Projection not found.")
		return
	}
	item, err := app.registry.Projection(reference, name)
	if err != nil {
		app.renderError(w, http.StatusNotFound, "Projection not found.")
		log.Printf("projection detail error: %v", err)
		return
	}
	if !matchesRequestedPermalink(r, item.ContentHash) {
		app.renderError(w, http.StatusNotFound, "This read model permalink does not match the current accepted registry state.")
		return
	}
	setResourceIdentityHeaders(w, item.CanonicalURL, item.PermalinkURL, item.ContentHash)
	app.render(w, "projection_detail", withResourceIdentity(map[string]any{
		"Title":      item.Name,
		"Nav":        "read-models",
		"Projection": item,
		"SourcePath": item.ContractFile,
		"APIRoute":   item.Self,
	}, item.CanonicalURL, item.PermalinkURL, item.ContentHash))
}

func (app *App) handleObjects(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	seedID := r.URL.Query().Get("seed_id")
	schemaRef := r.URL.Query().Get("schema_ref")
	relatedSeedID := strings.TrimSpace(r.URL.Query().Get("related_seed_id"))
	relatedKind := strings.TrimSpace(r.URL.Query().Get("related_kind"))
	relationDirection := normalizeRelationDirection(r.URL.Query().Get("relation_direction"))
	relationKind := strings.TrimSpace(r.URL.Query().Get("relation_kind"))
	if seedID == "" && relatedSeedID != "" {
		seedID = relatedSeedID
	}
	items, err := app.registry.Objects(seedID, schemaRef, q)
	if err != nil {
		app.renderError(w, http.StatusBadGateway, "Could not fetch objects.")
		log.Printf("objects error: %v", err)
		return
	}

	var relationBrowse *ObjectRelationBrowseContext
	if relatedKind != "" {
		if relatedSeedID == "" {
			relatedSeedID = seedID
		}
		if relatedSeedID == "" {
			app.renderError(w, http.StatusBadRequest, "Related object seed is required.")
			return
		}
		anchor, err := app.registry.Object(relatedSeedID, relatedKind)
		if err != nil {
			app.renderError(w, http.StatusNotFound, "Related object not found.")
			log.Printf("related object detail error: %v", err)
			return
		}
		matchedKinds := collectRelatedObjectKinds(anchor, relationDirection, relationKind)
		items = filterObjectSummariesByKindSet(items, matchedKinds)
		relationBrowse = &ObjectRelationBrowseContext{
			SeedID:          relatedSeedID,
			Kind:            relatedKind,
			Direction:       relationDirection,
			DirectionLabel:  relationDirectionLabel(relationDirection),
			RelationKind:    relationKind,
			MatchedKinds:    matchedKinds,
			MatchedCount:    len(items),
			ClearFiltersURL: browseObjectsPath(seedID, schemaRef, q, "", "", "", ""),
		}
	}

	seedIDs := uniqueStrings(items, func(o ObjectSummary) string { return o.SeedID })

	app.render(w, "objects", map[string]any{
		"Title":             "Objects",
		"Nav":               "objects",
		"Objects":           items,
		"Query":             q,
		"SeedID":            seedID,
		"SchemaRef":         schemaRef,
		"SeedIDs":           seedIDs,
		"RelatedSeedID":     relatedSeedID,
		"RelatedKind":       relatedKind,
		"RelationDirection": relationDirection,
		"RelationKind":      relationKind,
		"RelationBrowse":    relationBrowse,
		"APIRoute":          "/v1/registry/objects",
	})
}

func (app *App) handleObjectDetail(w http.ResponseWriter, r *http.Request) {
	seedID, kind, ok := parsePairPathParam(r, "/objects/")
	if !ok {
		app.renderError(w, http.StatusNotFound, "Object not found.")
		return
	}
	item, err := app.registry.Object(seedID, kind)
	if err != nil {
		app.renderError(w, http.StatusNotFound, "Object not found.")
		log.Printf("object detail error: %v", err)
		return
	}
	if !matchesRequestedPermalink(r, item.ContentHash) {
		app.renderError(w, http.StatusNotFound, "This object permalink does not match the current accepted registry state.")
		return
	}
	setResourceIdentityHeaders(w, item.CanonicalURL, item.PermalinkURL, item.ContentHash)
	app.render(w, "object_detail", withResourceIdentity(map[string]any{
		"Title":    item.Kind,
		"Nav":      "objects",
		"Object":   item,
		"APIRoute": item.Self,
	}, item.CanonicalURL, item.PermalinkURL, item.ContentHash))
}

func (app *App) handleSchemas(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	seedID := r.URL.Query().Get("seed_id")
	items, err := app.registry.Schemas(seedID, q)
	if err != nil {
		app.renderError(w, http.StatusBadGateway, "Could not fetch schemas.")
		log.Printf("schemas error: %v", err)
		return
	}
	app.render(w, "schemas", map[string]any{
		"Title":    "Schemas",
		"Nav":      "contracts",
		"Schemas":  items,
		"Query":    q,
		"SeedID":   seedID,
		"APIRoute": "/v1/registry/schemas",
	})
}

func (app *App) handleSchemaDetail(w http.ResponseWriter, r *http.Request) {
	ref, ok := parseSchemaBrowseRef(r)
	if !ok {
		ref = strings.TrimSpace(r.URL.Query().Get("ref"))
		if ref == "" {
			app.renderError(w, http.StatusBadRequest, "Schema ref is required.")
			return
		}
		if requestedPermalinkHash(r) == "" {
			http.Redirect(w, r, string(browseSchemaPath(ref)), http.StatusMovedPermanently)
			return
		}
	}
	if ref == "" {
		app.renderError(w, http.StatusBadRequest, "Schema ref is required.")
		return
	}
	item, err := app.registry.Schema(ref)
	if err != nil {
		app.renderError(w, http.StatusNotFound, "Schema not found.")
		log.Printf("schema detail error: %v", err)
		return
	}
	if !matchesRequestedPermalink(r, item.ContentHash) {
		app.renderError(w, http.StatusNotFound, "This schema permalink does not match the current accepted registry state.")
		return
	}
	setResourceIdentityHeaders(w, item.CanonicalURL, item.PermalinkURL, item.ContentHash)
	app.render(w, "schema_detail", withResourceIdentity(map[string]any{
		"Title":    item.Ref,
		"Nav":      "contracts",
		"Schema":   item,
		"SourcePath": item.Ref,
		"APIRoute": item.Self,
	}, item.CanonicalURL, item.PermalinkURL, item.ContentHash))
}

func (app *App) handlePermalinkLookup(w http.ResponseWriter, r *http.Request) {
	contentHash, ok := parseSinglePathParam(r, "/reg/")
	if !ok || !registryIsSHA256Hex(contentHash) {
		app.renderError(w, http.StatusNotFound, "Permalink not found.")
		return
	}

	lookup, err := app.registry.Lookup(contentHash)
	if err != nil {
		if strings.Contains(err.Error(), "returned 404") {
			app.renderError(w, http.StatusNotFound, "Permalink not found.")
			return
		}
		app.renderError(w, http.StatusBadGateway, "Could not resolve registry permalink.")
		log.Printf("registry permalink lookup error: %v", err)
		return
	}

	redirectURL := strings.TrimSpace(lookup.RedirectURL)
	if redirectURL == "" {
		redirectURL = permalinkResolvePath(lookup.CanonicalURL, lookup.ContentHash)
	}
	if redirectURL == "" {
		app.renderError(w, http.StatusNotFound, "Permalink not found.")
		return
	}
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (app *App) handleRegistryInternals(w http.ResponseWriter, r *http.Request) {
	const seedID = "0006-registry-browser"
	realizations, err := app.registry.Realizations(seedID, "")
	if err != nil {
		app.renderError(w, http.StatusBadGateway, "Could not fetch registry internals.")
		log.Printf("registry internals realizations error: %v", err)
		return
	}
	objects, err := app.registry.Objects(seedID, "", "")
	if err != nil {
		app.renderError(w, http.StatusBadGateway, "Could not fetch registry internals.")
		log.Printf("registry internals objects error: %v", err)
		return
	}
	commands, err := app.registry.Commands(seedID, "", "")
	if err != nil {
		app.renderError(w, http.StatusBadGateway, "Could not fetch registry internals.")
		log.Printf("registry internals commands error: %v", err)
		return
	}
	projections, err := app.registry.Projections(seedID, "", "")
	if err != nil {
		app.renderError(w, http.StatusBadGateway, "Could not fetch registry internals.")
		log.Printf("registry internals projections error: %v", err)
		return
	}
	schemas, err := app.registry.Schemas(seedID, "")
	if err != nil {
		app.renderError(w, http.StatusBadGateway, "Could not fetch registry internals.")
		log.Printf("registry internals schemas error: %v", err)
		return
	}
	system := buildSystemView(seedID, realizations, objects, app.hydrateCommandSummaries(commands), app.hydrateProjectionSummaries(projections), schemas)
	app.render(w, "registry_internals", map[string]any{
		"Title":    "Registry Internals",
		"Nav":      "registry-internals",
		"System":   system,
		"APIRoute": "/v1/registry/catalog",
	})
}

func (app *App) handleStyleCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(styleCSS)
}

// --- Middleware ---

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Microsecond))
	})
}

// --- Helpers ---

func uniqueStrings[T any](items []T, fn func(T) string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, item := range items {
		s := fn(item)
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func normalizeRelationDirection(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "incoming":
		return "incoming"
	case "outgoing":
		return "outgoing"
	default:
		return ""
	}
}

func relationDirectionLabel(direction string) string {
	switch normalizeRelationDirection(direction) {
	case "incoming":
		return "point to"
	case "outgoing":
		return "are pointed to by"
	default:
		return "relate to"
	}
}

func collectRelatedObjectKinds(item *ObjectDetail, direction, relationKind string) []string {
	if item == nil {
		return nil
	}

	matchRelation := func(name string) bool {
		if strings.TrimSpace(relationKind) == "" {
			return true
		}
		return strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(relationKind))
	}

	seen := make(map[string]bool)
	var out []string
	addKind := func(kind string) {
		kind = strings.TrimSpace(kind)
		if kind == "" || seen[kind] {
			return
		}
		seen[kind] = true
		out = append(out, kind)
	}
	addRelationKinds := func(relations []GraphRelation, pick func(GraphRelation) []ResourceLink) {
		for _, relation := range relations {
			if !matchRelation(relation.Kind) {
				continue
			}
			for _, object := range pick(relation) {
				addKind(object.Kind)
			}
		}
	}

	switch normalizeRelationDirection(direction) {
	case "incoming":
		addRelationKinds(item.IncomingRelations, func(relation GraphRelation) []ResourceLink { return relation.FromObjects })
	case "outgoing":
		addRelationKinds(item.OutgoingRelations, func(relation GraphRelation) []ResourceLink { return relation.ToObjects })
	default:
		addRelationKinds(item.IncomingRelations, func(relation GraphRelation) []ResourceLink { return relation.FromObjects })
		addRelationKinds(item.OutgoingRelations, func(relation GraphRelation) []ResourceLink { return relation.ToObjects })
	}

	sort.Strings(out)
	return out
}

func filterObjectSummariesByKindSet(items []ObjectSummary, kinds []string) []ObjectSummary {
	if len(kinds) == 0 {
		return nil
	}
	allowed := make(map[string]bool, len(kinds))
	for _, kind := range kinds {
		if kind = strings.TrimSpace(kind); kind != "" {
			allowed[kind] = true
		}
	}
	var out []ObjectSummary
	for _, item := range items {
		if allowed[strings.TrimSpace(item.Kind)] {
			out = append(out, item)
		}
	}
	return out
}

func splitSystems(systems []SystemView) ([]SystemView, []SystemView) {
	var domain, internal []SystemView
	for _, system := range systems {
		if system.IsRegistryInternal {
			internal = append(internal, system)
		} else {
			domain = append(domain, system)
		}
	}
	return domain, internal
}

func filterSystems(systems []SystemView, q string) []SystemView {
	if strings.TrimSpace(q) == "" {
		return systems
	}
	query := strings.ToLower(strings.TrimSpace(q))
	var out []SystemView
	for _, system := range systems {
		if strings.Contains(strings.ToLower(system.SeedID), query) ||
			strings.Contains(strings.ToLower(system.Title), query) ||
			strings.Contains(strings.ToLower(system.Summary), query) {
			out = append(out, system)
		}
	}
	return out
}

func buildSystemViewsFromCatalog(catalog *CatalogResponse) []SystemView {
	realizationBySeed := map[string][]RealizationSummary{}
	objectBySeed := map[string][]ObjectSummary{}
	commandBySeed := map[string][]CommandSummary{}
	projectionBySeed := map[string][]ProjectionSummary{}
	schemaBySeed := map[string][]SchemaSummary{}
	allSeeds := map[string]bool{}

	for _, item := range catalog.Realizations {
		realizationBySeed[item.SeedID] = append(realizationBySeed[item.SeedID], item)
		allSeeds[item.SeedID] = true
	}
	for _, item := range catalog.Objects {
		objectBySeed[item.SeedID] = append(objectBySeed[item.SeedID], item)
		allSeeds[item.SeedID] = true
	}
	for _, item := range catalog.Commands {
		commandBySeed[item.SeedID] = append(commandBySeed[item.SeedID], item)
		allSeeds[item.SeedID] = true
	}
	for _, item := range catalog.Projections {
		projectionBySeed[item.SeedID] = append(projectionBySeed[item.SeedID], item)
		allSeeds[item.SeedID] = true
	}
	for _, item := range catalog.Schemas {
		seedID := seedIDFromSchema(item)
		if seedID == "" {
			continue
		}
		schemaBySeed[seedID] = append(schemaBySeed[seedID], item)
		allSeeds[seedID] = true
	}

	var systems []SystemView
	for seedID := range allSeeds {
		systems = append(systems, buildSystemView(
			seedID,
			realizationBySeed[seedID],
			objectBySeed[seedID],
			commandBySeed[seedID],
			projectionBySeed[seedID],
			schemaBySeed[seedID],
		))
	}

	sort.Slice(systems, func(i, j int) bool { return systems[i].SeedID > systems[j].SeedID })
	return systems
}

func buildSystemView(seedID string, realizations []RealizationSummary, objects []ObjectSummary, commands []CommandSummary, projections []ProjectionSummary, schemas []SchemaSummary) SystemView {
	statuses := uniqueStrings(realizations, func(item RealizationSummary) string { return item.Status })
	surfaceKinds := uniqueStrings(realizations, func(item RealizationSummary) string { return item.SurfaceKind })
	return SystemView{
		SeedID:             seedID,
		Title:              humanizeSeedID(seedID),
		Summary:            pickSystemSummary(seedID, realizations, objects),
		Statuses:           statuses,
		SurfaceKinds:       surfaceKinds,
		RealizationCount:   len(realizations),
		ObjectCount:        len(objects),
		ActionCount:        len(commands),
		ReadModelCount:     len(projections),
		SchemaCount:        len(schemas),
		IsRegistryInternal: seedID == "0006-registry-browser",
		Realizations:       realizations,
		Objects:            objects,
		Commands:           commands,
		Projections:        projections,
		Schemas:            schemas,
	}
}

func pickSystemSummary(seedID string, realizations []RealizationSummary, objects []ObjectSummary) string {
	for _, item := range realizations {
		if strings.TrimSpace(item.Summary) != "" {
			return strings.TrimSpace(item.Summary)
		}
	}
	for _, item := range objects {
		if strings.TrimSpace(item.Summary) != "" {
			return strings.TrimSpace(item.Summary)
		}
	}
	if seedID == "0006-registry-browser" {
		return "The registry describing itself: its objects, actions, read models, realizations, and schemas."
	}
	return "Accepted registry resources grouped under one seed identity."
}

func humanizeSeedID(seedID string) string {
	trimmed := strings.TrimSpace(seedID)
	if strings.HasPrefix(trimmed, "0000-genesis") {
		return "Genesis"
	}
	if len(trimmed) > 5 && isDigits(trimmed[:4]) && trimmed[4] == '-' {
		trimmed = trimmed[5:]
	}
	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	for i, part := range parts {
		runes := []rune(strings.ToLower(part))
		if len(runes) == 0 {
			continue
		}
		runes[0] = unicode.ToUpper(runes[0])
		parts[i] = string(runes)
	}
	return strings.Join(parts, " ")
}

func isDigits(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return value != ""
}

func seedIDFromSchema(item SchemaSummary) string {
	path := strings.TrimSpace(item.Path)
	if strings.HasPrefix(path, "seeds/") {
		parts := strings.Split(path, "/")
		if len(parts) >= 2 {
			return parts[1]
		}
	}
	if strings.HasPrefix(path, "genesis/") || strings.HasPrefix(strings.TrimSpace(item.Ref), "genesis/") {
		return "0000-genesis"
	}
	return ""
}

func schemaCountBySeed(items []SchemaSummary) map[string]int {
	out := map[string]int{}
	for _, item := range items {
		seedID := seedIDFromSchema(item)
		if seedID == "" {
			continue
		}
		out[seedID]++
	}
	return out
}

func (app *App) hydrateCommandSummaries(items []CommandSummary) []CommandSummary {
	out := make([]CommandSummary, 0, len(items))
	for _, item := range items {
		detail, err := app.registry.Command(item.Reference, item.Name)
		if err != nil {
			log.Printf("command hydrate error for %s %s: %v", item.Reference, item.Name, err)
			out = append(out, item)
			continue
		}
		item.Summary = detail.Summary
		item.Projection = detail.Projection
		out = append(out, item)
	}
	return out
}

func (app *App) hydrateProjectionSummaries(items []ProjectionSummary) []ProjectionSummary {
	out := make([]ProjectionSummary, 0, len(items))
	for _, item := range items {
		detail, err := app.registry.Projection(item.Reference, item.Name)
		if err != nil {
			log.Printf("projection hydrate error for %s %s: %v", item.Reference, item.Name, err)
			out = append(out, item)
			continue
		}
		item.Summary = detail.Summary
		out = append(out, item)
	}
	return out
}

// --- Main ---

func main() {
	addr := os.Getenv("AS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:9006"
	}

	registryURL := os.Getenv("AS_REGISTRY_URL")
	if registryURL == "" {
		registryURL = "http://localhost:8090"
	}

	app := &App{
		registry: NewRegistryClient(registryURL),
	}
	app.loadTemplates()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", app.handleHealthz)
	mux.HandleFunc("GET /{$}", app.handleHome)
	mux.HandleFunc("GET /systems", app.handleSystems)
	mux.HandleFunc("GET /systems/", app.handleSystemDetail)
	mux.HandleFunc("GET /registry-internals", app.handleRegistryInternals)
	mux.HandleFunc("GET /contracts", app.handleRealizations)
	mux.HandleFunc("GET /contracts/", app.handleRealizationDetail)
	mux.HandleFunc("GET /actions", app.handleCommands)
	mux.HandleFunc("GET /actions/", app.handleCommandDetail)
	mux.HandleFunc("GET /read-models", app.handleProjections)
	mux.HandleFunc("GET /read-models/", app.handleProjectionDetail)
	mux.HandleFunc("GET /realizations", app.handleRealizations)
	mux.HandleFunc("GET /realizations/", app.handleRealizationDetail)
	mux.HandleFunc("GET /commands", app.handleCommands)
	mux.HandleFunc("GET /commands/", app.handleCommandDetail)
	mux.HandleFunc("GET /projections", app.handleProjections)
	mux.HandleFunc("GET /projections/", app.handleProjectionDetail)
	mux.HandleFunc("GET /objects", app.handleObjects)
	mux.HandleFunc("GET /objects/", app.handleObjectDetail)
	mux.HandleFunc("GET /schemas", app.handleSchemas)
	mux.HandleFunc("GET /schemas/", app.handleSchemaDetail)
	mux.HandleFunc("GET /schemas/detail", app.handleSchemaDetail)
	mux.HandleFunc("GET /reg/", app.handlePermalinkLookup)
	mux.HandleFunc("GET /assets/style.css", app.handleStyleCSS)

	handler := logMiddleware(app.canonicalHostMiddleware(app.permalinkMiddleware(mux)))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if strings.HasPrefix(addr, "/") || strings.HasPrefix(addr, ".") {
		os.MkdirAll(filepath.Dir(addr), 0755)
		ln, err := net.Listen("unix", addr)
		if err != nil {
			log.Fatalf("listen unix: %v", err)
		}
		defer ln.Close()
		defer os.Remove(addr)

		srv := &http.Server{Handler: handler}
		go func() {
			<-ctx.Done()
			srv.Shutdown(context.Background())
		}()

		log.Printf("registry browser listening on unix:%s (registry: %s)", addr, registryURL)
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("serve: %v", err)
		}
		return
	}

	srv := &http.Server{Addr: addr, Handler: handler}
	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()

	log.Printf("registry browser listening on %s (registry: %s)", addr, registryURL)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("serve: %v", err)
	}
}
