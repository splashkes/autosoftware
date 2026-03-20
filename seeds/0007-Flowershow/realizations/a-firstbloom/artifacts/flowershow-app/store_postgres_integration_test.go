package main

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func dockerAvailable() bool {
	return exec.Command("docker", "version").Run() == nil
}

func freeTCPPort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for free port: %v", err)
	}
	defer ln.Close()
	return fmt.Sprintf("%d", ln.Addr().(*net.TCPAddr).Port)
}

func startTestPostgresContainer(t *testing.T) (string, func()) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping docker-backed postgres integration test in short mode")
	}
	if !dockerAvailable() {
		t.Skip("docker unavailable")
	}

	containerName := "flowershow-pg-" + newID("test")
	hostPort := freeTCPPort(t)
	password := newID("pgpass")
	run := exec.Command(
		"docker", "run", "-d", "--rm",
		"--name", containerName,
		"-e", "POSTGRES_USER=postgres",
		"-e", "POSTGRES_PASSWORD="+password,
		"-e", "POSTGRES_DB=flowershow_test",
		"-p", "127.0.0.1:"+hostPort+":5432",
		"postgres:16-alpine",
	)
	if out, err := run.CombinedOutput(); err != nil {
		t.Skipf("start postgres docker container: %v: %s", err, strings.TrimSpace(string(out)))
	}
	cleanup := func() {
		_ = exec.Command("docker", "rm", "-f", containerName).Run()
	}
	dsn := fmt.Sprintf("postgres://postgres:%s@127.0.0.1:%s/flowershow_test?sslmode=disable", password, hostPort)
	deadline := time.Now().Add(45 * time.Second)
	for {
		store, err := newFlowershowStore(dsn)
		if err == nil {
			store.Close()
			break
		}
		if time.Now().After(deadline) {
			cleanup()
			t.Fatalf("wait for postgres readiness: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
	return dsn, cleanup
}

func TestFlowershowProjectionTablesRebuildFromClaimsAfterTruncate(t *testing.T) {
	dsn, cleanup := startTestPostgresContainer(t)
	defer cleanup()

	storeRaw, err := newFlowershowStore(dsn)
	if err != nil {
		t.Fatalf("open postgres flowershow store: %v", err)
	}
	store, ok := storeRaw.(*postgresFlowershowStore)
	if !ok {
		t.Fatalf("expected postgresFlowershowStore, got %T", storeRaw)
	}

	org, err := store.createOrganization(Organization{Name: "Projection Rebuild Club", Level: "club"})
	if err != nil {
		t.Fatalf("create organization: %v", err)
	}
	show, err := store.createShow(ShowInput{
		OrganizationID: org.ID,
		Name:           "Projection Rebuild Show",
		Location:       "Hall A",
		Date:           "2026-09-15",
		Season:         "2026",
	})
	if err != nil {
		t.Fatalf("create show: %v", err)
	}
	schedule, err := store.createSchedule(ShowSchedule{ShowID: show.ID, Notes: "initial"})
	if err != nil {
		t.Fatalf("create schedule: %v", err)
	}
	division, err := store.createDivision(DivisionInput{
		ShowScheduleID: schedule.ID,
		Code:           "A",
		Title:          "Horticulture",
		Domain:         "horticulture",
		SortOrder:      1,
	})
	if err != nil {
		t.Fatalf("create division: %v", err)
	}
	section, err := store.createSection(SectionInput{
		DivisionID: division.ID,
		Code:       "1",
		Title:      "Annuals",
		SortOrder:  1,
	})
	if err != nil {
		t.Fatalf("create section: %v", err)
	}
	classPrimary, err := store.createClass(ShowClassInput{
		SectionID:     section.ID,
		ClassNumber:   "1",
		SortOrder:     1,
		Title:         "Calendula",
		Domain:        "horticulture",
		Description:   "Orange",
		SpecimenCount: 3,
		ScheduleNotes: "keep authored wording",
	})
	if err != nil {
		t.Fatalf("create class: %v", err)
	}
	classSecondary, err := store.createClass(ShowClassInput{
		SectionID:     section.ID,
		ClassNumber:   "2",
		SortOrder:     2,
		Title:         "Calendula",
		Domain:        "horticulture",
		Description:   "Yellow",
		SpecimenCount: 3,
	})
	if err != nil {
		t.Fatalf("create secondary class: %v", err)
	}
	person, err := store.createPerson(PersonInput{
		FirstName:         "Alice",
		LastName:          "Garden",
		Email:             "alice@example.com",
		PublicDisplayMode: "full_name",
		OrganizationID:    org.ID,
		OrganizationRole:  "member",
	})
	if err != nil {
		t.Fatalf("create person: %v", err)
	}
	entry, err := store.createEntry(EntryInput{
		ShowID:   show.ID,
		ClassID:  classPrimary.ID,
		PersonID: person.ID,
		Name:     "Calendula Entry",
	})
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	source, err := store.createSourceDocument(SourceDocument{
		OrganizationID:  org.ID,
		ShowID:          show.ID,
		Title:           "Schedule PDF",
		DocumentType:    "schedule",
		PublicationDate: "2026-09-01",
		SourceURL:       "https://example.com/schedule.pdf",
	})
	if err != nil {
		t.Fatalf("create source document: %v", err)
	}
	if _, err := store.createSourceCitation(SourceCitation{
		SourceDocumentID:     source.ID,
		TargetType:           "show_class",
		TargetID:             classPrimary.ID,
		PageFrom:             "1",
		PageTo:               "1",
		QuotedText:           "Calendula class",
		ExtractionConfidence: 0.99,
	}); err != nil {
		t.Fatalf("create source citation: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := store.pool.Exec(ctx, "TRUNCATE "+strings.Join(flowershowProjectionTables, ", ")); err != nil {
		t.Fatalf("truncate projection tables: %v", err)
	}
	counts, err := store.projectionCounts(ctx)
	if err != nil {
		t.Fatalf("projection counts after truncate: %v", err)
	}
	for table, count := range counts {
		if count != 0 {
			t.Fatalf("expected %s to be empty after truncate, got %d", table, count)
		}
	}
	store.Close()

	reopenedRaw, err := newFlowershowStore(dsn)
	if err != nil {
		t.Fatalf("reopen postgres flowershow store: %v", err)
	}
	defer reopenedRaw.Close()
	reopened, ok := reopenedRaw.(*postgresFlowershowStore)
	if !ok {
		t.Fatalf("expected reopened postgresFlowershowStore, got %T", reopenedRaw)
	}

	if got, ok := reopened.showByID(show.ID); !ok || got.Name != show.Name {
		t.Fatalf("reopened show mismatch: %#v", got)
	}
	if got, ok := reopened.entryByID(entry.ID); !ok || got.ClassID != classPrimary.ID {
		t.Fatalf("reopened entry mismatch: %#v", got)
	}
	rebuiltCounts, err := reopened.projectionCounts(ctx)
	if err != nil {
		t.Fatalf("projection counts after rebuild: %v", err)
	}
	if rebuiltCounts["as_flowershow_m_entries"] == 0 || rebuiltCounts["as_flowershow_m_classes"] == 0 {
		t.Fatalf("expected rebuilt projection counts, got %#v", rebuiltCounts)
	}

	if _, err := reopened.moveEntry(entry.ID, classSecondary.ID, "judge correction"); err != nil {
		t.Fatalf("move entry after rebuild: %v", err)
	}
	moved, ok := reopened.entryByID(entry.ID)
	if !ok || moved.ClassID != classSecondary.ID {
		t.Fatalf("moved entry mismatch after rebuild: %#v", moved)
	}
}
