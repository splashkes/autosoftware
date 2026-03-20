package interactions

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

var authorityGrantStatuses = map[string]struct{}{
	"proposed":   {},
	"accepted":   {},
	"rejected":   {},
	"revoked":    {},
	"expired":    {},
	"superseded": {},
}

func (s *RuntimeService) BindAuthIdentity(ctx context.Context, input BindAuthIdentityInput) (AuthIdentity, error) {
	pool, err := expectReady(s)
	if err != nil {
		return AuthIdentity{}, err
	}

	input.ProviderID = strings.TrimSpace(input.ProviderID)
	input.PrincipalID = strings.TrimSpace(input.PrincipalID)
	input.ProviderSubject = strings.TrimSpace(input.ProviderSubject)
	if input.ProviderID == "" {
		return AuthIdentity{}, errors.New("provider_id is required")
	}
	if input.PrincipalID == "" {
		return AuthIdentity{}, errors.New("principal_id is required")
	}
	if input.ProviderSubject == "" {
		return AuthIdentity{}, errors.New("provider_subject is required")
	}
	if strings.TrimSpace(input.IdentityID) == "" {
		input.IdentityID = newID("idn")
	}

	row := pool.QueryRow(ctx, `
		insert into runtime_auth_identities (
		  identity_id, provider_id, principal_id, provider_subject, profile, linked_at, last_seen_at
		)
		values ($1, $2, $3, $4, $5::jsonb, $6, $7)
		on conflict (provider_id, provider_subject)
		do update set
		  principal_id = excluded.principal_id,
		  profile = excluded.profile,
		  last_seen_at = coalesce(excluded.last_seen_at, runtime_auth_identities.last_seen_at)
		returning identity_id, provider_id, principal_id, provider_subject, profile::text, linked_at, last_seen_at
	`, input.IdentityID, input.ProviderID, input.PrincipalID, input.ProviderSubject, jsonBytes(input.Profile), s.nowUTC(), nullTimeValue(input.LastSeenAt))

	item, err := scanAuthIdentity(row)
	return item, wrapErr("bind auth identity", err)
}

func (s *RuntimeService) ResolveAuthIdentity(ctx context.Context, providerID, providerSubject string) (AuthIdentity, error) {
	pool, err := expectReady(s)
	if err != nil {
		return AuthIdentity{}, err
	}

	row := pool.QueryRow(ctx, `
		select identity_id, provider_id, principal_id, provider_subject, profile::text, linked_at, last_seen_at
		from runtime_auth_identities
		where provider_id = $1 and provider_subject = $2
	`, strings.TrimSpace(providerID), strings.TrimSpace(providerSubject))

	item, err := scanAuthIdentity(row)
	return item, wrapErr("resolve auth identity", err)
}

func (s *RuntimeService) UpsertAuthorityBundle(ctx context.Context, input UpsertAuthorityBundleInput) (AuthorityBundle, error) {
	pool, err := expectReady(s)
	if err != nil {
		return AuthorityBundle{}, err
	}

	input.BundleID = strings.TrimSpace(input.BundleID)
	if input.BundleID == "" {
		return AuthorityBundle{}, errors.New("bundle_id is required")
	}
	capabilities := normalizeStringList(input.Capabilities)
	if len(capabilities) == 0 {
		return AuthorityBundle{}, errors.New("capabilities are required")
	}
	status := statusOrDefault(input.Status, "active")
	now := s.nowUTC()

	row := pool.QueryRow(ctx, `
		insert into runtime_authority_bundles (
		  bundle_id, display_name, capabilities, status, metadata, created_at, updated_at
		)
		values ($1, $2, $3, $4, $5::jsonb, $6, $7)
		on conflict (bundle_id)
		do update set
		  display_name = excluded.display_name,
		  capabilities = excluded.capabilities,
		  status = excluded.status,
		  metadata = excluded.metadata,
		  updated_at = excluded.updated_at
		returning bundle_id, display_name, capabilities, status, metadata::text, created_at, updated_at, retired_at
	`, input.BundleID, nullString(input.DisplayName), capabilities, status, jsonBytes(input.Metadata), now, now)

	item, err := scanAuthorityBundle(row)
	return item, wrapErr("upsert authority bundle", err)
}

func (s *RuntimeService) GetAuthorityBundle(ctx context.Context, bundleID string) (AuthorityBundle, error) {
	pool, err := expectReady(s)
	if err != nil {
		return AuthorityBundle{}, err
	}

	row := pool.QueryRow(ctx, `
		select bundle_id, display_name, capabilities, status, metadata::text, created_at, updated_at, retired_at
		from runtime_authority_bundles
		where bundle_id = $1
	`, strings.TrimSpace(bundleID))

	item, err := scanAuthorityBundle(row)
	return item, wrapErr("get authority bundle", err)
}

