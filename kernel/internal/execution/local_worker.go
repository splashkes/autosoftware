package execution

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"as/kernel/internal/interactions"
	"as/kernel/internal/telemetry"
)

const (
	LocalBackendName    = "localprocess"
	executionQueueName  = "realization-execution"
	executionLaunchKind = "realizations.launch"
	executionStopKind   = "realizations.stop"
)

type LocalWorker struct {
	RepoRoot            string
	Runtime             *interactions.RuntimeService
	Capabilities        CapabilityURLs
	WorkerName          string
	AutoActivate        bool
	LaunchHealthTimeout time.Duration
	executor            *LocalExecutor
	Budgets             ResourceBudgets
	lastSampleAt        time.Time
	lastPrunedAt        time.Time
	breachCounts        map[string]int
}

type ResourceBudgets struct {
	MaxRSSBytes         int64
	MaxCPUPercent       float64
	MaxLogBytes         int64
	MaxOpenFDs          int
	ConsecutiveBreaches int
	SampleEvery         time.Duration
	TelemetryTTL        time.Duration
	RemediationTarget   string
	RemediationHint     string
}

func NewLocalWorker(repoRoot string, runtime *interactions.RuntimeService, capabilities CapabilityURLs, workerName string, autoActivate bool) *LocalWorker {
	worker := &LocalWorker{
		RepoRoot:            repoRoot,
		Runtime:             runtime,
		Capabilities:        capabilities,
		WorkerName:          workerName,
		AutoActivate:        autoActivate,
		LaunchHealthTimeout: DefaultHealthWaitTimeout,
		Budgets: ResourceBudgets{
			MaxRSSBytes:         512 * 1024 * 1024,
			MaxCPUPercent:       250,
			MaxLogBytes:         16 * 1024 * 1024,
			MaxOpenFDs:          256,
			ConsecutiveBreaches: 3,
			SampleEvery:         5 * time.Second,
			TelemetryTTL:        30 * 24 * time.Hour,
			RemediationTarget:   "main",
		},
		breachCounts: make(map[string]int),
	}
	worker.executor = NewLocalExecutor(worker.handleProcessExit)
	return worker
}

func (w *LocalWorker) ReconcileStartup(ctx context.Context) error {
	if w.Runtime == nil {
		return errors.New("runtime service unavailable")
	}
	if err := w.Runtime.ResetBackendExecutions(ctx, LocalBackendName, "local executor restarted"); err != nil {
		return err
	}
	return w.Runtime.ResetRunningJobs(ctx, executionQueueName, "local executor restarted before job completion")
}

func (w *LocalWorker) Tick(ctx context.Context) error {
	if w.Runtime == nil {
		return errors.New("runtime service unavailable")
	}
	jobs, err := w.Runtime.ClaimJobs(ctx, interactions.JobClaimInput{
		Queue:  executionQueueName,
		Worker: firstNonEmpty(strings.TrimSpace(w.WorkerName), "execd-local"),
		Limit:  1,
	})
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if err := w.handleJob(ctx, job); err != nil {
			_, _ = w.Runtime.FailJob(ctx, job.JobID, interactions.JobFailInput{Error: err.Error()})
			return err
		}
		_, _ = w.Runtime.CompleteJob(ctx, job.JobID, interactions.JobCompleteInput{})
	}
	if err := w.sampleAndEnforce(ctx); err != nil {
		return err
	}
	if err := w.pruneTelemetry(ctx); err != nil {
		return err
	}
	return nil
}

func (w *LocalWorker) handleJob(ctx context.Context, job interactions.Job) error {
	switch job.Kind {
	case executionLaunchKind:
		return w.handleLaunch(ctx, job)
	case executionStopKind:
		return w.handleStop(ctx, job)
	default:
		return fmt.Errorf("unsupported execution job kind %q", job.Kind)
	}
}

