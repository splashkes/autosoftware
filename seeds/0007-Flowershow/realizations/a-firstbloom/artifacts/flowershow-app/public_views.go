package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type winnerHeroCard struct {
	Entry  *Entry
	Person *Person
	Class  *ShowClass
	Show   *Show
	Media  []*Media
}

type classWinnerRow struct {
	Class  *ShowClass
	First  *entryView
	Second *entryView
	Third  *entryView
}

type winnerSummaryData struct {
	Title       string
	CurrentPath string
	Show        *Show
	HeroCards   []*winnerHeroCard
	ClassRows   []*classWinnerRow
}

type browseResult struct {
	Show          *Show
	Entry         *Entry
	Person        *Person
	Class         *ShowClass
	Media         []*Media
	Judges        []*Person
	MatchedTaxons []*Taxon
	MockTheme     string
	MockLabel     string
}

type browseData struct {
	Title           string
	CurrentPath     string
	Query           string
	SelectedOrgID   string
	SelectedTaxonID string
	SelectedJudgeID string
	SelectedDomain  string
	Orgs            []*Organization
	Taxons          []*Taxon
	Judges          []*Person
	Results         []*browseResult
}

type showVisualFrame struct {
	Label     string
	MediaPath string
	Theme     string
}

type homeShowCard struct {
	Show          *Show
	Org           *Organization
	EntryCount    int
	ClassCount    int
	Featured      []showVisualFrame
	SeasonLabel   string
	StatusLabel   string
	RelativeLabel string
}

type clubTopPerson struct {
	Person      *Person
	EntryCount  int
	TotalPoints float64
}

type clubCardView struct {
	Org           *Organization
	Parent        *Organization
	UpcomingShows []*Show
	PastShows     []*Show
	TopPeople     []*clubTopPerson
}

type homeData struct {
	Title         string
	CurrentPath   string
	UpcomingShows []*homeShowCard
	PastShows     []*homeShowCard
	Clubs         []*clubCardView
	TotalShows    int
	TotalEntries  int
	TotalMembers  int
}

type clubsData struct {
	Title       string
	CurrentPath string
	Clubs       []*clubCardView
}

type publicClubMemberView struct {
	Person      *Person
	Role        string
	EntryCount  int
	TotalPoints float64
}

type clubCreditView struct {
	Credit      *ShowCredit
	Person      *Person
	Show        *Show
	DisplayName string
}

type clubDetailData struct {
	Title         string
	CurrentPath   string
	Org           *Organization
	Parent        *Organization
	UpcomingShows []*homeShowCard
	PastShows     []*homeShowCard
	TopMembers    []*clubTopPerson
	Members       []*publicClubMemberView
	Credits       []*clubCreditView
	TotalShows    int
	TotalEntries  int
	TotalMembers  int
}

type publicClassCard struct {
	Show     *Show
	Org      *Organization
	Class    *ShowClass
	Featured showVisualFrame
}

type classesDomainView struct {
	Domain string
	Label  string
	Items  []*publicClassCard
}

type classesIndexData struct {
	Title       string
	CurrentPath string
	Query       string
	Domains     []*classesDomainView
}

type leaderboardBoard struct {
	Org     *Organization
	Entries []LeaderboardEntry
}

func (a *app) publicEntriesByShow(showID string) []*Entry {
	var out []*Entry
	for _, entry := range a.store.entriesByShow(showID) {
		if isPublicEntry(entry) {
			out = append(out, entry)
		}
	}
	return out
}

func (a *app) winnerSummaryDataBySlug(slug string) (winnerSummaryData, error) {
	show, ok := a.store.showBySlug(slug)
	if !ok {
		return winnerSummaryData{}, fmt.Errorf("show not found")
	}
	return a.winnerSummaryDataByShow(show)
}

