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
	runtimedb "as/kernel/internal/runtime"
)

var materializationTemplate = template.Must(template.New("materialization").Parse(`
{{if .NotFound}}
<div class="empty">No realization matched <code>{{.Reference}}</code>.</div>
{{else}}
<div class="stack">
  <div class="row">
    <div>
      <div class="status">{{.Result.Status}}</div>
      <h2 style="margin:0.6rem 0 0.15rem;font-size:1.2rem;">{{.Result.Reference}}</h2>
      <p class="empty">{{or .Result.Summary "No summary recorded yet."}}</p>
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

type partialView struct {
	Reference string
	Result    materializer.Materialization
	NotFound  bool
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
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		options, err := service.ListRealizations(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		requestMeta := server.RequestMetadataFromContext(r.Context())
		view := newBootPageView(options, service.Remote != nil, server.CSPNonceFromContext(r.Context()), boot.ClientFeedbackLoopScript(boot.FeedbackLoopScriptConfig{
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

	handler := http.Handler(mux)
	if runtimeService != nil {
		handler = server.SessionResolutionMiddleware(server.RuntimeSessionResolver{Lookup: runtimeService}, handler)
	}

	addr := config.EnvOrDefault("AS_WEBD_ADDR", ":8090")
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
