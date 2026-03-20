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

	if _, err := pool.Exec(ctx, `delete from runtime_authority_grants`); err != nil {
		t.Fatalf("clear authority grants: %v", err)
	}
	if _, err := pool.Exec(ctx, `delete from runtime_auth_identities`); err != nil {
		t.Fatalf("clear auth identities: %v", err)
	}
	if _, err := pool.Exec(ctx, `delete from runtime_principal_identifiers`); err != nil {
		t.Fatalf("clear principal identifiers: %v", err)
	}
	if _, err := pool.Exec(ctx, `delete from runtime_principals where principal_id = any($1)`, []string{grantorID, granteeID}); err != nil {
		t.Fatalf("clear principals: %v", err)
	}
	if _, err := pool.Exec(ctx, `delete from runtime_authority_bundles where bundle_id = any($1)`, []string{orgBundleID, showBundleID}); err != nil {
		t.Fatalf("clear authority bundles: %v", err)
	}

	if _, err := service.MaterializeAuthorityState(ctx); err != nil {
		t.Fatalf("materialize authority state: %v", err)
	}

	resolvedAfterRebuild, err := service.ResolveAuthIdentity(ctx, providerID, providerSubject)
	if err != nil {
		t.Fatalf("resolve auth identity after rebuild: %v", err)
	}
	if resolvedAfterRebuild.PrincipalID != granteeID {
		t.Fatalf("expected rebuilt auth identity to point to %s, got %+v", granteeID, resolvedAfterRebuild)
	}

	effectiveAfterRebuild, err := service.GetEffectiveAuthorityByPrincipal(ctx, granteeID)
	if err != nil {
		t.Fatalf("get effective authority after rebuild: %v", err)
	}
	if len(effectiveAfterRebuild.ActiveGrants) != 1 || effectiveAfterRebuild.ActiveGrants[0].GrantID != showGrant.GrantID {
		t.Fatalf("unexpected effective grants after rebuild: %+v", effectiveAfterRebuild.ActiveGrants)
	}
}

func TestRuntimeAuthorityMaterializeMigratesLegacyGrantRowsIntoRegistry(t *testing.T) {
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
		return time.Date(2026, time.March, 20, 15, 0, 0, 0, time.UTC)
	}

	grantorID := newID("prn")
	granteeID := newID("prn")
	for _, item := range []CreatePrincipalInput{
		{PrincipalID: grantorID, Kind: "person", DisplayName: "Legacy Grantor"},
		{PrincipalID: granteeID, Kind: "person", DisplayName: "Legacy Grantee"},
	} {
		if _, err := service.CreatePrincipal(ctx, item); err != nil {
			t.Fatalf("create principal %s: %v", item.PrincipalID, err)
		}
	}
	if _, err := service.UpsertAuthorityBundle(ctx, UpsertAuthorityBundleInput{
		BundleID:     "legacy_bundle",
		DisplayName:  "Legacy Admin",
		Capabilities: []string{"legacy.manage"},
	}); err != nil {
		t.Fatalf("upsert legacy bundle: %v", err)
	}
	if _, err := service.BindAuthIdentity(ctx, BindAuthIdentityInput{
		ProviderID:      "legacy-provider",
		PrincipalID:     granteeID,
		ProviderSubject: "legacy-subject",
		Profile:         map[string]interface{}{"email": "legacy@example.com"},
	}); err != nil {
		t.Fatalf("bind legacy auth identity: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into runtime_authority_grants (
		  grant_id, grantor_principal_id, grantee_principal_id, bundle_id, capabilities_snapshot,
		  scope_kind, scope_id, delegation_mode, basis, status, effective_at, evidence_refs, metadata, created_at
		)
		values ($1, $2, $3, $4, $5, 'seed', '0007-Flowershow', 'same_scope', 'bootstrap', 'accepted', $6, '{}'::text[], '{}'::jsonb, $6)
	`, "legacy_grant_restore", grantorID, granteeID, "legacy_bundle", []string{"legacy.manage"}, service.now()); err != nil {
		t.Fatalf("insert legacy grant row: %v", err)
	}

	if _, err := pool.Exec(ctx, `delete from runtime_registry_rows where object_id = 'legacy_grant_restore'`); err != nil {
		t.Fatalf("clear legacy migration rows: %v", err)
	}
	if _, err := pool.Exec(ctx, `delete from runtime_registry_change_sets where change_set_id = 'authority-legacy-migration'`); err != nil {
		t.Fatalf("clear legacy migration change set: %v", err)
	}

	var before int
	if err := pool.QueryRow(ctx, `
		select count(*)
		from runtime_registry_rows
		where reference = 'runtime-authority' and object_id = 'legacy_grant_restore'
	`).Scan(&before); err != nil {
		t.Fatalf("count registry rows before materialize: %v", err)
	}
	if before != 0 {
		t.Fatalf("expected empty authority registry before migration, got %d", before)
	}

	if _, err := service.MaterializeAuthorityState(ctx); err != nil {
		t.Fatalf("materialize authority state with legacy rows: %v", err)
	}

	var after int
	if err := pool.QueryRow(ctx, `
		select count(*)
		from runtime_registry_rows
		where reference = 'runtime-authority' and object_id = 'legacy_grant_restore'
	`).Scan(&after); err != nil {
		t.Fatalf("count registry rows after materialize: %v", err)
	}
	if after != 1 {
		t.Fatalf("expected 1 migrated authority registry row, got %d", after)
	}

	effective, err := service.GetEffectiveAuthorityByPrincipal(ctx, granteeID)
	if err != nil {
		t.Fatalf("get effective authority after legacy migration: %v", err)
	}
	if len(effective.EffectivePolicies) != 1 || effective.EffectivePolicies[0].Capability != "legacy.manage" {
		t.Fatalf("unexpected effective authority after legacy migration: %+v", effective.EffectivePolicies)
	}
}
