package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	pathBoot      = "/__kernel/boot/status"
	pathIncidents = "/__kernel/feedback-loop/incidents"
	pathPlant     = "/__sprout/plant"
	pathAssets    = "/__sprout-assets"
	maxBodyBytes  = 64 << 10
)

//go:embed assets/sprout-logo/*
var sproutLogoAssets embed.FS

// ---------------------------------------------------------------------------
// Seed catalog (mock)
// ---------------------------------------------------------------------------

type seedEntry struct {
	ID           string
	Name         string
	Brief        string
	Popularity   int
	Realizations []realizationEntry
}

type realizationEntry struct {
	ID         string
	Label      string
	Status     string
	Popularity int
}

type bootData struct {
	Seeds []seedEntry
}

var mockData = bootData{
	Seeds: []seedEntry{
		{
			ID:         "0003-customer-service",
			Name:       "Customer Service App",
			Brief:      "Handle support tickets, live chat, and knowledge base",
			Popularity: 142,
			Realizations: []realizationEntry{
				{ID: "a-lightweight-queue", Label: "Lightweight ticket queue", Status: "draft", Popularity: 98},
				{ID: "b-full-helpdesk", Label: "Full helpdesk with live chat", Status: "proposed", Popularity: 44},
			},
		},
		{
			ID:         "0004-event-listings",
			Name:       "Event Listings",
			Brief:      "Create and manage public event calendars",
			Popularity: 89,
			Realizations: []realizationEntry{
				{ID: "a-simple-calendar", Label: "Simple calendar with RSVP", Status: "draft", Popularity: 89},
			},
		},
		{
			ID:         "0005-charity-auction",
			Name:       "Charity Auction Manager",
			Brief:      "Run online auctions for charitable causes",
			Popularity: 57,
			Realizations: []realizationEntry{
				{ID: "a-timed-bidding", Label: "Timed bidding with live updates", Status: "proposed", Popularity: 31},
				{ID: "b-silent-auction", Label: "Silent auction with sealed bids", Status: "proposed", Popularity: 19},
				{ID: "c-hybrid-gala", Label: "Hybrid gala event runner", Status: "draft", Popularity: 7},
			},
		},
	},
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	addr := envOrDefault("AS_ADDR", "127.0.0.1:8092")
	seedsDir := os.Getenv("AS_SEEDS_DIR")

	status := newBootStatus()
	var plantMu sync.Mutex

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", handleBoot)
	mux.Handle("GET "+pathAssets+"/", sproutAssetHandler())
	mux.HandleFunc("GET "+pathBoot, status.sseHandler)
	mux.HandleFunc("POST "+pathBoot, status.updateHandler)
	mux.HandleFunc("POST "+pathIncidents, ingestHandler("incident"))
	mux.HandleFunc("POST "+pathPlant, plantHandler(seedsDir, &plantMu))

	fmt.Println()
	fmt.Println("  \033[32m●\033[0m AS — autosoftware")
	fmt.Println()
	fmt.Printf("    surface   http://%s\n", addr)
	if seedsDir != "" {
		fmt.Printf("    seeds     %s\n", seedsDir)
	} else {
		fmt.Println("    seeds     (capture-only — set AS_SEEDS_DIR to persist)")
	}
	fmt.Printf("    feedback  POST %s\n", pathIncidents)
	fmt.Printf("    sprout    POST %s\n", pathPlant)
	fmt.Printf("    status    GET  %s\n", pathBoot)
	fmt.Println()
	fmt.Println("    ready.")
	fmt.Println()

	if strings.HasPrefix(addr, "/") || strings.HasPrefix(addr, ".") {
		if err := os.MkdirAll(filepath.Dir(addr), 0755); err != nil {
			log.Fatal(err)
		}
		os.Remove(addr)
		ln, err := net.Listen("unix", addr)
		if err != nil {
			log.Fatal(err)
		}
		defer ln.Close()
		defer os.Remove(addr)
		go func() {
			sig := make(chan os.Signal, 1)
			signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
			<-sig
			ln.Close()
			os.Remove(addr)
			os.Exit(0)
		}()
		fmt.Printf("    socket    %s\n", addr)
		fmt.Println()
		fmt.Println("    ready.")
		fmt.Println()
		if err := http.Serve(ln, requestLog(mux)); err != nil && !errors.Is(err, net.ErrClosed) {
			log.Fatal(err)
		}
		return
	}

	if err := http.ListenAndServe(addr, requestLog(mux)); err != nil {
		log.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// Boot page
// ---------------------------------------------------------------------------

func handleBoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := bootTemplate.Execute(w, mockData); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func sproutAssetHandler() http.Handler {
	sub, err := fs.Sub(sproutLogoAssets, "assets/sprout-logo")
	if err != nil {
		panic(err)
	}
	return http.StripPrefix(pathAssets+"/", http.FileServer(http.FS(sub)))
}

// ---------------------------------------------------------------------------
// Boot status — SSE stream for materialization progress
// ---------------------------------------------------------------------------

type bootStatus struct {
	mu      sync.Mutex
	clients map[chan string]struct{}
	current string
}

func newBootStatus() *bootStatus {
	return &bootStatus{clients: make(map[chan string]struct{})}
}

func (bs *bootStatus) broadcast(msg string) {
	bs.mu.Lock()
	bs.current = msg
	for ch := range bs.clients {
		select {
		case ch <- msg:
		default:
		}
	}
	bs.mu.Unlock()
}

func (bs *bootStatus) subscribe() chan string {
	ch := make(chan string, 4)
	bs.mu.Lock()
	bs.clients[ch] = struct{}{}
	if bs.current != "" {
		ch <- bs.current
	}
	bs.mu.Unlock()
	return ch
}

func (bs *bootStatus) unsubscribe(ch chan string) {
	bs.mu.Lock()
	delete(bs.clients, ch)
	bs.mu.Unlock()
}

func (bs *bootStatus) sseHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := bs.subscribe()
	defer bs.unsubscribe(ch)

	for {
		select {
		case msg := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (bs *bootStatus) updateHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	r.Body.Close()
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		http.Error(w, "empty status", http.StatusBadRequest)
		return
	}
	bs.broadcast(msg)
	log.Printf("[boot] %s", msg)
	w.WriteHeader(http.StatusAccepted)
}

// ---------------------------------------------------------------------------
// Feedback-loop incident ingest (dev scaffold)
// ---------------------------------------------------------------------------

func ingestHandler(label string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
		r.Body.Close()
		if err != nil || len(body) == 0 {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		log.Printf("[%s] %s", label, body)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, `{"status":"accepted"}`)
	}
}

// ---------------------------------------------------------------------------
// Sprout — plant a new seed from the running interface
// ---------------------------------------------------------------------------

type plantRequest struct {
	Summary string `json:"summary"`
	Detail  string `json:"detail"`
}

func plantHandler(seedsDir string, mu *sync.Mutex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req plantRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, maxBodyBytes)).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		r.Body.Close()

		req.Summary = strings.TrimSpace(req.Summary)
		if req.Summary == "" {
			http.Error(w, "summary required", http.StatusBadRequest)
			return
		}

		if seedsDir == "" {
			log.Printf("[sprout] captured (no AS_SEEDS_DIR): %s", req.Summary)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]string{
				"status": "captured",
				"note":   "AS_SEEDS_DIR not set — seed not written to disk",
			})
			return
		}

		mu.Lock()
		nextNum, err := nextSeedNumber(seedsDir)
		if err != nil {
			mu.Unlock()
			log.Printf("[sprout] error scanning seeds: %v", err)
			http.Error(w, "failed to scan seeds directory", http.StatusInternalServerError)
			return
		}

		slug := slugify(req.Summary)
		if slug == "" {
			slug = "seed"
		}
		seedID := fmt.Sprintf("%04d-%s", nextNum, slug)
		seedDir := filepath.Join(seedsDir, seedID)

		err = writeSeed(seedDir, seedID, req)
		mu.Unlock()
		if err != nil {
			log.Printf("[sprout] error writing seed: %v", err)
			http.Error(w, "failed to create seed", http.StatusInternalServerError)
			return
		}

		log.Printf("[sprout] planted %s", seedID)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "planted",
			"seed_id": seedID,
		})
	}
}

