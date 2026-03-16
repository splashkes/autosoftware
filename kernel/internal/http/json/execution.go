package jsontransport

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"as/kernel/internal/execution"
	"as/kernel/internal/http/server"
	"as/kernel/internal/interactions"
	"as/kernel/internal/realizations"
)

type ExecutionAPI struct {
	RepoRoot string
	Service  *interactions.RuntimeService
	Prefix   string
}

type LaunchRealizationInput struct {
	Reference string `json:"reference"`
}

type StopRealizationInput struct {
	ExecutionID string `json:"execution_id"`
}

type ActivateRealizationRequest struct {
	ExecutionID string `json:"execution_id"`
}

func NewExecutionAPI(repoRoot string, service *interactions.RuntimeService) *ExecutionAPI {
	return &ExecutionAPI{
		RepoRoot: repoRoot,
		Service:  service,
	}
}

func (api *ExecutionAPI) Register(mux *http.ServeMux) {
	api.RegisterPrefix(mux, "/v1")
}

func (api *ExecutionAPI) RegisterPrefix(mux *http.ServeMux, prefix string) {
	prefix = strings.TrimRight(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		prefix = "/v1"
	}
	api.Prefix = prefix
	mux.HandleFunc("POST "+prefix+"/commands/realizations.launch", api.handleLaunch)
	mux.HandleFunc("POST "+prefix+"/commands/realizations.stop", api.handleStop)
	mux.HandleFunc("POST "+prefix+"/commands/realizations.activate", api.handleActivate)
	mux.HandleFunc("GET "+prefix+"/projections/realization-execution/sessions", api.handleSessions)
	mux.HandleFunc("GET "+prefix+"/projections/realization-execution/sessions/{execution_id}", api.handleSession)
	mux.HandleFunc("GET "+prefix+"/projections/realization-execution/routes", api.handleRoutes)
}

