package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func validateAgentTokenIssue(input AgentTokenIssueInput) (AgentTokenIssueInput, error) {
	input.OwnerCognitoSub = strings.TrimSpace(input.OwnerCognitoSub)
	input.OwnerEmail = strings.TrimSpace(input.OwnerEmail)
	input.OwnerName = strings.TrimSpace(input.OwnerName)
	input.Label = strings.TrimSpace(input.Label)
	input.PermissionProfile = strings.TrimSpace(input.PermissionProfile)
	input.Permissions = normalizePermissions(input.Permissions)
	if input.OwnerCognitoSub == "" {
		return input, errors.New("owner subject required")
	}
	if input.Label == "" {
		return input, errors.New("token label required")
	}
	if input.PermissionProfile == "" {
		return input, errors.New("permission profile required")
	}
	if len(input.Permissions) == 0 {
		return input, errors.New("token must include at least one permission")
	}
	if input.ExpiresInDays < agentTokenMinDays || input.ExpiresInDays > agentTokenMaxDays {
		return input, fmt.Errorf("token expiry must be between %d and %d days", agentTokenMinDays, agentTokenMaxDays)
	}
	return input, nil
}

func (s *memoryStore) issueAgentToken(input AgentTokenIssueInput) (*IssuedAgentToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	input, err := validateAgentTokenIssue(input)
	if err != nil {
		return nil, err
	}

	secret, prefix, err := newAgentTokenSecret()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	token := &AgentToken{
		ID:                newID("agtok"),
		OwnerCognitoSub:   input.OwnerCognitoSub,
		OwnerEmail:        input.OwnerEmail,
		OwnerName:         input.OwnerName,
		Label:             input.Label,
		TokenPrefix:       prefix,
		PermissionProfile: input.PermissionProfile,
		Permissions:       append([]string(nil), input.Permissions...),
		CreatedAt:         now,
		ExpiresAt:         now.Add(time.Duration(input.ExpiresInDays) * 24 * time.Hour),
		TokenHash:         hashAgentTokenSecret(secret),
	}
	s.agentTokens[token.ID] = token
	s.agentTokenHash[token.TokenHash] = token.ID
	s.appendClaim(token.ID, "agent_token", "agent_token.issued", map[string]any{
		"id":                 token.ID,
		"owner_cognito_sub":  token.OwnerCognitoSub,
		"owner_email":        token.OwnerEmail,
		"label":              token.Label,
		"token_prefix":       token.TokenPrefix,
		"permission_profile": token.PermissionProfile,
		"permissions":        token.Permissions,
		"expires_at":         token.ExpiresAt,
	})
	return &IssuedAgentToken{
		Token:    cloneAgentToken(token),
		Secret:   secret,
		SecretID: prefix,
	}, nil
}

