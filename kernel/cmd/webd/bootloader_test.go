package main

import (
	"testing"

	"as/kernel/internal/materializer"
	"as/kernel/internal/realizations"
)

func TestNewBootPageViewGroupsRealizationsBySeed(t *testing.T) {
	options := []materializer.RealizationOption{
		{
			Reference:     "0001-shared-notepad/a-go-htmx-room",
			SeedID:        "0001-shared-notepad",
			SeedSummary:   "Shared notepad",
			SeedStatus:    "accepted",
			RealizationID: "a-go-htmx-room",
			Summary:       "Shared notepad room",
			Status:        "accepted",
			Readiness: realizations.ReadinessInfo{
				HasContract:    true,
				CanRun:         true,
				CanLaunchLocal: true,
				Label:          "Runnable",
				Stage:          "runnable",
			},
			PathPrefix: "/notepad/",
		},
		{
			Reference:     "0006-registry-browser/a-authoritative-browser",
			SeedID:        "0006-registry-browser",
			SeedSummary:   "Registry browser",
			SeedStatus:    "defined",
			RealizationID: "a-authoritative-browser",
			Summary:       "Authoritative registry browser",
			Status:        "draft",
			Readiness: realizations.ReadinessInfo{
				HasContract: true,
				Label:       "Defined",
				Stage:       "defined",
			},
		},
		{
			Reference:     "0006-registry-browser/b-alt-pass",
			SeedID:        "0006-registry-browser",
			SeedSummary:   "Registry browser",
			SeedStatus:    "defined",
			RealizationID: "b-alt-pass",
			Summary:       "Alternate pass",
			Status:        "draft",
			Readiness: realizations.ReadinessInfo{
				HasContract: true,
				Label:       "Defined",
				Stage:       "defined",
			},
		},
	}

	view := newBootPageView(options, map[string]executionBootState{
		"0001-shared-notepad/a-go-htmx-room": {
			ExecutionID: "exec_notepad",
			Status:      "healthy",
			OpenPath:    "/__runs/exec_notepad/",
		},
	}, true, false, false, "", "")

	if len(view.Seeds) != 2 {
		t.Fatalf("expected 2 seeds, got %d", len(view.Seeds))
	}
	if view.RealizationCount != 3 {
		t.Fatalf("expected 3 realizations, got %d", view.RealizationCount)
	}
	if view.GrowthReadyCount != 3 {
		t.Fatalf("expected 3 growth-ready realizations, got %d", view.GrowthReadyCount)
	}
	if view.RunnableCount != 1 {
		t.Fatalf("expected 1 runnable realization, got %d", view.RunnableCount)
	}

	notepad := view.Seeds[0]
	if notepad.SeedID != "0001-shared-notepad" {
		t.Fatalf("expected first seed to be 0001-shared-notepad, got %q", notepad.SeedID)
	}
	if !notepad.InitiallyOpen {
		t.Fatal("expected first seed to be initially open")
	}
	if !notepad.IsSingleRealization {
		t.Fatal("expected notepad seed to have a single realization")
	}
	if notepad.RunnableCount != 1 {
		t.Fatalf("expected notepad runnable count 1, got %d", notepad.RunnableCount)
	}
	if got := notepad.Realizations[0].ExecutionOpenPath; got != "/__runs/exec_notepad/" {
		t.Fatalf("expected notepad execution open path /__runs/exec_notepad/, got %q", got)
	}

	registryBrowser := view.Seeds[1]
	if registryBrowser.SeedID != "0006-registry-browser" {
		t.Fatalf("expected second seed to be 0006-registry-browser, got %q", registryBrowser.SeedID)
	}
	if registryBrowser.IsSingleRealization {
		t.Fatal("expected registry browser seed to have multiple realizations")
	}
	if registryBrowser.Count != 2 {
		t.Fatalf("expected registry browser count 2, got %d", registryBrowser.Count)
	}
	if registryBrowser.GrowthReadyCount != 2 {
		t.Fatalf("expected registry browser growth-ready count 2, got %d", registryBrowser.GrowthReadyCount)
	}
}

func TestHumanizeSeedID(t *testing.T) {
	if got := humanizeSeedID("0006-registry-browser"); got != "Registry Browser" {
		t.Fatalf("expected Registry Browser, got %q", got)
	}
	if got := humanizeSeedID("custom_seed_name"); got != "Custom Seed Name" {
		t.Fatalf("expected Custom Seed Name, got %q", got)
	}
}
