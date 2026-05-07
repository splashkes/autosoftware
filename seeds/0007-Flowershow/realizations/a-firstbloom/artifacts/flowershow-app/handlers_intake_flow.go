package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// intakeTemplateMu guards lazy registration of the admin_intake_photos
// template. Done here (rather than in parseTemplates) to avoid touching
// shared template wiring while a sibling agent is editing template files in
// the same package.
var intakeTemplateMu sync.Mutex

func (a *app) ensureIntakeTemplateRegistered() {
	intakeTemplateMu.Lock()
	defer intakeTemplateMu.Unlock()
	if a.templates == nil {
		a.templates = map[string]*template.Template{}
	}
	if _, ok := a.templates["admin_intake_photos.html"]; ok {
		return
	}
	base := template.Must(template.New("base").Funcs(templateFuncMap).ParseFS(
		templates, "templates/base.html", "templates/partials/*.html",
	))
	clone, err := template.Must(base.Clone()).ParseFS(templates, "templates/admin_intake_photos.html")
	if err != nil {
		log.Printf("intake flow: parse admin_intake_photos.html: %v", err)
		return
	}
	a.templates["admin_intake_photos.html"] = clone
}

// === PR: intake flow ===
// Sequential photo intake handlers. Lets show staff (admins or
// show_intake_operator badges) walk through classes one at a time, snap a
// photo per entry, and have anonymous entries created on the fly.

type intakeClassNav struct {
	ID          string
	ClassNumber string
	Title       string
}

type intakePhotosData struct {
	Title         string
	CurrentPath   string
	ShowID        string
	Show          *Show
	ClassID       string
	Class         *ShowClass
	ClassNumber   string
	ClassTitle    string
	ClassPosition int
	ClassCount    int
	PrevClass     *intakeClassNav
	NextClass     *intakeClassNav
	Classes       []*intakeClassNav
	WorkspaceURL  string
	IntakeBaseURL string
	UploadURL     string
	AnonEntryURL  string
}

func (a *app) handleIntakeSequentialPhotos(w http.ResponseWriter, r *http.Request) {
	a.ensureIntakeTemplateRegistered()
	showID := strings.TrimSpace(r.PathValue("showID"))
	show, ok := a.store.showByID(showID)
	if !ok {
		http.NotFound(w, r)
		return
	}

	classes := a.store.classesByShowID(show.ID)
	classNavs := make([]*intakeClassNav, 0, len(classes))
	for _, cls := range classes {
		classNavs = append(classNavs, &intakeClassNav{
			ID:          cls.ID,
			ClassNumber: cls.ClassNumber,
			Title:       cls.Title,
		})
	}

	requestedClassID := strings.TrimSpace(r.URL.Query().Get("class"))
	var current *ShowClass
	currentIndex := -1
	for i, cls := range classes {
		if cls == nil {
			continue
		}
		if requestedClassID == "" || cls.ID == requestedClassID {
			current = cls
			currentIndex = i
			break
		}
	}
	if current == nil && len(classes) > 0 {
		current = classes[0]
		currentIndex = 0
	}

	var prev, next *intakeClassNav
	if len(classNavs) > 1 && current != nil {
		prevIndex := (currentIndex - 1 + len(classNavs)) % len(classNavs)
		nextIndex := (currentIndex + 1) % len(classNavs)
		prev = classNavs[prevIndex]
		next = classNavs[nextIndex]
	}

	classID := ""
	classNumber := ""
	classTitle := ""
	if current != nil {
		classID = current.ID
		classNumber = current.ClassNumber
		classTitle = current.Title
	}

	intakeBase := "/admin/shows/" + show.ID + "/intake/photos"
	data := intakePhotosData{
		Title:         "Photo intake — " + show.Name,
		CurrentPath:   intakeBase,
		ShowID:        show.ID,
		Show:          show,
		ClassID:       classID,
		Class:         current,
		ClassNumber:   classNumber,
		ClassTitle:    classTitle,
		ClassPosition: currentIndex + 1,
		ClassCount:    len(classNavs),
		PrevClass:     prev,
		NextClass:     next,
		Classes:       classNavs,
		WorkspaceURL:  "/admin/shows/" + show.ID,
		IntakeBaseURL: intakeBase,
		AnonEntryURL:  intakeBase + "/anon-entry",
	}
	a.render(w, r, "admin_intake_photos.html", data)
}

type intakeAnonEntryResponse struct {
	EntryID    string `json:"entry_id"`
	ClassID    string `json:"class_id"`
	ShowID     string `json:"show_id"`
	UploadURL  string `json:"upload_url"`
	EntrySeq   int    `json:"entry_seq"`
	EntryLabel string `json:"entry_label"`
}

// handleIntakeNewAnonymousEntry creates a brand new anonymous entry for the
// given show + class. The intake JS calls this before uploading the photo so
// it has an entry_id to attach the media to.
func (a *app) handleIntakeNewAnonymousEntry(w http.ResponseWriter, r *http.Request) {
	showID := strings.TrimSpace(r.PathValue("showID"))
	show, ok := a.store.showByID(showID)
	if !ok {
		http.NotFound(w, r)
		return
	}

	classID := strings.TrimSpace(r.URL.Query().Get("class"))
	if classID == "" {
		// Fall back to body-encoded class id for clients that POST form data.
		_ = r.ParseForm()
		classID = strings.TrimSpace(r.FormValue("class_id"))
	}
	cls, ok := a.store.classByID(classID)
	if !ok {
		http.Error(w, "class not found", http.StatusBadRequest)
		return
	}

	// Count existing entries on this class so we can label the new entry
	// "Anonymous · N" without ever colliding.
	existing := 0
	for _, e := range a.store.entriesByShow(show.ID) {
		if e == nil || e.ClassID != cls.ID {
			continue
		}
		existing++
	}
	seq := existing + 1

	entry, err := a.store.createEntry(EntryInput{
		ShowID:   show.ID,
		ClassID:  cls.ID,
		PersonID: "",
		Name:     "",
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.publishAdminSections(show.ID, "intake", "floor", "board")
	a.publishShowSummary(show.ID)

	resp := intakeAnonEntryResponse{
		EntryID:    entry.ID,
		ClassID:    cls.ID,
		ShowID:     show.ID,
		UploadURL:  "/admin/entries/" + entry.ID + "/media",
		EntrySeq:   seq,
		EntryLabel: anonEntryLabel(seq),
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

func anonEntryLabel(seq int) string {
	return "Anonymous · " + strconv.Itoa(seq)
}
