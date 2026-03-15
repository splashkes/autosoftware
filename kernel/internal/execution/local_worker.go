package execution

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"as/kernel/internal/interactions"
)

const (
	LocalBackendName    = "localprocess"
	executionQueueName  = "realization-execution"
	executionLaunchKind = "realizations.launch"
	executionStopKind   = "realizations.stop"
)

type LocalWorker struct {
	RepoRoot     string
	Runtime      *interactions.RuntimeService
	Capabilities CapabilityURLs
	WorkerName   string
	AutoActivate bool
	executor     *LocalExecutor
}

func NewLocalWorker(repoRoot string, runtime *interactions.RuntimeService, capabilities CapabilityURLs, workerName string, autoActivate bool) *LocalWorker {
	worker := &LocalWorker{
		RepoRoot:     repoRoot,
		Runtime:      runtime,
		Capabilities: capabilities,
		WorkerName:   workerName,
		AutoActivate: autoActivate,
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

	healthCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
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
		if _, err := w.Runtime.GetRealizationActivation(ctx, execution.SeedID); errors.Is(err, interactions.ErrNotFound) {
			_, _ = w.Runtime.ActivateRealization(ctx, interactions.ActivateRealizationInput{
				SeedID:      execution.SeedID,
				Reference:   execution.Reference,
				ExecutionID: execution.ExecutionID,
				Metadata:    map[string]interface{}{"reason": "first_healthy_execution"},
			})
		}
	}

	return w.syncBindings(ctx, execution, spec)
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
	if activation, err := w.Runtime.GetRealizationActivation(ctx, execution.SeedID); err == nil && activation.ExecutionID == executionID {
		_ = w.Runtime.DeleteRealizationActivation(ctx, execution.SeedID)
	}
	_ = w.Runtime.DeleteStableRouteBindingsForSeed(ctx, execution.SeedID)
	_ = w.Runtime.DeleteRealizationRouteBindings(ctx, executionID)
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

	activation, err := w.Runtime.GetRealizationActivation(ctx, execution.SeedID)
	if err == nil && activation.ExecutionID == execution.ExecutionID {
		_ = w.Runtime.DeleteStableRouteBindingsForSeed(ctx, execution.SeedID)
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
	if execution.Status == "stopped" || execution.Status == "failed" {
		return
	}
	lastError := ""
	if err != nil {
		lastError = err.Error()
	}
	_, _ = w.Runtime.UpdateRealizationExecution(ctx, executionID, interactions.UpdateRealizationExecutionInput{
		Status:    "stopped",
		StoppedAt: timePtr(time.Now().UTC()),
		LastError: lastError,
	})
	_, _ = w.Runtime.RecordRealizationExecutionEvent(ctx, interactions.RecordRealizationExecutionEventInput{
		ExecutionID: executionID,
		Name:        "process_exited",
		Data: map[string]interface{}{
			"error": lastError,
		},
	})
	_ = w.Runtime.DeleteRealizationRouteBindings(ctx, executionID)
	if activation, err := w.Runtime.GetRealizationActivation(ctx, execution.SeedID); err == nil && activation.ExecutionID == executionID {
		_ = w.Runtime.DeleteRealizationActivation(ctx, execution.SeedID)
		_ = w.Runtime.DeleteStableRouteBindingsForSeed(ctx, execution.SeedID)
	}
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
