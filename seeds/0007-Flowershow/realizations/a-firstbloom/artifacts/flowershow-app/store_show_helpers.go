package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	// defaultShowHelperInviteDays is the default lifetime for a freshly
	// created share link. Admins can override per-invite via the create input.
	defaultShowHelperInviteDays = 7

	// defaultShowBadgeSessionHours is the default lifetime for a redeemed
	// badge session. Twenty-four hours covers an evening of intake or a one
	// day fair without surprise re-redeems.
	defaultShowBadgeSessionHours = 24
)

// IssuedShowHelperInvite carries the plaintext token surfaced ONCE at
// creation. The plaintext is never persisted; only the sha256 hash is stored.
type IssuedShowHelperInvite struct {
	Invite *ShowHelperInvite `json:"invite"`
	Token  string            `json:"token"`
}

// IssuedShowBadgeSession carries the plaintext session token surfaced ONCE
// when the badge session is created. The plaintext is set as the cookie value;
// only the sha256 hash is stored.
type IssuedShowBadgeSession struct {
	Session *ShowBadgeSession `json:"session"`
	Token   string            `json:"token"`
}

func newShowHelperInviteToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "fshi_" + base64.RawURLEncoding.EncodeToString(buf), nil
}

func newShowBadgeSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "fsbs_" + base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashShareSecret(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func cloneShowHelperInvite(in *ShowHelperInvite) *ShowHelperInvite {
	if in == nil {
		return nil
	}
	out := *in
	if in.RevokedAt != nil {
		t := *in.RevokedAt
		out.RevokedAt = &t
	}
	// never propagate plaintext token through clones
	out.Token = ""
	return &out
}

func cloneShowBadgeSession(in *ShowBadgeSession) *ShowBadgeSession {
	if in == nil {
		return nil
	}
	out := *in
	if in.RevokedAt != nil {
		t := *in.RevokedAt
		out.RevokedAt = &t
	}
	out.SessionToken = ""
	return &out
}

func validateShowHelperInviteInput(input ShowHelperInviteInput) ShowHelperInviteInput {
	input.ShowID = strings.TrimSpace(input.ShowID)
	input.Label = strings.TrimSpace(input.Label)
	input.CreatedBy = strings.TrimSpace(input.CreatedBy)
	if input.ExpiresInDays <= 0 {
		input.ExpiresInDays = defaultShowHelperInviteDays
	}
	if input.ExpiresInDays > 60 {
		input.ExpiresInDays = 60
	}
	return input
}

func validateShowBadgeSessionInput(input ShowBadgeSessionInput) ShowBadgeSessionInput {
	input.ShowID = strings.TrimSpace(input.ShowID)
	input.InviteID = strings.TrimSpace(input.InviteID)
	input.Name = strings.TrimSpace(input.Name)
	input.Email = strings.TrimSpace(input.Email)
	input.MatchedPersonID = strings.TrimSpace(input.MatchedPersonID)
	if input.ExpiresInHours <= 0 {
		input.ExpiresInHours = defaultShowBadgeSessionHours
	}
	if input.ExpiresInHours > 24*7 {
		input.ExpiresInHours = 24 * 7
	}
	return input
}

// --- memoryStore methods ---

func (s *memoryStore) createShowHelperInvite(input ShowHelperInviteInput) (*IssuedShowHelperInvite, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	input = validateShowHelperInviteInput(input)
	if input.ShowID == "" {
		return nil, errors.New("show_id is required")
	}
	if _, ok := s.shows[input.ShowID]; !ok {
		return nil, errors.New("show not found")
	}

	plaintext, err := newShowHelperInviteToken()
	if err != nil {
		return nil, fmt.Errorf("generate share token: %w", err)
	}
	now := time.Now().UTC()
	item := &ShowHelperInvite{
		ID:        newID("shvinv"),
		ShowID:    input.ShowID,
		TokenHash: hashShareSecret(plaintext),
		Label:     input.Label,
		CreatedBy: input.CreatedBy,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Duration(input.ExpiresInDays) * 24 * time.Hour),
	}
	s.showHelperInvites[item.ID] = item
	s.appendClaim(item.ID, "show_helper_invite", "show_helper_invite.created", item)

	out := cloneShowHelperInvite(item)
	return &IssuedShowHelperInvite{Invite: out, Token: plaintext}, nil
}

