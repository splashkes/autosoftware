package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

type mockAuthProvider struct {
	passwordLogin       func(context.Context, string, string) (*UserIdentity, error)
	startEmailOTP       func(context.Context, string) (*authStartResult, error)
	verifyEmailOTP      func(context.Context, string, string, string) (*UserIdentity, error)
	startForgotPassword func(context.Context, string) error
	confirmForgot       func(context.Context, string, string, string) error
}

func (m *mockAuthProvider) Enabled() bool { return true }

func (m *mockAuthProvider) PasswordLogin(ctx context.Context, email, password string) (*UserIdentity, error) {
	if m.passwordLogin == nil {
		return nil, nil
	}
	return m.passwordLogin(ctx, email, password)
}

func (m *mockAuthProvider) StartEmailOTP(ctx context.Context, email string) (*authStartResult, error) {
	if m.startEmailOTP == nil {
		return nil, nil
	}
	return m.startEmailOTP(ctx, email)
}

func (m *mockAuthProvider) VerifyEmailOTP(ctx context.Context, email, session, code string) (*UserIdentity, error) {
	if m.verifyEmailOTP == nil {
		return nil, nil
	}
	return m.verifyEmailOTP(ctx, email, session, code)
}

func (m *mockAuthProvider) StartForgotPassword(ctx context.Context, email string) error {
	if m.startForgotPassword == nil {
		return nil
	}
	return m.startForgotPassword(ctx, email)
}

func (m *mockAuthProvider) ConfirmForgotPassword(ctx context.Context, email, code, newPassword string) error {
	if m.confirmForgot == nil {
		return nil
	}
	return m.confirmForgot(ctx, email, code, newPassword)
}

func testApp() *app {
	dir, err := os.MkdirTemp("", "flowershow-media-test-*")
	if err != nil {
		panic(err)
	}
	store := newMemoryStore()
	a := &app{
		store:        store,
		authority:    newRuntimeAuthorityResolver(store),
		templates:    parseTemplates(),
		serviceToken: "test-token",
		sseBroker:    newSSEBroker(),
		media:        &localMediaStore{dir: dir},
		sessions:     newAuthStateStore(store, nil),
	}
	if a.authority != nil {
		if err := a.authority.Init(context.Background(), store); err != nil {
			panic(err)
		}
	}
	return a
}

func jsonRequest(method, path, body string) *http.Request {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func addServiceToken(req *http.Request) {
	req.Header.Set("Authorization", "Bearer test-token")
}

func addAdminSession(t *testing.T, a *app, req *http.Request) {
	t.Helper()
	assignRuntimeRole(t, a, UserRoleInput{SubjectID: "sub_admin_api", CognitoSub: "sub_admin_api", Role: "admin"})
	w := httptest.NewRecorder()
	if err := a.setUserSession(w, req, UserIdentity{
		SubjectID:  "sub_admin_api",
		CognitoSub: "sub_admin_api",
		Email:      "admin@example.com",
	}); err != nil {
		t.Fatalf("set user session: %v", err)
	}
	for _, cookie := range w.Result().Cookies() {
		req.AddCookie(cookie)
	}
}

func addRoleSession(t *testing.T, a *app, req *http.Request, input UserRoleInput, user UserIdentity) {
	t.Helper()
	assignRuntimeRole(t, a, input)
	w := httptest.NewRecorder()
	if err := a.setUserSession(w, req, user); err != nil {
		t.Fatalf("set user session: %v", err)
	}
	for _, cookie := range w.Result().Cookies() {
		req.AddCookie(cookie)
	}
}

func assignRuntimeRole(t *testing.T, a *app, input UserRoleInput) *UserRole {
	t.Helper()
	if a.authority == nil {
		t.Fatal("runtime authority unavailable")
	}
	role, err := a.authority.AssignRole(context.Background(), input, "test_grantor")
	if err != nil {
		t.Fatalf("assign runtime role: %v", err)
	}
	return role
}

func extractIssuedAgentToken(t *testing.T, body string) string {
	t.Helper()
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?s)<input[^>]*id="issued_agent_token"[^>]*value="([^"]+)"`),
		regexp.MustCompile(`(?s)<input[^>]*data-issued-agent-token[^>]*value="([^"]+)"`),
		regexp.MustCompile(`(?s)data-issued-agent-token>([^<]+)</textarea>`),
	}
	token := ""
	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(body)
		if len(matches) == 2 {
			token = strings.TrimSpace(matches[1])
			break
		}
	}
	if token == "" {
		t.Fatal("issued agent token value empty")
	}
	return token
}