func (s *RuntimeService) CreateAuthorityGrant(ctx context.Context, input CreateAuthorityGrantInput) (AuthorityGrant, error) {
	if err := s.ensureAuthorityMaterialized(ctx); err != nil {
		return AuthorityGrant{}, err
	}
	if strings.TrimSpace(input.GrantID) == "" {
		input.GrantID = newID("grant")
	}
	status := statusOrDefault(input.Status, "accepted")
	if _, ok := authorityGrantStatuses[status]; !ok {
		return AuthorityGrant{}, errors.New("status must be proposed, accepted, rejected, revoked, expired, or superseded")
	}
	input.SupersedesGrantID = strings.TrimSpace(input.SupersedesGrantID)

	var superseded AuthorityGrant
	if input.SupersedesGrantID != "" {
		var err error
		superseded, err = s.getAuthorityGrant(ctx, input.SupersedesGrantID)
		if err != nil {
			return AuthorityGrant{}, err
		}
		if strings.TrimSpace(input.GranteePrincipalID) == "" {
			input.GranteePrincipalID = superseded.GranteePrincipalID
		}
		if strings.TrimSpace(input.BundleID) == "" {
			input.BundleID = superseded.BundleID
		}
		if strings.TrimSpace(input.ScopeKind) == "" {
			input.ScopeKind = superseded.ScopeKind
		}
		if strings.TrimSpace(input.ScopeID) == "" {
			input.ScopeID = superseded.ScopeID
		}
		if strings.TrimSpace(input.DelegationMode) == "" {
			input.DelegationMode = superseded.DelegationMode
		}
		if strings.TrimSpace(input.Basis) == "" {
			input.Basis = superseded.Basis
		}
	}

	input.GranteePrincipalID = strings.TrimSpace(input.GranteePrincipalID)
	input.BundleID = strings.TrimSpace(input.BundleID)
	input.ScopeKind = strings.TrimSpace(input.ScopeKind)
	input.ScopeID = strings.TrimSpace(input.ScopeID)
	if input.GranteePrincipalID == "" {
		return AuthorityGrant{}, errors.New("grantee_principal_id is required")
	}
	if input.BundleID == "" {
		return AuthorityGrant{}, errors.New("bundle_id is required")
	}
	if input.ScopeKind == "" || input.ScopeID == "" {
		return AuthorityGrant{}, errors.New("scope_kind and scope_id are required")
	}

	bundle, err := s.GetAuthorityBundle(ctx, input.BundleID)
	if err != nil {
		return AuthorityGrant{}, err
	}

	capabilitiesSnapshot := normalizeStringList(bundle.Capabilities)
	if len(superseded.CapabilitiesSnapshot) > 0 && (status == "revoked" || status == "superseded") {
		capabilitiesSnapshot = normalizeStringList(superseded.CapabilitiesSnapshot)
	}
	if len(capabilitiesSnapshot) == 0 {
		return AuthorityGrant{}, errors.New("authority bundle resolved to no capabilities")
	}

	delegationMode := statusOrDefault(input.DelegationMode, "none")
	basis := statusOrDefault(input.Basis, "delegated")
	effectiveAt := input.EffectiveAt
	if effectiveAt == nil && status == "accepted" {
		now := s.nowUTC()
		effectiveAt = &now
	}
	grant := AuthorityGrant{
		GrantID:              input.GrantID,
		GrantorPrincipalID:   strings.TrimSpace(input.GrantorPrincipalID),
		GranteePrincipalID:   input.GranteePrincipalID,
		BundleID:             input.BundleID,
		CapabilitiesSnapshot: capabilitiesSnapshot,
		ScopeKind:            input.ScopeKind,
		ScopeID:              input.ScopeID,
		DelegationMode:       delegationMode,
		Basis:                basis,
		Status:               status,
		EffectiveAt:          effectiveAt,
		ExpiresAt:            input.ExpiresAt,
		SupersedesGrantID:    input.SupersedesGrantID,
		Reason:               strings.TrimSpace(input.Reason),
		EvidenceRefs:         normalizeStringList(input.EvidenceRefs),
		Metadata:             input.Metadata,
		CreatedAt:            s.nowUTC(),
	}
	payload, err := s.authorityGrantRegistryPayload(ctx, grant, bundle)
	if err != nil {
		return AuthorityGrant{}, err
	}
	_, err = s.AppendRegistryChangeSet(ctx, AppendRegistryChangeSetInput{
		ChangeSetID:    "auth-" + grant.GrantID,
		Reference:      authorityRegistryReference,
		SeedID:         authorityRegistrySeedID,
		RealizationID:  authorityRegistryRealizationID,
		IdempotencyKey: "auth-" + grant.GrantID,
		AcceptedBy:     statusOrDefault(strings.TrimSpace(input.GrantorPrincipalID), "system"),
		Metadata: map[string]interface{}{
			"domain": "authority",
		},
		Rows: []AppendRegistryRowInput{{
			RowType:  authorityGrantRegistryRowType,
			ObjectID: grant.GrantID,
			ClaimID:  grant.GrantID,
			Payload:  payload,
		}},
	})
	if err != nil {
		return AuthorityGrant{}, wrapErr("append authority registry change set", err)
	}
	if _, err := s.MaterializeAuthorityState(ctx); err != nil {
		return AuthorityGrant{}, err
	}
	return s.getAuthorityGrant(ctx, grant.GrantID)
}

