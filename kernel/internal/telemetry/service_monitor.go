package telemetry

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"as/kernel/internal/interactions"
)

type ServiceMonitor struct {
	ServiceName string
	Runtime     *interactions.RuntimeService
	BootID      string
	PID         int
	SampleEvery time.Duration
}

func NewServiceMonitor(serviceName string, runtimeService *interactions.RuntimeService) *ServiceMonitor {
	return &ServiceMonitor{
		ServiceName: serviceName,
		Runtime:     runtimeService,
		BootID:      fmt.Sprintf("%s-%d", serviceName, time.Now().UnixNano()),
		PID:         os.Getpid(),
		SampleEvery: 15 * time.Second,
	}
}

func (m *ServiceMonitor) Start(ctx context.Context) {
	if m == nil || m.Runtime == nil || strings.TrimSpace(m.ServiceName) == "" {
		return
	}
	if m.SampleEvery <= 0 {
		m.SampleEvery = 15 * time.Second
	}
	go m.run(ctx)
}

func (m *ServiceMonitor) run(ctx context.Context) {
	_, _ = m.Runtime.RecordServiceEvent(ctx, interactions.RecordServiceEventInput{
		ServiceName: m.ServiceName,
		EventName:   "service_started",
		Severity:    "info",
		BootID:      m.BootID,
		PID:         m.PID,
		Metadata: map[string]interface{}{
			"boot_id": m.BootID,
		},
	})

	ticker := time.NewTicker(m.SampleEvery)
	defer ticker.Stop()
	for {
		m.recordSample(ctx)
		select {
		case <-ctx.Done():
			_, _ = m.Runtime.RecordServiceEvent(context.Background(), interactions.RecordServiceEventInput{
				ServiceName: m.ServiceName,
				EventName:   "service_stopped",
				Severity:    "info",
				BootID:      m.BootID,
				PID:         m.PID,
			})
			return
		case <-ticker.C:
		}
	}
}

func (m *ServiceMonitor) recordSample(ctx context.Context) {
	sample, err := SampleOSProcess(m.PID, "")
	if err != nil {
		_, _ = m.Runtime.RecordServiceEvent(ctx, interactions.RecordServiceEventInput{
			ServiceName: m.ServiceName,
			EventName:   "service_sample_error",
			Severity:    "warn",
			Message:     err.Error(),
			BootID:      m.BootID,
			PID:         m.PID,
		})
		return
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	_, _ = m.Runtime.RecordProcessSample(ctx, interactions.RecordProcessSampleInput{
		ScopeKind:    "kernel_service",
		ServiceName:  m.ServiceName,
		PID:          sample.PID,
		CPUPercent:   sample.CPUPercent,
		RSSBytes:     sample.RSSBytes,
		VirtualBytes: sample.VirtualBytes,
		OpenFDs:      sample.OpenFDs,
		LogBytes:     sample.LogBytes,
		Metadata: map[string]interface{}{
			"boot_id":         m.BootID,
			"go_goroutines":   runtime.NumGoroutine(),
			"go_heap_alloc":   mem.HeapAlloc,
			"go_heap_sys":     mem.HeapSys,
			"go_heap_objects": mem.HeapObjects,
			"go_num_gc":       mem.NumGC,
		},
		ObservedAt: &sample.ObservedAt,
	})
}
