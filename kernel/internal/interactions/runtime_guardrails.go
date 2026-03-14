package interactions

import (
	"context"
	"errors"
	"strings"
)

func (s *RuntimeService) RecordGuardDecision(ctx context.Context, input RecordGuardDecisionInput) (GuardDecision, error) {
	pool, err := expectReady(s)
	if err != nil {
		return GuardDecision{}, err
	}
	if strings.TrimSpace(input.Namespace) == "" {
		return GuardDecision{}, errors.New("namespace is required")
	}
	if strings.TrimSpace(input.Action) == "" || strings.TrimSpace(input.Outcome) == "" {
		return GuardDecision{}, errors.New("action and outcome are required")
	}
	if strings.TrimSpace(input.DecisionID) == "" {
		input.DecisionID = newID("gd")
	}

	row := pool.QueryRow(ctx, `
		insert into runtime_guard_decisions (
		  decision_id, namespace, request_id, session_id, principal_id, subject_key,
		  action, outcome, reason, metadata
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb)
		returning decision_id, namespace, request_id, session_id, principal_id, subject_key,
		          action, outcome, reason, metadata::text, created_at
	`, input.DecisionID, input.Namespace, nullString(input.RequestID), nullString(input.SessionID),
		nullString(input.PrincipalID), nullString(input.SubjectKey), input.Action, input.Outcome,
		nullString(input.Reason), jsonBytes(input.Metadata))

	item, err := scanGuardDecision(row)
	return item, wrapErr("record guard decision", err)
}

func (s *RuntimeService) RecordRiskEvent(ctx context.Context, input RecordRiskEventInput) (RiskEvent, error) {
	pool, err := expectReady(s)
	if err != nil {
		return RiskEvent{}, err
	}
	if strings.TrimSpace(input.Namespace) == "" {
		return RiskEvent{}, errors.New("namespace is required")
	}
	if strings.TrimSpace(input.Kind) == "" || strings.TrimSpace(input.Severity) == "" {
		return RiskEvent{}, errors.New("kind and severity are required")
	}
	if strings.TrimSpace(input.RiskEventID) == "" {
		input.RiskEventID = newID("risk")
	}
	status := statusOrDefault(input.Status, "open")

	row := pool.QueryRow(ctx, `
		insert into runtime_risk_events (
		  risk_event_id, namespace, subject_key, request_id, session_id, principal_id,
		  kind, severity, status, data
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb)
		returning risk_event_id, namespace, subject_key, request_id, session_id, principal_id,
		          kind, severity, status, data::text, created_at, resolved_at
	`, input.RiskEventID, input.Namespace, nullString(input.SubjectKey), nullString(input.RequestID),
		nullString(input.SessionID), nullString(input.PrincipalID), input.Kind, input.Severity, status,
		jsonBytes(input.Data))

	item, err := scanRiskEvent(row)
	return item, wrapErr("record risk event", err)
}
