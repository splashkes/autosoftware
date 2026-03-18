package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"as/kernel/internal/interactions"
)

func TestLaunchRedirectPathPreservesEncodedReference(t *testing.T) {
	got := launchRedirectPath(
		"/__runs/exec_demo_123/",
		"/registry/reading-room/actions/0003-customer-service-app%2Fa-web-mvp/tickets.reply",
		"/registry/reading-room/",
	)

	want := "/__runs/exec_demo_123/actions/0003-customer-service-app%2Fa-web-mvp/tickets.reply/"
	if got != want {
		t.Fatalf("launch redirect path = %q, want %q", got, want)
	}
}

func TestLaunchRedirectPathPreservesPermalinkAndEncodedReference(t *testing.T) {
	got := launchRedirectPath(
		"/__runs/exec_demo_123/",
		"/registry/reading-room/@sha256-abc123/contracts/0004-event-listings%2Fa-web-mvp",
		"/registry/reading-room/",
	)

	want := "/__runs/exec_demo_123/@sha256-abc123/contracts/0004-event-listings%2Fa-web-mvp/"
	if got != want {
		t.Fatalf("launch redirect path = %q, want %q", got, want)
	}
}

func TestRequestRedirectPathPrefersEscapedPath(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://autosoftware.app/registry/reading-room/actions/0003-customer-service-app%2Fa-web-mvp/tickets.reply", nil)
	req.URL.Path = "/registry/reading-room/actions/0003-customer-service-app/a-web-mvp/tickets.reply"
	req.URL.RawPath = "/registry/reading-room/actions/0003-customer-service-app%2Fa-web-mvp/tickets.reply"

	got := requestRedirectPath(req)
	want := "/registry/reading-room/actions/0003-customer-service-app%2Fa-web-mvp/tickets.reply"
	if got != want {
		t.Fatalf("request redirect path = %q, want %q", got, want)
	}
}

func TestTrimMountedRequestPrefixPreservesEncodedReference(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://autosoftware.app/registry/reading-room/actions/0003-customer-service-app%2Fa-web-mvp/tickets.reply", nil)
	req.URL.Path = "/registry/reading-room/actions/0003-customer-service-app/a-web-mvp/tickets.reply"
	req.URL.RawPath = "/registry/reading-room/actions/0003-customer-service-app%2Fa-web-mvp/tickets.reply"

	trimmed := trimMountedRequestPrefix(req, "/registry/reading-room/")

	if got, want := trimmed.URL.Path, "/actions/0003-customer-service-app/a-web-mvp/tickets.reply"; got != want {
		t.Fatalf("trimmed path = %q, want %q", got, want)
	}
	if got, want := trimmed.URL.RawPath, "/actions/0003-customer-service-app%2Fa-web-mvp/tickets.reply"; got != want {
		t.Fatalf("trimmed raw path = %q, want %q", got, want)
	}
	if got, want := trimmed.URL.EscapedPath(), "/actions/0003-customer-service-app%2Fa-web-mvp/tickets.reply"; got != want {
		t.Fatalf("trimmed escaped path = %q, want %q", got, want)
	}
}

func TestRealizationRoutingMiddlewarePreservesRawPathForStableMount(t *testing.T) {
	testMountedRoutingPreservesRawPath(t, "/registry/reading-room/")
}

func TestRealizationRoutingMiddlewarePreservesRawPathForPreviewMount(t *testing.T) {
	testMountedRoutingPreservesRawPath(t, "/__runs/exec_demo_123/")
}

func TestRealizationRoutingMiddlewareFallsBackToRegistryPathRouteForRegistryHost(t *testing.T) {
	type observedRequest struct {
		Path            string `json:"path"`
		ForwardedPrefix string `json:"forwarded_prefix"`
	}

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(observedRequest{
			Path:            r.URL.Path,
			ForwardedPrefix: r.Header.Get("X-Forwarded-Prefix"),
		})
	}))
	defer backend.Close()

	handler := realizationRoutingMiddleware([]realizationRoute{{
		Reference:  "0006-registry-browser/a-ledger-reading-room",
		PathPrefix: registryRoutePathPrefix,
		ProxyAddr:  strings.TrimPrefix(backend.URL, "http://"),
	}}, nil, nil, "", nil, "autosoftware.app", http.NotFoundHandler())

	req := httptest.NewRequest(http.MethodGet, "http://registry.autosoftware.app/objects", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("proxy response status = %d, want 200", rec.Code)
	}

	var got observedRequest
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode proxy response: %v", err)
	}

	if got.Path != "/objects" {
		t.Fatalf("path = %q, want %q", got.Path, "/objects")
	}
	if got.ForwardedPrefix != "" {
		t.Fatalf("X-Forwarded-Prefix = %q, want empty", got.ForwardedPrefix)
	}
}

