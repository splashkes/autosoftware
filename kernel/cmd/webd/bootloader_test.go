package main

import (
	"strings"
	"testing"

	"as/kernel/internal/interactions"
	"as/kernel/internal/materializer"
	"as/kernel/internal/realizations"
)

func TestNewBootPageViewGroupsRealizationsBySeed(t *testing.T) {
	options := []materializer.RealizationOption{
		{
			Reference:     "0003-customer-service-app/a-web-mvp",
			SeedID:        "0003-customer-service-app",
			SeedSummary:   "Customer service app",
			SeedStatus:    "accepted",
			RealizationID: "a-web-mvp",
			Summary:       "Customer service app",
			Status:        "accepted",
			Readiness: realizations.ReadinessInfo{
				HasContract:    true,
				CanRun:         true,
				CanLaunchLocal: true,
				Label:          "Runnable",
				Stage:          "runnable",
			},
			PathPrefix: "/support/",
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
		"0003-customer-service-app/a-web-mvp": {
			ExecutionID: "exec_support",
			Status:      "healthy",
			OpenPath:    "/support/",
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

	support := view.Seeds[0]
	if support.SeedID != "0003-customer-service-app" {
		t.Fatalf("expected first seed to be 0003-customer-service-app, got %q", support.SeedID)
	}
	if !support.InitiallyOpen {
		t.Fatal("expected first seed to be initially open")
	}
	if !support.IsSingleRealization {
		t.Fatal("expected support seed to have a single realization")
	}
	if support.RunnableCount != 1 {
		t.Fatalf("expected support runnable count 1, got %d", support.RunnableCount)
	}
	if got := support.Realizations[0].ExecutionOpenPath; got != "/support/" {
		t.Fatalf("expected support execution open path /support/, got %q", got)
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

func TestNewBootPageViewSkipsArchivedSeeds(t *testing.T) {
	options := []materializer.RealizationOption{
		{
			Reference:     "0001-shared-notepad/a-go-htmx-room",
			SeedID:        "0001-shared-notepad",
			SeedSummary:   "Shared notepad",
			SeedStatus:    "archived",
			RealizationID: "a-go-htmx-room",
			Summary:       "Legacy prototype",
			Status:        "retired",
			Readiness: realizations.ReadinessInfo{
				HasContract:    true,
				HasRuntime:     true,
				CanRun:         true,
				CanLaunchLocal: true,
				Label:          "Runnable",
				Stage:          "runnable",
			},
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
	}

	view := newBootPageView(options, nil, true, false, false, "", "")

	if len(view.Seeds) != 1 {
		t.Fatalf("expected 1 visible seed, got %d", len(view.Seeds))
	}
	if view.Seeds[0].SeedID != "0006-registry-browser" {
		t.Fatalf("expected registry browser to remain visible, got %q", view.Seeds[0].SeedID)
	}
	if view.RealizationCount != 1 {
		t.Fatalf("expected 1 visible realization, got %d", view.RealizationCount)
	}
	if view.RunnableCount != 0 {
		t.Fatalf("expected archived runnable realization to be excluded, got %d runnable", view.RunnableCount)
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

func TestPreferredOpenPathForBindingPrefersStablePath(t *testing.T) {
	stable, stablePriority := preferredOpenPathForBinding(interactions.RealizationRouteBinding{
		BindingKind: "stable_path",
		PathPrefix:  "/event-listings/",
	})
	preview, previewPriority := preferredOpenPathForBinding(interactions.RealizationRouteBinding{
		BindingKind: "preview_path",
		PathPrefix:  "/__runs/exec_event/",
	})

	if stable != "/event-listings/" || stablePriority <= previewPriority {
		t.Fatalf("expected stable path to outrank preview path, got %q (%d) vs %q (%d)", stable, stablePriority, preview, previewPriority)
	}
}

func TestRewriteMountedHTMLPrefixesAppPathsButKeepsKernelPaths(t *testing.T) {
	body := []byte(`<link rel="stylesheet" href="/assets/app.css"><a href="/calendar">Calendar</a><form action="/admin/login"></form><div hx-post="/chat/123"></div><script src="/__sprout-assets/sprout-logo.js"></script>`)
	got := string(rewriteMountedHTML(body, "/event-listings/"))

	wantContains := []string{
		`href="/event-listings/assets/app.css"`,
		`href="/event-listings/calendar"`,
		`action="/event-listings/admin/login"`,
		`hx-post="/event-listings/chat/123"`,
		`src="/__sprout-assets/sprout-logo.js"`,
	}
	for _, want := range wantContains {
		if !strings.Contains(got, want) {
			t.Fatalf("expected rewritten HTML to contain %q, got %q", want, got)
		}
	}
}

func TestMountedRealizationContentSecurityPolicyAllowsCurrentRealizationAssets(t *testing.T) {
	csp := mountedRealizationContentSecurityPolicy()

	wantContains := []string{
		"default-src 'self'",
		"img-src 'self' data: https:",
		"script-src 'self' 'unsafe-inline' https://unpkg.com",
		"style-src 'self' 'unsafe-inline'",
	}
	for _, want := range wantContains {
		if !strings.Contains(csp, want) {
			t.Fatalf("expected mounted realization CSP to contain %q, got %q", want, csp)
		}
	}
}
