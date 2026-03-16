package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSameOriginUnsafeMethodsAllowsForwardedHTTPSOrigin(t *testing.T) {
	handler := SameOriginUnsafeMethodsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "http://autosoftware.app/boot/commands/realizations.launch", nil)
	req.Host = "autosoftware.app"
	req.Header.Set("Origin", "https://autosoftware.app")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("X-Forwarded-Proto", "https")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusNoContent, rec.Code, rec.Body.String())
	}
}

func TestSameOriginUnsafeMethodsRejectsMismatchedOrigin(t *testing.T) {
	handler := SameOriginUnsafeMethodsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "http://autosoftware.app/boot/commands/realizations.launch", nil)
	req.Host = "autosoftware.app"
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("X-Forwarded-Proto", "https")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if got := rec.Body.String(); got != "origin mismatch\n" {
		t.Fatalf("expected origin mismatch body, got %q", got)
	}
}
