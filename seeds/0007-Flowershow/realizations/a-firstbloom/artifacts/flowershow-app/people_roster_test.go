package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sesv2"
)

// TestPeopleRosterCTAsRenderInIntakePanel — the admin show detail page
// should now render all three roster CTAs above the intake grid.
func TestPeopleRosterCTAsRenderInIntakePanel(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/shows/{showID}", a.requireCapabilityPage("shows.workspace.read", a.handleAdminShowDetail))

	req := httptest.NewRequest("GET", "/admin/shows/show_spring2025", nil)
	addAdminSession(t, a, req)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, marker := range []string{
		`data-people-modal-open="judge"`,
		`data-people-modal-open="show_admin"`,
		`data-people-modal-open="member"`,
		"Add Judge",
		"Add Show Helper Admin",
		"Add Member / Entrant",
		"intake-people-roster",
	} {
		if !strings.Contains(body, marker) {
			t.Fatalf("expected admin show page to contain %q", marker)
		}
	}
}

// TestPeopleSearchEndpointReturnsMatches — substring match on first/last
// name and email is case-insensitive and capped at 12 results.
func TestPeopleSearchEndpointReturnsMatches(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/shows/{showID}/people/search", a.requireCapabilityPage("entries.manage", a.handlePeopleRosterSearch))

	// seed two persons (in addition to the demo persons: Margaret Chen,
	// Robert Williams, Susan Park).
	if _, err := a.store.createPerson(PersonInput{FirstName: "Greta", LastName: "Lindstrom", Email: "greta@example.com"}); err != nil {
		t.Fatalf("seed greta: %v", err)
	}
	if _, err := a.store.createPerson(PersonInput{FirstName: "Marcus", LastName: "Greene"}); err != nil {
		t.Fatalf("seed marcus: %v", err)
	}

	// match "gre" — should hit Greta + Marcus Greene
	req := httptest.NewRequest("GET", "/admin/shows/show_spring2025/people/search?q=gre", nil)
	addAdminSession(t, a, req)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Results []peopleRosterSearchResult `json:"results"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, w.Body.String())
	}
	got := map[string]bool{}
	for _, r := range resp.Results {
		got[r.FirstName+" "+r.LastName] = true
	}
	if !got["Greta Lindstrom"] {
		t.Fatalf("expected Greta Lindstrom in results, got %+v", resp.Results)
	}
	if !got["Marcus Greene"] {
		t.Fatalf("expected Marcus Greene in results, got %+v", resp.Results)
	}
}

// TestPeopleSearchEmptyQueryReturnsEmpty — never dump the whole table.
func TestPeopleSearchEmptyQueryReturnsEmpty(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/shows/{showID}/people/search", a.requireCapabilityPage("entries.manage", a.handlePeopleRosterSearch))

	req := httptest.NewRequest("GET", "/admin/shows/show_spring2025/people/search?q=", nil)
	addAdminSession(t, a, req)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Results []peopleRosterSearchResult `json:"results"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, w.Body.String())
	}
	if len(resp.Results) != 0 {
		t.Fatalf("expected empty results for blank query, got %d", len(resp.Results))
	}
}