func (a *app) winnerSummaryDataByShow(show *Show) (winnerSummaryData, error) {
	entries := a.publicEntriesByShow(show.ID)
	var heroCards []*winnerHeroCard
	classMap := map[string]*classWinnerRow{}
	for _, entry := range entries {
		if entry.Placement <= 0 || entry.Placement > 3 {
			continue
		}
		person, _ := a.store.personByID(entry.PersonID)
		class, _ := a.store.classByID(entry.ClassID)
		media := a.store.mediaByEntry(entry.ID)
		heroCards = append(heroCards, &winnerHeroCard{
			Entry:  entry,
			Person: person,
			Class:  class,
			Show:   show,
			Media:  media,
		})
		row, ok := classMap[entry.ClassID]
		if !ok {
			row = &classWinnerRow{Class: class}
			classMap[entry.ClassID] = row
		}
		view := &entryView{Entry: entry, Person: person, Class: class, Media: media}
		switch entry.Placement {
		case 1:
			row.First = view
		case 2:
			row.Second = view
		case 3:
			row.Third = view
		}
	}
	sort.Slice(heroCards, func(i, j int) bool {
		if heroCards[i].Entry.Placement == heroCards[j].Entry.Placement {
			return heroCards[i].Entry.Points > heroCards[j].Entry.Points
		}
		return heroCards[i].Entry.Placement < heroCards[j].Entry.Placement
	})
	if len(heroCards) > 9 {
		heroCards = heroCards[:9]
	}
	var classRows []*classWinnerRow
	for _, row := range classMap {
		classRows = append(classRows, row)
	}
	sort.Slice(classRows, func(i, j int) bool { return classRows[i].Class.ClassNumber < classRows[j].Class.ClassNumber })
	return winnerSummaryData{
		Title:       "Winners — " + show.Name,
		CurrentPath: "/shows/" + show.Slug + "/summary",
		Show:        show,
		HeroCards:   heroCards,
		ClassRows:   classRows,
	}, nil
}

func (a *app) publishShowSummary(showID string) {
	show, ok := a.store.showByID(showID)
	if !ok {
		return
	}
	data, err := a.winnerSummaryDataByShow(show)
	if err != nil {
		return
	}
	html, err := a.renderTemplateBlock("show_summary.html", "winner_summary_content", data)
	if err != nil {
		return
	}
	a.sseBroker.publish(showID, "summary-refresh", html)
}

func (a *app) browseResults(query, orgID, taxonID, judgeID, domain string) []*browseResult {
	query = strings.ToLower(strings.TrimSpace(query))
	taxonLookup := map[string]*Taxon{}
	for _, taxon := range a.store.allTaxons() {
		taxonLookup[taxon.ID] = taxon
	}
	var results []*browseResult
	for _, show := range a.store.allShows() {
		if orgID != "" && orgID != "all" && show.OrganizationID != orgID {
			continue
		}
		for _, entry := range a.publicEntriesByShow(show.ID) {
			class, _ := a.store.classByID(entry.ClassID)
			if domain != "" && class != nil && class.Domain != domain {
				continue
			}
			person, _ := a.store.personByID(entry.PersonID)
			var matchedTaxons []*Taxon
			matchesTaxon := taxonID == ""
			for _, ref := range entry.TaxonRefs {
				if taxon, ok := taxonLookup[ref]; ok {
					matchedTaxons = append(matchedTaxons, taxon)
					if ref == taxonID {
						matchesTaxon = true
					}
				}
			}
			if !matchesTaxon {
				continue
			}
			var judges []*Person
			if judgeID != "" {
				allowed := false
				for _, scorecard := range a.store.scorecardsByEntry(entry.ID) {
					if scorecard.JudgeID == judgeID {
						allowed = true
					}
					if judge, ok := a.store.personByID(scorecard.JudgeID); ok {
						judges = append(judges, judge)
					}
				}
				if !allowed {
					continue
				}
			} else {
				for _, scorecard := range a.store.scorecardsByEntry(entry.ID) {
					if judge, ok := a.store.personByID(scorecard.JudgeID); ok {
						judges = append(judges, judge)
					}
				}
			}
			if query != "" {
				fields := []string{entry.Name, show.Name}
				if class != nil {
					fields = append(fields, class.Title, class.ClassNumber, class.Domain)
				}
				for _, taxon := range matchedTaxons {
					fields = append(fields, taxon.Name, taxon.ScientificName)
				}
				joined := strings.ToLower(strings.Join(fields, " "))
				if !strings.Contains(joined, query) {
					continue
				}
			}
			results = append(results, &browseResult{
				Show:          show,
				Entry:         entry,
				Person:        person,
				Class:         class,
				Media:         a.store.mediaByEntry(entry.ID),
				Judges:        judges,
				MatchedTaxons: matchedTaxons,
				MockTheme:     []string{"rose", "fern", "gold"}[len(results)%3],
				MockLabel:     entry.Name,
			})
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Show.Date == results[j].Show.Date {
			return results[i].Entry.Name < results[j].Entry.Name
		}
		return results[i].Show.Date > results[j].Show.Date
	})
	return results
}