func TestHealthEndpoint(t *testing.T) {
	a := testApp()
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	a.handleHealth(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "ok" {
		t.Fatalf("expected ok, got %v", body["status"])
	}
	if body["seed"] != "0007-Flowershow" {
		t.Fatalf("expected 0007-Flowershow, got %v", body["seed"])
	}
}

func TestHomePageLoads(t *testing.T) {
	a := testApp()
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	a.handleHome(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Flowershow") {
		t.Fatal("home page missing title")
	}
	if !strings.Contains(body, "Spring Rose Show") {
		t.Fatal("home page missing seeded show")
	}
	if !strings.Contains(body, "/v1/contracts/0007-Flowershow/a-firstbloom") {
		t.Fatal("home page missing shared agent access widget")
	}
}

func TestHomePagePrefixesAgentWidgetLinksWhenBasePathIsSet(t *testing.T) {
	prev := globalBasePath
	globalBasePath = "/flowershow"
	defer func() {
		globalBasePath = prev
	}()

	a := testApp()
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	a.handleHome(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	for _, expected := range []string{
		`href="/flowershow/v1/contracts"`,
		`href="/flowershow/v1/contracts/0007-Flowershow/a-firstbloom"`,
		`href="/flowershow/v1/projections/0007-Flowershow/shows"`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("home page missing base-prefixed widget link %s", expected)
		}
	}
}

func TestShowDetailBySlug(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /shows/{slug}", a.handleShowDetail)

	req := httptest.NewRequest("GET", "/shows/spring-rose-show-2025", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Spring Rose Show 2025") {
		t.Fatal("show detail missing name")
	}
	if !strings.Contains(body, "Horticulture Specimens") {
		t.Fatal("show detail missing division")
	}
}

func TestShowSummaryPage(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /shows/{slug}/summary", a.handleShowSummary)

	req := httptest.NewRequest("GET", "/shows/spring-rose-show-2025/summary", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Live Winner Summary") {
		t.Fatal("winner summary missing live heading")
	}
	if !strings.Contains(body, "Formal Class Results") {
		t.Fatal("winner summary missing formal class list")
	}
	if !strings.Contains(body, "Peace") {
		t.Fatal("winner summary missing seeded winner")
	}
}

func TestShowDetailNotFound(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /shows/{slug}", a.handleShowDetail)

	req := httptest.NewRequest("GET", "/shows/nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestClassBrowse(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /shows/{slug}/classes", a.handleClassBrowse)

	req := httptest.NewRequest("GET", "/shows/spring-rose-show-2025/classes", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Hybrid Tea") {
		t.Fatal("class browse missing section")
	}
}

func TestPersonHistoryPage(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /people/{personID}", a.handlePersonDetail)

	req := httptest.NewRequest("GET", "/people/person_01", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "MC") {
		t.Fatal("person history missing entrant initials")
	}
	if !strings.Contains(body, "Peace") {
		t.Fatal("person history missing seeded entry")
	}
}

func TestEntryDetail(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /entries/{entryID}", a.handleEntryDetail)

	req := httptest.NewRequest("GET", "/entries/entry_01", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Peace") {
		t.Fatal("entry detail missing name")
	}
	// Privacy check: should show initials, not full name in public view
	if !strings.Contains(body, "MC") {
		t.Fatal("entry detail missing initials")
	}
}

func TestSuppressedEntryHiddenFromPublicViews(t *testing.T) {
	a := testApp()
	if err := a.store.setEntrySuppressed("entry_01", true); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /entries/{entryID}", a.handleEntryDetail)
	mux.HandleFunc("GET /shows/{slug}", a.handleShowDetail)

	req := httptest.NewRequest("GET", "/entries/entry_01", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for suppressed entry, got %d", w.Code)
	}

	req = httptest.NewRequest("GET", "/shows/spring-rose-show-2025", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "Peace") {
		t.Fatal("suppressed entry should not appear on public show page")
	}
}

func TestTaxonomyBrowse(t *testing.T) {
	a := testApp()
	req := httptest.NewRequest("GET", "/taxonomy", nil)
	w := httptest.NewRecorder()
	a.handleTaxonomyBrowse(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Rose") {
		t.Fatal("taxonomy missing Rose")
	}
}

func TestLeaderboard(t *testing.T) {
	a := testApp()
	req := httptest.NewRequest("GET", "/leaderboard?org=org_demo1&season=2025", nil)
	w := httptest.NewRecorder()
	a.handleLeaderboard(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Leaderboard") {
		t.Fatal("leaderboard page missing title")
	}
}

func TestLeaderboardAllOrganizations(t *testing.T) {
	a := testApp()
	req := httptest.NewRequest("GET", "/leaderboard?org=all&season=2025", nil)
	w := httptest.NewRecorder()
	a.handleLeaderboard(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "All Organizations") {
		t.Fatal("all-org leaderboard missing heading")
	}
	if !strings.Contains(body, "Metro Rose Society") {
		t.Fatal("all-org leaderboard missing organization board")
	}
}

func TestBrowsePageFiltersResults(t *testing.T) {
	a := testApp()
	req := httptest.NewRequest("GET", "/browse?q=Peace&domain=horticulture&taxon=taxon_ht", nil)
	w := httptest.NewRecorder()
	a.handleBrowse(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Peace") {
		t.Fatal("browse page missing matched entry")
	}
	if strings.Contains(body, "Garden Elegance") {
		t.Fatal("browse page included non-matching entry")
	}
}

func TestAdminLoginFlow(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/login", a.handleAdminLogin)
	mux.HandleFunc("GET /account", a.requireSignedInPage(a.handleAccount))
	mux.HandleFunc("GET /admin", a.requireAdmin(a.handleAdminDashboard))

	// GET login page
	req := httptest.NewRequest("GET", "/admin/login", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "Bootstrap Override") {
		t.Fatal("login page should not expose bootstrap override access")
	}
	if !strings.Contains(w.Body.String(), "Sign-In Unavailable") {
		t.Fatal("login page should explain missing cognito sign-in configuration")
	}

	// Access admin still requires a signed session with an admin role.
	req = httptest.NewRequest("GET", "/admin", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", w.Code)
	}
	if got := w.Result().Header.Get("Location"); got != "/admin/login" {
		t.Fatalf("expected redirect to /admin/login, got %q", got)
	}
}

func TestAdminLoginPageShowsDirectSiteAuthOptionsWhenCognitoEnabled(t *testing.T) {
	a := testApp()
	a.auth = &mockAuthProvider{}

	req := httptest.NewRequest("GET", "/admin/login", nil)
	w := httptest.NewRecorder()
	a.handleAdminLogin(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Login: Email") {
		t.Fatal("login page missing initial email step")
	}
	if !strings.Contains(body, ">Next<") {
		t.Fatal("login page missing next continuation action")
	}
	if strings.Contains(body, `type="password" id="login_password"`) {
		t.Fatal("initial step should not show a password field")
	}
	if strings.Contains(body, "Enter password instead") {
		t.Fatal("initial step should not show the email-code option")
	}
	if strings.Contains(body, "Continue With Cognito") {
		t.Fatal("login page should not send users to hosted cognito ui")
	}
}

func TestAdminDirectPasswordLogin(t *testing.T) {
	a := testApp()
	a.auth = &mockAuthProvider{
		passwordLogin: func(_ context.Context, email, password string) (*UserIdentity, error) {
			if email != "simon@example.com" || password != "secret" {
				t.Fatalf("unexpected credentials %q / %q", email, password)
			}
			return &UserIdentity{CognitoSub: "sub_direct", Email: email}, nil
		},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/login", a.handleAdminLogin)
	mux.HandleFunc("GET /account", a.requireSignedInPage(a.handleAccount))
	mux.HandleFunc("POST /auth/login/password", a.handleAdminPasswordLogin)
	mux.HandleFunc("GET /admin", a.requireAdmin(a.handleAdminDashboard))

	stepReq := httptest.NewRequest("GET", "/admin/login?email=simon%40example.com&mode=password", nil)
	stepW := httptest.NewRecorder()
	mux.ServeHTTP(stepW, stepReq)
	if stepW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", stepW.Code)
	}
	if !strings.Contains(stepW.Body.String(), "Enter password") {
		t.Fatal("password fallback step should render before credential submission")
	}
	if !strings.Contains(stepW.Body.String(), "Email a login code instead") {
		t.Fatal("password step should include the email-code fallback action")
	}

	req := httptest.NewRequest("POST", "/auth/login/password", strings.NewReader("email=simon%40example.com&password=secret"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	if got := w.Result().Header.Get("Location"); got != "/account" {
		t.Fatalf("expected redirect to /account, got %q", got)
	}

	accountReq := httptest.NewRequest("GET", "/account", nil)
	for _, cookie := range w.Result().Cookies() {
		accountReq.AddCookie(cookie)
	}
	accountW := httptest.NewRecorder()
	mux.ServeHTTP(accountW, accountReq)
	if accountW.Code != http.StatusOK {
		t.Fatalf("expected signed-in profile page, got %d", accountW.Code)
	}
	if !strings.Contains(accountW.Body.String(), "simon@example.com") {
		t.Fatal("account page missing signed-in email")
	}

	adminReq := httptest.NewRequest("GET", "/admin", nil)
	for _, cookie := range w.Result().Cookies() {
		adminReq.AddCookie(cookie)
	}
	adminW := httptest.NewRecorder()
	mux.ServeHTTP(adminW, adminReq)
	if adminW.Code != http.StatusSeeOther {
		t.Fatalf("expected non-admin user to redirect away from admin, got %d", adminW.Code)
	}
	if got := adminW.Result().Header.Get("Location"); got != "/account?notice=admin_required" {
		t.Fatalf("expected redirect to signed-in account page, got %q", got)
	}
}

func TestAdminEmailOTPLoginFlow(t *testing.T) {
	a := testApp()
	a.auth = &mockAuthProvider{
		startEmailOTP: func(_ context.Context, email string) (*authStartResult, error) {
			if email != "simon@example.com" {
				t.Fatalf("unexpected email %q", email)
			}
			return &authStartResult{
				Pending: &pendingAuthState{
					Flow:      pendingAuthFlowEmailOTP,
					Email:     email,
					Session:   "pending-session",
					ExpiresAt: time.Now().UTC().Add(10 * time.Minute).Unix(),
				},
			}, nil
		},
		verifyEmailOTP: func(_ context.Context, email, session, code string) (*UserIdentity, error) {
			if email != "simon@example.com" || session != "pending-session" || code != "123456" {
				t.Fatalf("unexpected verify payload %q %q %q", email, session, code)
			}
			return &UserIdentity{SubjectID: "sub_admin", CognitoSub: "sub_admin", Email: email}, nil
		},
	}
	assignRuntimeRole(t, a, UserRoleInput{SubjectID: "sub_admin", CognitoSub: "sub_admin", Role: "admin"})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/login", a.handleAdminLogin)
	mux.HandleFunc("GET /account", a.requireSignedInPage(a.handleAccount))
	mux.HandleFunc("POST /auth/login/back", a.handleAdminLoginBack)
	mux.HandleFunc("POST /auth/login/email-code", a.handleAdminEmailCodeStart)
	mux.HandleFunc("POST /auth/login/email-code/verify", a.handleAdminEmailCodeVerify)
	mux.HandleFunc("GET /admin", a.requireAdmin(a.handleAdminDashboard))

	req := httptest.NewRequest("POST", "/auth/login/email-code", strings.NewReader("email=simon%40example.com"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	if got := w.Result().Header.Get("Location"); got != "/admin/login?notice=email-code-sent" {
		t.Fatalf("unexpected location %q", got)
	}

	loginReq := httptest.NewRequest("GET", "/admin/login?notice=email-code-sent", nil)
	for _, cookie := range w.Result().Cookies() {
		loginReq.AddCookie(cookie)
	}
	loginW := httptest.NewRecorder()
	mux.ServeHTTP(loginW, loginReq)
	if !strings.Contains(loginW.Body.String(), "Let") || !strings.Contains(loginW.Body.String(), "confirm your email") {
		t.Fatal("login page missing otp verification form")
	}
	if !strings.Contains(loginW.Body.String(), "Enter password instead") {
		t.Fatal("otp verification step should expose the password fallback action")
	}
	if !strings.Contains(loginW.Body.String(), "You can request another code in") {
		t.Fatal("otp verification step should show the resend cooldown")
	}

	verifyReq := httptest.NewRequest("POST", "/auth/login/email-code/verify", strings.NewReader("code=123456"))
	verifyReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range w.Result().Cookies() {
		verifyReq.AddCookie(cookie)
	}
	verifyW := httptest.NewRecorder()
	mux.ServeHTTP(verifyW, verifyReq)
	if verifyW.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", verifyW.Code)
	}

	adminReq := httptest.NewRequest("GET", "/admin", nil)
	for _, cookie := range verifyW.Result().Cookies() {
		adminReq.AddCookie(cookie)
	}
	adminW := httptest.NewRecorder()
	mux.ServeHTTP(adminW, adminReq)
	if adminW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", adminW.Code)
	}
}

func TestAccountPageRequiresSignedInSession(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /account", a.requireSignedInPage(a.handleAccount))

	req := httptest.NewRequest("GET", "/account", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	if got := w.Result().Header.Get("Location"); got != "/admin/login" {
		t.Fatalf("expected redirect to /admin/login, got %q", got)
	}
}

func TestBrowserSessionCookieIsOpaqueAndSurvivesNewAppInstance(t *testing.T) {
	store := newMemoryStore()
	sessions := newAuthStateStore(store, nil)
	a1 := &app{
		store:        store,
		authority:    newRuntimeAuthorityResolver(store),
		templates:    parseTemplates(),
		serviceToken: "test-token",
		sseBroker:    newSSEBroker(),
		sessions:     sessions,
	}
	a2 := &app{
		store:        store,
		authority:    newRuntimeAuthorityResolver(store),
		templates:    parseTemplates(),
		serviceToken: "test-token",
		sseBroker:    newSSEBroker(),
		sessions:     sessions,
	}
	if err := a1.authority.Init(context.Background(), store); err != nil {
		t.Fatalf("init authority a1: %v", err)
	}
	if err := a2.authority.Init(context.Background(), store); err != nil {
		t.Fatalf("init authority a2: %v", err)
	}

	req := httptest.NewRequest("GET", "/account", nil)
	w := httptest.NewRecorder()
	if err := a1.setUserSession(w, req, UserIdentity{
		CognitoSub: "sub_reboot",
		Email:      "reboot@example.com",
		Name:       "Reboot User",
	}); err != nil {
		t.Fatalf("set user session: %v", err)
	}

	var sessionCookie *http.Cookie
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == authSessionCookieName {
			sessionCookie = cookie
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("session cookie missing")
	}
	if strings.Contains(sessionCookie.Value, "reboot@example.com") || strings.Contains(sessionCookie.Value, "sub_reboot") {
		t.Fatal("session cookie should be opaque, not an embedded identity payload")
	}

	rebootReq := httptest.NewRequest("GET", "/account", nil)
	rebootReq.AddCookie(sessionCookie)
	user, ok := a2.currentUser(rebootReq)
	if !ok {
		t.Fatal("expected a fresh app instance to resolve the stored browser session")
	}
	if user.Email != "reboot@example.com" {
		t.Fatalf("unexpected restored user %+v", *user)
	}
}

func TestAccountPageShowsTokenManagerAndAdminDashboardLinksToIt(t *testing.T) {
	a := testApp()
	assignRuntimeRole(t, a, UserRoleInput{
		SubjectID:  "sub_admin_account",
		CognitoSub: "sub_admin_account",
		Role:       "admin",
	})

	accountReq := httptest.NewRequest("GET", "/account", nil)
	sessionW := httptest.NewRecorder()
	if err := a.setUserSession(sessionW, accountReq, UserIdentity{
		SubjectID:  "sub_admin_account",
		CognitoSub: "sub_admin_account",
		Email:      "admin-account@example.com",
	}); err != nil {
		t.Fatalf("set session: %v", err)
	}
	for _, cookie := range sessionW.Result().Cookies() {
		accountReq.AddCookie(cookie)
	}
	accountW := httptest.NewRecorder()
	a.handleAccount(accountW, accountReq)
	if accountW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", accountW.Code)
	}
	body := accountW.Body.String()
	if !strings.Contains(body, "Tokens / API") {
		t.Fatal("account page missing token navigation")
	}
	if strings.Contains(body, "Generate Agent Token") {
		t.Fatal("account overview should not expand the token generator by default")
	}

	adminReq := httptest.NewRequest("GET", "/admin", nil)
	for _, cookie := range sessionW.Result().Cookies() {
		adminReq.AddCookie(cookie)
	}
	adminW := httptest.NewRecorder()
	a.requireAdmin(a.handleAdminDashboard)(adminW, adminReq)
	if adminW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", adminW.Code)
	}
	if !strings.Contains(adminW.Body.String(), "/account?section=tokens#agent-tokens") {
		t.Fatal("admin dashboard should link to the shared account token manager")
	}
}

func TestViewerAccountTokenCanReadAccountButNotAdminAPI(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /account/agent-tokens", a.requireSignedInPage(a.handleAccountTokenCreate))
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/account", a.handleAPIAccount)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/admin/dashboard", a.handleAPIAdminDashboard)

	sessionW := httptest.NewRecorder()
	sessionReq := httptest.NewRequest("GET", "/account", nil)
	if err := a.setUserSession(sessionW, sessionReq, UserIdentity{
		CognitoSub: "sub_viewer_token",
		Email:      "viewer-token@example.com",
		Name:       "Viewer Token",
	}); err != nil {
		t.Fatalf("set session: %v", err)
	}

	createReq := httptest.NewRequest("POST", "/account/agent-tokens", strings.NewReader("label=Viewer+Assistant&expires_in_days=7&permission_profile=account_agent"))
	createReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range sessionW.Result().Cookies() {
		createReq.AddCookie(cookie)
	}
	createW := httptest.NewRecorder()
	mux.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", createW.Code, createW.Body.String())
	}
	if !strings.Contains(createW.Body.String(), "Copy This Token Now") {
		t.Fatal("issued token flow should focus on the one-time token state")
	}
	if strings.Contains(createW.Body.String(), "Issue A New Agent / API Access Token") {
		t.Fatal("issued token flow should hide the token generator while the token is visible")
	}
	token := extractIssuedAgentToken(t, createW.Body.String())

	accountReq := httptest.NewRequest("GET", "/v1/projections/0007-Flowershow/account", nil)
	accountReq.Header.Set("Authorization", "Bearer "+token)
	accountW := httptest.NewRecorder()
	mux.ServeHTTP(accountW, accountReq)
	if accountW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", accountW.Code, accountW.Body.String())
	}
	if !strings.Contains(accountW.Body.String(), `"email":"viewer-token@example.com"`) {
		t.Fatal("account projection missing token owner identity")
	}

	adminReq := httptest.NewRequest("GET", "/v1/projections/0007-Flowershow/admin/dashboard", nil)
	adminReq.Header.Set("Authorization", "Bearer "+token)
	adminW := httptest.NewRecorder()
	mux.ServeHTTP(adminW, adminReq)
	if adminW.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", adminW.Code, adminW.Body.String())
	}
	if !strings.Contains(adminW.Body.String(), `"auth_mode":"agent_token"`) {
		t.Fatal("permission denial should report agent_token auth mode")
	}
}