// TestAddJudgeFormCreatesPersonAndAssignment — creates a fresh person with
// IsJudge=true and the right JudgingStartedYear, and assigns them to the show.
func TestAddJudgeFormCreatesPersonAndAssignment(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/shows/{showID}/people/judge", a.requireCapabilityPage("entries.manage", a.handleAdminAddJudgeForm))

	beforeJudges := len(a.store.judgesByShow("show_spring2025"))
	beforePersons := len(a.store.allPersons())

	body := "first_name=Imelda&last_name=Tester&email=imelda%40example.com&qualifications=ARS+Master&years_in_flower_shows=12"
	req := httptest.NewRequest("POST", "/admin/shows/show_spring2025/people/judge", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAdminSession(t, a, req)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d body=%s", w.Code, w.Body.String())
	}

	persons := a.store.allPersons()
	if len(persons) != beforePersons+1 {
		t.Fatalf("expected one new person, got %d (before %d)", len(persons), beforePersons)
	}
	var imelda *Person
	for _, p := range persons {
		if p.FirstName == "Imelda" && p.LastName == "Tester" {
			imelda = p
			break
		}
	}
	if imelda == nil {
		t.Fatal("created person not found")
	}
	if !imelda.IsJudge {
		t.Fatal("expected IsJudge=true on new judge")
	}
	if imelda.Qualifications != "ARS Master" {
		t.Fatalf("expected qualifications captured, got %q", imelda.Qualifications)
	}
	wantYear := time.Now().UTC().Year() - 12
	if imelda.JudgingStartedYear != wantYear {
		t.Fatalf("expected JudgingStartedYear=%d (currentYear-12), got %d", wantYear, imelda.JudgingStartedYear)
	}

	afterJudges := a.store.judgesByShow("show_spring2025")
	if len(afterJudges) != beforeJudges+1 {
		t.Fatalf("expected one new judge assignment, got %d (before %d)", len(afterJudges), beforeJudges)
	}
	var assigned *ShowJudgeAssignment
	for _, j := range afterJudges {
		if j.PersonID == imelda.ID {
			assigned = j
			break
		}
	}
	if assigned == nil {
		t.Fatal("expected new person to appear in judgesByShow projection")
	}
}

// TestAddShowAdminFormGrantsCorrectBundle — the show-admin endpoint grants
// the show_intake_operator bundle scoped to this show.
func TestAddShowAdminFormGrantsCorrectBundle(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/shows/{showID}/people/show-admin", a.requireCapabilityPage("entries.manage", a.handleAdminAddShowHelperAdminForm))

	body := "first_name=Helga&last_name=Helper&email=helga%40example.com"
	req := httptest.NewRequest("POST", "/admin/shows/show_spring2025/people/show-admin", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAdminSession(t, a, req)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d body=%s", w.Code, w.Body.String())
	}

	var helga *Person
	for _, p := range a.store.allPersons() {
		if p.FirstName == "Helga" && p.LastName == "Helper" {
			helga = p
			break
		}
	}
	if helga == nil {
		t.Fatal("expected helga to be created")
	}

	roles, err := a.authority.AllRoleAssignments(context.Background())
	if err != nil {
		t.Fatalf("list roles: %v", err)
	}
	found := false
	for _, role := range roles {
		if role.SubjectID == helga.ID && role.Role == "show_intake_operator" && role.ShowID == "show_spring2025" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected show_intake_operator bundle scoped to show for helga, got %+v", roles)
	}
}

// TestAddMemberWithEntrantRole — POST member endpoint with entrant role.
func TestAddMemberWithEntrantRole(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/shows/{showID}/people/member", a.requireCapabilityPage("entries.manage", a.handleAdminAddMemberEntrantForm))

	body := "first_name=Ella&last_name=Entrant&role=flowershow_entrant"
	req := httptest.NewRequest("POST", "/admin/shows/show_spring2025/people/member", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAdminSession(t, a, req)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d body=%s", w.Code, w.Body.String())
	}

	var ella *Person
	for _, p := range a.store.allPersons() {
		if p.FirstName == "Ella" && p.LastName == "Entrant" {
			ella = p
			break
		}
	}
	if ella == nil {
		t.Fatal("expected ella to be created")
	}
	roles, err := a.authority.AllRoleAssignments(context.Background())
	if err != nil {
		t.Fatalf("list roles: %v", err)
	}
	found := false
	for _, role := range roles {
		if role.SubjectID == ella.ID && role.Role == "entrant" && role.ShowID == "show_spring2025" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected entrant bundle for ella scoped to show, got %+v", roles)
	}
}

