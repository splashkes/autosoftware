package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
)

func adminSectionTemplate(section string) (string, bool) {
	templates := map[string]string{
		"info":       "admin_info_panel",
		"schedule":   "admin_schedule_panel",
		"entries":    "admin_entries_panel",
		"winners":    "admin_winners_panel",
		"scoring":    "admin_scoring_panel",
		"governance": "admin_governance_panel",
	}
	name, ok := templates[section]
	return name, ok
}

func (a *app) isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func (a *app) renderTemplateBlock(page, block string, data any) (string, error) {
	t, ok := a.templates[page]
	if !ok {
		return "", fmt.Errorf("template %q not found", page)
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, block, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (a *app) renderAdminSection(w http.ResponseWriter, showID, section string) {
	block, ok := adminSectionTemplate(section)
	if !ok {
		http.NotFound(w, nil)
		return
	}
	data, err := a.adminShowDetailData(showID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	html, err := a.renderTemplateBlock("show_admin.html", block, data)
	if err != nil {
		log.Printf("render admin section %s: %v", section, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}

func (a *app) publishAdminSections(showID string, sections ...string) {
	seen := map[string]bool{}
	for _, section := range sections {
		if seen[section] {
			continue
		}
		seen[section] = true
		block, ok := adminSectionTemplate(section)
		if !ok {
			continue
		}
		data, err := a.adminShowDetailData(showID)
		if err != nil {
			continue
		}
		html, err := a.renderTemplateBlock("show_admin.html", block, data)
		if err != nil {
			log.Printf("publish admin section %s: %v", section, err)
			continue
		}
		a.sseBroker.publish(showID, section+"-refresh", html)
	}
}

func (a *app) respondAdminSectionOrRedirect(w http.ResponseWriter, r *http.Request, showID, section string) {
	if a.isHTMX(r) {
		a.renderAdminSection(w, showID, section)
		return
	}
	http.Redirect(w, r, "/admin/shows/"+showID, http.StatusSeeOther)
}

func (a *app) handleAdminShowFragment(w http.ResponseWriter, r *http.Request) {
	showID := r.PathValue("showID")
	section := r.PathValue("section")
	a.renderAdminSection(w, showID, section)
}

func showIDForClass(store flowershowStore, classID string) string {
	cls, ok := store.classByID(classID)
	if !ok {
		return ""
	}
	sec, ok := store.sectionByID(cls.SectionID)
	if !ok {
		return ""
	}
	div, ok := store.divisionByID(sec.DivisionID)
	if !ok {
		return ""
	}
	for _, show := range store.allShows() {
		if sched, ok := store.scheduleByShowID(show.ID); ok && sched.ID == div.ShowScheduleID {
			return show.ID
		}
	}
	return ""
}
