package interactions

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	authorityRegistryReference     = "runtime-authority"
	authorityRegistrySeedID        = "kernel-runtime"
	authorityRegistryRealizationID = "authority"
	authorityGrantRegistryRowType  = "authority.grant.create"
)

type authorityGrantRegistryPayload struct {
	Grant   AuthorityGrant                    `json:"grant"`
	Bundle  AuthorityBundle                   `json:"bundle"`
	Grantor *authorityPrincipalBindingPayload `json:"grantor,omitempty"`
	Grantee *authorityPrincipalBindingPayload `json:"grantee,omitempty"`
}

type authorityPrincipalBindingPayload struct {
	PrincipalID     string                 `json:"principal_id"`
	DisplayName     string                 `json:"display_name,omitempty"`
	Profile         map[string]interface{} `json:"profile,omitempty"`
	ProviderID      string                 `json:"provider_id,omitempty"`
	ProviderSubject string                 `json:"provider_subject,omitempty"`
	Email           string                 `json:"email,omitempty"`
}

func (s *RuntimeService) MaterializeAuthorityState(ctx context.Context) (AuthorityMaterializationResult, error) {
	pool, err := expectReady(s)
	if err != nil {
		return AuthorityMaterializationResult{}, err
	}
	if err := s.migrateLegacyAuthorityGrantsToRegistry(ctx); err != nil {
		return AuthorityMaterializationResult{}, err
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AuthorityMaterializationResult{}, wrapErr("begin authority materialization tx", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := s.materializeAuthorityStateTx(ctx, tx)
	if err != nil {
		return AuthorityMaterializationResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AuthorityMaterializationResult{}, wrapErr("commit authority materialization", err)
	}
	return result, nil
}

func (s *RuntimeService) migrateLegacyAuthorityGrantsToRegistry(ctx context.Context) error {
	pool, err := expectReady(s)
	if err != nil {
		return err
	}
	var registryCount int
	if err := pool.QueryRow(ctx, `
		select count(*)
		from runtime_registry_rows
		where reference = $1 and seed_id = $2 and realization_id = $3 and row_type = $4
	`, authorityRegistryReference, authorityRegistrySeedID, authorityRegistryRealizationID, authorityGrantRegistryRowType).Scan(&registryCount); err != nil {
		return wrapErr("count legacy authority registry rows", err)
	}
	if registryCount > 0 {
		return nil
	}
	rows, err := pool.Query(ctx, `
		select grant_id, grantor_principal_id, grantee_principal_id, bundle_id, capabilities_snapshot,
		       scope_kind, scope_id, delegation_mode, basis, status, effective_at, expires_at,
		       supersedes_grant_id, reason, evidence_refs, metadata::text, created_at
		from runtime_authority_grants
		order by created_at asc, grant_id asc
	`)
	if err != nil {
		return wrapErr("load legacy authority grants", err)
	}
	defer rows.Close()

	appendInput := AppendRegistryChangeSetInput{
		ChangeSetID:    "authority-legacy-migration",
		Reference:      authorityRegistryReference,
		SeedID:         authorityRegistrySeedID,
		RealizationID:  authorityRegistryRealizationID,
		IdempotencyKey: "authority-legacy-migration",
		AcceptedBy:     "authority-legacy-migration",
		Metadata: map[string]interface{}{
			"source": "runtime_authority_grants",
		},
	}
	for rows.Next() {
		grant, err := scanAuthorityGrant(rows)
		if err != nil {
			return wrapErr("scan legacy authority grant", err)
		}
		bundle, err := s.GetAuthorityBundle(ctx, grant.BundleID)
		if err != nil {
			return wrapErr("load legacy authority bundle", err)
		}
		payload, err := s.authorityGrantRegistryPayload(ctx, grant, bundle)
		if err != nil {
			return err
		}
		appendInput.Rows = append(appendInput.Rows, AppendRegistryRowInput{
			RowType:  authorityGrantRegistryRowType,
			ObjectID: grant.GrantID,
			ClaimID:  grant.GrantID,
			Payload:  payload,
		})
	}
	if err := rows.Err(); err != nil {
		return wrapErr("iterate legacy authority grants", err)
	}
	if len(appendInput.Rows) == 0 {
		return nil
	}
	_, err = s.AppendRegistryChangeSet(ctx, appendInput)
	return wrapErr("append legacy authority grants to registry", err)
}

func (s *RuntimeService) ensureAuthorityMaterialized(ctx context.Context) error {
	pool, err := expectReady(s)
	if err != nil {
		return err
	}
	needs, err := s.authorityStateNeedsMaterialize(ctx, pool)
	if err != nil {
		return err
	}
	if !needs {
		return nil
	}
	_, err = s.MaterializeAuthorityState(ctx)
	return err
}

type authorityQueryer interface {
	registryQueryer
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (s *RuntimeService) authorityStateNeedsMaterialize(ctx context.Context, queryer authorityQueryer) (bool, error) {
	var projectedCount int
	if err := queryer.QueryRow(ctx, `select count(*) from runtime_authority_grants`).Scan(&projectedCount); err != nil {
		return false, wrapErr("count authority grants", err)
	}
	registryCount := 0
	var after int64
	for {
		rows, err := s.listRegistryRowsWithQuery(ctx, queryer, ListRegistryRowsInput{
			Reference:     authorityRegistryReference,
			SeedID:        authorityRegistrySeedID,
			RealizationID: authorityRegistryRealizationID,
			AfterRowID:    after,
			Limit:         5000,
		})
		if err != nil {
			return false, err
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			if strings.TrimSpace(row.RowType) == authorityGrantRegistryRowType {
				registryCount++
			}
		}
		after = rows[len(rows)-1].RowID
	}
	return projectedCount != registryCount, nil
}

func (s *RuntimeService) materializeAuthorityStateTx(ctx context.Context, tx pgx.Tx) (AuthorityMaterializationResult, error) {
	filtered := make([]RegistryRow, 0)
	var after int64
	for {
		rows, err := s.listRegistryRowsWithQuery(ctx, tx, ListRegistryRowsInput{
			Reference:     authorityRegistryReference,
			SeedID:        authorityRegistrySeedID,
			RealizationID: authorityRegistryRealizationID,
			AfterRowID:    after,
			Limit:         5000,
		})
		if err != nil {
			return AuthorityMaterializationResult{}, err
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			if strings.TrimSpace(row.RowType) == authorityGrantRegistryRowType {
				filtered = append(filtered, row)
			}
		}
		after = rows[len(rows)-1].RowID
	}

	if _, err := tx.Exec(ctx, `delete from runtime_authority_grants`); err != nil {
		return AuthorityMaterializationResult{}, wrapErr("clear authority grants", err)
	}

	for _, row := range filtered {
		var payload authorityGrantRegistryPayload
		if err := mapToAuthorityStruct(row.Payload, &payload); err != nil {
			return AuthorityMaterializationResult{}, wrapErr("decode authority registry payload", err)
		}
		if err := s.materializeAuthorityGrantPayload(ctx, tx, payload, row.AcceptedAt); err != nil {
			return AuthorityMaterializationResult{}, err
		}
	}
	return AuthorityMaterializationResult{
		Reference:        authorityRegistryReference,
		MaterializedRows: len(filtered),
		RegistryRows:     len(filtered),
		Rebuilt:          true,
	}, nil
}

func mapToAuthorityStruct(payload map[string]interface{}, target any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

func (s *RuntimeService) materializeAuthorityGrantPayload(ctx context.Context, tx pgx.Tx, payload authorityGrantRegistryPayload, acceptedAt time.Time) error {
	grant := payload.Grant
	if strings.TrimSpace(grant.GrantID) == "" {
		return fmt.Errorf("authority grant payload missing grant_id")
	}
	if strings.TrimSpace(grant.BundleID) == "" {
		return fmt.Errorf("authority grant payload missing bundle_id")
	}
	if strings.TrimSpace(grant.GranteePrincipalID) == "" {
		return fmt.Errorf("authority grant payload missing grantee_principal_id")
	}
	if strings.TrimSpace(grant.ScopeKind) == "" || strings.TrimSpace(grant.ScopeID) == "" {
		return fmt.Errorf("authority grant payload missing scope")
	}

	if err := upsertAuthorityBundleMaterialized(ctx, tx, payload.Bundle, grant); err != nil {
		return err
	}
	if err := upsertAuthorityPrincipalBinding(ctx, tx, payload.Grantor, grant.GrantorPrincipalID); err != nil {
		return err
	}
	if err := upsertAuthorityPrincipalBinding(ctx, tx, payload.Grantee, grant.GranteePrincipalID); err != nil {
		return err
	}

	createdAt := grant.CreatedAt
	if createdAt.IsZero() {
		createdAt = acceptedAt.UTC()
	}
	if _, err := tx.Exec(ctx, `
		insert into runtime_authority_grants (
		  grant_id, grantor_principal_id, grantee_principal_id, bundle_id, capabilities_snapshot,
		  scope_kind, scope_id, delegation_mode, basis, status, effective_at, expires_at,
		  supersedes_grant_id, reason, evidence_refs, metadata, created_at
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16::jsonb, $17)
		on conflict (grant_id) do update
		set grantor_principal_id = excluded.grantor_principal_id,
		    grantee_principal_id = excluded.grantee_principal_id,
		    bundle_id = excluded.bundle_id,
		    capabilities_snapshot = excluded.capabilities_snapshot,
		    scope_kind = excluded.scope_kind,
		    scope_id = excluded.scope_id,
		    delegation_mode = excluded.delegation_mode,
		    basis = excluded.basis,
		    status = excluded.status,
		    effective_at = excluded.effective_at,
		    expires_at = excluded.expires_at,
		    supersedes_grant_id = excluded.supersedes_grant_id,
		    reason = excluded.reason,
		    evidence_refs = excluded.evidence_refs,
		    metadata = excluded.metadata,
		    created_at = excluded.created_at
	`, grant.GrantID, nullString(grant.GrantorPrincipalID), grant.GranteePrincipalID, grant.BundleID, normalizedStringArray(grant.CapabilitiesSnapshot),
		grant.ScopeKind, grant.ScopeID, grant.DelegationMode, grant.Basis, grant.Status, nullTimeValue(grant.EffectiveAt),
		nullTimeValue(grant.ExpiresAt), nullString(grant.SupersedesGrantID), nullString(grant.Reason), normalizedStringArray(grant.EvidenceRefs),
		jsonBytes(grant.Metadata), createdAt); err != nil {
		return wrapErr("materialize authority grant", err)
	}
	return nil
}

func upsertAuthorityBundleMaterialized(ctx context.Context, tx pgx.Tx, bundle AuthorityBundle, grant AuthorityGrant) error {
	bundleID := strings.TrimSpace(bundle.BundleID)
	if bundleID == "" {
		bundleID = strings.TrimSpace(grant.BundleID)
	}
	if bundleID == "" {
		return nil
	}
	capabilities := normalizedStringArray(bundle.Capabilities)
	if len(capabilities) == 0 {
		capabilities = normalizedStringArray(grant.CapabilitiesSnapshot)
	}
	if _, err := tx.Exec(ctx, `
		insert into runtime_authority_bundles (
		  bundle_id, display_name, capabilities, status, metadata, created_at, updated_at
		)
		values ($1, $2, $3, 'active', $4::jsonb, now(), now())
		on conflict (bundle_id) do update
		set display_name = coalesce(excluded.display_name, runtime_authority_bundles.display_name),
		    capabilities = case when cardinality(excluded.capabilities) > 0 then excluded.capabilities else runtime_authority_bundles.capabilities end,
		    status = 'active',
		    metadata = excluded.metadata,
		    updated_at = now()
	`, bundleID, nullString(bundle.DisplayName), capabilities, jsonBytes(bundle.Metadata)); err != nil {
		return wrapErr("materialize authority bundle", err)
	}
	return nil
}

func normalizedStringArray(values []string) []string {
	normalized := normalizeStringList(values)
	if len(normalized) == 0 {
		return []string{}
	}
	return normalized
}

func upsertAuthorityPrincipalBinding(ctx context.Context, tx pgx.Tx, binding *authorityPrincipalBindingPayload, principalID string) error {
	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		return nil
	}
	displayName := principalID
	profile := map[string]interface{}{}
	if binding != nil {
		if strings.TrimSpace(binding.DisplayName) != "" {
			displayName = strings.TrimSpace(binding.DisplayName)
		}
		if len(binding.Profile) > 0 {
			profile = binding.Profile
		}
	}
	if _, err := tx.Exec(ctx, `
		insert into runtime_principals (
		  principal_id, kind, display_name, status, profile, created_at
		)
		values ($1, 'person', $2, 'active', $3::jsonb, now())
		on conflict (principal_id) do update
		set display_name = coalesce(excluded.display_name, runtime_principals.display_name),
		    status = 'active',
		    profile = case when excluded.profile = '{}'::jsonb then runtime_principals.profile else excluded.profile end
	`, principalID, nullString(displayName), jsonBytes(profile)); err != nil {
		return wrapErr("materialize authority principal", err)
	}
	if binding == nil {
		return nil
	}
	if email := normalizeIdentifier(strings.TrimSpace(binding.Email)); email != "" {
		if _, err := tx.Exec(ctx, `
			insert into runtime_principal_identifiers (
			  identifier_id, principal_id, identifier_type, value, normalized_value,
			  is_primary, is_verified, verified_at, metadata, created_at
			)
			values ($1, $2, 'email', $3, $4, true, true, now(), '{}'::jsonb, now())
			on conflict (identifier_type, normalized_value) do update
			set principal_id = excluded.principal_id,
			    value = excluded.value,
			    is_primary = true,
			    is_verified = true,
			    verified_at = excluded.verified_at
		`, newID("ident"), principalID, strings.TrimSpace(binding.Email), email); err != nil {
			return wrapErr("materialize authority email identifier", err)
		}
	}
	if strings.TrimSpace(binding.ProviderID) != "" && strings.TrimSpace(binding.ProviderSubject) != "" {
		if _, err := tx.Exec(ctx, `
			insert into runtime_auth_identities (
			  identity_id, provider_id, principal_id, provider_subject, profile, linked_at, last_seen_at
			)
			values ($1, $2, $3, $4, $5::jsonb, now(), now())
			on conflict (provider_id, provider_subject) do update
			set principal_id = excluded.principal_id,
			    profile = excluded.profile,
			    last_seen_at = excluded.last_seen_at
		`, newID("ident"), strings.TrimSpace(binding.ProviderID), principalID, strings.TrimSpace(binding.ProviderSubject), jsonBytes(profile)); err != nil {
			return wrapErr("materialize authority auth identity", err)
		}
	}
	return nil
}
