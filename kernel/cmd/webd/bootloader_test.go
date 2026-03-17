package main

import (
	"strings"
	"testing"

	"as/kernel/internal/interactions"
	"as/kernel/internal/materializer"
	"as/kernel/internal/realizations"
	registrycatalog "as/kernel/internal/registry"
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

	view := newBootPageView(options, registrycatalog.Catalog{
		Realizations: []registrycatalog.CatalogRealization{
			{SeedID: "0003-customer-service-app"},
			{SeedID: "0006-registry-browser"},
			{SeedID: "0006-registry-browser"},
		},
		Objects: []registrycatalog.CatalogObject{
			{SeedID: "0003-customer-service-app"},
			{SeedID: "0006-registry-browser"},
		},
		Commands: []registrycatalog.CatalogCommand{
			{SeedID: "0003-customer-service-app"},
			{SeedID: "0006-registry-browser"},
			{SeedID: "0006-registry-browser"},
		},
		Projections: []registrycatalog.CatalogProjection{
			{SeedID: "0003-customer-service-app"},
			{SeedID: "0006-registry-browser"},
		},
	}, map[string]executionBootState{
		"0003-customer-service-app/a-web-mvp": {
			ExecutionID: "exec_support",
			Status:      "healthy",
			OpenPath:    "/support/",
		},
	}, true, false, false, "", "")

	if len(view.Seeds) != 2 {
		t.Fatalf("expected 2 seeds, got %d", len(view.Seeds))
	}
	if len(view.Featured) != 2 {
		t.Fatalf("expected 2 featured seeds, got %d", len(view.Featured))
	}
	if len(view.ReadinessGroups) != 2 {
		t.Fatalf("expected 2 readiness groups, got %d", len(view.ReadinessGroups))
	}
	if view.ReadinessGroups[0].Title != "Running Now" {
		t.Fatalf("expected first readiness group to be Running Now, got %q", view.ReadinessGroups[0].Title)
	}
	if len(view.ReadinessGroups[0].Realizations) != 1 {
		t.Fatalf("expected 1 running realization, got %d", len(view.ReadinessGroups[0].Realizations))
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
	if got := support.Realizations[0].RuntimeStateLabel; got != "Running" {
		t.Fatalf("expected support runtime state Running, got %q", got)
	}
	if support.Metrics.Total != 4 {
		t.Fatalf("expected support metrics total 4, got %d", support.Metrics.Total)
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
	if registryBrowser.Metrics.Total != 6 {
		t.Fatalf("expected registry browser metrics total 6, got %d", registryBrowser.Metrics.Total)
	}
}

func TestNewBootPageViewSkipsArchivedSeeds(t *testing.T) {
	options := []materializer.RealizationOption{
		{
			Reference:     "9998-archived-demo/a-retired-prototype",
			SeedID:        "9998-archived-demo",
			SeedSummary:   "Archived demo",
			SeedStatus:    "archived",
			RealizationID: "a-retired-prototype",
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

	view := newBootPageView(options, registrycatalog.Catalog{}, nil, true, false, false, "", "")

	if len(view.Seeds) != 1 {
		t.Fatalf("expected 1 visible seed, got %d", len(view.Seeds))
	}
	if view.Seeds[0].SeedID != "0006-registry-browser" {
		t.Fatalf("expected registry browser to remain visible, got %q", view.Seeds[0].SeedID)
	}
	if len(view.ReadinessGroups) != 1 {
		t.Fatalf("expected 1 readiness group, got %d", len(view.ReadinessGroups))
	}
	if len(view.ReadinessGroups[0].Realizations) != 1 {
		t.Fatalf("expected 1 visible realization, got %d", len(view.ReadinessGroups[0].Realizations))
	}
	if view.ReadinessGroups[0].Title != "Ready To Grow" {
		t.Fatalf("expected remaining readiness group to be Ready To Grow, got %q", view.ReadinessGroups[0].Title)
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
	body := []byte(`<link rel="stylesheet" href="/assets/app.css"><a href="/calendar">Calendar</a><a href="/v1/contracts/0004-event-listings/a-web-mvp">Contract</a><form action="/admin/login"></form><div hx-post="/chat/123"></div><script src="/__sprout-assets/sprout-logo.js"></script>`)
	got := string(rewriteMountedHTML(body, "/event-listings/"))

	wantContains := []string{
		`href="/event-listings/assets/app.css"`,
		`href="/event-listings/calendar"`,
		`href="/v1/contracts/0004-event-listings/a-web-mvp"`,
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
		"script-src 'self' 'unsafe-inline' https://unpkg.com https://static.cloudflareinsights.com",
		"style-src 'self' 'unsafe-inline'",
	}
	for _, want := range wantContains {
		if !strings.Contains(csp, want) {
			t.Fatalf("expected mounted realization CSP to contain %q, got %q", want, csp)
		}
	}
}
