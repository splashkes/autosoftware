package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