// TestAddMemberWithShowAdminRole — same endpoint, promote to show admin.
func TestAddMemberWithShowAdminRole(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/shows/{showID}/people/member", a.requireCapabilityPage("entries.manage", a.handleAdminAddMemberEntrantForm))

	body := "first_name=Owen&last_name=Operator&role=flowershow_show_intake_operator"
	req := httptest.NewRequest("POST", "/admin/shows/show_spring2025/people/member", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAdminSession(t, a, req)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d body=%s", w.Code, w.Body.String())
	}

	var owen *Person
	for _, p := range a.store.allPersons() {
		if p.FirstName == "Owen" && p.LastName == "Operator" {
			owen = p
			break
		}
	}
	if owen == nil {
		t.Fatal("expected owen to be created")
	}
	roles, err := a.authority.AllRoleAssignments(context.Background())
	if err != nil {
		t.Fatalf("list roles: %v", err)
	}
	found := false
	for _, role := range roles {
		if role.SubjectID == owen.ID && role.Role == "show_intake_operator" && role.ShowID == "show_spring2025" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected show_intake_operator bundle for owen via member form, got %+v", roles)
	}
}

// TestAllFormsReuseExistingPersonByID — if the form posts a person_id,
// no new person is created and only the role/judge assignment lands.
func TestAllFormsReuseExistingPersonByID(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/shows/{showID}/people/judge", a.requireCapabilityPage("entries.manage", a.handleAdminAddJudgeForm))
	mux.HandleFunc("POST /admin/shows/{showID}/people/show-admin", a.requireCapabilityPage("entries.manage", a.handleAdminAddShowHelperAdminForm))
	mux.HandleFunc("POST /admin/shows/{showID}/people/member", a.requireCapabilityPage("entries.manage", a.handleAdminAddMemberEntrantForm))

	beforePersons := len(a.store.allPersons())

	// Use seeded person_01 (Margaret Chen).
	post := func(t *testing.T, path, body string) {
		t.Helper()
		req := httptest.NewRequest("POST", path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		addAdminSession(t, a, req)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusSeeOther {
			t.Fatalf("path %s expected 303, got %d body=%s", path, w.Code, w.Body.String())
		}
	}

	post(t, "/admin/shows/show_spring2025/people/judge", "person_id=person_01&years_in_flower_shows=8")
	post(t, "/admin/shows/show_spring2025/people/show-admin", "person_id=person_02")
	post(t, "/admin/shows/show_spring2025/people/member", "person_id=person_03&role=flowershow_entrant")

	if got := len(a.store.allPersons()); got != beforePersons {
		t.Fatalf("expected no new persons created; before=%d after=%d", beforePersons, got)
	}

	// Judge promotion landed on existing person_01.
	p1, ok := a.store.personByID("person_01")
	if !ok || !p1.IsJudge {
		t.Fatalf("expected person_01 to be promoted to judge, got %+v", p1)
	}
	wantYear := time.Now().UTC().Year() - 8
	if p1.JudgingStartedYear != wantYear {
		t.Fatalf("expected JudgingStartedYear=%d on person_01, got %d", wantYear, p1.JudgingStartedYear)
	}
	judges := a.store.judgesByShow("show_spring2025")
	hasP1 := false
	for _, j := range judges {
		if j.PersonID == "person_01" {
			hasP1 = true
		}
	}
	if !hasP1 {
		t.Fatal("expected person_01 to be assigned as judge for show")
	}

	// Role grants landed for person_02 and person_03.
	roles, err := a.authority.AllRoleAssignments(context.Background())
	if err != nil {
		t.Fatalf("list roles: %v", err)
	}
	hasHelper, hasEntrant := false, false
	for _, role := range roles {
		if role.SubjectID == "person_02" && role.Role == "show_intake_operator" && role.ShowID == "show_spring2025" {
			hasHelper = true
		}
		if role.SubjectID == "person_03" && role.Role == "entrant" && role.ShowID == "show_spring2025" {
			hasEntrant = true
		}
	}
	if !hasHelper {
		t.Fatalf("expected show_intake_operator grant for person_02, got %+v", roles)
	}
	if !hasEntrant {
		t.Fatalf("expected entrant grant for person_03, got %+v", roles)
	}
}

// TestModalHelperAdminHidesJudgeAndMemberFields — when the people-roster
// modal HTML renders, both the judge-mode-only block and the member-mode-only
// block must carry the `hidden` attribute on the wrapper. The Helper-Admin
// mode (default banner-driven) leaves both wrappers hidden client-side, but
// even the initial server render needs them hidden so JS can opt them in.
func TestModalHelperAdminHidesJudgeAndMemberFields(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/shows/{showID}", a.requireCapabilityPage("shows.workspace.read", a.handleAdminShowDetail))

	req := httptest.NewRequest("GET", "/admin/shows/show_spring2025", nil)
	addAdminSession(t, a, req)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, marker := range []string{
		`data-people-fields-judge hidden`,
		`data-people-fields-member hidden`,
	} {
		if !strings.Contains(body, marker) {
			t.Fatalf("expected modal markup to include %q so initial render hides the block", marker)
		}
	}
}

// TestMemberRoleDefaultsToShowAdmin — the Member-mode role select should
// default to flowershow_show_intake_operator (Show admin (helper)).
func TestMemberRoleDefaultsToShowAdmin(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/shows/{showID}", a.requireCapabilityPage("shows.workspace.read", a.handleAdminShowDetail))

	req := httptest.NewRequest("GET", "/admin/shows/show_spring2025", nil)
	addAdminSession(t, a, req)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	want := `<option value="flowershow_show_intake_operator" selected>Show admin (helper)</option>`
	if !strings.Contains(body, want) {
		t.Fatalf("expected default-selected show admin option %q in member role select", want)
	}
}

// TestMemberRoleHelpTextIsPresent — the role help paragraph element must
// exist directly under the role dropdown and start with the show-admin copy
// (matching the default selection).
func TestMemberRoleHelpTextIsPresent(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/shows/{showID}", a.requireCapabilityPage("shows.workspace.read", a.handleAdminShowDetail))

	req := httptest.NewRequest("GET", "/admin/shows/show_spring2025", nil)
	addAdminSession(t, a, req)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `class="form-help-text people-roster-role-help"`) {
		t.Fatalf("expected .people-roster-role-help element on initial render")
	}
	if !strings.Contains(body, "Show admin (helper) — can upload photos") {
		t.Fatalf("expected role help text to default to show-admin copy")
	}
}

