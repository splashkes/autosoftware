package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	flowershowAuthorityScopeSeed     = "seed"
	flowershowAuthorityScopeSeedID   = "0007-Flowershow"
	flowershowAuthorityScopePlatform = "platform"
	flowershowAuthorityPlatformID    = "autosoftware"
)

type authorityScope struct {
	Kind string
	ID   string
}

type authorityPolicy struct {
	Capability string
	ScopeKind  string
	ScopeID    string
}

type runtimeAuthorityResolver interface {
	Init(context.Context, flowershowStore) error
	EffectivePolicies(context.Context, string) ([]authorityPolicy, error)
	RoleAssignmentsForUser(context.Context, UserIdentity) ([]*UserRole, error)
	AllRoleAssignments(context.Context) ([]*UserRole, error)
	AssignRole(context.Context, UserRoleInput, string) (*UserRole, error)
}

type postgresRuntimeAuthorityResolver struct {
	pool     *pgxpool.Pool
	boundary registryBoundary
}

type memoryRuntimeAuthorityResolver struct {
	mu    sync.RWMutex
	roles map[string]*UserRole
}

type authorityBundleDef struct {
	BundleID     string
	DisplayName  string
	Role         string
	Capabilities []string
}

var flowershowAuthorityBundles = map[string]authorityBundleDef{
	"admin": {
		BundleID:    "flowershow_admin",
		DisplayName: "Flowershow Admin",
		Role:        "admin",
		Capabilities: []string{
			"account.read",
			"admin.dashboard.read",
			"organization.manage",
			"organization.invites.manage",
			"shows.workspace.read",
			"entries.private.read",
			"ledger.read",
			"shows.manage",
			"schedule.manage",
			"classes.manage",
			"judges.manage",
			"entries.manage",
			"persons.manage",
			"awards.manage",
			"taxonomy.manage",
			"media.manage",
			"rubrics.manage",
			"standards.manage",
			"sources.manage",
			"show_credits.manage",
			"roles.manage",
		},
	},
	"organization_admin": {
		BundleID:    "flowershow_organization_admin",
		DisplayName: "Club Admin",
		Role:        "organization_admin",
		Capabilities: []string{
			"account.read",
			"organization.manage",
			"organization.invites.manage",
			"shows.workspace.read",
			"entries.private.read",
			"shows.manage",
			"schedule.manage",
			"classes.manage",
			"judges.manage",
			"entries.manage",
			"persons.manage",
			"awards.manage",
			"media.manage",
			"show_credits.manage",
			"roles.manage",
		},
	},
	"judge": {
		BundleID:    "flowershow_judge",
		DisplayName: "Flowershow Judge",
		Role:        "judge",
		Capabilities: []string{
			"shows.workspace.read",
			"entries.private.read",
			"rubrics.manage",
		},
	},
	"show_intake_operator": {
		BundleID:    "flowershow_show_intake_operator",
		DisplayName: "Show Intake Operator",
		Role:        "show_intake_operator",
		Capabilities: []string{
			"account.read",
			"shows.workspace.read",
			"entries.private.read",
			"entries.manage",
			"media.manage",
			"persons.manage",
			"show_credits.manage",
		},
	},
	"show_judge_support": {
		BundleID:    "flowershow_show_judge_support",
		DisplayName: "Show Judge Support",
		Role:        "show_judge_support",
		Capabilities: []string{
			"account.read",
			"shows.workspace.read",
			"entries.private.read",
			"entries.manage",
			"media.manage",
			"persons.manage",
			"show_credits.manage",
			"judges.manage",
		},
	},
	"photographer": {
		BundleID:    "flowershow_photographer",
		DisplayName: "Photographer",
		Role:        "photographer",
		Capabilities: []string{
			"account.read",
			"shows.workspace.read",
			"entries.private.read",
			"media.manage",
		},
	},
	"entrant": {
		BundleID:    "flowershow_entrant",
		DisplayName: "Flowershow Entrant",
		Role:        "entrant",
		Capabilities: []string{
			"account.read",
		},
	},
}

