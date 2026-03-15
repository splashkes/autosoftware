package main

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"os"
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
  <title>AS</title>
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
      width: min(52rem, calc(100vw - 2rem));
      padding: 1.25rem 0 2.5rem;
    }

    /* ── Branding ── */
    .brand {
      display: grid;
      justify-items: center;
      text-align: center;
      gap: 0.4rem;
      margin-bottom: 1.25rem;
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
    .gh-link {
      display: inline-flex;
      color: #8d94a0;
      margin-top: 0.35rem;
      transition: color 0.15s ease;
    }
    .gh-link:hover { color: #22a05a; }

    /* ── Meta pills ── */
    .console-meta {
      display: flex;
      justify-content: center;
      gap: 0.5rem;
      flex-wrap: wrap;
      margin: 0 0 1.25rem;
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

    /* ── Tile grid ── */
    .tile-grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(9rem, 1fr));
      gap: 0.75rem;
    }
    .tile {
      position: relative;
      aspect-ratio: 1;
      border: 1px solid #cfd4dc;
      background: rgba(245, 246, 248, 0.92);
      padding: 0.75rem;
      display: flex;
      flex-direction: column;
      justify-content: space-between;
      cursor: pointer;
      transition: all 0.28s cubic-bezier(0.4, 0, 0.2, 1);
      overflow: hidden;
      z-index: 1;
    }
    .tile:hover {
      border-color: #b0b6c0;
      box-shadow: 0 0.25rem 1rem rgba(28, 35, 48, 0.06);
    }
    .tile-face {
      display: flex;
      flex-direction: column;
      gap: 0.3rem;
    }
    .tile-name {
      font-size: 0.82rem;
      font-weight: 600;
      color: #222730;
      line-height: 1.35;
      overflow: hidden;
      display: -webkit-box;
      -webkit-line-clamp: 3;
      -webkit-box-orient: vertical;
    }
    .tile-count {
      font-size: 0.68rem;
      color: #8d94a0;
    }
    .tile-route {
      font-size: 0.62rem;
      color: #22a05a;
      font-family: monospace;
      letter-spacing: 0.02em;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .tile-foot {
      display: flex;
      align-items: center;
      gap: 0.35rem;
    }
    .tile-dot {
      width: 6px;
      height: 6px;
      border-radius: 50%;
      background: #8d94a0;
      flex-shrink: 0;
    }
    .tile-dot[data-status="published"],
    .tile-dot[data-status="accepted"] { background: #22a05a; }
    .tile-dot[data-status="draft"] { background: #d4a017; }
    .tile-dot[data-status="proposed"] { background: #2563eb; }
    .tile-dot[data-status="failed"],
    .tile-dot[data-status="error"] { background: #dc2626; }
    .tile-stage {
      font-size: 0.6rem;
      color: #8d94a0;
      text-transform: uppercase;
      letter-spacing: 0.04em;
    }

    /* ── Tile expanded (State 1) ── */
    .tile-expanded {
      display: none;
    }
    .tile.is-expanded {
      aspect-ratio: auto;
      grid-column: span 2;
      grid-row: span 2;
      z-index: 10;
      border-color: #22a05a;
      background: rgba(255, 255, 255, 0.96);
      box-shadow: 0 0.5rem 2rem rgba(28, 35, 48, 0.12);
      cursor: default;
    }
    .tile.is-expanded .tile-expanded {
      display: flex;
      flex-direction: column;
      gap: 0.55rem;
      margin-top: 0.65rem;
      animation: tile-reveal 0.2s ease-out;
    }
    .tile-grid.has-expanded .tile:not(.is-expanded) {
      opacity: 0.35;
      pointer-events: none;
      filter: grayscale(0.3);
      transition: opacity 0.25s ease, filter 0.25s ease;
    }
    @keyframes tile-reveal {
      from { opacity: 0; transform: translateY(4px); }
      to   { opacity: 1; transform: translateY(0); }
    }
    .tile-summary {
      margin: 0;
      color: #69707c;
      font-size: 0.78rem;
      line-height: 1.5;
    }
    .tile-meta {
      display: flex;
      gap: 0.35rem;
      flex-wrap: wrap;
    }
    .tile-actions {
      display: flex;
      gap: 0.4rem;
      flex-wrap: wrap;
      margin-top: 0.25rem;
    }

    /* Seed tile children (multi-realization) */
    .tile-children {
      display: grid;
      gap: 0.45rem;
    }
    .tile-child {
      display: flex;
      align-items: center;
      gap: 0.5rem;
      padding: 0.45rem 0.5rem;
      border: 1px solid #e0e3e8;
      background: rgba(248, 249, 251, 0.8);
    }
    .tile-child-name {
      flex: 1;
      font-size: 0.76rem;
      color: #3a4250;
      font-weight: 500;
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .tile-child .tile-actions {
      margin-top: 0;
      flex-shrink: 0;
    }

    /* ── Shared badge styles ── */
    .status,
    .readiness {
      display: inline-flex;
      align-items: center;
      gap: 0.35rem;
      padding: 0.18rem 0.48rem;
      border-radius: 999px;
      border: 1px solid #cbd1da;
      background: rgba(255, 255, 255, 0.78);
      font-size: 0.64rem;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      line-height: 1;
    }
    .status { color: #2f855a; }
    .status.draft { color: #9a6700; }
    .status.published, .status.accepted { color: #15803d; }
    .status.failed, .status.error { color: #b91c1c; }
    .readiness.defined { color: #1d4ed8; }
    .readiness.runnable, .readiness.accepted { color: #15803d; }
    .readiness.bootstrap { color: #7c3aed; }
    .readiness.designed { color: #9a6700; }

    /* ── Action buttons ── */
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

    /* ── Full-screen modal (State 2) ── */
    .modal-backdrop {
      position: fixed;
      inset: 0;
      z-index: 1000;
      display: none;
      align-items: center;
      justify-content: center;
      background: rgba(0, 0, 0, 0.15);
      backdrop-filter: blur(18px);
      -webkit-backdrop-filter: blur(18px);
      opacity: 0;
      transition: opacity 0.3s ease;
    }
    .modal-backdrop.is-visible {
      display: flex;
      opacity: 1;
    }
    .modal-shell {
      position: relative;
      width: min(52rem, calc(100vw - 2rem));
      max-height: calc(100vh - 3rem);
      overflow-y: auto;
      background: rgba(255, 255, 255, 0.95);
      box-shadow: 0 2rem 4rem rgba(28, 35, 48, 0.18);
      padding: 1.5rem;
      animation: modal-enter 0.3s cubic-bezier(0.16, 1, 0.3, 1);
    }
    .modal-close {
      position: sticky;
      top: 0;
      float: right;
      background: none;
      border: none;
      font-size: 1.5rem;
      color: #7a818d;
      cursor: pointer;
      padding: 0.25rem 0.5rem;
      z-index: 2;
      line-height: 1;
    }
    .modal-close:hover { color: #222730; }
    @keyframes modal-enter {
      from { opacity: 0; transform: scale(0.96) translateY(12px); }
      to   { opacity: 1; transform: scale(1) translateY(0); }
    }

    /* ── RUN mode (immersive transition) ── */
    .modal-backdrop.is-run-mode {
      background: rgba(0, 0, 0, 0.6);
      backdrop-filter: blur(28px);
      -webkit-backdrop-filter: blur(28px);
    }
    .modal-backdrop.is-run-mode .modal-shell {
      width: 100vw;
      max-width: 100vw;
      max-height: 100vh;
      height: 100vh;
      border: none;
      box-shadow: none;
      padding: 0;
      animation: run-enter 0.5s cubic-bezier(0.16, 1, 0.3, 1);
    }
    .modal-backdrop.is-run-mode .modal-close {
      position: fixed;
      top: 1rem;
      right: 1rem;
      color: rgba(255, 255, 255, 0.7);
      z-index: 1001;
    }
    .modal-backdrop.is-run-mode .modal-close:hover { color: #fff; }
    @keyframes run-enter {
      from { opacity: 0; transform: scale(1.05); filter: blur(4px); }
      to   { opacity: 1; transform: scale(1); filter: blur(0); }
    }

    /* ── Sprout FAB ── */
    .sprout-fab {
      position: fixed;
      bottom: 1.5rem;
      right: 1.5rem;
      width: 3rem;
      height: 3rem;
      border-radius: 50%;
      border: 1px solid #22a05a;
      background: rgba(34, 160, 90, 0.08);
      color: #22a05a;
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: center;
      z-index: 100;
      transition: all 0.2s ease;
      box-shadow: 0 0.25rem 1rem rgba(34, 160, 90, 0.15);
    }
    .sprout-fab:hover {
      background: rgba(34, 160, 90, 0.18);
      transform: scale(1.08);
    }

    /* ── Styles used by partials loaded into modal ── */
    .indicator {
      display: grid;
      align-content: center;
      min-height: 12rem;
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
    .stack { display: grid; gap: 0.85rem; }
    .row {
      display: flex;
      gap: 0.75rem;
      align-items: center;
      justify-content: space-between;
      flex-wrap: wrap;
    }
    .meta { display: flex; gap: 0.45rem; flex-wrap: wrap; }
    .subtle { color: #7a818d; font-size: 0.76rem; line-height: 1.5; }
    .source { border-top: 1px solid #d4d8df; padding-top: 0.85rem; }
    .source h3 { margin: 0 0 0.25rem; font-size: 0.9rem; color: #222730; }
    .pathline {
      color: #848b96;
      font-size: 0.74rem;
      line-height: 1.5;
      word-break: break-word;
    }
    .form-grid { display: grid; gap: 0.75rem; }
    .form-grid.two-up { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .field { display: grid; gap: 0.32rem; }
    .field label {
      color: #4b5563;
      font-size: 0.74rem;
      letter-spacing: 0.04em;
      text-transform: uppercase;
    }
    input[type="text"], select, textarea {
      width: 100%;
      border: 1px solid #cfd4dc;
      background: rgba(255, 255, 255, 0.9);
      color: #222730;
      font: inherit;
      padding: 0.62rem 0.7rem;
      border-radius: 0;
    }
    textarea { min-height: 7rem; resize: vertical; line-height: 1.55; }
    .checkbox-row {
      display: flex;
      align-items: center;
      gap: 0.55rem;
      font-size: 0.78rem;
      color: #4b5563;
    }
    .checkbox-row input { margin: 0; }
    .doc-grid { display: grid; gap: 0.65rem; }
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
    details.doc summary::-webkit-details-marker { display: none; }
    details.doc summary::after {
      content: "open";
      float: right;
      color: #9aa1ac;
      font-size: 0.62rem;
      letter-spacing: 0.05em;
      text-transform: uppercase;
    }
    details.doc[open] summary::after { content: "close"; }
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

    /* ── Footer + status ── */
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
    .footer code { color: #4f5664; }

    /* ── Responsive ── */
    @media (max-width: 720px) {
      .page { width: min(52rem, calc(100vw - 1rem)); }
      .tile-grid {
        grid-template-columns: repeat(auto-fill, minmax(7rem, 1fr));
      }
      .tile.is-expanded { grid-column: 1 / -1; }
      .form-grid.two-up { grid-template-columns: 1fr; }
    }

    /* ── Reduced motion ── */
    @media (prefers-reduced-motion: reduce) {
      .tile, .modal-backdrop, .modal-shell { transition: none; animation: none; }
      .tile.is-expanded .tile-expanded { animation: none; }
    }
  </style>
</head>
<body>
  <main class="page">
    <section class="brand">
      <div class="sprout-logo-shell" data-sprout-logo aria-hidden="true"></div>
      <div class="wordmark">AS</div>
      <div class="tagline">autosoftware</div>
      <p class="lede">Software that evolves from within.</p>
      {{if .GitHubURL}}<a class="gh-link" href="{{.GitHubURL}}" target="_blank" rel="noopener" aria-label="GitHub repository">
        <svg width="20" height="20" viewBox="0 0 16 16" fill="currentColor"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/></svg>
      </a>{{end}}
    </section>

    <div class="console-meta">
      <span class="pill">{{len .Seeds}} seeds</span>
      <span class="pill">{{.RealizationCount}} realizations</span>
      <span class="pill">{{.GrowthReadyCount}} growth-ready</span>
      <span class="pill">{{.RunnableCount}} runnable</span>
      {{if .RuntimeConfigured}}<span class="pill">runtime on</span>{{else}}<span class="pill">runtime off</span>{{end}}
      {{if .RemoteConfigured}}<span class="pill">remote on</span>{{else}}<span class="pill">remote off</span>{{end}}
      <button class="action-button" id="registry-open" type="button">Registry</button>
    </div>

    <section class="tile-grid" id="tile-grid">
      {{range .Seeds}}
      {{if .IsSingleRealization}}
        {{with index .Realizations 0}}
        <article class="tile" data-tile data-tile-type="realization"
                 data-reference="{{.Reference}}" data-label="{{.Summary}}"
                 data-can-run="{{.CanRun}}" tabindex="0">
          <div class="tile-face">
            <span class="tile-name">{{.Summary}}</span>
            {{if .Subdomain}}<span class="tile-route">{{.Subdomain}}</span>
            {{else if .PathPrefix}}<span class="tile-route">{{.PathPrefix}}</span>{{end}}
          </div>
          <div class="tile-foot">
            <span class="tile-dot" data-status="{{.Status}}"></span>
            <span class="tile-stage">{{.ReadinessLabel}}</span>
          </div>
          <div class="tile-expanded" aria-hidden="true">
            <p class="tile-summary">{{.ReadinessSummary}}</p>
            <div class="tile-meta">
              <span class="status {{.Status}}">{{.Status}}</span>
              <span class="readiness {{.ReadinessStage}}">{{.ReadinessLabel}}</span>
              {{if .SurfaceKind}}<span class="pill">{{.SurfaceKind}}</span>{{end}}
            </div>
            <div class="tile-actions">
              <button class="action-button" type="button" data-action="inspect" data-reference="{{.Reference}}" data-label="{{.Summary}}">Inspect</button>
              <button class="action-button is-primary" type="button" data-action="grow" data-reference="{{.Reference}}" data-label="{{.Summary}}">Grow</button>
              {{if .CanRun}}<button class="action-button" type="button" data-action="run" data-reference="{{.Reference}}" data-label="{{.Summary}}"{{if .CanLaunchLocal}} data-launchable="true"{{end}}{{if .ExecutionOpenPath}} data-open-path="{{.ExecutionOpenPath}}"{{end}}>{{if .ExecutionOpenPath}}Open{{else if .CanLaunchLocal}}Run{{else}}Show Run{{end}}</button>
              {{else}}<button class="action-button" type="button" disabled>Run</button>{{end}}
            </div>
          </div>
        </article>
        {{end}}
      {{else}}
        <article class="tile tile-seed" data-tile data-tile-type="seed" data-seed-id="{{.SeedID}}" tabindex="0">
          <div class="tile-face">
            <span class="tile-name">{{.DisplayName}}</span>
            <span class="tile-count">{{.Count}} realizations</span>
          </div>
          <div class="tile-foot">
            <span class="tile-dot" data-status="{{.Status}}"></span>
            <span class="tile-stage">{{.Status}}</span>
          </div>
          <div class="tile-expanded" aria-hidden="true">
            {{if .Summary}}<p class="tile-summary">{{.Summary}}</p>{{end}}
            <div class="tile-children">
              {{range .Realizations}}
              <div class="tile-child">
                <span class="tile-dot" data-status="{{.Status}}"></span>
                <span class="tile-child-name">{{.Summary}}</span>
                <span class="readiness {{.ReadinessStage}}">{{.ReadinessLabel}}</span>
                <div class="tile-actions">
                  <button class="action-button" type="button" data-action="inspect" data-reference="{{.Reference}}" data-label="{{.Summary}}">Inspect</button>
                  <button class="action-button is-primary" type="button" data-action="grow" data-reference="{{.Reference}}" data-label="{{.Summary}}">Grow</button>
                  {{if .CanRun}}<button class="action-button" type="button" data-action="run" data-reference="{{.Reference}}" data-label="{{.Summary}}"{{if .CanLaunchLocal}} data-launchable="true"{{end}}{{if .ExecutionOpenPath}} data-open-path="{{.ExecutionOpenPath}}"{{end}}>{{if .ExecutionOpenPath}}Open{{else if .CanLaunchLocal}}Run{{else}}Show Run{{end}}</button>
                  {{else}}<button class="action-button" type="button" disabled>Run</button>{{end}}
                </div>
              </div>
              {{end}}
            </div>
          </div>
        </article>
      {{end}}
      {{end}}
    </section>

    <button class="sprout-fab" id="sprout-fab" type="button" aria-label="Plant a new seed" data-sprout-trigger>
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M12 22v-8"/>
        <path d="M12 14c-4 0-8-4-8-8 4 0 8 4 8 8z"/>
        <path d="M12 14c4 0 8-4 8-8-4 0-8 4-8 8z"/>
      </svg>
    </button>

    <div class="modal-backdrop" id="modal-backdrop" aria-hidden="true">
      <div class="modal-shell" id="modal-shell">
        <button class="modal-close" id="modal-close" type="button" aria-label="Close">&times;</button>
        <div class="modal-content" id="modal-content"></div>
      </div>
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
	ExecutionEnabled  bool
	RemoteConfigured  bool
	RuntimeConfigured bool
	GitHubURL         string
	CSPNonce          string
	LoaderScript      template.JS
	FeedbackScript    template.JS
}

type seedBootView struct {
	SeedID              string
	DisplayName         string
	Summary             string
	Status              string
	Count               int
	GrowthReadyCount    int
	RunnableCount       int
	InitiallyOpen       bool
	IsSingleRealization bool
	Realizations        []realizationBootView
}

type realizationBootView struct {
	Reference         string
	RealizationID     string
	ApproachID        string
	Summary           string
	Status            string
	SurfaceKind       string
	ReadinessStage    string
	ReadinessLabel    string
	ReadinessSummary  string
	HasContract       bool
	HasRuntime        bool
	CanRun            bool
	CanLaunchLocal    bool
	ExecutionStatus   string
	ExecutionID       string
	ExecutionOpenPath string
	Subdomain         string
	PathPrefix        string
}

type executionBootState struct {
	ExecutionID string
	Status      string
	OpenPath    string
}

func newBootPageView(options []materializer.RealizationOption, executions map[string]executionBootState, executionEnabled, remoteConfigured, runtimeConfigured bool, nonce string, feedbackScript string) bootPageView {
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
		execution := executions[option.Reference]

		item := realizationBootView{
			Reference:         option.Reference,
			RealizationID:     option.RealizationID,
			ApproachID:        option.ApproachID,
			Summary:           firstNonEmpty(strings.TrimSpace(option.Summary), option.RealizationID),
			Status:            firstNonEmpty(strings.TrimSpace(option.Status), "draft"),
			SurfaceKind:       strings.TrimSpace(option.SurfaceKind),
			ReadinessStage:    readinessStage,
			ReadinessLabel:    readinessLabel,
			ReadinessSummary:  readinessSummary,
			HasContract:       option.Readiness.HasContract,
			HasRuntime:        option.Readiness.HasRuntime,
			CanRun:            option.Readiness.CanRun,
			CanLaunchLocal:    executionEnabled && option.Readiness.CanLaunchLocal,
			ExecutionStatus:   execution.Status,
			ExecutionID:       execution.ExecutionID,
			ExecutionOpenPath: execution.OpenPath,
			Subdomain:         option.Subdomain,
			PathPrefix:        option.PathPrefix,
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

	for i := range seeds {
		seeds[i].IsSingleRealization = len(seeds[i].Realizations) == 1
	}

	return bootPageView{
		Seeds:             seeds,
		RealizationCount:  len(options),
		GrowthReadyCount:  growthReadyCount,
		RunnableCount:     runnableCount,
		ExecutionEnabled:  executionEnabled,
		RemoteConfigured:  remoteConfigured,
		RuntimeConfigured: runtimeConfigured,
		GitHubURL:         envOrDefault("AS_GITHUB_URL", "https://github.com/anthropics/autosoftware"),
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

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
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
  var grid = document.getElementById("tile-grid");
  var backdrop = document.getElementById("modal-backdrop");
  var modalShell = document.getElementById("modal-shell");
  var modalContent = document.getElementById("modal-content");
  var modalCloseBtn = document.getElementById("modal-close");
  var sproutFab = document.getElementById("sprout-fab");
  var registryOpen = document.getElementById("registry-open");
  var status = document.getElementById("console-status");
  if (!grid || !backdrop || !modalContent) return;

  var expandedTile = null;
  var modalOpen = false;

  function escapeHTML(v) {
    return String(v).replace(/[&<>"]/g, function (c) {
      return {"&":"&amp;","<":"&lt;",">":"&gt;","\"":"&quot;"}[c];
    });
  }

  function setStatus(copy) {
    if (status) status.textContent = copy || "";
  }

  async function waitForExecution(projectionPath, label) {
    for (var attempt = 0; attempt < 60; attempt++) {
      var response = await fetch(projectionPath, {
        method: "GET",
        credentials: "same-origin",
        headers: { "Accept": "application/json" }
      });
      var result = await response.json();
      if (!response.ok) {
        throw new Error(result.error || ("Execution poll failed: " + response.status));
      }
      var session = result.session || {};
      if (session.status === "healthy" && session.open_path) {
        setStatus("Running " + label + ".");
        var launched = window.open(session.open_path, "_blank", "noopener");
        if (!launched) window.location.assign(session.open_path);
        return session;
      }
      if (session.status === "failed" || session.status === "stopped") {
        throw new Error(session.last_error || ("Execution " + session.status));
      }
      setStatus("Launching " + label + " (" + (session.status || "starting") + ")...");
      await new Promise(function (resolve) { setTimeout(resolve, 500); });
    }
    throw new Error("Execution timed out before becoming healthy.");
  }

  async function launchRealization(reference, label) {
    ensureModal(true);
    modalContent.innerHTML =
      '<div class="indicator">' +
      '<div class="indicator-title">Launching</div>' +
      '<p class="indicator-copy">Starting ' + escapeHTML(label || reference) + ' through the kernel execution layer.</p></div>';
    setStatus("Launching " + (label || reference) + "...");

    var response = await fetch("/boot/commands/realizations.launch", {
      method: "POST",
      credentials: "same-origin",
      headers: { "Accept": "application/json", "Content-Type": "application/json" },
      body: JSON.stringify({ reference: reference })
    });
    var result = await response.json();
    if (!response.ok) {
      throw new Error(result.error || ("Launch failed: " + response.status));
    }
    return waitForExecution(result.projection, label || reference);
  }

  /* ── Tile expand / collapse (State 0 ↔ 1) ── */

  function expandTile(tile) {
    collapseAll();
    tile.classList.add("is-expanded");
    var exp = tile.querySelector(".tile-expanded");
    if (exp) exp.setAttribute("aria-hidden", "false");
    grid.classList.add("has-expanded");
    expandedTile = tile;
  }

  function collapseAll() {
    if (!expandedTile) return;
    expandedTile.classList.remove("is-expanded");
    var exp = expandedTile.querySelector(".tile-expanded");
    if (exp) exp.setAttribute("aria-hidden", "true");
    expandedTile = null;
    grid.classList.remove("has-expanded");
  }

  /* ── Modal (State 2) ── */

  function ensureModal(isRun) {
    backdrop.classList.toggle("is-run-mode", isRun);

    if (!modalOpen) {
      backdrop.style.display = "flex";
      requestAnimationFrame(function () {
        backdrop.classList.add("is-visible");
      });
      document.body.style.overflow = "hidden";
      modalOpen = true;
    }

    backdrop.setAttribute("aria-hidden", "false");
  }

  function openModal(action, reference, label) {
    var isRun = action === "run";
    ensureModal(isRun);

    modalContent.innerHTML =
      '<div class="indicator">' +
      '<div class="indicator-title">' + escapeHTML(action === "inspect" ? "Inspecting" : action === "grow" ? "Preparing Growth" : action === "run" ? "Launching" : action === "registry" ? "Loading Registry" : "Loading") + '</div>' +
      '<p class="indicator-copy">Preparing ' + escapeHTML(label || reference || "content") + '...</p></div>';

    if (action === "inspect" || action === "grow" || action === "run" || action === "registry") {
      loadPartialIntoModal(action, reference, label);
    } else if (action === "create") {
      loadMutationWizard("", label);
    } else if (action === "mutate" && reference) {
      loadMutationWizard(reference, label);
    }
  }

  function closeModal() {
    backdrop.classList.remove("is-visible", "is-run-mode");
    backdrop.setAttribute("aria-hidden", "true");
    setTimeout(function () {
      backdrop.style.display = "none";
      modalContent.innerHTML = "";
      document.body.style.overflow = "";
      modalOpen = false;
    }, 300);
  }

  /* ── Partial loading into modal ── */

  async function loadPartialIntoModal(action, reference, label) {
    var path;
    if (action === "inspect") {
      path = "/partials/materialization?reference=" + encodeURIComponent(reference);
    } else if (action === "grow") {
      path = "/partials/grow?reference=" + encodeURIComponent(reference);
    } else if (action === "run") {
      path = "/partials/run?reference=" + encodeURIComponent(reference);
    } else if (action === "registry") {
      path = "/partials/registry";
    } else {
      return;
    }

    setStatus((action === "inspect" ? "Inspecting " : action === "grow" ? "Preparing growth for " : action === "run" ? "Launching " : "Loading ") + (label || reference || "registry") + "...");

    try {
      var response = await fetch(path, {
        method: "GET",
        credentials: "same-origin",
        headers: { "Accept": "text/html" }
      });
      var html = await response.text();
      if (!response.ok) {
        throw new Error(html || ("Request failed: " + response.status));
      }
      modalContent.innerHTML = html;
      setStatus((action === "inspect" ? "Inspecting " : action === "grow" ? "Ready to grow " : action === "run" ? "Run loaded for " : "Loaded ") + (label || reference || "registry") + ".");
    } catch (err) {
      modalContent.innerHTML =
        '<div class="stack">' +
        '<div class="indicator-title">Request Failed</div>' +
        '<p class="indicator-copy">' + escapeHTML(err && err.message ? err.message : String(err)) + '</p></div>';
      setStatus("Request failed.");
      console.error(err);
    }
  }

  /* ── Mutation wizard (loaded into modal) ── */

  async function loadMutationWizard(reference, label) {
    var path = "/partials/mutate";
    if (reference) path += "?reference=" + encodeURIComponent(reference);
    setStatus(reference ? "Loading mutation wizard for " + (label || reference) + "..." : "Starting new seed wizard...");

    try {
      var response = await fetch(path, {
        method: "GET",
        credentials: "same-origin",
        headers: { "Accept": "text/html" }
      });
      var html = await response.text();
      if (!response.ok) {
        throw new Error(html || ("Request failed: " + response.status));
      }
      modalContent.innerHTML = html;
      initWizardSteps();
      setStatus(reference ? "Mutation wizard ready." : "New seed wizard ready.");
    } catch (err) {
      modalContent.innerHTML =
        '<div class="stack">' +
        '<h2 style="margin:0;font-size:1.1rem;">' + escapeHTML(reference ? "Mutate Seed" : "Create from Bare Earth") + '</h2>' +
        '<p class="indicator-copy">The mutation wizard is not yet available. Use the CLI: <code>as seed create</code></p></div>';
      setStatus("Wizard endpoint not available yet.");
    }
  }

  function initWizardSteps() {
    var steps = modalContent.querySelectorAll("[data-wizard-step]");
    if (steps.length === 0) return;
    var current = 0;
    function show(index) {
      steps.forEach(function (s, i) {
        s.style.display = i === index ? "block" : "none";
      });
      current = index;
    }
    show(0);
    modalContent.addEventListener("click", function (e) {
      var next = e.target.closest("[data-wizard-next]");
      if (next && current < steps.length - 1) { show(current + 1); return; }
      var prev = e.target.closest("[data-wizard-prev]");
      if (prev && current > 0) { show(current - 1); }
    });
  }

  /* ── Growth form submission (inside modal) ── */

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

    modalContent.innerHTML = [
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

    modalContent.innerHTML =
      '<div class="indicator">' +
      '<div class="indicator-title">Queueing Growth</div>' +
      '<p class="indicator-copy">Writing a growth job into the shared runtime queue.</p></div>';
    setStatus("Queueing growth for " + reference + "...");

    try {
      var commandResponse = await fetch("/v1/commands/realizations.grow", {
        method: "POST",
        credentials: "same-origin",
        headers: { "Accept": "application/json", "Content-Type": "application/json" },
        body: JSON.stringify(payload)
      });
      var commandResult = await commandResponse.json();
      if (!commandResponse.ok) {
        throw new Error(commandResult.error || ("Queue failed: " + commandResponse.status));
      }
      var projectionResponse = await fetch(commandResult.projection, {
        method: "GET",
        credentials: "same-origin",
        headers: { "Accept": "application/json" }
      });
      var projection = await projectionResponse.json();
      if (!projectionResponse.ok) {
        throw new Error(projection.error || ("Projection failed: " + projectionResponse.status));
      }
      renderJobResult(projection);
      setStatus("Queued growth for " + (commandResult.target_reference || reference) + ".");
    } catch (err) {
      modalContent.innerHTML =
        '<div class="stack">' +
        '<div class="indicator-title">Growth Failed</div>' +
        '<p class="indicator-copy">' + escapeHTML(err && err.message ? err.message : String(err)) + '</p></div>';
      setStatus("Growth request failed.");
      console.error(err);
    }
  }

  async function stopExecution(executionID, label) {
    ensureModal(true);
    modalContent.innerHTML =
      '<div class="indicator">' +
      '<div class="indicator-title">Stopping</div>' +
      '<p class="indicator-copy">Stopping ' + escapeHTML(label || executionID) + '...</p></div>';
    setStatus("Stopping " + (label || executionID) + "...");

    var response = await fetch("/boot/commands/realizations.stop", {
      method: "POST",
      credentials: "same-origin",
      headers: { "Accept": "application/json", "Content-Type": "application/json" },
      body: JSON.stringify({ execution_id: executionID })
    });
    var result = await response.json();
    if (!response.ok) {
      throw new Error(result.error || ("Stop failed: " + response.status));
    }
    modalContent.innerHTML =
      '<div class="stack"><div class="indicator-title">Stopped</div><p class="indicator-copy">' +
      escapeHTML(label || executionID) +
      ' has been asked to stop.</p></div>';
    setStatus("Stopped " + (label || executionID) + ".");
  }

  /* ── Event delegation ── */

  document.addEventListener("click", function (event) {
    // Action buttons (Inspect / Grow / Run) → open modal
    var actionBtn = event.target.closest("[data-action][data-reference]");
    if (actionBtn) {
      event.preventDefault();
      event.stopPropagation();
      if (actionBtn.getAttribute("data-action") === "run") {
        if (actionBtn.getAttribute("data-open-path")) {
          var openPath = actionBtn.getAttribute("data-open-path");
          var launched = window.open(openPath, "_blank", "noopener");
          if (!launched) window.location.assign(openPath);
          setStatus("Opening " + (actionBtn.getAttribute("data-label") || actionBtn.getAttribute("data-reference")) + "...");
          return;
        }
        if (actionBtn.getAttribute("data-launchable") === "true") {
          launchRealization(
            actionBtn.getAttribute("data-reference"),
            actionBtn.getAttribute("data-label") || actionBtn.getAttribute("data-reference")
          ).catch(function (err) {
            modalContent.innerHTML =
              '<div class="stack"><div class="indicator-title">Launch Failed</div><p class="indicator-copy">' +
              escapeHTML(err && err.message ? err.message : String(err)) +
              '</p></div>';
            setStatus("Launch failed.");
            console.error(err);
          });
          return;
        }
      }
      openModal(
        actionBtn.getAttribute("data-action"),
        actionBtn.getAttribute("data-reference"),
        actionBtn.getAttribute("data-label") || actionBtn.getAttribute("data-reference")
      );
      return;
    }

    var stopBtn = event.target.closest("[data-stop-execution]");
    if (stopBtn) {
      event.preventDefault();
      event.stopPropagation();
      stopExecution(
        stopBtn.getAttribute("data-stop-execution"),
        stopBtn.getAttribute("data-label") || stopBtn.getAttribute("data-stop-execution")
      ).catch(function (err) {
        modalContent.innerHTML =
          '<div class="stack"><div class="indicator-title">Stop Failed</div><p class="indicator-copy">' +
          escapeHTML(err && err.message ? err.message : String(err)) +
          '</p></div>';
        setStatus("Stop failed.");
        console.error(err);
      });
      return;
    }

    // Tile click → expand/collapse
    var tile = event.target.closest("[data-tile]");
    if (tile && !modalOpen) {
      event.preventDefault();
      if (tile.classList.contains("is-expanded")) {
        collapseAll();
      } else {
        expandTile(tile);
      }
      return;
    }

    // Click outside expanded tile → collapse
    if (expandedTile && !event.target.closest(".tile.is-expanded") && !modalOpen) {
      collapseAll();
    }

    // Toggle create_new checkbox inside modal
    var toggleNew = event.target.closest("[data-toggle-create-new]");
    if (toggleNew) {
      var form = toggleNew.closest("form");
      if (form) {
        var group = form.querySelector("[data-new-realization-fields]");
        if (group) group.style.display = form.elements.create_new.checked ? "grid" : "none";
      }
    }
  });

  // Sprout FAB → open create wizard
  if (sproutFab) {
    sproutFab.addEventListener("click", function (event) {
      event.stopPropagation();
      openModal("create", "", "New Seed");
    });
  }
  if (registryOpen) {
    registryOpen.addEventListener("click", function (event) {
      event.stopPropagation();
      openModal("registry", "", "Registry");
    });
  }

  // Modal close
  if (modalCloseBtn) {
    modalCloseBtn.addEventListener("click", closeModal);
  }
  backdrop.addEventListener("click", function (event) {
    if (event.target === backdrop) closeModal();
  });
  document.addEventListener("keydown", function (event) {
    if (event.key === "Escape") {
      if (modalOpen) {
        closeModal();
      } else if (expandedTile) {
        collapseAll();
      }
    }
    // Enter on focused tile → expand
    if (event.key === "Enter" && document.activeElement && document.activeElement.hasAttribute("data-tile")) {
      event.preventDefault();
      if (document.activeElement.classList.contains("is-expanded")) {
        collapseAll();
      } else {
        expandTile(document.activeElement);
      }
    }
  });

  // Growth form submission (inside modal)
  document.addEventListener("submit", function (event) {
    var form = event.target.closest("[data-growth-form]");
    if (!form) return;
    event.preventDefault();
    submitGrowthForm(form);
  });
})();`
}