func TestAdminScopedAgentTokensRespectCapabilitiesAndRevocation(t *testing.T) {
	a := testApp()
	assignRuntimeRole(t, a, UserRoleInput{
		SubjectID:  "sub_admin_token_owner",
		CognitoSub: "sub_admin_token_owner",
		Role:       "admin",
	})

	mux := http.NewServeMux()
	mux.HandleFunc("POST /account/agent-tokens", a.requireSignedInPage(a.handleAccountTokenCreate))
	mux.HandleFunc("POST /account/agent-tokens/{tokenID}/revoke", a.requireSignedInPage(a.handleAccountTokenRevoke))
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/shows/{id}/workspace", a.handleAPIShowWorkspace)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/shows/{id}/board", a.handleAPIShowBoard)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/ledger/{objectID}", a.handleAPILedger)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/roles.assign", a.handleAPICommand)

	sessionW := httptest.NewRecorder()
	sessionReq := httptest.NewRequest("GET", "/account", nil)
	if err := a.setUserSession(sessionW, sessionReq, UserIdentity{
		SubjectID:  "sub_admin_token_owner",
		CognitoSub: "sub_admin_token_owner",
		Email:      "admin-token@example.com",
		Name:       "Admin Token Owner",
	}); err != nil {
		t.Fatalf("set session: %v", err)
	}

	createReq := httptest.NewRequest("POST", "/account/agent-tokens", strings.NewReader("label=Show+Operator&expires_in_days=7&permission_profile=show_operator"))
	createReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range sessionW.Result().Cookies() {
		createReq.AddCookie(cookie)
	}
	createW := httptest.NewRecorder()
	mux.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", createW.Code, createW.Body.String())
	}
	operatorToken := extractIssuedAgentToken(t, createW.Body.String())

	workspaceReq := httptest.NewRequest("GET", "/v1/projections/0007-Flowershow/shows/show_spring2025/workspace", nil)
	workspaceReq.Header.Set("Authorization", "Bearer "+operatorToken)
	workspaceW := httptest.NewRecorder()
	mux.ServeHTTP(workspaceW, workspaceReq)
	if workspaceW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", workspaceW.Code, workspaceW.Body.String())
	}

	boardReq := httptest.NewRequest("GET", "/v1/projections/0007-Flowershow/shows/show_spring2025/board", nil)
	boardReq.Header.Set("Authorization", "Bearer "+operatorToken)
	boardW := httptest.NewRecorder()
	mux.ServeHTTP(boardW, boardReq)
	if boardW.Code != http.StatusOK {
		t.Fatalf("expected 200 board, got %d: %s", boardW.Code, boardW.Body.String())
	}
	if !strings.Contains(boardW.Body.String(), "board_divisions") {
		t.Fatal("board projection missing board_divisions")
	}

	ledgerReq := httptest.NewRequest("GET", "/v1/projections/0007-Flowershow/ledger/show_spring2025", nil)
	ledgerReq.Header.Set("Authorization", "Bearer "+operatorToken)
	ledgerW := httptest.NewRecorder()
	mux.ServeHTTP(ledgerW, ledgerReq)
	if ledgerW.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", ledgerW.Code, ledgerW.Body.String())
	}

	roleReq := jsonRequest("POST", "/v1/commands/0007-Flowershow/roles.assign", `{"cognito_sub":"sub_agent_target","role":"admin"}`)
	roleReq.Header.Set("Authorization", "Bearer "+operatorToken)
	roleW := httptest.NewRecorder()
	mux.ServeHTTP(roleW, roleReq)
	if roleW.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", roleW.Code, roleW.Body.String())
	}

	tokenID := a.store.listAgentTokensBySubject("sub_admin_token_owner")[0].ID
	revokeReq := httptest.NewRequest("POST", "/account/agent-tokens/"+tokenID+"/revoke", nil)
	for _, cookie := range sessionW.Result().Cookies() {
		revokeReq.AddCookie(cookie)
	}
	revokeW := httptest.NewRecorder()
	mux.ServeHTTP(revokeW, revokeReq)
	if revokeW.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", revokeW.Code)
	}

	accountReq := httptest.NewRequest("GET", "/v1/projections/0007-Flowershow/shows/show_spring2025/workspace", nil)
	accountReq.Header.Set("Authorization", "Bearer "+operatorToken)
	accountW := httptest.NewRecorder()
	mux.ServeHTTP(accountW, accountReq)
	if accountW.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after revocation, got %d: %s", accountW.Code, accountW.Body.String())
	}
}

