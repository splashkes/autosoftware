package interactions

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

func (s *RuntimeService) AssignHandle(ctx context.Context, input AssignHandleInput) (Handle, error) {
	pool, err := expectReady(s)
	if err != nil {
		return Handle{}, err
	}

	if strings.TrimSpace(input.Namespace) == "" || strings.TrimSpace(input.Handle) == "" {
		return Handle{}, errors.New("namespace and handle are required")
	}
	if strings.TrimSpace(input.SubjectKind) == "" || strings.TrimSpace(input.SubjectID) == "" {
		return Handle{}, errors.New("subject_kind and subject_id are required")
	}
	if strings.TrimSpace(input.HandleID) == "" {
		input.HandleID = newID("hdl")
	}
	input.Status = statusOrDefault(input.Status, "active")

	row := pool.QueryRow(ctx, `
		insert into runtime_handles (
		  handle_id, namespace, handle, subject_kind, subject_id, status, redirect_to_handle_id, metadata
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8::jsonb)
		on conflict (namespace, handle)
		do update set
		  subject_kind = excluded.subject_kind,
		  subject_id = excluded.subject_id,
		  status = excluded.status,
		  redirect_to_handle_id = excluded.redirect_to_handle_id,
		  metadata = excluded.metadata
		returning handle_id, namespace, handle, subject_kind, subject_id, status,
		          redirect_to_handle_id, metadata::text, created_at, retired_at
	`, input.HandleID, input.Namespace, input.Handle, input.SubjectKind, input.SubjectID, input.Status,
		nullString(input.RedirectToHandleID), jsonBytes(input.Metadata))

	item, err := scanHandle(row)
	return item, wrapErr("assign handle", err)
}

func (s *RuntimeService) ResolveHandle(ctx context.Context, namespace, handle string) (Handle, error) {
	pool, err := expectReady(s)
	if err != nil {
		return Handle{}, err
	}
	row := pool.QueryRow(ctx, `
		select handle_id, namespace, handle, subject_kind, subject_id, status,
		       redirect_to_handle_id, metadata::text, created_at, retired_at
		from runtime_handles
		where namespace = $1 and handle = $2
	`, strings.TrimSpace(namespace), strings.TrimSpace(handle))
	item, err := scanHandle(row)
	return item, wrapErr("resolve handle", err)
}

func (s *RuntimeService) CreateAccessLink(ctx context.Context, input CreateAccessLinkInput) (AccessLinkIssue, error) {
	pool, err := expectReady(s)
	if err != nil {
		return AccessLinkIssue{}, err
	}

	if strings.TrimSpace(input.SubjectKind) == "" || strings.TrimSpace(input.SubjectID) == "" {
		return AccessLinkIssue{}, errors.New("subject_kind and subject_id are required")
	}
	if strings.TrimSpace(input.AccessLinkID) == "" {
		input.AccessLinkID = newID("lnk")
	}
	if strings.TrimSpace(input.Token) == "" {
		input.Token, err = newToken()
		if err != nil {
			return AccessLinkIssue{}, err
		}
	}
	input.Status = statusOrDefault(input.Status, "active")

	row := pool.QueryRow(ctx, `
		insert into runtime_access_links (
		  access_link_id, token_hash, subject_kind, subject_id, bound_principal_id,
		  scope, status, max_uses, expires_at, created_by_principal_id, metadata
		)
		values ($1, $2, $3, $4, $5, $6::jsonb, $7, $8, $9, $10, $11::jsonb)
		returning access_link_id, subject_kind, subject_id, bound_principal_id, scope::text,
		          status, max_uses, use_count, expires_at, last_used_at, created_by_principal_id,
		          metadata::text, created_at, revoked_at
	`, input.AccessLinkID, hashToken(input.Token), input.SubjectKind, input.SubjectID,
		nullString(input.BoundPrincipalID), jsonBytes(input.Scope), input.Status, nullInt(input.MaxUses),
		nullTimeValue(input.ExpiresAt), nullString(input.CreatedByPrincipalID), jsonBytes(input.Metadata))

	item, err := scanAccessLink(row)
	if err != nil {
		return AccessLinkIssue{}, wrapErr("create access link", err)
	}
	return AccessLinkIssue{AccessLink: item, Token: input.Token}, nil
}

