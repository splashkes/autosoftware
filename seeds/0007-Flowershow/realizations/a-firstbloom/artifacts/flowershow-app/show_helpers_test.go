package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

const (
	testHelperShowID   = "show_spring2025"
	testHelperShowSlug = "spring-rose-show-2025"
)

func TestShowHelperInviteCreateAndRedeem(t *testing.T) {
	a := testApp()

	issued, err := a.store.createShowHelperInvite(ShowHelperInviteInput{
		ShowID:    testHelperShowID,
		Label:     "Saturday volunteers",
		CreatedBy: "sub_admin_test",
	})
	if err != nil || issued == nil || issued.Invite == nil || issued.Token == "" {
		t.Fatalf("createShowHelperInvite returned err=%v issued=%#v", err, issued)
	}
	if issued.Invite.TokenHash == "" {
		t.Fatal("invite TokenHash must be set")
	}
	if issued.Invite.RevokedAt != nil {
		t.Fatal("freshly created invite must not be revoked")
	}
	if issued.Invite.ExpiresAt.Before(time.Now()) {
		t.Fatal("invite must expire in the future")
	}

	// Invite is findable by plaintext token
	invite, ok := a.store.findShowHelperInviteByToken(issued.Token)
	if !ok || invite == nil || invite.ID != issued.Invite.ID {
		t.Fatalf("findShowHelperInviteByToken: ok=%v invite=%#v", ok, invite)
	}

	// Redeem via the public POST handler
	mux := http.NewServeMux()
	mux.HandleFunc("POST /shows/{slug}/help-redeem", a.handlePublicShowHelpRedeem)

	form := url.Values{}
	form.Set("token", issued.Token)
	form.Set("name", "Volunteer Vee")
	form.Set("email", "vee@example.com")
	form.Set("role", "helper")
	req := httptest.NewRequest("POST", "/shows/"+testHelperShowSlug+"/help-redeem", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d body=%s", w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	var badgeCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == showBadgeCookieName {
			badgeCookie = c
		}
	}
	if badgeCookie == nil {
		t.Fatalf("expected %s cookie to be set, cookies=%v", showBadgeCookieName, cookies)
	}
	session, ok := a.store.findShowBadgeSessionByToken(badgeCookie.Value)
	if !ok || session == nil {
		t.Fatalf("findShowBadgeSessionByToken: ok=%v session=%#v", ok, session)
	}
	if session.Name != "Volunteer Vee" || session.Email != "vee@example.com" {
		t.Fatalf("session identity mismatch: %#v", session)
	}
	if session.ShowID != testHelperShowID {
		t.Fatalf("session ShowID=%q expected %q", session.ShowID, testHelperShowID)
	}
	if session.InviteID != issued.Invite.ID {
		t.Fatalf("session InviteID=%q expected %q", session.InviteID, issued.Invite.ID)
	}
}

func TestShowHelperInviteRevoke(t *testing.T) {
	a := testApp()
	issued, err := a.store.createShowHelperInvite(ShowHelperInviteInput{
		ShowID:    testHelperShowID,
		CreatedBy: "sub_admin_test",
	})
	if err != nil {
		t.Fatalf("createShowHelperInvite: %v", err)
	}
	if err := a.store.revokeShowHelperInvite(issued.Invite.ID); err != nil {
		t.Fatalf("revokeShowHelperInvite: %v", err)
	}

	// Direct lookup should now fail
	if _, ok := a.store.findShowHelperInviteByToken(issued.Token); ok {
		t.Fatal("findShowHelperInviteByToken should reject revoked invites")
	}

	// Public POST should return 400
	mux := http.NewServeMux()
	mux.HandleFunc("POST /shows/{slug}/help-redeem", a.handlePublicShowHelpRedeem)

	form := url.Values{}
	form.Set("token", issued.Token)
	form.Set("name", "Volunteer Vee")
	form.Set("email", "vee@example.com")
	form.Set("role", "helper")
	req := httptest.NewRequest("POST", "/shows/"+testHelperShowSlug+"/help-redeem", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for revoked token, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "no longer active") {
		t.Fatalf("expected friendly error in response body, got %s", w.Body.String())
	}
}