func (s *memoryStore) listAgentTokensBySubject(cognitoSub string) []*AgentToken {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*AgentToken
	for _, token := range s.agentTokens {
		if token.OwnerCognitoSub == cognitoSub {
			out = append(out, cloneAgentToken(token))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

func (s *memoryStore) revokeAgentToken(tokenID, ownerCognitoSub string) (*AgentToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	token, ok := s.agentTokens[tokenID]
	if !ok {
		return nil, errors.New("agent token not found")
	}
	if ownerCognitoSub != "" && token.OwnerCognitoSub != ownerCognitoSub {
		return nil, errors.New("agent token does not belong to this account")
	}
	if token.RevokedAt == nil {
		now := time.Now().UTC()
		token.RevokedAt = &now
		token.RevokedReason = "revoked_by_owner"
		s.appendClaim(token.ID, "agent_token", "agent_token.revoked", map[string]any{
			"id":             token.ID,
			"revoked_at":     now,
			"revoked_reason": token.RevokedReason,
		})
	}
	return cloneAgentToken(token), nil
}

func (s *memoryStore) authenticateAgentToken(raw string) (*AgentToken, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tokenID, ok := s.agentTokenHash[hashAgentTokenSecret(raw)]
	if !ok {
		return nil, false
	}
	token, ok := s.agentTokens[tokenID]
	if !ok {
		return nil, false
	}
	now := time.Now().UTC()
	if token.RevokedAt != nil || !token.ExpiresAt.After(now) {
		return nil, false
	}
	token.LastUsedAt = &now
	return cloneAgentToken(token), true
}

func (s *postgresFlowershowStore) issueAgentToken(input AgentTokenIssueInput) (*IssuedAgentToken, error) {
	input, err := validateAgentTokenIssue(input)
	if err != nil {
		return nil, err
	}

	secret, prefix, err := newAgentTokenSecret()
	if err != nil {
		return nil, err
	}
	token := &AgentToken{
		ID:                newID("agtok"),
		OwnerCognitoSub:   input.OwnerCognitoSub,
		OwnerEmail:        input.OwnerEmail,
		OwnerName:         input.OwnerName,
		Label:             input.Label,
		TokenPrefix:       prefix,
		PermissionProfile: input.PermissionProfile,
		Permissions:       append([]string(nil), input.Permissions...),
		CreatedAt:         time.Now().UTC(),
		ExpiresAt:         time.Now().UTC().Add(time.Duration(input.ExpiresInDays) * 24 * time.Hour),
		TokenHash:         hashAgentTokenSecret(secret),
	}

	perms, err := json.Marshal(token.Permissions)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = s.pool.Exec(ctx, `
		insert into as_flowershow_agent_tokens (
			id, owner_cognito_sub, owner_email, owner_name, label, token_prefix, token_hash,
			permission_profile, permissions, created_at, expires_at
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`, token.ID, token.OwnerCognitoSub, token.OwnerEmail, token.OwnerName, token.Label, token.TokenPrefix, token.TokenHash, token.PermissionProfile, perms, token.CreatedAt, token.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("issue agent token: %w", err)
	}
	return &IssuedAgentToken{
		Token:    cloneAgentToken(token),
		Secret:   secret,
		SecretID: prefix,
	}, nil
}

func (s *postgresFlowershowStore) listAgentTokensBySubject(cognitoSub string) []*AgentToken {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := s.pool.Query(ctx, `
		select id, owner_cognito_sub, owner_email, owner_name, label, token_prefix, permission_profile,
		       permissions, created_at, expires_at, last_used_at, revoked_at, revoked_reason
		  from as_flowershow_agent_tokens
		 where owner_cognito_sub = $1
		 order by created_at desc
	`, cognitoSub)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []*AgentToken
	for rows.Next() {
		token, err := scanAgentToken(rows)
		if err != nil {
			return nil
		}
		out = append(out, token)
	}
	return out
}

func (s *postgresFlowershowStore) revokeAgentToken(tokenID, ownerCognitoSub string) (*AgentToken, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	now := time.Now().UTC()
	cmd, err := s.pool.Exec(ctx, `
		update as_flowershow_agent_tokens
		   set revoked_at = coalesce(revoked_at, $3),
		       revoked_reason = case when revoked_at is null then 'revoked_by_owner' else revoked_reason end
		 where id = $1
		   and owner_cognito_sub = $2
	`, tokenID, ownerCognitoSub, now)
	if err != nil {
		return nil, fmt.Errorf("revoke agent token: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return nil, errors.New("agent token not found")
	}

	row := s.pool.QueryRow(ctx, `
		select id, owner_cognito_sub, owner_email, owner_name, label, token_prefix, permission_profile,
		       permissions, created_at, expires_at, last_used_at, revoked_at, revoked_reason
		  from as_flowershow_agent_tokens
		 where id = $1
	`, tokenID)
	token, err := scanAgentToken(row)
	if err != nil {
		return nil, fmt.Errorf("load revoked agent token: %w", err)
	}
	return token, nil
}

func (s *postgresFlowershowStore) authenticateAgentToken(raw string) (*AgentToken, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	row := s.pool.QueryRow(ctx, `
		select id, owner_cognito_sub, owner_email, owner_name, label, token_prefix, permission_profile,
		       permissions, created_at, expires_at, last_used_at, revoked_at, revoked_reason
		  from as_flowershow_agent_tokens
		 where token_hash = $1
	`, hashAgentTokenSecret(raw))
	token, err := scanAgentToken(row)
	if err != nil {
		return nil, false
	}
	now := time.Now().UTC()
	if token.RevokedAt != nil || !token.ExpiresAt.After(now) {
		return nil, false
	}
	if _, err := s.pool.Exec(ctx, `
		update as_flowershow_agent_tokens
		   set last_used_at = $2
		 where id = $1
	`, token.ID, now); err == nil {
		token.LastUsedAt = &now
	}
	return token, true
}

type agentTokenScanner interface {
	Scan(dest ...any) error
}

func scanAgentToken(row agentTokenScanner) (*AgentToken, error) {
	var (
		token       AgentToken
		permissions []byte
	)
	if err := row.Scan(
		&token.ID,
		&token.OwnerCognitoSub,
		&token.OwnerEmail,
		&token.OwnerName,
		&token.Label,
		&token.TokenPrefix,
		&token.PermissionProfile,
		&permissions,
		&token.CreatedAt,
		&token.ExpiresAt,
		&token.LastUsedAt,
		&token.RevokedAt,
		&token.RevokedReason,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		return nil, err
	}
	if len(permissions) > 0 {
		if err := json.Unmarshal(permissions, &token.Permissions); err != nil {
			return nil, err
		}
	}
	token.Permissions = normalizePermissions(token.Permissions)
	return &token, nil
}
