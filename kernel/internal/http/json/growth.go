package jsontransport

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"as/kernel/internal/http/server"
	"as/kernel/internal/interactions"
	"as/kernel/internal/realizations"
)

type GrowthAPI struct {
	RepoRoot string
	Service  *interactions.RuntimeService
	Now      func() time.Time
}

type GrowthCommandInput struct {
	Reference             string `json:"reference"`
	Operation             string `json:"operation,omitempty"`
	CreateNew             bool   `json:"create_new,omitempty"`
	NewRealizationID      string `json:"new_realization_id,omitempty"`
	NewSummary            string `json:"new_summary,omitempty"`
	Profile               string `json:"profile,omitempty"`
	Target                string `json:"target,omitempty"`
	DeveloperInstructions string `json:"developer_instructions,omitempty"`
	IdempotencyKey        string `json:"idempotency_key,omitempty"`
}

func NewGrowthAPI(repoRoot string, service *interactions.RuntimeService) *GrowthAPI {
	return &GrowthAPI{
		RepoRoot: repoRoot,
		Service:  service,
		Now:      time.Now,
	}
}

func (api *GrowthAPI) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/projections/realization-growth/seed-packet", api.handleSeedPacket)
	mux.HandleFunc("POST /v1/commands/realizations.grow", api.handleGrow)
	mux.HandleFunc("GET /v1/projections/realization-growth/jobs/{job_id}", api.handleGrowthJob)
}

func (api *GrowthAPI) handleSeedPacket(w http.ResponseWriter, r *http.Request) {
	reference := strings.TrimSpace(r.URL.Query().Get("reference"))
	if reference == "" {
		respondError(w, http.StatusBadRequest, fmt.Errorf("reference is required"))
		return
	}

	packet, err := realizations.LoadGrowthContext(api.RepoRoot, reference)
	if err != nil {
		respondError(w, http.StatusBadRequest, server.BadRequest("realization reference could not be loaded"))
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"packet": packet,
		"profiles": []map[string]string{
			{"id": "minimal", "label": "Minimal", "summary": "Prefer the smallest coherent runnable change set."},
			{"id": "balanced", "label": "Balanced", "summary": "Balance pragmatism, polish, and correctness for the next growth pass."},
			{"id": "ornate", "label": "Ornate", "summary": "Push presentation, UX detail, and richer implementation choices where justified."},
			{"id": "custom", "label": "Custom", "summary": "Honor explicit developer instructions as the primary style constraint."},
		},
		"operations": []map[string]string{
			{"id": "grow", "label": "Grow", "summary": "Advance the seed toward a more complete realization."},
			{"id": "tweak", "label": "Tweak", "summary": "Make a targeted pass on an existing realization."},
			{"id": "validate", "label": "Validate", "summary": "Run review and validation work without broad product changes."},
		},
		"targets": []map[string]string{
			{"id": "runnable_mvp", "label": "Runnable MVP", "summary": "Prefer buildable runtime artifacts and coherent user-facing flows."},
			{"id": "api_first", "label": "API First", "summary": "Bias toward operational API shape and shared capability alignment."},
			{"id": "ux_surface", "label": "UX Surface", "summary": "Focus on the interactive surface and navigation without forcing full runtime completion."},
			{"id": "validation_only", "label": "Validation Only", "summary": "Produce checks, critiques, and next-step guidance instead of implementation."},
		},
	})
}

func (api *GrowthAPI) handleGrow(w http.ResponseWriter, r *http.Request) {
	if api.Service == nil {
		respondError(w, http.StatusServiceUnavailable, server.ServiceUnavailable("runtime service unavailable"))
		return
	}

	var input GrowthCommandInput
	if !decodeJSON(w, r, &input) {
		return
	}

	packet, err := realizations.LoadGrowthContext(api.RepoRoot, input.Reference)
	if err != nil {
		respondError(w, http.StatusBadRequest, server.BadRequest("realization reference could not be loaded"))
		return
	}

	operation := normalizedOperation(input.Operation)
	profile := normalizedProfile(input.Profile)
	target := normalizedTarget(input.Target, operation)
	targetReference, err := targetReferenceForGrowth(packet, input)
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}

	requestMeta := server.RequestMetadataFromContext(r.Context())
	resolvedSession, _ := server.SessionFromContext(r.Context())
	runAt := api.nowUTC()

	payload := map[string]any{
		"operation":              operation,
		"profile":                profile,
		"target":                 target,
		"source_reference":       packet.Reference,
		"target_reference":       targetReference,
		"create_new":             input.CreateNew,
		"new_realization_id":     strings.TrimSpace(input.NewRealizationID),
		"new_summary":            strings.TrimSpace(input.NewSummary),
		"developer_instructions": strings.TrimSpace(input.DeveloperInstructions),
		"requested_at":           runAt.Format(time.RFC3339),
		"request": map[string]any{
			"request_id":   requestMeta.RequestID,
			"session_id":   resolvedSession.SessionID,
			"principal_id": resolvedSession.PrincipalID,
			"seed_id":      packet.SeedID,
		},
		"seed_packet":  packet,
		"prompt_brief": buildGrowthPrompt(packet, targetReference, operation, profile, target, strings.TrimSpace(input.DeveloperInstructions)),
	}

	job, err := api.Service.EnqueueJob(r.Context(), interactions.EnqueueJobInput{
		Queue:       "realization-growth",
		Kind:        "realizations." + operation,
		DedupeKey:   strings.TrimSpace(input.IdempotencyKey),
		Priority:    priorityForOperation(operation),
		RunAt:       &runAt,
		MaxAttempts: 5,
		Payload:     payload,
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"command":          "realizations.grow",
		"job":              job,
		"operation":        operation,
		"profile":          profile,
		"target":           target,
		"target_reference": targetReference,
		"projection":       "/v1/projections/realization-growth/jobs/" + job.JobID,
	})
}