func (api *ExecutionAPI) handleLaunch(w http.ResponseWriter, r *http.Request) {
	if api.Service == nil {
		respondError(w, http.StatusServiceUnavailable, errors.New("runtime service unavailable"))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	var input LaunchRealizationInput
	if !decodeJSON(w, r, &input) {
		return
	}
	reference := strings.TrimSpace(input.Reference)
	if reference == "" {
		respondError(w, http.StatusBadRequest, fmt.Errorf("reference is required"))
		return
	}

	packet, err := realizations.LoadGrowthContext(api.RepoRoot, reference)
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	if !packet.Readiness.CanLaunchLocal {
		respondError(w, http.StatusBadRequest, fmt.Errorf("realization %s is not launchable through the local execution backend", reference))
		return
	}

	requestMeta := server.RequestMetadataFromContext(r.Context())
	resolvedSession, _ := server.SessionFromContext(r.Context())
	suffix := requestMeta.RequestID
	if len(suffix) > 6 {
		suffix = suffix[len(suffix)-6:]
	}
	if strings.TrimSpace(suffix) == "" {
		suffix = fmt.Sprintf("%d", time.Now().UnixNano()%1000000)
	}
	executionID := "exec_" + strings.ReplaceAll(strings.ReplaceAll(reference, "/", "_"), "-", "_")
	executionID = executionID + "_" + suffix
	previewPath := execution.PreviewPath(executionID)

	execRow, err := api.Service.CreateRealizationExecution(r.Context(), interactions.CreateRealizationExecutionInput{
		ExecutionID:           executionID,
		Reference:             packet.Reference,
		SeedID:                packet.SeedID,
		RealizationID:         packet.RealizationID,
		Backend:               execution.LocalBackendName,
		Mode:                  "preview",
		Status:                "launch_requested",
		RouteSubdomain:        packet.Subdomain,
		RoutePathPrefix:       packet.PathPrefix,
		PreviewPathPrefix:     previewPath,
		LaunchedByPrincipalID: resolvedSession.PrincipalID,
		LaunchedBySessionID:   resolvedSession.SessionID,
		RequestID:             requestMeta.RequestID,
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}

	job, err := api.Service.EnqueueJob(r.Context(), interactions.EnqueueJobInput{
		Queue:    "realization-execution",
		Kind:     "realizations.launch",
		Priority: 120,
		Payload: map[string]interface{}{
			"execution_id": executionID,
			"reference":    packet.Reference,
			"seed_id":      packet.SeedID,
		},
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	_, _ = api.Service.RecordRealizationExecutionEvent(r.Context(), interactions.RecordRealizationExecutionEventInput{
		ExecutionID: executionID,
		Name:        "launch_requested",
		Data:        map[string]interface{}{"job_id": job.JobID},
	})

	respondJSON(w, http.StatusOK, map[string]any{
		"command":       "realizations.launch",
		"job":           job,
		"execution":     api.executionProjection(execRow, ""),
		"projection":    api.path("/projections/realization-execution/sessions/" + executionID),
		"open_path":     previewPath,
		"poll_after_ms": 350,
	})
}

func (api *ExecutionAPI) handleStop(w http.ResponseWriter, r *http.Request) {
	if api.Service == nil {
		respondError(w, http.StatusServiceUnavailable, errors.New("runtime service unavailable"))
		return
	}
	var input StopRealizationInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if strings.TrimSpace(input.ExecutionID) == "" {
		respondError(w, http.StatusBadRequest, fmt.Errorf("execution_id is required"))
		return
	}
	job, err := api.Service.EnqueueJob(r.Context(), interactions.EnqueueJobInput{
		Queue:    "realization-execution",
		Kind:     "realizations.stop",
		Priority: 140,
		Payload: map[string]interface{}{
			"execution_id": strings.TrimSpace(input.ExecutionID),
		},
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"command":    "realizations.stop",
		"job":        job,
		"projection": api.path("/projections/realization-execution/sessions/" + strings.TrimSpace(input.ExecutionID)),
	})
}

func (api *ExecutionAPI) handleActivate(w http.ResponseWriter, r *http.Request) {
	if api.Service == nil {
		respondError(w, http.StatusServiceUnavailable, errors.New("runtime service unavailable"))
		return
	}
	var input ActivateRealizationRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	execRow, err := api.Service.GetRealizationExecution(r.Context(), input.ExecutionID)
	if err != nil {
		writeRuntimeResult(w, nil, err)
		return
	}
	if execRow.Status != "healthy" {
		respondError(w, http.StatusBadRequest, fmt.Errorf("execution must be healthy before activation"))
		return
	}
	requestMeta := server.RequestMetadataFromContext(r.Context())
	resolvedSession, _ := server.SessionFromContext(r.Context())
	activation, err := api.Service.ActivateRealization(r.Context(), interactions.ActivateRealizationInput{
		SeedID:                 execRow.SeedID,
		Reference:              execRow.Reference,
		ExecutionID:            execRow.ExecutionID,
		ActivatedByPrincipalID: resolvedSession.PrincipalID,
		ActivatedBySessionID:   resolvedSession.SessionID,
		RequestID:              requestMeta.RequestID,
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	if err := syncActiveRouteBindings(r.Context(), api.Service, execRow); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"activation": activation,
		"execution":  api.executionProjection(execRow, activeRoutePathsByExecution(r.Context(), api.Service)[execRow.ExecutionID]),
	})
}

func (api *ExecutionAPI) handleSessions(w http.ResponseWriter, r *http.Request) {
	if api.Service == nil {
		respondError(w, http.StatusServiceUnavailable, errors.New("runtime service unavailable"))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	items, err := api.Service.ListRealizationExecutions(r.Context(), strings.TrimSpace(r.URL.Query().Get("reference")), 50)
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	openPaths := activeRoutePathsByExecution(r.Context(), api.Service)
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, api.executionProjection(item, openPaths[item.ExecutionID]))
	}
	respondJSON(w, http.StatusOK, map[string]any{"sessions": out})
}

func (api *ExecutionAPI) handleSession(w http.ResponseWriter, r *http.Request) {
	if api.Service == nil {
		respondError(w, http.StatusServiceUnavailable, errors.New("runtime service unavailable"))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	item, err := api.Service.GetRealizationExecution(r.Context(), r.PathValue("execution_id"))
	if err != nil {
		writeRuntimeResult(w, nil, err)
		return
	}
	events, err := api.Service.ListRealizationExecutionEvents(r.Context(), item.ExecutionID, 40)
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	openPaths := activeRoutePathsByExecution(r.Context(), api.Service)
	respondJSON(w, http.StatusOK, map[string]any{
		"session": api.executionProjection(item, openPaths[item.ExecutionID]),
		"events":  events,
	})
}

func (api *ExecutionAPI) handleRoutes(w http.ResponseWriter, r *http.Request) {
	if api.Service == nil {
		respondError(w, http.StatusServiceUnavailable, errors.New("runtime service unavailable"))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	items, err := api.Service.ListRealizationRouteBindings(r.Context(), true)
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"routes": items})
}

func (api *ExecutionAPI) executionProjection(item interactions.RealizationExecution, openPath string) map[string]any {
	return map[string]any{
		"execution_id":        item.ExecutionID,
		"reference":           item.Reference,
		"seed_id":             item.SeedID,
		"realization_id":      item.RealizationID,
		"backend":             item.Backend,
		"mode":                item.Mode,
		"status":              item.Status,
		"route_subdomain":     item.RouteSubdomain,
		"route_path_prefix":   item.RoutePathPrefix,
		"preview_path_prefix": item.PreviewPathPrefix,
		"upstream_addr":       item.UpstreamAddr,
		"started_at":          item.StartedAt,
		"healthy_at":          item.HealthyAt,
		"stopped_at":          item.StoppedAt,
		"last_error":          item.LastError,
		"metadata":            item.Metadata,
		"open_path":           strings.TrimSpace(openPath),
		"self":                api.path("/projections/realization-execution/sessions/" + item.ExecutionID),
	}
}

func activeRoutePathsByExecution(ctx context.Context, service *interactions.RuntimeService) map[string]string {
	if service == nil {
		return map[string]string{}
	}
	bindings, err := service.ListRealizationRouteBindings(ctx, true)
	if err != nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(bindings))
	priorities := make(map[string]int, len(bindings))
	for _, binding := range bindings {
		executionID := strings.TrimSpace(binding.ExecutionID)
		if executionID == "" {
			continue
		}
		candidate, priority := preferredExecutionOpenPath(binding)
		if candidate == "" || priority == 0 {
			continue
		}
		if existing, ok := priorities[executionID]; ok && existing >= priority {
			continue
		}
		out[executionID] = candidate
		priorities[executionID] = priority
	}
	return out
}

func preferredExecutionOpenPath(binding interactions.RealizationRouteBinding) (string, int) {
	pathPrefix := strings.TrimSpace(binding.PathPrefix)
	if pathPrefix == "" {
		return "", 0
	}
	if !strings.HasPrefix(pathPrefix, "/") {
		pathPrefix = "/" + pathPrefix
	}
	if !strings.HasSuffix(pathPrefix, "/") {
		pathPrefix += "/"
	}
	switch strings.TrimSpace(binding.BindingKind) {
	case "stable_path":
		return pathPrefix, 30
	case "preview_path":
		return pathPrefix, 20
	default:
		return pathPrefix, 10
	}
}

func syncActiveRouteBindings(ctx context.Context, service *interactions.RuntimeService, execRow interactions.RealizationExecution) error {
	if service == nil {
		return errors.New("runtime service unavailable")
	}
	if err := service.DeleteStableRouteBindingsForSeed(ctx, execRow.SeedID); err != nil {
		return err
	}
	bindings := []interactions.RealizationRouteBindingInput{
		{
			ExecutionID:  execRow.ExecutionID,
			SeedID:       execRow.SeedID,
			Reference:    execRow.Reference,
			BindingKind:  "preview_path",
			PathPrefix:   execRow.PreviewPathPrefix,
			UpstreamAddr: execRow.UpstreamAddr,
			Metadata:     map[string]interface{}{"preview": true},
		},
	}
	if execRow.RouteSubdomain != "" {
		bindings = append(bindings, interactions.RealizationRouteBindingInput{
			ExecutionID:  execRow.ExecutionID,
			SeedID:       execRow.SeedID,
			Reference:    execRow.Reference,
			BindingKind:  "stable_subdomain",
			Subdomain:    execRow.RouteSubdomain,
			UpstreamAddr: execRow.UpstreamAddr,
		})
	}
	if execRow.RoutePathPrefix != "" {
		bindings = append(bindings, interactions.RealizationRouteBindingInput{
			ExecutionID:  execRow.ExecutionID,
			SeedID:       execRow.SeedID,
			Reference:    execRow.Reference,
			BindingKind:  "stable_path",
			PathPrefix:   execRow.RoutePathPrefix,
			UpstreamAddr: execRow.UpstreamAddr,
		})
	}
	_, err := service.ReplaceRealizationRouteBindings(ctx, execRow.ExecutionID, bindings)
	return err
}

func (api *ExecutionAPI) path(suffix string) string {
	base := strings.TrimRight(strings.TrimSpace(api.Prefix), "/")
	if base == "" {
		base = "/v1"
	}
	return base + suffix
}
