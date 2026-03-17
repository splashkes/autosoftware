package interactions

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *RuntimeService) EnforceRateLimit(ctx context.Context, input EnforceRateLimitInput) (RateLimitDecision, error) {
	pool, err := expectReady(s)
	if err != nil {
		return RateLimitDecision{}, err
	}
	if strings.TrimSpace(input.Namespace) == "" {
		return RateLimitDecision{}, errors.New("namespace is required")
	}
	if strings.TrimSpace(input.SubjectKey) == "" {
		return RateLimitDecision{}, errors.New("subject_key is required")
	}
	if input.Limit <= 0 {
		return RateLimitDecision{}, errors.New("limit must be greater than zero")
	}

	window := input.Window
	if window <= 0 {
		window = time.Minute
	}
	blockDuration := input.BlockDuration
	if blockDuration <= 0 {
		blockDuration = window
	}

	now := s.nowUTC()
	windowStartedAt := now.Truncate(window)
	windowEndsAt := windowStartedAt.Add(window)

	tx, err := pool.Begin(ctx)
	if err != nil {
		return RateLimitDecision{}, wrapErr("begin rate limit transaction", err)
	}
	defer tx.Rollback(ctx)

	var bucketID string
	var hitCount int64
	var blockedUntil sql.NullTime
	var metadata string

	row := tx.QueryRow(ctx, `
		select bucket_id, hit_count, blocked_until, metadata::text
		from runtime_rate_limit_buckets
		where namespace = $1
		  and subject_key = $2
		  and window_started_at = $3
		  and window_ends_at = $4
		for update
	`, input.Namespace, input.SubjectKey, windowStartedAt, windowEndsAt)

	switch err := row.Scan(&bucketID, &hitCount, &blockedUntil, &metadata); {
	case errors.Is(err, pgx.ErrNoRows):
		bucketID = newID("rl")
		hitCount = 1
		row = tx.QueryRow(ctx, `
			insert into runtime_rate_limit_buckets (
			  bucket_id, namespace, subject_key, window_started_at, window_ends_at,
			  hit_count, blocked_until, metadata, updated_at
			)
			values ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9)
			returning blocked_until, metadata::text
		`, bucketID, input.Namespace, input.SubjectKey, windowStartedAt, windowEndsAt, hitCount, nil, jsonBytes(input.Metadata), now)
		if err := row.Scan(&blockedUntil, &metadata); err != nil {
			return RateLimitDecision{}, wrapErr("insert rate limit bucket", err)
		}
	case err != nil:
		return RateLimitDecision{}, wrapErr("load rate limit bucket", err)
	default:
		hitCount++
		nextBlockedUntil := blockedUntil
		if blockedUntil.Valid && blockedUntil.Time.After(now) {
			// Keep the active block in place while still counting follow-up requests.
		} else if hitCount > input.Limit {
			nextBlockedUntil = sql.NullTime{Time: now.Add(blockDuration), Valid: true}
		}
		row = tx.QueryRow(ctx, `
			update runtime_rate_limit_buckets
			set hit_count = $2,
			    blocked_until = $3,
			    metadata = coalesce(metadata, '{}'::jsonb) || $4::jsonb,
			    updated_at = $5
			where bucket_id = $1
			returning blocked_until, metadata::text
		`, bucketID, hitCount, nullTimeValue(toTimePtr(nextBlockedUntil)), jsonBytes(input.Metadata), now)
		if err := row.Scan(&blockedUntil, &metadata); err != nil {
			return RateLimitDecision{}, wrapErr("update rate limit bucket", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return RateLimitDecision{}, wrapErr("commit rate limit transaction", err)
	}

	decision := RateLimitDecision{
		Namespace:       input.Namespace,
		SubjectKey:      input.SubjectKey,
		Allowed:         true,
		HitCount:        hitCount,
		Limit:           input.Limit,
		WindowStartedAt: windowStartedAt,
		WindowEndsAt:    windowEndsAt,
		Metadata:        parseJSON(metadata),
	}
	if blockedUntil.Valid {
		value := blockedUntil.Time.UTC()
		decision.BlockedUntil = &value
	}

	if decision.BlockedUntil != nil && decision.BlockedUntil.After(now) {
		decision.Allowed = false
		decision.RetryAfter = decision.BlockedUntil.Sub(now)
	} else if decision.HitCount > decision.Limit {
		decision.Allowed = false
		decision.RetryAfter = windowEndsAt.Sub(now)
	}

	if decision.Allowed {
		return decision, nil
	}

	action := strings.TrimSpace(input.Action)
	if action == "" {
		action = "request"
	}
	_, _ = s.RecordGuardDecision(ctx, RecordGuardDecisionInput{
		Namespace:   input.Namespace,
		RequestID:   input.RequestID,
		SessionID:   input.SessionID,
		PrincipalID: input.PrincipalID,
		SubjectKey:  input.SubjectKey,
		Action:      action,
		Outcome:     "blocked",
		Reason:      "rate_limit_exceeded",
		Metadata: map[string]interface{}{
			"hit_count":        decision.HitCount,
			"limit":            decision.Limit,
			"window_ends_at":   decision.WindowEndsAt.Format(time.RFC3339),
			"retry_after_secs": int(decision.RetryAfter / time.Second),
		},
	})

	return decision, &RateLimitError{
		Namespace:  input.Namespace,
		SubjectKey: input.SubjectKey,
		Message:    fmt.Sprintf("Too many requests for %s. Try again shortly.", input.Namespace),
		RetryAfter: decision.RetryAfter,
	}
}