func (s *memoryStore) findShowHelperInviteByToken(plaintext string) (*ShowHelperInvite, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	plaintext = strings.TrimSpace(plaintext)
	if plaintext == "" {
		return nil, false
	}
	hashed := hashShareSecret(plaintext)
	now := time.Now().UTC()
	for _, item := range s.showHelperInvites {
		if item == nil || item.TokenHash != hashed {
			continue
		}
		if item.RevokedAt != nil {
			return nil, false
		}
		if !item.ExpiresAt.IsZero() && now.After(item.ExpiresAt) {
			return nil, false
		}
		return cloneShowHelperInvite(item), true
	}
	return nil, false
}

func (s *memoryStore) showHelperInviteByID(id string) (*ShowHelperInvite, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id = strings.TrimSpace(id)
	if item, ok := s.showHelperInvites[id]; ok && item != nil {
		return cloneShowHelperInvite(item), true
	}
	return nil, false
}

func (s *memoryStore) revokeShowHelperInvite(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id = strings.TrimSpace(id)
	item, ok := s.showHelperInvites[id]
	if !ok || item == nil {
		return errors.New("helper invite not found")
	}
	if item.RevokedAt != nil {
		return nil
	}
	now := time.Now().UTC()
	item.RevokedAt = &now
	s.appendClaim(item.ID, "show_helper_invite", "show_helper_invite.revoked", item)
	return nil
}

