package feedbackloop

import (
	"context"
	"encoding/json"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore persists feedback loop records to the runtime database.
type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) RecordIncident(_ context.Context, inc ClientIncident) error {
	tags := jsonBytes(inc.Tags)
	data := jsonBytes(inc.Data)

	// Fire-and-forget: incidents are telemetry and should not block the caller.
	go func() {
		_, err := s.pool.Exec(context.Background(), `
			insert into runtime_client_incidents (
				incident_id, request_id, session_id, seed_id, realization_id,
				route, method, page_url, referrer, user_agent,
				kind, severity, message, stack, component_stack, source,
				tags, data, created_at
			) values (
				$1, $2, $3, $4, $5,
				$6, $7, $8, $9, $10,
				$11, $12, $13, $14, $15, $16,
				$17, $18, $19
			)
			on conflict (incident_id) do nothing`,
			inc.ID, inc.Request.RequestID, inc.Request.SessionID, inc.Request.SeedID, inc.Request.RealizationID,
			inc.Request.Route, inc.Request.Method, inc.Request.PageURL, inc.Request.Referrer, inc.Request.UserAgent,
			inc.Kind, inc.Severity, inc.Message, inc.Stack, inc.ComponentStack, inc.Source,
			tags, data, inc.CreatedAt,
		)
		if err != nil {
			log.Printf("feedback loop: record incident: %v", err)
		}
	}()
	return nil
}

func (s *PostgresStore) RecordRequestEvent(_ context.Context, evt RequestEvent) error {
	data := jsonBytes(evt.Data)

	go func() {
		_, err := s.pool.Exec(context.Background(), `
			insert into runtime_request_events (
				event_id, request_id, session_id, seed_id, realization_id,
				route, method, name, status_code, latency_ms,
				data, created_at
			) values (
				$1, $2, $3, $4, $5,
				$6, $7, $8, $9, $10,
				$11, $12
			)
			on conflict (event_id) do nothing`,
			evt.ID, evt.Request.RequestID, evt.Request.SessionID, evt.Request.SeedID, evt.Request.RealizationID,
			evt.Request.Route, evt.Request.Method, evt.Name, evt.StatusCode, evt.LatencyMS,
			data, evt.CreatedAt,
		)
		if err != nil {
			log.Printf("feedback loop: record request event: %v", err)
		}
	}()
	return nil
}

func (s *PostgresStore) RecordTestRun(ctx context.Context, run TestRun) error {
	summary := jsonBytes(run.Summary)

	_, err := s.pool.Exec(ctx, `
		insert into runtime_test_runs (
			test_run_id, seed_id, realization_id, suite, status,
			request_id, session_id, summary, started_at, finished_at
		) values (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10
		)
		on conflict (test_run_id) do nothing`,
		run.ID, run.SeedID, run.RealizationID, run.Suite, run.Status,
		run.Request.RequestID, run.Request.SessionID, summary, run.StartedAt, run.FinishedAt,
	)
	if err != nil {
		log.Printf("feedback loop: record test run: %v", err)
	}
	return err
}

func (s *PostgresStore) RecordAgentReview(ctx context.Context, rev AgentReview) error {
	findings := jsonBytes(rev.Findings)
	if rev.Findings == nil {
		findings = []byte("[]")
	}

	_, err := s.pool.Exec(ctx, `
		insert into runtime_agent_reviews (
			review_id, seed_id, realization_id, reviewer, status,
			summary, findings, request_id, session_id, created_at
		) values (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10
		)
		on conflict (review_id) do nothing`,
		rev.ID, rev.SeedID, rev.RealizationID, rev.Reviewer, rev.Status,
		rev.Summary, findings, rev.Request.RequestID, rev.Request.SessionID, rev.CreatedAt,
	)
	if err != nil {
		log.Printf("feedback loop: record agent review: %v", err)
	}
	return err
}

func jsonBytes(v any) []byte {
	if v == nil {
		return []byte("{}")
	}
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return b
}
