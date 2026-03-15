package main

import (
	"testing"

	"as/kernel/internal/materializer"
	"as/kernel/internal/realizations"
)

func TestNewBootPageViewElevatesFeaturedSeeds(t *testing.T) {
	options := []materializer.RealizationOption{
		{
			Reference:     "0001-shared-notepad/a-go-htmx-room",
			SeedID:        "0001-shared-notepad",
			SeedSummary:   "Shared notepad",
			SeedStatus:    "accepted",
			RealizationID: "a-go-htmx-room",
			Summary:       "Shared notepad room",
			Status:        "accepted",
			Readiness:     realizations.ReadinessInfo{HasContract: true, CanRun: true, Label: "Runnable", Stage: "runnable"},
		},
		{
			Reference:     "0003-customer-service-app/a-web-mvp",
			SeedID:        "0003-customer-service-app",
			SeedSummary:   "Customer service app",
			SeedStatus:    "defined",
			RealizationID: "a-web-mvp",
			Summary:       "Customer service MVP",
			Status:        "draft",
			Readiness:     realizations.ReadinessInfo{HasContract: true, Label: "Defined", Stage: "defined"},
		},
		{
			Reference:     "0005-charity-auction-manager/a-web-mvp",
			SeedID:        "0005-charity-auction-manager",
			SeedSummary:   "Auction manager",
			SeedStatus:    "proposed",
			RealizationID: "a-web-mvp",
			Summary:       "Auction MVP",
			Status:        "draft",
			Readiness:     realizations.ReadinessInfo{HasContract: true, Label: "Defined", Stage: "defined"},
		},
	}

	view := newBootPageView(options, false, false, "", "")

	if len(view.FeaturedSeeds) == 0 {
		t.Fatal("expected featured seeds")
	}
	if view.FeaturedSeeds[0].SeedID != "0001-shared-notepad" {
		t.Fatalf("expected first featured seed to be 0001-shared-notepad, got %q", view.FeaturedSeeds[0].SeedID)
	}
	if !view.FeaturedSeeds[0].Featured {
		t.Fatal("expected featured seed to be marked featured")
	}
	if len(view.LibrarySeeds) != 0 {
		t.Fatalf("expected no library seeds after fallback fill, got %d", len(view.LibrarySeeds))
	}
}

func TestSelectFeaturedSeedsLeavesRemainderInLibrary(t *testing.T) {
	seeds := []seedBootView{
		{SeedID: "0001-shared-notepad"},
		{SeedID: "0003-customer-service-app"},
		{SeedID: "0004-event-listings"},
		{SeedID: "0005-charity-auction-manager"},
		{SeedID: "0006-registry-browser"},
	}

	featured, library := selectFeaturedSeeds(seeds, []string{
		"0001-shared-notepad",
		"0003-customer-service-app",
		"0004-event-listings",
		"0006-registry-browser",
	}, 4)

	if len(featured) != 4 {
		t.Fatalf("expected 4 featured seeds, got %d", len(featured))
	}
	if len(library) != 1 {
		t.Fatalf("expected 1 library seed, got %d", len(library))
	}
	if library[0].SeedID != "0005-charity-auction-manager" {
		t.Fatalf("expected 0005-charity-auction-manager in library, got %q", library[0].SeedID)
	}
	for _, seed := range featured {
		if !seed.Featured {
			t.Fatalf("expected featured seed %q to be marked featured", seed.SeedID)
		}
	}
	if library[0].Featured {
		t.Fatalf("expected library seed %q not to be marked featured", library[0].SeedID)
	}
}
