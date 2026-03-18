package main

import (
	"bytes"
	"context"
	"errors"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"as/kernel/internal/boot"
	"as/kernel/internal/config"
	"as/kernel/internal/execution"
	feedbackloop "as/kernel/internal/feedback_loop"
	jsontransport "as/kernel/internal/http/json"
	"as/kernel/internal/http/server"
	"as/kernel/internal/interactions"
	"as/kernel/internal/materializer"
	"as/kernel/internal/realizations"
	registrycatalog "as/kernel/internal/registry"
	runtimedb "as/kernel/internal/runtime"
	"as/kernel/internal/telemetry"
)

func registryPermalinkRedirectPath(record registrycatalog.HashLookupRecord) string {
	return registrycatalog.PermalinkResolvePath(record.CanonicalURL, record.ContentHash)
}

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

  {{if .Registry.Available}}
  <article class="source">
    <h3>Registry Footprint</h3>
    <p class="subtle">Relative registry surface for this realization across objects, commands, projections, and schemas.</p>
    <div class="footprint">
      <div class="footprint-bar" aria-label="Registry footprint">
        {{range .Registry.Segments}}
        <span class="footprint-segment {{.ClassName}}" style="width: {{.Width}}%;" title="{{.Label}} {{.Count}}"></span>
        {{end}}
      </div>
      <div class="footprint-legend">
        {{range .Registry.Segments}}
        <div class="footprint-legend-item">
          <span class="footprint-swatch {{.ClassName}}"></span>
          <span>{{.Label}} {{.Count}}</span>
        </div>
        {{end}}
      </div>
    </div>
  </article>
  {{end}}

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
      <button class="action-button" type="button" data-action="run" data-reference="{{.Packet.Reference}}" data-label="{{.Packet.Summary}}"{{if and .ExecutionEnabled .Packet.Readiness.CanLaunchLocal}} data-launchable="true"{{end}}{{if .Current.OpenPath}} data-open-path="{{.Current.OpenPath}}"{{end}}>{{if .Current.OpenPath}}Open{{else if and .ExecutionEnabled .Packet.Readiness.CanLaunchLocal}}Run{{else}}Show Run{{end}}</button>
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
      {{if .Packet.Readiness.CanLaunchLocal}}
      <div class="readiness runnable">Runnable</div>
      {{else if .Packet.Readiness.CanRun}}
      <div class="readiness defined">Runtime Defined</div>
      {{else}}
      <div class="readiness designed">Not Runnable</div>
      {{end}}
      <h2 style="margin:0.6rem 0 0.15rem;font-size:1.2rem;">Run {{.Packet.Reference}}</h2>
      {{if and .ExecutionEnabled .Packet.Readiness.CanLaunchLocal}}
      <p class="empty">This realization can be launched by the local kernel execution backend. The kernel assigns the upstream address and injects kernel capability URLs at launch time.</p>
      {{else if .Packet.Readiness.CanRun}}
      <p class="empty">This realization carries a runtime manifest, but it is not yet launchable through the currently enabled execution backend.</p>
      {{else}}
      <p class="empty">This realization does not yet carry a runtime artifact. Use <strong>Grow</strong> to move it toward a runnable state.</p>
      {{end}}
    </div>
  </div>

  {{if .Packet.Readiness.CanRun}}
  <div class="source">
    <h3>Runtime Manifest</h3>
    <p class="pathline">{{.Packet.Readiness.RuntimeFile}}</p>
    {{range .Packet.RuntimeDocs}}
    <pre>{{.Preview}}</pre>
    {{end}}
  </div>
  {{end}}

  {{if .Current.ExecutionID}}
  <div class="source">
    <h3>Current Execution</h3>
    <div class="pathline">{{.Current.ExecutionID}} :: {{.Current.Status}}</div>
    {{if .Current.OpenPath}}<p class="subtle">Preview route: <code>{{.Current.OpenPath}}</code></p>{{end}}
    {{if .Current.LastError}}<pre>{{.Current.LastError}}</pre>{{end}}
  </div>
  {{end}}

  {{if .Current.Suspended}}
  <div class="source">
    <h3>Suspended</h3>
    <div class="pathline">{{.Current.SuspensionReason}}</div>
    {{if .Current.SuspensionMessage}}<pre>{{.Current.SuspensionMessage}}</pre>{{end}}
    {{if .Current.RemediationTarget}}<p class="subtle">Target branch: <code>{{.Current.RemediationTarget}}</code></p>{{end}}
    {{if .Current.RemediationHint}}<p class="subtle">{{.Current.RemediationHint}}</p>{{end}}
  </div>
  {{end}}

  <div class="action-row">
    {{if .Current.OpenPath}}<a class="action-button is-primary" href="{{.Current.OpenPath}}" target="_blank" rel="noopener">Open Preview</a>{{end}}
    {{if and .ExecutionEnabled .Packet.Readiness.CanLaunchLocal}}<button class="action-button" type="button" data-action="run" data-reference="{{.Packet.Reference}}" data-label="{{.Packet.Summary}}" data-launchable="true">{{if .Current.ExecutionID}}Relaunch{{else}}Launch{{end}}</button>{{end}}
    {{if and .ExecutionEnabled .Current.CanStop}}<button class="action-button" type="button" data-stop-execution="{{.Current.ExecutionID}}" data-label="{{.Packet.Summary}}">Stop</button>{{end}}
    <button class="action-button" type="button" data-action="grow" data-reference="{{.Packet.Reference}}" data-label="{{.Packet.Summary}}">Grow</button>
    <button class="action-button" type="button" data-action="inspect" data-reference="{{.Packet.Reference}}" data-label="{{.Packet.Summary}}">Inspect</button>
  </div>
