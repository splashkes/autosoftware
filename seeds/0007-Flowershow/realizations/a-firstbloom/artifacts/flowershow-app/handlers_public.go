package main

import (
	"net/http"
)

// --- Home Page ---

type homeData struct {
	Title       string
	CurrentPath string
	Shows       []*Show
	Orgs        []*Organization
}

func (a *app) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	a.render(w, "home.html", homeData{
		Title:       "Flowershow",
		CurrentPath: "/",
		Shows:       a.store.allShows(),
		Orgs:        a.store.allOrganizations(),
	})
}

// --- Show Detail ---

type showDetailData struct {
	Title       string
	CurrentPath string
	Show        *Show
	Schedule    *ShowSchedule
	Divisions   []*divisionView
	Entries     []*entryView
	Awards      []*AwardDefinition
	Org         *Organization
}

type divisionView struct {
	Division *Division
	Sections []*sectionView
}

type sectionView struct {
	Section *Section
	Classes []*ShowClass
}

type entryView struct {
	Entry  *Entry
	Person *Person
	Class  *ShowClass
	Media  []*Media
}

func (a *app) handleShowDetail(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	show, ok := a.store.showBySlug(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}

	org, _ := a.store.organizationByID(show.OrganizationID)
	sched, _ := a.store.scheduleByShowID(show.ID)

	var divisions []*divisionView
	if sched != nil {
		for _, div := range a.store.divisionsBySchedule(sched.ID) {
			dv := &divisionView{Division: div}
			for _, sec := range a.store.sectionsByDivision(div.ID) {
				sv := &sectionView{
					Section: sec,
					Classes: a.store.classesBySection(sec.ID),
				}
				dv.Sections = append(dv.Sections, sv)
			}
			divisions = append(divisions, dv)
		}
	}

	var entries []*entryView
	for _, e := range a.store.entriesByShow(show.ID) {
		person, _ := a.store.personByID(e.PersonID)
		cls, _ := a.store.classByID(e.ClassID)
		entries = append(entries, &entryView{
			Entry:  e,
			Person: person,
			Class:  cls,
			Media:  a.store.mediaByEntry(e.ID),
		})
	}

	awards := a.store.awardsByOrganization(show.OrganizationID)

	a.render(w, "show_detail.html", showDetailData{
		Title:       show.Name,
		CurrentPath: "/shows/" + slug,
		Show:        show,
		Schedule:    sched,
		Divisions:   divisions,
		Entries:     entries,
		Awards:      awards,
		Org:         org,
	})
}

// --- Class Browse ---

type classBrowseData struct {
	Title       string
	CurrentPath string
	Show        *Show
	Divisions   []*divisionView
}

func (a *app) handleClassBrowse(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	show, ok := a.store.showBySlug(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}

	sched, _ := a.store.scheduleByShowID(show.ID)
	var divisions []*divisionView
	if sched != nil {
		for _, div := range a.store.divisionsBySchedule(sched.ID) {
			dv := &divisionView{Division: div}
			for _, sec := range a.store.sectionsByDivision(div.ID) {
				sv := &sectionView{
					Section: sec,
					Classes: a.store.classesBySection(sec.ID),
				}
				dv.Sections = append(dv.Sections, sv)
			}
			divisions = append(divisions, dv)
		}
	}

	a.render(w, "class_browse.html", classBrowseData{
		Title:       "Classes — " + show.Name,
		CurrentPath: "/shows/" + slug + "/classes",
		Show:        show,
		Divisions:   divisions,
	})
}

// --- Class Detail ---

type classDetailData struct {
	Title       string
	CurrentPath string
	Show        *Show
	Class       *ShowClass
	Section     *Section
	Division    *Division
	Entries     []*entryView
}

func (a *app) handleClassDetail(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	classID := r.PathValue("classID")
	show, ok := a.store.showBySlug(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}

	cls, ok := a.store.classByID(classID)
	if !ok {
		http.NotFound(w, r)
		return
	}

	sec, _ := a.store.sectionByID(cls.SectionID)
	var div *Division
	if sec != nil {
		div, _ = a.store.divisionByID(sec.DivisionID)
	}

	var entries []*entryView
	for _, e := range a.store.entriesByClass(classID) {
		person, _ := a.store.personByID(e.PersonID)
		entries = append(entries, &entryView{
			Entry:  e,
			Person: person,
			Class:  cls,
			Media:  a.store.mediaByEntry(e.ID),
		})
	}

	a.render(w, "class_detail.html", classDetailData{
		Title:       cls.Title + " — " + show.Name,
		CurrentPath: "/shows/" + slug + "/classes/" + classID,
		Show:        show,
		Class:       cls,
		Section:     sec,
		Division:    div,
		Entries:     entries,
	})
}

// --- Entry Detail ---

type entryDetailData struct {
	Title       string
	CurrentPath string
	Entry       *Entry
	Person      *Person
	Class       *ShowClass
	Show        *Show
	Taxons      []*Taxon
	Scorecards  []*scorecardView
	Media       []*Media
}

type scorecardView struct {
	Scorecard *EntryScorecard
	Judge     *Person
	Scores    []*EntryCriterionScore
}

