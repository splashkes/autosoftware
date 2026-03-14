package interactions

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *RuntimeService) CreatePrincipal(ctx context.Context, input CreatePrincipalInput) (Principal, error) {
	pool, err := expectReady(s)
	if err != nil {
		return Principal{}, err
	}

	if strings.TrimSpace(input.Kind) == "" {
		return Principal{}, errors.New("kind is required")
	}
	if strings.TrimSpace(input.PrincipalID) == "" {
		input.PrincipalID = newID("prn")
	}
	input.Status = statusOrDefault(input.Status, "active")

	row := pool.QueryRow(ctx, `
		insert into runtime_principals (
		  principal_id, kind, display_name, status, profile
		)
		values ($1, $2, $3, $4, $5::jsonb)
		returning principal_id, kind, display_name, status, profile::text, created_at, deactivated_at
	`, input.PrincipalID, input.Kind, nullString(input.DisplayName), input.Status, jsonBytes(input.Profile))

	item, err := scanPrincipal(row)
	return item, wrapErr("create principal", err)
}

func (s *RuntimeService) CreatePrincipalIdentifier(ctx context.Context, input CreatePrincipalIdentifierInput) (PrincipalIdentifier, error) {
	pool, err := expectReady(s)
	if err != nil {
		return PrincipalIdentifier{}, err
	}

	if strings.TrimSpace(input.PrincipalID) == "" {
		return PrincipalIdentifier{}, errors.New("principal_id is required")
	}
	if strings.TrimSpace(input.IdentifierType) == "" {
		return PrincipalIdentifier{}, errors.New("identifier_type is required")
	}
	if strings.TrimSpace(input.Value) == "" {
		return PrincipalIdentifier{}, errors.New("value is required")
	}
	if strings.TrimSpace(input.IdentifierID) == "" {
		input.IdentifierID = newID("ident")
	}
	if strings.TrimSpace(input.NormalizedValue) == "" {
		input.NormalizedValue = normalizeIdentifier(input.Value)
	}

	row := pool.QueryRow(ctx, `
		insert into runtime_principal_identifiers (
		  identifier_id, principal_id, identifier_type, value, normalized_value,
		  is_primary, is_verified, verified_at, metadata
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb)
		returning identifier_id, principal_id, identifier_type, value, normalized_value,
		          is_primary, is_verified, verified_at, metadata::text, created_at
	`, input.IdentifierID, input.PrincipalID, input.IdentifierType, input.Value, input.NormalizedValue,
		input.IsPrimary, input.IsVerified, nullTimeValue(input.VerifiedAt), jsonBytes(input.Metadata))

	item, err := scanIdentifier(row)
	return item, wrapErr("create principal identifier", err)
}

func (s *RuntimeService) CreateSession(ctx context.Context, input CreateSessionInput) (Session, error) {
	pool, err := expectReady(s)
	if err != nil {
		return Session{}, err
	}

	if strings.TrimSpace(input.SessionID) == "" {
		input.SessionID = newID("sess")
	}
	input.Status = statusOrDefault(input.Status, "active")
	startedAt := s.nowUTC()
	if input.StartedAt != nil && !input.StartedAt.IsZero() {
		startedAt = input.StartedAt.UTC()
	}

	row := pool.QueryRow(ctx, `
		insert into runtime_sessions (
		  session_id, principal_id, status, auth_context, user_agent, ip_address, started_at, expires_at
		)
		values ($1, $2, $3, $4::jsonb, $5, $6, $7, $8)
		returning session_id, principal_id, status, auth_context::text, coalesce(user_agent, ''),
		          coalesce(ip_address, ''), started_at, last_seen_at, expires_at, ended_at
	`, input.SessionID, nullString(input.PrincipalID), input.Status, jsonBytes(input.AuthContext),
		nullString(input.UserAgent), nullString(input.IPAddress), startedAt, nullTimeValue(input.ExpiresAt))

	item, err := scanSession(row)
	return item, wrapErr("create session", err)
}

func (s *RuntimeService) GetSession(ctx context.Context, sessionID string) (Session, error) {
	pool, err := expectReady(s)
	if err != nil {
		return Session{}, err
	}

	row := pool.QueryRow(ctx, `
		select session_id, principal_id, status, auth_context::text, coalesce(user_agent, ''),
		       coalesce(ip_address, ''), started_at, last_seen_at, expires_at, ended_at
		from runtime_sessions
		where session_id = $1
	`, strings.TrimSpace(sessionID))

	item, err := scanSession(row)
	return item, wrapErr("get session", err)
}

func (s *RuntimeService) ResolveSession(ctx context.Context, sessionID string) (ResolvedSession, error) {
	item, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return ResolvedSession{}, err
	}

	now := s.nowUTC()
	if item.Status != "active" {
		return ResolvedSession{}, ErrNotFound
	}
	if item.ExpiresAt != nil && !item.ExpiresAt.After(now) {
		return ResolvedSession{}, ErrNotFound
	}
	if item.EndedAt != nil {
		return ResolvedSession{}, ErrNotFound
	}

	return ResolvedSession{
		SessionID:   item.SessionID,
		PrincipalID: item.PrincipalID,
		Status:      item.Status,
		AuthContext: item.AuthContext,
		ExpiresAt:   item.ExpiresAt,
	}, nil
}