func (w *LocalWorker) handleLaunch(ctx context.Context, job interactions.Job) error {
	executionID := strings.TrimSpace(stringValue(job.Payload, "execution_id"))
	reference := strings.TrimSpace(stringValue(job.Payload, "reference"))
	if executionID == "" || reference == "" {
		return fmt.Errorf("execution launch job missing execution_id or reference")
	}

	execution, err := w.Runtime.UpdateRealizationExecution(ctx, executionID, interactions.UpdateRealizationExecutionInput{
		Status: "starting",
	})
	if err != nil {
		return err
	}
	_, _ = w.Runtime.RecordRealizationExecutionEvent(ctx, interactions.RecordRealizationExecutionEventInput{
		ExecutionID: executionID,
		Name:        "launch_started",
		Data: map[string]interface{}{
			"job_id": job.JobID,
		},
	})

	spec, err := BuildLocalSpec(w.RepoRoot, reference, executionID, w.Capabilities)
	if err != nil {
		_, _ = w.Runtime.UpdateRealizationExecution(ctx, executionID, interactions.UpdateRealizationExecutionInput{
			Status:    "failed",
			StoppedAt: timePtr(time.Now().UTC()),
			LastError: err.Error(),
		})
		_, _ = w.Runtime.RecordRealizationExecutionEvent(ctx, interactions.RecordRealizationExecutionEventInput{
			ExecutionID: executionID,
			Name:        "launch_failed",
			Data:        map[string]interface{}{"error": err.Error()},
		})
		return err
	}
	_, _ = w.Runtime.RecordRealizationExecutionEvent(ctx, interactions.RecordRealizationExecutionEventInput{
		ExecutionID: executionID,
		Name:        "launch_spec_resolved",
		Data: map[string]interface{}{
			"preview_path_prefix": spec.PreviewPathPrefix,
			"route_path_prefix":   spec.RoutePathPrefix,
			"route_subdomain":     spec.RouteSubdomain,
			"upstream_addr":       spec.UpstreamAddr,
		},
	})

	logPath, err := w.executor.Launch(ctx, spec)
	if err != nil {
		_, _ = w.Runtime.UpdateRealizationExecution(ctx, executionID, interactions.UpdateRealizationExecutionInput{
			Status:    "failed",
			StoppedAt: timePtr(time.Now().UTC()),
			LastError: err.Error(),
		})
		_, _ = w.Runtime.RecordRealizationExecutionEvent(ctx, interactions.RecordRealizationExecutionEventInput{
			ExecutionID: executionID,
			Name:        "launch_failed",
			Data:        map[string]interface{}{"error": err.Error()},
		})
		return err
	}

	_, err = w.Runtime.UpdateRealizationExecution(ctx, executionID, interactions.UpdateRealizationExecutionInput{
		Status:            "starting",
		UpstreamAddr:      spec.UpstreamAddr,
		PreviewPathPrefix: spec.PreviewPathPrefix,
		Metadata: map[string]interface{}{
			"log_file":     logPath,
			"runtime_kind": "source_local",
		},
	})
	if err != nil {
		return err
	}
	_, _ = w.Runtime.RecordRealizationExecutionEvent(ctx, interactions.RecordRealizationExecutionEventInput{
		ExecutionID: executionID,
		Name:        "process_started",
		Data: map[string]interface{}{
			"log_file":            logPath,
			"preview_path_prefix": spec.PreviewPathPrefix,
			"upstream_addr":       spec.UpstreamAddr,
		},
	})

	launchHealthTimeout := w.LaunchHealthTimeout
	if launchHealthTimeout <= 0 {
		launchHealthTimeout = DefaultHealthWaitTimeout
	}
	_, _ = w.Runtime.RecordRealizationExecutionEvent(ctx, interactions.RecordRealizationExecutionEventInput{
		ExecutionID: executionID,
		Name:        "health_check_started",
		Data: map[string]interface{}{
			"timeout_seconds": int(launchHealthTimeout / time.Second),
			"upstream_addr":   spec.UpstreamAddr,
		},
	})
	healthCtx, cancel := context.WithTimeout(ctx, launchHealthTimeout)
	defer cancel()
	if err := WaitForHealthy(healthCtx, spec.UpstreamAddr); err != nil {
		_, _ = w.Runtime.UpdateRealizationExecution(ctx, executionID, interactions.UpdateRealizationExecutionInput{
			Status:    "failed",
			StoppedAt: timePtr(time.Now().UTC()),
			LastError: err.Error(),
		})
		_, _ = w.Runtime.RecordRealizationExecutionEvent(ctx, interactions.RecordRealizationExecutionEventInput{
			ExecutionID: executionID,
			Name:        "health_failed",
			Data:        map[string]interface{}{"error": err.Error()},
		})
		_ = w.executor.Stop(executionID)
		_ = w.Runtime.DeleteRealizationRouteBindings(ctx, executionID)
		return err
	}

	execution, err = w.Runtime.UpdateRealizationExecution(ctx, executionID, interactions.UpdateRealizationExecutionInput{
		Status:            "healthy",
		UpstreamAddr:      spec.UpstreamAddr,
		PreviewPathPrefix: spec.PreviewPathPrefix,
		HealthyAt:         timePtr(time.Now().UTC()),
	})
	if err != nil {
		return err
	}
	_, _ = w.Runtime.RecordRealizationExecutionEvent(ctx, interactions.RecordRealizationExecutionEventInput{
		ExecutionID: executionID,
		Name:        "health_passed",
		Data:        map[string]interface{}{"upstream_addr": spec.UpstreamAddr},
	})

	if w.AutoActivate {
		if _, err := w.Runtime.GetRealizationActivation(ctx, execution.Reference); errors.Is(err, interactions.ErrNotFound) {
			_, _ = w.Runtime.ActivateRealization(ctx, interactions.ActivateRealizationInput{
				SeedID:      execution.SeedID,
				Reference:   execution.Reference,
				ExecutionID: execution.ExecutionID,
				Metadata:    map[string]interface{}{"reason": "first_healthy_execution_for_reference"},
			})
		}
	}

	if err := w.syncBindings(ctx, execution, spec); err != nil {
		return err
	}
	return w.Runtime.ClearRealizationSuspension(ctx, execution.Reference)
}

