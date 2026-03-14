package main

import (
	"bytes"
	"context"
	"errors"
	"html/template"
	"log"
	"net/http"
	"strings"

	"as/kernel/internal/boot"
	"as/kernel/internal/config"
	feedbackloop "as/kernel/internal/feedback_loop"
	jsontransport "as/kernel/internal/http/json"
	"as/kernel/internal/http/server"
	"as/kernel/internal/interactions"
	"as/kernel/internal/materializer"
	"as/kernel/internal/realizations"
	runtimedb "as/kernel/internal/runtime"
)

var materializationTemplate = template.Must(template.New("materialization").Parse(`
{{if .NotFound}}
<div class="empty">No realization matched <code>{{.Reference}}</code>.</div>
{{else}}
<div class="stack">
  <div class="row">
    <div>
      <div class="row" style="justify-content:flex-start;">
        <div class="status {{.Result.Status}}">{{.Result.Status}}</div>
        {{if .Result.Local}}<div class="readiness {{.Result.Local.Readiness.Stage}}">{{.Result.Local.Readiness.Label}}</div>{{end}}
      </div>
      <h2 style="margin:0.6rem 0 0.15rem;font-size:1.2rem;">{{.Result.Reference}}</h2>
      <p class="empty">{{or .Result.Summary "No summary recorded yet."}}</p>
      {{if .Result.Local}}<p class="subtle" style="margin-top:0.35rem;">{{.Result.Local.Readiness.Summary}}</p>{{end}}
    </div>
    <div class="subtle">
      <div>generated {{.Result.GeneratedAt.Format "2006-01-02 15:04:05 MST"}}</div>
      <div>persisted {{.Result.PersistedPath}}</div>
    </div>
  </div>

  <div class="meta">
    {{if .Result.Sources.Local}}<span class="pill">repo</span>{{end}}
    {{if .Result.Sources.Remote}}<span class="pill">remote</span>{{end}}
    {{if .Result.ApproachID}}<span class="pill">{{.Result.ApproachID}}</span>{{end}}
  </div>

  {{if .Result.Warnings}}
  <div class="source">
    {{range .Result.Warnings}}<div class="subtle">{{.}}</div>{{end}}
  </div>
  {{end}}

  {{if .Result.Local}}
  <article class="source">
    <h3>Local</h3>
    <div class="pathline">{{.Result.Local.RootDir}}</div>
    {{range .Result.Local.Files}}
    <section style="margin-top:0.8rem;">
      <div class="pathline">{{.Kind}} :: {{.Path}}</div>
      <pre>{{.Preview}}</pre>
    </section>
    {{end}}
  </article>
  {{end}}

  {{if .Result.Remote}}
  <article class="source">
    <h3>Remote</h3>
    <div class="pathline">{{.Result.Remote.RegistryURL}}</div>
    {{if .Result.Remote.Files}}
    {{range .Result.Remote.Files}}
    <section style="margin-top:0.8rem;">
      <div class="pathline">{{.Kind}} :: {{.Path}}</div>
      <pre>{{.Preview}}</pre>
    </section>
    {{end}}
    {{else}}
    <p class="empty" style="margin-top:0.8rem;">Remote registry returned summary metadata without file previews.</p>
    {{end}}
  </article>
  {{end}}
</div>
{{end}}
`))