// fakeSESSender records SendEmail calls for assertions.
type fakeSESSender struct {
	mu    sync.Mutex
	calls int
	last  *sesv2.SendEmailInput
}

func (f *fakeSESSender) SendEmail(ctx context.Context, in *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.last = in
	return &sesv2.SendEmailOutput{}, nil
}

// TestEmailInvitationSentWhenSESConfigured — when AWS_REGION + the from-email
// env var are set and a new person with email is added via Add Show Helper
// Admin, the SES sender should receive a SendEmail call whose body embeds the
// help-redeem URL and the show name.
func TestEmailInvitationSentWhenSESConfigured(t *testing.T) {
	a := testApp()
	fake := &fakeSESSender{}
	a.inviteEmailSender = fake

	t.Setenv("AWS_REGION", "us-east-2")
	t.Setenv("FLOWERSHOW_INVITE_FROM_EMAIL", "noreply@flowershow.test")

	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/shows/{showID}/people/show-admin", a.requireCapabilityPage("entries.manage", a.handleAdminAddShowHelperAdminForm))

	body := "first_name=Nina&last_name=Newcomer&email=nina%40example.com"
	req := httptest.NewRequest("POST", "/admin/shows/show_spring2025/people/show-admin", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAdminSession(t, a, req)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d body=%s", w.Code, w.Body.String())
	}

	if fake.calls != 1 {
		t.Fatalf("expected exactly 1 SES SendEmail call, got %d", fake.calls)
	}
	if fake.last == nil || fake.last.Content == nil || fake.last.Content.Simple == nil {
		t.Fatalf("expected Simple-mode email content, got %+v", fake.last)
	}
	if fake.last.FromEmailAddress == nil || *fake.last.FromEmailAddress != "noreply@flowershow.test" {
		t.Fatalf("expected From=noreply@flowershow.test, got %+v", fake.last.FromEmailAddress)
	}
	if fake.last.Destination == nil || len(fake.last.Destination.ToAddresses) != 1 || fake.last.Destination.ToAddresses[0] != "nina@example.com" {
		t.Fatalf("expected To=nina@example.com, got %+v", fake.last.Destination)
	}

	subject := ""
	if fake.last.Content.Simple.Subject != nil && fake.last.Content.Simple.Subject.Data != nil {
		subject = *fake.last.Content.Simple.Subject.Data
	}
	if !strings.Contains(subject, "added to") {
		t.Fatalf("expected subject to mention adding to show, got %q", subject)
	}

	htmlBody := ""
	if fake.last.Content.Simple.Body != nil && fake.last.Content.Simple.Body.Html != nil && fake.last.Content.Simple.Body.Html.Data != nil {
		htmlBody = *fake.last.Content.Simple.Body.Html.Data
	}
	textBody := ""
	if fake.last.Content.Simple.Body != nil && fake.last.Content.Simple.Body.Text != nil && fake.last.Content.Simple.Body.Text.Data != nil {
		textBody = *fake.last.Content.Simple.Body.Text.Data
	}
	if !strings.Contains(htmlBody, "/help-redeem?token=") {
		t.Fatalf("expected redeem URL embedded in HTML body, got:\n%s", htmlBody)
	}
	if !strings.Contains(textBody, "/help-redeem?token=") {
		t.Fatalf("expected redeem URL in text body, got:\n%s", textBody)
	}
	if !strings.Contains(textBody, "/admin/login") {
		t.Fatalf("expected admin/login fallback URL in text body, got:\n%s", textBody)
	}

	var nina *Person
	for _, p := range a.store.allPersons() {
		if p.FirstName == "Nina" && p.LastName == "Newcomer" {
			nina = p
			break
		}
	}
	if nina == nil {
		t.Fatal("expected nina to be created")
	}
}