func (w *LocalWorker) handleStop(ctx context.Context, job interactions.Job) error {
	executionID := strings.TrimSpace(stringValue(job.Payload, "execution_id"))
	if executionID == "" {
		return fmt.Errorf("execution stop job missing execution_id")
	}
	execution, err := w.Runtime.GetRealizationExecution(ctx, executionID)
	if err != nil {
		return err
	}

	_ = w.executor.Stop(executionID)
	_, _ = w.Runtime.RecordRealizationExecutionEvent(ctx, interactions.RecordRealizationExecutionEventInput{
		ExecutionID: executionID,
		Name:        "stop_requested",
	})
	_, err = w.Runtime.UpdateRealizationExecution(ctx, executionID, interactions.UpdateRealizationExecutionInput{
		Status:    "stopped",
		StoppedAt: timePtr(time.Now().UTC()),
	})
	if err != nil {
		return err
	}
	if activation, err := w.Runtime.GetRealizationActivation(ctx, execution.Reference); err == nil && activation.ExecutionID == executionID {
		_ = w.Runtime.DeleteRealizationActivation(ctx, execution.Reference)
	}
	_ = w.Runtime.DeleteStableRouteBindingsForReference(ctx, execution.Reference)
	_ = w.Runtime.DeleteRealizationRouteBindings(ctx, executionID)
	delete(w.breachCounts, executionID)
	return nil
}