func (a *app) handleEntryDetail(w http.ResponseWriter, r *http.Request) {
	entryID := r.PathValue("entryID")
	entry, ok := a.store.entryByID(entryID)
	if !ok {
		http.NotFound(w, r)
		return
	}

	person, _ := a.store.personByID(entry.PersonID)
	cls, _ := a.store.classByID(entry.ClassID)
	show, _ := a.store.showByID(entry.ShowID)

	var taxons []*Taxon
	for _, ref := range entry.TaxonRefs {
		if t, ok := a.store.taxonByID(ref); ok {
			taxons = append(taxons, t)
		}
	}

	var scorecardViews []*scorecardView
	for _, sc := range a.store.scorecardsByEntry(entryID) {
		judge, _ := a.store.personByID(sc.JudgeID)
		scores := a.store.criterionScoresByScorecard(sc.ID)
		scorecardViews = append(scorecardViews, &scorecardView{
			Scorecard: sc,
			Judge:     judge,
			Scores:    scores,
		})
	}

	a.render(w, "entry_detail.html", entryDetailData{
		Title:       entry.Name,
		CurrentPath: "/entries/" + entryID,
		Entry:       entry,
		Person:      person,
		Class:       cls,
		Show:        show,
		Taxons:      taxons,
		Scorecards:  scorecardViews,
		Media:       a.store.mediaByEntry(entry.ID),
	})
}

// --- Taxonomy ---

type taxonomyData struct {
	Title       string
	CurrentPath string
	Taxons      []*Taxon
}

func (a *app) handleTaxonomyBrowse(w http.ResponseWriter, r *http.Request) {
	a.render(w, "taxonomy_browse.html", taxonomyData{
		Title:       "Taxonomy",
		CurrentPath: "/taxonomy",
		Taxons:      a.store.allTaxons(),
	})
}

type taxonDetailData struct {
	Title       string
	CurrentPath string
	Taxon       *Taxon
	Children    []*Taxon
	Entries     []*entryView
}

func (a *app) handleTaxonDetail(w http.ResponseWriter, r *http.Request) {
	taxonID := r.PathValue("taxonID")
	taxon, ok := a.store.taxonByID(taxonID)
	if !ok {
		http.NotFound(w, r)
		return
	}

	// Find child taxons
	var children []*Taxon
	for _, t := range a.store.allTaxons() {
		if t.ParentID == taxonID {
			children = append(children, t)
		}
	}

	// Find entries referencing this taxon
	var entries []*entryView
	for _, show := range a.store.allShows() {
		for _, e := range a.store.entriesByShow(show.ID) {
			for _, ref := range e.TaxonRefs {
				if ref == taxonID {
					person, _ := a.store.personByID(e.PersonID)
					cls, _ := a.store.classByID(e.ClassID)
					entries = append(entries, &entryView{
						Entry:  e,
						Person: person,
						Class:  cls,
						Media:  a.store.mediaByEntry(e.ID),
					})
					break
				}
			}
		}
	}

	a.render(w, "taxon_detail.html", taxonDetailData{
		Title:       taxon.Name,
		CurrentPath: "/taxonomy/" + taxonID,
		Taxon:       taxon,
		Children:    children,
		Entries:     entries,
	})
}

// --- Leaderboard ---

type leaderboardData struct {
	Title       string
	CurrentPath string
	Entries     []LeaderboardEntry
	OrgName     string
	Season      string
}

func (a *app) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	orgs := a.store.allOrganizations()
	orgID := r.URL.Query().Get("org")
	season := r.URL.Query().Get("season")
	if season == "" {
		season = "2025"
	}

	var orgName string
	if orgID == "" && len(orgs) > 0 {
		orgID = orgs[0].ID
	}
	if org, ok := a.store.organizationByID(orgID); ok {
		orgName = org.Name
	}

	a.render(w, "leaderboard.html", leaderboardData{
		Title:       "Leaderboard",
		CurrentPath: "/leaderboard",
		Entries:     a.store.leaderboard(orgID, season),
		OrgName:     orgName,
		Season:      season,
	})
}

// --- Standards ---

type standardsData struct {
	Title     string
	Standards []*StandardDocument
}

func (a *app) handleStandards(w http.ResponseWriter, r *http.Request) {
	a.render(w, "standards.html", standardsData{
		Title:     "Standards",
		Standards: a.store.allStandardDocuments(),
	})
}

// --- Show Rules ---

type showRulesData struct {
	Title    string
	Show     *Show
	Rules    []effectiveRule
	Schedule *ShowSchedule
}

func (a *app) handleShowRules(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	show, ok := a.store.showBySlug(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}

	sched, _ := a.store.scheduleByShowID(show.ID)
	var rules []effectiveRule
	if sched != nil && sched.EffectiveStandardEditionID != "" {
		for _, cls := range a.store.classesByShowID(show.ID) {
			classRules := a.store.effectiveRulesForClass(cls.ID, sched.EffectiveStandardEditionID)
			rules = append(rules, classRules...)
		}
	}

	a.render(w, "show_rules.html", showRulesData{
		Title:    "Rules — " + show.Name,
		Show:     show,
		Rules:    rules,
		Schedule: sched,
	})
}