func (s *RuntimeService) TouchSession(ctx context.Context, sessionID string) (Session, error) {
	pool, err := expectReady(s)
	if err != nil {
		return Session{}, err
	}

	row := pool.QueryRow(ctx, `
		update runtime_sessions
		set last_seen_at = $2
		where session_id = $1
		returning session_id, principal_id, status, auth_context::text, coalesce(user_agent, ''),
		          coalesce(ip_address, ''), started_at, last_seen_at, expires_at, ended_at
	`, strings.TrimSpace(sessionID), s.nowUTC())

	item, err := scanSession(row)
	return item, wrapErr("touch session", err)
}

func (s *RuntimeService) CreateAuthChallenge(ctx context.Context, input CreateAuthChallengeInput) (AuthChallengeIssue, error) {
	pool, err := expectReady(s)
	if err != nil {
		return AuthChallengeIssue{}, err
	}

	if strings.TrimSpace(input.ChallengeKind) == "" {
		return AuthChallengeIssue{}, errors.New("challenge_kind is required")
	}
	if strings.TrimSpace(input.ChallengeID) == "" {
		input.ChallengeID = newID("chal")
	}
	if strings.TrimSpace(input.Verifier) == "" {
		input.Verifier, err = newToken()
		if err != nil {
			return AuthChallengeIssue{}, err
		}
	}
	input.Status = statusOrDefault(input.Status, "pending")

	row := pool.QueryRow(ctx, `
		insert into runtime_auth_challenges (
		  challenge_id, challenge_kind, provider_id, principal_id, session_id,
		  delivery_target, verifier_hash, scope, status, expires_at, metadata
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9, $10, $11::jsonb)
		returning challenge_id, challenge_kind, provider_id, principal_id, session_id,
		          delivery_target, status, scope::text, expires_at, used_at,
		          metadata::text, created_at
	`, input.ChallengeID, input.ChallengeKind, nullString(input.ProviderID), nullString(input.PrincipalID),
		nullString(input.SessionID), nullString(input.DeliveryTarget), hashToken(input.Verifier),
		jsonBytes(input.Scope), input.Status, nullTimeValue(input.ExpiresAt), jsonBytes(input.Metadata))

	item, err := scanAuthChallenge(row)
	if err != nil {
		return AuthChallengeIssue{}, wrapErr("create auth challenge", err)
	}

	return AuthChallengeIssue{
		Challenge: item,
		Verifier:  input.Verifier,
	}, nil
}

func (s *RuntimeService) ConsumeAuthChallenge(ctx context.Context, challengeID, verifier string) (AuthChallenge, error) {
	pool, err := expectReady(s)
	if err != nil {
		return AuthChallenge{}, err
	}

	if strings.TrimSpace(challengeID) == "" || strings.TrimSpace(verifier) == "" {
		return AuthChallenge{}, errors.New("challenge_id and verifier are required")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return AuthChallenge{}, wrapErr("begin consume auth challenge", err)
	}
	defer tx.Rollback(ctx)

	var status string
	var verifierHash string
	var expiresAt *time.Time
	var usedAt *time.Time
	row := tx.QueryRow(ctx, `
		select status, verifier_hash, expires_at, used_at
		from runtime_auth_challenges
		where challenge_id = $1
		for update
	`, strings.TrimSpace(challengeID))

	var expiresAtNull sql.NullTime
	var usedAtNull sql.NullTime
	if err := row.Scan(&status, &verifierHash, &expiresAtNull, &usedAtNull); err != nil {
		return AuthChallenge{}, wrapErr("load auth challenge", rowNotFound(err))
	}
	expiresAt = toTimePtr(expiresAtNull)
	usedAt = toTimePtr(usedAtNull)

	now := s.nowUTC()
	if status != "pending" || usedAt != nil || (expiresAt != nil && !expiresAt.After(now)) {
		return AuthChallenge{}, ErrNotFound
	}
	if verifierHash != hashToken(verifier) {
		return AuthChallenge{}, ErrNotFound
	}

	item, err := scanAuthChallenge(tx.QueryRow(ctx, `
		update runtime_auth_challenges
		set status = 'used', used_at = $2
		where challenge_id = $1
		returning challenge_id, challenge_kind, provider_id, principal_id, session_id,
		          delivery_target, status, scope::text, expires_at, used_at,
		          metadata::text, created_at
	`, strings.TrimSpace(challengeID), now))
	if err != nil {
		return AuthChallenge{}, wrapErr("consume auth challenge", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return AuthChallenge{}, wrapErr("commit consume auth challenge", err)
	}
	return item, nil
}

func (s *RuntimeService) lookupChallenge(ctx context.Context, challengeID string) (AuthChallenge, error) {
	pool, err := expectReady(s)
	if err != nil {
		return AuthChallenge{}, err
	}
	row := pool.QueryRow(ctx, `
		select challenge_id, challenge_kind, provider_id, principal_id, session_id,
		       delivery_target, status, scope::text, expires_at, used_at,
		       metadata::text, created_at
		from runtime_auth_challenges
		where challenge_id = $1
	`, strings.TrimSpace(challengeID))
	return scanAuthChallenge(row)
}

func rowsToMessages(rows pgx.Rows) ([]Message, error) {
	defer rows.Close()
	var items []Message
	for rows.Next() {
		item, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
