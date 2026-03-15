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
	"strings"
	"time"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed assets/style.css
var styleCSS []byte

// --- Registry API response types ---

type CatalogResponse struct {
	Summary      CatalogSummary        `json:"summary"`
	Realizations []RealizationSummary  `json:"realizations"`
	Commands     []CommandSummary      `json:"commands"`
	Projections  []ProjectionSummary   `json:"projections"`
	Objects      []ObjectSummary       `json:"objects"`
	Schemas      []SchemaSummary       `json:"schemas"`
	Discovery    map[string]string     `json:"discovery"`
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
	Self            string   `json:"self"`
}

type RealizationDetail struct {
	Reference      string              `json:"reference"`
	SeedID         string              `json:"seed_id"`
	RealizationID  string              `json:"realization_id"`
	ApproachID     string              `json:"approach_id,omitempty"`
	Summary        string              `json:"summary"`
	Status         string              `json:"status"`
	SurfaceKind    string              `json:"surface_kind"`
	ContractFile   string              `json:"contract_file"`
	AuthModes      []string            `json:"auth_modes"`
	Capabilities   []string            `json:"capabilities"`
	ObjectKinds    []string            `json:"object_kinds"`
	Objects        []ResourceLink      `json:"objects"`
	Commands       []ResourceLink      `json:"commands"`
	Projections    []ResourceLink      `json:"projections"`
	Contract       string              `json:"contract"`
	Self           string              `json:"self"`
}

type CommandSummary struct {
	Reference       string `json:"reference"`
	SeedID          string `json:"seed_id"`
	RealizationID   string `json:"realization_id"`
	Name            string `json:"name"`
	Path            string `json:"path"`
	InputSchemaRef  string `json:"input_schema_ref"`
	ResultSchemaRef string `json:"result_schema_ref"`
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
}

