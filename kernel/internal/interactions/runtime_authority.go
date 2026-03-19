package interactions

import (
	"context"
	"errors"
	"sort"
	"strings"
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
	pool, err := expectReady(s)
	if err != nil {
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

	row := pool.QueryRow(ctx, `
		insert into runtime_authority_grants (
		  grant_id, grantor_principal_id, grantee_principal_id, bundle_id, capabilities_snapshot,
		  scope_kind, scope_id, delegation_mode, basis, status, effective_at, expires_at,
		  supersedes_grant_id, reason, evidence_refs, metadata
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16::jsonb)
		returning grant_id, grantor_principal_id, grantee_principal_id, bundle_id, capabilities_snapshot,
		          scope_kind, scope_id, delegation_mode, basis, status, effective_at, expires_at,
		          supersedes_grant_id, reason, evidence_refs, metadata::text, created_at
	`, input.GrantID, nullString(input.GrantorPrincipalID), input.GranteePrincipalID, input.BundleID, capabilitiesSnapshot,
		input.ScopeKind, input.ScopeID, delegationMode, basis, status, nullTimeValue(effectiveAt), nullTimeValue(input.ExpiresAt),
		nullString(input.SupersedesGrantID), nullString(input.Reason), normalizeStringList(input.EvidenceRefs), jsonBytes(input.Metadata))

	item, err := scanAuthorityGrant(row)
	return item, wrapErr("create authority grant", err)
}

func (s *RuntimeService) ListAuthorityLedgerByPrincipal(ctx context.Context, principalID string, limit int) ([]AuthorityGrant, error) {
	pool, err := expectReady(s)
	if err != nil {
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