func (w *LocalWorker) syncBindings(ctx context.Context, execution interactions.RealizationExecution, spec LocalSpec) error {
	bindings := []interactions.RealizationRouteBindingInput{
		{
			ExecutionID:  execution.ExecutionID,
			SeedID:       execution.SeedID,
			Reference:    execution.Reference,
			BindingKind:  "preview_path",
			PathPrefix:   spec.PreviewPathPrefix,
			UpstreamAddr: spec.UpstreamAddr,
			Metadata:     map[string]interface{}{"preview": true},
		},
	}

	activation, err := w.Runtime.GetRealizationActivation(ctx, execution.Reference)
	if err == nil && activation.ExecutionID == execution.ExecutionID {
		_ = w.Runtime.DeleteStableRouteBindingsForReference(ctx, execution.Reference)
		if spec.RouteSubdomain != "" {
			bindings = append(bindings, interactions.RealizationRouteBindingInput{
				ExecutionID:  execution.ExecutionID,
				SeedID:       execution.SeedID,
				Reference:    execution.Reference,
				BindingKind:  "stable_subdomain",
				Subdomain:    spec.RouteSubdomain,
				UpstreamAddr: spec.UpstreamAddr,
				Metadata:     map[string]interface{}{"preview": false},
			})
		}
		if spec.RoutePathPrefix != "" {
			bindings = append(bindings, interactions.RealizationRouteBindingInput{
				ExecutionID:  execution.ExecutionID,
				SeedID:       execution.SeedID,
				Reference:    execution.Reference,
				BindingKind:  "stable_path",
				PathPrefix:   spec.RoutePathPrefix,
				UpstreamAddr: spec.UpstreamAddr,
				Metadata:     map[string]interface{}{"preview": false},
			})
		}
	} else if err != nil && !errors.Is(err, interactions.ErrNotFound) {
		return err
	}

	_, err = w.Runtime.ReplaceRealizationRouteBindings(ctx, execution.ExecutionID, bindings)
	if err != nil {
		return err
	}
	_, _ = w.Runtime.RecordRealizationExecutionEvent(ctx, interactions.RecordRealizationExecutionEventInput{
		ExecutionID: execution.ExecutionID,
		Name:        "route_registered",
		Data: map[string]interface{}{
			"bindings": len(bindings),
		},
	})
	return nil
}

func (w *LocalWorker) handleProcessExit(executionID string, err error) {
	if w.Runtime == nil || strings.TrimSpace(executionID) == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	execution, getErr := w.Runtime.GetRealizationExecution(ctx, executionID)
	if getErr != nil {
		return
	}
	if execution.Status == "stopped" || execution.Status == "failed" || execution.Status == "terminated" {
		return
	}
	rawError := ""
	if err != nil {
		rawError = err.Error()
	}
	reasonMessage := firstNonEmpty(rawError, "The realization process exited unexpectedly.")
	_, _ = w.Runtime.UpdateRealizationExecution(ctx, executionID, interactions.UpdateRealizationExecutionInput{
		Status:    "terminated",
		StoppedAt: timePtr(time.Now().UTC()),
		LastError: reasonMessage,
	})
	_, _ = w.Runtime.RecordRealizationExecutionEvent(ctx, interactions.RecordRealizationExecutionEventInput{
		ExecutionID: executionID,
		Name:        "process_exited",
		Data: map[string]interface{}{
			"error": rawError,
		},
	})
	_ = w.Runtime.DeleteRealizationRouteBindings(ctx, executionID)
	if activation, err := w.Runtime.GetRealizationActivation(ctx, execution.Reference); err == nil && activation.ExecutionID == executionID {
		_ = w.Runtime.DeleteRealizationActivation(ctx, execution.Reference)
		_ = w.Runtime.DeleteStableRouteBindingsForReference(ctx, execution.Reference)
	}
	_, _ = w.Runtime.UpsertRealizationSuspension(ctx, interactions.UpsertRealizationSuspensionInput{
		SeedID:            execution.SeedID,
		Reference:         execution.Reference,
		ExecutionID:       execution.ExecutionID,
		RouteSubdomain:    execution.RouteSubdomain,
		RoutePathPrefix:   execution.RoutePathPrefix,
		ReasonCode:        "unexpected_process_exit",
		Message:           reasonMessage,
		RemediationTarget: firstNonEmpty(strings.TrimSpace(w.Budgets.RemediationTarget), "main"),
		RemediationHint:   firstNonEmpty(strings.TrimSpace(w.Budgets.RemediationHint), "Fix the realization, open a PR, and relaunch once merged."),
		Metadata: map[string]interface{}{
			"execution_status": execution.Status,
		},
	})
	delete(w.breachCounts, executionID)
}