func (s *RuntimeService) ConsumeAccessLink(ctx context.Context, token string) (AccessLink, error) {
	pool, err := expectReady(s)
	if err != nil {
		return AccessLink{}, err
	}
	if strings.TrimSpace(token) == "" {
		return AccessLink{}, errors.New("token is required")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return AccessLink{}, wrapErr("begin consume access link", err)
	}
	defer tx.Rollback(ctx)

	var accessLinkID string
	var status string
	var useCount int
	var maxUses sql.NullInt32
	var expiresAt sql.NullTime
	var revokedAt sql.NullTime
	row := tx.QueryRow(ctx, `
		select access_link_id, status, use_count, max_uses, expires_at, revoked_at
		from runtime_access_links
		where token_hash = $1
		for update
	`, hashToken(token))
	if err := row.Scan(&accessLinkID, &status, &useCount, &maxUses, &expiresAt, &revokedAt); err != nil {
		return AccessLink{}, wrapErr("load access link", rowNotFound(err))
	}

	now := s.nowUTC()
	if status != "active" || revokedAt.Valid || (expiresAt.Valid && !expiresAt.Time.After(now)) {
		return AccessLink{}, ErrNotFound
	}
	if maxUses.Valid && useCount >= int(maxUses.Int32) {
		return AccessLink{}, ErrNotFound
	}

	item, err := scanAccessLink(tx.QueryRow(ctx, `
		update runtime_access_links
		set use_count = use_count + 1,
		    last_used_at = $2
		where access_link_id = $1
		returning access_link_id, subject_kind, subject_id, bound_principal_id, scope::text,
		          status, max_uses, use_count, expires_at, last_used_at, created_by_principal_id,
		          metadata::text, created_at, revoked_at
	`, accessLinkID, now))
	if err != nil {
		return AccessLink{}, wrapErr("consume access link", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return AccessLink{}, wrapErr("commit consume access link", err)
	}
	return item, nil
}

func (s *RuntimeService) UpsertPublication(ctx context.Context, input UpsertPublicationInput) (Publication, error) {
	pool, err := expectReady(s)
	if err != nil {
		return Publication{}, err
	}

	if strings.TrimSpace(input.SubjectKind) == "" || strings.TrimSpace(input.SubjectID) == "" {
		return Publication{}, errors.New("subject_kind and subject_id are required")
	}
	if strings.TrimSpace(input.Status) == "" {
		return Publication{}, errors.New("status is required")
	}
	if strings.TrimSpace(input.PublicationID) == "" {
		input.PublicationID = newID("pub")
	}
	input.Visibility = statusOrDefault(input.Visibility, "private")

	row := pool.QueryRow(ctx, `
		insert into runtime_publications (
		  publication_id, subject_kind, subject_id, status, visibility, publish_at,
		  unpublish_at, starts_at, ends_at, timezone, all_day, metadata
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb)
		on conflict (subject_kind, subject_id)
		do update set
		  status = excluded.status,
		  visibility = excluded.visibility,
		  publish_at = excluded.publish_at,
		  unpublish_at = excluded.unpublish_at,
		  starts_at = excluded.starts_at,
		  ends_at = excluded.ends_at,
		  timezone = excluded.timezone,
		  all_day = excluded.all_day,
		  metadata = excluded.metadata,
		  updated_at = now()
		returning publication_id, subject_kind, subject_id, status, visibility, publish_at,
		          unpublish_at, starts_at, ends_at, timezone, all_day, metadata::text,
		          created_at, updated_at
	`, input.PublicationID, input.SubjectKind, input.SubjectID, input.Status, input.Visibility,
		nullTimeValue(input.PublishAt), nullTimeValue(input.UnpublishAt), nullTimeValue(input.StartsAt),
		nullTimeValue(input.EndsAt), nullString(input.Timezone), input.AllDay, jsonBytes(input.Metadata))

	item, err := scanPublication(row)
	return item, wrapErr("upsert publication", err)
}

func (s *RuntimeService) RecordStateTransition(ctx context.Context, input RecordStateTransitionInput) (StateTransition, error) {
	pool, err := expectReady(s)
	if err != nil {
		return StateTransition{}, err
	}

	if strings.TrimSpace(input.SubjectKind) == "" || strings.TrimSpace(input.SubjectID) == "" {
		return StateTransition{}, errors.New("subject_kind and subject_id are required")
	}
	if strings.TrimSpace(input.Machine) == "" || strings.TrimSpace(input.ToState) == "" {
		return StateTransition{}, errors.New("machine and to_state are required")
	}
	if strings.TrimSpace(input.TransitionID) == "" {
		input.TransitionID = newID("st")
	}
	occurredAt := s.nowUTC()
	if input.OccurredAt != nil && !input.OccurredAt.IsZero() {
		occurredAt = input.OccurredAt.UTC()
	}

	row := pool.QueryRow(ctx, `
		insert into runtime_state_transitions (
		  transition_id, subject_kind, subject_id, machine, from_state, to_state,
		  actor_principal_id, actor_session_id, request_id, reason, metadata, occurred_at
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::jsonb, $12)
		returning transition_id, subject_kind, subject_id, machine, from_state, to_state,
		          actor_principal_id, actor_session_id, request_id, reason, metadata::text, occurred_at
	`, input.TransitionID, input.SubjectKind, input.SubjectID, input.Machine, nullString(input.FromState),
		input.ToState, nullString(input.ActorPrincipalID), nullString(input.ActorSessionID),
		nullString(input.RequestID), nullString(input.Reason), jsonBytes(input.Metadata), occurredAt)

	item, err := scanStateTransition(row)
	return item, wrapErr("record state transition", err)
}

func (s *RuntimeService) RecordActivityEvent(ctx context.Context, input RecordActivityEventInput) (ActivityEvent, error) {
	pool, err := expectReady(s)
	if err != nil {
		return ActivityEvent{}, err
	}

	if strings.TrimSpace(input.SubjectKind) == "" || strings.TrimSpace(input.SubjectID) == "" {
		return ActivityEvent{}, errors.New("subject_kind and subject_id are required")
	}
	if strings.TrimSpace(input.Name) == "" {
		return ActivityEvent{}, errors.New("name is required")
	}
	if strings.TrimSpace(input.ActivityID) == "" {
		input.ActivityID = newID("act")
	}
	visibility := statusOrDefault(input.Visibility, "internal")
	occurredAt := s.nowUTC()
	if input.OccurredAt != nil && !input.OccurredAt.IsZero() {
		occurredAt = input.OccurredAt.UTC()
	}

	row := pool.QueryRow(ctx, `
		insert into runtime_activity_events (
		  activity_id, subject_kind, subject_id, actor_principal_id, actor_session_id,
		  request_id, name, visibility, data, occurred_at
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10)
		returning activity_id, subject_kind, subject_id, actor_principal_id, actor_session_id,
		          request_id, name, visibility, data::text, occurred_at
	`, input.ActivityID, input.SubjectKind, input.SubjectID, nullString(input.ActorPrincipalID),
		nullString(input.ActorSessionID), nullString(input.RequestID), input.Name, visibility,
		jsonBytes(input.Data), occurredAt)

	item, err := scanActivityEvent(row)
	return item, wrapErr("record activity event", err)
}

func (s *RuntimeService) EnqueueJob(ctx context.Context, input EnqueueJobInput) (Job, error) {
	pool, err := expectReady(s)
	if err != nil {
		return Job{}, err
	}

	if strings.TrimSpace(input.Kind) == "" {
		return Job{}, errors.New("kind is required")
	}
	if strings.TrimSpace(input.JobID) == "" {
		input.JobID = newID("job")
	}
	input.Queue = statusOrDefault(input.Queue, "default")
	input.Status = statusOrDefault(input.Status, "pending")
	if input.Priority == 0 {
		input.Priority = 100
	}
	if input.MaxAttempts == 0 {
		input.MaxAttempts = 10
	}
	runAt := s.nowUTC()
	if input.RunAt != nil && !input.RunAt.IsZero() {
		runAt = input.RunAt.UTC()
	}

	row := pool.QueryRow(ctx, `
		insert into runtime_jobs (
		  job_id, queue, kind, dedupe_key, status, priority, run_at, max_attempts, payload
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb)
		returning job_id, queue, kind, dedupe_key, status, priority, run_at, locked_at,
		          locked_by, attempts, max_attempts, payload::text, last_error, created_at, finished_at
	`, input.JobID, input.Queue, input.Kind, nullString(input.DedupeKey), input.Status,
		input.Priority, runAt, input.MaxAttempts, jsonBytes(input.Payload))

	item, err := scanJob(row)
	return item, wrapErr("enqueue job", err)
}

func (s *RuntimeService) GetJob(ctx context.Context, jobID string) (Job, error) {
	pool, err := expectReady(s)
	if err != nil {
		return Job{}, err
	}

	row := pool.QueryRow(ctx, `
		select job_id, queue, kind, dedupe_key, status, priority, run_at, locked_at,
		       locked_by, attempts, max_attempts, payload::text, last_error, created_at, finished_at
		from runtime_jobs
		where job_id = $1
	`, strings.TrimSpace(jobID))

	item, err := scanJob(row)
	return item, wrapErr("get job", err)
}

func (s *RuntimeService) ClaimJobs(ctx context.Context, input JobClaimInput) ([]Job, error) {
	pool, err := expectReady(s)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.Worker) == "" {
		return nil, errors.New("worker is required")
	}
	queue := statusOrDefault(input.Queue, "default")
	limit := clampLimit(input.Limit, 20, 100)

	rows, err := pool.Query(ctx, `
		with due as (
		  select job_id
		  from runtime_jobs
		  where queue = $1
		    and status = 'pending'
		    and run_at <= $2
		  order by priority desc, run_at asc
		  limit $3
		  for update skip locked
		)
		update runtime_jobs
		set status = 'running',
		    locked_at = $2,
		    locked_by = $4
		where job_id in (select job_id from due)
		returning job_id, queue, kind, dedupe_key, status, priority, run_at, locked_at,
		          locked_by, attempts, max_attempts, payload::text, last_error, created_at, finished_at
	`, queue, s.nowUTC(), limit, input.Worker)
	if err != nil {
		return nil, wrapErr("claim jobs", err)
	}
	defer rows.Close()

	var items []Job
	for rows.Next() {
		item, err := scanJob(rows)
		if err != nil {
			return nil, wrapErr("scan claimed job", err)
		}
		items = append(items, item)
	}
	return items, wrapErr("claim jobs", rows.Err())
}

