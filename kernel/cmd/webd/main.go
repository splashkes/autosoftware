package main

import (
	"bytes"
	"context"
	"errors"
	"html/template"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"as/kernel/internal/boot"
	"as/kernel/internal/config"
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
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Realization unavailable</title>
  <style nonce="{{.CSPNonce}}">
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
      background:
        radial-gradient(circle at top left, rgba(217, 119, 6, 0.14), transparent 26rem),
        linear-gradient(180deg, #f4f0e8 0%, #ede7dc 100%);
      color: #1f2933;
      font-family: Georgia, "Times New Roman", serif;
    }
    .shell {
      width: min(54rem, calc(100vw - 2rem));
      margin: 0 auto;
      padding: 2.5rem 0 3rem;
    }
    .card {
      border: 1px solid rgba(185, 174, 158, 0.92);
      background: rgba(255, 252, 247, 0.9);
      box-shadow: 0 1.4rem 3rem rgba(58, 49, 37, 0.08);
      padding: 1.6rem;
    }
    .eyebrow {
      color: #9a5a0a;
      font: 700 0.74rem/1.2 "Helvetica Neue", Helvetica, Arial, sans-serif;
      letter-spacing: 0.14em;
      text-transform: uppercase;
    }
    h1 {
      margin: 0.65rem 0 0.35rem;
      font-size: clamp(2rem, 5vw, 3.6rem);
      line-height: 0.96;
      letter-spacing: -0.05em;
      color: #181c24;
    }
    .copy,
    .meta,
    .list,
    .hint {
      font-family: "Helvetica Neue", Helvetica, Arial, sans-serif;
    }
    .copy {
      margin: 0.65rem 0 0;
      color: #47515f;
      font-size: 1rem;
      line-height: 1.75;
      max-width: 40rem;
    }
    .meta {
      display: flex;
      flex-wrap: wrap;
      gap: 0.45rem;
      margin: 1rem 0 1.1rem;
    }
    .pill {
      display: inline-flex;
      align-items: center;
      border: 1px solid #d7ccb8;
      background: rgba(255, 255, 255, 0.74);
      padding: 0.28rem 0.65rem;
      border-radius: 999px;
      color: #7a4c12;
      font-size: 0.73rem;
      letter-spacing: 0.04em;
      text-transform: uppercase;
    }
    .panel {
      margin-top: 1rem;
      border: 1px solid rgba(210, 201, 188, 0.9);
      background: rgba(255, 255, 255, 0.62);
      padding: 1rem;
    }
    .panel h2 {
      margin: 0 0 0.55rem;
      font: 700 0.86rem/1.2 "Helvetica Neue", Helvetica, Arial, sans-serif;
      letter-spacing: 0.08em;
      text-transform: uppercase;
      color: #5b6573;
    }
    .list {
      margin: 0;
      padding-left: 1.15rem;
      color: #334155;
      line-height: 1.7;
    }
    .hint {
      margin-top: 1rem;
      color: #596373;
      font-size: 0.94rem;
      line-height: 1.7;
    }
    code {
      font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
      font-size: 0.92em;
    }
  </style>
</head>
<body>
  <main class="shell">
    <section class="card">
      <div class="eyebrow">Execution halted</div>
      <h1>{{.Reference}}</h1>
      <p class="copy">{{.Message}}</p>
      <div class="meta">
        {{if .Status}}<span class="pill">{{.Status}}</span>{{end}}
        {{if .ReasonCode}}<span class="pill">{{.ReasonCode}}</span>{{end}}
        {{if .ExecutionID}}<span class="pill">{{.ExecutionID}}</span>{{end}}
      </div>

      <div class="panel">
        <h2>What happened</h2>
        <ul class="list">
          {{if .RouteDescription}}<li>Route: <code>{{.RouteDescription}}</code></li>{{end}}
          <li>The kernel stopped this realization and removed its live route.</li>
          {{if .RemediationTarget}}<li>Fix the issue on <code>{{.RemediationTarget}}</code> and relaunch after the change lands.</li>{{end}}
        </ul>
      </div>

      {{if .RemediationHint}}
      <p class="hint">{{.RemediationHint}}</p>
      {{end}}
    </section>
  </main>
</body>
</html>
`))

var mutateTemplate = template.Must(template.New("mutate").Parse(`
<div class="stack">
  {{if .IsNew}}
  <h2 style="margin:0;font-size:1.15rem;">Create from Bare Earth</h2>
  <p class="subtle">Define a new seed. Describe what you want to build, then review the generated brief, design, and acceptance criteria.</p>
  {{else}}
  <h2 style="margin:0;font-size:1.15rem;">Mutate {{.Packet.Reference}}</h2>
  <p class="subtle">Propose a change to this seed. Review the current specs, describe your mutation, then approve the approach and UAT updates.</p>
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
	CSPNonce          string
	Reference         string
	ExecutionID       string
	Status            string
	ReasonCode        string
	Message           string
	RemediationTarget string
	RemediationHint   string
	RouteDescription  string
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
		runtimeService = interactions.NewRuntimeService(pool)
		store = feedbackloop.NewPostgresStore(pool)
		log.Print("feedback loop: persisting to runtime database")
		telemetry.NewServiceMonitor("webd", runtimeService).Start(ctx)
	}
	bootExecutionEnabled := runtimeService != nil && boolEnv("AS_BOOT_EXECUTION_ENABLED", false)

	mux := http.NewServeMux()
	mux.Handle("POST /feedback/incidents", jsontransport.NewIncidentIngestHandler(store))
	mux.Handle("GET /assets/", sproutAssetHandler())
	jsontransport.NewGrowthAPI(repoRoot, runtimeService).Register(mux)
	jsontransport.NewRegistryCatalogAPI(registryReader).Register(mux)
	if bootExecutionEnabled {
		jsontransport.NewExecutionAPI(repoRoot, runtimeService).RegisterPrefix(mux, "/boot")
	}
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		options, err := service.ListRealizations(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		executions := latestExecutionStateByReference(r.Context(), runtimeService)

		requestMeta := server.RequestMetadataFromContext(r.Context())
		view := newBootPageView(options, executions, bootExecutionEnabled, service.Remote != nil, runtimeService != nil, server.CSPNonceFromContext(r.Context()), boot.ClientFeedbackLoopScript(boot.FeedbackLoopScriptConfig{
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
		if err := growthTemplate.Execute(w, growthView{
			Packet:           packet,
			ExecutionEnabled: bootExecutionEnabled,
			Current:          latestExecutionModalState(r.Context(), runtimeService, packet.Reference),
		}); err != nil {
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
		if err := runTemplate.Execute(w, runView{
			Packet:           packet,
			ExecutionEnabled: bootExecutionEnabled,
			Current:          latestExecutionModalState(r.Context(), runtimeService, packet.Reference),
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
	mux.HandleFunc("GET /partials/mutate", func(w http.ResponseWriter, r *http.Request) {
		reference := strings.TrimSpace(r.URL.Query().Get("reference"))
		view := mutateView{IsNew: reference == ""}
		if reference != "" {
			packet, err := realizations.LoadGrowthContext(repoRoot, reference)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			view.Packet = packet
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := mutateTemplate.Execute(w, view); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
	mux.HandleFunc("GET /partials/registry", func(w http.ResponseWriter, r *http.Request) {
		catalog, err := registryReader.Catalog()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := registryTemplate.Execute(w, registryView{Catalog: catalog}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	baseDomain := config.EnvOrDefault("AS_BASE_DOMAIN", "localhost")
	handler := buildRoutingHandler(ctx, runtimeService, baseDomain, http.Handler(mux))
	if runtimeService != nil {
		handler = server.SessionResolutionMiddleware(server.RuntimeSessionResolver{Lookup: runtimeService}, handler)
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
	for _, binding := range bindings {
		executionID := strings.TrimSpace(binding.ExecutionID)
		if executionID == "" {
			continue
		}
		candidate := strings.TrimSpace(binding.PathPrefix)
		if candidate == "" {
			continue
		}
		if existing := strings.TrimSpace(out[executionID]); existing != "" && binding.BindingKind != "preview_path" {
			continue
		}
		out[executionID] = candidate
	}
	return out
}

func buildRoutingHandler(ctx context.Context, runtimeService *interactions.RuntimeService, baseDomain string, fallback http.Handler) http.Handler {
	if runtimeService == nil {
		log.Printf("realization routes: runtime service disabled; dynamic execution routing unavailable")
		return fallback
	}

	routeSource := newRuntimeRouteSource(runtimeService)
	suspensionSource := newRuntimeSuspensionSource(runtimeService)
	if routes, err := routeSource.routes(ctx); err == nil && len(routes) > 0 {
		log.Printf("realization routes: %d runtime (base domain %s)", len(routes), baseDomain)
	}
	if suspensions, err := suspensionSource.suspensions(ctx); err == nil && len(suspensions) > 0 {
		log.Printf("realization suspensions: %d active", len(suspensions))
	}
	return dynamicRealizationRoutingMiddleware(routeSource, suspensionSource, runtimeService, baseDomain, fallback)
}

// --- Realization routing (subdomain + path prefix) ---

type realizationRoute struct {
	Reference  string
	Subdomain  string
	PathPrefix string
	ProxyAddr  string
}

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
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}
	host = strings.ToLower(strings.TrimSpace(host))
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

func realizationRoutingMiddleware(routes []realizationRoute, suspensions []interactions.RealizationSuspension, runtimeService *interactions.RuntimeService, baseDomain string, fallback http.Handler) http.Handler {
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
		// Subdomain takes priority.
		if sub := extractSubdomain(r.Host, baseDomain); sub != "" {
			if route, ok := subdomainMap[sub]; ok {
				proxyToRealization(route, w, r)
				return
			}
			if suspension, ok := suspensionSubdomainMap[sub]; ok {
				renderSuspensionPage(w, r, suspension)
				return
			}
		}

		// Path prefix fallback.
		for _, route := range routes {
			if route.PathPrefix != "" && strings.HasPrefix(r.URL.Path, route.PathPrefix) {
				r2 := r.Clone(r.Context())
				r2.URL.Path = strings.TrimPrefix(r.URL.Path, strings.TrimSuffix(route.PathPrefix, "/"))
				if r2.URL.Path == "" {
					r2.URL.Path = "/"
				}
				r2.URL.RawPath = ""
				proxyToRealization(route, w, r2)
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

		fallback.ServeHTTP(w, r)
	})
}

func dynamicRealizationRoutingMiddleware(routeSource *runtimeRouteSource, suspensionSource *runtimeSuspensionSource, runtimeService *interactions.RuntimeService, baseDomain string, fallback http.Handler) http.Handler {
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
		realizationRoutingMiddleware(routes, suspensions, runtimeService, baseDomain, fallback).ServeHTTP(w, r)
	})
}

func isUnixSocketAddr(addr string) bool {
	return strings.HasPrefix(addr, "/") || strings.HasPrefix(addr, ".")
}

func proxyToRealization(route realizationRoute, w http.ResponseWriter, r *http.Request) {
	seedID, realizationID := realizations.SplitReference(route.Reference)
	r.Header.Set("X-AS-Seed-ID", seedID)
	r.Header.Set("X-AS-Realization-ID", realizationID)

	if isUnixSocketAddr(route.ProxyAddr) {
		target, _ := url.Parse("http://unix")
		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.Transport = &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", route.ProxyAddr)
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

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ServeHTTP(w, r)
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
		return false
	}
	renderUnavailablePage(w, r, realizationUnavailableView{
		CSPNonce:    server.CSPNonceFromContext(r.Context()),
		Reference:   execution.Reference,
		ExecutionID: execution.ExecutionID,
		Status:      execution.Status,
		ReasonCode:  "execution_" + strings.TrimSpace(execution.Status),
		Message: firstNonEmpty(
			strings.TrimSpace(execution.LastError),
			"This realization is not currently running. Fix the issue and relaunch it through the kernel.",
		),
		RouteDescription: strings.TrimSpace(execution.PreviewPathPrefix),
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
	})
}

func renderUnavailablePage(w http.ResponseWriter, _ *http.Request, view realizationUnavailableView) {
	if view.Reference == "" {
		view.Reference = "Unknown realization"
	}
	if view.Message == "" {
		view.Message = "This realization is not currently available."
	}
	var body bytes.Buffer
	if err := realizationUnavailableTemplate.Execute(&body, view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Robots-Tag", "noindex")
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
