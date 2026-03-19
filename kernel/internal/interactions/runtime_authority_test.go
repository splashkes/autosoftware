package interactions

import (
	"context"
	"os"
	"testing"
	"time"

	realizations "as/kernel/internal/realizations"
	runtimedb "as/kernel/internal/runtime"
)

func TestRuntimeAuthorityLedgerAndEffectiveAccess(t *testing.T) {
	dsn := os.Getenv("AS_RUNTIME_DATABASE_URL")
	if dsn == "" {
		t.Skip("AS_RUNTIME_DATABASE_URL is required")
	}

	ctx := context.Background()
	pool, err := runtimedb.OpenPool(ctx, dsn)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	defer pool.Close()

	repoRoot, err := realizations.FindRepoRoot(".")
	if err != nil {
		t.Fatalf("find repo root: %v", err)
	}
	if err := runtimedb.RunMigrations(ctx, pool, repoRoot); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	service := NewRuntimeService(pool)
	service.now = func() time.Time {
		return time.Date(2026, time.March, 19, 15, 30, 0, 0, time.UTC)
	}

	grantorID := newID("prn")
	granteeID := newID("prn")
	if _, err := service.CreatePrincipal(ctx, CreatePrincipalInput{
		PrincipalID: grantorID,
		Kind:        "person",
		DisplayName: "Grantor",
	}); err != nil {
		t.Fatalf("create grantor principal: %v", err)
	}
	if _, err := service.CreatePrincipal(ctx, CreatePrincipalInput{
		PrincipalID: granteeID,
		Kind:        "person",
		DisplayName: "Grantee",
	}); err != nil {
		t.Fatalf("create grantee principal: %v", err)
	}

	providerID := "cognito-test"
	providerSubject := newID("sub")
	bound, err := service.BindAuthIdentity(ctx, BindAuthIdentityInput{
		ProviderID:      providerID,
		PrincipalID:     granteeID,
		ProviderSubject: providerSubject,
		Profile: map[string]interface{}{
			"email": "authority-test@example.com",
		},
	})
	if err != nil {
		t.Fatalf("bind auth identity: %v", err)
	}
	resolved, err := service.ResolveAuthIdentity(ctx, providerID, providerSubject)
	if err != nil {
		t.Fatalf("resolve auth identity: %v", err)
	}
	if resolved.PrincipalID != granteeID || bound.IdentityID != resolved.IdentityID {
		t.Fatalf("expected resolved auth identity to point to %s, got %+v", granteeID, resolved)
	}

	orgBundleID := newID("bundle_org_admin")
	orgBundle, err := service.UpsertAuthorityBundle(ctx, UpsertAuthorityBundleInput{
		BundleID:    orgBundleID,
		DisplayName: "Organization Admin",
		Capabilities: []string{
			"organization.executive.manage_roster",
			"organization.member.read_private",
		},
	})
	if err != nil {
		t.Fatalf("upsert org bundle: %v", err)
	}
	if len(orgBundle.Capabilities) != 2 {
		t.Fatalf("expected 2 capabilities in org bundle, got %+v", orgBundle.Capabilities)
	}

	showBundleID := newID("bundle_show_judge")
	showBundle, err := service.UpsertAuthorityBundle(ctx, UpsertAuthorityBundleInput{
		BundleID:    showBundleID,
		DisplayName: "Show Judge",
		Capabilities: []string{
			"show.entry.score",
		},
	})
	if err != nil {
		t.Fatalf("upsert show bundle: %v", err)
	}
	if len(showBundle.Capabilities) != 1 || showBundle.Capabilities[0] != "show.entry.score" {
		t.Fatalf("unexpected show bundle capabilities: %+v", showBundle.Capabilities)
	}

	organizationGrant, err := service.CreateAuthorityGrant(ctx, CreateAuthorityGrantInput{
		GrantorPrincipalID: grantorID,
		GranteePrincipalID: granteeID,
		BundleID:           orgBundleID,
		ScopeKind:          "organization",
		ScopeID:            newID("org"),
		DelegationMode:     "same_scope",
		Status:             "accepted",
	})
	if err != nil {
		t.Fatalf("create organization grant: %v", err)
	}

	if _, err := service.CreateAuthorityGrant(ctx, CreateAuthorityGrantInput{
		GrantorPrincipalID: grantorID,
		SupersedesGrantID:  organizationGrant.GrantID,
		Status:             "revoked",
		Reason:             "Access withdrawn",
	}); err != nil {
		t.Fatalf("create revoked grant event: %v", err)
	}

	showGrant, err := service.CreateAuthorityGrant(ctx, CreateAuthorityGrantInput{
		GrantorPrincipalID: grantorID,
		GranteePrincipalID: granteeID,
		BundleID:           showBundleID,
		ScopeKind:          "show",
		ScopeID:            newID("show"),
		DelegationMode:     "none",
		Status:             "accepted",
	})
	if err != nil {
		t.Fatalf("create show grant: %v", err)
	}

	ledger, err := service.ListAuthorityLedgerByPrincipal(ctx, granteeID, 10)
	if err != nil {
		t.Fatalf("list authority ledger: %v", err)
	}
	if len(ledger) != 3 {
		t.Fatalf("expected 3 ledger rows, got %d", len(ledger))
	}

	effective, err := service.GetEffectiveAuthorityByPrincipal(ctx, granteeID)
	if err != nil {
		t.Fatalf("get effective authority: %v", err)
	}
	if effective.PrincipalID != granteeID {
		t.Fatalf("expected effective principal %s, got %s", granteeID, effective.PrincipalID)
	}
	if len(effective.ActiveGrants) != 1 {
		t.Fatalf("expected 1 active grant after revocation, got %d", len(effective.ActiveGrants))
	}
	if effective.ActiveGrants[0].GrantID != showGrant.GrantID {
		t.Fatalf("expected active show grant %s, got %+v", showGrant.GrantID, effective.ActiveGrants[0])
	}
	if len(effective.EffectivePolicies) != 1 {
		t.Fatalf("expected 1 effective capability, got %d", len(effective.EffectivePolicies))
	}
	if effective.EffectivePolicies[0].Capability != "show.entry.score" {
		t.Fatalf("expected show.entry.score capability, got %+v", effective.EffectivePolicies[0])
	}
}