func nextSeedNumber(seedsDir string) (int, error) {
	entries, err := os.ReadDir(seedsDir)
	if err != nil {
		return 0, err
	}
	max := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if len(name) >= 4 {
			if n, err := strconv.Atoi(name[:4]); err == nil && n < 9000 && n > max {
				max = n
			}
		}
	}
	return max + 1, nil
}

func writeSeed(dir, seedID string, req plantRequest) error {
	for _, sub := range []string{"", "approaches", "realizations"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0755); err != nil {
			return err
		}
	}

	brief := req.Summary
	if req.Detail != "" {
		brief += "\n\n" + req.Detail
	}

	files := map[string]string{
		"seed.yaml":              fmt.Sprintf("seed_id: %s\nversion: 1\nsummary: %s\nstatus: proposed\n", seedID, yamlQuote(req.Summary)),
		"brief.md":               fmt.Sprintf("# Brief\n\n%s\n", brief),
		"design.md":              "# Design\n",
		"acceptance.md":          "# Acceptance\n",
		"decision_log.md":        "# Decisions\n",
		"approaches/README.md":   "# Approaches\n",
		"realizations/README.md": "# Realizations\n",
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			return err
		}
	}
	return nil
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var buf strings.Builder
	prevHyphen := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			buf.WriteRune(r)
			prevHyphen = false
		} else if !prevHyphen {
			buf.WriteByte('-')
			prevHyphen = true
		}
	}
	result := strings.Trim(buf.String(), "-")
	if len(result) > 40 {
		result = result[:40]
		result = strings.TrimRight(result, "-")
	}
	return result
}

