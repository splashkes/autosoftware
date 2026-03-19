package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func normalizeAuthIdentifier(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func nullableString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

type authStateStore interface {
	CreateUserSession(ctx context.Context, user UserIdentity, r *http.Request) (string, error)
	ResolveUserSession(ctx context.Context, sessionID string) (*UserIdentity, bool, error)
	EndUserSession(ctx context.Context, sessionID string) error
	CreatePendingAuth(ctx context.Context, pending pendingAuthState) (string, error)
	GetPendingAuth(ctx context.Context, pendingID string) (*pendingAuthState, bool, error)
	DeletePendingAuth(ctx context.Context, pendingID string) error
}

type runtimeAuthProviderDescriptor struct {
	ID       string
	Kind     string
	Issuer   string
	ClientID string
}

type runtimeAuthProviderReporter interface {
	RuntimeProvider() runtimeAuthProviderDescriptor
}

type memoryAuthStateStore struct {
	mu       sync.RWMutex
	sessions map[string]memoryUserSession
	pending  map[string]pendingAuthState
}

type memoryUserSession struct {
	User      UserIdentity
	ExpiresAt time.Time
	EndedAt   *time.Time
}

func newAuthStateStore(store flowershowStore, auth authProvider) authStateStore {
	if pg, ok := store.(*postgresFlowershowStore); ok && pg != nil && pg.pool != nil {
		return newPostgresAuthStateStore(pg.pool, auth)
	}
	return &memoryAuthStateStore{
		sessions: make(map[string]memoryUserSession),
		pending:  make(map[string]pendingAuthState),
	}
}

func (s *memoryAuthStateStore) CreateUserSession(_ context.Context, user UserIdentity, _ *http.Request) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sessionID := newID("sess")
	if strings.TrimSpace(user.SubjectID) == "" {
		user.SubjectID = newID("subj")
	}
	s.sessions[sessionID] = memoryUserSession{
		User:      user,
		ExpiresAt: time.Now().UTC().Add(authSessionDuration),
	}
	return sessionID, nil
}

func (s *memoryAuthStateStore) ResolveUserSession(_ context.Context, sessionID string) (*UserIdentity, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.sessions[strings.TrimSpace(sessionID)]
	if !ok {
		return nil, false, nil
	}
	now := time.Now().UTC()
	if item.EndedAt != nil || !item.ExpiresAt.After(now) {
		return nil, false, nil
	}
	user := item.User
	return &user, true, nil
}

func (s *memoryAuthStateStore) EndUserSession(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.sessions[strings.TrimSpace(sessionID)]
	if !ok {
		return nil
	}
	now := time.Now().UTC()
	item.EndedAt = &now
	s.sessions[strings.TrimSpace(sessionID)] = item
	return nil
}

func (s *memoryAuthStateStore) CreatePendingAuth(_ context.Context, pending pendingAuthState) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pendingID := newID("pend")
	s.pending[pendingID] = pending
	return pendingID, nil
}

func (s *memoryAuthStateStore) GetPendingAuth(_ context.Context, pendingID string) (*pendingAuthState, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.pending[strings.TrimSpace(pendingID)]
	if !ok {
		return nil, false, nil
	}
	if time.Now().UTC().Unix() >= item.ExpiresAt {
		return nil, false, nil
	}
	copy := item
	return &copy, true, nil
}

func (s *memoryAuthStateStore) DeletePendingAuth(_ context.Context, pendingID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pending, strings.TrimSpace(pendingID))
	return nil
}

type postgresAuthStateStore struct {
	pool     *pgxpool.Pool
	provider runtimeAuthProviderDescriptor
}

func newPostgresAuthStateStore(pool *pgxpool.Pool, auth authProvider) *postgresAuthStateStore {
	store := &postgresAuthStateStore{pool: pool}
	if reporter, ok := auth.(runtimeAuthProviderReporter); ok {
		store.provider = reporter.RuntimeProvider()
	}
	return store
}

func (s *postgresAuthStateStore) CreateUserSession(ctx context.Context, user UserIdentity, r *http.Request) (string, error) {
	if s == nil || s.pool == nil {
		return "", errors.New("runtime auth store unavailable")
	}

	principalID, err := s.ensurePrincipal(ctx, user)
	if err != nil {
		return "", err
	}
	user.SubjectID = principalID
	sessionID := newID("sess")
	authContext, err := json.Marshal(map[string]any{
		"subject_id":  principalID,
		"cognito_sub": user.CognitoSub,
		"email":       user.Email,
		"name":        user.Name,
		"provider_id": s.provider.ID,
		"seed_id":     "0007-Flowershow",
	})
	if err != nil {
		return "", err
	}

	var userAgent any
	if ua := strings.TrimSpace(r.UserAgent()); ua != "" {
		userAgent = ua
	}
	var ipAddress any
	if ip := requestIP(r); ip != "" {
		ipAddress = ip
	}
	expiresAt := time.Now().UTC().Add(authSessionDuration)
	if _, err := s.pool.Exec(ctx, `
		insert into runtime_sessions (
		  session_id, principal_id, status, auth_context, user_agent, ip_address, started_at, expires_at
		)
		values ($1, $2, 'active', $3::jsonb, $4, $5, now(), $6)
	`, sessionID, principalID, authContext, userAgent, ipAddress, expiresAt); err != nil {
		return "", err
	}
	return sessionID, nil
}