// TestEmailInvitationSkippedWhenSESUnconfigured — when env vars are unset, the
// handler runs cleanly without panic and never calls SendEmail; the person and
// role grant still land.
func TestEmailInvitationSkippedWhenSESUnconfigured(t *testing.T) {
	a := testApp()
	fake := &fakeSESSender{}
	a.inviteEmailSender = fake

	t.Setenv("FLOWERSHOW_INVITE_FROM_EMAIL", "")
	t.Setenv("AWS_REGION", "")

	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/shows/{showID}/people/show-admin", a.requireCapabilityPage("entries.manage", a.handleAdminAddShowHelperAdminForm))

	body := "first_name=Quinn&last_name=Quiet&email=quinn%40example.com"
	req := httptest.NewRequest("POST", "/admin/shows/show_spring2025/people/show-admin", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAdminSession(t, a, req)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d body=%s", w.Code, w.Body.String())
	}
	if fake.calls != 0 {
		t.Fatalf("expected 0 SES calls when env unset, got %d", fake.calls)
	}

	var quinn *Person
	for _, p := range a.store.allPersons() {
		if p.FirstName == "Quinn" && p.LastName == "Quiet" {
			quinn = p
			break
		}
	}
	if quinn == nil {
		t.Fatal("expected quinn to be created even when SES is unconfigured")
	}
	roles, err := a.authority.AllRoleAssignments(context.Background())
	if err != nil {
		t.Fatalf("list roles: %v", err)
	}
	found := false
	for _, role := range roles {
		if role.SubjectID == quinn.ID && role.Role == "show_intake_operator" && role.ShowID == "show_spring2025" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected show_intake_operator grant for quinn even when SES is unconfigured, got %+v", roles)
	}
}
