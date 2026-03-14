package main

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"unicode"

	"as/kernel/internal/materializer"
)

//go:embed assets/sprout-logo.css assets/sprout-logo.js
var bootloaderAssets embed.FS

var bootPageTemplate = template.Must(template.New("boot-page").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>AS Growth Console</title>
  <link rel="stylesheet" href="/assets/sprout-logo.css">
  <style nonce="{{.CSPNonce}}">
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
      background:
        radial-gradient(circle at top, rgba(34, 197, 94, 0.08), transparent 28rem),
        linear-gradient(180deg, #eceef2 0%, #e7eaee 100%);
      color: #20242c;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
      display: flex;
      align-items: center;
      justify-content: center;
    }
    .page {
      width: min(58rem, calc(100vw - 2rem));
      padding: 1.25rem 0 2rem;
    }
    .brand {
      display: grid;
      justify-items: center;
      text-align: center;
      gap: 0.4rem;
      margin-bottom: 1rem;
    }
    .wordmark {
      font-size: 2rem;
      font-weight: 700;
      letter-spacing: 0.52rem;
      color: #181c24;
      padding-left: 0.52rem;
    }
    .tagline {
      font-size: 0.72rem;
      letter-spacing: 0.22rem;
      text-transform: uppercase;
      color: #7e8592;
      padding-left: 0.22rem;
    }
    .lede {
      margin: 0.4rem auto 0;
      max-width: 32rem;
      color: #69707c;
      font-size: 0.82rem;
      line-height: 1.6;
    }
    .console-meta {
      display: flex;
      justify-content: center;
      gap: 0.5rem;
      flex-wrap: wrap;
      margin: 1rem 0 1.25rem;
    }
    .pill {
      display: inline-flex;
      align-items: center;
      gap: 0.35rem;
      padding: 0.24rem 0.62rem;
      border-radius: 999px;
      border: 1px solid #c8cdd6;
      background: rgba(255, 255, 255, 0.7);
      color: #6c7380;
      font-size: 0.7rem;
      letter-spacing: 0.04em;
      text-transform: uppercase;
    }
    .layout {
      display: grid;
      grid-template-columns: minmax(0, 26rem) minmax(0, 1fr);
      gap: 1rem;
      align-items: start;
    }
    .shell {
      border: 1px solid #cfd4dc;
      background: rgba(245, 246, 248, 0.92);
      box-shadow: 0 1rem 2.5rem rgba(28, 35, 48, 0.08);
    }
    .seed {
      border-bottom: 1px solid #d4d8df;
    }
    .seed:last-of-type {
      border-bottom: none;
    }
    .seed-head {
      width: 100%;
      display: flex;
      align-items: flex-start;
      gap: 0.9rem;
      padding: 1rem 1.15rem;
      border: none;
      background: transparent;
      cursor: pointer;
      text-align: left;
    }
    .seed-count {
      min-width: 2rem;
      font-size: 0.76rem;
      color: #8d94a0;
      text-align: right;
      flex-shrink: 0;
      padding-top: 0.2rem;
    }
    .seed-copy {
      flex: 1;
      min-width: 0;
    }
    .seed-name {
      display: block;
      color: #222730;
      font-size: 0.92rem;
      font-weight: 600;
    }
    .seed-id {
      display: block;
      margin-top: 0.16rem;
      color: #88909c;
      font-size: 0.72rem;
      letter-spacing: 0.04em;
      text-transform: uppercase;
    }
    .seed-summary {
      margin: 0.32rem 0 0;
      color: #69707c;
      font-size: 0.78rem;
      line-height: 1.55;
    }
    .seed-meta {
      display: flex;
      gap: 0.35rem;
      flex-wrap: wrap;
      margin-top: 0.45rem;
    }
    .seed-arrow {
      color: #a4aab5;
      font-size: 1rem;
      transition: transform 0.18s ease;
      flex-shrink: 0;
      padding-top: 0.25rem;
    }
    .seed.open .seed-arrow {
      transform: rotate(90deg);
    }
    .seed-body {
      display: none;
      padding: 0 1.15rem 1rem 4rem;
      gap: 0.75rem;
    }
    .seed.open .seed-body {
      display: grid;
    }
    .realization {
      display: grid;
      gap: 0.45rem;
      padding: 0.65rem 0 0.75rem;
      border-bottom: 1px dashed #d8dde4;
    }
    .realization:last-child {
      border-bottom: none;
      padding-bottom: 0;
    }
    .realization-copy {
      min-width: 0;
    }
    .realization-title {
      display: block;
      color: #3a4250;
      font-size: 0.82rem;
      line-height: 1.45;
    }
    .realization-meta,
    .realization-flags {
      display: flex;
      gap: 0.42rem;
      flex-wrap: wrap;
      color: #959ca7;
      font-size: 0.64rem;
      text-transform: uppercase;
      letter-spacing: 0.05em;
    }
    .realization-flags {
      margin-top: 0.08rem;
    }
    .status,
    .readiness {
      display: inline-flex;
      align-items: center;
      gap: 0.35rem;
      padding: 0.18rem 0.48rem;
      border-radius: 999px;
      border: 1px solid #cbd1da;
      background: rgba(255, 255, 255, 0.78);
      line-height: 1;
    }
    .status {
      color: #2f855a;
    }
    .status.draft {
      color: #9a6700;
    }
    .status.published,
    .status.accepted {
      color: #15803d;
    }
    .status.failed,
    .status.error {
      color: #b91c1c;
    }
    .readiness.defined {
      color: #1d4ed8;
    }
    .readiness.runnable,
    .readiness.accepted {
      color: #15803d;
    }
    .readiness.bootstrap {
      color: #7c3aed;
    }
    .readiness.designed {
      color: #9a6700;
    }
    .realization-summary {
      margin: 0;
      color: #69707c;
      font-size: 0.75rem;
      line-height: 1.55;
    }
    .action-row {
      display: flex;
      gap: 0.45rem;
      flex-wrap: wrap;
    }
    .action-button {
      padding: 0.28rem 0.7rem;
      border: 1px solid #c8ccd4;
      background: transparent;
      color: #616875;
      font: inherit;
      font-size: 0.68rem;
      letter-spacing: 0.06em;
      text-transform: uppercase;
      cursor: pointer;
    }
    .action-button:hover,
    .action-button:focus-visible {
      border-color: #22a05a;
      color: #178243;
      outline: none;
    }
    .action-button.is-primary {
      border-color: #22a05a;
      color: #178243;
      background: rgba(34, 160, 90, 0.08);
    }
    .action-button[disabled] {
      cursor: default;
      opacity: 0.45;
      border-color: #d5d8de;
      color: #9aa1ac;
    }
    .panel {
      min-height: 18rem;
      border: 1px solid #d0d5dd;
      background: rgba(255, 255, 255, 0.62);
      padding: 1.1rem;
      box-shadow: 0 1rem 2.5rem rgba(28, 35, 48, 0.08);
    }
    .indicator {
      display: grid;
      align-content: center;
      min-height: 100%;
      gap: 0.45rem;
    }
    .indicator-title {
      font-size: 0.72rem;
      color: #7a818d;
      letter-spacing: 0.12em;
      text-transform: uppercase;
    }
    .indicator-copy {
      margin: 0;
      color: #69707c;
      font-size: 0.82rem;
      line-height: 1.6;
    }
    .empty {
      margin: 0;
      color: #69707c;
      font-size: 0.82rem;
      line-height: 1.6;
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
    .meta {
      display: flex;
      gap: 0.45rem;
      flex-wrap: wrap;
    }
    .subtle {
      color: #7a818d;
      font-size: 0.76rem;
      line-height: 1.5;
    }
    .source {
      border-top: 1px solid #d4d8df;
      padding-top: 0.85rem;
    }
    .source h3 {
      margin: 0 0 0.25rem;
      font-size: 0.9rem;
      color: #222730;
    }
    .pathline {
      color: #848b96;
      font-size: 0.74rem;
      line-height: 1.5;
      word-break: break-word;
    }
    .form-grid {
      display: grid;
      gap: 0.75rem;
    }
    .form-grid.two-up {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
    .field {
      display: grid;
      gap: 0.32rem;
    }
    .field label {
      color: #4b5563;
      font-size: 0.74rem;
      letter-spacing: 0.04em;
      text-transform: uppercase;
    }
    input[type="text"],
    select,
    textarea {
      width: 100%;
      border: 1px solid #cfd4dc;
      background: rgba(255, 255, 255, 0.9);
      color: #222730;
      font: inherit;
      padding: 0.62rem 0.7rem;
      border-radius: 0;
    }
    textarea {
      min-height: 7rem;
      resize: vertical;
      line-height: 1.55;
    }
    .checkbox-row {
      display: flex;
      align-items: center;
      gap: 0.55rem;
      font-size: 0.78rem;
      color: #4b5563;
    }
    .checkbox-row input {
      margin: 0;
    }
    .doc-grid {
      display: grid;
      gap: 0.65rem;
    }
    details.doc {
      border: 1px solid #d7dbe2;
      background: rgba(255, 255, 255, 0.82);
      padding: 0.7rem 0.8rem;
    }
    details.doc summary {
      cursor: pointer;
      color: #374151;
      font-size: 0.78rem;
      font-weight: 600;
      list-style: none;
    }
    details.doc summary::-webkit-details-marker {
      display: none;
    }
    details.doc summary::after {
      content: "open";
      float: right;
      color: #9aa1ac;
      font-size: 0.62rem;
      letter-spacing: 0.05em;
      text-transform: uppercase;
    }
    details.doc[open] summary::after {
      content: "close";
    }
    pre {
      margin: 0.5rem 0 0;
      padding: 0.75rem;
      white-space: pre-wrap;
      border: 1px solid #d0d5dd;
      background: rgba(240, 243, 247, 0.92);
      color: #303744;
      font-size: 0.78rem;
      line-height: 1.5;
      overflow-x: auto;
    }
    .job-list {
      margin: 0;
      padding-left: 1.1rem;
      color: #4b5563;
      font-size: 0.78rem;
      line-height: 1.6;
    }
    #console-status {
      margin-top: 0.9rem;
      text-align: center;
      color: #7b828f;
      font-size: 0.76rem;
      min-height: 1.2rem;
    }
    .footer {
      margin-top: 1rem;
      text-align: center;
      color: #848b96;
      font-size: 0.72rem;
      line-height: 1.6;
    }
    .footer code {
      color: #4f5664;
    }
    @media (max-width: 980px) {
      .layout {
        grid-template-columns: 1fr;
      }
    }
    @media (max-width: 720px) {
      .page {
        width: min(58rem, calc(100vw - 1rem));
      }
      .seed-body {
        padding-left: 1.15rem;
      }
      .form-grid.two-up {
        grid-template-columns: 1fr;
      }
    }
  </style>
</head>
<body>
  <main class="page">
    <section class="brand">
      <div class="sprout-logo-shell" data-sprout-logo aria-hidden="true"></div>
      <div class="wordmark">AS</div>
      <div class="tagline">Growth Console</div>
      <p class="lede">Software that evolves from within.</p>
      <p class="lede">Built to scale securely, share data across apps, and give agents and humans a common surface to inspect, grow, validate, and run software.</p>
    </section>

    <div class="console-meta">
      <span class="pill">{{len .Seeds}} seeds</span>
      <span class="pill">{{.RealizationCount}} realizations</span>
      <span class="pill">{{.GrowthReadyCount}} growth-ready</span>
      <span class="pill">{{.RunnableCount}} runnable</span>
      {{if .RuntimeConfigured}}<span class="pill">runtime on</span>{{else}}<span class="pill">runtime off</span>{{end}}
      {{if .RemoteConfigured}}<span class="pill">remote on</span>{{else}}<span class="pill">remote off</span>{{end}}
    </div>

    <div class="layout">
      <section class="shell">
        {{range .Seeds}}
        <section class="seed{{if .InitiallyOpen}} open{{end}}">
          <button class="seed-head" type="button" data-seed-toggle>
            <span class="seed-count">{{.Count}}</span>
            <span class="seed-copy">
              <span class="seed-name">{{.DisplayName}}</span>
              <span class="seed-id">{{.SeedID}}</span>
              {{if .Summary}}<p class="seed-summary">{{.Summary}}</p>{{end}}
              <span class="seed-meta">
                {{if .Status}}<span class="pill">{{.Status}}</span>{{end}}
                {{if gt .GrowthReadyCount 0}}<span class="pill">{{.GrowthReadyCount}} growth-ready</span>{{end}}
                {{if gt .RunnableCount 0}}<span class="pill">{{.RunnableCount}} runnable</span>{{end}}
              </span>
            </span>
            <span class="seed-arrow">&#8250;</span>
          </button>
          <div class="seed-body">
            {{range .Realizations}}
            <div class="realization">
              <div class="realization-copy">
                <span class="realization-title">{{.Summary}}</span>
                <span class="realization-meta">
                  <span class="status {{.Status}}">{{.Status}}</span>
                  <span class="readiness {{.ReadinessStage}}">{{.ReadinessLabel}}</span>
                  {{if .SurfaceKind}}<span>{{.SurfaceKind}}</span>{{end}}
                  {{if .ApproachID}}<span>{{.ApproachID}}</span>{{end}}
                  <span>{{.Reference}}</span>
                </span>
                <span class="realization-flags">
                  {{if .HasContract}}<span>contract</span>{{end}}
                  {{if .HasRuntime}}<span>runtime</span>{{end}}
                </span>
                <p class="realization-summary">{{.ReadinessSummary}}</p>
              </div>
              <div class="action-row">
                <button class="action-button" type="button" data-action="inspect" data-reference="{{.Reference}}" data-label="{{.Summary}}">Inspect</button>
                <button class="action-button is-primary" type="button" data-action="grow" data-reference="{{.Reference}}" data-label="{{.Summary}}">Grow</button>
                {{if .CanRun}}
                <button class="action-button" type="button" data-action="run" data-reference="{{.Reference}}" data-label="{{.Summary}}">Run</button>
                {{else}}
                <button class="action-button" type="button" disabled>Run</button>
                {{end}}
              </div>
            </div>
            {{end}}
          </div>
        </section>
        {{end}}
      </section>

      <section id="console-panel" class="panel">
        <div id="loader-indicator" class="indicator">
          <div class="indicator-title">Paused</div>
          <p class="indicator-copy">Inspect the current seed packet, queue a growth pass, or open run instructions for realizations that already carry runtime artifacts.</p>
        </div>
      </section>
    </div>

    <div id="console-status"></div>
    <p class="footer">Draft realizations materialize into <code>materialized/</code> for inspection. Growth requests enqueue agent-ready jobs in <code>runtime_jobs</code>.</p>

    <script src="/assets/sprout-logo.js"></script>
    <script nonce="{{.CSPNonce}}">{{.LoaderScript}}</script>
    <script nonce="{{.CSPNonce}}">{{.FeedbackScript}}</script>
  </main>
</body>
</html>`))

type bootPageView struct {
	Seeds             []seedBootView
	RealizationCount  int
	GrowthReadyCount  int
	RunnableCount     int
	RemoteConfigured  bool
	RuntimeConfigured bool
	CSPNonce          string
	LoaderScript      template.JS
	FeedbackScript    template.JS
}

type seedBootView struct {
	SeedID           string
	DisplayName      string
	Summary          string
	Status           string
	Count            int
	GrowthReadyCount int
	RunnableCount    int
	InitiallyOpen    bool
	Realizations     []realizationBootView
}

type realizationBootView struct {
	Reference        string
	RealizationID    string
	ApproachID       string
	Summary          string
	Status           string
	SurfaceKind      string
	ReadinessStage   string
	ReadinessLabel   string
	ReadinessSummary string
	HasContract      bool
	HasRuntime       bool
	CanRun           bool
}

func newBootPageView(options []materializer.RealizationOption, remoteConfigured, runtimeConfigured bool, nonce string, feedbackScript string) bootPageView {
	seen := make(map[string]int)
	seeds := make([]seedBootView, 0)
	runnableCount := 0
	growthReadyCount := 0

	for _, option := range options {
		index, ok := seen[option.SeedID]
		if !ok {
			index = len(seeds)
			seen[option.SeedID] = index
			seeds = append(seeds, seedBootView{
				SeedID:        option.SeedID,
				DisplayName:   humanizeSeedID(option.SeedID),
				Summary:       strings.TrimSpace(option.SeedSummary),
				Status:        strings.TrimSpace(option.SeedStatus),
				InitiallyOpen: len(seeds) == 0,
			})
		}

		readinessStage := firstNonEmpty(strings.TrimSpace(option.Readiness.Stage), "designed")
		readinessLabel := firstNonEmpty(strings.TrimSpace(option.Readiness.Label), "Designed")
		readinessSummary := firstNonEmpty(strings.TrimSpace(option.Readiness.Summary), "This realization is ready for inspection and growth.")

		item := realizationBootView{
			Reference:        option.Reference,
			RealizationID:    option.RealizationID,
			ApproachID:       option.ApproachID,
			Summary:          firstNonEmpty(strings.TrimSpace(option.Summary), option.RealizationID),
			Status:           firstNonEmpty(strings.TrimSpace(option.Status), "draft"),
			SurfaceKind:      strings.TrimSpace(option.SurfaceKind),
			ReadinessStage:   readinessStage,
			ReadinessLabel:   readinessLabel,
			ReadinessSummary: readinessSummary,
			HasContract:      option.Readiness.HasContract,
			HasRuntime:       option.Readiness.HasRuntime,
			CanRun:           option.Readiness.CanRun,
		}
		seeds[index].Realizations = append(seeds[index].Realizations, item)
		seeds[index].Count = len(seeds[index].Realizations)
		if item.HasContract {
			seeds[index].GrowthReadyCount++
			growthReadyCount++
		}
		if item.CanRun {
			seeds[index].RunnableCount++
			runnableCount++
		}
	}

	return bootPageView{
		Seeds:             seeds,
		RealizationCount:  len(options),
		GrowthReadyCount:  growthReadyCount,
		RunnableCount:     runnableCount,
		RemoteConfigured:  remoteConfigured,
		RuntimeConfigured: runtimeConfigured,
		CSPNonce:          nonce,
		LoaderScript:      template.JS(consoleLoaderScript()),
		FeedbackScript:    template.JS(feedbackScript),
	}
}

func sproutAssetHandler() http.Handler {
	sub, err := fs.Sub(bootloaderAssets, "assets")
	if err != nil {
		panic(err)
	}
	return http.StripPrefix("/assets/", http.FileServer(http.FS(sub)))
}

func humanizeSeedID(seedID string) string {
	trimmed := strings.TrimSpace(seedID)
	if trimmed == "" {
		return "Unnamed Seed"
	}

	parts := strings.SplitN(trimmed, "-", 2)
	label := trimmed
	if len(parts) == 2 && parts[1] != "" {
		label = parts[1]
	}

	words := strings.FieldsFunc(label, func(r rune) bool {
		return r == '-' || r == '_' || unicode.IsSpace(r)
	})
	if len(words) == 0 {
		return trimmed
	}

	for index, word := range words {
		words[index] = capitalizeWord(word)
	}
	return strings.Join(words, " ")
}

func capitalizeWord(word string) string {
	if word == "" {
		return word
	}
	runes := []rune(strings.ToLower(word))
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func consoleLoaderScript() string {
	return `(function () {
  var panel = document.getElementById("console-panel");
  var indicator = document.getElementById("loader-indicator");
  var status = document.getElementById("console-status");
  if (!panel || !indicator || !status) {
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

  function setStatus(copy) {
    status.textContent = copy || "";
  }

  function setLoading(title, copy) {
    indicator.innerHTML = [
      '<div class="indicator-title">' + escapeHTML(title || "Loading") + '</div>',
      '<p class="indicator-copy">' + escapeHTML(copy || "Preparing the selected realization.") + '</p>'
    ].join("");
    panel.replaceChildren(indicator);
  }

  function setError(title, message) {
    panel.innerHTML = [
      '<div class="stack">',
      '  <div class="indicator-title">' + escapeHTML(title || "Request Failed") + '</div>',
      '  <p class="indicator-copy">' + escapeHTML(message || "The console could not complete that request.") + '</p>',
      '</div>'
    ].join("\n");
  }

  async function loadPartial(action, reference, label) {
    var path;
    var loadingTitle;
    var loadingCopy;

    if (action === "inspect") {
      path = "/partials/materialization?reference=" + encodeURIComponent(reference);
      loadingTitle = "Inspecting";
      loadingCopy = "Materializing the current realization snapshot for inspection.";
    } else if (action === "grow") {
      path = "/partials/grow?reference=" + encodeURIComponent(reference);
      loadingTitle = "Preparing Growth";
      loadingCopy = "Loading the seed packet and growth controls for the selected realization.";
    } else if (action === "run") {
      path = "/partials/run?reference=" + encodeURIComponent(reference);
      loadingTitle = "Preparing Run";
      loadingCopy = "Loading runtime instructions for the selected realization.";
    } else {
      return;
    }

    setLoading(loadingTitle, loadingCopy);
    setStatus(loadingTitle + " " + (label || reference) + "...");

    try {
      var response = await fetch(path, {
        method: "GET",
        credentials: "same-origin",
        headers: { "Accept": "text/html" }
      });
      var html = await response.text();
      if (!response.ok) {
        throw new Error(html || ("Request failed with status " + response.status));
      }
      panel.innerHTML = html;
      setStatus((action === "inspect" ? "Inspecting " : action === "grow" ? "Ready to grow " : "Run details loaded for ") + (label || reference) + ".");
    } catch (err) {
      setError("Request Failed", err && err.message ? err.message : String(err));
      setStatus("Request failed.");
      console.error(err);
    }
  }

  function growthPayload(form) {
    return {
      reference: form.getAttribute("data-reference"),
      operation: form.elements.operation.value,
      create_new: form.elements.create_new.checked,
      new_realization_id: form.elements.new_realization_id.value.trim(),
      new_summary: form.elements.new_summary.value.trim(),
      profile: form.elements.profile.value,
      target: form.elements.target.value,
      developer_instructions: form.elements.developer_instructions.value.trim()
    };
  }

  function renderJobResult(result) {
    var job = result.job || {};
    var nextActions = Array.isArray(result.next_actions) ? result.next_actions : [];
    var actionHTML = nextActions.map(function (item) {
      return "<li>" + escapeHTML(item) + "</li>";
    }).join("");

    panel.innerHTML = [
      '<div class="stack">',
      '  <div class="row">',
      '    <div>',
      '      <div class="readiness defined">Queued</div>',
      '      <h2 style="margin:0.55rem 0 0.2rem;font-size:1.15rem;">' + escapeHTML(result.summary || "Realization growth queued") + '</h2>',
      '      <p class="empty">The next AI or developer pass should claim this job from <code>runtime_jobs</code> and follow the prompt brief plus linked seed packet.</p>',
      '    </div>',
      '    <div class="subtle">',
      '      <div>job ' + escapeHTML(job.job_id || "") + '</div>',
      '      <div>status ' + escapeHTML(job.status || "") + '</div>',
      '    </div>',
      '  </div>',
      '  <div class="source">',
      '    <h3>Next Actions</h3>',
      '    <ol class="job-list">' + actionHTML + '</ol>',
      '  </div>',
      (job.payload && job.payload.prompt_brief ? [
        '  <div class="source">',
        '    <h3>Prompt Brief</h3>',
        '    <pre>' + escapeHTML(job.payload.prompt_brief) + '</pre>',
        '  </div>'
      ].join("\n") : ''),
      '</div>'
    ].join("\n");
  }

  async function submitGrowthForm(form) {
    var reference = form.getAttribute("data-reference");
    var payload = growthPayload(form);

    setLoading("Queueing Growth", "Writing a growth job into the shared runtime queue.");
    setStatus("Queueing growth for " + reference + "...");

    try {
      var commandResponse = await fetch("/v1/commands/realizations.grow", {
        method: "POST",
        credentials: "same-origin",
        headers: {
          "Accept": "application/json",
          "Content-Type": "application/json"
        },
        body: JSON.stringify(payload)
      });
      var commandResult = await commandResponse.json();
      if (!commandResponse.ok) {
        throw new Error(commandResult.error || ("Queue failed with status " + commandResponse.status));
      }

      var projectionResponse = await fetch(commandResult.projection, {
        method: "GET",
        credentials: "same-origin",
        headers: { "Accept": "application/json" }
      });
      var projection = await projectionResponse.json();
      if (!projectionResponse.ok) {
        throw new Error(projection.error || ("Projection failed with status " + projectionResponse.status));
      }

      renderJobResult(projection);
      setStatus("Queued growth for " + (commandResult.target_reference || reference) + ".");
    } catch (err) {
      setError("Growth Failed", err && err.message ? err.message : String(err));
      setStatus("Growth request failed.");
      console.error(err);
    }
  }

  document.addEventListener("click", function (event) {
    var toggle = event.target.closest("[data-seed-toggle]");
    if (toggle) {
      var seed = toggle.closest(".seed");
      var wasOpen = seed.classList.contains("open");
      document.querySelectorAll(".seed.open").forEach(function (item) {
        item.classList.remove("open");
      });
      if (!wasOpen) {
        seed.classList.add("open");
      }
      return;
    }

    var actionButton = event.target.closest("[data-action][data-reference]");
    if (actionButton) {
      event.preventDefault();
      loadPartial(
        actionButton.getAttribute("data-action"),
        actionButton.getAttribute("data-reference"),
        actionButton.getAttribute("data-label") || actionButton.getAttribute("data-reference")
      );
      return;
    }

    var toggleNew = event.target.closest("[data-toggle-create-new]");
    if (toggleNew) {
      var form = toggleNew.closest("form");
      if (!form) return;
      var group = form.querySelector("[data-new-realization-fields]");
      if (!group) return;
      group.style.display = form.elements.create_new.checked ? "grid" : "none";
    }
  });

  document.addEventListener("submit", function (event) {
    var form = event.target.closest("[data-growth-form]");
    if (!form) {
      return;
    }
    event.preventDefault();
    submitGrowthForm(form);
  });

  var firstSeed = document.querySelector(".seed");
  if (firstSeed && !document.querySelector(".seed.open")) {
    firstSeed.classList.add("open");
  }
})();`
}