func (s *postgresAuthStateStore) ResolveUserSession(ctx context.Context, sessionID string) (*UserIdentity, bool, error) {
	if s == nil || s.pool == nil {
		return nil, false, errors.New("runtime auth store unavailable")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, false, nil
	}

	var status string
	var authContextRaw string
	var expiresAt *time.Time
	var endedAt *time.Time
	row := s.pool.QueryRow(ctx, `
		select status, auth_context::text, expires_at, ended_at
		from runtime_sessions
		where session_id = $1
	`, sessionID)
	if err := row.Scan(&status, &authContextRaw, &expiresAt, &endedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	now := time.Now().UTC()
	if status != "active" || endedAt != nil || (expiresAt != nil && !expiresAt.After(now)) {
		return nil, false, nil
	}

	var authContext struct {
		SubjectID  string `json:"subject_id"`
		CognitoSub string `json:"cognito_sub"`
		Email      string `json:"email"`
		Name       string `json:"name"`
	}
	if err := json.Unmarshal([]byte(authContextRaw), &authContext); err != nil {
		return nil, false, err
	}

	go s.touchSession(sessionID)

	return &UserIdentity{
		SubjectID:  authContext.SubjectID,
		CognitoSub: authContext.CognitoSub,
		Email:      authContext.Email,
		Name:       authContext.Name,
	}, true, nil
}

func (s *postgresAuthStateStore) EndUserSession(ctx context.Context, sessionID string) error {
	if s == nil || s.pool == nil {
		return nil
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		update runtime_sessions
		set status = 'ended', ended_at = now()
		where session_id = $1 and ended_at is null
	`, sessionID)
	return err
}

func (s *postgresAuthStateStore) CreatePendingAuth(ctx context.Context, pending pendingAuthState) (string, error) {
	if s == nil || s.pool == nil {
		return "", errors.New("runtime auth store unavailable")
	}
	pendingID := newID("pend")
	_, err := s.pool.Exec(ctx, `
		insert into as_flowershow_auth_pending (
		  pending_id, flow, email, cognito_session, expires_at, created_at
		)
		values ($1, $2, $3, $4, to_timestamp($5), now())
	`, pendingID, pending.Flow, pending.Email, pending.Session, pending.ExpiresAt)
	if err != nil {
		return "", err
	}
	return pendingID, nil
}

func (s *postgresAuthStateStore) GetPendingAuth(ctx context.Context, pendingID string) (*pendingAuthState, bool, error) {
	if s == nil || s.pool == nil {
		return nil, false, errors.New("runtime auth store unavailable")
	}
	pendingID = strings.TrimSpace(pendingID)
	if pendingID == "" {
		return nil, false, nil
	}
	var pending pendingAuthState
	var expiresAt time.Time
	row := s.pool.QueryRow(ctx, `
		select flow, email, coalesce(cognito_session, ''), expires_at
		from as_flowershow_auth_pending
		where pending_id = $1
	`, pendingID)
	if err := row.Scan(&pending.Flow, &pending.Email, &pending.Session, &expiresAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if !expiresAt.After(time.Now().UTC()) {
		return nil, false, nil
	}
	pending.ExpiresAt = expiresAt.UTC().Unix()
	return &pending, true, nil
}

func (s *postgresAuthStateStore) DeletePendingAuth(ctx context.Context, pendingID string) error {
	if s == nil || s.pool == nil {
		return nil
	}
	pendingID = strings.TrimSpace(pendingID)
	if pendingID == "" {
		return nil
	}
	_, err := s.pool.Exec(ctx, `delete from as_flowershow_auth_pending where pending_id = $1`, pendingID)
	return err
}

func (s *postgresAuthStateStore) ensurePrincipal(ctx context.Context, user UserIdentity) (string, error) {
	if s.provider.ID != "" && strings.TrimSpace(user.CognitoSub) != "" {
		var principalID string
		row := s.pool.QueryRow(ctx, `
			select principal_id
			from runtime_auth_identities
			where provider_id = $1 and provider_subject = $2
		`, s.provider.ID, strings.TrimSpace(user.CognitoSub))
		if err := row.Scan(&principalID); err == nil {
			if err := s.syncPrincipalMetadata(ctx, principalID, user); err != nil {
				return "", err
			}
			return principalID, nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return "", err
		}
	}

	if email := normalizeAuthIdentifier(user.Email); email != "" {
		var principalID string
		row := s.pool.QueryRow(ctx, `
			select principal_id
			from runtime_principal_identifiers
			where identifier_type = 'email' and normalized_value = $1
			order by is_primary desc, is_verified desc, created_at asc
			limit 1
		`, email)
		if err := row.Scan(&principalID); err == nil {
			if err := s.syncPrincipalMetadata(ctx, principalID, user); err != nil {
				return "", err
			}
			return principalID, nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return "", err
		}
	}

	principalID := newID("prn")
	displayName := strings.TrimSpace(user.Name)
	if displayName == "" {
		displayName = strings.TrimSpace(user.Email)
	}
	profile, err := json.Marshal(map[string]any{
		"email":       user.Email,
		"name":        user.Name,
		"cognito_sub": user.CognitoSub,
	})
	if err != nil {
		return "", err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	if s.provider.ID != "" {
		issuer := nullableString(s.provider.Issuer)
		clientID := nullableString(s.provider.ClientID)
		if _, err := tx.Exec(ctx, `
			insert into runtime_auth_providers (
			  provider_id, kind, issuer, client_id, status, config, created_at
			)
			values ($1, $2, $3, $4, 'active', '{}'::jsonb, now())
			on conflict (provider_id) do update
			set kind = excluded.kind,
			    issuer = excluded.issuer,
			    client_id = excluded.client_id,
			    status = 'active'
		`, s.provider.ID, strings.TrimSpace(s.provider.Kind), issuer, clientID); err != nil {
			return "", err
		}
	}

	if _, err := tx.Exec(ctx, `
		insert into runtime_principals (
		  principal_id, kind, display_name, status, profile, created_at
		)
		values ($1, 'person', $2, 'active', $3::jsonb, now())
	`, principalID, nullableString(displayName), profile); err != nil {
		return "", err
	}

	if email := normalizeAuthIdentifier(user.Email); email != "" {
		if _, err := tx.Exec(ctx, `
			insert into runtime_principal_identifiers (
			  identifier_id, principal_id, identifier_type, value, normalized_value,
			  is_primary, is_verified, verified_at, metadata, created_at
			)
			values ($1, $2, 'email', $3, $4, true, true, now(), '{}'::jsonb, now())
			on conflict (identifier_type, normalized_value) do nothing
		`, newID("ident"), principalID, strings.TrimSpace(user.Email), email); err != nil {
			return "", err
		}
	}

	if s.provider.ID != "" && strings.TrimSpace(user.CognitoSub) != "" {
		if _, err := tx.Exec(ctx, `
			insert into runtime_auth_identities (
			  identity_id, provider_id, principal_id, provider_subject, profile, linked_at, last_seen_at
			)
			values ($1, $2, $3, $4, $5::jsonb, now(), now())
			on conflict (provider_id, provider_subject) do update
			set principal_id = excluded.principal_id,
			    profile = excluded.profile,
			    last_seen_at = excluded.last_seen_at
		`, newID("ident"), s.provider.ID, principalID, strings.TrimSpace(user.CognitoSub), profile); err != nil {
			return "", err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return principalID, nil
}

func (s *postgresAuthStateStore) syncPrincipalMetadata(ctx context.Context, principalID string, user UserIdentity) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	displayName := strings.TrimSpace(user.Name)
	if displayName == "" {
		displayName = strings.TrimSpace(user.Email)
	}
	profile, err := json.Marshal(map[string]any{
		"email":       user.Email,
		"name":        user.Name,
		"cognito_sub": user.CognitoSub,
	})
	if err != nil {
		return err
	}

	if displayName != "" {
		if _, err := tx.Exec(ctx, `
			update runtime_principals
			set display_name = $2,
			    profile = $3::jsonb
			where principal_id = $1
		`, principalID, displayName, profile); err != nil {
			return err
		}
	}

	if email := normalizeAuthIdentifier(user.Email); email != "" {
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
		`, newID("ident"), principalID, strings.TrimSpace(user.Email), email); err != nil {
			return err
		}
	}

	if s.provider.ID != "" && strings.TrimSpace(user.CognitoSub) != "" {
		if _, err := tx.Exec(ctx, `
			insert into runtime_auth_identities (
			  identity_id, provider_id, principal_id, provider_subject, profile, linked_at, last_seen_at
			)
			values ($1, $2, $3, $4, $5::jsonb, now(), now())
			on conflict (provider_id, provider_subject) do update
			set principal_id = excluded.principal_id,
			    profile = excluded.profile,
			    last_seen_at = excluded.last_seen_at
		`, newID("ident"), s.provider.ID, principalID, strings.TrimSpace(user.CognitoSub), profile); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *postgresAuthStateStore) touchSession(sessionID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = s.pool.Exec(ctx, `
		update runtime_sessions
		set last_seen_at = now()
		where session_id = $1 and status = 'active'
	`, sessionID)
}

func requestIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0])
	if forwarded != "" {
		return forwarded
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