func (a *app) leaderboardBoards(season string) []*leaderboardBoard {
	var boards []*leaderboardBoard
	for _, org := range a.store.allOrganizations() {
		entries := a.store.leaderboard(org.ID, season)
		boards = append(boards, &leaderboardBoard{
			Org:     org,
			Entries: entries,
		})
	}
	return boards
}

func (a *app) homeShowCards(now time.Time) ([]*homeShowCard, []*homeShowCard) {
	var upcoming []*homeShowCard
	var past []*homeShowCard
	today := dateOnly(now)
	for _, show := range a.store.allShows() {
		card := a.homeShowCard(show, today)
		showDate, ok := parseShowDate(show.Date)
		if ok && showDate.Before(today) {
			past = append(past, card)
			continue
		}
		upcoming = append(upcoming, card)
	}
	sort.Slice(upcoming, func(i, j int) bool { return upcoming[i].Show.Date < upcoming[j].Show.Date })
	sort.Slice(past, func(i, j int) bool { return past[i].Show.Date > past[j].Show.Date })
	return upcoming, past
}

func (a *app) homeShowCard(show *Show, today time.Time) *homeShowCard {
	org, _ := a.store.organizationByID(show.OrganizationID)
	entryCount := len(a.publicEntriesByShow(show.ID))
	classCount := len(a.store.classesByShowID(show.ID))
	showDate, _ := parseShowDate(show.Date)
	relative := "Upcoming"
	if !showDate.IsZero() && showDate.Before(today) {
		relative = "Past show"
	}
	return &homeShowCard{
		Show:          show,
		Org:           org,
		EntryCount:    entryCount,
		ClassCount:    classCount,
		Featured:      a.showVisualFrames(show),
		SeasonLabel:   "Season " + show.Season,
		StatusLabel:   show.Status,
		RelativeLabel: relative,
	}
}

func (a *app) showVisualFrames(show *Show) []showVisualFrame {
	entries := a.publicEntriesByShow(show.ID)
	var frames []showVisualFrame
	themes := []string{"rose", "fern", "gold"}
	for _, entry := range entries {
		media := a.store.mediaByEntry(entry.ID)
		frame := showVisualFrame{
			Label: entry.Name,
			Theme: themes[len(frames)%len(themes)],
		}
		if len(media) > 0 {
			frame.MediaPath = "/media/" + media[0].ID
		}
		frames = append(frames, frame)
		if len(frames) == 3 {
			break
		}
	}
	if len(frames) == 0 {
		labels := []string{"Featured blooms", "Show highlights", "Class favorites"}
		for i, label := range labels {
			frames = append(frames, showVisualFrame{
				Label: label,
				Theme: themes[i%len(themes)],
			})
		}
	}
	return frames
}