var growthTemplate = template.Must(template.New("growth").Parse(`
<div class="stack">
  <div class="row">
    <div>
      <div class="row" style="justify-content:flex-start;">
        <div class="readiness {{.Packet.Readiness.Stage}}">{{.Packet.Readiness.Label}}</div>
        <div class="status {{.Packet.Status}}">{{.Packet.Status}}</div>
      </div>
      <h2 style="margin:0.6rem 0 0.15rem;font-size:1.2rem;">Grow {{.Packet.Reference}}</h2>
      <p class="empty">{{or .Packet.ContractSummary .Packet.Summary}}</p>
      <p class="subtle" style="margin-top:0.35rem;">{{.Packet.Readiness.Summary}}</p>
    </div>
    <div class="subtle">
      <div>{{.Packet.SeedID}}</div>
      {{if .Packet.SeedSummary}}<div>{{.Packet.SeedSummary}}</div>{{end}}
    </div>
  </div>

  <form class="stack" data-growth-form data-reference="{{.Packet.Reference}}">
    <div class="form-grid two-up">
      <div class="field">
        <label for="growth-operation">Operation</label>
        <select id="growth-operation" name="operation">
          <option value="grow">Grow</option>
          <option value="tweak">Tweak</option>
          <option value="validate">Validate</option>
        </select>
      </div>
      <div class="field">
        <label for="growth-profile">Style Profile</label>
        <select id="growth-profile" name="profile">
          <option value="balanced">Balanced</option>
          <option value="minimal">Minimal</option>
          <option value="ornate">Ornate</option>
          <option value="custom">Custom</option>
        </select>
      </div>
    </div>

    <div class="form-grid two-up">
      <div class="field">
        <label for="growth-target">Target</label>
        <select id="growth-target" name="target">
          <option value="runnable_mvp">Runnable MVP</option>
          <option value="api_first">API First</option>
          <option value="ux_surface">UX Surface</option>
          <option value="validation_only">Validation Only</option>
        </select>
      </div>
      <div class="field">
        <label>Target Realization</label>
        <div class="checkbox-row">
          <input id="growth-create-new" type="checkbox" name="create_new" data-toggle-create-new>
          <label for="growth-create-new" style="text-transform:none;letter-spacing:0;color:#4b5563;">Create a new realization variant from this seed</label>
        </div>
      </div>
    </div>

    <div class="form-grid two-up" data-new-realization-fields style="display:none;">
      <div class="field">
        <label for="growth-new-id">New Realization ID</label>
        <input id="growth-new-id" type="text" name="new_realization_id" placeholder="b-minimal-pass">
      </div>
      <div class="field">
        <label for="growth-new-summary">New Realization Summary</label>
        <input id="growth-new-summary" type="text" name="new_summary" placeholder="Short description of the variant">
      </div>
    </div>

    <div class="field">
      <label for="growth-instructions">Developer Instructions</label>
      <textarea id="growth-instructions" name="developer_instructions" placeholder="Bias toward a server-rendered MVP. Keep runtime capabilities generic. Prefer smaller coherent steps."></textarea>
    </div>

    <div class="action-row">
      <button class="action-button is-primary" type="submit">Queue Growth Job</button>
      <button class="action-button" type="button" data-action="inspect" data-reference="{{.Packet.Reference}}" data-label="{{.Packet.Summary}}">Inspect Instead</button>
      {{if .Packet.Readiness.CanRun}}
      <button class="action-button" type="button" data-action="run" data-reference="{{.Packet.Reference}}" data-label="{{.Packet.Summary}}">Show Run</button>
      {{end}}
    </div>
  </form>

  <div class="source">
    <h3>Seed Packet</h3>
    <p class="subtle">This is the packet a future worker or agent should consume before making the next realization pass.</p>
    <div class="doc-grid">
      {{range .Packet.SeedDocs}}
      <details class="doc">
        <summary>{{.Kind}} <span class="pathline">{{.Path}}</span></summary>
        <pre>{{.Preview}}</pre>
      </details>
      {{end}}
      {{range .Packet.ApproachDocs}}
      <details class="doc">
        <summary>{{.Kind}} <span class="pathline">{{.Path}}</span></summary>
        <pre>{{.Preview}}</pre>
      </details>
      {{end}}
      {{range .Packet.RealizationDocs}}
      <details class="doc">
        <summary>{{.Kind}} <span class="pathline">{{.Path}}</span></summary>
        <pre>{{.Preview}}</pre>
      </details>
      {{end}}
      {{range .Packet.RuntimeDocs}}
      <details class="doc">
        <summary>{{.Kind}} <span class="pathline">{{.Path}}</span></summary>
        <pre>{{.Preview}}</pre>
      </details>
      {{end}}
    </div>
  </div>
</div>
`))

var runTemplate = template.Must(template.New("run").Parse(`
<div class="stack">
  <div class="row">
    <div>
      {{if .Packet.Readiness.CanRun}}
      <div class="readiness runnable">Runnable</div>
      {{else}}
      <div class="readiness designed">Not Runnable</div>
      {{end}}
      <h2 style="margin:0.6rem 0 0.15rem;font-size:1.2rem;">Run {{.Packet.Reference}}</h2>
      {{if .Packet.Readiness.CanRun}}
      <p class="empty">This realization carries a runtime manifest. Treat this panel as the launch recipe until the kernel grows a first-class process manager.</p>
      {{else}}
      <p class="empty">This realization does not yet carry a runtime artifact. Use <strong>Grow</strong> to move it toward a runnable state.</p>
      {{end}}
    </div>
  </div>

  {{if .Packet.Readiness.CanRun}}
  <div class="source">
    <h3>Runtime Instructions</h3>
    <p class="pathline">{{.Packet.Readiness.RuntimeFile}}</p>
    {{range .Packet.RuntimeDocs}}
    <pre>{{.Preview}}</pre>
    {{end}}
  </div>
  {{end}}

  <div class="action-row">
    <button class="action-button" type="button" data-action="grow" data-reference="{{.Packet.Reference}}" data-label="{{.Packet.Summary}}">Grow</button>
    <button class="action-button" type="button" data-action="inspect" data-reference="{{.Packet.Reference}}" data-label="{{.Packet.Summary}}">Inspect</button>
  </div>
</div>
`))

