package feedbackloop

import (
	"context"
	"sync"
)

type Recorder interface {
	RecordIncident(context.Context, ClientIncident) error
	RecordRequestEvent(context.Context, RequestEvent) error
	RecordTestRun(context.Context, TestRun) error
	RecordAgentReview(context.Context, AgentReview) error
}

// MemoryStore is a local-first scaffold for development and tests before the
// runtime SQL backing is wired in.
type MemoryStore struct {
	mu           sync.Mutex
	incidents    []ClientIncident
	requests     []RequestEvent
	testRuns     []TestRun
	agentReviews []AgentReview
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

func (s *MemoryStore) RecordIncident(_ context.Context, incident ClientIncident) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.incidents = append(s.incidents, incident)
	return nil
}

func (s *MemoryStore) RecordRequestEvent(_ context.Context, event RequestEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.requests = append(s.requests, event)
	return nil
}

func (s *MemoryStore) RecordTestRun(_ context.Context, run TestRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.testRuns = append(s.testRuns, run)
	return nil
}

func (s *MemoryStore) RecordAgentReview(_ context.Context, review AgentReview) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.agentReviews = append(s.agentReviews, review)
	return nil
}

func (s *MemoryStore) Incidents() []ClientIncident {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]ClientIncident, len(s.incidents))
	copy(out, s.incidents)
	return out
}

func (s *MemoryStore) RequestEvents() []RequestEvent {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]RequestEvent, len(s.requests))
	copy(out, s.requests)
	return out
}

func (s *MemoryStore) TestRuns() []TestRun {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]TestRun, len(s.testRuns))
	copy(out, s.testRuns)
	return out
}

func (s *MemoryStore) AgentReviews() []AgentReview {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]AgentReview, len(s.agentReviews))
	copy(out, s.agentReviews)
	return out
}