// yamlQuote wraps a value in double quotes with proper escaping for YAML
// double-quoted scalar syntax.
var yamlReplacer = strings.NewReplacer(
	`\`, `\\`,
	`"`, `\"`,
	"\n", `\n`,
	"\r", `\r`,
	"\t", `\t`,
	"\x00", `\0`,
)

func yamlQuote(s string) string {
	return `"` + yamlReplacer.Replace(s) + `"`
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

// ---------------------------------------------------------------------------
// Request logging middleware
// ---------------------------------------------------------------------------

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, code: 200}
		next.ServeHTTP(sw, r)
		dur := time.Since(start)

		tag := "req"
		extra := ""
		if r.Header.Get("HX-Request") == "true" {
			tag = "htmx"
			var parts []string
			if v := r.Header.Get("HX-Trigger"); v != "" {
				parts = append(parts, "trigger="+v)
			}
			if v := r.Header.Get("HX-Trigger-Name"); v != "" {
				parts = append(parts, "name="+v)
			}
			if v := r.Header.Get("HX-Target"); v != "" {
				parts = append(parts, "target="+v)
			}
			if r.Header.Get("HX-Boosted") == "true" {
				parts = append(parts, "boosted")
			}
			if len(parts) > 0 {
				extra = "  " + strings.Join(parts, " ")
			}
		}

		statusColor := "\033[32m" // green
		if sw.code >= 400 {
			statusColor = "\033[31m" // red
		} else if sw.code >= 300 {
			statusColor = "\033[33m" // yellow
		}

		fmt.Printf("    [%s] %-4s %s  %s%d\033[0m  %s%s\n",
			tag, r.Method, r.URL.Path, statusColor, sw.code, dur.Round(time.Microsecond), extra)
	})
}

// ---------------------------------------------------------------------------
// Boot template
// ---------------------------------------------------------------------------

var bootTemplate = template.Must(template.New("boot").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>AS</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }

body {
  min-height: 100vh;
  background: #eef0f3;
  display: flex;
  align-items: center;
  justify-content: center;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
  color: #2a2d35;
}

.page {
  width: min(26rem, calc(100vw - 2rem));
  padding: 18px 0 40px;
  transform: translateY(-26px);
}

.brand {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  position: relative;
}

.kernel-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #22c55e;
  animation: breathe 4s ease-in-out infinite;
}

@keyframes breathe {
  0%, 100% {
    box-shadow: 0 0 4px rgba(34,197,94,0.25), 0 0 14px rgba(34,197,94,0.06);
    opacity: 0.5;
  }
  50% {
    box-shadow: 0 0 8px rgba(34,197,94,0.5), 0 0 28px rgba(34,197,94,0.12);
    opacity: 1;
  }
}