func (a *app) clubCards(now time.Time) []*clubCardView {
	today := dateOnly(now)
	var cards []*clubCardView
	for _, org := range a.store.allOrganizations() {
		card := &clubCardView{Org: org}
		if org.ParentID != "" {
			card.Parent, _ = a.store.organizationByID(org.ParentID)
		}
		for _, show := range a.store.allShows() {
			if show.OrganizationID != org.ID {
				continue
			}
			showDate, _ := parseShowDate(show.Date)
			if !showDate.IsZero() && showDate.Before(today) {
				card.PastShows = append(card.PastShows, show)
			} else {
				card.UpcomingShows = append(card.UpcomingShows, show)
			}
		}
		sort.Slice(card.UpcomingShows, func(i, j int) bool { return card.UpcomingShows[i].Date < card.UpcomingShows[j].Date })
		sort.Slice(card.PastShows, func(i, j int) bool { return card.PastShows[i].Date > card.PastShows[j].Date })
		card.TopPeople = a.topPeopleForOrganization(org.ID, 3)
		cards = append(cards, card)
	}
	sort.Slice(cards, func(i, j int) bool { return cards[i].Org.Name < cards[j].Org.Name })
	return cards
}

func (a *app) clubDetailData(organizationID string, now time.Time) (clubDetailData, bool) {
	org, ok := a.store.organizationByID(organizationID)
	if !ok {
		return clubDetailData{}, false
	}
	today := dateOnly(now)
	data := clubDetailData{
		Title:       org.Name,
		CurrentPath: "/clubs/" + org.ID,
		Org:         org,
		TopMembers:  a.topPeopleForOrganization(org.ID, 8),
	}
	if org.ParentID != "" {
		data.Parent, _ = a.store.organizationByID(org.ParentID)
	}

	memberStats := map[string]*publicClubMemberView{}
	for _, person := range a.store.allPersons() {
		for _, link := range a.store.personOrganizationsByPerson(person.ID) {
			if link.OrganizationID != org.ID {
				continue
			}
			memberStats[person.ID] = &publicClubMemberView{
				Person: person,
				Role:   link.Role,
			}
			break
		}
	}

	for _, show := range a.store.allShows() {
		if show.OrganizationID != org.ID {
			continue
		}
		data.TotalShows++
		card := a.homeShowCard(show, today)
		showDate, ok := parseShowDate(show.Date)
		if ok && showDate.Before(today) {
			data.PastShows = append(data.PastShows, card)
		} else {
			data.UpcomingShows = append(data.UpcomingShows, card)
		}
		for _, entry := range a.publicEntriesByShow(show.ID) {
			data.TotalEntries++
			if item := memberStats[entry.PersonID]; item != nil {
				item.EntryCount++
				item.TotalPoints += entry.Points
			}
		}
		for _, credit := range a.store.showCreditsByShow(show.ID) {
			person, _ := a.store.personByID(credit.PersonID)
			displayName := strings.TrimSpace(credit.DisplayName)
			if displayName == "" && person != nil {
				displayName = publicPersonLabel(person)
			}
			if displayName == "" {
				displayName = "TBD"
			}
			data.Credits = append(data.Credits, &clubCreditView{
				Credit:      credit,
				Person:      person,
				Show:        show,
				DisplayName: displayName,
			})
		}
	}

	for _, item := range memberStats {
		data.Members = append(data.Members, item)
	}
	data.TotalMembers = len(data.Members)
	sort.Slice(data.UpcomingShows, func(i, j int) bool { return data.UpcomingShows[i].Show.Date < data.UpcomingShows[j].Show.Date })
	sort.Slice(data.PastShows, func(i, j int) bool { return data.PastShows[i].Show.Date > data.PastShows[j].Show.Date })
	sort.Slice(data.Members, func(i, j int) bool {
		if data.Members[i].TotalPoints == data.Members[j].TotalPoints {
			return publicPersonLabel(data.Members[i].Person) < publicPersonLabel(data.Members[j].Person)
		}
		return data.Members[i].TotalPoints > data.Members[j].TotalPoints
	})
	sort.Slice(data.Credits, func(i, j int) bool {
		if data.Credits[i].Show.Date == data.Credits[j].Show.Date {
			return data.Credits[i].Credit.SortOrder < data.Credits[j].Credit.SortOrder
		}
		return data.Credits[i].Show.Date > data.Credits[j].Show.Date
	})
	return data, true
}