func (w *LocalWorker) sampleAndEnforce(ctx context.Context) error {
	if w.Runtime == nil {
		return nil
	}
	if w.Budgets.SampleEvery <= 0 {
		w.Budgets.SampleEvery = 5 * time.Second
	}
	if !w.lastSampleAt.IsZero() && time.Since(w.lastSampleAt) < w.Budgets.SampleEvery {
		return nil
	}
	w.lastSampleAt = time.Now()

	for _, process := range w.executor.RunningProcesses() {
		execution, err := w.Runtime.GetRealizationExecution(ctx, process.ExecutionID)
		if err != nil {
			continue
		}
		sample, err := telemetry.SampleOSProcess(process.PID, process.LogFile)
		if err != nil {
			_, _ = w.Runtime.RecordRealizationExecutionEvent(ctx, interactions.RecordRealizationExecutionEventInput{
				ExecutionID: execution.ExecutionID,
				Name:        "resource_sample_error",
				Data:        map[string]interface{}{"error": err.Error()},
			})
			continue
		}
		_, _ = w.Runtime.RecordProcessSample(ctx, interactions.RecordProcessSampleInput{
			ScopeKind:    "realization_execution",
			ExecutionID:  execution.ExecutionID,
			SeedID:       execution.SeedID,
			Reference:    execution.Reference,
			PID:          sample.PID,
			CPUPercent:   sample.CPUPercent,
			RSSBytes:     sample.RSSBytes,
			VirtualBytes: sample.VirtualBytes,
			OpenFDs:      sample.OpenFDs,
			LogBytes:     sample.LogBytes,
			Metadata: map[string]interface{}{
				"log_file": process.LogFile,
			},
			ObservedAt: &sample.ObservedAt,
		})

		reasonCode, reasonMessage, metadata, breached := w.evaluateBudget(sample)
		if !breached {
			w.breachCounts[execution.ExecutionID] = 0
			continue
		}

		w.breachCounts[execution.ExecutionID]++
		if w.breachCounts[execution.ExecutionID] < maxInt(w.Budgets.ConsecutiveBreaches, 1) {
			continue
		}

		if err := w.terminateExecution(ctx, execution, reasonCode, reasonMessage, metadata); err != nil {
			return err
		}
	}
	return nil
}

func (w *LocalWorker) evaluateBudget(sample telemetry.ProcessSample) (string, string, map[string]interface{}, bool) {
	if w.Budgets.MaxRSSBytes > 0 && sample.RSSBytes > w.Budgets.MaxRSSBytes {
		return "memory_budget_exceeded",
			fmt.Sprintf("This realization was shut down because memory usage exceeded the allowed budget (%d > %d bytes).", sample.RSSBytes, w.Budgets.MaxRSSBytes),
			map[string]interface{}{"rss_bytes": sample.RSSBytes, "max_rss_bytes": w.Budgets.MaxRSSBytes}, true
	}
	if w.Budgets.MaxLogBytes > 0 && sample.LogBytes > w.Budgets.MaxLogBytes {
		return "log_budget_exceeded",
			fmt.Sprintf("This realization was shut down because log output exceeded the allowed budget (%d > %d bytes).", sample.LogBytes, w.Budgets.MaxLogBytes),
			map[string]interface{}{"log_bytes": sample.LogBytes, "max_log_bytes": w.Budgets.MaxLogBytes}, true
	}
	if w.Budgets.MaxCPUPercent > 0 && sample.CPUPercent > w.Budgets.MaxCPUPercent {
		return "cpu_budget_exceeded",
			fmt.Sprintf("This realization was shut down because CPU usage exceeded the allowed budget (%.1f > %.1f).", sample.CPUPercent, w.Budgets.MaxCPUPercent),
			map[string]interface{}{"cpu_percent": sample.CPUPercent, "max_cpu_percent": w.Budgets.MaxCPUPercent}, true
	}
	if w.Budgets.MaxOpenFDs > 0 && sample.OpenFDs > w.Budgets.MaxOpenFDs {
		return "fd_budget_exceeded",
			fmt.Sprintf("This realization was shut down because open file descriptors exceeded the allowed budget (%d > %d).", sample.OpenFDs, w.Budgets.MaxOpenFDs),
			map[string]interface{}{"open_fds": sample.OpenFDs, "max_open_fds": w.Budgets.MaxOpenFDs}, true
	}
	return "", "", nil, false
}