func TestSelectLaunchTargetPrefersRoutableExecutionOverNewerQueuedExecution(t *testing.T) {
	items := []interactions.RealizationExecution{
		{
			ExecutionID: "exec_new",
			Reference:   "0006-registry-browser/a-ledger-reading-room",
			Status:      "launch_requested",
		},
		{
			ExecutionID: "exec_ready",
			Reference:   "0006-registry-browser/a-ledger-reading-room",
			Status:      "starting",
		},
	}

	got := selectLaunchTarget(items, map[string]string{
		"exec_ready": "/__runs/exec_ready/",
	})

	if got.ExecutionID != "exec_ready" {
		t.Fatalf("selected execution = %q, want %q", got.ExecutionID, "exec_ready")
	}
	if got.OpenPath != "/__runs/exec_ready/" {
		t.Fatalf("selected open path = %q, want %q", got.OpenPath, "/__runs/exec_ready/")
	}
}

func TestSelectLaunchTargetPrefersMostAdvancedPendingExecution(t *testing.T) {
	items := []interactions.RealizationExecution{
		{
			ExecutionID: "exec_new",
			Reference:   "0006-registry-browser/a-ledger-reading-room",
			Status:      "launch_requested",
		},
		{
			ExecutionID: "exec_starting",
			Reference:   "0006-registry-browser/a-ledger-reading-room",
			Status:      "starting",
		},
	}

	got := selectLaunchTarget(items, map[string]string{})

	if got.ExecutionID != "exec_starting" {
		t.Fatalf("selected execution = %q, want %q", got.ExecutionID, "exec_starting")
	}
	if got.OpenPath != "" {
		t.Fatalf("selected open path = %q, want empty", got.OpenPath)
	}
}

func TestExecutionSessionProjectionPath(t *testing.T) {
	got := executionSessionProjectionPath("exec_demo_123")
	want := "/boot/projections/realization-execution/sessions/exec_demo_123"
	if got != want {
		t.Fatalf("execution session projection path = %q, want %q", got, want)
	}
}

func TestRenderLaunchingPageIncludesSharedLaunchAssetsAndProjectionPath(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://autosoftware.app/event-ledger/", nil)
	rec := httptest.NewRecorder()

	renderLaunchingPage(rec, req, launchTarget{
		ExecutionID: "exec_event_123",
		Reference:   "0004-event-listings/a-ledger-web",
		Status:      "starting",
	}, "/event-ledger/")

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	body := rec.Body.String()
	wantContains := []string{
		`src="/assets/launch-state.js"`,
		`data-projection-path="/boot/projections/realization-execution/sessions/exec_event_123"`,
		`data-refresh-path="/event-ledger/"`,
		`Permanent Route`,
		`View Home`,
	}
	for _, want := range wantContains {
		if !strings.Contains(body, want) {
			t.Fatalf("expected body to contain %q, got %q", want, body)
		}
	}
}

func testMountedRoutingPreservesRawPath(t *testing.T, mountPrefix string) {
	t.Helper()

	type observedRequest struct {
		Path            string `json:"path"`
		RawPath         string `json:"raw_path"`
		EscapedPath     string `json:"escaped_path"`
		ForwardedPrefix string `json:"forwarded_prefix"`
	}

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(observedRequest{
			Path:            r.URL.Path,
			RawPath:         r.URL.RawPath,
			EscapedPath:     r.URL.EscapedPath(),
			ForwardedPrefix: r.Header.Get("X-Forwarded-Prefix"),
		})
	}))
	defer backend.Close()

	route := realizationRoute{
		Reference:  "0006-registry-browser/a-ledger-reading-room",
		PathPrefix: mountPrefix,
		ProxyAddr:  strings.TrimPrefix(backend.URL, "http://"),
	}
	handler := realizationRoutingMiddleware([]realizationRoute{route}, nil, nil, "", nil, "", http.NotFoundHandler())

	req := httptest.NewRequest(http.MethodGet, "http://autosoftware.app"+mountPrefix+"actions/0003-customer-service-app%2Fa-web-mvp/tickets.reply", nil)
	req.URL.Path = mountPrefix + "actions/0003-customer-service-app/a-web-mvp/tickets.reply"
	req.URL.RawPath = mountPrefix + "actions/0003-customer-service-app%2Fa-web-mvp/tickets.reply"

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("proxy response status = %d, want 200", rec.Code)
	}

	var got observedRequest
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode proxy response: %v", err)
	}

	if want := "/actions/0003-customer-service-app%2Fa-web-mvp/tickets.reply"; got.EscapedPath != want {
		t.Fatalf("escaped path = %q, want %q", got.EscapedPath, want)
	}
	if want := "/actions/0003-customer-service-app%2Fa-web-mvp/tickets.reply"; got.RawPath != want {
		t.Fatalf("raw path = %q, want %q", got.RawPath, want)
	}
	if want := strings.TrimSuffix(mountPrefix, "/"); got.ForwardedPrefix != want {
		t.Fatalf("X-Forwarded-Prefix = %q, want %q", got.ForwardedPrefix, want)
	}
}
