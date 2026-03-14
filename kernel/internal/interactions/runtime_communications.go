package interactions

import (
	"context"
	"errors"
	"strings"
)

func (s *RuntimeService) CreateThread(ctx context.Context, input CreateThreadInput) (Thread, error) {
	pool, err := expectReady(s)
	if err != nil {
		return Thread{}, err
	}
	if strings.TrimSpace(input.SubjectKind) == "" || strings.TrimSpace(input.SubjectID) == "" {
		return Thread{}, errors.New("subject_kind and subject_id are required")
	}
	if strings.TrimSpace(input.ThreadID) == "" {
		input.ThreadID = newID("thr")
	}
	input.ThreadKind = statusOrDefault(input.ThreadKind, "conversation")
	input.Status = statusOrDefault(input.Status, "open")
	input.Visibility = statusOrDefault(input.Visibility, "shared")

	row := pool.QueryRow(ctx, `
		insert into runtime_threads (
		  thread_id, subject_kind, subject_id, thread_kind, status, visibility, title, metadata
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8::jsonb)
		returning thread_id, subject_kind, subject_id, thread_kind, status, visibility,
		          title, metadata::text, created_at, closed_at
	`, input.ThreadID, input.SubjectKind, input.SubjectID, input.ThreadKind, input.Status,
		input.Visibility, nullString(input.Title), jsonBytes(input.Metadata))

	item, err := scanThread(row)
	return item, wrapErr("create thread", err)
}

func (s *RuntimeService) AddThreadParticipant(ctx context.Context, input AddThreadParticipantInput) (ThreadParticipant, error) {
	pool, err := expectReady(s)
	if err != nil {
		return ThreadParticipant{}, err
	}
	if strings.TrimSpace(input.ThreadID) == "" {
		return ThreadParticipant{}, errors.New("thread_id is required")
	}
	if strings.TrimSpace(input.ParticipantID) == "" {
		input.ParticipantID = newID("part")
	}
	role := statusOrDefault(input.Role, "participant")
	status := statusOrDefault(input.Status, "active")

	row := pool.QueryRow(ctx, `
		insert into runtime_thread_participants (
		  participant_id, thread_id, principal_id, role, status, delivery_policy, metadata
		)
		values ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb)
		returning participant_id, thread_id, principal_id, role, status, delivery_policy::text,
		          metadata::text, joined_at, left_at
	`, input.ParticipantID, input.ThreadID, nullString(input.PrincipalID), role, status,
		jsonBytes(input.DeliveryPolicy), jsonBytes(input.Metadata))

	item, err := scanThreadParticipant(row)
	return item, wrapErr("add thread participant", err)
}

func (s *RuntimeService) PostMessage(ctx context.Context, input PostMessageInput) (Message, error) {
	pool, err := expectReady(s)
	if err != nil {
		return Message{}, err
	}
	if strings.TrimSpace(input.ThreadID) == "" {
		return Message{}, errors.New("thread_id is required")
	}
	if strings.TrimSpace(input.Body) == "" {
		return Message{}, errors.New("body is required")
	}
	if strings.TrimSpace(input.MessageID) == "" {
		input.MessageID = newID("msg")
	}
	kind := statusOrDefault(input.Kind, "message")
	visibility := statusOrDefault(input.Visibility, "shared")
	bodyFormat := statusOrDefault(input.BodyFormat, "plain_text")

	row := pool.QueryRow(ctx, `
		insert into runtime_messages (
		  message_id, thread_id, author_principal_id, author_session_id, request_id,
		  kind, visibility, body_format, body, metadata
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb)
		returning message_id, thread_id, author_principal_id, author_session_id, request_id,
		          kind, visibility, body_format, body, metadata::text, created_at, edited_at, deleted_at
	`, input.MessageID, input.ThreadID, nullString(input.AuthorPrincipalID), nullString(input.AuthorSessionID),
		nullString(input.RequestID), kind, visibility, bodyFormat, input.Body, jsonBytes(input.Metadata))

	item, err := scanMessage(row)
	return item, wrapErr("post message", err)
}

func (s *RuntimeService) ListMessages(ctx context.Context, threadID string, limit int) ([]Message, error) {
	pool, err := expectReady(s)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(threadID) == "" {
		return nil, errors.New("thread_id is required")
	}
	rows, err := pool.Query(ctx, `
		select message_id, thread_id, author_principal_id, author_session_id, request_id,
		       kind, visibility, body_format, body, metadata::text, created_at, edited_at, deleted_at
		from runtime_messages
		where thread_id = $1
		order by created_at desc
		limit $2
	`, strings.TrimSpace(threadID), clampLimit(limit, 50, 200))
	if err != nil {
		return nil, wrapErr("list messages", err)
	}
	return rowsToMessages(rows)
}
