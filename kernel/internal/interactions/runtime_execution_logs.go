package interactions

import (
	"context"
	"errors"
	"strings"
	"time"
)

type RealizationExecutionLog struct {
	LogID       string    `json:"log_id"`
	ExecutionID string    `json:"execution_id"`
	Source      string    `json:"source"`
	Stream      string    `json:"stream,omitempty"`
	Message     string    `json:"message"`
	OccurredAt  time.Time `json:"occurred_at"`
}

type RecordRealizationExecutionLogInput struct {
	LogID       string     `json:"log_id,omitempty"`
	ExecutionID string     `json:"execution_id"`
	Source      string     `json:"source"`
	Stream      string     `json:"stream,omitempty"`
	Message     string     `json:"message"`
	OccurredAt  *time.Time `json:"occurred_at,omitempty"`
}

func (s *RuntimeService) RecordRealizationExecutionLog(ctx context.Context, input RecordRealizationExecutionLogInput) (RealizationExecutionLog, error) {
	pool, err := expectReady(s)
	if err != nil {
		return RealizationExecutionLog{}, err
	}
	if strings.TrimSpace(input.ExecutionID) == "" || strings.TrimSpace(input.Source) == "" {
		return RealizationExecutionLog{}, errors.New("execution_id and source are required")
	}
	if strings.TrimSpace(input.Message) == "" {
		return RealizationExecutionLog{}, errors.New("message is required")
	}
	if strings.TrimSpace(input.LogID) == "" {
		input.LogID = newID("log")
	}
	occurredAt := s.nowUTC()
	if input.OccurredAt != nil && !input.OccurredAt.IsZero() {
		occurredAt = input.OccurredAt.UTC()
	}

	row := pool.QueryRow(ctx, `
		insert into runtime_realization_execution_logs (log_id, execution_id, source, stream, message, occurred_at)
		values ($1, $2, $3, $4, $5, $6)
		returning log_id, execution_id, source, stream, message, occurred_at
	`, input.LogID, input.ExecutionID, strings.TrimSpace(input.Source), strings.TrimSpace(input.Stream), input.Message, occurredAt)

	var item RealizationExecutionLog
	var stream string
	if err := row.Scan(&item.LogID, &item.ExecutionID, &item.Source, &stream, &item.Message, &item.OccurredAt); err != nil {
		return RealizationExecutionLog{}, wrapErr("record realization execution log", rowNotFound(err))
	}
	item.Stream = stream
	return item, nil
}