func TestShowBadgeSessionExpires(t *testing.T) {
	mem := newEmptyMemoryStore()
	mem.shows[testHelperShowID] = &Show{ID: testHelperShowID, Slug: testHelperShowSlug}
	issuedInvite, err := mem.createShowHelperInvite(ShowHelperInviteInput{
		ShowID:    testHelperShowID,
		CreatedBy: "sub_admin_test",
	})
	if err != nil {
		t.Fatalf("createShowHelperInvite: %v", err)
	}
	issuedSession, err := mem.createShowBadgeSession(ShowBadgeSessionInput{
		ShowID:   testHelperShowID,
		InviteID: issuedInvite.Invite.ID,
		Name:     "Volunteer Vee",
		Email:    "vee@example.com",
	})
	if err != nil || issuedSession == nil || issuedSession.Token == "" {
		t.Fatalf("createShowBadgeSession: %v %#v", err, issuedSession)
	}

	// Force the stored session to be already expired.
	stored := mem.showBadgeSessions[issuedSession.Session.ID]
	if stored == nil {
		t.Fatal("stored session missing")
	}
	stored.ExpiresAt = time.Now().UTC().Add(-time.Minute)

	if _, ok := mem.findShowBadgeSessionByToken(issuedSession.Token); ok {
		t.Fatal("expired badge session must be rejected by findShowBadgeSessionByToken")
	}

	// And with revoke we also reject:
	stored.ExpiresAt = time.Now().UTC().Add(time.Hour)
	now := time.Now().UTC()
	stored.RevokedAt = &now
	if _, ok := mem.findShowBadgeSessionByToken(issuedSession.Token); ok {
		t.Fatal("revoked badge session must be rejected by findShowBadgeSessionByToken")
	}
}

func TestRequireShowBadgeMiddleware(t *testing.T) {
	a := testApp()

	issued, err := a.store.createShowHelperInvite(ShowHelperInviteInput{
		ShowID:    testHelperShowID,
		CreatedBy: "sub_admin_test",
	})
	if err != nil {
		t.Fatalf("createShowHelperInvite: %v", err)
	}
	issuedSession, err := a.store.createShowBadgeSession(ShowBadgeSessionInput{
		ShowID:   testHelperShowID,
		InviteID: issued.Invite.ID,
		Name:     "Volunteer Vee",
		Email:    "vee@example.com",
	})
	if err != nil {
		t.Fatalf("createShowBadgeSession: %v", err)
	}

	called := false
	protected := a.requireShowBadge(func(w http.ResponseWriter, r *http.Request) {
		called = true
		session, ok := showBadgeFromContext(r.Context())
		if !ok || session == nil {
			t.Fatal("expected badge session in request context")
		}
		w.WriteHeader(http.StatusOK)
	})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /shows/{slug}/protected", protected)

	// No cookie → 303 redirect to /help-redeem
	req := httptest.NewRequest("GET", "/shows/"+testHelperShowSlug+"/protected", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if called {
		t.Fatal("protected handler must not run without a badge cookie")
	}
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", w.Code)
	}
	location := w.Result().Header.Get("Location")
	if !strings.Contains(location, "/shows/"+testHelperShowSlug+"/help-redeem") {
		t.Fatalf("expected redirect to help-redeem, got %s", location)
	}

	// With valid cookie → 200, handler called
	req = httptest.NewRequest("GET", "/shows/"+testHelperShowSlug+"/protected", nil)
	req.AddCookie(&http.Cookie{Name: showBadgeCookieName, Value: issuedSession.Token})
	w = httptest.NewRecorder()
	called = false
	mux.ServeHTTP(w, req)
	if !called {
		t.Fatalf("protected handler must run with valid cookie, got status %d", w.Code)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