func (s *RuntimeService) CompleteJob(ctx context.Context, jobID string, input JobCompleteInput) (Job, error) {
	pool, err := expectReady(s)
	if err != nil {
		return Job{}, err
	}
	finishedAt := s.nowUTC()
	if input.FinishedAt != nil && !input.FinishedAt.IsZero() {
		finishedAt = input.FinishedAt.UTC()
	}

	row := pool.QueryRow(ctx, `
		update runtime_jobs
		set status = 'completed',
		    locked_at = null,
		    locked_by = null,
		    finished_at = $2
		where job_id = $1
		returning job_id, queue, kind, dedupe_key, status, priority, run_at, locked_at,
		          locked_by, attempts, max_attempts, payload::text, last_error, created_at, finished_at
	`, strings.TrimSpace(jobID), finishedAt)

	item, err := scanJob(row)
	return item, wrapErr("complete job", err)
}

func (s *RuntimeService) FailJob(ctx context.Context, jobID string, input JobFailInput) (Job, error) {
	pool, err := expectReady(s)
	if err != nil {
		return Job{}, err
	}
	if strings.TrimSpace(input.Error) == "" {
		return Job{}, errors.New("error is required")
	}
	retryAt := s.nowUTC()
	if input.RetryAt != nil && !input.RetryAt.IsZero() {
		retryAt = input.RetryAt.UTC()
	}
	failedAt := retryAt
	if input.FailedAt != nil && !input.FailedAt.IsZero() {
		failedAt = input.FailedAt.UTC()
	}

	row := pool.QueryRow(ctx, `
		update runtime_jobs
		set attempts = attempts + 1,
		    last_error = $2,
		    locked_at = null,
		    locked_by = null,
		    run_at = case when attempts + 1 >= max_attempts then run_at else $3 end,
		    finished_at = case when attempts + 1 >= max_attempts then $4 else null end,
		    status = case when attempts + 1 >= max_attempts then 'failed' else 'pending' end
		where job_id = $1
		returning job_id, queue, kind, dedupe_key, status, priority, run_at, locked_at,
		          locked_by, attempts, max_attempts, payload::text, last_error, created_at, finished_at
	`, strings.TrimSpace(jobID), input.Error, retryAt, failedAt)

	item, err := scanJob(row)
	return item, wrapErr("fail job", err)
}