type partialView struct {
	Reference string
	Result    materializer.Materialization
	NotFound  bool
}

type growthView struct {
	Packet realizations.GrowthContext
}

type runView struct {
	Packet realizations.GrowthContext
}

func main() {
	repoRoot, err := config.RepoRootFromEnvOrWD()
	if err != nil {
		log.Fatal(err)
	}

	store := feedbackloop.NewMemoryStore()
	service, err := materializer.NewService(repoRoot, remoteClient())
	if err != nil {
		log.Fatal(err)
	}

	var runtimeService *interactions.RuntimeService
	runtimeConfig := config.LoadRuntimeConfigFromEnv()
	if runtimeConfig.Enabled() {
		pool, err := runtimedb.OpenPool(context.Background(), runtimeConfig.DatabaseURL)
		if err != nil {
			log.Fatal(err)
		}
		defer pool.Close()
		if runtimeConfig.AutoMigrate {
			if err := runtimedb.RunMigrations(context.Background(), pool, repoRoot); err != nil {
				log.Fatal(err)
			}
		}
		runtimeService = interactions.NewRuntimeService(pool)
	}

	mux := http.NewServeMux()
	mux.Handle("POST /feedback/incidents", jsontransport.NewIncidentIngestHandler(store))
	mux.Handle("GET /assets/", sproutAssetHandler())
	jsontransport.NewGrowthAPI(repoRoot, runtimeService).Register(mux)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		options, err := service.ListRealizations(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		requestMeta := server.RequestMetadataFromContext(r.Context())
		view := newBootPageView(options, service.Remote != nil, runtimeService != nil, server.CSPNonceFromContext(r.Context()), boot.ClientFeedbackLoopScript(boot.FeedbackLoopScriptConfig{
			EndpointPath: "/feedback/incidents",
			Request:      requestMeta,
			Selection: boot.PinnedSelection{
				RealizationID: requestMeta.RealizationID,
				SeedID:        requestMeta.SeedID,
			},
			IncludeConsoleErrors: true,
			IncludeHTMX:          true,
		}))

		var body bytes.Buffer
		if err := bootPageTemplate.Execute(&body, view); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = body.WriteTo(w)
	})
	mux.HandleFunc("GET /partials/materialization", func(w http.ResponseWriter, r *http.Request) {
		reference := strings.TrimSpace(r.URL.Query().Get("reference"))
		result, err := service.Materialize(r.Context(), reference)
		view := partialView{Reference: reference, Result: result}
		if err != nil {
			if errors.Is(err, materializer.ErrReferenceNotFound) {
				view.NotFound = true
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := materializationTemplate.Execute(w, view); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
	mux.HandleFunc("GET /partials/grow", func(w http.ResponseWriter, r *http.Request) {
		reference := strings.TrimSpace(r.URL.Query().Get("reference"))
		packet, err := realizations.LoadGrowthContext(repoRoot, reference)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := growthTemplate.Execute(w, growthView{Packet: packet}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
	mux.HandleFunc("GET /partials/run", func(w http.ResponseWriter, r *http.Request) {
		reference := strings.TrimSpace(r.URL.Query().Get("reference"))
		packet, err := realizations.LoadGrowthContext(repoRoot, reference)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := runTemplate.Execute(w, runView{Packet: packet}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	handler := http.Handler(mux)
	if runtimeService != nil {
		handler = server.SessionResolutionMiddleware(server.RuntimeSessionResolver{Lookup: runtimeService}, handler)
	}

	addr := config.EnvOrDefault("AS_WEBD_ADDR", "127.0.0.1:8090")
	log.Printf("webd listening on %s (repo root %s)", addr, repoRoot)
	if err := http.ListenAndServe(addr, server.DefaultMiddlewareStack(handler)); err != nil {
		log.Fatal(err)
	}
}

func remoteClient() *materializer.RemoteRegistryClient {
	baseURL := strings.TrimSpace(config.EnvOrDefault("AS_REMOTE_REGISTRY_URL", ""))
	if baseURL == "" {
		return nil
	}

	return &materializer.RemoteRegistryClient{BaseURL: baseURL}
}
