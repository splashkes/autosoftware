package boot

import (
	feedbackloop "as/kernel/internal/feedback_loop"
	"as/kernel/internal/http/server"
)

type PinnedSelection struct {
	SeedID        string
	RealizationID string
}

type FeedbackLoopScriptConfig struct {
	EndpointPath         string
	Selection            PinnedSelection
	Request              server.RequestMetadata
	IncludeConsoleErrors bool
	IncludeHTMX          bool
}

// ClientFeedbackLoopScript is intended for local and preview boot surfaces that
// need to shorten the loop between a failing browser interaction and an agent
// reviewable incident in feedback-loop runtime storage.
func ClientFeedbackLoopScript(cfg FeedbackLoopScriptConfig) string {
	return feedbackloop.ClientReporterConfig{
		EndpointPath:         cfg.EndpointPath,
		SeedID:               cfg.Selection.SeedID,
		RealizationID:        cfg.Selection.RealizationID,
		RequestID:            cfg.Request.RequestID,
		SessionID:            cfg.Request.SessionID,
		IncludeConsoleErrors: cfg.IncludeConsoleErrors,
		IncludeHTMX:          cfg.IncludeHTMX,
	}.Script()
}
