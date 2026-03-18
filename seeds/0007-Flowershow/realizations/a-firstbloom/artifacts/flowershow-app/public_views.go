package main

import (
	"fmt"
	"sort"
	"strings"
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