func (w *LocalWorker) terminateExecution(ctx context.Context, execution interactions.RealizationExecution, reasonCode, reasonMessage string, metadata map[string]interface{}) error {
	_ = w.executor.Stop(execution.ExecutionID)
	_, err := w.Runtime.UpdateRealizationExecution(ctx, execution.ExecutionID, interactions.UpdateRealizationExecutionInput{
		Status:    "terminated",
		StoppedAt: timePtr(time.Now().UTC()),
		LastError: reasonMessage,
		Metadata: map[string]interface{}{
			"termination_reason_code": reasonCode,
		},
	})
	if err != nil {
		return err
	}
	_, _ = w.Runtime.RecordRealizationExecutionEvent(ctx, interactions.RecordRealizationExecutionEventInput{
		ExecutionID: execution.ExecutionID,
		Name:        "budget_terminated",
		Data: map[string]interface{}{
			"reason_code": reasonCode,
			"message":     reasonMessage,
			"details":     metadata,
		},
	})
	_, _ = w.Runtime.UpsertRealizationSuspension(ctx, interactions.UpsertRealizationSuspensionInput{
		SeedID:            execution.SeedID,
		Reference:         execution.Reference,
		ExecutionID:       execution.ExecutionID,
		RouteSubdomain:    execution.RouteSubdomain,
		RoutePathPrefix:   execution.RoutePathPrefix,
		ReasonCode:        reasonCode,
		Message:           reasonMessage,
		RemediationTarget: firstNonEmpty(strings.TrimSpace(w.Budgets.RemediationTarget), "main"),
		RemediationHint:   firstNonEmpty(strings.TrimSpace(w.Budgets.RemediationHint), "Fix the realization, open a PR, and relaunch once merged."),
		Metadata:          metadata,
	})
	if activation, err := w.Runtime.GetRealizationActivation(ctx, execution.Reference); err == nil && activation.ExecutionID == execution.ExecutionID {
		_ = w.Runtime.DeleteRealizationActivation(ctx, execution.Reference)
		_ = w.Runtime.DeleteStableRouteBindingsForReference(ctx, execution.Reference)
	}
	_ = w.Runtime.DeleteRealizationRouteBindings(ctx, execution.ExecutionID)
	delete(w.breachCounts, execution.ExecutionID)
	return nil
}

func (w *LocalWorker) pruneTelemetry(ctx context.Context) error {
	if w.Runtime == nil {
		return nil
	}
	if w.Budgets.TelemetryTTL <= 0 {
		w.Budgets.TelemetryTTL = 30 * 24 * time.Hour
	}
	if !w.lastPrunedAt.IsZero() && time.Since(w.lastPrunedAt) < 6*time.Hour {
		return nil
	}
	if err := w.Runtime.PruneOperationalTelemetry(ctx, w.Budgets.TelemetryTTL); err != nil {
		return err
	}
	w.lastPrunedAt = time.Now()
	return nil
}

func maxInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func stringValue(values map[string]interface{}, key string) string {
	raw, ok := values[key]
	if !ok {
		return ""
	}
	switch value := raw.(type) {
	case string:
		return value
	default:
		return fmt.Sprint(value)
	}
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
