package main

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"unicode"

	"as/kernel/internal/materializer"
	registrycatalog "as/kernel/internal/registry"
)

//go:embed assets/sprout-logo.css assets/sprout-logo.js assets/launch-state.js
var bootloaderAssets embed.FS

var bootPageTemplate = template.Must(template.New("boot-page").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Autosoftware</title>
  <link rel="stylesheet" href="/assets/sprout-logo.css">
  <style nonce="{{.CSPNonce}}">
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
      background: #eef0f3;
      color: #2a2d35;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
      display: flex;
      justify-content: center;
    }
    .page {
      width: min(36rem, calc(100vw - 2rem));
      padding: 1.25rem 0 2.5rem;
    }

    .brand {
      display: grid;
      justify-items: center;
      text-align: center;
      gap: 0.4rem;
      margin-bottom: 1.1rem;
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
    .lede,
    .intro,
    .catalog-text {
      margin: 0;
      color: #69707c;
      font-size: 0.82rem;
      line-height: 1.6;
    }
    .lede {
      max-width: 19rem;
      margin-top: 0.2rem;
    }
    .gh-link {
      display: inline-flex;
      color: #8d94a0;
      margin-top: 0.35rem;
      transition: color 0.15s ease;
    }
    .gh-link:hover { color: #22a05a; }

    .intro {
      max-width: 31rem;
      margin: 0 auto 1rem;
      text-align: center;
    }

    .pill {
      display: inline-flex;
      align-items: center;
      gap: 0.35rem;
      padding: 0.24rem 0.62rem;
      border-radius: 999px;
      border: 1px solid #c8cdd6;
      background: rgba(255, 255, 255, 0.72);
      color: #6c7380;
      font-size: 0.68rem;
      letter-spacing: 0.04em;
      text-transform: uppercase;
    }

    .hero-note {
      margin: 0 auto 1.5rem;
      max-width: 34rem;
      padding: 0.85rem 1rem;
      border: 1px solid #d3d9e1;
      background: rgba(255, 255, 255, 0.75);
      color: #5c6370;
      font-size: 0.78rem;
      line-height: 1.6;
      text-align: center;
    }

    .featured-shell,
    .catalog-shell,
    .agent-shell {
      margin-top: 1.2rem;
    }
    .section-head,
    .catalog-head {
      display: flex;
      align-items: end;
      justify-content: space-between;
      gap: 0.8rem;
      margin-bottom: 0.75rem;
    }
    .section-copy,
    .catalog-copy {
      min-width: 0;
    }
    .section-title,
    .catalog-title {
      margin: 0 0 0.2rem;
      color: #2a2d35;
      font-size: 0.74rem;
      letter-spacing: 0.14em;
      text-transform: uppercase;
      font-weight: 600;
    }
    .featured-grid {
      display: grid;
      grid-template-columns: repeat(3, minmax(0, 1fr));
      gap: 0.8rem;
    }
    .featured-card,
    .tile {
      position: relative;
      border: 1px solid hsl(var(--seed-hue) 22% 82%);
      background: linear-gradient(180deg, hsl(var(--seed-hue) 48% 98%) 0%, rgba(255, 255, 255, 0.96) 100%);
      box-shadow: 0 0.9rem 2rem rgba(23, 29, 38, 0.06);
    }
    .featured-card {
      padding: 1rem;
      display: grid;
      gap: 0.8rem;
      min-height: 15rem;
    }
    .featured-card::before,
    .tile::before {
      content: "";
      position: absolute;
      inset: 0 auto 0 0;
      width: 4px;
      background: hsl(var(--seed-hue) 62% 46%);
    }
    .featured-kicker {
      display: inline-flex;
      align-items: center;
      width: fit-content;
      gap: 0.35rem;
      padding: 0.18rem 0.48rem;
      border-radius: 999px;
      background: hsl(var(--seed-hue) 75% 96%);
      color: hsl(var(--seed-hue) 48% 32%);
      font-size: 0.62rem;
      font-weight: 600;
      letter-spacing: 0.08em;
      text-transform: uppercase;
    }
    .featured-card h3 {
      margin: 0;
      color: #1d2430;
      font-size: 1.05rem;
      line-height: 1.2;
    }
    .featured-summary {
      margin: 0;
      color: #616977;
      font-size: 0.8rem;
      line-height: 1.55;
    }
    .footprint-label {
      font-size: 0.63rem;
      font-weight: 600;
      letter-spacing: 0.09em;
      text-transform: uppercase;
      color: #77808e;
      margin-bottom: 0.3rem;
    }
    .footprint-track {
      width: 100%;
      height: 0.58rem;
      background: #e4e8ee;
      overflow: hidden;
      border-radius: 999px;
    }
    .footprint-fill {
      display: flex;
      height: 100%;
      min-width: 1px;
    }
    .footprint-segment.objects,
    .metric-dot.objects {
      background: hsl(var(--seed-hue) 64% 38%);
    }
    .footprint-segment.commands,
    .metric-dot.commands {
      background: hsl(var(--seed-hue) 68% 47%);
    }
    .footprint-segment.projections,
    .metric-dot.projections {
      background: hsl(var(--seed-hue) 72% 58%);
    }
    .footprint-segment.realizations,
    .metric-dot.realizations {
      background: hsl(var(--seed-hue) 68% 72%);
    }
    .metric-row {
      display: flex;
      flex-wrap: wrap;
      gap: 0.45rem 0.7rem;
      margin-top: 0.55rem;
      color: #616977;
      font-size: 0.69rem;
      line-height: 1.4;
    }
    .metric {
      display: inline-flex;
      align-items: center;
      gap: 0.32rem;
    }
    .metric-dot {
      width: 0.48rem;
      height: 0.48rem;
      border-radius: 50%;
      display: inline-block;
      flex-shrink: 0;
    }
    .featured-actions {
      display: flex;
      gap: 0.4rem;
      flex-wrap: wrap;
      margin-top: auto;
    }

    .tile-grid {
      display: grid;
      gap: 1rem;
    }
    .readiness-band {
      display: grid;
      gap: 0.65rem;
    }
    .group-grid {
      display: grid;
      grid-template-columns: repeat(3, minmax(0, 1fr));
      gap: 0.75rem;
    }
    .tile {
      cursor: pointer;
      transition: transform 0.16s ease, box-shadow 0.16s ease;
      padding: 0.85rem 0.9rem 0.95rem;
    }
    .tile:hover {
      transform: translateY(-1px);
      box-shadow: 0 1.1rem 2.1rem rgba(23, 29, 38, 0.08);
    }
    .tile-face {
      display: grid;
      gap: 0.32rem;
      padding: 0.25rem 0 0.4rem;
    }
    .seed-chip {
      display: inline-flex;
      width: fit-content;
      align-items: center;
      gap: 0.3rem;
      padding: 0.15rem 0.45rem;
      border-radius: 999px;
      background: hsl(var(--seed-hue) 75% 96%);
      color: hsl(var(--seed-hue) 48% 32%);
      font-size: 0.63rem;
      font-weight: 600;
      letter-spacing: 0.07em;
      text-transform: uppercase;
    }
    .tile-name {
      font-size: 0.84rem;
      font-weight: 500;
      color: #2a2d35;
      line-height: 1.4;
    }
    .tile-route {
      font-size: 0.68rem;
      color: hsl(var(--seed-hue) 58% 38%);
      font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .tile-foot {
      display: flex;
      align-items: center;
      gap: 0.35rem;
      flex-wrap: wrap;
      padding: 0 0 0.95rem;
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
      font-size: 0.64rem;
      color: #8d94a0;
      text-transform: uppercase;
      letter-spacing: 0.05em;
    }
    .runtime-state {
      display: inline-flex;
      align-items: center;
      gap: 0.28rem;
      padding: 0.16rem 0.45rem;
      border-radius: 999px;
      border: 1px solid #cbd1da;
      background: rgba(255, 255, 255, 0.9);
      font-size: 0.62rem;
      text-transform: uppercase;
      letter-spacing: 0.06em;
      line-height: 1;
    }
    .runtime-state.running {
      border-color: rgba(34, 160, 90, 0.32);
      background: rgba(34, 160, 90, 0.1);
      color: #178243;
    }
    .runtime-state.launching,
    .runtime-state.queued {
      border-color: rgba(37, 99, 235, 0.28);
      background: rgba(37, 99, 235, 0.08);
      color: #1d4ed8;
    }

    .tile-expanded {
      display: none;
      padding: 0.1rem 0 0;
    }
    .tile.is-expanded .tile-expanded {
      display: grid;
      gap: 0.55rem;
    }
    .tile-grid.has-expanded .tile:not(.is-expanded) {
      opacity: 0.72;
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
      margin-top: 0.1rem;
    }

    .status,
    .readiness {
      display: inline-flex;
      align-items: center;
      gap: 0.35rem;
      padding: 0.18rem 0.48rem;
      border-radius: 999px;
      border: 1px solid #cbd1da;
      background: rgba(255, 255, 255, 0.82);
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

    .action-row {
      display: flex;
      gap: 0.45rem;
      flex-wrap: wrap;
    }
    .action-button {
      padding: 0.22rem 0.65rem;
      border: 1px solid #c8ccd4;
      background: transparent;
      color: #616875;
      font: inherit;
      font-size: 0.66rem;
      letter-spacing: 0.06em;
      text-transform: uppercase;
      cursor: pointer;
      text-decoration: none;
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
    .agent-shell {
      padding: 1rem;
      border: 1px solid #d3d9e1;
      background: rgba(255, 255, 255, 0.84);
    }
    .agent-shell p {
      margin: 0.35rem 0 0;
      color: #616977;
      font-size: 0.8rem;
      line-height: 1.6;
    }
    .agent-grid {
      display: grid;
      grid-template-columns: 1.2fr 0.8fr;
      gap: 0.9rem;
      margin-top: 0.9rem;
    }
    .agent-box {
      border: 1px solid #d9dde4;
      background: #fbfbfc;
      padding: 0.9rem;
    }
    .agent-box h3 {
      margin: 0 0 0.4rem;
      font-size: 0.75rem;
      letter-spacing: 0.08em;
      text-transform: uppercase;
      color: #2a2d35;
    }
    .agent-box pre {
      margin: 0;
      overflow-x: auto;
      font-size: 0.72rem;
      line-height: 1.5;
      color: #26303d;
      white-space: pre-wrap;
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
    .launch-screen {
      min-height: 100vh;
      display: grid;
      place-items: center;
      padding: 2rem;
    }
    .launch-panel {
      width: min(32rem, calc(100vw - 3rem));
      display: grid;
      gap: 0.78rem;
      padding: 1.15rem 1.2rem;
      background: rgba(255, 255, 255, 0.96);
      border: 1px solid #d4d8df;
      box-shadow: 0 1.4rem 3rem rgba(28, 35, 48, 0.22);
      color: #222730;
    }
    .launch-heading {
      display: grid;
      gap: 0.18rem;
    }
    .launch-kicker {
      font-size: 0.66rem;
      letter-spacing: 0.1em;
      text-transform: uppercase;
      color: #7a818d;
    }
    .launch-name {
      margin: 0;
      font-size: 0.98rem;
      line-height: 1.35;
      letter-spacing: 0;
      color: #222730;
    }
    .launch-progress {
      height: 0.26rem;
      overflow: hidden;
      background: #e6ebf0;
    }
    .launch-progress-fill {
      display: block;
      height: 100%;
      width: 0;
      background: #22a05a;
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
      font-size: 0.85rem;
      font-weight: 600;
      line-height: 1.4;
      color: #222730;
    }
    .launch-copy {
      margin: 0;
      color: #69707c;
      font-size: 0.78rem;
      line-height: 1.6;
    }
    .launch-debug {
      margin: 0;
      padding: 0.62rem 0.72rem;
      border: 1px solid #d4d8df;
      background: rgba(244, 246, 248, 0.98);
      color: #59606b;
      font-family: ui-monospace, SFMono-Regular, SFMono, Menlo, Consolas, monospace;
      font-size: 0.72rem;
      line-height: 1.5;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .launch-sprout {
      width: 9rem;
      margin: 0 auto;
      cursor: default;
    }
    .launch-meta-row {
      display: flex;
      gap: 0.45rem;
      flex-wrap: wrap;
      font-size: 0.67rem;
      color: #6f7783;
    }
    .launch-meta-item {
      border: 1px solid #d5d9e1;
      border-radius: 999px;
      padding: 0.22rem 0.58rem;
      background: rgba(255, 255, 255, 0.82);
      text-transform: none;
      letter-spacing: 0;
    }
    .launch-timer {
      margin: 0;
      color: #59606b;
      font-size: 0.8rem;
      letter-spacing: 0.01em;
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
      .page { width: min(36rem, calc(100vw - 1rem)); }
      .featured-grid,
      .group-grid,
      .agent-grid { grid-template-columns: 1fr; }
      .section-head,
      .catalog-head {
        align-items: start;
        flex-direction: column;
      }
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

    <p class="intro">User- and agent-driven rapid software evolution, guided by purpose, design docs, and accepted decision history. Realizations can be rerun as technology improves without losing the contract beneath them.</p>
    <div class="hero-note">Use the sprout any time to understand a page, request improvements, or fork your own version on the same data or a new dataset. Security and scale stay centralized while each realization keeps evolving.</div>

    <section class="featured-shell">
      <div class="section-head">
        <div class="section-copy">
          <h2 class="section-title">Featured Systems</h2>
          <p class="catalog-text">Highlighted surfaces first. The registry footprint bar under each card is relative across the seeds shown on this page.</p>
        </div>
        <button class="action-button" id="registry-open" type="button">Registry</button>
      </div>
      <div class="featured-grid">
        {{range .Featured}}
        <article class="featured-card" style="--seed-hue: {{.AccentHue}};">
          <div class="tile-meta">
            {{if .Primary.RuntimeStateLabel}}<span class="runtime-state {{.Primary.RuntimeStateClass}}">{{.Primary.RuntimeStateLabel}}</span>{{end}}
            <div class="featured-kicker">{{.Primary.ReadinessLabel}}</div>
          </div>
          <div>
            <h3>{{.Label}}</h3>
            <p class="featured-summary">{{.Summary}}</p>
          </div>
          <div>
            <div class="footprint-label">Registry Footprint</div>
            <div class="footprint-track">
              <div class="footprint-fill" style="width: {{.Metrics.Width}};">
                {{range .Metrics.Segments}}{{if gt .Count 0}}<span class="footprint-segment {{.Key}}" style="width: {{.Width}};"></span>{{end}}{{end}}
              </div>
            </div>
            <div class="metric-row">
              {{range .Metrics.Segments}}{{if gt .Count 0}}<span class="metric"><span class="metric-dot {{.Key}}"></span>{{.Count}} {{.Label}}</span>{{end}}{{end}}
            </div>
          </div>
          <div class="featured-actions">
            <button class="action-button" type="button" data-action="inspect" data-reference="{{.Primary.Reference}}" data-label="{{.Label}}">Inspect</button>
            <button class="action-button is-primary" type="button" data-action="grow" data-reference="{{.Primary.Reference}}" data-label="{{.Label}}">Grow</button>
            {{if .Primary.CanRun}}<button class="action-button" type="button" data-action="run" data-reference="{{.Primary.Reference}}" data-label="{{.Label}}"{{if .Primary.CanLaunchLocal}} data-launchable="true"{{end}}{{if .Primary.ExecutionOpenPath}} data-open-path="{{.Primary.ExecutionOpenPath}}"{{end}}>{{if .Primary.ExecutionOpenPath}}Open{{else if .Primary.CanLaunchLocal}}Run{{else}}Show Run{{end}}</button>{{else}}<button class="action-button" type="button" disabled>Run</button>{{end}}
          </div>
        </article>
        {{end}}
      </div>
    </section>

    <section class="catalog-shell" id="catalog">
      <div class="catalog-head">
        <div class="catalog-copy">
          <h2 class="catalog-title">Explore By Readiness</h2>
          <p class="catalog-text">Smaller tiles keep the active surfaces visible while grouping realizations by what you can do with them right now.</p>
        </div>
      </div>
    <section class="tile-grid" id="tile-grid">
      {{range .ReadinessGroups}}
      <section class="readiness-band">
        <div class="catalog-copy">
          <h2 class="catalog-title">{{.Title}}</h2>
          <p class="catalog-text">{{.Summary}}</p>
        </div>
        <div class="group-grid">
          {{range .Realizations}}
          <article class="tile" style="--seed-hue: {{.SeedHue}};" data-tile data-tile-type="realization"
                   data-reference="{{.Reference}}" data-label="{{.Summary}}"
                   data-can-run="{{.CanRun}}" tabindex="0">
            <span class="seed-chip">{{.SeedDisplayName}}</span>
            <div class="tile-face">
              <span class="tile-name">{{.Summary}}</span>
              {{if .Subdomain}}<span class="tile-route">{{.Subdomain}}</span>
              {{else if .PathPrefix}}<span class="tile-route">{{.PathPrefix}}</span>{{end}}
            </div>
            <div class="tile-foot">
              <span class="tile-dot" data-status="{{.Status}}"></span>
              {{if .RuntimeStateLabel}}<span class="runtime-state {{.RuntimeStateClass}}">{{.RuntimeStateLabel}}</span>{{end}}
              <span class="tile-stage">{{.ReadinessLabel}}</span>
            </div>
            <div class="tile-expanded" aria-hidden="true">
              <p class="tile-summary">{{.ReadinessSummary}}</p>
              <div class="tile-meta">
                {{if .RuntimeStateLabel}}<span class="runtime-state {{.RuntimeStateClass}}">{{.RuntimeStateLabel}}</span>{{end}}
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
        </div>
      </section>
      {{end}}
    </section>
    </section>

    <section class="agent-shell">
      <div class="section-copy">
        <h2 class="section-title">For Agents</h2>
        <p>Do not depend on scraping the interface. Every AS surface is intended to be reachable through scoped API access, with the same purpose and ledger history available to humans and agents.</p>
      </div>
      <div class="agent-grid">
        <div class="agent-box">
          <h3>Discovery</h3>
          <pre>GET /v1/registry/catalog
GET /v1/registry/objects?seed_id=...
GET /v1/registry/commands?seed_id=...
GET /v1/registry/projections?seed_id=...
GET /v1/registry/realizations?seed_id=...</pre>
        </div>
        <div class="agent-box">
          <h3>Operating Model</h3>
          <pre>Purpose, design docs, and legacy decisions guide change.
Security and scale stay centralized in the kernel.
Evolution cost can be funded by users, paid directly, or executed by coding agents.
Immutable object and schema history keeps rollback and replay possible.</pre>
        </div>
      </div>
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
    <p class="footer">Draft realizations materialize into <code>materialized/</code> for inspection. Growth requests enqueue agent-ready jobs in <code>runtime_jobs</code>, and accepted history stays replayable through the registry layer.</p>

    <script src="/assets/sprout-logo.js"></script>
    <script src="/assets/launch-state.js"></script>
    <script nonce="{{.CSPNonce}}">{{.LoaderScript}}</script>
    <script nonce="{{.CSPNonce}}">{{.FeedbackScript}}</script>
  </main>
</body>
</html>`))

type bootPageView struct {
	Seeds             []seedBootView
	Featured          []featuredSeedView
	ReadinessGroups   []readinessGroupView
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
	AccentHue           int
	Metrics             seedMetricsView
	Count               int
	GrowthReadyCount    int
	RunnableCount       int
	InitiallyOpen       bool
	IsSingleRealization bool
	Realizations        []realizationBootView
}

type featuredSeedView struct {
	SeedID    string
	Label     string
	Summary   string
	AccentHue int
	Metrics   seedMetricsView
	Primary   realizationBootView
}

type readinessGroupView struct {
	Title        string
	Summary      string
	Realizations []realizationBootView
}

type seedMetricsView struct {
	Total    int
	Width    string
	Segments []metricSegmentView
}

type metricSegmentView struct {
	Key   string
	Label string
	Count int
	Width string
}

type realizationBootView struct {
	Reference         string
	SeedID            string
	SeedDisplayName   string
	SeedHue           int
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
	RuntimeStateLabel string
	RuntimeStateClass string
	IsRunning         bool
	Subdomain         string
	PathPrefix        string
}

type executionBootState struct {
	ExecutionID string
	Status      string
	OpenPath    string
}

func newBootPageView(options []materializer.RealizationOption, catalog registrycatalog.Catalog, executions map[string]executionBootState, executionEnabled, remoteConfigured, runtimeConfigured bool, nonce string, feedbackScript string) bootPageView {
	seen := make(map[string]int)
	seeds := make([]seedBootView, 0)
	allRealizations := make([]realizationBootView, 0)

	for _, option := range options {
		if !seedVisibleOnBootPage(option.SeedStatus) {
			continue
		}

		index, ok := seen[option.SeedID]
		if !ok {
			index = len(seeds)
			seen[option.SeedID] = index
			seeds = append(seeds, seedBootView{
				SeedID:        option.SeedID,
				DisplayName:   humanizeSeedID(option.SeedID),
				Summary:       strings.TrimSpace(option.SeedSummary),
				Status:        strings.TrimSpace(option.SeedStatus),
				AccentHue:     seedAccentHue(option.SeedID),
				InitiallyOpen: len(seeds) == 0,
			})
		}

		readinessStage := firstNonEmpty(strings.TrimSpace(option.Readiness.Stage), "designed")
		readinessLabel := firstNonEmpty(strings.TrimSpace(option.Readiness.Label), "Designed")
		readinessSummary := firstNonEmpty(strings.TrimSpace(option.Readiness.Summary), "This realization is ready for inspection and growth.")
		execution := executions[option.Reference]
		runtimeStateLabel, runtimeStateClass, isRunning := bootRuntimeState(execution)

		item := realizationBootView{
			Reference:         option.Reference,
			SeedID:            option.SeedID,
			SeedDisplayName:   humanizeSeedID(option.SeedID),
			SeedHue:           seeds[index].AccentHue,
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
			RuntimeStateLabel: runtimeStateLabel,
			RuntimeStateClass: runtimeStateClass,
			IsRunning:         isRunning,
			Subdomain:         option.Subdomain,
			PathPrefix:        option.PathPrefix,
		}
		seeds[index].Realizations = append(seeds[index].Realizations, item)
		allRealizations = append(allRealizations, item)
		seeds[index].Count = len(seeds[index].Realizations)
		if item.HasContract {
			seeds[index].GrowthReadyCount++
		}
		if item.CanRun {
			seeds[index].RunnableCount++
		}
	}

	metricsBySeed := buildSeedMetrics(catalog, seeds)
	for i := range seeds {
		seeds[i].IsSingleRealization = len(seeds[i].Realizations) == 1
		if metric, ok := metricsBySeed[seeds[i].SeedID]; ok {
			seeds[i].Metrics = metric
		}
	}

	return bootPageView{
		Seeds:             seeds,
		Featured:          buildFeaturedSeeds(seeds),
		ReadinessGroups:   buildReadinessGroups(allRealizations),
		ExecutionEnabled:  executionEnabled,
		RemoteConfigured:  remoteConfigured,
		RuntimeConfigured: runtimeConfigured,
		GitHubURL:         envOrDefault("AS_GITHUB_URL", "https://github.com/anthropics/autosoftware"),
		CSPNonce:          nonce,
		LoaderScript:      template.JS(consoleLoaderScript()),
		FeedbackScript:    template.JS(feedbackScript),
	}
}

func seedVisibleOnBootPage(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "archived", "deprecated", "retired":
		return false
	default:
		return true
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

func buildSeedMetrics(catalog registrycatalog.Catalog, seeds []seedBootView) map[string]seedMetricsView {
	type seedCounts struct {
		Realizations int
		Objects      int
		Commands     int
		Projections  int
	}

	visible := make(map[string]bool, len(seeds))
	counts := make(map[string]*seedCounts, len(seeds))
	for _, seed := range seeds {
		visible[seed.SeedID] = true
		counts[seed.SeedID] = &seedCounts{}
	}

	for _, item := range catalog.Realizations {
		if visible[item.SeedID] {
			counts[item.SeedID].Realizations++
		}
	}
	for _, item := range catalog.Objects {
		if visible[item.SeedID] {
			counts[item.SeedID].Objects++
		}
	}
	for _, item := range catalog.Commands {
		if visible[item.SeedID] {
			counts[item.SeedID].Commands++
		}
	}
	for _, item := range catalog.Projections {
		if visible[item.SeedID] {
			counts[item.SeedID].Projections++
		}
	}

	maxTotal := 0
	for _, seed := range seeds {
		total := counts[seed.SeedID].Realizations + counts[seed.SeedID].Objects + counts[seed.SeedID].Commands + counts[seed.SeedID].Projections
		if total > maxTotal {
			maxTotal = total
		}
	}

	out := make(map[string]seedMetricsView, len(seeds))
	for _, seed := range seeds {
		count := counts[seed.SeedID]
		total := count.Realizations + count.Objects + count.Commands + count.Projections
		out[seed.SeedID] = seedMetricsView{
			Total: total,
			Width: percentString(total, maxTotal),
			Segments: []metricSegmentView{
				{Key: "objects", Label: "objects", Count: count.Objects, Width: percentString(count.Objects, total)},
				{Key: "commands", Label: "actions", Count: count.Commands, Width: percentString(count.Commands, total)},
				{Key: "projections", Label: "read models", Count: count.Projections, Width: percentString(count.Projections, total)},
				{Key: "realizations", Label: "realizations", Count: count.Realizations, Width: percentString(count.Realizations, total)},
			},
		}
	}
	return out
}

func buildFeaturedSeeds(seeds []seedBootView) []featuredSeedView {
	plans := []struct {
		SeedID             string
		Label              string
		PreferredReference string
	}{
		{SeedID: "0006-registry-browser", Label: "Registry Browser", PreferredReference: "0006-registry-browser/a-ledger-reading-room"},
		{SeedID: "0004-event-listings", Label: "Event Listings", PreferredReference: "0004-event-listings/a-web-mvp"},
		{SeedID: "0003-customer-service-app", Label: "Ticketing", PreferredReference: "0003-customer-service-app/a-web-mvp"},
	}

	index := make(map[string]seedBootView, len(seeds))
	for _, seed := range seeds {
		index[seed.SeedID] = seed
	}

	featured := make([]featuredSeedView, 0, len(plans))
	for _, plan := range plans {
		seed, ok := index[plan.SeedID]
		if !ok || len(seed.Realizations) == 0 {
			continue
		}
		primary := preferredFeaturedRealization(seed.Realizations, plan.PreferredReference)
		featured = append(featured, featuredSeedView{
			SeedID:    seed.SeedID,
			Label:     plan.Label,
			Summary:   firstNonEmpty(seed.Summary, primary.ReadinessSummary),
			AccentHue: seed.AccentHue,
			Metrics:   seed.Metrics,
			Primary:   primary,
		})
	}
	return featured
}

func preferredFeaturedRealization(items []realizationBootView, preferredReference string) realizationBootView {
	for _, item := range items {
		if item.Reference == preferredReference {
			return item
		}
	}
	for _, item := range items {
		if item.CanRun {
			return item
		}
	}
	for _, item := range items {
		if item.HasContract {
			return item
		}
	}
	return items[0]
}

func buildReadinessGroups(items []realizationBootView) []readinessGroupView {
	running := make([]realizationBootView, 0)
	runnable := make([]realizationBootView, 0)
	ready := make([]realizationBootView, 0)
	designed := make([]realizationBootView, 0)

	for _, item := range items {
		switch {
		case item.IsRunning:
			running = append(running, item)
		case item.CanRun:
			runnable = append(runnable, item)
		case item.HasContract:
			ready = append(ready, item)
		default:
			designed = append(designed, item)
		}
	}

	groups := make([]readinessGroupView, 0, 3)
	if len(running) > 0 {
		groups = append(groups, readinessGroupView{
			Title:        "Running Now",
			Summary:      "Live routes already backed by an active execution.",
			Realizations: running,
		})
	}
	if len(runnable) > 0 {
		groups = append(groups, readinessGroupView{
			Title:        "Runnable But Idle",
			Summary:      "Launchable realizations that are not currently serving a live route.",
			Realizations: runnable,
		})
	}
	if len(ready) > 0 {
		groups = append(groups, readinessGroupView{
			Title:        "Ready To Grow",
			Summary:      "Contract-defined realizations that are prepared for the next agent or developer pass.",
			Realizations: ready,
		})
	}
	if len(designed) > 0 {
		groups = append(groups, readinessGroupView{
			Title:        "Designed Drafts",
			Summary:      "Earlier passes still shaping their contract, runtime, or delivery path.",
			Realizations: designed,
		})
	}
	return groups
}

func seedAccentHue(seedID string) int {
	hash := 0
	for _, r := range seedID {
		hash = (hash*33 + int(r)) % 360
	}
	return 18 + (hash % 300)
}

func percentString(value, total int) string {
	if value <= 0 || total <= 0 {
		return "0%"
	}
	return fmt.Sprintf("%.1f%%", (float64(value)*100)/float64(total))
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

func bootRuntimeState(execution executionBootState) (label, class string, running bool) {
	status := strings.ToLower(strings.TrimSpace(execution.Status))
	switch status {
	case "healthy":
		return "Running", "running", true
	case "starting":
		return "Launching", "launching", false
	case "launch_requested":
		return "Queued", "queued", false
	default:
		if strings.TrimSpace(execution.OpenPath) != "" {
			return "Running", "running", true
		}
		return "", "", false
	}
}

func consoleLoaderScript() string {
	return `(function () {
  var grid = document.getElementById("tile-grid");
  var backdrop = document.getElementById("modal-backdrop");
  var modalShell = document.getElementById("modal-shell");
  var modalContent = document.getElementById("modal-content");
  var modalCloseBtn = document.getElementById("modal-close");
  var sproutFab = document.getElementById("sprout-fab");
  var registryButtons = document.querySelectorAll("#registry-open");
  var status = document.getElementById("console-status");
  if (!grid || !backdrop || !modalContent) return;

  var expandedTile = null;
  var modalOpen = false;
  var launchView = null;
  var activeLaunchToken = 0;

  function escapeHTML(v) {
    return String(v).replace(/[&<>"]/g, function (c) {
      return {"&":"&amp;","<":"&lt;",">":"&gt;","\"":"&quot;"}[c];
    });
  }

  function sleep(ms) {
    return new Promise(function (resolve) { setTimeout(resolve, ms); });
  }

  function setStatus(copy) {
    if (status) status.textContent = copy || "";
  }

  async function readJSONResponse(response) {
    var body = await response.text();
    if (!body) return {};
    try {
      return JSON.parse(body);
    } catch (err) {
      if (!response.ok) return { error: body.trim() || ("Request failed: " + response.status) };
      throw err;
    }
  }

  function launchStateAPI() {
    if (typeof window.ASLaunchState === "object" && window.ASLaunchState) {
      return window.ASLaunchState;
    }
    return null;
  }

  function isTerminalExecutionStatus(statusValue) {
    var api = launchStateAPI();
    if (api && typeof api.isTerminalExecutionStatus === "function") {
      return api.isTerminalExecutionStatus(statusValue);
    }
    return statusValue === "failed" || statusValue === "stopped" || statusValue === "terminated";
  }

  function latestExecutionEvent(events) {
    return Array.isArray(events) && events.length ? events[0] : null;
  }

  function hasExecutionEvent(events, name) {
    if (!Array.isArray(events)) return false;
    for (var i = 0; i < events.length; i++) {
      if (events[i] && events[i].name === name) return true;
    }
    return false;
  }

  function formatElapsed(ms) {
    var totalMs = Math.max(0, ms || 0);
    if (totalMs < 60000) {
      return (totalMs / 1000).toFixed(1) + "s";
    }
    var totalSeconds = Math.round(totalMs / 1000);
    var minutes = Math.floor(totalSeconds / 60);
    var seconds = totalSeconds % 60;
    return minutes + "m" + seconds + "s";
  }

  function launchMetadataKey(reference) {
    return "as-launch-metadata:" + String(reference || "unknown");
  }

  function launchMetadataLoad(reference) {
    var key = launchMetadataKey(reference);
    if (typeof localStorage === "undefined") return {};
    try {
      var raw = localStorage.getItem(key);
      if (!raw) return {};
      var parsed = JSON.parse(raw);
      if (!parsed || typeof parsed !== "object") return {};
      return parsed;
    } catch (err) {
      return {};
    }
  }

  function launchMetadataPersist(reference, openPath) {
    var key = launchMetadataKey(reference);
    if (typeof localStorage === "undefined") return;
    try {
      localStorage.setItem(key, JSON.stringify({
        lastLoadedAt: Date.now(),
        openPath: openPath || ""
      }));
    } catch (err) {}
  }

  function launchMetadataText(reference) {
    var metadata = launchMetadataLoad(reference);
    if (!metadata || !metadata.lastLoadedAt) {
      return "Last time this was loaded: never";
    }
    return "Last time this was loaded: " + new Date(Number(metadata.lastLoadedAt)).toLocaleString();
  }

  function launchInitSprout(root) {
    if (typeof window.ASSproutLogo === "object" && window.ASSproutLogo && typeof window.ASSproutLogo.init === "function") {
      window.ASSproutLogo.init(root);
    }
  }

  function launchStepLabel(session, events) {
    var api = launchStateAPI();
    if (api && typeof api.stepLabel === "function") {
      return api.stepLabel(session, events);
    }
    var statusValue = session && session.status ? session.status : "";
    if (statusValue === "launch_requested") return "Queued in kernel runtime";
    if (statusValue === "healthy") return session && session.open_path ? "Route ready" : "Registering route";
    if (statusValue === "failed") return "Launch failed";
    if (statusValue === "stopped") return "Launch stopped";
    if (statusValue === "terminated") return "Process terminated";
    if (hasExecutionEvent(events, "route_registered")) return "Route registered";
    if (hasExecutionEvent(events, "health_check_started")) return "Waiting for health check";
    if (hasExecutionEvent(events, "build_started")) return "Building launch artifact";
    if (hasExecutionEvent(events, "process_started")) return "Process started";
    if (hasExecutionEvent(events, "launch_spec_resolved")) return "Runtime manifest resolved";
    if (hasExecutionEvent(events, "launch_started")) return "Worker claimed launch";
    return "Starting execution";
  }

  function launchCopyText(session, events, label, elapsedMs, transientError) {
    var api = launchStateAPI();
    if (api && typeof api.copyText === "function") {
      return api.copyText(session, events, label, elapsedMs, transientError);
    }
    if (transientError) {
      return "Status polling was interrupted. Retrying against the runtime projection without abandoning the launch.";
    }
    var statusValue = session && session.status ? session.status : "";
    if (statusValue === "launch_requested") {
      if (elapsedMs > 15000) {
        return "Still waiting for the execution worker to claim the launch job and begin process startup.";
      }
      return "Writing the launch job into the runtime queue and waiting for the worker to claim it.";
    }
    if (statusValue === "healthy") {
      if (session && session.open_path) {
        return "The realization is healthy and routed. Opening " + session.open_path + " as soon as the launch trace settles.";
      }
      return "The realization is healthy. Waiting for the canonical route to finish registering.";
    }
    if (isTerminalExecutionStatus(statusValue)) {
      return (session && session.last_error) || ((label || "This realization") + " did not become runnable.");
    }
    if (hasExecutionEvent(events, "health_check_started")) {
      return "The process is running. Waiting for it to answer health checks on /healthz or its root route.";
    }
    if (hasExecutionEvent(events, "build_started")) {
      if (elapsedMs > 30000) {
        return "The worker is still building the launch artifact for this realization. Cold builds can take longer the first time or after source changes.";
      }
      return "The execution worker is building a local launch artifact before the process can start.";
    }
    if (hasExecutionEvent(events, "process_started")) {
      return "The process has started. Beginning health checks and route registration.";
    }
    if (hasExecutionEvent(events, "launch_spec_resolved")) {
      return "Runtime manifest resolved. Starting the process with kernel-provided capability URLs.";
    }
    if (hasExecutionEvent(events, "launch_started")) {
      return "The execution worker claimed the launch and is preparing the runtime.";
    }
    return "Preparing the runtime process, route bindings, and health checks.";
  }

  function launchMinimumProgressCap(launchContext, elapsedMs) {
    var minimumDisplayMs = Math.max(0, Number(launchContext.minimumDisplayMs) || 0);
    if (minimumDisplayMs <= 0 || elapsedMs >= minimumDisplayMs) {
      return 100;
    }
    return 8 + ((96 - 8) * (elapsedMs / minimumDisplayMs));
  }

  function minimumLaunchDisplayRemaining(launchContext) {
    var minimumDisplayMs = Math.max(0, Number(launchContext.minimumDisplayMs) || 0);
    return Math.max(0, minimumDisplayMs - (Date.now() - launchContext.requestedAt));
  }

  function launchProgressPercent(session, events, elapsedMs) {
    var api = launchStateAPI();
    if (api && typeof api.progressPercent === "function") {
      return api.progressPercent(session, events, elapsedMs);
    }
    var statusValue = session && session.status ? session.status : "";
    if (statusValue === "healthy") {
      return session && session.open_path ? 100 : 94;
    }
    if (isTerminalExecutionStatus(statusValue)) {
      return 100;
    }

    var progress = 8;
    if (statusValue === "launch_requested" || hasExecutionEvent(events, "launch_requested")) progress = 12;
    if (hasExecutionEvent(events, "launch_started")) progress = 24;
    if (hasExecutionEvent(events, "build_started")) progress = 34;
    if (hasExecutionEvent(events, "launch_spec_resolved")) progress = 42;
    if (hasExecutionEvent(events, "process_started") || (session && session.upstream_addr)) progress = 62;
    if (hasExecutionEvent(events, "health_check_started")) progress = 78 + Math.min(14, Math.floor(elapsedMs / 1500));
    if (hasExecutionEvent(events, "health_passed")) progress = 90;
    if (hasExecutionEvent(events, "route_registered") || (session && session.open_path)) progress = 96;
    return Math.max(8, Math.min(progress, 97));
  }

  function launchDisplayedProgress(session, events, launchContext, elapsedMs, allowCompletion) {
    var targetProgress = launchProgressPercent(session, events, elapsedMs);
    if (allowCompletion) {
      return 100;
    }
    if (isTerminalExecutionStatus(session && session.status ? session.status : "")) {
      return 100;
    }
    return Math.max(8, Math.min(targetProgress, Math.round(launchMinimumProgressCap(launchContext, elapsedMs))));
  }

  function launchDebugLine(session, events, launchContext, elapsedMs, transientError) {
    var api = launchStateAPI();
    if (api && typeof api.debugLine === "function") {
      return api.debugLine(session, events, launchContext, elapsedMs, transientError);
    }
    var latestEvent = latestExecutionEvent(events);
    var parts = [];
    parts.push("step=" + launchStepLabel(session, events));
    parts.push("status=" + ((session && session.status) || "unknown"));
    if (launchContext.jobID) parts.push("job=" + launchContext.jobID);
    if (session && session.execution_id) parts.push("exec=" + session.execution_id);
    if (latestEvent && latestEvent.name) parts.push("event=" + latestEvent.name);
    if (session && session.upstream_addr) parts.push("upstream=" + session.upstream_addr);
    if (session && session.route_path_prefix) parts.push("route=" + session.route_path_prefix);
    if (session && session.open_path) parts.push("open=" + session.open_path);
    parts.push("elapsed=" + formatElapsed(elapsedMs));
    if (transientError) parts.push("transport=" + transientError);
    return parts.join(" | ");
  }

  function resetLaunchView() {
    launchView = null;
  }

  function ensureLaunchView(label, reference) {
    if (launchView && modalContent.contains(launchView.root)) {
      return launchView;
    }
    modalContent.innerHTML = [
      '<div class="launch-screen">',
      '  <div class="launch-panel">',
      '    <div class="sprout-logo-shell launch-sprout" data-sprout-logo aria-hidden="true"></div>',
      '    <div class="launch-heading">',
      '      <div class="launch-kicker">Launching</div>',
      '      <h2 class="launch-name"></h2>',
      '    </div>',
      '    <div class="launch-progress" role="progressbar" aria-label="Launch progress" aria-valuemin="0" aria-valuemax="100" aria-valuenow="0">',
      '      <span class="launch-progress-fill"></span>',
      '    </div>',
      '    <p class="launch-meta-row">',
      '      <span class="launch-meta-item" data-launch-last-loaded></span>',
      '      <span class="launch-meta-item" data-launch-route></span>',
      '    </p>',
      '    <p class="launch-timer"></p>',
      '    <p class="launch-step"></p>',
      '    <p class="launch-copy"></p>',
      '    <p class="launch-debug"></p>',
      '  </div>',
      '</div>'
    ].join("");
    var root = modalContent.querySelector(".launch-screen");
    launchView = {
      root: root,
      sprout: root.querySelector("[data-sprout-logo]"),
      title: root.querySelector(".launch-name"),
      bar: root.querySelector(".launch-progress"),
      fill: root.querySelector(".launch-progress-fill"),
      lastLoaded: root.querySelector("[data-launch-last-loaded]"),
      routeLine: root.querySelector("[data-launch-route]"),
      timer: root.querySelector(".launch-timer"),
      step: root.querySelector(".launch-step"),
      copy: root.querySelector(".launch-copy"),
      debug: root.querySelector(".launch-debug")
    };
    launchInitSprout(root);
    launchView.title.textContent = label || "Launching realization";
    if (launchView.lastLoaded) {
      launchView.lastLoaded.textContent = launchMetadataText(reference || label || "unknown");
    }
    if (launchView.routeLine) {
      launchView.routeLine.textContent = "";
    }
    return launchView;
  }

  function renderLaunchState(snapshot, launchContext, transientError, allowCompletion) {
    var session = snapshot && snapshot.session ? snapshot.session : {};
    var events = snapshot && Array.isArray(snapshot.events) ? snapshot.events : [];
    var elapsedMs = Date.now() - launchContext.requestedAt;
    var view = ensureLaunchView(launchContext.label, launchContext.reference);
    var stepLabel = launchStepLabel(session, events);
    var progress = launchDisplayedProgress(session, events, launchContext, elapsedMs, !!allowCompletion);

    view.title.textContent = launchContext.label;
    view.step.textContent = stepLabel;
    if (view.timer) {
      view.timer.textContent = "Elapsed " + formatElapsed(elapsedMs);
    }
    if (view.routeLine) {
      var openPath = "";
      if (session && session.open_path) {
        openPath = session.open_path;
      } else if (launchContext.openPath) {
        openPath = launchContext.openPath;
      }
      view.routeLine.textContent = openPath ? ("Route: " + openPath) : "";
    }
    if (view.lastLoaded) {
      view.lastLoaded.textContent = launchMetadataText(launchContext.reference || launchContext.label || "");
    }
    view.copy.textContent = launchCopyText(session, events, launchContext.label, elapsedMs, transientError);
    view.debug.textContent = launchDebugLine(session, events, launchContext, elapsedMs, transientError);
    view.bar.setAttribute("aria-valuenow", String(progress));
    view.bar.classList.toggle("is-failed", isTerminalExecutionStatus(session.status));
    view.bar.classList.toggle("is-ready", session.status === "healthy" && !!session.open_path);
    view.fill.style.width = progress + "%";
    if (view.sprout && window.ASSproutLogo && typeof window.ASSproutLogo.setProgress === "function") {
      window.ASSproutLogo.setProgress(view.sprout, progress / 100);
    }

    if (session.status === "healthy" && session.open_path && allowCompletion) {
      setStatus("Opening " + launchContext.label + "...");
      return;
    }
    if (isTerminalExecutionStatus(session.status)) {
      setStatus("Launch failed for " + launchContext.label + ".");
      return;
    }
    setStatus("Launching " + launchContext.label + " [" + stepLabel + "]...");
  }

  async function waitForMinimumLaunchDisplay(launchContext, snapshot) {
    while (launchContext.token === activeLaunchToken) {
      var remainingMs = minimumLaunchDisplayRemaining(launchContext);
      if (remainingMs <= 0) {
        return;
      }
      renderLaunchState(snapshot, launchContext, "", false);
      await sleep(Math.min(80, remainingMs));
    }
  }

  async function waitForExecution(projectionPath, launchContext) {
    var pollDelay = Math.max(150, Number(launchContext.pollAfterMs) || 350);
    var consecutivePollErrors = 0;
    var snapshot = { session: { status: "launch_requested" }, events: [] };

    while (launchContext.token === activeLaunchToken) {
      try {
        var response = await fetch(projectionPath, {
          method: "GET",
          credentials: "same-origin",
          cache: "no-store",
          headers: { "Accept": "application/json" }
        });
        var result = await readJSONResponse(response);
        if (!response.ok) {
          var projectionError = new Error(result.error || ("Execution poll failed: " + response.status));
          projectionError.terminal = response.status >= 400 && response.status < 500;
          throw projectionError;
        }

        snapshot = {
          session: result.session || {},
          events: Array.isArray(result.events) ? result.events : []
        };
        if (snapshot.session && snapshot.session.open_path) {
          launchContext.openPath = snapshot.session.open_path;
          launchMetadataPersist(launchContext.reference, snapshot.session.open_path);
        }
        consecutivePollErrors = 0;
        renderLaunchState(snapshot, launchContext, "");

        var session = snapshot.session || {};
        if (session.status === "healthy" && session.open_path) {
          launchContext.openPath = session.open_path;
          launchMetadataPersist(launchContext.reference, session.open_path);
          await waitForMinimumLaunchDisplay(launchContext, snapshot);
          if (launchContext.token !== activeLaunchToken) {
            return null;
          }
          renderLaunchState(snapshot, launchContext, "", true);
          window.location.assign(session.open_path);
          return session;
        }
        if (isTerminalExecutionStatus(session.status)) {
          var terminalError = new Error(session.last_error || ("Execution " + session.status));
          terminalError.terminal = true;
          throw terminalError;
        }

        await sleep(pollDelay);
      } catch (err) {
        if (launchContext.token !== activeLaunchToken) {
          return null;
        }
        renderLaunchState(snapshot, launchContext, err && err.message ? err.message : String(err));
        if (err && err.terminal) {
          throw err;
        }
        consecutivePollErrors++;
        if (consecutivePollErrors >= 20) {
          throw err;
        }
        await sleep(Math.min(2000, 250 * consecutivePollErrors));
      }
    }

    return null;
  }

  async function launchRealization(reference, label) {
    var launchContext = {
      token: ++activeLaunchToken,
      label: label || reference,
      reference: reference,
      openPath: "",
      requestedAt: Date.now(),
      jobID: "",
      pollAfterMs: 350,
      minimumDisplayMs: 3000
    };

    resetLaunchView();
    ensureModal(true);
    renderLaunchState({ session: { status: "launch_requested", reference: reference }, events: [] }, launchContext, "");

    var response = await fetch("/boot/commands/realizations.launch", {
      method: "POST",
      credentials: "same-origin",
      headers: { "Accept": "application/json", "Content-Type": "application/json" },
      body: JSON.stringify({ reference: reference })
    });
    var result = await readJSONResponse(response);
    if (!response.ok) {
      throw new Error(result.error || ("Launch failed: " + response.status));
    }
    if (launchContext.token !== activeLaunchToken) {
      return null;
    }

    launchContext.jobID = result.job && result.job.job_id ? result.job.job_id : "";
    if (result.poll_after_ms) {
      launchContext.pollAfterMs = result.poll_after_ms;
    }
    if (result.open_path) {
      launchContext.openPath = result.open_path;
      launchMetadataPersist(reference, result.open_path);
    }
    renderLaunchState({ session: result.execution || { status: "launch_requested" }, events: [] }, launchContext, "");
    return waitForExecution(result.projection, launchContext);
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
    activeLaunchToken++;
    resetLaunchView();
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
    activeLaunchToken++;
    resetLaunchView();
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
    resetLaunchView();
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
    resetLaunchView();
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
    activeLaunchToken++;
    resetLaunchView();
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
          window.location.assign(openPath);
          setStatus("Opening " + (actionBtn.getAttribute("data-label") || actionBtn.getAttribute("data-reference")) + "...");
          return;
        }
        if (actionBtn.getAttribute("data-launchable") === "true") {
          launchRealization(
            actionBtn.getAttribute("data-reference"),
            actionBtn.getAttribute("data-label") || actionBtn.getAttribute("data-reference")
          ).catch(function (err) {
            if (!modalOpen) return;
            resetLaunchView();
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
  registryButtons.forEach(function (button) {
    button.addEventListener("click", function (event) {
      event.stopPropagation();
      openModal("registry", "", "Registry");
    });
  });

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