type ProjectionSummary struct {
	Reference     string `json:"reference"`
	SeedID        string `json:"seed_id"`
	RealizationID string `json:"realization_id"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	Freshness     string `json:"freshness"`
	Self          string `json:"self"`
}

type ProjectionDetail struct {
	Reference     string   `json:"reference"`
	SeedID        string   `json:"seed_id"`
	RealizationID string   `json:"realization_id"`
	Name          string   `json:"name"`
	Summary       string   `json:"summary"`
	Path          string   `json:"path"`
	Capabilities  []string `json:"capabilities"`
	Freshness     string   `json:"freshness"`
	ContractFile  string   `json:"contract_file"`
	Contract      string   `json:"contract"`
	Self          string   `json:"self"`
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
	SeedID       string                `json:"seed_id"`
	Kind         string                `json:"kind"`
	Summary      string                `json:"summary"`
	Capabilities []string              `json:"capabilities"`
	SchemaRefs   []string              `json:"schema_refs"`
	Schemas      []ResourceLink        `json:"schemas"`
	Realizations []ObjectRealization   `json:"realizations"`
	Commands     []CommandDetail       `json:"commands"`
	Projections  []ProjectionDetail    `json:"projections"`
	Self         string                `json:"self"`
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
	Ref            string              `json:"ref"`
	Path           string              `json:"path"`
	Anchor         string              `json:"anchor,omitempty"`
	ObjectUses     []SchemaObjectUse   `json:"object_uses"`
	CommandInputs  []SchemaCommandUse  `json:"command_inputs"`
	CommandResults []SchemaCommandUse  `json:"command_results"`
	Self           string              `json:"self"`
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

// --- Registry client ---

type RegistryClient struct {
	baseURL string
	http    *http.Client
}

func NewRegistryClient(baseURL string) *RegistryClient {
	return &RegistryClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 10 * time.Second},
	}
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
	var resp struct{ Realizations []RealizationSummary `json:"realizations"` }
	if err := c.get("/v1/registry/realizations"+params, &resp); err != nil {
		return nil, err
	}
	return resp.Realizations, nil
}

func (c *RegistryClient) Realization(reference string) (*RealizationDetail, error) {
	var resp struct{ Realization RealizationDetail `json:"realization"` }
	if err := c.get("/v1/registry/realization?reference="+url.QueryEscape(reference), &resp); err != nil {
		return nil, err
	}
	return &resp.Realization, nil
}

func (c *RegistryClient) Commands(seedID, reference, q string) ([]CommandSummary, error) {
	params := buildQuery("seed_id", seedID, "reference", reference, "q", q)
	var resp struct{ Commands []CommandSummary `json:"commands"` }
	if err := c.get("/v1/registry/commands"+params, &resp); err != nil {
		return nil, err
	}
	return resp.Commands, nil
}

func (c *RegistryClient) Command(reference, name string) (*CommandDetail, error) {
	var resp struct{ Command CommandDetail `json:"command"` }
	path := "/v1/registry/command?reference=" + url.QueryEscape(reference) + "&name=" + url.QueryEscape(name)
	if err := c.get(path, &resp); err != nil {
		return nil, err
	}
	return &resp.Command, nil
}

func (c *RegistryClient) Projections(seedID, reference, q string) ([]ProjectionSummary, error) {
	params := buildQuery("seed_id", seedID, "reference", reference, "q", q)
	var resp struct{ Projections []ProjectionSummary `json:"projections"` }
	if err := c.get("/v1/registry/projections"+params, &resp); err != nil {
		return nil, err
	}
	return resp.Projections, nil
}

func (c *RegistryClient) Projection(reference, name string) (*ProjectionDetail, error) {
	var resp struct{ Projection ProjectionDetail `json:"projection"` }
	path := "/v1/registry/projection?reference=" + url.QueryEscape(reference) + "&name=" + url.QueryEscape(name)
	if err := c.get(path, &resp); err != nil {
		return nil, err
	}
	return &resp.Projection, nil
}

func (c *RegistryClient) Objects(seedID, schemaRef, q string) ([]ObjectSummary, error) {
	params := buildQuery("seed_id", seedID, "schema_ref", schemaRef, "q", q)
	var resp struct{ Objects []ObjectSummary `json:"objects"` }
	if err := c.get("/v1/registry/objects"+params, &resp); err != nil {
		return nil, err
	}
	return resp.Objects, nil
}

func (c *RegistryClient) Object(seedID, kind string) (*ObjectDetail, error) {
	var resp struct{ Object ObjectDetail `json:"object"` }
	path := "/v1/registry/object?seed_id=" + url.QueryEscape(seedID) + "&kind=" + url.QueryEscape(kind)
	if err := c.get(path, &resp); err != nil {
		return nil, err
	}
	return &resp.Object, nil
}

func (c *RegistryClient) Schemas(seedID, q string) ([]SchemaSummary, error) {
	params := buildQuery("seed_id", seedID, "q", q)
	var resp struct{ Schemas []SchemaSummary `json:"schemas"` }
	if err := c.get("/v1/registry/schemas"+params, &resp); err != nil {
		return nil, err
	}
	return resp.Schemas, nil
}

func (c *RegistryClient) Schema(ref string) (*SchemaDetail, error) {
	var resp struct{ Schema SchemaDetail `json:"schema"` }
	if err := c.get("/v1/registry/schema?ref="+url.QueryEscape(ref), &resp); err != nil {
		return nil, err
	}
	return &resp.Schema, nil
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

func (app *App) loadTemplates() {
	funcMap := template.FuncMap{
		"join": strings.Join,
		"pathEscape": func(s string) string {
			return url.PathEscape(s)
		},
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
	}

	pages := []string{
		"home",
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
<body><div class="container"><h1>%d</h1><p>%s</p><p><a href="/">Back to home</a></p></div></body></html>`, status, template.HTMLEscapeString(msg))
}

// --- Handlers ---