func (a *app) topPeopleForOrganization(orgID string, limit int) []*clubTopPerson {
	type personStats struct {
		entryCount  int
		totalPoints float64
	}
	byPerson := map[string]*personStats{}
	for _, show := range a.store.allShows() {
		if show.OrganizationID != orgID {
			continue
		}
		for _, entry := range a.publicEntriesByShow(show.ID) {
			item := byPerson[entry.PersonID]
			if item == nil {
				item = &personStats{}
				byPerson[entry.PersonID] = item
			}
			item.entryCount++
			item.totalPoints += entry.Points
		}
	}
	var out []*clubTopPerson
	for personID, item := range byPerson {
		person, ok := a.store.personByID(personID)
		if !ok {
			continue
		}
		out = append(out, &clubTopPerson{
			Person:      person,
			EntryCount:  item.entryCount,
			TotalPoints: item.totalPoints,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TotalPoints == out[j].TotalPoints {
			return out[i].Person.Initials < out[j].Person.Initials
		}
		return out[i].TotalPoints > out[j].TotalPoints
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (a *app) classVisualFrame(show *Show, classID string) showVisualFrame {
	themes := []string{"rose", "fern", "gold"}
	idx := 0
	for _, entry := range a.publicEntriesByShow(show.ID) {
		if entry.ClassID != classID {
			continue
		}
		frame := showVisualFrame{
			Label: entry.Name,
			Theme: themes[idx%len(themes)],
		}
		media := a.store.mediaByEntry(entry.ID)
		if len(media) > 0 {
			frame.MediaPath = "/media/" + media[0].ID
		}
		return frame
	}
	frames := a.showVisualFrames(show)
	if len(frames) > 0 {
		return frames[0]
	}
	return showVisualFrame{Label: "Class highlights", Theme: "rose"}
}

func (a *app) classesIndexDomains(query string) []*classesDomainView {
	query = strings.ToLower(strings.TrimSpace(query))
	domains := map[string]*classesDomainView{}
	for _, show := range a.store.allShows() {
		org, _ := a.store.organizationByID(show.OrganizationID)
		for _, class := range a.store.classesByShowID(show.ID) {
			haystack := strings.ToLower(strings.Join([]string{
				class.ClassNumber,
				class.Title,
				class.Description,
				class.Domain,
				show.Name,
				show.Location,
				show.Date,
				func() string {
					if org == nil {
						return ""
					}
					return org.Name
				}(),
			}, " "))
			if query != "" && !strings.Contains(haystack, query) {
				continue
			}
			key := class.Domain
			if key == "" {
				key = "other"
			}
			view := domains[key]
			if view == nil {
				view = &classesDomainView{
					Domain: key,
					Label:  strings.Title(key),
				}
				domains[key] = view
			}
			view.Items = append(view.Items, &publicClassCard{
				Show:     show,
				Org:      org,
				Class:    class,
				Featured: a.classVisualFrame(show, class.ID),
			})
		}
	}
	var out []*classesDomainView
	for _, key := range []string{"horticulture", "design", "special", "other"} {
		if view := domains[key]; view != nil {
			sort.Slice(view.Items, func(i, j int) bool {
				if view.Items[i].Show.Name == view.Items[j].Show.Name {
					return view.Items[i].Class.ClassNumber < view.Items[j].Class.ClassNumber
				}
				return view.Items[i].Show.Name < view.Items[j].Show.Name
			})
			out = append(out, view)
			delete(domains, key)
		}
	}
	return out
}

func parseShowDate(raw string) (time.Time, bool) {
	if strings.TrimSpace(raw) == "" {
		return time.Time{}, false
	}
	value, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, false
	}
	return dateOnly(value), true
}

func dateOnly(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}
