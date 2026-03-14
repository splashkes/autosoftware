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

var pageTemplate = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>AS Kernel Loader</title>
  <style nonce="{{.CSPNonce}}">
    :root {
      --bg: #0b0d12;
      --panel: #11141b;
      --panel-2: #151922;
      --line: #242a36;
      --text: #f3f6fb;
      --muted: #8e98ab;
      --accent: #7dd3fc;
      --accent-2: #34d399;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
      color: var(--text);
      background:
        radial-gradient(circle at top, rgba(125, 211, 252, 0.08), transparent 30rem),
        linear-gradient(180deg, #090b10 0%, #0b0d12 100%);
      min-height: 100vh;
      display: grid;
      place-items: center;
    }
    main {
      width: min(44rem, calc(100vw - 1.5rem));
      padding: 1.25rem 0;
    }
    .shell {
      display: grid;
      gap: 1rem;
      padding: 1.25rem;
      border: 1px solid var(--line);
      background: linear-gradient(180deg, rgba(17, 20, 27, 0.96), rgba(12, 15, 21, 0.96));
      border-radius: 1rem;
    }
    .eyebrow {
      font-size: 0.78rem;
      letter-spacing: 0.12em;
      text-transform: uppercase;
      color: var(--muted);
    }
    h1 {
      margin: 0;
      font-size: clamp(1.4rem, 4vw, 2rem);
      line-height: 1.08;
    }
    .lede {
      margin: 0;
      font-size: 0.92rem;
      color: var(--muted);
    }
    .controls {
      display: grid;
      grid-template-columns: minmax(0, 1fr) auto;
      gap: 0.75rem;
      align-items: center;
    }
    select, button {
      width: 100%;
      border-radius: 0.8rem;
      border: 1px solid var(--line);
      padding: 0.85rem 0.95rem;
      font: inherit;
      background: var(--panel-2);
      color: var(--text);
    }
    button {
      width: auto;
      white-space: nowrap;
      background: var(--text);
      color: var(--bg);
      cursor: pointer;
    }
    .meta {
      display: flex;
      flex-wrap: wrap;
      gap: 0.5rem;
    }
    .pill {
      padding: 0.2rem 0.5rem;
      border-radius: 999px;
      border: 1px solid var(--line);
      color: var(--muted);
      font-size: 0.72rem;
    }
    .content {
      min-height: 16rem;
      border: 1px solid var(--line);
      border-radius: 1rem;
      background: rgba(10, 12, 17, 0.95);
      padding: 1rem;
    }
    .status {
      display: inline-flex;
      align-items: center;
      gap: 0.4rem;
      padding: 0.2rem 0.55rem;
      border-radius: 999px;
      border: 1px solid var(--line);
      font-size: 0.72rem;
      text-transform: uppercase;
      color: var(--accent-2);
    }
    .empty {
      color: var(--muted);
      margin: 0;
      font-size: 0.9rem;
    }
    .footer {
      margin: 0;
      color: var(--muted);
      font-size: 0.78rem;
    }
    pre {
      margin: 0.4rem 0 0;
      padding: 0.75rem;
      white-space: pre-wrap;
      border-radius: 0.75rem;
      border: 1px solid var(--line);
      background: #0d1016;
      color: #d7deea;
      font-size: 0.82rem;
      line-height: 1.45;
    }
    .stack {
      display: grid;
      gap: 0.85rem;
    }
    .row {
      display: flex;
      gap: 0.75rem;
      align-items: center;
      justify-content: space-between;
      flex-wrap: wrap;
    }
    .subtle {
      color: var(--muted);
      font-size: 0.8rem;
    }
    .source {
      border-top: 1px solid var(--line);
      padding-top: 0.85rem;
    }
    .source h3 {
      margin: 0 0 0.25rem;
      font-size: 0.9rem;
    }
    .pathline {
      color: var(--muted);
      font-size: 0.75rem;
    }
    @media (max-width: 720px) {
      .controls { grid-template-columns: 1fr; }
      button { width: 100%; }
    }
  </style>
</head>
<body>
  <main>
    <section class="shell">
      <div class="eyebrow">AS Kernel Boot</div>
      <h1>Choose a realization. Click once. Watch it boot.</h1>
      <p class="lede">Local manifests only unless <code>AS_REMOTE_REGISTRY_URL</code> is set.</p>
      <form id="loader-form" class="stack">
        <div class="controls">
          <select id="realization-select" name="reference" aria-label="Realization reference">
            {{range .Options}}
            <option value="{{.Reference}}" {{if eq .Reference $.DefaultReference}}selected{{end}}>
              {{.Reference}} :: {{or .Summary "Untitled"}}
            </option>
            {{end}}
          </select>
          <button
            id="boot-button"
            type="submit"
          >Boot</button>
        </div>
        <div class="meta">
          <span class="pill">{{len .Options}} options</span>
          {{if .RemoteConfigured}}<span class="pill">remote on</span>{{else}}<span class="pill">remote off</span>{{end}}
        </div>
      </form>
      <section id="materialization" class="content">
        <div id="loader-indicator" class="stack" style="min-height:100%;align-content:center;">
          <div class="eyebrow">Paused</div>
          <p class="empty">Boot sequence is waiting for your click.</p>
        </div>
      </section>
      <p class="footer">Materialization persists into <code>materialized/</code> after boot.</p>
    </section>
  </main>
  <script nonce="{{.CSPNonce}}">{{.LoaderScript}}</script>
  <script nonce="{{.CSPNonce}}">{{.FeedbackScript}}</script>
</body>
</html>`))

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

type pageView struct {
	Options          []materializer.RealizationOption
	DefaultReference string
	RemoteConfigured bool
	CSPNonce         string
	LoaderScript     template.JS
	FeedbackScript   template.JS
}

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
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		options, err := service.ListRealizations(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		requestMeta := server.RequestMetadataFromContext(r.Context())
		view := pageView{
			Options:          options,
			DefaultReference: defaultReference(options),
			RemoteConfigured: service.Remote != nil,
			CSPNonce:         server.CSPNonceFromContext(r.Context()),
			LoaderScript:     template.JS(loaderScript()),
			FeedbackScript: template.JS(boot.ClientFeedbackLoopScript(boot.FeedbackLoopScriptConfig{
				EndpointPath: "/feedback/incidents",
				Request:      requestMeta,
				Selection: boot.PinnedSelection{
					RealizationID: requestMeta.RealizationID,
					SeedID:        requestMeta.SeedID,
				},
				IncludeConsoleErrors: true,
				IncludeHTMX:          true,
			})),
		}

		var body bytes.Buffer
		if err := pageTemplate.Execute(&body, view); err != nil {
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

func defaultReference(options []materializer.RealizationOption) string {
	if len(options) == 0 {
		return ""
	}
	return options[0].Reference
}

func remoteClient() *materializer.RemoteRegistryClient {
	baseURL := strings.TrimSpace(config.EnvOrDefault("AS_REMOTE_REGISTRY_URL", ""))
	if baseURL == "" {
		return nil
	}

	return &materializer.RemoteRegistryClient{BaseURL: baseURL}
}

func loaderScript() string {
	return `(function () {
  var form = document.getElementById("loader-form");
  var button = document.getElementById("boot-button");
  var target = document.getElementById("materialization");
  var indicator = document.getElementById("loader-indicator");
  if (!form || !button || !target || !indicator) {
    return;
  }

  function escapeHTML(value) {
    return String(value).replace(/[&<>"]/g, function (char) {
      return {
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        "\"": "&quot;"
      }[char];
    });
  }

  function setLoading() {
    indicator.innerHTML = [
      '<div class="eyebrow">Booting</div>',
      '<p class="empty">Materializing the selected realization.</p>'
    ].join("");
    target.replaceChildren(indicator);
  }

  async function boot() {
    var params = new URLSearchParams(new FormData(form));
    button.disabled = true;
    setLoading();

    try {
      var response = await fetch("/partials/materialization?" + params.toString(), {
        method: "GET",
        credentials: "same-origin",
        headers: { "Accept": "text/html" }
      });
      var html = await response.text();
      if (!response.ok) {
        throw new Error(html || ("Boot failed with status " + response.status));
      }
      target.innerHTML = html;
    } catch (err) {
      target.innerHTML = [
        '<div class="stack">',
        '  <div class="eyebrow">Boot Failed</div>',
        '  <p class="empty">The kernel could not materialize that realization.</p>',
        '  <pre>' + escapeHTML(err && err.message ? err.message : err) + '</pre>',
        '</div>'
      ].join("\n");
      console.error(err);
    } finally {
      button.disabled = false;
    }
  }

  form.addEventListener("submit", function (event) {
    event.preventDefault();
    boot();
  });
})();`
}