func (s *RuntimeService) EnqueueOutboxMessage(ctx context.Context, input EnqueueOutboxMessageInput) (OutboxMessage, error) {
	pool, err := expectReady(s)
	if err != nil {
		return OutboxMessage{}, err
	}
	if strings.TrimSpace(input.Channel) == "" || strings.TrimSpace(input.Template) == "" {
		return OutboxMessage{}, errors.New("channel and template are required")
	}
	if strings.TrimSpace(input.MessageID) == "" {
		input.MessageID = newID("out")
	}
	input.Status = statusOrDefault(input.Status, "pending")
	enqueueAfter := s.nowUTC()
	if input.EnqueueAfter != nil && !input.EnqueueAfter.IsZero() {
		enqueueAfter = input.EnqueueAfter.UTC()
	}

	row := pool.QueryRow(ctx, `
		insert into runtime_outbox_messages (
		  message_id, subject_kind, subject_id, recipient_principal_id, recipient_address,
		  channel, template, dedupe_key, status, enqueue_after, payload
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::jsonb)
		returning message_id, subject_kind, subject_id, recipient_principal_id, recipient_address,
		          channel, template, dedupe_key, status, enqueue_after, payload::text,
		          created_at, sent_at, canceled_at
	`, input.MessageID, nullString(input.SubjectKind), nullString(input.SubjectID),
		nullString(input.RecipientPrincipalID), nullString(input.RecipientAddress),
		input.Channel, input.Template, nullString(input.DedupeKey), input.Status,
		enqueueAfter, jsonBytes(input.Payload))

	item, err := scanOutboxMessage(row)
	return item, wrapErr("enqueue outbox message", err)
}