func newRuntimeAuthorityResolver(store flowershowStore) runtimeAuthorityResolver {
	switch typed := store.(type) {
	case *postgresFlowershowStore:
		if typed == nil || typed.pool == nil {
			return nil
		}
		return &postgresRuntimeAuthorityResolver{pool: typed.pool, boundary: typed.registry}
	case *memoryStore:
		return &memoryRuntimeAuthorityResolver{
			roles: make(map[string]*UserRole),
		}
	default:
		return nil
	}
}

func (r *postgresRuntimeAuthorityResolver) Init(ctx context.Context, store flowershowStore) error {
	if r == nil || r.pool == nil {
		return nil
	}
	if err := r.ensureBundles(ctx); err != nil {
		return err
	}
	if err := r.migrateLegacyRoleTable(ctx); err != nil {
		return err
	}
	if r.boundary != nil {
		if err := r.boundary.MaterializeAuthorityState(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (r *memoryRuntimeAuthorityResolver) Init(context.Context, flowershowStore) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.roles == nil {
		r.roles = make(map[string]*UserRole)
	}
	return nil
}

func (r *postgresRuntimeAuthorityResolver) ensureBundles(ctx context.Context) error {
	if r.boundary == nil {
		return errors.New("kernel runtime authority boundary unavailable")
	}
	for _, def := range flowershowAuthorityBundles {
		if err := r.boundary.UpsertAuthorityBundle(ctx, authorityBundleUpsertInput{
			BundleID:     def.BundleID,
			DisplayName:  def.DisplayName,
			Capabilities: normalizeStringList(def.Capabilities),
			Status:       "active",
			Metadata:     map[string]any{"role": def.Role},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *postgresRuntimeAuthorityResolver) EffectivePolicies(ctx context.Context, principalID string) ([]authorityPolicy, error) {
	if r == nil || r.pool == nil {
		return nil, nil
	}
	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		return nil, nil
	}
	now := time.Now().UTC()
	rows, err := r.pool.Query(ctx, `
		select coalesce(b.capabilities, g.capabilities_snapshot), g.scope_kind, g.scope_id
		from runtime_authority_grants g
		left join runtime_authority_bundles b on b.bundle_id = g.bundle_id
		where g.grantee_principal_id = $1
		  and g.status = 'accepted'
		  and (g.effective_at is null or g.effective_at <= $2)
		  and (g.expires_at is null or g.expires_at > $2)
		  and not exists (
		    select 1
		    from runtime_authority_grants newer
		    where newer.supersedes_grant_id = g.grant_id
		      and newer.status in ('accepted', 'revoked', 'superseded')
		  )
		order by g.created_at desc, g.grant_id desc
	`, principalID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seen := make(map[string]struct{})
	out := make([]authorityPolicy, 0)
	for rows.Next() {
		var capabilities []string
		var scopeKind string
		var scopeID string
		if err := rows.Scan(&capabilities, &scopeKind, &scopeID); err != nil {
			return nil, err
		}
		for _, capability := range normalizeStringList(capabilities) {
			key := capability + "|" + scopeKind + "|" + scopeID
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, authorityPolicy{
				Capability: capability,
				ScopeKind:  strings.TrimSpace(scopeKind),
				ScopeID:    strings.TrimSpace(scopeID),
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Capability != out[j].Capability {
			return out[i].Capability < out[j].Capability
		}
		if out[i].ScopeKind != out[j].ScopeKind {
			return out[i].ScopeKind < out[j].ScopeKind
		}
		return out[i].ScopeID < out[j].ScopeID
	})
	return out, nil
}

func (r *memoryRuntimeAuthorityResolver) EffectivePolicies(_ context.Context, principalID string) ([]authorityPolicy, error) {
	if r == nil {
		return nil, nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		return nil, nil
	}
	out := make([]authorityPolicy, 0)
	seen := make(map[string]struct{})
	for _, role := range r.roles {
		if role == nil || role.SubjectID != principalID {
			continue
		}
		def, ok := flowershowAuthorityBundles[strings.TrimSpace(role.Role)]
		if !ok {
			continue
		}
		scope := roleScopeFromInput(UserRoleInput{
			OrganizationID: role.OrganizationID,
			ShowID:         role.ShowID,
		})
		for _, capability := range normalizeStringList(def.Capabilities) {
			key := capability + "|" + scope.Kind + "|" + scope.ID
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, authorityPolicy{
				Capability: capability,
				ScopeKind:  scope.Kind,
				ScopeID:    scope.ID,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Capability != out[j].Capability {
			return out[i].Capability < out[j].Capability
		}
		if out[i].ScopeKind != out[j].ScopeKind {
			return out[i].ScopeKind < out[j].ScopeKind
		}
		return out[i].ScopeID < out[j].ScopeID
	})
	return out, nil
}

func (r *postgresRuntimeAuthorityResolver) RoleAssignmentsForUser(ctx context.Context, user UserIdentity) ([]*UserRole, error) {
	return r.roleAssignments(ctx, user.subjectLookupKey())
}

func (r *memoryRuntimeAuthorityResolver) RoleAssignmentsForUser(_ context.Context, user UserIdentity) ([]*UserRole, error) {
	if r == nil {
		return nil, nil
	}
	return r.roleAssignments(user.subjectLookupKey()), nil
}

func (r *postgresRuntimeAuthorityResolver) AllRoleAssignments(ctx context.Context) ([]*UserRole, error) {
	if r == nil || r.pool == nil {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, `
		select g.grant_id, g.grantee_principal_id, coalesce(ai.provider_subject, ''), g.scope_kind, g.scope_id, g.bundle_id, g.created_at
		from runtime_authority_grants g
		left join lateral (
		  select provider_subject
		  from runtime_auth_identities
		  where principal_id = g.grantee_principal_id
		  order by last_seen_at desc nulls last, linked_at desc
		  limit 1
		) ai on true
		where g.status = 'accepted'
		  and not exists (
		    select 1
		    from runtime_authority_grants newer
		    where newer.supersedes_grant_id = g.grant_id
		      and newer.status in ('accepted', 'revoked', 'superseded')
		  )
		  and g.bundle_id = any($1)
		order by g.created_at asc, g.grant_id asc
	`, runtimeAuthorityBundleIDs())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRoleAssignments(rows)
}

func (r *memoryRuntimeAuthorityResolver) AllRoleAssignments(context.Context) ([]*UserRole, error) {
	if r == nil {
		return nil, nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*UserRole, 0, len(r.roles))
	for _, role := range r.roles {
		if role == nil {
			continue
		}
		copied := *role
		out = append(out, &copied)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (r *postgresRuntimeAuthorityResolver) AssignRole(ctx context.Context, input UserRoleInput, grantorPrincipalID string) (*UserRole, error) {
	if r == nil || r.pool == nil {
		return nil, errors.New("runtime authority unavailable")
	}
	input.Role = strings.TrimSpace(input.Role)
	def, ok := flowershowAuthorityBundles[input.Role]
	if !ok {
		return nil, errors.New("unsupported runtime role")
	}
	if err := r.ensureBundles(ctx); err != nil {
		return nil, err
	}

	principalID, cognitoSub, err := r.resolvePrincipalForRole(ctx, input)
	if err != nil {
		return nil, err
	}
	scope := roleScopeFromInput(input)
	if existing, ok, err := r.findActiveGrant(ctx, principalID, def.BundleID, scope.Kind, scope.ID); err != nil {
		return nil, err
	} else if ok {
		return existing, nil
	}
	if r.boundary == nil {
		return nil, errors.New("kernel runtime authority boundary unavailable")
	}

	role := &UserRole{
		ID:             newID("grant"),
		SubjectID:      principalID,
		CognitoSub:     cognitoSub,
		OrganizationID: input.OrganizationID,
		ShowID:         input.ShowID,
		Role:           input.Role,
		CreatedAt:      time.Now().UTC(),
	}
	err = r.boundary.CreateAuthorityGrant(ctx, authorityGrantCreateInput{
		GrantID:            role.ID,
		GrantorPrincipalID: strings.TrimSpace(grantorPrincipalID),
		GranteePrincipalID: principalID,
		BundleID:           def.BundleID,
		ScopeKind:          scope.Kind,
		ScopeID:            scope.ID,
		DelegationMode:     "same_scope",
		Basis:              "delegated",
		Status:             "accepted",
		Reason:             "flowershow role assignment: " + input.Role,
		Metadata: map[string]any{
			"role":            input.Role,
			"organization_id": strings.TrimSpace(input.OrganizationID),
			"show_id":         strings.TrimSpace(input.ShowID),
			"cognito_sub":     cognitoSub,
		},
	})
	if err != nil {
		return nil, err
	}
	return role, nil
}

func (r *memoryRuntimeAuthorityResolver) AssignRole(_ context.Context, input UserRoleInput, _ string) (*UserRole, error) {
	if r == nil {
		return nil, errors.New("runtime authority unavailable")
	}
	input.Role = strings.TrimSpace(input.Role)
	def, ok := flowershowAuthorityBundles[input.Role]
	if !ok {
		return nil, errors.New("unsupported runtime role")
	}
	principalID := strings.TrimSpace(input.SubjectID)
	if principalID == "" {
		principalID = strings.TrimSpace(input.CognitoSub)
	}
	cognitoSub := strings.TrimSpace(input.CognitoSub)
	if principalID == "" {
		return nil, errors.New("subject identifier required")
	}
	role := &UserRole{
		ID:             newID("grant"),
		SubjectID:      principalID,
		CognitoSub:     cognitoSub,
		OrganizationID: strings.TrimSpace(input.OrganizationID),
		ShowID:         strings.TrimSpace(input.ShowID),
		Role:           def.Role,
		CreatedAt:      time.Now().UTC(),
	}
	key := role.SubjectID + "|" + role.CognitoSub + "|" + role.OrganizationID + "|" + role.ShowID + "|" + role.Role
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.roles[key]; ok {
		copied := *existing
		return &copied, nil
	}
	r.roles[key] = role
	copied := *role
	return &copied, nil
}

func (r *postgresRuntimeAuthorityResolver) migrateLegacyRoleTable(ctx context.Context) error {
	if r == nil || r.pool == nil {
		return nil
	}
	var legacyTable string
	if err := r.pool.QueryRow(ctx, `select coalesce(to_regclass('public.as_flowershow_user_roles')::text, '')`).Scan(&legacyTable); err != nil {
		return err
	}
	if legacyTable == "" {
		return nil
	}
	rows, err := r.pool.Query(ctx, `
		select subject_id, cognito_sub, organization_id, show_id, role
		from as_flowershow_user_roles
		order by created_at asc, id asc
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var input UserRoleInput
		if err := rows.Scan(&input.SubjectID, &input.CognitoSub, &input.OrganizationID, &input.ShowID, &input.Role); err != nil {
			return err
		}
		if _, ok := flowershowAuthorityBundles[strings.TrimSpace(input.Role)]; !ok {
			continue
		}
		if _, err := r.AssignRole(ctx, input, ""); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "sign in once") {
				continue
			}
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `drop table if exists as_flowershow_user_roles`)
	return err
}

func (r *postgresRuntimeAuthorityResolver) roleAssignments(ctx context.Context, subjectKey string) ([]*UserRole, error) {
	if r == nil || r.pool == nil {
		return nil, nil
	}
	subjectKey = strings.TrimSpace(subjectKey)
	if subjectKey == "" {
		return nil, nil
	}
	principalID, cognitoSub, err := r.resolvePrincipalLookupKey(ctx, subjectKey)
	if err != nil {
		return nil, err
	}
	if principalID == "" {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, `
		select g.grant_id, g.grantee_principal_id, $2 as cognito_sub, g.scope_kind, g.scope_id, g.bundle_id, g.created_at
		from runtime_authority_grants g
		where g.grantee_principal_id = $1
		  and g.status = 'accepted'
		  and not exists (
		    select 1
		    from runtime_authority_grants newer
		    where newer.supersedes_grant_id = g.grant_id
		      and newer.status in ('accepted', 'revoked', 'superseded')
		  )
		  and g.bundle_id = any($3)
		order by g.created_at asc, g.grant_id asc
	`, principalID, cognitoSub, runtimeAuthorityBundleIDs())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRoleAssignments(rows)
}

func (r *memoryRuntimeAuthorityResolver) roleAssignments(subjectKey string) []*UserRole {
	r.mu.RLock()
	defer r.mu.RUnlock()
	subjectKey = strings.TrimSpace(subjectKey)
	if subjectKey == "" {
		return nil
	}
	out := make([]*UserRole, 0)
	for _, role := range r.roles {
		if role == nil {
			continue
		}
		if role.SubjectID != subjectKey && role.CognitoSub != subjectKey {
			continue
		}
		copied := *role
		out = append(out, &copied)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func (r *postgresRuntimeAuthorityResolver) resolvePrincipalLookupKey(ctx context.Context, subjectKey string) (string, string, error) {
	subjectKey = strings.TrimSpace(subjectKey)
	if subjectKey == "" {
		return "", "", nil
	}
	var principalID string
	var cognitoSub string
	row := r.pool.QueryRow(ctx, `
		select principal_id, provider_subject
		from runtime_auth_identities
		where principal_id = $1 or provider_subject = $1
		order by last_seen_at desc nulls last, linked_at desc
		limit 1
	`, subjectKey)
	if err := row.Scan(&principalID, &cognitoSub); err == nil {
		return principalID, cognitoSub, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return "", "", err
	}
	if strings.HasPrefix(subjectKey, "prn_") {
		return subjectKey, "", nil
	}
	return "", "", nil
}

func (r *postgresRuntimeAuthorityResolver) resolvePrincipalForRole(ctx context.Context, input UserRoleInput) (string, string, error) {
	if subjectID := strings.TrimSpace(input.SubjectID); subjectID != "" {
		_, cognitoSub, err := r.resolvePrincipalLookupKey(ctx, subjectID)
		return subjectID, cognitoSub, err
	}
	if cognitoSub := strings.TrimSpace(input.CognitoSub); cognitoSub != "" {
		principalID, providerSubject, err := r.resolvePrincipalLookupKey(ctx, cognitoSub)
		if err != nil {
			return "", "", err
		}
		if principalID == "" {
			return "", "", errors.New("target subject must sign in once before runtime authority can be granted")
		}
		if providerSubject == "" {
			providerSubject = cognitoSub
		}
		return principalID, providerSubject, nil
	}
	return "", "", errors.New("subject identifier required")
}

func (r *postgresRuntimeAuthorityResolver) findActiveGrant(ctx context.Context, principalID, bundleID, scopeKind, scopeID string) (*UserRole, bool, error) {
	row := r.pool.QueryRow(ctx, `
		select g.grant_id, g.grantee_principal_id, coalesce(ai.provider_subject, ''), g.scope_kind, g.scope_id, g.bundle_id, g.created_at
		from runtime_authority_grants g
		left join lateral (
		  select provider_subject
		  from runtime_auth_identities
		  where principal_id = g.grantee_principal_id
		  order by last_seen_at desc nulls last, linked_at desc
		  limit 1
		) ai on true
		where g.grantee_principal_id = $1
		  and g.bundle_id = $2
		  and g.scope_kind = $3
		  and g.scope_id = $4
		  and g.status = 'accepted'
		  and not exists (
		    select 1
		    from runtime_authority_grants newer
		    where newer.supersedes_grant_id = g.grant_id
		      and newer.status in ('accepted', 'revoked', 'superseded')
		  )
		limit 1
	`, principalID, bundleID, scopeKind, scopeID)
	item, err := scanRoleAssignmentRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return item, true, nil
}

func roleScopeFromInput(input UserRoleInput) authorityScope {
	if showID := strings.TrimSpace(input.ShowID); showID != "" {
		return authorityScope{Kind: "show", ID: showID}
	}
	if orgID := strings.TrimSpace(input.OrganizationID); orgID != "" {
		return authorityScope{Kind: "organization", ID: orgID}
	}
	return authorityScope{Kind: flowershowAuthorityScopeSeed, ID: flowershowAuthorityScopeSeedID}
}

func runtimeAuthorityBundleIDs() []string {
	out := make([]string, 0, len(flowershowAuthorityBundles))
	for _, def := range flowershowAuthorityBundles {
		out = append(out, def.BundleID)
	}
	sort.Strings(out)
	return out
}

func roleForBundle(bundleID string) string {
	for _, def := range flowershowAuthorityBundles {
		if def.BundleID == bundleID {
			return def.Role
		}
	}
	return ""
}

func scopeFields(kind, id string) (string, string) {
	switch strings.TrimSpace(kind) {
	case "organization":
		return strings.TrimSpace(id), ""
	case "show":
		return "", strings.TrimSpace(id)
	default:
		return "", ""
	}
}

func scanRoleAssignments(rows pgx.Rows) ([]*UserRole, error) {
	out := make([]*UserRole, 0)
	for rows.Next() {
		item, err := scanRoleAssignmentRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

type roleRowScanner interface {
	Scan(dest ...any) error
}

func scanRoleAssignmentRow(row roleRowScanner) (*UserRole, error) {
	var grantID string
	var subjectID string
	var cognitoSub string
	var scopeKind string
	var scopeID string
	var bundleID string
	var createdAt time.Time
	if err := row.Scan(&grantID, &subjectID, &cognitoSub, &scopeKind, &scopeID, &bundleID, &createdAt); err != nil {
		return nil, err
	}
	role := roleForBundle(bundleID)
	orgID, showID := scopeFields(scopeKind, scopeID)
	return &UserRole{
		ID:             grantID,
		SubjectID:      subjectID,
		CognitoSub:     cognitoSub,
		OrganizationID: orgID,
		ShowID:         showID,
		Role:           role,
		CreatedAt:      createdAt,
	}, nil
}

func normalizeStringList(values []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func jsonBytes(v any) []byte {
	if v == nil {
		return []byte(`{}`)
	}
	raw, err := json.Marshal(v)
	if err != nil || len(raw) == 0 {
		return []byte(`{}`)
	}
	return raw
}

func (a *app) userHasCapability(ctx context.Context, user UserIdentity, capability string, scopes ...authorityScope) bool {
	capability = strings.TrimSpace(capability)
	if capability == "" {
		return true
	}
	if a.authority != nil && strings.TrimSpace(user.SubjectID) != "" {
		policies, err := a.authority.EffectivePolicies(ctx, user.SubjectID)
		if err == nil {
			return authorityPoliciesAllow(policies, capability, scopes...)
		}
		return false
	}
	return false
}

func authorityPoliciesAllow(policies []authorityPolicy, capability string, scopes ...authorityScope) bool {
	capability = strings.TrimSpace(capability)
	if capability == "" {
		return true
	}
	scopeSet := make(map[string]struct{})
	scopeSet[flowershowAuthorityScopeSeed+"|"+flowershowAuthorityScopeSeedID] = struct{}{}
	for _, scope := range scopes {
		scope.Kind = strings.TrimSpace(scope.Kind)
		scope.ID = strings.TrimSpace(scope.ID)
		if scope.Kind == "" || scope.ID == "" {
			continue
		}
		scopeSet[scope.Kind+"|"+scope.ID] = struct{}{}
	}
	for _, policy := range policies {
		if policy.Capability != capability {
			continue
		}
		switch {
		case policy.ScopeKind == flowershowAuthorityScopeSeed && policy.ScopeID == flowershowAuthorityScopeSeedID:
			return true
		case policy.ScopeKind == flowershowAuthorityScopePlatform && (policy.ScopeID == flowershowAuthorityPlatformID || policy.ScopeID == "*" || policy.ScopeID == ""):
			return true
		default:
			if _, ok := scopeSet[policy.ScopeKind+"|"+policy.ScopeID]; ok {
				return true
			}
		}
	}
	return false
}

func (a *app) authorityScopesForRequest(r *http.Request) []authorityScope {
	scopes := []authorityScope{{Kind: flowershowAuthorityScopeSeed, ID: flowershowAuthorityScopeSeedID}}
	add := func(kind, id string) {
		kind = strings.TrimSpace(kind)
		id = strings.TrimSpace(id)
		if kind == "" || id == "" {
			return
		}
		for _, existing := range scopes {
			if existing.Kind == kind && existing.ID == id {
				return
			}
		}
		scopes = append(scopes, authorityScope{Kind: kind, ID: id})
	}
	if r == nil {
		return scopes
	}
	if showID := strings.TrimSpace(r.PathValue("showID")); showID != "" {
		a.expandShowScopes(&scopes, showID)
	}
	if showID := strings.TrimSpace(r.PathValue("id")); showID != "" && strings.Contains(r.URL.Path, "/shows/") {
		a.expandShowScopes(&scopes, showID)
	}
	if orgID := strings.TrimSpace(r.PathValue("id")); orgID != "" && strings.Contains(r.URL.Path, "/clubs/") {
		add("organization", orgID)
	}
	if classID := strings.TrimSpace(r.PathValue("classID")); classID != "" {
		a.expandClassScopes(&scopes, classID)
	}
	if classID := strings.TrimSpace(r.PathValue("id")); classID != "" && strings.Contains(r.URL.Path, "/classes/") {
		a.expandClassScopes(&scopes, classID)
	}
	if entryID := strings.TrimSpace(r.PathValue("entryID")); entryID != "" {
		a.expandEntryScopes(&scopes, entryID)
	}
	if entryID := strings.TrimSpace(r.PathValue("id")); entryID != "" && strings.Contains(r.URL.Path, "/entries/") {
		a.expandEntryScopes(&scopes, entryID)
	}
	if orgID := strings.TrimSpace(r.URL.Query().Get("org_id")); orgID != "" {
		add("organization", orgID)
	}
	if orgID := strings.TrimSpace(r.PathValue("organizationID")); orgID != "" {
		add("organization", orgID)
	}
	if showID := strings.TrimSpace(r.URL.Query().Get("show_id")); showID != "" {
		a.expandShowScopes(&scopes, showID)
	}
	if classID := strings.TrimSpace(r.URL.Query().Get("class_id")); classID != "" {
		a.expandClassScopes(&scopes, classID)
	}
	return scopes
}

func (a *app) commandScopesFromPayload(command string, body []byte) []authorityScope {
	scopes := []authorityScope{{Kind: flowershowAuthorityScopeSeed, ID: flowershowAuthorityScopeSeedID}}
	add := func(kind, id string) {
		kind = strings.TrimSpace(kind)
		id = strings.TrimSpace(id)
		if kind == "" || id == "" {
			return
		}
		for _, existing := range scopes {
			if existing.Kind == kind && existing.ID == id {
				return
			}
		}
		scopes = append(scopes, authorityScope{Kind: kind, ID: id})
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return scopes
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return scopes
	}
	switch command {
	case "shows.create", "awards.create":
		add("organization", stringValue(payload["organization_id"]))
	case "shows.reset_schedule":
		a.expandShowScopes(&scopes, stringValue(payload["show_id"]))
	case "schedules.upsert", "judges.assign", "entries.create", "show_credits.create":
		a.expandShowScopes(&scopes, stringValue(payload["show_id"]))
	case "divisions.create":
		a.expandScheduleScopes(&scopes, stringValue(payload["show_schedule_id"]))
	case "sections.create":
		a.expandDivisionScopes(&scopes, stringValue(payload["division_id"]))
	case "classes.create":
		a.expandSectionScopes(&scopes, stringValue(payload["section_id"]))
	case "classes.update":
		a.expandClassScopes(&scopes, stringValue(payload["id"]))
	case "classes.reorder":
		a.expandClassScopes(&scopes, stringValue(payload["class_id"]))
	case "entries.update", "entries.delete", "entries.reassign_entrant":
		a.expandEntryScopes(&scopes, stringValue(payload["id"]))
	case "entries.move":
		a.expandEntryScopes(&scopes, stringValue(payload["id"]))
		a.expandClassScopes(&scopes, stringValue(payload["class_id"]))
	case "awards.compute":
		add("award", stringValue(payload["award_id"]))
	case "show_credits.delete":
		if credit, ok := a.store.showCreditByID(stringValue(payload["id"])); ok {
			a.expandShowScopes(&scopes, credit.ShowID)
			add("show_credit", credit.ID)
		}
	case "roles.assign":
		if showID := stringValue(payload["show_id"]); showID != "" {
			a.expandShowScopes(&scopes, showID)
		} else if orgID := stringValue(payload["organization_id"]); orgID != "" {
			add("organization", orgID)
		}
	case "clubs.invites.create":
		add("organization", stringValue(payload["organization_id"]))
	}
	return scopes
}

func (a *app) authorizeCommandRequest(w http.ResponseWriter, r *http.Request, command string) ([]byte, bool) {
	if a.isServiceToken(r) {
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		return body, true
	}
	body, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(body))
	scopes := a.commandScopesFromPayload(command, body)
	for _, scope := range a.authorityScopesForRequest(r) {
		exists := false
		for _, existing := range scopes {
			if existing.Kind == scope.Kind && existing.ID == scope.ID {
				exists = true
				break
			}
		}
		if !exists {
			scopes = append(scopes, scope)
		}
	}
	user, ok := a.currentUser(r)
	if ok && a.userHasCapability(r.Context(), *user, requiredCapabilityForCommand(command), scopes...) {
		return body, true
	}
	if token, ok := a.currentAgentToken(r); ok {
		permission := requiredCapabilityForCommand(command)
		if permission == "" || agentTokenHasPermission(token, permission) {
			return body, true
		}
		a.writeAPIError(w, r, http.StatusForbidden, "permission_denied", "This agent token does not grant that action.", fmt.Sprintf("This token currently grants: %s. Generate a new token from /account with %q if the agent needs it.", formatPermissionList(token.Permissions), agentPermissionLookup(permission).Label), []apiFieldError{
			{Field: "required_permission", Message: permission},
		})
		return nil, false
	}
	a.writeAPIError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required.", "Use an admin session, a Bearer service token, or an account-issued agent token with the required command permissions.", nil)
	return nil, false
}

func (a *app) expandShowScopes(scopes *[]authorityScope, showID string) {
	showID = strings.TrimSpace(showID)
	if showID == "" {
		return
	}
	appendScope(scopes, "show", showID)
	if show, ok := a.store.showByID(showID); ok {
		appendScope(scopes, "organization", show.OrganizationID)
	}
}

func (a *app) expandEntryScopes(scopes *[]authorityScope, entryID string) {
	entryID = strings.TrimSpace(entryID)
	if entryID == "" {
		return
	}
	appendScope(scopes, "entry", entryID)
	if entry, ok := a.store.entryByID(entryID); ok {
		a.expandShowScopes(scopes, entry.ShowID)
		appendScope(scopes, "class", entry.ClassID)
	}
}

func (a *app) expandClassScopes(scopes *[]authorityScope, classID string) {
	classID = strings.TrimSpace(classID)
	if classID == "" {
		return
	}
	appendScope(scopes, "class", classID)
	if classItem, ok := a.store.classByID(classID); ok {
		if section, ok := a.store.sectionByID(classItem.SectionID); ok {
			a.expandSectionScopes(scopes, section.ID)
		}
	}
}

func (a *app) expandSectionScopes(scopes *[]authorityScope, sectionID string) {
	sectionID = strings.TrimSpace(sectionID)
	if sectionID == "" {
		return
	}
	appendScope(scopes, "section", sectionID)
	if section, ok := a.store.sectionByID(sectionID); ok {
		a.expandDivisionScopes(scopes, section.DivisionID)
	}
}

func (a *app) expandDivisionScopes(scopes *[]authorityScope, divisionID string) {
	divisionID = strings.TrimSpace(divisionID)
	if divisionID == "" {
		return
	}
	appendScope(scopes, "division", divisionID)
	if division, ok := a.store.divisionByID(divisionID); ok {
		a.expandScheduleScopes(scopes, division.ShowScheduleID)
	}
}

func (a *app) expandScheduleScopes(scopes *[]authorityScope, scheduleID string) {
	scheduleID = strings.TrimSpace(scheduleID)
	if scheduleID == "" {
		return
	}
	appendScope(scopes, "schedule", scheduleID)
	if show, _ := a.showByScheduleID(scheduleID); show != nil {
		a.expandShowScopes(scopes, show.ID)
	}
}

func appendScope(scopes *[]authorityScope, kind, id string) {
	kind = strings.TrimSpace(kind)
	id = strings.TrimSpace(id)
	if kind == "" || id == "" {
		return
	}
	for _, existing := range *scopes {
		if existing.Kind == kind && existing.ID == id {
			return
		}
	}
	*scopes = append(*scopes, authorityScope{Kind: kind, ID: id})
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}