</div>
`))

var realizationUnavailableTemplate = template.Must(template.New("realization-unavailable").Parse(`<!doctype html>
<html lang="en">
<head>
  {{$isCause := and (not .RefreshPath) (or .ReasonCode (eq .Status "failed") (eq .Status "stopped") (eq .Status "terminated") (eq .Status "suspended"))}}
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{if $isCause}}Route unavailable{{else}}Starting {{.Reference}}{{end}}</title>
  <link rel="stylesheet" href="/assets/sprout-logo.css">
  {{if and .RefreshPath (not $isCause)}}<noscript><meta http-equiv="refresh" content="{{if gt .RefreshAfter 0}}{{.RefreshAfter}}{{else}}2{{end}};url={{.RefreshPath}}"></noscript>{{end}}
  <style nonce="{{.CSPNonce}}">
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
      background: #eef0f3;
      color: #1f2933;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
    }
    .launch-page {
      width: min(46rem, calc(100vw - 2rem));
      margin: 0 auto;
      padding: 2rem 0 3rem;
    }
    .launch-panel {
      display: grid;
      gap: 0.95rem;
      padding: 1.35rem;
      background: rgba(255, 255, 255, 0.96);
      border: 1px solid #d4d8df;
      box-shadow: 0 1.4rem 3rem rgba(28, 35, 48, 0.14);
      color: #222730;
    }
    .launch-panel.is-cause {
      border-color: rgba(196, 71, 93, 0.24);
      box-shadow: 0 1.4rem 3rem rgba(78, 30, 38, 0.14);
    }
    .launch-top {
      display: grid;
      grid-template-columns: minmax(0, 8.5rem) minmax(0, 1fr);
      gap: 1rem;
      align-items: center;
    }
    .launch-sprout {
      width: 8.5rem;
      margin: 0 auto;
      cursor: default;
    }
    .launch-heading {
      display: grid;
      gap: 0.3rem;
    }
    .launch-kicker {
      font-size: 0.7rem;
      letter-spacing: 0.14em;
      text-transform: uppercase;
      color: #7a818d;
      font-weight: 700;
    }
    .launch-name {
      margin: 0;
      font-size: clamp(1.4rem, 4vw, 2.3rem);
      line-height: 1.05;
      letter-spacing: -0.04em;
      color: #181c24;
    }
    .launch-intro {
      margin: 0;
      color: #5f6875;
      font-size: 0.92rem;
      line-height: 1.7;
      max-width: 34rem;
    }
    .launch-progress {
      height: 0.32rem;
      overflow: hidden;
      background: #e6ebf0;
    }
    .launch-progress-fill {
      display: block;
      height: 100%;
      width: 0;
      background: linear-gradient(90deg, #2563eb 0%, #22a05a 100%);
      transition: width 0.12s linear;
    }
    .launch-progress.is-failed .launch-progress-fill {
      background: #c4475d;
    }
    .launch-progress.is-ready .launch-progress-fill {
      background: #178243;
    }
    .launch-step {
      margin: 0;
      font-size: 0.9rem;
      font-weight: 600;
      line-height: 1.45;
      color: #1f2933;
    }
    .launch-copy {
      margin: 0;
      color: #66707d;
      font-size: 0.82rem;
      line-height: 1.65;
    }
    .launch-meta-row {
      display: flex;
      flex-wrap: wrap;
      gap: 0.45rem;
    }
    .launch-meta-item {
      display: inline-flex;
      align-items: center;
      gap: 0.35rem;
      border: 1px solid #d5d9e1;
      border-radius: 999px;
      padding: 0.24rem 0.6rem;
      background: rgba(255, 255, 255, 0.82);
      color: #616875;
      font-size: 0.69rem;
      letter-spacing: 0.02em;
    }
    .launch-meta-item.is-loading {
      border-color: rgba(37, 99, 235, 0.24);
      background: rgba(37, 99, 235, 0.08);
      color: #1d4ed8;
    }
    .launch-meta-item.is-cause {
      border-color: rgba(196, 71, 93, 0.24);
      background: rgba(196, 71, 93, 0.08);
      color: #b4233d;
    }
    .launch-timer {
      margin: 0;
      color: #59606b;
      font-size: 0.8rem;
      letter-spacing: 0.01em;
    }
    .launch-assurance,
    .launch-cause {
      border: 1px solid #dde2ea;
      background: rgba(247, 249, 251, 0.92);
      padding: 1rem;
    }
    .launch-assurance strong,
    .launch-cause strong {
      display: block;
      margin-bottom: 0.35rem;
      color: #2b3340;
      font-size: 0.76rem;
      letter-spacing: 0.12em;
      text-transform: uppercase;
    }
    .launch-assurance p,
    .launch-cause p {
      margin: 0;
      color: #586272;
      font-size: 0.84rem;
      line-height: 1.7;
    }
    .launch-cause {
      border-color: rgba(196, 71, 93, 0.18);
      background: rgba(252, 246, 247, 0.96);
    }
    .launch-details {
      border: 1px solid #dde2ea;
      background: rgba(247, 249, 251, 0.9);
      padding: 0.85rem 1rem;
    }
    .launch-details summary {
      cursor: pointer;
      color: #586272;
      font-size: 0.74rem;
      letter-spacing: 0.08em;
      text-transform: uppercase;
      font-weight: 700;
    }
    .launch-debug {
      margin: 0.75rem 0 0;
      color: #59606b;
      font-family: ui-monospace, SFMono-Regular, SFMono, Menlo, Consolas, monospace;
      font-size: 0.72rem;
      line-height: 1.55;
      white-space: pre-wrap;
      word-break: break-word;
    }
    .hint {
      margin: 0;
      color: #5b6573;
      font-size: 0.84rem;
      line-height: 1.7;
    }
    .actions {
      display: flex;
      gap: 0.55rem;
      flex-wrap: wrap;
    }
    .action {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      padding: 0.52rem 0.82rem;
      border: 1px solid #c8ccd4;
      color: #616875;
      background: transparent;
      text-decoration: none;
      font-size: 0.7rem;
      letter-spacing: 0.06em;
      text-transform: uppercase;
    }
    .action.primary {
      border-color: #22a05a;
      color: #178243;
      background: rgba(34, 160, 90, 0.08);
    }
    code {
      font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
      font-size: 0.92em;
    }
    @media (max-width: 720px) {
      .launch-page { width: min(42rem, calc(100vw - 1rem)); }
      .launch-top { grid-template-columns: 1fr; text-align: center; }
      .launch-sprout { width: 7.25rem; }
    }
  </style>
</head>
<body>
  <main class="launch-page">
    <section
      class="launch-panel{{if $isCause}} is-cause{{end}}"
      data-route-launch-shell
      data-reference="{{.Reference}}"
      data-execution-id="{{.ExecutionID}}"
      data-status="{{.Status}}"
      data-refresh-path="{{.RefreshPath}}"
      data-projection-path="{{.ExecutionProjectionPath}}">
      <div class="launch-top">
        <div class="sprout-logo-shell launch-sprout" data-sprout-logo aria-hidden="true"></div>
        <div class="launch-heading">
          <div class="launch-kicker">{{if $isCause}}Route unavailable{{else}}Starting route{{end}}</div>
          <h1 class="launch-name">{{.Reference}}</h1>
          <p class="launch-intro" data-launch-copy>{{.Message}}</p>
        </div>
      </div>

      {{if not $isCause}}
      <div class="launch-progress" data-launch-progress role="progressbar" aria-label="Launch progress" aria-valuemin="0" aria-valuemax="100" aria-valuenow="10">
        <span class="launch-progress-fill" data-launch-progress-fill></span>
      </div>
      <p class="launch-step" data-launch-step>Starting execution</p>
      <p class="launch-timer" data-launch-timer>Elapsed 0.0s</p>
      {{end}}

      <div class="launch-meta-row">
        {{if .Status}}<span class="launch-meta-item {{if $isCause}}is-cause{{else}}is-loading{{end}}" data-launch-status>{{.Status}}</span>{{end}}
        {{if .ExecutionID}}<span class="launch-meta-item">{{.ExecutionID}}</span>{{end}}
        {{if .RouteDescription}}<span class="launch-meta-item" data-launch-route>{{.RouteDescription}}</span>{{end}}
      </div>

      {{if $isCause}}
      <div class="launch-cause">
        <strong>Cause</strong>
        <p>The stable route is not live right now. {{if .RemediationTarget}}Fix the issue on <code>{{.RemediationTarget}}</code> and relaunch once the change lands.{{else}}The current cause is shown below.{{end}}</p>
      </div>
      {{else}}
      <div class="launch-assurance">
        <strong>Permanent Route</strong>
        <p>You are already on the permanent route. This page will go live here as soon as health checks and route registration finish.</p>
      </div>
      {{end}}

      {{if .RemediationHint}}<p class="hint">{{.RemediationHint}}</p>{{end}}

      <details class="launch-details">
        <summary>{{if $isCause}}Execution details{{else}}Launch details{{end}}</summary>
        <pre class="launch-debug" data-launch-debug>{{if .RouteDescription}}route={{.RouteDescription}}{{end}}{{if .ExecutionID}}
exec={{.ExecutionID}}{{end}}{{if .ReasonCode}}
cause={{.ReasonCode}}{{end}}</pre>
      </details>

      <div class="actions">
        {{if .RefreshPath}}<a class="action primary" href="{{.RefreshPath}}">Refresh now</a>{{end}}
        <a class="action" href="{{.HomePath}}">View Home</a>
      </div>
    </section>
  </main>
  <script src="/assets/sprout-logo.js"></script>
  <script src="/assets/launch-state.js"></script>
  <script nonce="{{.CSPNonce}}">
    (function () {
      var shell = document.querySelector("[data-route-launch-shell]");
      if (!shell) return;

      var sprout = shell.querySelector("[data-sprout-logo]");
      var progressBar = shell.querySelector("[data-launch-progress]");
      var progressFill = shell.querySelector("[data-launch-progress-fill]");
      var stepLine = shell.querySelector("[data-launch-step]");
      var copyLine = shell.querySelector("[data-launch-copy]");
      var timerLine = shell.querySelector("[data-launch-timer]");
      var routeLine = shell.querySelector("[data-launch-route]");
      var statusPill = shell.querySelector("[data-launch-status]");
      var debugLine = shell.querySelector("[data-launch-debug]");
      var reference = shell.dataset.reference || "Unknown realization";
      var refreshPath = shell.dataset.refreshPath || "";
      var projectionPath = shell.dataset.projectionPath || "";
      var initialStatus = shell.dataset.status || "launch_requested";
      var requestedAt = Date.now();
      var pollDelay = 900;

      if (window.ASSproutLogo && typeof window.ASSproutLogo.init === "function") {
        window.ASSproutLogo.init(shell);
      }

      function readJSONResponse(response) {
        return response.text().then(function (body) {
          if (!body) return {};
          try {
            return JSON.parse(body);
          } catch (err) {
            if (!response.ok) {
              return { error: body.trim() || ("Request failed: " + response.status) };
            }
            throw err;
          }
        });
      }

      function setProgress(progress, statusValue) {
        if (!progressBar || !progressFill) return;
        progressBar.setAttribute("aria-valuenow", String(progress));
        progressBar.classList.toggle("is-ready", statusValue === "healthy");
        progressBar.classList.toggle("is-failed", window.ASLaunchState && window.ASLaunchState.isTerminalExecutionStatus(statusValue));
        progressFill.style.width = progress + "%";
        if (sprout && window.ASSproutLogo && typeof window.ASSproutLogo.setProgress === "function") {
          window.ASSproutLogo.setProgress(sprout, progress / 100);
        }
      }

      function completionTarget(session) {
        if (refreshPath) return refreshPath;
        if (session && session.open_path && session.open_path.indexOf("/__runs/") !== 0) return session.open_path;
        return window.location.pathname + window.location.search;
      }

      function render(snapshot, transientError) {
        var session = snapshot && snapshot.session ? snapshot.session : { status: initialStatus };
        var events = snapshot && Array.isArray(snapshot.events) ? snapshot.events : [];
        var startedAt = session && session.started_at ? Date.parse(session.started_at) : NaN;
        var elapsedMs = Number.isFinite(startedAt) ? Math.max(0, Date.now() - startedAt) : Math.max(0, Date.now() - requestedAt);
        var step = window.ASLaunchState ? window.ASLaunchState.stepLabel(session, events) : "Starting execution";
        var copy = window.ASLaunchState ? window.ASLaunchState.copyText(session, events, reference, elapsedMs, transientError) : "";
        var progress = window.ASLaunchState ? window.ASLaunchState.displayedProgress(session, events, { requestedAt: requestedAt, minimumDisplayMs: 0, reference: reference }, elapsedMs, false) : 10;
        var debug = window.ASLaunchState ? window.ASLaunchState.debugLine(session, events, { requestedAt: requestedAt, reference: reference }, elapsedMs, transientError) : "";

        if (stepLine) stepLine.textContent = step;
        if (copyLine) copyLine.textContent = copy;
        if (timerLine) timerLine.textContent = "Elapsed " + (window.ASLaunchState ? window.ASLaunchState.formatElapsed(elapsedMs) : "0.0s");
        if (routeLine) routeLine.textContent = refreshPath || (session && session.open_path) || routeLine.textContent;
        if (statusPill) statusPill.textContent = session && session.status ? session.status : initialStatus;
        if (debugLine) debugLine.textContent = debug;
        setProgress(progress, session && session.status ? session.status : initialStatus);

        if (session && session.status === "healthy" && (session.open_path || refreshPath)) {
          window.location.replace(completionTarget(session));
          return true;
        }
        if (window.ASLaunchState && window.ASLaunchState.isTerminalExecutionStatus(session && session.status ? session.status : "")) {
          return true;
        }
        return false;
      }

      if (!projectionPath) {
        setProgress(100, initialStatus);
        return;
      }

      async function poll() {
        try {
          var response = await fetch(projectionPath, {
            method: "GET",
            credentials: "same-origin",
            cache: "no-store",
            headers: { "Accept": "application/json" }
          });
          var payload = await readJSONResponse(response);
          if (!response.ok) {
            throw new Error((payload && payload.error) || ("Execution poll failed: " + response.status));
          }
          if (render(payload, "")) return;
          var nextStatus = payload && payload.session && payload.session.status ? payload.session.status : initialStatus;
          pollDelay = nextStatus === "launch_requested" ? 700 : 900;
        } catch (err) {
          render({ session: { status: initialStatus } }, err && err.message ? err.message : "poll_error");
        }
        window.setTimeout(poll, pollDelay);
      }

      render({ session: { status: initialStatus } }, "");
      window.setTimeout(poll, 250);
    })();
  </script>
</body>
</html>
`))

var mutateTemplate = template.Must(template.New("mutate").Parse(`
<div class="stack">
{{if .IsNew}}
  <h2 style="margin:0;font-size:1.15rem;">Create from Bare Earth</h2>
  <p class="subtle">Define a new seed, request improvements, or fork this software model. Describe your goal and whether this should reuse the current data lineage or start from fresh data.</p>
{{else}}
  <h2 style="margin:0;font-size:1.15rem;">Mutate {{.Packet.Reference}}</h2>
  <p class="subtle">Propose a change to this seed. Review the current specs, describe your mutation, and queue a growth job so the agent can optimize from the same design/contracts.</p>
{{end}}

  <!-- Step 1: Current Specs / Intent -->
  <div data-wizard-step>
    {{if .IsNew}}
    <div class="field" style="margin-top:0.75rem;">
      <label for="mutate-summary">What should this seed do?</label>
      <input id="mutate-summary" type="text" name="summary" placeholder="A short summary of the application" maxlength="200">
    </div>
    <div class="field">
      <label for="mutate-description">Describe the scope and key features</label>
      <textarea id="mutate-description" name="description" placeholder="Who are the users? What workflows should it support? What are the boundaries?"></textarea>
    </div>
    {{else}}
    <h3 style="margin:0.75rem 0 0.5rem;font-size:0.88rem;">Current Seed Specs</h3>
    <div class="doc-grid">
      {{range .Packet.SeedDocs}}
      <details class="doc">
        <summary>{{.Kind}} <span class="pathline">{{.Path}}</span></summary>
        <pre>{{.Preview}}</pre>
      </details>
      {{end}}
    </div>
    <div class="field" style="margin-top:0.75rem;">
      <label for="mutate-description">Describe the mutation</label>
      <textarea id="mutate-description" name="description" placeholder="What do you want to change or add to this seed?"></textarea>
    </div>
    {{end}}
    <div class="action-row" style="margin-top:0.75rem;">
      <button class="action-button is-primary" type="button" data-wizard-next>Next: Review Approach</button>
    </div>
  </div>

  <!-- Step 2: AI Approach Review (stub) -->
  <div data-wizard-step style="display:none;">
    <h3 style="margin:0.75rem 0 0.5rem;font-size:0.88rem;">Proposed Approach</h3>
    <p class="indicator-copy">After you submit, an AI agent will review your description against the seed specs and propose a high-level implementation approach here.</p>
    <div class="field" style="margin-top:0.75rem;">
      <label for="mutate-approach">Approach notes (manual for now)</label>
      <textarea id="mutate-approach" name="approach" placeholder="Describe the high-level approach you would take, or leave blank to let the AI decide."></textarea>
    </div>
    <div class="action-row" style="margin-top:0.75rem;">
      <button class="action-button" type="button" data-wizard-prev>Back</button>
      <button class="action-button is-primary" type="button" data-wizard-next>Next: UAT Criteria</button>
    </div>
  </div>

  <!-- Step 3: UAT Criteria (stub) -->
  <div data-wizard-step style="display:none;">
    <h3 style="margin:0.75rem 0 0.5rem;font-size:0.88rem;">Acceptance &amp; UAT Criteria</h3>
    {{if not .IsNew}}
    <p class="subtle">Current acceptance criteria from the seed:</p>
    {{range .Packet.SeedDocs}}{{if eq .Kind "seed_acceptance"}}
    <details class="doc" open>
      <summary>{{.Kind}} <span class="pathline">{{.Path}}</span></summary>
      <pre>{{.Preview}}</pre>
    </details>
    {{end}}{{end}}
    {{end}}
    <div class="field" style="margin-top:0.75rem;">
      <label for="mutate-uat">Additional UAT criteria</label>
      <textarea id="mutate-uat" name="uat" placeholder="What tests should verify this change? The AI will suggest criteria here in the future."></textarea>
    </div>
    <div class="action-row" style="margin-top:0.75rem;">
      <button class="action-button" type="button" data-wizard-prev>Back</button>
      <button class="action-button is-primary" type="button" data-wizard-next>Next: Confirm</button>
    </div>
  </div>

  <!-- Step 4: Confirm & Queue -->
  <div data-wizard-step style="display:none;">
    <h3 style="margin:0.75rem 0 0.5rem;font-size:0.88rem;">Confirm &amp; Queue</h3>
    <p class="indicator-copy">Review your inputs above, then queue this {{if .IsNew}}seed creation{{else}}mutation{{end}} as a growth job.</p>
    {{if not .IsNew}}
    <form class="stack" data-growth-form data-reference="{{.Packet.Reference}}" style="margin-top:0.75rem;">
      <input type="hidden" name="operation" value="grow">
      <input type="hidden" name="create_new" value="">
      <input type="hidden" name="new_realization_id" value="">
      <input type="hidden" name="new_summary" value="">
      <input type="hidden" name="profile" value="balanced">
      <input type="hidden" name="target" value="runnable_mvp">
      <div class="field">
        <label for="mutate-final-instructions">Developer Instructions</label>
        <textarea id="mutate-final-instructions" name="developer_instructions" placeholder="Any final instructions for the growth agent."></textarea>
      </div>
      <div class="action-row">
        <button class="action-button" type="button" data-wizard-prev>Back</button>
        <button class="action-button is-primary" type="submit">Queue Growth Job</button>
      </div>
    </form>
    {{else}}
    <p class="subtle" style="margin-top:0.5rem;">New seed creation is not yet wired to the backend. Use the CLI: <code>as seed create</code></p>
    <div class="action-row" style="margin-top:0.75rem;">
      <button class="action-button" type="button" data-wizard-prev>Back</button>
      <button class="action-button" type="button" disabled>Create Seed (coming soon)</button>
    </div>
    {{end}}
  </div>
</div>
`))

var registryTemplate = template.Must(template.New("registry").Parse(`
<div class="stack">
  <div class="row">
    <div>
      <div class="readiness defined">Phase 1</div>
      <h2 style="margin:0.55rem 0 0.15rem;font-size:1.2rem;">Registry Catalog</h2>
      <p class="empty">Repo-derived object and schema navigation built from validated realization contracts. This is agent-safe discovery data, not the final append-only ledger.</p>
    </div>
    <div class="subtle">
      <div>{{.Catalog.Summary.Realizations}} realizations</div>
      <div>{{.Catalog.Summary.Contracts}} contracts</div>
    </div>
  </div>

  <div class="meta">
    <span class="pill">{{.Catalog.Summary.Realizations}} realizations</span>
    <span class="pill">{{.Catalog.Summary.Objects}} objects</span>
    <span class="pill">{{.Catalog.Summary.Schemas}} schemas</span>
    <span class="pill">{{.Catalog.Summary.Commands}} commands</span>
    <span class="pill">{{.Catalog.Summary.Projections}} projections</span>
  </div>

  <article class="source">
    <h3>Realizations</h3>
    <p class="subtle">This is the top-level read-only registry catalog for realized surfaces. It shows which objects, commands, and projections each realization exposes.</p>
    <div class="doc-grid">
      {{range .Catalog.Realizations}}
      <details class="doc">
        <summary>{{.Reference}} <span class="pathline">{{.SurfaceKind}} :: {{.Status}}</span></summary>
        <div class="stack" style="margin-top:0.65rem;">
          <p class="empty">{{.Summary}}</p>
          <div class="pathline">{{.ContractFile}}</div>
          {{if .AuthModes}}
          <div>
            <div class="subtle">Auth Modes</div>
            <div class="meta">{{range .AuthModes}}<span class="pill">{{.}}</span>{{end}}</div>
          </div>
          {{end}}
          {{if .Capabilities}}
          <div>
            <div class="subtle">Capabilities</div>
            <div class="meta">{{range .Capabilities}}<span class="pill">{{.}}</span>{{end}}</div>
          </div>
          {{end}}
          {{if .ObjectKinds}}
          <div>
            <div class="subtle">Objects</div>
            {{range .ObjectKinds}}<div class="pathline">{{.}}</div>{{end}}
          </div>
          {{end}}
          {{if .CommandNames}}
          <div>
            <div class="subtle">Commands</div>
            {{range .CommandNames}}<div class="pathline">{{.}}</div>{{end}}
          </div>
          {{end}}
          {{if .Projections}}
          <div>
            <div class="subtle">Projections</div>
            {{range .Projections}}<div class="pathline">{{.}}</div>{{end}}
          </div>
          {{end}}
        </div>
      </details>
      {{else}}
      <p class="empty">No realizations discovered yet.</p>
      {{end}}
    </div>
  </article>

  <article class="source">
    <h3>Commands</h3>
    <p class="subtle">Commands are shown directly so agents and humans can inspect their schema refs and projection bindings without traversing object detail first.</p>
    <div class="doc-grid">
      {{range .Catalog.Commands}}
      <details class="doc">
        <summary>{{.Reference}} :: {{.Name}} <span class="pathline">{{.Path}}</span></summary>
        <div class="stack" style="margin-top:0.65rem;">
          <p class="empty">{{.Summary}}</p>
          <div class="pathline">input {{.InputSchemaRef}}</div>
          <div class="pathline">result {{.ResultSchemaRef}}</div>
          <div class="pathline">projection {{.Projection}}</div>
          <div class="pathline">consistency {{.Consistency}} :: idempotency {{.Idempotency}}</div>
          {{if .AuthModes}}
          <div class="meta">{{range .AuthModes}}<span class="pill">{{.}}</span>{{end}}</div>
          {{end}}
        </div>
      </details>
      {{else}}
      <p class="empty">No commands discovered yet.</p>
      {{end}}
    </div>
  </article>

  <article class="source">
    <h3>Projections</h3>
    <p class="subtle">Projections are first-class read surfaces. Keeping them visible here makes the human console match the agent discovery surface.</p>
    <div class="doc-grid">
      {{range .Catalog.Projections}}
      <details class="doc">
        <summary>{{.Reference}} :: {{.Name}} <span class="pathline">{{.Path}}</span></summary>
        <div class="stack" style="margin-top:0.65rem;">
          <p class="empty">{{.Summary}}</p>
          <div class="pathline">freshness {{.Freshness}}</div>
          {{if .Capabilities}}
          <div class="meta">{{range .Capabilities}}<span class="pill">{{.}}</span>{{end}}</div>
          {{end}}
        </div>
      </details>
      {{else}}
      <p class="empty">No projections discovered yet.</p>
      {{end}}
    </div>
  </article>

  <article class="source">
    <h3>Objects</h3>
    <p class="subtle">Objects are grouped by seed and object kind. Each detail section shows the realizations, commands, projections, and schema refs attached to that declaration.</p>
    <div class="doc-grid">
      {{range .Catalog.Objects}}
      <details class="doc">
        <summary>{{.SeedID}} :: {{.Kind}} <span class="pathline">{{len .Realizations}} realizations</span></summary>
        <div class="stack" style="margin-top:0.65rem;">
          {{if .Summary}}<p class="empty">{{.Summary}}</p>{{end}}
          {{if .Capabilities}}
          <div class="meta">{{range .Capabilities}}<span class="pill">{{.}}</span>{{end}}</div>
          {{end}}
          <div>
            <div class="subtle">Schemas</div>
            {{range .SchemaRefs}}<div class="pathline">{{.}}</div>{{end}}
          </div>
          <div>
            <div class="subtle">Realizations</div>
            {{range .Realizations}}
            <div class="pathline">{{.Reference}} :: {{.SurfaceKind}} :: {{.SchemaRef}}</div>
            {{end}}
          </div>
          {{if .Commands}}
          <div>
            <div class="subtle">Commands</div>
            {{range .Commands}}
            <div class="pathline">{{.Reference}} :: {{.Name}} :: {{.Path}}</div>
            {{end}}
          </div>
          {{end}}
          {{if .Projections}}
          <div>
            <div class="subtle">Projections</div>
            {{range .Projections}}
            <div class="pathline">{{.Reference}} :: {{.Name}} :: {{.Path}}</div>
            {{end}}
          </div>
          {{end}}
        </div>
      </details>
      {{else}}
      <p class="empty">No objects discovered yet.</p>
      {{end}}
    </div>
  </article>

  <article class="source">
    <h3>Schemas</h3>
    <p class="subtle">Schema refs are canonicalized relative to each contract file so humans and agents navigate the same identifiers.</p>
    <div class="doc-grid">
      {{range .Catalog.Schemas}}
      <details class="doc">
        <summary>{{.Ref}} <span class="pathline">{{len .ObjectUses}} object uses / {{len .CommandInputs}} inputs / {{len .CommandResults}} results</span></summary>
        <div class="stack" style="margin-top:0.65rem;">
          <div class="pathline">{{.Path}}{{if .Anchor}}#{{.Anchor}}{{end}}</div>
          {{if .ObjectUses}}
          <div>
            <div class="subtle">Object Uses</div>
            {{range .ObjectUses}}
            <div class="pathline">{{.Reference}} :: {{.Kind}}</div>
            {{end}}
          </div>
          {{end}}
          {{if .CommandInputs}}
          <div>
            <div class="subtle">Command Inputs</div>
            {{range .CommandInputs}}
            <div class="pathline">{{.Reference}} :: {{.Name}} :: {{.Path}}</div>
            {{end}}
          </div>
          {{end}}
          {{if .CommandResults}}
          <div>
            <div class="subtle">Command Results</div>
            {{range .CommandResults}}
            <div class="pathline">{{.Reference}} :: {{.Name}} :: {{.Path}}</div>
            {{end}}
          </div>
          {{end}}
        </div>
      </details>
      {{else}}
      <p class="empty">No schemas discovered yet.</p>
      {{end}}
    </div>
  </article>
</div>
`))

type mutateView struct {
	IsNew  bool
	Packet realizations.GrowthContext
}

type partialView struct {
	Reference string
	Result    materializer.Materialization
	NotFound  bool
	Registry  registryFootprintView
}

type growthView struct {
	Packet           realizations.GrowthContext
	ExecutionEnabled bool
	Current          executionModalState
}

type runView struct {
	Packet           realizations.GrowthContext
	ExecutionEnabled bool
	Current          executionModalState
}

type executionModalState struct {
	ExecutionID       string
	Status            string
	OpenPath          string
	LastError         string
	CanStop           bool
	Suspended         bool
	SuspensionReason  string
	SuspensionMessage string
	RemediationTarget string
	RemediationHint   string
}

type registryView struct {
	Catalog registrycatalog.Catalog
}

type realizationUnavailableView struct {
	CSPNonce                string
	Reference               string
	ExecutionID             string
	Status                  string
	ReasonCode              string
	Message                 string
	RemediationTarget       string
	RemediationHint         string
	RouteDescription        string
	MountPrefix             string
	RefreshPath             string
	HomePath                string
	RefreshAfter            int
	ExecutionProjectionPath string
}

type registryFootprintView struct {
	Available bool
	Segments  []registryFootprintSegment
}

type registryFootprintSegment struct {
	Label     string
	ClassName string
	Count     int
	Width     int
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	repoRoot, err := config.RepoRootFromEnvOrWD()
	if err != nil {
		log.Fatal(err)
	}

	var store feedbackloop.Recorder = feedbackloop.NewMemoryStore()
	service, err := materializer.NewService(repoRoot, remoteClient())
	if err != nil {
		log.Fatal(err)
	}
	registryReader := registrycatalog.NewCatalogReader(repoRoot)
	var hashIndex *runtimedb.RegistryHashIndex

	var runtimeService *interactions.RuntimeService
	runtimeConfig := config.LoadRuntimeConfigFromEnv()
	if runtimeConfig.Enabled() {
		pool, err := runtimedb.OpenPool(ctx, runtimeConfig.DatabaseURL)
		if err != nil {
			log.Fatal(err)
		}
		defer pool.Close()
		if runtimeConfig.AutoMigrate {
			if err := runtimedb.RunMigrations(ctx, pool, repoRoot); err != nil {
				log.Fatal(err)
			}
		}
		hashIndex = runtimedb.NewRegistryHashIndex(pool)
		if err := hashIndex.SyncCatalogReader(ctx, registryReader); err != nil {
			log.Fatal(err)
		}
		runtimeService = interactions.NewRuntimeService(pool)
		store = feedbackloop.NewPostgresStore(pool)
		log.Print("feedback loop: persisting to runtime database")
		telemetry.NewServiceMonitor("webd", runtimeService).Start(ctx)
	}
	bootExecutionEnabled := runtimeService != nil && boolEnv("AS_BOOT_EXECUTION_ENABLED", false)

	mux := http.NewServeMux()
	mux.Handle("POST /feedback/incidents", jsontransport.NewIncidentIngestHandler(store))
	mux.Handle("GET /assets/", sproutAssetHandler())
	mux.Handle("GET /__sprout-assets/", sproutAssetHandler())
	jsontransport.NewGrowthAPI(repoRoot, runtimeService).Register(mux)
	jsontransport.NewRegistryCatalogAPI(registryReader, hashIndex).Register(mux)
	mux.HandleFunc("GET /reg/{hash}", func(w http.ResponseWriter, r *http.Request) {
		if hashIndex == nil {
			http.Error(w, "registry permalink lookup unavailable", http.StatusServiceUnavailable)
			return
		}
		contentHash := strings.TrimSpace(r.PathValue("hash"))
		if !registrycatalog.IsSHA256Hex(contentHash) {
			http.NotFound(w, r)
			return
		}
		record, err := hashIndex.Resolve(r.Context(), contentHash)
		if err != nil {
			if errors.Is(err, registrycatalog.ErrHashLookupNotFound) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		redirectPath := registryPermalinkRedirectPath(record)
		if strings.TrimSpace(redirectPath) == "" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, redirectPath, http.StatusFound)
	})
	mux.HandleFunc("GET /r/{hash}", func(w http.ResponseWriter, r *http.Request) {
		if hashIndex == nil {
			http.Error(w, "registry share lookup unavailable", http.StatusServiceUnavailable)
			return
		}
		contentHashPrefix := strings.ToLower(strings.TrimSpace(r.PathValue("hash")))
		if len(contentHashPrefix) < registrycatalog.ShortShareHashLength || len(contentHashPrefix) > 64 {
			http.NotFound(w, r)
			return
		}
		record, err := hashIndex.ResolvePrefix(r.Context(), contentHashPrefix)
		if err != nil {
			if errors.Is(err, registrycatalog.ErrHashLookupNotFound) || errors.Is(err, registrycatalog.ErrHashLookupAmbiguous) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		redirectPath := registryPermalinkRedirectPath(record)
		if strings.TrimSpace(redirectPath) == "" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, redirectPath, http.StatusFound)
	})
	if bootExecutionEnabled {
		jsontransport.NewExecutionAPI(repoRoot, runtimeService).RegisterPrefix(mux, "/boot")
	}
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		options, err := service.ListRealizations(r.Context())
		if err != nil {
			server.WriteTextError(w, r, http.StatusInternalServerError, server.Internal("failed to load realizations"))
			return
		}
		catalog, err := registryReader.Catalog()
		if err != nil {
			server.WriteTextError(w, r, http.StatusInternalServerError, server.Internal("failed to load registry catalog"))
			return
		}
		executions := latestExecutionStateByReference(r.Context(), runtimeService)

		requestMeta := server.RequestMetadataFromContext(r.Context())
		view := newBootPageView(options, catalog, executions, bootExecutionEnabled, service.Remote != nil, runtimeService != nil, server.CSPNonceFromContext(r.Context()), boot.ClientFeedbackLoopScript(boot.FeedbackLoopScriptConfig{
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
			server.WriteTextError(w, r, http.StatusInternalServerError, server.Internal("failed to render boot page"))
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
				server.WriteTextError(w, r, http.StatusInternalServerError, server.Internal("failed to materialize realization"))
				return
			}
		}
		if catalog, catalogErr := registryReader.Catalog(); catalogErr == nil {
			view.Registry = newRegistryFootprintView(catalog, reference)
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := materializationTemplate.Execute(w, view); err != nil {
			server.WriteTextError(w, r, http.StatusInternalServerError, server.Internal("failed to render materialization"))
			return
		}
	})
	mux.HandleFunc("GET /partials/grow", func(w http.ResponseWriter, r *http.Request) {
		reference := strings.TrimSpace(r.URL.Query().Get("reference"))
		packet, err := realizations.LoadGrowthContext(repoRoot, reference)
		if err != nil {
			server.WriteTextError(w, r, http.StatusBadRequest, server.BadRequest("realization reference could not be loaded"))
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := growthTemplate.Execute(w, growthView{
			Packet:           packet,
			ExecutionEnabled: bootExecutionEnabled,
			Current:          latestExecutionModalState(r.Context(), runtimeService, packet.Reference),
		}); err != nil {
			server.WriteTextError(w, r, http.StatusInternalServerError, server.Internal("failed to render growth panel"))
			return
		}
	})
	mux.HandleFunc("GET /partials/run", func(w http.ResponseWriter, r *http.Request) {
		reference := strings.TrimSpace(r.URL.Query().Get("reference"))
		packet, err := realizations.LoadGrowthContext(repoRoot, reference)
		if err != nil {
			server.WriteTextError(w, r, http.StatusBadRequest, server.BadRequest("realization reference could not be loaded"))
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := runTemplate.Execute(w, runView{
			Packet:           packet,
			ExecutionEnabled: bootExecutionEnabled,
			Current:          latestExecutionModalState(r.Context(), runtimeService, packet.Reference),
		}); err != nil {
			server.WriteTextError(w, r, http.StatusInternalServerError, server.Internal("failed to render run panel"))
			return
		}
	})
	mux.HandleFunc("GET /partials/mutate", func(w http.ResponseWriter, r *http.Request) {
		reference := strings.TrimSpace(r.URL.Query().Get("reference"))
		view := mutateView{IsNew: reference == ""}
		if reference != "" {
			packet, err := realizations.LoadGrowthContext(repoRoot, reference)
			if err != nil {
				server.WriteTextError(w, r, http.StatusBadRequest, server.BadRequest("realization reference could not be loaded"))
				return
			}
			view.Packet = packet
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := mutateTemplate.Execute(w, view); err != nil {
			server.WriteTextError(w, r, http.StatusInternalServerError, server.Internal("failed to render mutation wizard"))
			return
		}
	})
	mux.HandleFunc("GET /partials/registry", func(w http.ResponseWriter, r *http.Request) {
		catalog, err := registryReader.Catalog()
		if err != nil {
			server.WriteTextError(w, r, http.StatusInternalServerError, server.Internal("failed to load registry catalog"))
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := registryTemplate.Execute(w, registryView{Catalog: catalog}); err != nil {
			server.WriteTextError(w, r, http.StatusInternalServerError, server.Internal("failed to render registry panel"))
			return
		}
	})

	baseDomain := config.EnvOrDefault("AS_BASE_DOMAIN", "localhost")
	registryHost := config.EnvOrDefault("AS_REGISTRY_HOST", "")
	handler := buildRoutingHandler(ctx, repoRoot, service, runtimeService, baseDomain, registryHost, http.Handler(mux))
	if runtimeService != nil {
		handler = server.SessionResolutionMiddleware(server.RuntimeSessionResolver{Lookup: runtimeService},
			server.RateLimitMiddleware(runtimeService, rateLimitOptions(runtimeConfig), handler),
		)
	}

	addr := config.EnvOrDefault("AS_WEBD_ADDR", "127.0.0.1:8090")
	log.Printf("webd listening on %s (repo root %s)", addr, repoRoot)
	httpServer := &http.Server{Addr: addr, Handler: server.DefaultMiddlewareStack(handler)}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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

func boolEnv(key string, fallback bool) bool {
	raw := strings.TrimSpace(config.EnvOrDefault(key, ""))
	if raw == "" {
		return fallback
	}
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func rateLimitOptions(runtimeConfig config.RuntimeConfig) server.RateLimitOptions {
	return server.RateLimitOptions{
		Enabled:             runtimeConfig.RateLimitsEnabled,
		Window:              time.Duration(runtimeConfig.RateLimitWindow) * time.Second,
		BlockDuration:       time.Duration(runtimeConfig.RateLimitBlock) * time.Second,
		AnonymousReadLimit:  int64(runtimeConfig.AnonymousRead),
		AnonymousWriteLimit: int64(runtimeConfig.AnonymousWrite),
		SessionReadLimit:    int64(runtimeConfig.SessionRead),
		SessionWriteLimit:   int64(runtimeConfig.SessionWrite),
		InternalLimit:       int64(runtimeConfig.Internal),
		WorkerLimit:         int64(runtimeConfig.Worker),
		FeedbackLimit:       int64(runtimeConfig.Feedback),
	}
}

func newRegistryFootprintView(catalog registrycatalog.Catalog, reference string) registryFootprintView {
	realization, ok := registrycatalog.GetRealization(catalog, reference)
	if !ok {
		return registryFootprintView{}
	}

	schemaRefs := make(map[string]struct{})
	for _, schema := range catalog.Schemas {
		for _, use := range schema.ObjectUses {
			if use.Reference == reference {
				schemaRefs[schema.Ref] = struct{}{}
				break
			}
		}
		for _, use := range schema.CommandInputs {
			if use.Reference == reference {
				schemaRefs[schema.Ref] = struct{}{}
				break
			}
		}
		for _, use := range schema.CommandResults {
			if use.Reference == reference {
				schemaRefs[schema.Ref] = struct{}{}
				break
			}
		}
	}

	segments := []registryFootprintSegment{
		{Label: "Objects", ClassName: "objects", Count: len(realization.ObjectKinds)},
		{Label: "Commands", ClassName: "commands", Count: len(realization.CommandNames)},
		{Label: "Projections", ClassName: "projections", Count: len(realization.Projections)},
		{Label: "Schemas", ClassName: "schemas", Count: len(schemaRefs)},
	}

	total := 0
	nonZero := 0
	for _, segment := range segments {
		total += segment.Count
		if segment.Count > 0 {
			nonZero++
		}
	}
	if total == 0 || nonZero == 0 {
		return registryFootprintView{}
	}

	compacted := make([]registryFootprintSegment, 0, nonZero)
	remainingWidth := 100
	seen := 0
	for _, segment := range segments {
		if segment.Count <= 0 {
			continue
		}
		seen++
		width := (segment.Count * 100) / total
		if width == 0 {
			width = 1
		}
		if seen == nonZero || width > remainingWidth {
			width = remainingWidth
		}
		remainingWidth -= width
		segment.Width = width
		compacted = append(compacted, segment)
	}

	return registryFootprintView{
		Available: true,
		Segments:  compacted,
	}
}

func latestExecutionStateByReference(ctx context.Context, runtimeService *interactions.RuntimeService) map[string]executionBootState {
	if runtimeService == nil {
		return map[string]executionBootState{}
	}
	openPaths := activeRoutePathsByExecution(ctx, runtimeService)
	items, err := runtimeService.ListRealizationExecutions(ctx, "", 200)
	if err != nil {
		log.Printf("warning: could not list realization executions: %v", err)
		return map[string]executionBootState{}
	}
	out := make(map[string]executionBootState)
	for _, item := range items {
		state := executionBootState{
			ExecutionID: item.ExecutionID,
			Status:      item.Status,
			OpenPath:    strings.TrimSpace(openPaths[item.ExecutionID]),
		}
		existing, ok := out[item.Reference]
		if !ok {
			out[item.Reference] = state
			continue
		}
		if existing.OpenPath == "" && state.OpenPath != "" {
			out[item.Reference] = state
		}
	}
	return out
}

func latestExecutionModalState(ctx context.Context, runtimeService *interactions.RuntimeService, reference string) executionModalState {
	if runtimeService == nil || strings.TrimSpace(reference) == "" {
		return executionModalState{}
	}
	openPaths := activeRoutePathsByExecution(ctx, runtimeService)
	items, err := runtimeService.ListRealizationExecutions(ctx, reference, 20)
	if err != nil || len(items) == 0 {
		return executionModalState{}
	}
	item := items[0]
	for _, candidate := range items {
		if strings.TrimSpace(openPaths[candidate.ExecutionID]) != "" {
			item = candidate
			break
		}
	}
	state := executionModalState{
		ExecutionID: item.ExecutionID,
		Status:      item.Status,
		OpenPath:    strings.TrimSpace(openPaths[item.ExecutionID]),
		LastError:   strings.TrimSpace(item.LastError),
		CanStop:     canStopExecutionStatus(item.Status),
	}
	suspension, err := runtimeService.GetActiveRealizationSuspension(ctx, reference)
	if err == nil {
		state.Suspended = true
		state.SuspensionReason = strings.TrimSpace(suspension.ReasonCode)
		state.SuspensionMessage = strings.TrimSpace(suspension.Message)
		state.RemediationTarget = strings.TrimSpace(suspension.RemediationTarget)
		state.RemediationHint = strings.TrimSpace(suspension.RemediationHint)
		if state.LastError == "" {
			state.LastError = state.SuspensionMessage
		}
	}
	return state
}

func activeRoutePathsByExecution(ctx context.Context, runtimeService *interactions.RuntimeService) map[string]string {
	if runtimeService == nil {
		return map[string]string{}
	}
	bindings, err := runtimeService.ListRealizationRouteBindings(ctx, true)
	if err != nil {
		log.Printf("warning: could not list runtime route bindings: %v", err)
		return map[string]string{}
	}
	out := make(map[string]string, len(bindings))
	priorities := make(map[string]int, len(bindings))
	for _, binding := range bindings {
		executionID := strings.TrimSpace(binding.ExecutionID)
		if executionID == "" {
			continue
		}
		candidate, priority := preferredOpenPathForBinding(binding)
		if candidate == "" || priority == 0 {
			continue
		}
		if existing, ok := priorities[executionID]; ok && existing >= priority {
			continue
		}
		out[executionID] = candidate
		priorities[executionID] = priority
	}
	return out
}

func preferredOpenPathForBinding(binding interactions.RealizationRouteBinding) (string, int) {
	pathPrefix := normalizeRoutePrefix(binding.PathPrefix)
	if pathPrefix == "" {
		return "", 0
	}
	switch strings.TrimSpace(binding.BindingKind) {
	case "stable_path":
		return pathPrefix, 30
	case "preview_path":
		return pathPrefix, 20
	default:
		return pathPrefix, 10
	}
}

func normalizeRoutePrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return ""
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return prefix
}

func buildRoutingHandler(
	ctx context.Context,
	repoRoot string,
	catalogService *materializer.Service,
	runtimeService *interactions.RuntimeService,
	baseDomain string,
	registryHost string,
	fallback http.Handler,
) http.Handler {
	if runtimeService == nil {
		log.Printf("realization routes: runtime service disabled; dynamic execution routing unavailable")
		return fallback
	}

	routeSource := newRuntimeRouteSource(runtimeService)
	suspensionSource := newRuntimeSuspensionSource(runtimeService)
	if routes, err := routeSource.routes(ctx); err == nil && len(routes) > 0 {
		log.Printf("realization routes: %d runtime (base domain %s, registry host %s)", len(routes), baseDomain, registryHost)
	}
	if suspensions, err := suspensionSource.suspensions(ctx); err == nil && len(suspensions) > 0 {
		log.Printf("realization suspensions: %d active", len(suspensions))
	}
	return dynamicRealizationRoutingMiddleware(routeSource, suspensionSource, runtimeService, repoRoot, catalogService, baseDomain, registryHost, fallback)
}

// --- Realization routing (subdomain + path prefix) ---

type realizationRoute struct {
	Reference  string
	Subdomain  string
	PathPrefix string
	ProxyAddr  string
}

const registryRoutePathPrefix = "/registry/reading-room/"

type runtimeRouteSource struct {
	service  *interactions.RuntimeService
	mu       sync.Mutex
	cachedAt time.Time
	cached   []realizationRoute
}

type runtimeSuspensionSource struct {
	service  *interactions.RuntimeService
	mu       sync.Mutex
	cachedAt time.Time
	cached   []interactions.RealizationSuspension
}

func newRuntimeRouteSource(service *interactions.RuntimeService) *runtimeRouteSource {
	return &runtimeRouteSource{service: service}
}

func newRuntimeSuspensionSource(service *interactions.RuntimeService) *runtimeSuspensionSource {
	return &runtimeSuspensionSource{service: service}
}

func (s *runtimeRouteSource) routes(ctx context.Context) ([]realizationRoute, error) {
	s.mu.Lock()
	if time.Since(s.cachedAt) < 500*time.Millisecond && len(s.cached) > 0 {
		out := append([]realizationRoute(nil), s.cached...)
		s.mu.Unlock()
		return out, nil
	}
	s.mu.Unlock()

	if s.service == nil {
		return nil, nil
	}
	bindings, err := s.service.ListRealizationRouteBindings(ctx, true)
	if err != nil {
		return nil, err
	}
	routes := make([]realizationRoute, 0, len(bindings))
	for _, binding := range bindings {
		if strings.TrimSpace(binding.UpstreamAddr) == "" {
			continue
		}
		routes = append(routes, realizationRoute{
			Reference:  binding.Reference,
			Subdomain:  strings.TrimSpace(binding.Subdomain),
			PathPrefix: strings.TrimSpace(binding.PathPrefix),
			ProxyAddr:  strings.TrimSpace(binding.UpstreamAddr),
		})
	}
	sort.Slice(routes, func(i, j int) bool {
		return len(routes[i].PathPrefix) > len(routes[j].PathPrefix)
	})

	s.mu.Lock()
	s.cached = append([]realizationRoute(nil), routes...)
	s.cachedAt = time.Now()
	s.mu.Unlock()
	return routes, nil
}

func (s *runtimeSuspensionSource) suspensions(ctx context.Context) ([]interactions.RealizationSuspension, error) {
	s.mu.Lock()
	if time.Since(s.cachedAt) < 500*time.Millisecond && len(s.cached) > 0 {
		out := append([]interactions.RealizationSuspension(nil), s.cached...)
		s.mu.Unlock()
		return out, nil
	}
	s.mu.Unlock()

	if s.service == nil {
		return nil, nil
	}
	items, err := s.service.ListRealizationSuspensions(ctx, interactions.ListRealizationSuspensionsInput{
		ActiveOnly: true,
		Limit:      500,
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool {
		return len(items[i].RoutePathPrefix) > len(items[j].RoutePathPrefix)
	})

	s.mu.Lock()
	s.cached = append([]interactions.RealizationSuspension(nil), items...)
	s.cachedAt = time.Now()
	s.mu.Unlock()
	return items, nil
}

func extractSubdomain(host, baseDomain string) string {
	host = normalizedHost(host)
	baseDomain = strings.ToLower(strings.TrimSpace(baseDomain))

	if host == baseDomain {
		return ""
	}
	suffix := "." + baseDomain
	if strings.HasSuffix(host, suffix) {
		return host[:len(host)-len(suffix)]
	}
	return ""
}

func normalizedHost(host string) string {
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}
	return strings.ToLower(strings.TrimSpace(host))
}

func prefixMountedRequestPath(r *http.Request, mountPrefix string) *http.Request {
	r2 := r.Clone(r.Context())
	mountPrefix = normalizeRoutePrefix(mountPrefix)
	if mountPrefix == "" {
		return r2
	}
	mountBase := strings.TrimSuffix(mountPrefix, "/")
	if strings.HasPrefix(r.URL.Path, mountPrefix) || r.URL.Path == mountBase {
		return r2
	}
	path := r.URL.Path
	if path == "" {
		path = "/"
	}
	if path == "/" {
		r2.URL.Path = mountPrefix
	} else {
		r2.URL.Path = mountBase + path
	}
	if r.URL.RawPath != "" {
		rawPath := r.URL.RawPath
		if rawPath == "/" {
			r2.URL.RawPath = mountPrefix
		} else {
			r2.URL.RawPath = mountBase + rawPath
		}
	}
	return r2
}

func realizationRoutingMiddleware(
	routes []realizationRoute,
	suspensions []interactions.RealizationSuspension,
	runtimeService *interactions.RuntimeService,
	repoRoot string,
	catalogService *materializer.Service,
	baseDomain string,
	registryHost string,
	fallback http.Handler,
) http.Handler {
	subdomainMap := make(map[string]realizationRoute)
	for _, r := range routes {
		if r.Subdomain != "" {
			subdomainMap[strings.ToLower(r.Subdomain)] = r
		}
	}
	suspensionSubdomainMap := make(map[string]interactions.RealizationSuspension)
	for _, item := range suspensions {
		if subdomain := strings.ToLower(strings.TrimSpace(item.RouteSubdomain)); subdomain != "" {
			suspensionSubdomainMap[subdomain] = item
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestHost := normalizedHost(r.Host)
		requestPath := strings.TrimSpace(r.URL.Path)
		registryReservedNamespace := requestPath == "/reg" ||
			strings.HasPrefix(requestPath, "/reg/") ||
			requestPath == "/r" ||
			strings.HasPrefix(requestPath, "/r/") ||
			requestPath == "/v1/registry" ||
			strings.HasPrefix(requestPath, "/v1/registry/") ||
			requestPath == "/v1/contracts" ||
			strings.HasPrefix(requestPath, "/v1/contracts/")
		if registryHost = normalizedHost(registryHost); registryHost != "" && requestHost == registryHost {
			registryMountedBase := strings.TrimSuffix(registryRoutePathPrefix, "/")
			if requestPath == registryMountedBase || strings.HasPrefix(requestPath, registryRoutePathPrefix) {
				redirectPath := requestRedirectPath(r)
				switch {
				case redirectPath == "" || redirectPath == registryMountedBase:
					redirectPath = "/"
				case strings.HasPrefix(redirectPath, registryRoutePathPrefix):
					redirectPath = "/" + strings.TrimPrefix(redirectPath, registryRoutePathPrefix)
				}
				if r.URL != nil && r.URL.RawQuery != "" {
					redirectPath += "?" + r.URL.RawQuery
				}
				http.Redirect(w, r, "https://"+registryHost+redirectPath, http.StatusFound)
				return
			}
			if !registryReservedNamespace {
				if route, ok := subdomainMap["registry"]; ok {
					proxyToRealization(route, w, r)
					return
				}
				for _, route := range routes {
					if normalizeRoutePrefix(route.PathPrefix) == registryRoutePathPrefix {
						proxyToRealization(route, w, r)
						return
					}
				}
				r = prefixMountedRequestPath(r, registryRoutePathPrefix)
			}
		}

		// Subdomain takes priority.
		if sub := extractSubdomain(requestHost, baseDomain); sub != "" {
			skipRegistrySubdomainProxy := sub == "registry" && registryReservedNamespace
			if route, ok := subdomainMap[sub]; ok {
				if !skipRegistrySubdomainProxy {
					proxyToRealization(route, w, r)
					return
				}
			}
			if sub == "registry" && !skipRegistrySubdomainProxy {
				for _, route := range routes {
					if normalizeRoutePrefix(route.PathPrefix) == registryRoutePathPrefix {
						proxyToRealization(route, w, r)
						return
					}
				}
			}
			if suspension, ok := suspensionSubdomainMap[sub]; ok {
				renderSuspensionPage(w, r, suspension)
				return
			}
		}

		// Path prefix fallback.
		for _, route := range routes {
			if route.PathPrefix != "" && strings.HasPrefix(r.URL.Path, route.PathPrefix) {
				r2 := trimMountedRequestPrefix(r, route.PathPrefix)
				proxyToMountedRealization(route, route.PathPrefix, w, r2)
				return
			}
		}

		if strings.HasPrefix(r.URL.Path, "/__runs/") {
			if renderInactiveExecutionPage(runtimeService, w, r) {
				return
			}
			http.NotFound(w, r)
			return
		}
		for _, suspension := range suspensions {
			pathPrefix := strings.TrimSpace(suspension.RoutePathPrefix)
			if pathPrefix != "" && strings.HasPrefix(r.URL.Path, pathPrefix) {
				renderSuspensionPage(w, r, suspension)
				return
			}
		}

		if runtimeService != nil && catalogService != nil && r.URL.Path != "" {
			reference, matchedPrefix := realizationReferenceForPath(r.Context(), catalogService, r.URL.Path)
			if reference != "" {
				requestPath := requestRedirectPath(r)
				if existingLaunch := preferredLaunchTarget(r.Context(), runtimeService, reference); existingLaunch.ExecutionID != "" {
					if existingLaunch.OpenPath != "" {
						targetPath := launchRedirectPath(existingLaunch.OpenPath, requestPath, matchedPrefix)
						if r.URL.RawQuery != "" {
							targetPath += "?" + r.URL.RawQuery
						}
						http.Redirect(w, r, targetPath, http.StatusFound)
						return
					}
					renderLaunchingPage(w, r, existingLaunch, currentRequestTarget(r), matchedPrefix)
					return
				}
				if launchExecution, err := enqueueLaunchForMissingPath(r.Context(), runtimeService, repoRoot, reference, r); err == nil {
					renderLaunchingPage(w, r, launchTarget{
						ExecutionID: launchExecution.ExecutionID,
						Reference:   launchExecution.Reference,
						Status:      launchExecution.Status,
					}, currentRequestTarget(r), matchedPrefix)
					return
				} else if err != nil {
					log.Printf("warning: could not auto-launch %s for path %s: %v", reference, r.URL.Path, err)
				}
			}
		}

		fallback.ServeHTTP(w, r)
	})
}

func dynamicRealizationRoutingMiddleware(
	routeSource *runtimeRouteSource,
	suspensionSource *runtimeSuspensionSource,
	runtimeService *interactions.RuntimeService,
	repoRoot string,
	catalogService *materializer.Service,
	baseDomain string,
	registryHost string,
	fallback http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		routes, err := routeSource.routes(r.Context())
		if err != nil {
			log.Printf("warning: could not load runtime routes: %v", err)
			fallback.ServeHTTP(w, r)
			return
		}
		suspensions, err := suspensionSource.suspensions(r.Context())
		if err != nil {
			log.Printf("warning: could not load realization suspensions: %v", err)
		}
		realizationRoutingMiddleware(routes, suspensions, runtimeService, repoRoot, catalogService, baseDomain, registryHost, fallback).ServeHTTP(w, r)
	})
}

func realizationReferenceForPath(ctx context.Context, catalogService *materializer.Service, rawPath string) (string, string) {
	if catalogService == nil {
		return "", ""
	}
	targetPath := strings.TrimSpace(rawPath)
	if targetPath == "" || targetPath == "/" {
		return "", ""
	}
	if !strings.HasPrefix(targetPath, "/") {
		targetPath = "/" + targetPath
	}
	lookupPath := targetPath
	if !strings.HasSuffix(lookupPath, "/") {
		lookupPath += "/"
	}

	options, err := catalogService.ListRealizations(ctx)
	if err != nil {
		log.Printf("warning: could not list realizations for path launch fallback: %v", err)
		return "", ""
	}
	bestPrefix := ""
	bestReference := ""
	for _, option := range options {
		pathPrefix := normalizeRoutePrefix(strings.TrimSpace(option.PathPrefix))
		if pathPrefix == "" {
			continue
		}
		if strings.HasPrefix(lookupPath, pathPrefix) && len(pathPrefix) > len(bestPrefix) {
			bestPrefix = pathPrefix
			bestReference = strings.TrimSpace(option.Reference)
		}
	}
	return bestReference, bestPrefix
}

func launchRedirectPath(openPath, requestPath, matchedPrefix string) string {
	matchedPrefix = strings.TrimSpace(matchedPrefix)
	if matchedPrefix == "" || strings.TrimSpace(openPath) == "" {
		return strings.TrimSpace(openPath)
	}
	matchedPath := strings.TrimSpace(requestPath)
	if matchedPath == "" {
		return strings.TrimSpace(openPath)
	}
	if !strings.HasSuffix(matchedPath, "/") {
		matchedPath += "/"
	}
	remainder := strings.TrimPrefix(matchedPath, matchedPrefix)
	remainder = strings.TrimPrefix(remainder, "/")
	if remainder == "" {
		return strings.TrimSpace(openPath)
	}
	if strings.HasSuffix(openPath, "/") {
		return strings.TrimSpace(openPath) + remainder
	}
	return strings.TrimSpace(openPath) + "/" + remainder
}

func requestRedirectPath(r *http.Request) string {
	if r == nil || r.URL == nil {
		return ""
	}
	if escaped := strings.TrimSpace(r.URL.EscapedPath()); escaped != "" {
		return escaped
	}
	return strings.TrimSpace(r.URL.Path)
}

type launchTarget struct {
	ExecutionID string
	Reference   string
	Status      string
	OpenPath    string
}

func selectLaunchTarget(items []interactions.RealizationExecution, openPaths map[string]string) launchTarget {
	bestReady := launchTarget{}
	bestPending := launchTarget{}
	bestReadyPriority := 0
	bestPendingPriority := 0

	for _, item := range items {
		if isTerminalExecutionStatus(item.Status) {
			continue
		}
		candidate := launchTarget{
			ExecutionID: strings.TrimSpace(item.ExecutionID),
			Reference:   strings.TrimSpace(item.Reference),
			Status:      strings.TrimSpace(item.Status),
			OpenPath:    strings.TrimSpace(openPaths[item.ExecutionID]),
		}
		if candidate.ExecutionID == "" {
			continue
		}
		if candidate.OpenPath != "" {
			priority := launchStatusPriority(candidate.Status)
			if priority > bestReadyPriority {
				bestReady = candidate
				bestReadyPriority = priority
			}
			continue
		}
		priority := launchStatusPriority(candidate.Status)
		if priority > bestPendingPriority {
			bestPending = candidate
			bestPendingPriority = priority
		}
	}

	if bestReady.ExecutionID != "" {
		return bestReady
	}
	return bestPending
}

func launchStatusPriority(status string) int {
	switch strings.TrimSpace(status) {
	case "healthy":
		return 30
	case "starting":
		return 20
	case "launch_requested":
		return 10
	default:
		return 1
	}
}

func currentRequestTarget(r *http.Request) string {
	target := requestRedirectPath(r)
	if r != nil && r.URL != nil && strings.TrimSpace(r.URL.RawQuery) != "" {
		target += "?" + strings.TrimSpace(r.URL.RawQuery)
	}
	return target
}

func executionSessionProjectionPath(executionID string) string {
	if strings.TrimSpace(executionID) == "" {
		return ""
	}
	return "/boot/projections/realization-execution/sessions/" + url.PathEscape(strings.TrimSpace(executionID))
}

func preferredLaunchTarget(ctx context.Context, runtimeService *interactions.RuntimeService, reference string) launchTarget {
	execItems, err := runtimeService.ListRealizationExecutions(ctx, strings.TrimSpace(reference), 20)
	if err != nil || len(execItems) == 0 {
		return launchTarget{}
	}
	openPaths := activeRoutePathsByExecution(ctx, runtimeService)
	return selectLaunchTarget(execItems, openPaths)
}

func enqueueLaunchForMissingPath(ctx context.Context, runtimeService *interactions.RuntimeService, repoRoot, reference string, r *http.Request) (interactions.RealizationExecution, error) {
	packet, err := realizations.LoadGrowthContext(repoRoot, strings.TrimSpace(reference))
	if err != nil {
		return interactions.RealizationExecution{}, err
	}
	if !packet.Readiness.CanLaunchLocal {
		return interactions.RealizationExecution{}, errors.New("realization is not launchable through local execution backend")
	}

	requestMeta := server.RequestMetadataFromContext(ctx)
	resolvedSession, _ := server.SessionFromContext(ctx)

	executionID := "exec_" + strings.ReplaceAll(strings.ReplaceAll(packet.Reference, "/", "_"), "-", "_")
	suffix := strings.TrimSpace(requestMeta.RequestID)
	if len(suffix) > 6 {
		suffix = suffix[len(suffix)-6:]
	}
	if suffix == "" {
		suffix = strconv.FormatInt(time.Now().UnixNano()%1000000, 10)
	}
	executionID = executionID + "_" + suffix
	previewPath := execution.PreviewPath(executionID)

	execRow, err := runtimeService.CreateRealizationExecution(ctx, interactions.CreateRealizationExecutionInput{
		ExecutionID:           executionID,
		Reference:             packet.Reference,
		SeedID:                packet.SeedID,
		RealizationID:         packet.RealizationID,
		Backend:               execution.LocalBackendName,
		Mode:                  "preview",
		Status:                "launch_requested",
		RouteSubdomain:        packet.Subdomain,
		RoutePathPrefix:       packet.PathPrefix,
		PreviewPathPrefix:     previewPath,
		LaunchedByPrincipalID: resolvedSession.PrincipalID,
		LaunchedBySessionID:   resolvedSession.SessionID,
		RequestID:             requestMeta.RequestID,
	})
	if err != nil {
		return interactions.RealizationExecution{}, err
	}

	job, err := runtimeService.EnqueueJob(ctx, interactions.EnqueueJobInput{
		Queue:    "realization-execution",
		Kind:     "realizations.launch",
		Priority: 120,
		Payload: map[string]interface{}{
			"execution_id": execRow.ExecutionID,
			"reference":    packet.Reference,
			"seed_id":      packet.SeedID,
		},
	})
	if err != nil {
		return interactions.RealizationExecution{}, err
	}
	_, _ = runtimeService.RecordRealizationExecutionEvent(ctx, interactions.RecordRealizationExecutionEventInput{
		ExecutionID: execRow.ExecutionID,
		Name:        "launch_requested",
		Data:        map[string]interface{}{"job_id": job.JobID},
	})
	return execRow, nil
}

func isUnixSocketAddr(addr string) bool {
	return strings.HasPrefix(addr, "/") || strings.HasPrefix(addr, ".")
}

func proxyToRealization(route realizationRoute, w http.ResponseWriter, r *http.Request) {
	proxyToMountedRealization(route, "", w, r)
}

func proxyToMountedRealization(route realizationRoute, mountPrefix string, w http.ResponseWriter, r *http.Request) {
	seedID, realizationID := realizations.SplitReference(route.Reference)
	mountPrefix = normalizeRoutePrefix(mountPrefix)
	w.Header().Set("Content-Security-Policy", mountedRealizationContentSecurityPolicy())

	if isUnixSocketAddr(route.ProxyAddr) {
		target, _ := url.Parse("http://unix")
		proxy := &httputil.ReverseProxy{
			Rewrite: func(pr *httputil.ProxyRequest) {
				pr.SetURL(target)
				pr.SetXForwarded()
				pr.Out.Host = pr.In.Host
				pr.Out.Header.Set("X-AS-Seed-ID", seedID)
				pr.Out.Header.Set("X-AS-Realization-ID", realizationID)
				pr.Out.Header.Set("X-Forwarded-Proto", externalRequestScheme(pr.In))
				pr.Out.Header.Set("X-Forwarded-Host", pr.In.Host)
				if mountPrefix != "" {
					pr.Out.Header.Set("X-Forwarded-Prefix", strings.TrimSuffix(mountPrefix, "/"))
				}
			},
			ModifyResponse: func(res *http.Response) error {
				return rewriteMountedResponse(res, mountPrefix)
			},
			Transport: &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", route.ProxyAddr)
				},
			},
		}
		proxy.ServeHTTP(w, r)
		return
	}

	target, err := url.Parse("http://" + route.ProxyAddr)
	if err != nil {
		http.Error(w, "invalid proxy target", http.StatusBadGateway)
		return
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.SetXForwarded()
			pr.Out.Host = pr.In.Host
			pr.Out.Header.Set("X-AS-Seed-ID", seedID)
			pr.Out.Header.Set("X-AS-Realization-ID", realizationID)
			pr.Out.Header.Set("X-Forwarded-Proto", externalRequestScheme(pr.In))
			pr.Out.Header.Set("X-Forwarded-Host", pr.In.Host)
			if mountPrefix != "" {
				pr.Out.Header.Set("X-Forwarded-Prefix", strings.TrimSuffix(mountPrefix, "/"))
			}
		},
		ModifyResponse: func(res *http.Response) error {
			return rewriteMountedResponse(res, mountPrefix)
		},
	}
	proxy.ServeHTTP(w, r)
}

func trimMountedRequestPrefix(r *http.Request, mountPrefix string) *http.Request {
	r2 := r.Clone(r.Context())
	trimmedPrefix := strings.TrimSuffix(mountPrefix, "/")
	r2.URL.Path = strings.TrimPrefix(r.URL.Path, trimmedPrefix)
	if r2.URL.Path == "" {
		r2.URL.Path = "/"
	}
	if r.URL.RawPath != "" {
		r2.URL.RawPath = strings.TrimPrefix(r.URL.RawPath, trimmedPrefix)
		if r2.URL.RawPath == "" {
			r2.URL.RawPath = "/"
		}
	}
	return r2
}

func externalRequestScheme(r *http.Request) string {
	switch forwarded := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))); forwarded {
	case "http", "https":
		return forwarded
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func rewriteMountedResponse(res *http.Response, mountPrefix string) error {
	contentType := strings.ToLower(res.Header.Get("Content-Type"))
	if strings.Contains(contentType, "text/html") {
		res.Header.Set("Content-Security-Policy", mountedRealizationContentSecurityPolicy())
	}

	mountPrefix = normalizeRoutePrefix(mountPrefix)
	if mountPrefix == "" {
		return nil
	}

	if location := strings.TrimSpace(res.Header.Get("Location")); location != "" {
		res.Header.Set("Location", prefixedMountedPath(location, mountPrefix))
	}

	if values := res.Header.Values("Set-Cookie"); len(values) > 0 {
		res.Header.Del("Set-Cookie")
		for _, value := range values {
			res.Header.Add("Set-Cookie", rewriteMountedCookiePath(value, mountPrefix))
		}
	}

	if !strings.Contains(contentType, "text/html") {
		return nil
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if err := res.Body.Close(); err != nil {
		return err
	}

	rewritten := rewriteMountedHTML(body, mountPrefix)
	res.Body = io.NopCloser(bytes.NewReader(rewritten))
	res.ContentLength = int64(len(rewritten))
	res.Header.Set("Content-Length", strconv.Itoa(len(rewritten)))
	return nil
}

func prefixedMountedPath(path, mountPrefix string) string {
	if mountPrefix == "" {
		return path
	}
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || strings.HasPrefix(path, "/__") {
		return path
	}
	if path == "/" {
		return mountPrefix
	}
	mountBase := strings.TrimSuffix(mountPrefix, "/")
	if strings.HasPrefix(path, mountPrefix) || path == mountBase {
		return path
	}
	return mountBase + path
}

func rewriteMountedCookiePath(raw, mountPrefix string) string {
	lower := strings.ToLower(raw)
	idx := strings.Index(lower, "path=/")
	if idx < 0 {
		return raw
	}
	after := idx + len("path=/")
	if after < len(raw) && raw[after] != ';' {
		return raw
	}
	return raw[:idx] + "Path=" + mountPrefix + raw[after:]
}

func rewriteMountedHTML(body []byte, mountPrefix string) []byte {
	mountPrefix = normalizeRoutePrefix(mountPrefix)
	if mountPrefix == "" {
		return body
	}

	prefix := strings.TrimSuffix(mountPrefix, "/") + "/"
	rewritten := strings.NewReplacer(
		`href="/`, `href="`+prefix,
		`href='/`, `href='`+prefix,
		`src="/`, `src="`+prefix,
		`src='/`, `src='`+prefix,
		`action="/`, `action="`+prefix,
		`action='/`, `action='`+prefix,
		`formaction="/`, `formaction="`+prefix,
		`formaction='/`, `formaction='`+prefix,
		`hx-get="/`, `hx-get="`+prefix,
		`hx-get='/`, `hx-get='`+prefix,
		`hx-post="/`, `hx-post="`+prefix,
		`hx-post='/`, `hx-post='`+prefix,
		`hx-put="/`, `hx-put="`+prefix,
		`hx-put='/`, `hx-put='`+prefix,
		`hx-delete="/`, `hx-delete="`+prefix,
		`hx-delete='/`, `hx-delete='`+prefix,
		`hx-patch="/`, `hx-patch="`+prefix,
		`hx-patch='/`, `hx-patch='`+prefix,
		`sse-connect="/`, `sse-connect="`+prefix,
		`sse-connect='/`, `sse-connect='`+prefix,
		`data-copy="/`, `data-copy="`+prefix,
		`data-copy='/`, `data-copy='`+prefix,
	).Replace(string(body))

	doublePrefix := strings.TrimSuffix(mountPrefix, "/") + mountPrefix
	rewritten = strings.NewReplacer(
		`href="`+doublePrefix, `href="`+mountPrefix,
		`href='`+doublePrefix, `href='`+mountPrefix,
		`src="`+doublePrefix, `src="`+mountPrefix,
		`src='`+doublePrefix, `src='`+mountPrefix,
		`action="`+doublePrefix, `action="`+mountPrefix,
		`action='`+doublePrefix, `action='`+mountPrefix,
		`formaction="`+doublePrefix, `formaction="`+mountPrefix,
		`formaction='`+doublePrefix, `formaction='`+mountPrefix,
		`hx-get="`+doublePrefix, `hx-get="`+mountPrefix,
		`hx-get='`+doublePrefix, `hx-get='`+mountPrefix,
		`hx-post="`+doublePrefix, `hx-post="`+mountPrefix,
		`hx-post='`+doublePrefix, `hx-post='`+mountPrefix,
		`hx-put="`+doublePrefix, `hx-put="`+mountPrefix,
		`hx-put='`+doublePrefix, `hx-put='`+mountPrefix,
		`hx-delete="`+doublePrefix, `hx-delete="`+mountPrefix,
		`hx-delete='`+doublePrefix, `hx-delete='`+mountPrefix,
		`hx-patch="`+doublePrefix, `hx-patch="`+mountPrefix,
		`hx-patch='`+doublePrefix, `hx-patch='`+mountPrefix,
		`sse-connect="`+doublePrefix, `sse-connect="`+mountPrefix,
		`sse-connect='`+doublePrefix, `sse-connect='`+mountPrefix,
		`data-copy="`+doublePrefix, `data-copy="`+mountPrefix,
		`data-copy='`+doublePrefix, `data-copy='`+mountPrefix,
	).Replace(rewritten)

	reserved := strings.TrimSuffix(mountPrefix, "/") + "/__"
	rewritten = strings.NewReplacer(
		`href="`+reserved, `href="/__`,
		`href='`+reserved, `href='/__`,
		`src="`+reserved, `src="/__`,
		`src='`+reserved, `src='/__`,
		`action="`+reserved, `action="/__`,
		`action='`+reserved, `action='/__`,
		`formaction="`+reserved, `formaction="/__`,
		`formaction='`+reserved, `formaction='/__`,
		`hx-get="`+reserved, `hx-get="/__`,
		`hx-get='`+reserved, `hx-get='/__`,
		`hx-post="`+reserved, `hx-post="/__`,
		`hx-post='`+reserved, `hx-post='/__`,
		`hx-put="`+reserved, `hx-put="/__`,
		`hx-put='`+reserved, `hx-put='/__`,
		`hx-delete="`+reserved, `hx-delete="/__`,
		`hx-delete='`+reserved, `hx-delete='/__`,
		`hx-patch="`+reserved, `hx-patch="/__`,
		`hx-patch='`+reserved, `hx-patch='/__`,
		`sse-connect="`+reserved, `sse-connect="/__`,
		`sse-connect='`+reserved, `sse-connect='/__`,
		`data-copy="`+reserved, `data-copy="/__`,
		`data-copy='`+reserved, `data-copy='/__`,
	).Replace(rewritten)

	apiRoot := strings.TrimSuffix(mountPrefix, "/") + "/v1/"
	rewritten = strings.NewReplacer(
		`href="`+apiRoot, `href="/v1/`,
		`href='`+apiRoot, `href='/v1/`,
		`src="`+apiRoot, `src="/v1/`,
		`src='`+apiRoot, `src='/v1/`,
		`action="`+apiRoot, `action="/v1/`,
		`action='`+apiRoot, `action='/v1/`,
		`formaction="`+apiRoot, `formaction="/v1/`,
		`formaction='`+apiRoot, `formaction='/v1/`,
		`hx-get="`+apiRoot, `hx-get="/v1/`,
		`hx-get='`+apiRoot, `hx-get='/v1/`,
		`hx-post="`+apiRoot, `hx-post="/v1/`,
		`hx-post='`+apiRoot, `hx-post='/v1/`,
		`hx-put="`+apiRoot, `hx-put="/v1/`,
		`hx-put='`+apiRoot, `hx-put='/v1/`,
		`hx-delete="`+apiRoot, `hx-delete="/v1/`,
		`hx-delete='`+apiRoot, `hx-delete='/v1/`,
		`hx-patch="`+apiRoot, `hx-patch="/v1/`,
		`hx-patch='`+apiRoot, `hx-patch='/v1/`,
		`sse-connect="`+apiRoot, `sse-connect="/v1/`,
		`sse-connect='`+apiRoot, `sse-connect='/v1/`,
		`data-copy="`+apiRoot, `data-copy="/v1/`,
		`data-copy='`+apiRoot, `data-copy='/v1/`,
	).Replace(rewritten)

	return []byte(rewritten)
}

func mountedRealizationContentSecurityPolicy() string {
	return strings.Join([]string{
		"default-src 'self'",
		"base-uri 'self'",
		"connect-src 'self'",
		"font-src 'self' data:",
		"form-action 'self'",
		"frame-ancestors 'none'",
		"img-src 'self' data: https:",
		"manifest-src 'self'",
		"object-src 'none'",
		"script-src 'self' 'unsafe-inline' https://unpkg.com https://static.cloudflareinsights.com",
		"style-src 'self' 'unsafe-inline'",
	}, "; ")
}

func renderInactiveExecutionPage(runtimeService *interactions.RuntimeService, w http.ResponseWriter, r *http.Request) bool {
	if runtimeService == nil {
		return false
	}
	executionID := previewExecutionID(r.URL.Path)
	if executionID == "" {
		return false
	}
	suspension, err := runtimeService.GetRealizationSuspensionByExecution(r.Context(), executionID)
	if err == nil {
		renderSuspensionPage(w, r, suspension)
		return true
	}
	if err != nil && !errors.Is(err, interactions.ErrNotFound) {
		log.Printf("warning: could not load realization suspension for %s: %v", executionID, err)
	}
	execution, err := runtimeService.GetRealizationExecution(r.Context(), executionID)
	if err != nil {
		if !errors.Is(err, interactions.ErrNotFound) {
			log.Printf("warning: could not load realization execution %s: %v", executionID, err)
		}
		return false
	}
	if !isTerminalExecutionStatus(execution.Status) {
		renderLaunchingPage(w, r, launchTarget{
			ExecutionID: execution.ExecutionID,
			Reference:   execution.Reference,
			Status:      execution.Status,
		}, currentRequestTarget(r), "")
		return true
	}
	renderUnavailablePage(w, r, realizationUnavailableView{
		CSPNonce:    server.CSPNonceFromContext(r.Context()),
		Reference:   execution.Reference,
		ExecutionID: execution.ExecutionID,
		Status:      execution.Status,
		ReasonCode:  "execution_" + strings.TrimSpace(execution.Status),
		Message: firstNonEmpty(
			strings.TrimSpace(execution.LastError),
			"This realization is not active on a live route right now. If the execution can be restored, this screen will retry automatically; if it was terminated, the cause is shown here.",
		),
		RouteDescription: strings.TrimSpace(execution.PreviewPathPrefix),
		MountPrefix:      strings.TrimSpace(execution.PreviewPathPrefix),
	})
	return true
}

func renderSuspensionPage(w http.ResponseWriter, r *http.Request, item interactions.RealizationSuspension) {
	routeDescription := strings.TrimSpace(item.RoutePathPrefix)
	if routeDescription == "" {
		routeDescription = strings.TrimSpace(item.RouteSubdomain)
	}
	renderUnavailablePage(w, r, realizationUnavailableView{
		CSPNonce:          server.CSPNonceFromContext(r.Context()),
		Reference:         item.Reference,
		ExecutionID:       item.ExecutionID,
		Status:            "suspended",
		ReasonCode:        item.ReasonCode,
		Message:           firstNonEmpty(strings.TrimSpace(item.Message), "This realization was shut down by the kernel."),
		RemediationTarget: strings.TrimSpace(item.RemediationTarget),
		RemediationHint:   strings.TrimSpace(item.RemediationHint),
		RouteDescription:  routeDescription,
		MountPrefix:       strings.TrimSpace(item.RoutePathPrefix),
	})
}

func renderLaunchingPage(w http.ResponseWriter, r *http.Request, target launchTarget, refreshPath, mountPrefix string) {
	reference := firstNonEmpty(strings.TrimSpace(target.Reference), "Unknown realization")
	message := "The kernel is launching this realization. The stable URL will refresh automatically when the route is ready."
	switch strings.TrimSpace(target.Status) {
	case "starting":
		message = "The kernel started this realization and is waiting for it to pass health checks before routing traffic."
	case "launch_requested":
		message = "The launch job is queued and waiting for the execution worker to finish startup."
	}
	renderUnavailablePage(w, r, realizationUnavailableView{
		CSPNonce:                server.CSPNonceFromContext(r.Context()),
		Reference:               reference,
		ExecutionID:             strings.TrimSpace(target.ExecutionID),
		Status:                  firstNonEmpty(strings.TrimSpace(target.Status), "launch_requested"),
		ReasonCode:              "launch_in_progress",
		Message:                 message,
		RouteDescription:        firstNonEmpty(strings.TrimSpace(refreshPath), currentRequestTarget(r)),
		MountPrefix:             strings.TrimSpace(mountPrefix),
		RefreshPath:             firstNonEmpty(strings.TrimSpace(refreshPath), currentRequestTarget(r)),
		RefreshAfter:            2,
		ExecutionProjectionPath: executionSessionProjectionPath(strings.TrimSpace(target.ExecutionID)),
	})
}

func renderUnavailablePage(w http.ResponseWriter, r *http.Request, view realizationUnavailableView) {
	if view.Reference == "" {
		view.Reference = "Unknown realization"
	}
	if view.Message == "" {
		view.Message = "This realization is not currently available."
	}
	view.MountPrefix = normalizeRoutePrefix(view.MountPrefix)
	view.RefreshPath = prefixedMountedPath(strings.TrimSpace(view.RefreshPath), view.MountPrefix)
	view.HomePath = prefixedMountedPath("/", view.MountPrefix)
	if view.HomePath == "" {
		view.HomePath = "/"
	}
	var body bytes.Buffer
	if err := realizationUnavailableTemplate.Execute(&body, view); err != nil {
		server.WriteTextError(w, r, http.StatusInternalServerError, server.Internal("failed to render unavailable page"))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Robots-Tag", "noindex")
	if view.RefreshPath != "" {
		retryAfter := view.RefreshAfter
		if retryAfter <= 0 {
			retryAfter = 2
		}
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	}
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = body.WriteTo(w)
}

func previewExecutionID(path string) string {
	if !strings.HasPrefix(path, "/__runs/") {
		return ""
	}
	trimmed := strings.TrimPrefix(path, "/__runs/")
	trimmed = strings.TrimPrefix(trimmed, "/")
	if trimmed == "" {
		return ""
	}
	if idx := strings.Index(trimmed, "/"); idx >= 0 {
		return strings.TrimSpace(trimmed[:idx])
	}
	return strings.TrimSpace(trimmed)
}

func canStopExecutionStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "", "failed", "stopped", "terminated":
		return false
	default:
		return true
	}
}

func isTerminalExecutionStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "failed", "stopped", "terminated":
		return true
	default:
		return false
	}
}