.wordmark {
  font-size: 32px;
  font-weight: 700;
  letter-spacing: 9px;
  color: #1a1d24;
  margin-top: 12px;
  padding-left: 9px;
  position: relative;
  z-index: 2;
}

.tagline {
  font-size: 12px;
  color: #8a8e99;
  letter-spacing: 3.8px;
  margin-top: 6px;
  padding-left: 4px;
  position: relative;
  z-index: 2;
}

.desc {
  font-size: 12px;
  color: #6b7080;
  line-height: 1.6;
  margin-top: 16px;
  max-width: 240px;
  position: relative;
  z-index: 2;
}

.divider {
  width: 32px;
  height: 1px;
  background: #d0d4dc;
  margin: 28px auto;
}

.seed {
  border-bottom: 1px solid #d0d4dc;
}

.seed:first-child {
  border-top: 1px solid #d0d4dc;
}

.seed-head {
  display: flex;
  align-items: center;
  padding: 14px 0;
  cursor: pointer;
  gap: 14px;
}

.seed-pop {
  font-size: 11px;
  color: #8a8e99;
  min-width: 28px;
  text-align: right;
  flex-shrink: 0;
}

.seed-mid {
  flex: 1;
  min-width: 0;
}

.seed-head:hover .seed-name {
  color: #000;
}

.seed-name {
  font-size: 13px;
  color: #2a2d35;
  transition: color .15s;
}

.seed-brief {
  font-size: 11px;
  color: #8a8e99;
  margin-top: 3px;
}

.arrow {
  color: #b0b4bd;
  font-size: 16px;
  transition: transform .15s;
  flex-shrink: 0;
  margin-left: 12px;
}

.seed.open .arrow {
  transform: rotate(90deg);
}

.seed-body {
  display: none;
  padding: 0 0 14px 0;
}

.seed.open .seed-body {
  display: block;
}

.real {
  display: flex;
  align-items: center;
  padding: 5px 0;
  gap: 14px;
}

.real-pop {
  font-size: 10px;
  color: #b0b4bd;
  min-width: 28px;
  text-align: right;
  flex-shrink: 0;
}

.real-mid {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
}

.real-label {
  font-size: 12px;
  color: #5a5e6a;
}

.real-status {
  font-size: 9px;
  letter-spacing: 0.5px;
  text-transform: uppercase;
}

.real-status.draft { color: #a16207; }
.real-status.proposed { color: #2563eb; }
.real-status.accepted { color: #16a34a; }

.real-boot {
  font: inherit;
  font-size: 10px;
  background: none;
  border: 1px solid #c0c4cc;
  color: #6b7080;
  padding: 2px 10px;
  cursor: pointer;
}

.real-boot:hover {
  color: #16a34a;
  border-color: #16a34a;
}

.action-row {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  font-size: 11px;
  color: #8a8e99;
  padding: 10px 0 2px;
}

.act-btn {
  font: inherit;
  font-size: 10px;
  background: none;
  border: 1px solid #c0c4cc;
  color: #6b7080;
  padding: 1px 8px;
  cursor: pointer;
}

.act-btn:hover {
  color: #16a34a;
  border-color: #16a34a;
}

.bare-earth {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  width: 100%;
  padding: 14px 0;
  font-size: 12px;
  color: #8a8e99;
  border: none;
  border-top: 1px solid #d0d4dc;
  background: none;
}

#boot-status {
  font-size: 11px;
  color: #8a8e99;
  text-align: center;
  margin-top: 20px;
  line-height: 1.5;
}

.sprout-btn {
  position: fixed;
  bottom: 16px;
  right: 16px;
  background: none;
  border: none;
  color: #b0b4bd;
  cursor: pointer;
  padding: 4px;
  transition: color .2s;
}

.sprout-btn:hover {
  color: #16a34a;
}

.overlay {
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,0.15);
  display: none;
  align-items: center;
  justify-content: center;
  z-index: 100;
}

.overlay.open { display: flex; }

.modal {
  width: min(24rem, calc(100vw - 2rem));
  background: #f5f6f8;
  border: 1px solid #c0c4cc;
  padding: 20px;
}

.modal h2 {
  font-size: 13px;
  font-weight: 400;
  color: #6b7080;
  margin-bottom: 16px;
}

.modal input,
.modal textarea {
  width: 100%;
  background: #fff;
  border: 1px solid #c0c4cc;
  color: #2a2d35;
  padding: 8px 10px;
  font: inherit;
  font-size: 13px;
  margin-bottom: 10px;
  outline: none;
}

.modal input:focus,
.modal textarea:focus {
  border-color: #8a8e99;
}

.modal textarea {
  min-height: 80px;
  resize: vertical;
  line-height: 1.5;
}

.modal-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
  margin-top: 4px;
}