func TestAdminLoginRedirectsSignedInUserToRoleAwareDestination(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/login", a.handleAdminLogin)
	mux.HandleFunc("GET /account", a.requireSignedInPage(a.handleAccount))
	mux.HandleFunc("GET /admin", a.requireAdmin(a.handleAdminDashboard))

	userReq := httptest.NewRequest("GET", "/admin/login", nil)
	userW := httptest.NewRecorder()
	if err := a.setUserSession(userW, userReq, UserIdentity{
		CognitoSub: "sub_user",
		Email:      "viewer@example.com",
	}); err != nil {
		t.Fatalf("set viewer session: %v", err)
	}
	for _, cookie := range userW.Result().Cookies() {
		userReq.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, userReq)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	if got := w.Result().Header.Get("Location"); got != "/account" {
		t.Fatalf("expected redirect to /account, got %q", got)
	}

	assignRuntimeRole(t, a, UserRoleInput{SubjectID: "sub_admin_login", CognitoSub: "sub_admin_login", Role: "admin"})
	adminReq := httptest.NewRequest("GET", "/admin/login", nil)
	adminSession := httptest.NewRecorder()
	if err := a.setUserSession(adminSession, adminReq, UserIdentity{
		SubjectID:  "sub_admin_login",
		CognitoSub: "sub_admin_login",
		Email:      "admin@example.com",
	}); err != nil {
		t.Fatalf("set admin session: %v", err)
	}
	for _, cookie := range adminSession.Result().Cookies() {
		adminReq.AddCookie(cookie)
	}
	adminW := httptest.NewRecorder()
	mux.ServeHTTP(adminW, adminReq)
	if adminW.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", adminW.Code)
	}
	if got := adminW.Result().Header.Get("Location"); got != "/admin" {
		t.Fatalf("expected redirect to /admin, got %q", got)
	}
}

func TestAdminForgotPasswordFlow(t *testing.T) {
	a := testApp()
	a.auth = &mockAuthProvider{
		startForgotPassword: func(_ context.Context, email string) error {
			if email != "simon@example.com" {
				t.Fatalf("unexpected email %q", email)
			}
			return nil
		},
		confirmForgot: func(_ context.Context, email, code, newPassword string) error {
			if email != "simon@example.com" || code != "654321" || newPassword != "new-secret" {
				t.Fatalf("unexpected reset payload %q %q %q", email, code, newPassword)
			}
			return nil
		},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/login/back", a.handleAdminLoginBack)
	mux.HandleFunc("POST /auth/login/forgot-password", a.handleAdminForgotPasswordStart)
	mux.HandleFunc("POST /auth/login/forgot-password/confirm", a.handleAdminForgotPasswordConfirm)

	req := httptest.NewRequest("POST", "/auth/login/forgot-password", strings.NewReader("email=simon%40example.com"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	if got := w.Result().Header.Get("Location"); got != "/admin/login?notice=password-reset-code-sent" {
		t.Fatalf("unexpected location %q", got)
	}

	loginReq := httptest.NewRequest("GET", "/admin/login?notice=password-reset-code-sent", nil)
	for _, cookie := range w.Result().Cookies() {
		loginReq.AddCookie(cookie)
	}
	loginW := httptest.NewRecorder()
	a.handleAdminLogin(loginW, loginReq)
	if !strings.Contains(loginW.Body.String(), "Reset Password") {
		t.Fatal("reset step should render after reset start")
	}
	if strings.Contains(loginW.Body.String(), "Enter Password") {
		t.Fatal("reset step should hide the password choice step")
	}

	confirmReq := httptest.NewRequest("POST", "/auth/login/forgot-password/confirm", strings.NewReader("code=654321&new_password=new-secret&confirm_password=new-secret"))
	confirmReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range w.Result().Cookies() {
		confirmReq.AddCookie(cookie)
	}
	confirmW := httptest.NewRecorder()
	mux.ServeHTTP(confirmW, confirmReq)
	if confirmW.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", confirmW.Code)
	}
	if got := confirmW.Result().Header.Get("Location"); got != "/admin/login?notice=password-reset-complete" {
		t.Fatalf("unexpected location %q", got)
	}
}

func TestAdminRequiresAuth(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin", a.requireAdmin(a.handleAdminDashboard))

	req := httptest.NewRequest("GET", "/admin", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", w.Code)
	}
}

func TestAdminAllowsCognitoSessionWithAdminRole(t *testing.T) {
	a := testApp()
	assignRuntimeRole(t, a, UserRoleInput{SubjectID: "sub_admin", CognitoSub: "sub_admin", Role: "admin"})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin", a.requireAdmin(a.handleAdminDashboard))

	req := httptest.NewRequest("GET", "/admin", nil)
	w := httptest.NewRecorder()
	if err := a.setUserSession(w, req, UserIdentity{SubjectID: "sub_admin", CognitoSub: "sub_admin", Email: "admin@example.com"}); err != nil {
		t.Fatal(err)
	}
	for _, cookie := range w.Result().Cookies() {
		req.AddCookie(cookie)
	}

	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestShowIntakeOperatorCanAccessScopedWorkspaceAndEntryMoveButNotGlobalAdmin(t *testing.T) {
	a := testApp()

	showReq := httptest.NewRequest("GET", "/admin/shows/show_spring2025", nil)
	addRoleSession(t, a, showReq, UserRoleInput{
		SubjectID:  "sub_show_support",
		CognitoSub: "sub_show_support",
		ShowID:     "show_spring2025",
		Role:       "show_intake_operator",
	}, UserIdentity{
		SubjectID:  "sub_show_support",
		CognitoSub: "sub_show_support",
		Email:      "support@example.com",
	})

	showMux := http.NewServeMux()
	showMux.HandleFunc("GET /admin/shows/{showID}", a.requireCapabilityPage("shows.workspace.read", a.handleAdminShowDetail))
	showW := httptest.NewRecorder()
	showMux.ServeHTTP(showW, showReq)
	if showW.Code != http.StatusOK {
		t.Fatalf("expected scoped workspace access, got %d", showW.Code)
	}

	moveReq := httptest.NewRequest("POST", "/admin/entries/entry_01/move", strings.NewReader("class_id=class_02&reason=Judge+correction"))
	moveReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addRoleSession(t, a, moveReq, UserRoleInput{
		SubjectID:  "sub_show_support",
		CognitoSub: "sub_show_support",
		ShowID:     "show_spring2025",
		Role:       "show_intake_operator",
	}, UserIdentity{
		SubjectID:  "sub_show_support",
		CognitoSub: "sub_show_support",
		Email:      "support@example.com",
	})

	moveMux := http.NewServeMux()
	moveMux.HandleFunc("POST /admin/entries/{entryID}/move", a.requireCapabilityPage("entries.manage", a.handleAdminEntryMove))
	moveW := httptest.NewRecorder()
	moveMux.ServeHTTP(moveW, moveReq)
	if moveW.Code != http.StatusSeeOther {
		t.Fatalf("expected move redirect, got %d", moveW.Code)
	}
	if got := moveW.Result().Header.Get("Location"); got != "/admin/shows/show_spring2025" {
		t.Fatalf("unexpected move redirect %q", got)
	}
	movedEntry, ok := a.store.entryByID("entry_01")
	if !ok || movedEntry.ClassID != "class_02" {
		t.Fatal("entry should move within scoped show for intake operator")
	}

	globalReq := httptest.NewRequest("GET", "/admin/roles", nil)
	addRoleSession(t, a, globalReq, UserRoleInput{
		SubjectID:  "sub_show_support",
		CognitoSub: "sub_show_support",
		ShowID:     "show_spring2025",
		Role:       "show_intake_operator",
	}, UserIdentity{
		SubjectID:  "sub_show_support",
		CognitoSub: "sub_show_support",
		Email:      "support@example.com",
	})
	globalMux := http.NewServeMux()
	globalMux.HandleFunc("GET /admin/roles", a.requireAdmin(a.handleRoleManagement))
	globalW := httptest.NewRecorder()
	globalMux.ServeHTTP(globalW, globalReq)
	if globalW.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect from global admin page, got %d", globalW.Code)
	}
	if got := globalW.Result().Header.Get("Location"); got != "/account?notice=admin_required" {
		t.Fatalf("unexpected global admin redirect %q", got)
	}
}

func TestShowJudgeSupportCanAssignJudgesWithinScopedShow(t *testing.T) {
	a := testApp()
	req := httptest.NewRequest("POST", "/admin/shows/show_spring2025/judges", strings.NewReader("person_id=person_02"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addRoleSession(t, a, req, UserRoleInput{
		SubjectID:  "sub_judge_support",
		CognitoSub: "sub_judge_support",
		ShowID:     "show_spring2025",
		Role:       "show_judge_support",
	}, UserIdentity{
		SubjectID:  "sub_judge_support",
		CognitoSub: "sub_judge_support",
		Email:      "judge-support@example.com",
	})

	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/shows/{showID}/judges", a.requireCapabilityPage("judges.manage", a.handleAdminJudgeAssign))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected judge assign redirect, got %d", w.Code)
	}
	if got := w.Result().Header.Get("Location"); got != "/admin/shows/show_spring2025" {
		t.Fatalf("unexpected redirect %q", got)
	}
	judges := a.store.judgesByShow("show_spring2025")
	found := false
	for _, judge := range judges {
		if judge.PersonID == "person_02" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("show judge support should be able to assign judges within scoped show")
	}
}

func TestShowPeopleLookupProjectionReturnsScopedMembersAndGuests(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/shows/{id}/people.lookup", a.handleAPIShowPeopleLookup)

	req := httptest.NewRequest("GET", "/v1/projections/0007-Flowershow/shows/show_spring2025/people.lookup?q=guest", nil)
	addRoleSession(t, a, req, UserRoleInput{
		SubjectID:  "sub_lookup",
		CognitoSub: "sub_lookup",
		ShowID:     "show_spring2025",
		Role:       "show_intake_operator",
	}, UserIdentity{
		SubjectID:  "sub_lookup",
		CognitoSub: "sub_lookup",
		Email:      "lookup@example.com",
	})

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var payload []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal lookup payload: %v", err)
	}
	if len(payload) != 1 {
		t.Fatalf("expected one guest result, got %d", len(payload))
	}
	if got := payload[0]["AffiliationRole"]; got != "guest" {
		t.Fatalf("expected guest affiliation, got %#v", got)
	}
	if got := payload[0]["OrganizationID"]; got != "org_demo1" {
		t.Fatalf("expected host organization org_demo1, got %#v", got)
	}
	if got := payload[0]["Label"]; got != "Susan Park · guest · Metro Rose Society" {
		t.Fatalf("unexpected lookup label %#v", got)
	}
}

func TestCreatePersonWithOrganizationLinkAppearsInShowLookup(t *testing.T) {
	a := testApp()
	created, err := a.store.createPerson(PersonInput{
		FirstName:        "Nina",
		LastName:         "North",
		Email:            "nina@example.com",
		OrganizationID:   "org_demo1",
		OrganizationRole: "member",
	})
	if err != nil {
		t.Fatalf("create person: %v", err)
	}
	items := a.personLookupViewsForShow("show_spring2025", "nina")
	if len(items) != 1 {
		t.Fatalf("expected 1 lookup result, got %d", len(items))
	}
	if items[0].Person.ID != created.ID {
		t.Fatalf("expected created person %q, got %q", created.ID, items[0].Person.ID)
	}
	if items[0].AffiliationRole != "member" {
		t.Fatalf("expected member affiliation, got %q", items[0].AffiliationRole)
	}
}

func TestAdminShowDetailIncludesGovernanceAndScoringControls(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/shows/{showID}", a.requireAdmin(a.handleAdminShowDetail))

	req := httptest.NewRequest("GET", "/admin/shows/show_spring2025", nil)
	addAdminSession(t, a, req)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Schedule Governance") {
		t.Fatal("admin show missing schedule governance section")
	}
	if !strings.Contains(body, "Official Judging and Exhibiting Standards") {
		t.Fatal("admin show missing seeded standard")
	}
	if !strings.Contains(body, `name="score_crit_form"`) {
		t.Fatal("admin show missing per-criterion scoring inputs")
	}
	if !strings.Contains(body, "Assign Judge") {
		t.Fatal("admin show missing judge assignment controls")
	}
	if !strings.Contains(body, "Add Photo") {
		t.Fatal("admin show missing mobile photo add control")
	}
	if !strings.Contains(body, `accept="image/jpeg,image/png,image/webp"`) {
		t.Fatal("admin show missing constrained mobile photo accept types")
	}
}