func (s *RuntimeService) ListAuthorityLedgerByPrincipal(ctx context.Context, principalID string, limit int) ([]AuthorityGrant, error) {
	pool, err := expectReady(s)
	if err != nil {
		return nil, err
	}
	if err := s.ensureAuthorityMaterialized(ctx); err != nil {
		return nil, err
	}

	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		return nil, errors.New("principal_id is required")
	}
	limit = clampLimit(limit, 100, 500)

	rows, err := pool.Query(ctx, `
		select grant_id, grantor_principal_id, grantee_principal_id, bundle_id, capabilities_snapshot,
		       scope_kind, scope_id, delegation_mode, basis, status, effective_at, expires_at,
		       supersedes_grant_id, reason, evidence_refs, metadata::text, created_at
		from runtime_authority_grants
		where grantee_principal_id = $1 or grantor_principal_id = $1
		order by created_at desc, grant_id desc
		limit $2
	`, principalID, limit)
	if err != nil {
		return nil, wrapErr("list authority ledger", err)
	}
	defer rows.Close()

	items := make([]AuthorityGrant, 0, limit)
	for rows.Next() {
		item, err := scanAuthorityGrant(rows)
		if err != nil {
			return nil, wrapErr("scan authority ledger", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapErr("iterate authority ledger", err)
	}
	return items, nil
}

func (s *RuntimeService) GetEffectiveAuthorityByPrincipal(ctx context.Context, principalID string) (PrincipalEffectiveAuthority, error) {
	pool, err := expectReady(s)
	if err != nil {
		return PrincipalEffectiveAuthority{}, err
	}
	if err := s.ensureAuthorityMaterialized(ctx); err != nil {
		return PrincipalEffectiveAuthority{}, err
	}

	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		return PrincipalEffectiveAuthority{}, errors.New("principal_id is required")
	}
	now := s.nowUTC()

	rows, err := pool.Query(ctx, `
		select g.grant_id, g.grantor_principal_id, g.grantee_principal_id, g.bundle_id, g.capabilities_snapshot,
		       g.scope_kind, g.scope_id, g.delegation_mode, g.basis, g.status, g.effective_at, g.expires_at,
		       g.supersedes_grant_id, g.reason, g.evidence_refs, g.metadata::text, g.created_at
		from runtime_authority_grants g
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
		return PrincipalEffectiveAuthority{}, wrapErr("get effective authority", err)
	}
	defer rows.Close()

	var grants []AuthorityGrant
	type capKey struct {
		capability string
		scopeKind  string
		scopeID    string
	}
	capabilityMap := map[capKey]*EffectiveAuthorityCapability{}
	for rows.Next() {
		item, err := scanAuthorityGrant(rows)
		if err != nil {
			return PrincipalEffectiveAuthority{}, wrapErr("scan effective authority", err)
		}
		grants = append(grants, item)
		for _, capability := range normalizeStringList(item.CapabilitiesSnapshot) {
			key := capKey{
				capability: capability,
				scopeKind:  item.ScopeKind,
				scopeID:    item.ScopeID,
			}
			existing, ok := capabilityMap[key]
			if !ok {
				existing = &EffectiveAuthorityCapability{
					Capability: capability,
					ScopeKind:  item.ScopeKind,
					ScopeID:    item.ScopeID,
				}
				capabilityMap[key] = existing
			}
			existing.GrantIDs = append(existing.GrantIDs, item.GrantID)
		}
	}
	if err := rows.Err(); err != nil {
		return PrincipalEffectiveAuthority{}, wrapErr("iterate effective authority", err)
	}

	policies := make([]EffectiveAuthorityCapability, 0, len(capabilityMap))
	for _, item := range capabilityMap {
		item.GrantIDs = normalizeStringList(item.GrantIDs)
		policies = append(policies, *item)
	}
	sort.Slice(policies, func(i, j int) bool {
		if policies[i].Capability != policies[j].Capability {
			return policies[i].Capability < policies[j].Capability
		}
		if policies[i].ScopeKind != policies[j].ScopeKind {
			return policies[i].ScopeKind < policies[j].ScopeKind
		}
		return policies[i].ScopeID < policies[j].ScopeID
	})

	return PrincipalEffectiveAuthority{
		PrincipalID:       principalID,
		ComputedAt:        now,
		ActiveGrants:      grants,
		EffectivePolicies: policies,
	}, nil
}

func (s *RuntimeService) getAuthorityGrant(ctx context.Context, grantID string) (AuthorityGrant, error) {
	pool, err := expectReady(s)
	if err != nil {
		return AuthorityGrant{}, err
	}

	row := pool.QueryRow(ctx, `
		select grant_id, grantor_principal_id, grantee_principal_id, bundle_id, capabilities_snapshot,
		       scope_kind, scope_id, delegation_mode, basis, status, effective_at, expires_at,
		       supersedes_grant_id, reason, evidence_refs, metadata::text, created_at
		from runtime_authority_grants
		where grant_id = $1
	`, strings.TrimSpace(grantID))

	item, err := scanAuthorityGrant(row)
	return item, wrapErr("get authority grant", err)
}

func (s *RuntimeService) authorityGrantRegistryPayload(ctx context.Context, grant AuthorityGrant, bundle AuthorityBundle) (map[string]interface{}, error) {
	payload := authorityGrantRegistryPayload{
		Grant: grant,
		Bundle: AuthorityBundle{
			BundleID:     bundle.BundleID,
			DisplayName:  bundle.DisplayName,
			Capabilities: normalizeStringList(bundle.Capabilities),
			Status:       bundle.Status,
			Metadata:     bundle.Metadata,
		},
	}
	var err error
	payload.Grantor, err = s.loadAuthorityPrincipalBinding(ctx, grant.GrantorPrincipalID)
	if err != nil {
		return nil, err
	}
	payload.Grantee, err = s.loadAuthorityPrincipalBinding(ctx, grant.GranteePrincipalID)
	if err != nil {
		return nil, err
	}
	return authorityStructToMap(payload)
}

func (s *RuntimeService) loadAuthorityPrincipalBinding(ctx context.Context, principalID string) (*authorityPrincipalBindingPayload, error) {
	pool, err := expectReady(s)
	if err != nil {
		return nil, err
	}
	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		return nil, nil
	}
	binding := &authorityPrincipalBindingPayload{
		PrincipalID: principalID,
		Profile:     map[string]interface{}{},
	}
	var profileText string
	err = pool.QueryRow(ctx, `
		select coalesce(display_name, ''), profile::text
		from runtime_principals
		where principal_id = $1
	`, principalID).Scan(&binding.DisplayName, &profileText)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, wrapErr("load authority principal", err)
	}
	if err == nil {
		binding.Profile = parseJSON(profileText)
		if email := strings.TrimSpace(stringValue(binding.Profile["email"])); email != "" {
			binding.Email = email
		}
	}
	err = pool.QueryRow(ctx, `
		select provider_id, provider_subject, profile::text
		from runtime_auth_identities
		where principal_id = $1
		order by last_seen_at desc nulls last, linked_at desc
		limit 1
	`, principalID).Scan(&binding.ProviderID, &binding.ProviderSubject, &profileText)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, wrapErr("load authority auth identity", err)
	}
	if err == nil {
		profile := parseJSON(profileText)
		if len(profile) > 0 {
			binding.Profile = profile
		}
		if email := strings.TrimSpace(stringValue(profile["email"])); email != "" {
			binding.Email = email
		}
	}
	if binding.Email == "" {
		_ = pool.QueryRow(ctx, `
			select value
			from runtime_principal_identifiers
			where principal_id = $1 and identifier_type = 'email'
			order by is_primary desc, is_verified desc, created_at asc
			limit 1
		`, principalID).Scan(&binding.Email)
	}
	if binding.DisplayName == "" {
		binding.DisplayName = principalID
	}
	return binding, nil
}

func stringValue(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

func authorityStructToMap(value any) (map[string]interface{}, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}