.modal-actions button {
  padding: 6px 14px;
  font: inherit;
  font-size: 12px;
  cursor: pointer;
  border: 1px solid #c0c4cc;
  background: none;
  color: #6b7080;
}

.modal-actions button:hover {
  color: #2a2d35;
  border-color: #8a8e99;
}

.modal-actions .go {
  color: #16a34a;
  border-color: #86d89b;
}

.modal-actions .go:hover {
  border-color: #16a34a;
}

.toast {
  position: fixed;
  bottom: 16px;
  left: 50%;
  transform: translateX(-50%) translateY(8px);
  background: #fff;
  border: 1px solid #c0c4cc;
  padding: 6px 14px;
  font-size: 11px;
  color: #16a34a;
  opacity: 0;
  transition: all .2s;
  pointer-events: none;
  z-index: 200;
}

.toast.show {
  opacity: 1;
  transform: translateX(-50%) translateY(0);
}
</style>
<link rel="stylesheet" href="` + pathAssets + `/sprout-logo.css">
</head>
<body>

<div class="page">
  <div class="brand">
    <div class="sprout-logo-shell" data-sprout-logo aria-hidden="true"></div>
    <div class="wordmark">AS</div>
    <div class="tagline">autosoftware</div>
    <div class="desc">Software that evolves from within.</div>
  </div>

  <div class="divider"></div>

  <div class="seeds">
    {{range .Seeds}}
    {{$seedName := .Name}}
    <div class="seed" onclick="toggleSeed(this)">
      <div class="seed-head">
        <span class="seed-pop">{{.Popularity}}</span>
        <div class="seed-mid">
          <div class="seed-name">{{.Name}}</div>
          <div class="seed-brief">{{.Brief}}</div>
        </div>
        <span class="arrow">&#x203A;</span>
      </div>
      <div class="seed-body">
        {{range .Realizations}}
        <div class="real">
          <span class="real-pop">{{.Popularity}}</span>
          <div class="real-mid">
            <span class="real-label">{{.Label}}</span>
            <span class="real-status {{.Status}}">{{.Status}}</span>
          </div>
          <button class="real-boot" onclick="event.stopPropagation(); bootReal(this)" data-label="{{.Label}}" data-seed="{{$seedName}}">LOAD</button>
        </div>
        {{end}}
        <div class="action-row" onclick="event.stopPropagation()">sprout new <button class="act-btn" onclick="sproutFrom('{{$seedName}}')">BRANCH</button></div>
      </div>
    </div>
    {{end}}
    <div class="bare-earth">create your own from bare earth <button class="act-btn" onclick="openSprout()">CREATE</button></div>
  </div>

  <div id="boot-status"></div>
</div>

<button class="sprout-btn" onclick="openSprout()" title="Plant a seed">
  <svg viewBox="0 0 24 24" width="14" height="14" fill="none"
       stroke="currentColor" stroke-width="1.5"
       stroke-linecap="round" stroke-linejoin="round">
    <path d="M12 22v-8"/>
    <path d="M12 14c-4 0-8-4-8-8 4 0 8 4 8 8z"/>
    <path d="M12 14c4 0 8-4 8-8-4 0-8 4-8 8z"/>
  </svg>
</button>

<div class="overlay" id="sprout-modal" onclick="closeSprout(event)">
  <div class="modal" onclick="event.stopPropagation()">
    <h2>What should change?</h2>
    <input type="text" id="sprout-summary" placeholder="Summary" maxlength="120">
    <textarea id="sprout-detail" placeholder="Describe what you want to see"></textarea>
    <div class="modal-actions">
      <button type="button" onclick="closeSprout()">Cancel</button>
      <button type="button" class="go" onclick="submitSprout()">Plant</button>
    </div>
  </div>