func (s *memoryStore) listShowHelperInvitesByShow(showID string) []*ShowHelperInvite {
	s.mu.RLock()
	defer s.mu.RUnlock()

	showID = strings.TrimSpace(showID)
	out := make([]*ShowHelperInvite, 0)
	for _, item := range s.showHelperInvites {
		if item == nil || item.ShowID != showID {
			continue
		}
		out = append(out, cloneShowHelperInvite(item))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

func (s *memoryStore) createShowBadgeSession(input ShowBadgeSessionInput) (*IssuedShowBadgeSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	input = validateShowBadgeSessionInput(input)
	if input.ShowID == "" {
		return nil, errors.New("show_id is required")
	}
	if input.InviteID == "" {
		return nil, errors.New("invite_id is required")
	}
	if input.Name == "" {
		return nil, errors.New("name is required")
	}
	if input.Email == "" {
		return nil, errors.New("email is required")
	}

	plaintext, err := newShowBadgeSessionToken()
	if err != nil {
		return nil, fmt.Errorf("generate badge session token: %w", err)
	}
	now := time.Now().UTC()
	item := &ShowBadgeSession{
		ID:              newID("shbsess"),
		ShowID:          input.ShowID,
		InviteID:        input.InviteID,
		Name:            input.Name,
		Email:           input.Email,
		MatchedPersonID: input.MatchedPersonID,
		SessionHash:     hashShareSecret(plaintext),
		CreatedAt:       now,
		LastSeenAt:      now,
		ExpiresAt:       now.Add(time.Duration(input.ExpiresInHours) * time.Hour),
	}
	s.showBadgeSessions[item.ID] = item
	// Badge sessions are runtime-only; no claim is appended to keep the
	// kernel ledger free of high-volume helper traffic. They live in a
	// dedicated projection table and are reconstructed from there.

	out := cloneShowBadgeSession(item)
	return &IssuedShowBadgeSession{Session: out, Token: plaintext}, nil
}

func (s *memoryStore) findShowBadgeSessionByToken(plaintext string) (*ShowBadgeSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	plaintext = strings.TrimSpace(plaintext)
	if plaintext == "" {
		return nil, false
	}
	hashed := hashShareSecret(plaintext)
	now := time.Now().UTC()
	for _, item := range s.showBadgeSessions {
		if item == nil || item.SessionHash != hashed {
			continue
		}
		if item.RevokedAt != nil {
			return nil, false
		}
		if !item.ExpiresAt.IsZero() && now.After(item.ExpiresAt) {
			return nil, false
		}
		return cloneShowBadgeSession(item), true
	}
	return nil, false
}

func (s *memoryStore) touchShowBadgeSession(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id = strings.TrimSpace(id)
	item, ok := s.showBadgeSessions[id]
	if !ok || item == nil {
		return errors.New("badge session not found")
	}
	item.LastSeenAt = time.Now().UTC()
	return nil
}

func (s *memoryStore) revokeShowBadgeSession(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id = strings.TrimSpace(id)
	item, ok := s.showBadgeSessions[id]
	if !ok || item == nil {
		return errors.New("badge session not found")
	}
	if item.RevokedAt != nil {
		return nil
	}
	now := time.Now().UTC()
	item.RevokedAt = &now
	s.appendClaim(item.ID, "show_badge_session", "show_badge_session.ended", item)
	return nil
}

func (s *memoryStore) listShowBadgeSessionsByShow(showID string) []*ShowBadgeSession {
	s.mu.RLock()
	defer s.mu.RUnlock()

	showID = strings.TrimSpace(showID)
	out := make([]*ShowBadgeSession, 0)
	for _, item := range s.showBadgeSessions {
		if item == nil || item.ShowID != showID {
			continue
		}
		out = append(out, cloneShowBadgeSession(item))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

// --- postgresFlowershowStore methods ---

func (s *postgresFlowershowStore) createShowHelperInvite(input ShowHelperInviteInput) (*IssuedShowHelperInvite, error) {
	if s == nil || s.pool == nil {
		return nil, errors.New("store unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mem, claimStart, err := s.prepareMutation(ctx)
	if err != nil {
		return nil, err
	}
	issued, err := mem.createShowHelperInvite(input)
	if err != nil {
		return nil, err
	}
	if err := s.commitDomainMutation(ctx, mem, claimStart); err != nil {
		return nil, err
	}
	return issued, nil
}

func (s *postgresFlowershowStore) findShowHelperInviteByToken(plaintext string) (*ShowHelperInvite, bool) {
	return s.currentMem().findShowHelperInviteByToken(plaintext)
}

func (s *postgresFlowershowStore) showHelperInviteByID(id string) (*ShowHelperInvite, bool) {
	return s.currentMem().showHelperInviteByID(id)
}

func (s *postgresFlowershowStore) revokeShowHelperInvite(id string) error {
	if s == nil || s.pool == nil {
		return errors.New("store unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mem, claimStart, err := s.prepareMutation(ctx)
	if err != nil {
		return err
	}
	if err := mem.revokeShowHelperInvite(id); err != nil {
		return err
	}
	return s.commitDomainMutation(ctx, mem, claimStart)
}

func (s *postgresFlowershowStore) listShowHelperInvitesByShow(showID string) []*ShowHelperInvite {
	return s.currentMem().listShowHelperInvitesByShow(showID)
}

func (s *postgresFlowershowStore) createShowBadgeSession(input ShowBadgeSessionInput) (*IssuedShowBadgeSession, error) {
	if s == nil || s.pool == nil {
		return nil, errors.New("store unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mem, claimStart, err := s.prepareMutation(ctx)
	if err != nil {
		return nil, err
	}
	issued, err := mem.createShowBadgeSession(input)
	if err != nil {
		return nil, err
	}
	if err := s.commitDomainMutation(ctx, mem, claimStart); err != nil {
		return nil, err
	}
	return issued, nil
}

func (s *postgresFlowershowStore) findShowBadgeSessionByToken(plaintext string) (*ShowBadgeSession, bool) {
	return s.currentMem().findShowBadgeSessionByToken(plaintext)
}

func (s *postgresFlowershowStore) touchShowBadgeSession(id string) error {
	return s.currentMem().touchShowBadgeSession(id)
}

func (s *postgresFlowershowStore) revokeShowBadgeSession(id string) error {
	if s == nil || s.pool == nil {
		return errors.New("store unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mem, claimStart, err := s.prepareMutation(ctx)
	if err != nil {
		return err
	}
	if err := mem.revokeShowBadgeSession(id); err != nil {
		return err
	}
	return s.commitDomainMutation(ctx, mem, claimStart)
}

func (s *postgresFlowershowStore) listShowBadgeSessionsByShow(showID string) []*ShowBadgeSession {
	return s.currentMem().listShowBadgeSessionsByShow(showID)
}
