package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"as/kernel/internal/interactions"
)

type stubRateLimitEnforcer struct {
	input    interactions.EnforceRateLimitInput
	decision interactions.RateLimitDecision
	err      error
}

func (s *stubRateLimitEnforcer) EnforceRateLimit(_ context.Context, input interactions.EnforceRateLimitInput) (interactions.RateLimitDecision, error) {
	s.input = input
	return s.decision, s.err
}

type stubSessionResolver struct {
	session ResolvedSession
}

func (s stubSessionResolver) ResolveSession(context.Context, string) (ResolvedSession, error) {
	return s.session, nil
}

func TestRateLimitMiddlewareBlocksAnonymousJSONRequests(t *testing.T) {
	enforcer := &stubRateLimitEnforcer{
		decision: interactions.RateLimitDecision{
			Namespace:  "api.commands",
			Limit:      20,
			RetryAfter: 5 * time.Second,
		},
		err: &interactions.RateLimitError{
			Namespace:  "api.commands",
			Message:    "too many requests",
			RetryAfter: 5 * time.Second,
		},
	}

	handler := CorrelationMiddleware(RateLimitMiddleware(enforcer, RateLimitOptions{
		Enabled:             true,
		Window:              time.Minute,
		BlockDuration:       time.Minute,
		AnonymousReadLimit:  120,
		AnonymousWriteLimit: 20,
		SessionReadLimit:    240,
		SessionWriteLimit:   60,
		InternalLimit:       180,
		WorkerLimit:         1200,
		FeedbackLimit:       30,
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be reached when rate limited")
	})))

	req := httptest.NewRequest(http.MethodPost, "http://autosoftware.app/v1/commands/demo-items.create", nil)
	req.RemoteAddr = "203.0.113.9:1234"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d with body %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Retry-After") != "5" {
		t.Fatalf("expected Retry-After 5, got %q", rec.Header().Get("Retry-After"))
	}
	if enforcer.input.SubjectKey != "ip:203.0.113.9" {
		t.Fatalf("expected ip subject key, got %q", enforcer.input.SubjectKey)
	}
	if enforcer.input.Metadata["auth_state"] != "anonymous" {
		t.Fatalf("expected anonymous auth state, got %#v", enforcer.input.Metadata["auth_state"])
	}

	var payload ErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Error.Code != "rate_limited" {
		t.Fatalf("expected rate_limited code, got %q", payload.Error.Code)
	}
	if payload.Error.Details["auth_state"] != "anonymous" {
		t.Fatalf("expected anonymous detail, got %#v", payload.Error.Details["auth_state"])
	}
}

func TestRateLimitMiddlewareUsesPrincipalForResolvedSessions(t *testing.T) {
	enforcer := &stubRateLimitEnforcer{}

	handler := CorrelationMiddleware(SessionResolutionMiddleware(stubSessionResolver{
		session: ResolvedSession{
			SessionID:   "sess_123",
			PrincipalID: "prn_123",
			Status:      "active",
		},
	}, RateLimitMiddleware(enforcer, RateLimitOptions{
		Enabled:             true,
		Window:              time.Minute,
		BlockDuration:       time.Minute,
		AnonymousReadLimit:  120,
		AnonymousWriteLimit: 20,
		SessionReadLimit:    240,
		SessionWriteLimit:   60,
		InternalLimit:       180,
		WorkerLimit:         1200,
		FeedbackLimit:       30,
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))))

	req := httptest.NewRequest(http.MethodGet, "http://autosoftware.app/v1/registry/catalog", nil)
	req.AddCookie(&http.Cookie{Name: DefaultSessionCookieName, Value: "sess_123"})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if enforcer.input.SubjectKey != "principal:prn_123" {
		t.Fatalf("expected principal subject key, got %q", enforcer.input.SubjectKey)
	}
	if enforcer.input.PrincipalID != "prn_123" {
		t.Fatalf("expected principal_id prn_123, got %q", enforcer.input.PrincipalID)
	}
	if enforcer.input.Metadata["auth_state"] != "session" {
		t.Fatalf("expected session auth state, got %#v", enforcer.input.Metadata["auth_state"])
	}
}

func TestRateLimitMiddlewarePrefersCloudflareClientIP(t *testing.T) {
	enforcer := &stubRateLimitEnforcer{}

	handler := CorrelationMiddleware(RateLimitMiddleware(enforcer, RateLimitOptions{
		Enabled:             true,
		Window:              time.Minute,
		BlockDuration:       time.Minute,
		AnonymousReadLimit:  120,
		AnonymousWriteLimit: 20,
		SessionReadLimit:    240,
		SessionWriteLimit:   60,
		InternalLimit:       180,
		WorkerLimit:         1200,
		FeedbackLimit:       30,
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))

	req := httptest.NewRequest(http.MethodGet, "http://autosoftware.app/v1/registry/catalog", nil)
	req.RemoteAddr = "10.0.0.8:1234"
	req.Header.Set("CF-Connecting-IP", "198.51.100.40")
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.8")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if enforcer.input.SubjectKey != "ip:198.51.100.40" {
		t.Fatalf("expected Cloudflare IP subject key, got %q", enforcer.input.SubjectKey)
	}
}

func TestRateLimitMiddlewareFallsBackToTrueClientIP(t *testing.T) {
	enforcer := &stubRateLimitEnforcer{}

	handler := CorrelationMiddleware(RateLimitMiddleware(enforcer, RateLimitOptions{
		Enabled:             true,
		Window:              time.Minute,
		BlockDuration:       time.Minute,
		AnonymousReadLimit:  120,
		AnonymousWriteLimit: 20,
		SessionReadLimit:    240,
		SessionWriteLimit:   60,
		InternalLimit:       180,
		WorkerLimit:         1200,
		FeedbackLimit:       30,
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))

	req := httptest.NewRequest(http.MethodGet, "http://autosoftware.app/v1/registry/catalog", nil)
	req.RemoteAddr = "10.0.0.8:1234"
	req.Header.Set("True-Client-IP", "198.51.100.41")
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.8")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if enforcer.input.SubjectKey != "ip:198.51.100.41" {
		t.Fatalf("expected True-Client-IP subject key, got %q", enforcer.input.SubjectKey)
	}
}