func (api *GrowthAPI) handleGrowthJob(w http.ResponseWriter, r *http.Request) {
	if api.Service == nil {
		respondError(w, http.StatusServiceUnavailable, server.ServiceUnavailable("runtime service unavailable"))
		return
	}

	job, err := api.Service.GetJob(r.Context(), r.PathValue("job_id"))
	if err != nil {
		writeRuntimeResult(w, nil, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"job":           job,
		"summary":       summarizeGrowthJob(job),
		"next_actions":  nextGrowthActions(job),
		"worker_queue":  "realization-growth",
		"worker_kind":   job.Kind,
		"target_status": targetStatusForJob(job),
	})
}

func (api *GrowthAPI) nowUTC() time.Time {
	if api != nil && api.Now != nil {
		return api.Now().UTC()
	}
	return time.Now().UTC()
}

func normalizedOperation(value string) string {
	v := strings.TrimSpace(value)
	switch v {
	case "tweak", "validate":
		return v
	default:
		return "grow"
	}
}

func normalizedProfile(value string) string {
	v := strings.TrimSpace(value)
	switch v {
	case "minimal", "ornate", "custom":
		return v
	default:
		return "balanced"
	}
}

func normalizedTarget(value, operation string) string {
	v := strings.TrimSpace(value)
	switch v {
	case "api_first", "ux_surface", "validation_only":
		return v
	case "runnable_mvp":
		return v
	default:
		if operation == "validate" {
			return "validation_only"
		}
		return "runnable_mvp"
	}
}

func targetReferenceForGrowth(packet realizations.GrowthContext, input GrowthCommandInput) (string, error) {
	if !input.CreateNew {
		return packet.Reference, nil
	}

	newRealizationID := strings.TrimSpace(input.NewRealizationID)
	if newRealizationID == "" {
		return "", fmt.Errorf("new_realization_id is required when create_new is true")
	}
	if strings.Contains(newRealizationID, "/") || strings.Contains(newRealizationID, " ") {
		return "", fmt.Errorf("new_realization_id must be a slash-free identifier")
	}
	return packet.SeedID + "/" + newRealizationID, nil
}

func priorityForOperation(operation string) int {
	switch operation {
	case "validate":
		return 80
	case "tweak":
		return 90
	default:
		return 100
	}
}

func buildGrowthPrompt(packet realizations.GrowthContext, targetReference, operation, profile, target, developerInstructions string) string {
	lines := []string{
		"Grow the AS seed into a realization update.",
		"seed_id: " + packet.SeedID,
		"source_reference: " + packet.Reference,
		"target_reference: " + targetReference,
		"operation: " + operation,
		"profile: " + profile,
		"target: " + target,
		"current_readiness: " + packet.Readiness.Label,
		"seed_summary: " + coalesce(packet.SeedSummary, packet.Summary),
	}

	if developerInstructions != "" {
		lines = append(lines, "developer_instructions: "+developerInstructions)
	}

	files := make([]string, 0, len(packet.SeedDocs)+len(packet.ApproachDocs)+len(packet.RealizationDocs)+len(packet.RuntimeDocs))
	appendPaths := func(items []realizations.ContextFile) {
		for _, item := range items {
			if strings.TrimSpace(item.Path) != "" {
				files = append(files, item.Path)
			}
		}
	}
	appendPaths(packet.SeedDocs)
	appendPaths(packet.ApproachDocs)
	appendPaths(packet.RealizationDocs)
	appendPaths(packet.RuntimeDocs)
	sort.Strings(files)
	lines = append(lines, "source_files: "+strings.Join(files, ", "))

	return strings.Join(lines, "\n")
}

func summarizeGrowthJob(job interactions.Job) string {
	targetReference := nestedString(job.Payload, "target_reference")
	operation := nestedString(job.Payload, "operation")
	profile := nestedString(job.Payload, "profile")
	if targetReference == "" {
		targetReference = nestedString(job.Payload, "source_reference")
	}

	parts := []string{
		strings.TrimSpace(job.Kind),
		strings.TrimSpace(operation),
		strings.TrimSpace(profile),
		strings.TrimSpace(targetReference),
	}
	filtered := parts[:0]
	for _, part := range parts {
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	if len(filtered) == 0 {
		return "queued realization growth job"
	}
	return strings.Join(filtered, " · ")
}

func nextGrowthActions(job interactions.Job) []string {
	switch job.Status {
	case "pending":
		return []string{
			"Claim the job from queue `realization-growth`.",
			"Load the seed packet and follow the prompt_brief plus linked docs.",
			"Write changes into the target realization and update runtime job status.",
		}
	case "running":
		return []string{
			"Inspect the claimed worker output and commit state changes back into the realization.",
			"Complete or fail the job with a clear result summary.",
		}
	case "completed":
		return []string{
			"Inspect the resulting realization diff or validation output.",
			"Queue a tweak or validate pass if another iteration is needed.",
		}
	default:
		return []string{
			"Inspect last_error and decide whether to retry, tweak instructions, or queue a fresh growth pass.",
		}
	}
}

func targetStatusForJob(job interactions.Job) string {
	switch job.Status {
	case "pending":
		return "queued"
	case "running":
		return "growing"
	case "completed":
		return "ready_for_review"
	default:
		return job.Status
	}
}

func nestedString(payload map[string]interface{}, key string) string {
	if payload == nil {
		return ""
	}
	if value, ok := payload[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func coalesce(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