func (app *App) handleHome(w http.ResponseWriter, r *http.Request) {
	catalog, err := app.registry.Catalog()
	if err != nil {
		app.renderError(w, http.StatusBadGateway, "Could not reach the registry API.")
		log.Printf("catalog error: %v", err)
		return
	}
	app.render(w, "home", map[string]any{
		"Title":     "Registry",
		"Summary":   catalog.Summary,
		"Discovery": catalog.Discovery,
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
	app.render(w, "realizations", map[string]any{
		"Title":        "Realizations",
		"Realizations": items,
		"Query":        q,
		"APIRoute":     "/v1/registry/realizations",
	})
}

func (app *App) handleRealizationDetail(w http.ResponseWriter, r *http.Request) {
	reference := r.PathValue("reference")
	item, err := app.registry.Realization(reference)
	if err != nil {
		app.renderError(w, http.StatusNotFound, "Realization not found.")
		log.Printf("realization detail error: %v", err)
		return
	}
	app.render(w, "realization_detail", map[string]any{
		"Title":       item.RealizationID,
		"Realization": item,
		"APIRoute":    item.Self,
	})
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
	app.render(w, "commands", map[string]any{
		"Title":    "Commands",
		"Commands": items,
		"Query":    q,
		"SeedID":   seedID,
		"APIRoute": "/v1/registry/commands",
	})
}

func (app *App) handleCommandDetail(w http.ResponseWriter, r *http.Request) {
	reference := r.PathValue("reference")
	name := r.PathValue("name")
	item, err := app.registry.Command(reference, name)
	if err != nil {
		app.renderError(w, http.StatusNotFound, "Command not found.")
		log.Printf("command detail error: %v", err)
		return
	}
	app.render(w, "command_detail", map[string]any{
		"Title":    item.Name,
		"Command":  item,
		"APIRoute": item.Self,
	})
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
	app.render(w, "projections", map[string]any{
		"Title":       "Projections",
		"Projections": items,
		"Query":       q,
		"SeedID":      seedID,
		"APIRoute":    "/v1/registry/projections",
	})
}

func (app *App) handleProjectionDetail(w http.ResponseWriter, r *http.Request) {
	reference := r.PathValue("reference")
	name := r.PathValue("name")
	item, err := app.registry.Projection(reference, name)
	if err != nil {
		app.renderError(w, http.StatusNotFound, "Projection not found.")
		log.Printf("projection detail error: %v", err)
		return
	}
	app.render(w, "projection_detail", map[string]any{
		"Title":      item.Name,
		"Projection": item,
		"APIRoute":   item.Self,
	})
}

func (app *App) handleObjects(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	seedID := r.URL.Query().Get("seed_id")
	schemaRef := r.URL.Query().Get("schema_ref")
	items, err := app.registry.Objects(seedID, schemaRef, q)
	if err != nil {
		app.renderError(w, http.StatusBadGateway, "Could not fetch objects.")
		log.Printf("objects error: %v", err)
		return
	}

	seedIDs := uniqueStrings(items, func(o ObjectSummary) string { return o.SeedID })

	app.render(w, "objects", map[string]any{
		"Title":         "Objects",
		"Objects":       items,
		"Query":         q,
		"SeedID":        seedID,
		"SchemaRef":     schemaRef,
		"SeedIDs":       seedIDs,
		"APIRoute":      "/v1/registry/objects",
	})
}

func (app *App) handleObjectDetail(w http.ResponseWriter, r *http.Request) {
	seedID := r.PathValue("seed_id")
	kind := r.PathValue("kind")
	item, err := app.registry.Object(seedID, kind)
	if err != nil {
		app.renderError(w, http.StatusNotFound, "Object not found.")
		log.Printf("object detail error: %v", err)
		return
	}
	app.render(w, "object_detail", map[string]any{
		"Title":    item.Kind,
		"Object":   item,
		"APIRoute": item.Self,
	})
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
		"Schemas":  items,
		"Query":    q,
		"SeedID":   seedID,
		"APIRoute": "/v1/registry/schemas",
	})
}

func (app *App) handleSchemaDetail(w http.ResponseWriter, r *http.Request) {
	ref := r.URL.Query().Get("ref")
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
	app.render(w, "schema_detail", map[string]any{
		"Title":    item.Ref,
		"Schema":   item,
		"APIRoute": item.Self,
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

	mux.HandleFunc("GET /{$}", app.handleHome)
	mux.HandleFunc("GET /realizations", app.handleRealizations)
	mux.HandleFunc("GET /realizations/{reference}", app.handleRealizationDetail)
	mux.HandleFunc("GET /commands", app.handleCommands)
	mux.HandleFunc("GET /commands/{reference}/{name}", app.handleCommandDetail)
	mux.HandleFunc("GET /projections", app.handleProjections)
	mux.HandleFunc("GET /projections/{reference}/{name}", app.handleProjectionDetail)
	mux.HandleFunc("GET /objects", app.handleObjects)
	mux.HandleFunc("GET /objects/{seed_id}/{kind}", app.handleObjectDetail)
	mux.HandleFunc("GET /schemas", app.handleSchemas)
	mux.HandleFunc("GET /schemas/detail", app.handleSchemaDetail)
	mux.HandleFunc("GET /assets/style.css", app.handleStyleCSS)

	handler := logMiddleware(mux)

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