</div>

<div class="toast" id="toast"></div>

<script src="` + pathAssets + `/sprout-logo.js" defer></script>
<script>
/* boot status stream */
(function() {
  var el = document.getElementById('boot-status');
  var es = new EventSource('` + pathBoot + `');
  es.onmessage = function(e) { el.textContent = e.data; };
})();

/* seed expand/collapse — accordion: only one open at a time */
function toggleSeed(el) {
  var wasOpen = el.classList.contains('open');
  var all = document.querySelectorAll('.seed.open');
  for (var i = 0; i < all.length; i++) all[i].classList.remove('open');
  if (!wasOpen) el.classList.add('open');
}

/* load a realization (mock) */
function bootReal(el) {
  var label = el.dataset.label;
  var seed = el.dataset.seed;
  document.getElementById('boot-status').textContent = 'materializing ' + seed + ' (' + label + ')...';
  toast('Loading ' + label);
}

/* sprout new from a seed */
function sproutFrom(seedName) {
  document.getElementById('sprout-modal').classList.add('open');
  var s = document.getElementById('sprout-summary');
  s.value = seedName + ' — ';
  s.focus();
}

/* sprout */
function openSprout() {
  document.getElementById('sprout-modal').classList.add('open');
  document.getElementById('sprout-summary').focus();
}

function closeSprout(e) {
  if (e && e.target !== e.currentTarget) return;
  document.getElementById('sprout-modal').classList.remove('open');
  document.getElementById('sprout-summary').value = '';
  document.getElementById('sprout-detail').value = '';
}

function submitSprout() {
  var s = document.getElementById('sprout-summary').value.trim();
  var d = document.getElementById('sprout-detail').value.trim();
  if (!s) { document.getElementById('sprout-summary').focus(); return; }

  fetch('` + pathPlant + `', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ summary: s, detail: d })
  })
    .then(function(r) {
      if (!r.ok) throw new Error(r.status);
      return r.json();
    })
    .then(function(data) {
      closeSprout();
      toast(data.seed_id ? 'Planted ' + data.seed_id : 'Captured');
    })
    .catch(function(err) { toast('Failed: ' + err.message); });
}

function toast(msg) {
  var el = document.getElementById('toast');
  el.textContent = msg;
  el.classList.add('show');
  setTimeout(function() { el.classList.remove('show'); }, 2200);
}

document.addEventListener('keydown', function(e) {
  if (e.key === 'Escape') closeSprout();
});

/* feedback loop */
(function() {
  var ep = '` + pathIncidents + `';

  function send(kind, sev, p) {
    try {
      var b = JSON.stringify({
        kind: kind,
        severity: sev,
        message: p.message || kind,
        stack: p.stack || '',
        source: p.source || '',
        created_at: new Date().toISOString(),
        request: {
          route: location.pathname,
          method: 'BROWSER',
          page_url: location.href,
          user_agent: navigator.userAgent
        },
        data: p.data || {}
      });
      if (navigator.sendBeacon) {
        navigator.sendBeacon(ep, new Blob([b], { type: 'application/json' }));
      } else {
        fetch(ep, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          keepalive: true,
          body: b
        }).catch(function() {});
      }
    } catch(e) {}
  }

  window.addEventListener('error', function(ev) {
    send('window.error', 'error', {
      message: ev.message || 'Unhandled error',
      stack: ev.error && ev.error.stack || '',
      source: ev.filename || '',
      data: { lineno: ev.lineno || 0, colno: ev.colno || 0 }
    });
  });

  window.addEventListener('unhandledrejection', function(ev) {
    var r = ev.reason;
    send('window.unhandledrejection', 'error', {
      message: r instanceof Error ? r.message : String(r),
      stack: r && r.stack || ''
    });
  });

  if (document.body) {
    ['htmx:responseError', 'htmx:sendError', 'htmx:swapError', 'htmx:targetError']
      .forEach(function(n) {
        document.body.addEventListener(n, function(ev) {
          var d = ev.detail || {};
          send(n, 'error', {
            message: n,
            data: {
              request_path: d.pathInfo && d.pathInfo.requestPath || '',
              status: d.xhr && d.xhr.status || 0
            }
          });
        });
      });
  }
})();
</script>
</body>
</html>`))