func TestHTMXJudgeAssignReturnsInfoPanel(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/shows/{showID}/judges", a.requireAdmin(a.handleAdminJudgeAssign))

	req := httptest.NewRequest("POST", "/admin/shows/show_fall2025/judges", strings.NewReader("person_id=person_01"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	addAdminSession(t, a, req)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 HTMX fragment, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Judge assigned") && !strings.Contains(w.Body.String(), "assigned") {
		t.Fatal("expected refreshed info panel with assigned judge")
	}
}

func TestAPIShowsDirectory(t *testing.T) {
	a := testApp()
	req := httptest.NewRequest("GET", "/v1/projections/0007-Flowershow/shows", nil)
	w := httptest.NewRecorder()
	a.handleAPIShowsDirectory(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var shows []*Show
	json.Unmarshal(w.Body.Bytes(), &shows)
	if len(shows) == 0 {
		t.Fatal("expected seeded shows")
	}
}

func TestCommandEndpointsRequireAuth(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/shows.create", a.handleAPICommand)

	req := httptest.NewRequest("POST", "/v1/commands/0007-Flowershow/shows.create",
		strings.NewReader(`{"name":"Test"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	var payload struct {
		Error struct {
			Code      string `json:"code"`
			AuthMode  string `json:"auth_mode"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal unauthorized error: %v", err)
	}
	if payload.Error.Code != "unauthorized" {
		t.Fatalf("expected unauthorized code, got %q", payload.Error.Code)
	}
	if payload.Error.AuthMode != "anonymous" {
		t.Fatalf("expected anonymous auth mode, got %q", payload.Error.AuthMode)
	}
	if payload.Error.RequestID == "" {
		t.Fatal("expected request id in error payload")
	}
}

func TestCommandEndpointsAcceptServiceToken(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/shows.create", a.handleAPICommand)

	req := httptest.NewRequest("POST", "/v1/commands/0007-Flowershow/shows.create",
		strings.NewReader(`{"name":"Token Show","organization_id":"org_demo1","season":"2025"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		body, _ := io.ReadAll(w.Body)
		t.Fatalf("expected 201, got %d: %s", w.Code, body)
	}
}

func TestCommandEndpointsReturnUsefulStructuredErrorsForAuthenticatedCallers(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/shows.create", a.handleAPICommand)

	req := httptest.NewRequest("POST", "/v1/commands/0007-Flowershow/shows.create", strings.NewReader(`{"name":"bad"`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var payload struct {
		Error struct {
			Code        string `json:"code"`
			Hint        string `json:"hint"`
			ContractRef string `json:"contract_ref"`
			RequestID   string `json:"request_id"`
			AuthMode    string `json:"auth_mode"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal authenticated error: %v", err)
	}
	if payload.Error.Code != "invalid_json" {
		t.Fatalf("expected invalid_json code, got %q", payload.Error.Code)
	}
	if payload.Error.AuthMode != "service_token" {
		t.Fatalf("expected service_token auth mode, got %q", payload.Error.AuthMode)
	}
	if payload.Error.Hint == "" {
		t.Fatal("expected recovery hint for authenticated caller")
	}
	if payload.Error.ContractRef != "/v1/contracts/0007-Flowershow/a-firstbloom" {
		t.Fatalf("unexpected contract ref %q", payload.Error.ContractRef)
	}
	if payload.Error.RequestID == "" {
		t.Fatal("expected request id in authenticated error")
	}
}

func TestProjectionsReturnJSON(t *testing.T) {
	a := testApp()

	tests := []struct {
		name    string
		handler http.HandlerFunc
		path    string
	}{
		{"shows", a.handleAPIShowsDirectory, "/v1/projections/0007-Flowershow/shows"},
		{"taxonomy", a.handleAPITaxonomy, "/v1/projections/0007-Flowershow/taxonomy"},
		{"leaderboard", a.handleAPILeaderboard, "/v1/projections/0007-Flowershow/leaderboard?org_id=org_demo1&season=2025"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			tt.handler(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", w.Code)
			}
			ct := w.Header().Get("Content-Type")
			if !strings.HasPrefix(ct, "application/json") {
				t.Fatalf("expected JSON content-type, got %s", ct)
			}
		})
	}
}

func TestLedgerProjection(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/ledger/{objectID}", a.handleAPILedger)

	// Without auth
	req := httptest.NewRequest("GET", "/v1/projections/0007-Flowershow/ledger/show_spring2025", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	// With service token
	req = httptest.NewRequest("GET", "/v1/projections/0007-Flowershow/ledger/show_spring2025", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestContractsEndpointsReturnLocalContract(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/contracts", a.handleContractsList)
	mux.HandleFunc("GET /v1/contracts/{seed_id}/{realization_id}", a.handleContractDetail)

	req := httptest.NewRequest("GET", "/v1/contracts", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"self":"/v1/contracts/0007-Flowershow/a-firstbloom"`) {
		t.Fatal("contract list missing self link")
	}

	req = httptest.NewRequest("GET", "/v1/contracts/0007-Flowershow/a-firstbloom", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"seed_agent_principles"`) {
		t.Fatal("contract detail missing agent principles links")
	}
	if !strings.Contains(w.Body.String(), `"ui_surfaces"`) {
		t.Fatal("contract detail missing ui surface declarations")
	}
}

func TestInteractionContractPathPrefersWorkingDirectoryFallback(t *testing.T) {
	root := t.TempDir()
	workingDir := filepath.Join(root, "artifacts", "flowershow-app")
	if err := os.MkdirAll(workingDir, 0o755); err != nil {
		t.Fatalf("mkdir working dir: %v", err)
	}

	want := filepath.Join(root, "interaction_contract.yaml")
	if err := os.WriteFile(want, []byte("seed_id: 0007-Flowershow\nrealization_id: a-firstbloom\nsurface_kind: app\nsummary: test\n"), 0o644); err != nil {
		t.Fatalf("write contract fixture: %v", err)
	}

	t.Chdir(workingDir)
	t.Setenv("AS_INTERACTION_CONTRACT_PATH", "")

	got, err := interactionContractPath()
	if err != nil {
		t.Fatalf("interactionContractPath returned error: %v", err)
	}
	if got != want {
		t.Fatalf("interactionContractPath = %q, want %q", got, want)
	}
}

func TestShowWorkspaceProjectionAcceptsServiceToken(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/shows/{id}/workspace", a.handleAPIShowWorkspace)

	req := httptest.NewRequest("GET", "/v1/projections/0007-Flowershow/shows/show_spring2025/workspace", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"Title":"Admin: Spring Rose Show 2025"`) {
		t.Fatal("workspace projection missing admin workspace payload")
	}
}

func TestScheduleUpsertCommandCreatesSchedule(t *testing.T) {
	a := testApp()
	show, err := a.store.createShow(ShowInput{
		OrganizationID: "org_demo1",
		Name:           "Winter Daffodil Show",
		Season:         "2026",
	})
	if err != nil {
		t.Fatalf("create show: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/schedules.upsert", a.handleAPICommand)

	req := httptest.NewRequest("POST", "/v1/commands/0007-Flowershow/schedules.upsert",
		strings.NewReader(`{"show_id":"`+show.ID+`","notes":"OJES governs unless the local schedule is narrower."}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	schedule, ok := a.store.scheduleByShowID(show.ID)
	if !ok {
		t.Fatal("expected schedule to be created")
	}
	if schedule.Notes == "" {
		t.Fatal("expected schedule notes to be stored")
	}
}

func TestCommandEndpointsAcceptRuntimeContextEnvelopeWithoutPersistence(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/shows.create", a.handleAPICommand)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/shows/{id}", a.handleAPIShowDetail)

	req := jsonRequest("POST", "/v1/commands/0007-Flowershow/shows.create", `{
		"input":{
			"organization_id":"org_demo1",
			"name":"Runtime Envelope Show",
			"season":"2026"
		},
		"runtime_context":{
			"assistant_goal":"author the initial governed show shell",
			"source_excerpt":"Treat this as prompt-only context."
		}
	}`)
	addServiceToken(req)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var show Show
	if err := json.Unmarshal(w.Body.Bytes(), &show); err != nil {
		t.Fatalf("unmarshal show: %v", err)
	}

	req = httptest.NewRequest("GET", "/v1/projections/0007-Flowershow/shows/"+show.ID, nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, "assistant_goal") || strings.Contains(body, "runtime_context") {
		t.Fatal("runtime-only context must not be persisted into show projections")
	}
}

func TestPrivateByIDProjectionsRespectAuthModes(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/entries/{id}", a.handleAPIEntryDetail)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/classes/{id}", a.handleAPIClassDetail)

	req := httptest.NewRequest("GET", "/v1/projections/0007-Flowershow/entries/entry_01", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), `"first_name"`) {
		t.Fatal("anonymous entry projection should not expose private identity fields")
	}
	if !strings.Contains(w.Body.String(), `"initials":"MC"`) {
		t.Fatal("anonymous entry projection should include public initials")
	}

	req = httptest.NewRequest("GET", "/v1/projections/0007-Flowershow/classes/class_01", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"effective_rules"`) {
		t.Fatal("class detail should include effective rules")
	}
	if !strings.Contains(w.Body.String(), `"show":{"id":"show_spring2025"`) {
		t.Fatal("class detail should include parent show context")
	}

	if err := a.store.setEntrySuppressed("entry_01", true); err != nil {
		t.Fatal(err)
	}

	req = httptest.NewRequest("GET", "/v1/projections/0007-Flowershow/entries/entry_01", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for suppressed anonymous entry, got %d", w.Code)
	}

	req = httptest.NewRequest("GET", "/v1/projections/0007-Flowershow/entries/entry_01", nil)
	addServiceToken(req)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for service-token entry detail, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"first_name":"Margaret"`) {
		t.Fatal("service-token entry projection should expose private identity fields")
	}
}

func TestSessionAuthCanExecuteParityCommandAndWorkspaceProjection(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/shows.create", a.handleAPICommand)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/shows/{id}/workspace", a.handleAPIShowWorkspace)

	req := jsonRequest("POST", "/v1/commands/0007-Flowershow/shows.create", `{
		"organization_id":"org_demo1",
		"name":"Session Auth Show",
		"season":"2026"
	}`)
	addAdminSession(t, a, req)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var show Show
	if err := json.Unmarshal(w.Body.Bytes(), &show); err != nil {
		t.Fatalf("unmarshal show: %v", err)
	}

	req = httptest.NewRequest("GET", "/v1/projections/0007-Flowershow/shows/"+show.ID+"/workspace", nil)
	addAdminSession(t, a, req)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"Title":"Admin: Session Auth Show"`) {
		t.Fatal("session-authenticated workspace projection should return admin payload")
	}
}

func TestServiceTokenParityCommandChain(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/shows.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/schedules.upsert", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/divisions.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/sections.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/classes.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/entries.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/judges.assign", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/entries.set_visibility", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/media.attach", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/media.delete", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/roles.assign", a.handleAPICommand)
	mux.HandleFunc("GET /v1/projections/0007-Flowershow/entries/{id}", a.handleAPIEntryDetail)

	req := jsonRequest("POST", "/v1/commands/0007-Flowershow/shows.create", `{
		"organization_id":"org_demo1",
		"name":"Parity Chain Show",
		"season":"2026"
	}`)
	addServiceToken(req)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create show: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var show Show
	if err := json.Unmarshal(w.Body.Bytes(), &show); err != nil {
		t.Fatalf("unmarshal show: %v", err)
	}

	req = jsonRequest("POST", "/v1/commands/0007-Flowershow/schedules.upsert", `{
		"show_id":"`+show.ID+`",
		"notes":"Parity chain schedule"
	}`)
	addServiceToken(req)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("schedule upsert: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var schedule ShowSchedule
	if err := json.Unmarshal(w.Body.Bytes(), &schedule); err != nil {
		t.Fatalf("unmarshal schedule: %v", err)
	}

	req = jsonRequest("POST", "/v1/commands/0007-Flowershow/divisions.create", `{
		"show_schedule_id":"`+schedule.ID+`",
		"code":"I",
		"title":"Parity Division",
		"domain":"horticulture",
		"sort_order":1
	}`)
	addServiceToken(req)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("division create: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var division Division
	if err := json.Unmarshal(w.Body.Bytes(), &division); err != nil {
		t.Fatalf("unmarshal division: %v", err)
	}

	req = jsonRequest("POST", "/v1/commands/0007-Flowershow/sections.create", `{
		"division_id":"`+division.ID+`",
		"code":"A",
		"title":"Parity Section",
		"sort_order":1
	}`)
	addServiceToken(req)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("section create: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var section Section
	if err := json.Unmarshal(w.Body.Bytes(), &section); err != nil {
		t.Fatalf("unmarshal section: %v", err)
	}

	req = jsonRequest("POST", "/v1/commands/0007-Flowershow/classes.create", `{
		"section_id":"`+section.ID+`",
		"class_number":"12",
		"title":"Parity Bloom",
		"domain":"horticulture",
		"specimen_count":1
	}`)
	addServiceToken(req)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("class create: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var cls ShowClass
	if err := json.Unmarshal(w.Body.Bytes(), &cls); err != nil {
		t.Fatalf("unmarshal class: %v", err)
	}

	req = jsonRequest("POST", "/v1/commands/0007-Flowershow/judges.assign", `{
		"show_id":"`+show.ID+`",
		"person_id":"person_02"
	}`)
	addServiceToken(req)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("judge assign: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	req = jsonRequest("POST", "/v1/commands/0007-Flowershow/entries.create", `{
		"show_id":"`+show.ID+`",
		"class_id":"`+cls.ID+`",
		"person_id":"person_01",
		"name":"Parity Entry"
	}`)
	addServiceToken(req)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("entry create: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var entry Entry
	if err := json.Unmarshal(w.Body.Bytes(), &entry); err != nil {
		t.Fatalf("unmarshal entry: %v", err)
	}

	req = jsonRequest("POST", "/v1/commands/0007-Flowershow/media.attach", `{
		"entry_id":"`+entry.ID+`",
		"media_type":"photo",
		"url":"https://example.com/entry.jpg",
		"file_name":"entry.jpg"
	}`)
	addServiceToken(req)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("media attach: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var media Media
	if err := json.Unmarshal(w.Body.Bytes(), &media); err != nil {
		t.Fatalf("unmarshal media: %v", err)
	}

	req = jsonRequest("POST", "/v1/commands/0007-Flowershow/media.delete", `{
		"media_id":"`+media.ID+`"
	}`)
	addServiceToken(req)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("media delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	req = jsonRequest("POST", "/v1/commands/0007-Flowershow/roles.assign", `{
		"cognito_sub":"sub_remote_agent",
		"role":"admin",
		"show_id":"`+show.ID+`"
	}`)
	addServiceToken(req)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("role assign: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	roles, err := a.authority.RoleAssignmentsForUser(context.Background(), UserIdentity{SubjectID: "sub_remote_agent", CognitoSub: "sub_remote_agent"})
	if err != nil {
		t.Fatalf("list runtime roles: %v", err)
	}
	if len(roles) != 1 {
		t.Fatal("expected assigned role to be stored")
	}

	req = jsonRequest("POST", "/v1/commands/0007-Flowershow/entries.set_visibility", `{
		"id":"`+entry.ID+`",
		"suppressed":true
	}`)
	addServiceToken(req)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("entry visibility: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest("GET", "/v1/projections/0007-Flowershow/entries/"+entry.ID, nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for suppressed anonymous entry, got %d", w.Code)
	}

	req = httptest.NewRequest("GET", "/v1/projections/0007-Flowershow/entries/"+entry.ID, nil)
	addServiceToken(req)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for suppressed service-token entry, got %d", w.Code)
	}
}

func TestOnsiteWorkflowCommandsSupportReclassificationAndCredits(t *testing.T) {
	a := testApp()

	show, err := a.store.createShow(ShowInput{
		OrganizationID: "org_demo1",
		Name:           "Onsite Ops Show",
		Season:         "2026",
	})
	if err != nil {
		t.Fatalf("create show: %v", err)
	}
	schedule, err := a.store.createSchedule(ShowSchedule{ShowID: show.ID})
	if err != nil {
		t.Fatalf("create schedule: %v", err)
	}
	division, err := a.store.createDivision(DivisionInput{
		ShowScheduleID: schedule.ID,
		Code:           "I",
		Title:          "Ops Division",
		Domain:         "horticulture",
		SortOrder:      1,
	})
	if err != nil {
		t.Fatalf("create division: %v", err)
	}
	section, err := a.store.createSection(SectionInput{
		DivisionID: division.ID,
		Code:       "A",
		Title:      "Ops Section",
		SortOrder:  1,
	})
	if err != nil {
		t.Fatalf("create section: %v", err)
	}
	classOne, err := a.store.createClass(ShowClassInput{
		SectionID:     section.ID,
		ClassNumber:   "1",
		SortOrder:     1,
		Title:         "Class One",
		Domain:        "horticulture",
		SpecimenCount: 1,
	})
	if err != nil {
		t.Fatalf("create class one: %v", err)
	}
	classTwo, err := a.store.createClass(ShowClassInput{
		SectionID:     section.ID,
		ClassNumber:   "2",
		SortOrder:     2,
		Title:         "Class Two",
		Domain:        "horticulture",
		SpecimenCount: 1,
	})
	if err != nil {
		t.Fatalf("create class two: %v", err)
	}
	entry, err := a.store.createEntry(EntryInput{
		ShowID:   show.ID,
		ClassID:  classOne.ID,
		PersonID: "person_01",
		Name:     "Movable Entry",
	})
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	deleteMe, err := a.store.createEntry(EntryInput{
		ShowID:   show.ID,
		ClassID:  classOne.ID,
		PersonID: "person_02",
		Name:     "Delete Me",
	})
	if err != nil {
		t.Fatalf("create delete entry: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/classes.update", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/classes.reorder", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/entries.move", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/entries.reassign_entrant", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/entries.delete", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/show_credits.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/show_credits.delete", a.handleAPICommand)

	req := jsonRequest("POST", "/v1/commands/0007-Flowershow/classes.update", `{
		"id":"`+classOne.ID+`",
		"section_id":"`+section.ID+`",
		"class_number":"1A",
		"sort_order":3,
		"title":"Class One Updated",
		"domain":"horticulture",
		"description":"Updated wording",
		"specimen_count":2
	}`)
	addServiceToken(req)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("class update: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	req = jsonRequest("POST", "/v1/commands/0007-Flowershow/classes.reorder", `{
		"class_id":"`+classTwo.ID+`",
		"sort_order":1
	}`)
	addServiceToken(req)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("class reorder: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	req = jsonRequest("POST", "/v1/commands/0007-Flowershow/entries.reassign_entrant", `{
		"id":"`+entry.ID+`",
		"person_id":"person_03"
	}`)
	addServiceToken(req)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("entry reassign: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	req = jsonRequest("POST", "/v1/commands/0007-Flowershow/entries.move", `{
		"id":"`+entry.ID+`",
		"class_id":"`+classTwo.ID+`",
		"reason":"Judge corrected the class"
	}`)
	addServiceToken(req)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("entry move: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	req = jsonRequest("POST", "/v1/commands/0007-Flowershow/show_credits.create", `{
		"show_id":"`+show.ID+`",
		"credit_label":"Scribe",
		"person_id":"person_02",
		"sort_order":1
	}`)
	addServiceToken(req)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("show credit create: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var credit ShowCredit
	if err := json.Unmarshal(w.Body.Bytes(), &credit); err != nil {
		t.Fatalf("unmarshal credit: %v", err)
	}

	req = jsonRequest("POST", "/v1/commands/0007-Flowershow/show_credits.delete", `{
		"id":"`+credit.ID+`"
	}`)
	addServiceToken(req)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("show credit delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	req = jsonRequest("POST", "/v1/commands/0007-Flowershow/entries.delete", `{
		"id":"`+deleteMe.ID+`"
	}`)
	addServiceToken(req)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("entry delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	updatedEntry, ok := a.store.entryByID(entry.ID)
	if !ok {
		t.Fatal("moved entry should still exist")
	}
	if updatedEntry.ClassID != classTwo.ID {
		t.Fatalf("expected entry class to be %s, got %s", classTwo.ID, updatedEntry.ClassID)
	}
	if updatedEntry.PersonID != "person_03" {
		t.Fatalf("expected entry person to be person_03, got %s", updatedEntry.PersonID)
	}
	if _, ok := a.store.entryByID(deleteMe.ID); ok {
		t.Fatal("deleted entry should no longer exist")
	}
	if credits := a.store.showCreditsByShow(show.ID); len(credits) != 0 {
		t.Fatalf("expected deleted show credit to be removed, got %d remaining", len(credits))
	}
	classes := a.store.classesBySection(section.ID)
	if len(classes) != 2 {
		t.Fatalf("expected 2 classes, got %d", len(classes))
	}
	if classes[0].ID != classTwo.ID {
		t.Fatalf("expected reordered class %s first, got %s", classTwo.ID, classes[0].ID)
	}
	if classes[1].ID != classOne.ID {
		t.Fatalf("expected updated class %s second, got %s", classOne.ID, classes[1].ID)
	}
}

func TestComputePlacementsCommandUpdatesEntryRankings(t *testing.T) {
	a := testApp()

	show, err := a.store.createShow(ShowInput{
		OrganizationID: "org_demo1",
		Name:           "Compute Placements Show",
		Season:         "2026",
	})
	if err != nil {
		t.Fatalf("create show: %v", err)
	}
	schedule, err := a.store.createSchedule(ShowSchedule{ShowID: show.ID})
	if err != nil {
		t.Fatalf("create schedule: %v", err)
	}
	division, err := a.store.createDivision(DivisionInput{
		ShowScheduleID: schedule.ID,
		Code:           "I",
		Title:          "Compute Division",
		Domain:         "horticulture",
		SortOrder:      1,
	})
	if err != nil {
		t.Fatalf("create division: %v", err)
	}
	section, err := a.store.createSection(SectionInput{
		DivisionID: division.ID,
		Code:       "A",
		Title:      "Compute Section",
		SortOrder:  1,
	})
	if err != nil {
		t.Fatalf("create section: %v", err)
	}
	cls, err := a.store.createClass(ShowClassInput{
		SectionID:     section.ID,
		ClassNumber:   "88",
		Title:         "Compute Class",
		Domain:        "horticulture",
		SpecimenCount: 1,
	})
	if err != nil {
		t.Fatalf("create class: %v", err)
	}
	if _, err := a.store.assignJudgeToShow(show.ID, "person_02"); err != nil {
		t.Fatalf("assign judge: %v", err)
	}
	rubric, err := a.store.createRubric(JudgingRubric{
		ShowID: show.ID,
		Domain: "horticulture",
		Title:  "Compute Rubric",
	})
	if err != nil {
		t.Fatalf("create rubric: %v", err)
	}
	criterion, err := a.store.createCriterion(JudgingCriterion{
		JudgingRubricID: rubric.ID,
		Name:            "Condition",
		MaxPoints:       100,
		SortOrder:       1,
	})
	if err != nil {
		t.Fatalf("create criterion: %v", err)
	}
	entryHigh, err := a.store.createEntry(EntryInput{
		ShowID:   show.ID,
		ClassID:  cls.ID,
		PersonID: "person_01",
		Name:     "High Score Entry",
	})
	if err != nil {
		t.Fatalf("create high entry: %v", err)
	}
	entryLow, err := a.store.createEntry(EntryInput{
		ShowID:   show.ID,
		ClassID:  cls.ID,
		PersonID: "person_03",
		Name:     "Low Score Entry",
	})
	if err != nil {
		t.Fatalf("create low entry: %v", err)
	}
	if _, err := a.store.submitScorecard(EntryScorecard{
		EntryID:  entryHigh.ID,
		JudgeID:  "person_02",
		RubricID: rubric.ID,
	}, []EntryCriterionScore{{CriterionID: criterion.ID, Score: 96}}); err != nil {
		t.Fatalf("submit high scorecard: %v", err)
	}
	if _, err := a.store.submitScorecard(EntryScorecard{
		EntryID:  entryLow.ID,
		JudgeID:  "person_02",
		RubricID: rubric.ID,
	}, []EntryCriterionScore{{CriterionID: criterion.ID, Score: 81}}); err != nil {
		t.Fatalf("submit low scorecard: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/classes.compute_placements", a.handleAPICommand)
	req := jsonRequest("POST", "/v1/commands/0007-Flowershow/classes.compute_placements", `{
		"class_id":"`+cls.ID+`"
	}`)
	addServiceToken(req)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	updatedHigh, _ := a.store.entryByID(entryHigh.ID)
	updatedLow, _ := a.store.entryByID(entryLow.ID)
	if updatedHigh.Placement != 1 {
		t.Fatalf("expected high score entry to be 1st, got %d", updatedHigh.Placement)
	}
	if updatedLow.Placement != 2 {
		t.Fatalf("expected low score entry to be 2nd, got %d", updatedLow.Placement)
	}
}

func TestFullAPIFlow(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/persons.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/entries.create", a.handleAPICommand)
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/entries.set_placement", a.handleAPICommand)

	auth := "Bearer test-token"

	// Create person
	req := httptest.NewRequest("POST", "/v1/commands/0007-Flowershow/persons.create",
		strings.NewReader(`{"first_name":"Alice","last_name":"Brown"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", auth)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create person: expected 201, got %d", w.Code)
	}
	var person Person
	json.Unmarshal(w.Body.Bytes(), &person)

	// Create entry
	body := `{"show_id":"show_spring2025","class_id":"class_01","person_id":"` + person.ID + `","name":"Test Rose"}`
	req = httptest.NewRequest("POST", "/v1/commands/0007-Flowershow/entries.create",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", auth)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create entry: expected 201, got %d", w.Code)
	}
	var entry Entry
	json.Unmarshal(w.Body.Bytes(), &entry)

	// Set placement
	body = `{"id":"` + entry.ID + `","placement":1,"points":6}`
	req = httptest.NewRequest("POST", "/v1/commands/0007-Flowershow/entries.set_placement",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", auth)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("set placement: expected 200, got %d", w.Code)
	}
}

func TestIngestionImportAPI(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/commands/0007-Flowershow/ingestions.import", a.handleAPICommand)

	payload := `{
		"source_document":{"title":"Imported Rulebook","document_type":"rulebook","show_id":"show_spring2025"},
		"citations":[{"target_type":"show_class","target_id":"class_01","page_from":"2","quoted_text":"Imported citation"}]
	}`
	req := httptest.NewRequest("POST", "/v1/commands/0007-Flowershow/ingestions.import", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
	if len(a.store.allSourceDocuments()) < 2 {
		t.Fatal("expected imported source document to be stored")
	}
	if len(a.store.citationsByTarget("show_class", "class_01")) < 2 {
		t.Fatal("expected imported citation to be stored")
	}
}

func TestStoreMemoryBasics(t *testing.T) {
	s := newMemoryStore()

	// Shows exist from seed
	shows := s.allShows()
	if len(shows) == 0 {
		t.Fatal("expected seeded shows")
	}

	// Persons exist from seed
	persons := s.allPersons()
	if len(persons) != 3 {
		t.Fatalf("expected 3 persons, got %d", len(persons))
	}

	// Create new show
	show, err := s.createShow(ShowInput{
		Name:           "Test Show",
		OrganizationID: "org_demo1",
		Season:         "2025",
	})
	if err != nil {
		t.Fatal(err)
	}
	if show.Slug != "test-show" {
		t.Fatalf("expected slug test-show, got %s", show.Slug)
	}

	// Create entry
	entry, err := s.createEntry(EntryInput{
		ShowID:   show.ID,
		ClassID:  "class_01",
		PersonID: "person_01",
		Name:     "Test Entry",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Set placement
	if err := s.setPlacement(entry.ID, 1, 6); err != nil {
		t.Fatal(err)
	}
	e, ok := s.entryByID(entry.ID)
	if !ok || e.Placement != 1 || e.Points != 6 {
		t.Fatal("placement not set correctly")
	}

	// Ledger
	claims, err := s.ledgerByObjectID(show.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(claims) == 0 {
		t.Fatal("expected ledger claims for show")
	}

	// Leaderboard
	lb := s.leaderboard("org_demo1", "2025")
	if len(lb) == 0 {
		t.Fatal("expected leaderboard entries")
	}

	// Award compute
	results, err := s.computeAward("award_hp")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected award results")
	}
}

func TestMediaUploadAndRender(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/entries/{entryID}/media", a.requireAdmin(a.handleMediaUpload))
	mux.HandleFunc("GET /entries/{entryID}", a.handleEntryDetail)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("media", "rose.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("fake image bytes")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/admin/entries/entry_01/media", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	addAdminSession(t, a, req)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}

	media := a.store.mediaByEntry("entry_01")
	if len(media) != 1 {
		t.Fatalf("expected 1 media record, got %d", len(media))
	}
	if _, err := os.Stat(media[0].StorageKey); err != nil {
		t.Fatalf("expected uploaded media on disk: %v", err)
	}

	req = httptest.NewRequest("GET", "/entries/entry_01", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "/media/"+media[0].ID) {
		t.Fatal("entry detail missing uploaded media")
	}
}

func TestMediaUploadRejectsHEIC(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/entries/{entryID}/media", a.requireAdmin(a.handleMediaUpload))

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="media"; filename="rose.heic"`)
	header.Set("Content-Type", "image/heic")
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("fake heic bytes")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/admin/entries/entry_01/media", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	addAdminSession(t, a, req)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "HEIC/HEIF is not supported") {
		t.Fatal("expected HEIC rejection message")
	}
}

func TestScorecardRequiresAssignedJudge(t *testing.T) {
	a := testApp()
	person, err := a.store.createPerson(PersonInput{FirstName: "Una", LastName: "Signed"})
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/scorecards", a.requireAdmin(a.handleAdminScorecardSubmit))

	body := strings.NewReader("entry_id=entry_01&judge_id=" + person.ID + "&rubric_id=rubric_hort&score_crit_form=10")
	req := httptest.NewRequest("POST", "/admin/scorecards", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAdminSession(t, a, req)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unassigned judge, got %d", w.Code)
	}

	body = strings.NewReader("entry_id=entry_01&judge_id=person_03&rubric_id=rubric_hort&score_crit_form=20&score_crit_color=20")
	req = httptest.NewRequest("POST", "/admin/scorecards", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAdminSession(t, a, req)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 for assigned judge, got %d", w.Code)
	}
	if len(a.store.scorecardsByEntry("entry_01")) == 0 {
		t.Fatal("expected scorecard to be recorded for assigned judge")
	}
}

func TestEffectiveRulesForClass(t *testing.T) {
	s := newMemoryStore()

	// Create standard and edition
	doc, _ := s.createStandardDocument(StandardDocument{Name: "OJES", DomainScope: "horticulture"})
	ed, _ := s.createStandardEdition(StandardEdition{
		StandardDocumentID: doc.ID,
		EditionLabel:       "2019",
		PublicationYear:    2019,
		Status:             "current",
	})

	// Create rule
	rule, _ := s.createStandardRule(StandardRule{
		StandardEditionID: ed.ID,
		Domain:            "horticulture",
		RuleType:          "presentation",
		SubjectLabel:      "Hybrid Tea Display",
		Body:              "Must be displayed in a clear container",
	})

	// Create override
	s.createClassRuleOverride(ClassRuleOverride{
		ShowClassID:        "class_01",
		BaseStandardRuleID: rule.ID,
		OverrideType:       "narrow",
		Body:               "Container must be exactly 6 inches",
		Rationale:          "Local venue constraint",
	})

	// Create local-only override
	s.createClassRuleOverride(ClassRuleOverride{
		ShowClassID:  "class_01",
		OverrideType: "local_only",
		Body:         "Must include exhibitor tag",
	})

	effective := s.effectiveRulesForClass("class_01", ed.ID)
	if len(effective) != 2 {
		t.Fatalf("expected 2 effective rules, got %d", len(effective))
	}

	// Check we have both override and local_only sources
	sources := map[string]bool{}
	for _, r := range effective {
		sources[r.Source] = true
	}
	if !sources["override"] {
		t.Fatal("missing override rule")
	}
	if !sources["local_only"] {
		t.Fatal("missing local_only rule")
	}
}

func TestScorecardAndPlacement(t *testing.T) {
	s := newMemoryStore()

	// Create rubric with criteria
	rubric, _ := s.createRubric(JudgingRubric{Domain: "horticulture", Title: "Test Rubric"})
	crit1, _ := s.createCriterion(JudgingCriterion{JudgingRubricID: rubric.ID, Name: "Form", MaxPoints: 50, SortOrder: 1})
	crit2, _ := s.createCriterion(JudgingCriterion{JudgingRubricID: rubric.ID, Name: "Color", MaxPoints: 50, SortOrder: 2})

	// Submit scorecards for entry_01 and entry_02
	s.submitScorecard(EntryScorecard{
		EntryID:  "entry_01",
		JudgeID:  "person_03",
		RubricID: rubric.ID,
	}, []EntryCriterionScore{
		{CriterionID: crit1.ID, Score: 45},
		{CriterionID: crit2.ID, Score: 40},
	})

	s.submitScorecard(EntryScorecard{
		EntryID:  "entry_02",
		JudgeID:  "person_03",
		RubricID: rubric.ID,
	}, []EntryCriterionScore{
		{CriterionID: crit1.ID, Score: 35},
		{CriterionID: crit2.ID, Score: 30},
	})

	// Verify scorecards
	scs := s.scorecardsByEntry("entry_01")
	if len(scs) == 0 {
		t.Fatal("expected scorecard")
	}
	if scs[0].TotalScore != 85 {
		t.Fatalf("expected total 85, got %f", scs[0].TotalScore)
	}

	// Compute placements
	s.computePlacementsFromScores("class_01")

	e1, _ := s.entryByID("entry_01")
	e2, _ := s.entryByID("entry_02")
	if e1.Placement != 1 {
		t.Fatalf("expected entry_01 1st, got %d", e1.Placement)
	}
	if e2.Placement != 2 {
		t.Fatalf("expected entry_02 2nd, got %d", e2.Placement)
	}
}
