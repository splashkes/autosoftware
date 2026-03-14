package feedbackloop

import "time"

type Severity string

const (
	SeverityDebug Severity = "debug"
	SeverityInfo  Severity = "info"
	SeverityWarn  Severity = "warn"
	SeverityError Severity = "error"
	SeverityFatal Severity = "fatal"
)

// RequestContext ties browser-side or runtime events back to a specific request
// and pinned realization so agents can review failures against the exact output
// that produced them.
type RequestContext struct {
	RequestID     string `json:"request_id,omitempty"`
	SessionID     string `json:"session_id,omitempty"`
	SeedID        string `json:"seed_id,omitempty"`
	RealizationID string `json:"realization_id,omitempty"`
	Route         string `json:"route,omitempty"`
	Method        string `json:"method,omitempty"`
	PageURL       string `json:"page_url,omitempty"`
	Referrer      string `json:"referrer,omitempty"`
	UserAgent     string `json:"user_agent,omitempty"`
}

type ClientIncident struct {
	ID             string            `json:"id,omitempty"`
	Kind           string            `json:"kind,omitempty"`
	Severity       Severity          `json:"severity,omitempty"`
	Message        string            `json:"message,omitempty"`
	Stack          string            `json:"stack,omitempty"`
	ComponentStack string            `json:"component_stack,omitempty"`
	Source         string            `json:"source,omitempty"`
	CreatedAt      time.Time         `json:"created_at,omitempty"`
	Request        RequestContext    `json:"request,omitempty"`
	Tags           map[string]string `json:"tags,omitempty"`
	Data           map[string]any    `json:"data,omitempty"`
}

type RequestEvent struct {
	ID         string         `json:"id,omitempty"`
	Name       string         `json:"name,omitempty"`
	StatusCode int            `json:"status_code,omitempty"`
	LatencyMS  int64          `json:"latency_ms,omitempty"`
	CreatedAt  time.Time      `json:"created_at,omitempty"`
	Request    RequestContext `json:"request,omitempty"`
	Data       map[string]any `json:"data,omitempty"`
}

type TestRun struct {
	ID            string         `json:"id,omitempty"`
	SeedID        string         `json:"seed_id,omitempty"`
	RealizationID string         `json:"realization_id,omitempty"`
	Suite         string         `json:"suite,omitempty"`
	Status        string         `json:"status,omitempty"`
	StartedAt     time.Time      `json:"started_at,omitempty"`
	FinishedAt    time.Time      `json:"finished_at,omitempty"`
	Request       RequestContext `json:"request,omitempty"`
	Summary       map[string]any `json:"summary,omitempty"`
}

type AgentReviewFinding struct {
	Title    string `json:"title,omitempty"`
	Severity string `json:"severity,omitempty"`
	Body     string `json:"body,omitempty"`
}

type AgentReview struct {
	ID            string               `json:"id,omitempty"`
	SeedID        string               `json:"seed_id,omitempty"`
	RealizationID string               `json:"realization_id,omitempty"`
	Reviewer      string               `json:"reviewer,omitempty"`
	Status        string               `json:"status,omitempty"`
	Summary       string               `json:"summary,omitempty"`
	CreatedAt     time.Time            `json:"created_at,omitempty"`
	Findings      []AgentReviewFinding `json:"findings,omitempty"`
	Request       RequestContext       `json:"request,omitempty"`
}
