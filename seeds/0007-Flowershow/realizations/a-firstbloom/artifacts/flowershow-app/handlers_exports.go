package main

import (
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type exportSheet struct {
	Name    string
	Headers []string
	Rows    [][]string
}

type classExportMeta struct {
	Class         *ShowClass
	Section       *Section
	Division      *Division
	DivisionOrder int
	SectionOrder  int
}

type tallyGridColumn struct {
	Class *ShowClass
	Meta  classExportMeta
	Tally bool
	Block int
}

type tallyPersonReport struct {
	Person      *Person
	HortCounts  [4]int
	OtherCounts [4]int
	TotalPoints int
	EntryCount  int
	Rank        int
}

func (a *app) handleAdminShowExport(w http.ResponseWriter, r *http.Request) {
	a.handleShowExport(w, r, r.PathValue("showID"), r.PathValue("file"))
}

func (a *app) handleAPIShowExport(w http.ResponseWriter, r *http.Request) {
	a.handleShowExport(w, r, r.PathValue("id"), r.PathValue("file"))
}

func (a *app) handleShowExport(w http.ResponseWriter, r *http.Request, showID, file string) {
	show, ok := a.store.showByID(showID)
	if !ok {
		show, ok = a.store.showBySlug(showID)
	}
	if !ok {
		http.NotFound(w, r)
		return
	}

	switch file {
	case "entries.csv":
		a.writeCSVExport(w, show, a.entriesExportSheet(show))
	case "schedule.csv":
		a.writeCSVExport(w, show, a.scheduleExportSheet(show))
	case "leaderboard.csv":
		a.writeCSVExport(w, show, a.leaderboardExportSheet(show))
	case "scorecards.csv":
		a.writeCSVExport(w, show, a.scorecardsExportSheet(show))
	case "workbook.xls":
		a.writeWorkbookExport(w, show, []exportSheet{
			a.entriesExportSheet(show),
			a.scheduleExportSheet(show),
			a.leaderboardExportSheet(show),
			a.scorecardsExportSheet(show),
		})
	case "tally.xls":
		a.writeTallyWorkbookExport(w, show)
	default:
		http.NotFound(w, r)
	}
}

func (a *app) writeCSVExport(w http.ResponseWriter, show *Show, sheet exportSheet) {
	filename := exportFilename(show, sheet.Name, "csv")
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})

	cw := csv.NewWriter(w)
	_ = cw.Write(safeCSVRecord(sheet.Headers))
	for _, row := range sheet.Rows {
		_ = cw.Write(safeCSVRecord(row))
	}
	cw.Flush()
}

func (a *app) writeWorkbookExport(w http.ResponseWriter, show *Show, sheets []exportSheet) {
	filename := exportFilename(show, "workbook", "xls")
	w.Header().Set("Content-Type", "application/vnd.ms-excel; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)

	var b bytes.Buffer
	writeWorkbookStart(&b)
	for _, sheet := range sheets {
		writeExcelSheet(&b, sheet)
	}
	writeWorkbookEnd(&b)
	_, _ = w.Write(b.Bytes())
}

func (a *app) writeTallyWorkbookExport(w http.ResponseWriter, show *Show) {
	filename := exportFilename(show, "tally", "xls")
	w.Header().Set("Content-Type", "application/vnd.ms-excel; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)

	var b bytes.Buffer
	writeWorkbookStart(&b)
	a.writeShowTallySheet(&b, show)
	a.writePointSummarySheet(&b, show)
	a.writeParticipationSheet(&b, show)
	a.writeResultsSheet(&b, show)
	writeWorkbookEnd(&b)
	_, _ = w.Write(b.Bytes())
}

func (a *app) entriesExportSheet(show *Show) exportSheet {
	meta := a.classMetaByID(show)
	rows := make([][]string, 0)
	for _, entry := range a.store.entriesByShow(show.ID) {
		person, _ := a.store.personByID(entry.PersonID)
		cm := meta[entry.ClassID]
		row := []string{
			entry.ID,
			show.Name,
			classNumber(cm),
			classTitle(cm),
			divisionTitle(cm),
			sectionTitle(cm),
			entry.Name,
			personField(person, "first"),
			personField(person, "last"),
			personField(person, "initials"),
			personField(person, "email"),
			placementText(entry.Placement),
			formatFloat(entry.Points),
			formatCentsAmount(entry.FixedPrizeCents),
			entry.Notes,
			strings.Join(entry.TaxonRefs, "; "),
			entry.CreatedAt.UTC().Format(time.RFC3339),
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		return strings.Join([]string{rows[i][4], rows[i][5], rows[i][2], rows[i][6]}, "\x00") <
			strings.Join([]string{rows[j][4], rows[j][5], rows[j][2], rows[j][6]}, "\x00")
	})
	return exportSheet{
		Name: "entries",
		Headers: []string{
			"Entry ID", "Show", "Class Number", "Class Title", "Division", "Section", "Entry Name",
			"Exhibitor First Name", "Exhibitor Last Name", "Initials", "Email", "Placement",
			"Points", "Fixed Prize Amount", "Notes", "Taxon References", "Created At",
		},
		Rows: rows,
	}
}

func (a *app) scheduleExportSheet(show *Show) exportSheet {
	meta := a.classMetaByID(show)
	rows := make([][]string, 0, len(meta))
	for _, cm := range meta {
		cls := cm.Class
		row := []string{
			divisionCode(cm.Division),
			divisionTitle(cm),
			divisionDomain(cm.Division),
			strconv.Itoa(cm.DivisionOrder),
			sectionCode(cm.Section),
			sectionTitle(cm),
			strconv.Itoa(cm.SectionOrder),
			cls.ClassNumber,
			cls.Title,
			cls.Domain,
			cls.Description,
			intOrEmpty(cls.SpecimenCount),
			cls.Unit,
			cls.MeasurementRule,
			cls.NamingRequirement,
			cls.ContainerRule,
			cls.EligibilityRule,
			cls.ScheduleNotes,
			strings.Join(cls.TaxonRefs, "; "),
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		return strings.Join([]string{rows[i][3], rows[i][6], rows[i][7]}, "\x00") <
			strings.Join([]string{rows[j][3], rows[j][6], rows[j][7]}, "\x00")
	})
	return exportSheet{
		Name: "schedule",
		Headers: []string{
			"Division Code", "Division Title", "Division Domain", "Division Sort Order",
			"Section Code", "Section Title", "Section Sort Order", "Class Number", "Class Title",
			"Class Domain", "Description", "Specimen Count", "Unit", "Measurement Rule",
			"Naming Requirement", "Container Rule", "Eligibility Rule", "Schedule Notes", "Taxon References",
		},
		Rows: rows,
	}
}

func (a *app) leaderboardExportSheet(show *Show) exportSheet {
	orgName := ""
	if org, ok := a.store.organizationByID(show.OrganizationID); ok {
		orgName = org.Name
	}
	rows := make([][]string, 0)
	for _, entry := range a.store.leaderboard(show.OrganizationID, show.Season) {
		rows = append(rows, []string{
			strconv.Itoa(entry.Rank),
			entry.PersonID,
			entry.PersonName,
			entry.Initials,
			formatFloat(entry.TotalPoints),
			strconv.Itoa(entry.EntryCount),
			strconv.Itoa(entry.FirstCount),
			show.Season,
			orgName,
		})
	}
	return exportSheet{
		Name: "leaderboard",
		Headers: []string{
			"Rank", "Person ID", "Person Name", "Initials", "Total Points",
			"Entry Count", "First Place Count", "Season", "Organization",
		},
		Rows: rows,
	}
}

func (a *app) scorecardsExportSheet(show *Show) exportSheet {
	meta := a.classMetaByID(show)
	rows := make([][]string, 0)
	for _, entry := range a.store.entriesByShow(show.ID) {
		cm := meta[entry.ClassID]
		for _, scorecard := range a.store.scorecardsByEntry(entry.ID) {
			judge, _ := a.store.personByID(scorecard.JudgeID)
			rubricTitle := scorecard.RubricID
			if rubric, ok := a.store.rubricByID(scorecard.RubricID); ok {
				rubricTitle = rubric.Title
			}
			scores := a.store.criterionScoresByScorecard(scorecard.ID)
			if len(scores) == 0 {
				rows = append(rows, scorecardRow(entry, cm, scorecard, judge, rubricTitle, "", "", ""))
				continue
			}
			for _, score := range scores {
				criterionName := score.CriterionID
				for _, criterion := range a.store.criteriaByRubric(scorecard.RubricID) {
					if criterion.ID == score.CriterionID {
						criterionName = criterion.Name
						break
					}
				}
				rows = append(rows, scorecardRow(entry, cm, scorecard, judge, rubricTitle, criterionName, formatFloat(score.Score), score.Comment))
			}
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		return strings.Join([]string{rows[i][2], rows[i][4], rows[i][6]}, "\x00") <
			strings.Join([]string{rows[j][2], rows[j][4], rows[j][6]}, "\x00")
	})
	return exportSheet{
		Name: "scorecards",
		Headers: []string{
			"Scorecard ID", "Entry ID", "Entry Name", "Class Number", "Class Title",
			"Judge", "Rubric", "Total Score", "Notes", "Criterion", "Criterion Score", "Criterion Comment",
		},
		Rows: rows,
	}
}

func scorecardRow(entry *Entry, cm classExportMeta, scorecard *EntryScorecard, judge *Person, rubricTitle, criterionName, criterionScore, criterionComment string) []string {
	return []string{
		scorecard.ID,
		entry.ID,
		entry.Name,
		classNumber(cm),
		classTitle(cm),
		personName(judge),
		rubricTitle,
		formatFloat(scorecard.TotalScore),
		scorecard.Notes,
		criterionName,
		criterionScore,
		criterionComment,
	}
}

func (a *app) writeShowTallySheet(b *bytes.Buffer, show *Show) {
	classes := a.sortedClassMetas(show)
	columns := tallyGridColumns(classes, 17)
	placements := a.placementGrid(show)
	people := a.sortedPeople()

	writeWorksheetStart(b, "Show Tally")
	writeColumn(b, 150)
	for _, col := range columns {
		if col.Tally {
			writeColumn(b, 92)
		} else {
			writeColumn(b, 42)
		}
	}

	writeTallyTitleRow(b, "Tally", columns)
	writeTallyClassHeaderRow(b, columns)
	for i, person := range people {
		if i > 0 && i%8 == 0 {
			writeTallyClassHeaderRow(b, columns)
		}
		countsByBlock := tallyCountsByBlock(person.ID, columns, placements)
		row := []excelCell{{Value: personTallyName(person), StyleID: "name"}}
		for _, col := range columns {
			if col.Tally {
				row = append(row, excelCell{Value: formatTallyCounts(countsByBlock[col.Block]), StyleID: "tally"})
				continue
			}
			row = append(row, excelCell{Value: placements[person.ID][col.Class.ID], StyleID: "center"})
		}
		writeExcelCellsRow(b, row)
	}
	writeWorksheetEndWithOptions(b, true)
}

func (a *app) writePointSummarySheet(b *bytes.Buffer, show *Show) {
	reports := a.tallyPersonReports(show)

	writeWorksheetStart(b, "Point Summary")
	for _, width := range []float64{150, 96, 96, 96, 96, 96, 96, 90, 54} {
		writeColumn(b, width)
	}
	writeExcelCellsRow(b, []excelCell{{Value: "FLOWER SHOW TALLY SHEET", StyleID: "title", MergeAcross: 8}})
	writeExcelCellsRow(b, []excelCell{{Value: show.Name, StyleID: "subheader", MergeAcross: 7}, {Value: "RANK", StyleID: "header"}})
	writeExcelCellsRow(b, []excelCell{
		{}, {Value: "FLOWERS AND VEGETABLES", StyleID: "header", MergeAcross: 2},
		{Value: "DESIGNS AND SPECIAL EXHIBITS", StyleID: "header", MergeAcross: 2},
		{Value: "TOTAL POINTS", StyleID: "header"}, {Value: "RANK", StyleID: "header"},
	})
	writeExcelCellsRow(b, []excelCell{
		{Value: "Exhibitor", StyleID: "header"},
		{Value: "1st Place Wins", StyleID: "header"}, {Value: "2nd Place Wins", StyleID: "header"}, {Value: "3rd Place Wins", StyleID: "header"},
		{Value: "1st Place Wins", StyleID: "header"}, {Value: "2nd Place Wins", StyleID: "header"}, {Value: "3rd Place Wins", StyleID: "header"},
		{Value: "Total", StyleID: "header"}, {Value: "Rank", StyleID: "header"},
	})

	for _, report := range reports {
		row := []excelCell{
			{Value: personTallyName(report.Person), StyleID: "name"},
			{Value: formatCountPoints(report.HortCounts[1], 4), StyleID: "center"},
			{Value: formatCountPoints(report.HortCounts[2], 3), StyleID: "center"},
			{Value: formatCountPoints(report.HortCounts[3], 2), StyleID: "center"},
			{Value: formatCountPoints(report.OtherCounts[1], 12), StyleID: "center"},
			{Value: formatCountPoints(report.OtherCounts[2], 9), StyleID: "center"},
			{Value: formatCountPoints(report.OtherCounts[3], 6), StyleID: "center"},
			{Value: strconv.Itoa(report.TotalPoints), StyleID: "tally"},
			{Value: intOrEmpty(report.Rank), StyleID: "center"},
		}
		writeExcelCellsRow(b, row)
	}
	writeWorksheetEnd(b)
}

func (a *app) writeParticipationSheet(b *bytes.Buffer, show *Show) {
	seasonShows := a.seasonShows(show)
	people := a.sortedPeople()
	entryCounts := make(map[string]map[string]int)
	totalByPerson := make(map[string]int)
	for _, seasonShow := range seasonShows {
		for _, entry := range a.store.entriesByShow(seasonShow.ID) {
			if entryCounts[entry.PersonID] == nil {
				entryCounts[entry.PersonID] = make(map[string]int)
			}
			entryCounts[entry.PersonID][seasonShow.ID]++
			totalByPerson[entry.PersonID]++
		}
	}

	writeWorksheetStart(b, "Participation")
	writeColumn(b, 150)
	for range seasonShows {
		writeColumn(b, 92)
	}
	writeColumn(b, 80)
	writeColumn(b, 80)
	writeExcelCellsRow(b, []excelCell{{Value: "FLOWER SHOW PARTICIPATION SHEET", StyleID: "title", MergeAcross: len(seasonShows) + 2}})

	header := []excelCell{{Value: "Exhibitor", StyleID: "header"}}
	for _, seasonShow := range seasonShows {
		header = append(header, excelCell{Value: compactShowLabel(seasonShow), StyleID: "header"})
	}
	header = append(header, excelCell{Value: "Shows", StyleID: "header"}, excelCell{Value: "Entries", StyleID: "header"})
	writeExcelCellsRow(b, header)

	for _, person := range people {
		showsEntered := 0
		row := []excelCell{{Value: personTallyName(person), StyleID: "name"}}
		for _, seasonShow := range seasonShows {
			count := entryCounts[person.ID][seasonShow.ID]
			if count > 0 {
				showsEntered++
				row = append(row, excelCell{Value: strconv.Itoa(count), StyleID: "center"})
			} else {
				row = append(row, excelCell{Value: "", StyleID: "center"})
			}
		}
		row = append(row,
			excelCell{Value: intOrEmpty(showsEntered), StyleID: "center"},
			excelCell{Value: intOrEmpty(totalByPerson[person.ID]), StyleID: "center"},
		)
		writeExcelCellsRow(b, row)
	}
	writeWorksheetEnd(b)
}

func (a *app) writeResultsSheet(b *bytes.Buffer, show *Show) {
	reports := a.tallyPersonReports(show)
	firstPlaceEntries := a.firstPlaceEntries(show)
	newParticipants := a.newParticipants(show)

	writeWorksheetStart(b, "Results")
	for _, width := range []float64{120, 160, 80, 32, 170, 180, 32, 160, 110, 180} {
		writeColumn(b, width)
	}
	writeExcelCellsRow(b, []excelCell{{Value: show.Name, StyleID: "title", MergeAcross: 9}})
	writeExcelCellsRow(b, []excelCell{{Value: "Top Folks", StyleID: "header", MergeAcross: 2}, {}, {Value: "New Participants", StyleID: "header", MergeAcross: 1}, {}, {Value: "Best Ofs", StyleID: "header", MergeAcross: 3}})
	writeExcelCellsRow(b, []excelCell{
		{Value: "Rank", StyleID: "header"}, {Value: "Exhibitor", StyleID: "header"}, {Value: "Points", StyleID: "header"},
		{}, {Value: "Exhibitor", StyleID: "header"}, {Value: "Entries", StyleID: "header"},
		{}, {Value: "Class", StyleID: "header"}, {Value: "Entry", StyleID: "header"}, {Value: "Exhibitor", StyleID: "header"},
	})

	maxRows := maxInt(10, len(newParticipants), len(firstPlaceEntries))
	for i := 0; i < maxRows; i++ {
		row := make([]excelCell, 10)
		if i < len(reports) && reports[i].TotalPoints > 0 {
			row[0] = excelCell{Value: strconv.Itoa(reports[i].Rank), StyleID: "center"}
			row[1] = excelCell{Value: personTallyName(reports[i].Person), StyleID: "name"}
			row[2] = excelCell{Value: strconv.Itoa(reports[i].TotalPoints), StyleID: "tally"}
		}
		if i < len(newParticipants) {
			row[4] = excelCell{Value: personTallyName(newParticipants[i].Person), StyleID: "name"}
			row[5] = excelCell{Value: strconv.Itoa(newParticipants[i].EntryCount), StyleID: "center"}
		}
		if i < len(firstPlaceEntries) {
			entry := firstPlaceEntries[i]
			person, _ := a.store.personByID(entry.PersonID)
			class, _ := a.store.classByID(entry.ClassID)
			row[7] = excelCell{Value: classLabel(class), StyleID: "name"}
			row[8] = excelCell{Value: entry.Name, StyleID: "name"}
			row[9] = excelCell{Value: personTallyName(person), StyleID: "name"}
		}
		writeExcelCellsRow(b, row)
	}
	writeWorksheetEnd(b)
}

func (a *app) classMetaByID(show *Show) map[string]classExportMeta {
	out := make(map[string]classExportMeta)
	schedule, ok := a.store.scheduleByShowID(show.ID)
	if !ok {
		return out
	}
	for _, division := range a.store.divisionsBySchedule(schedule.ID) {
		for _, section := range a.store.sectionsByDivision(division.ID) {
			for _, class := range a.store.classesBySection(section.ID) {
				out[class.ID] = classExportMeta{
					Class:         class,
					Section:       section,
					Division:      division,
					DivisionOrder: division.SortOrder,
					SectionOrder:  section.SortOrder,
				}
			}
		}
	}
	return out
}

func (a *app) sortedClassMetas(show *Show) []classExportMeta {
	meta := a.classMetaByID(show)
	out := make([]classExportMeta, 0, len(meta))
	for _, cm := range meta {
		out = append(out, cm)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.DivisionOrder != b.DivisionOrder {
			return a.DivisionOrder < b.DivisionOrder
		}
		if a.SectionOrder != b.SectionOrder {
			return a.SectionOrder < b.SectionOrder
		}
		return classNumberLess(classNumber(a), classNumber(b))
	})
	return out
}

func (a *app) sortedPeople() []*Person {
	people := append([]*Person(nil), a.store.allPersons()...)
	sort.Slice(people, func(i, j int) bool {
		left := strings.ToLower(people[i].LastName + "\x00" + people[i].FirstName)
		right := strings.ToLower(people[j].LastName + "\x00" + people[j].FirstName)
		return left < right
	})
	return people
}

func (a *app) placementGrid(show *Show) map[string]map[string]string {
	grid := make(map[string]map[string]string)
	for _, entry := range a.store.entriesByShow(show.ID) {
		if grid[entry.PersonID] == nil {
			grid[entry.PersonID] = make(map[string]string)
		}
		marker := placementMarker(entry.Placement)
		existing := grid[entry.PersonID][entry.ClassID]
		if existing == "" {
			grid[entry.PersonID][entry.ClassID] = marker
		} else if !strings.Contains(existing, marker) {
			grid[entry.PersonID][entry.ClassID] = existing + "," + marker
		}
	}
	return grid
}

func (a *app) tallyPersonReports(show *Show) []tallyPersonReport {
	people := a.sortedPeople()
	reports := make([]tallyPersonReport, 0, len(people))
	byPerson := make(map[string]*tallyPersonReport, len(people))
	meta := a.classMetaByID(show)
	for _, person := range people {
		reports = append(reports, tallyPersonReport{Person: person})
		byPerson[person.ID] = &reports[len(reports)-1]
	}

	for _, entry := range a.store.entriesByShow(show.ID) {
		report := byPerson[entry.PersonID]
		if report == nil {
			continue
		}
		report.EntryCount++
		if entry.Placement < 1 || entry.Placement > 3 {
			continue
		}
		cm := meta[entry.ClassID]
		if isDesignOrSpecialDomain(classDomain(cm)) {
			report.OtherCounts[entry.Placement]++
		} else {
			report.HortCounts[entry.Placement]++
		}
		report.TotalPoints += operatorPointsForPlacement(classDomain(cm), entry.Placement)
	}

	sort.Slice(reports, func(i, j int) bool {
		if reports[i].TotalPoints != reports[j].TotalPoints {
			return reports[i].TotalPoints > reports[j].TotalPoints
		}
		return personTallyName(reports[i].Person) < personTallyName(reports[j].Person)
	})
	for i := range reports {
		if reports[i].TotalPoints > 0 {
			reports[i].Rank = i + 1
		}
	}
	return reports
}

func (a *app) seasonShows(show *Show) []*Show {
	shows := make([]*Show, 0)
	for _, candidate := range a.store.allShows() {
		if candidate.OrganizationID == show.OrganizationID && candidate.Season == show.Season {
			shows = append(shows, candidate)
		}
	}
	sort.Slice(shows, func(i, j int) bool {
		if shows[i].Date != shows[j].Date {
			return shows[i].Date < shows[j].Date
		}
		return shows[i].Name < shows[j].Name
	})
	return shows
}

func (a *app) firstPlaceEntries(show *Show) []*Entry {
	entries := make([]*Entry, 0)
	classOrder := make(map[string]int)
	for i, cm := range a.sortedClassMetas(show) {
		if cm.Class != nil {
			classOrder[cm.Class.ID] = i
		}
	}
	for _, entry := range a.store.entriesByShow(show.ID) {
		if entry.Placement == 1 {
			entries = append(entries, entry)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if classOrder[entries[i].ClassID] != classOrder[entries[j].ClassID] {
			return classOrder[entries[i].ClassID] < classOrder[entries[j].ClassID]
		}
		return entries[i].Name < entries[j].Name
	})
	return entries
}

func (a *app) newParticipants(show *Show) []tallyPersonReport {
	currentParticipants := make(map[string]int)
	for _, entry := range a.store.entriesByShow(show.ID) {
		currentParticipants[entry.PersonID]++
	}
	priorParticipants := make(map[string]bool)
	for _, candidate := range a.seasonShows(show) {
		if candidate.ID == show.ID {
			continue
		}
		if candidate.Date != "" && show.Date != "" && candidate.Date >= show.Date {
			continue
		}
		for _, entry := range a.store.entriesByShow(candidate.ID) {
			priorParticipants[entry.PersonID] = true
		}
	}

	var out []tallyPersonReport
	for personID, entryCount := range currentParticipants {
		if priorParticipants[personID] {
			continue
		}
		person, ok := a.store.personByID(personID)
		if !ok {
			continue
		}
		out = append(out, tallyPersonReport{Person: person, EntryCount: entryCount})
	}
	sort.Slice(out, func(i, j int) bool {
		return personTallyName(out[i].Person) < personTallyName(out[j].Person)
	})
	return out
}

func tallyGridColumns(classes []classExportMeta, blockSize int) []tallyGridColumn {
	if blockSize <= 0 {
		blockSize = 17
	}
	var out []tallyGridColumn
	block := 0
	lastWasTally := false
	for i, cm := range classes {
		out = append(out, tallyGridColumn{Class: cm.Class, Meta: cm, Block: block})
		lastWasTally = false
		if (i+1)%blockSize == 0 {
			out = append(out, tallyGridColumn{Tally: true, Block: block})
			block++
			lastWasTally = true
		}
	}
	if len(classes) > 0 && !lastWasTally {
		out = append(out, tallyGridColumn{Tally: true, Block: block})
	}
	return out
}

func tallyCountsByBlock(personID string, columns []tallyGridColumn, placements map[string]map[string]string) map[int][4]int {
	out := make(map[int][4]int)
	for _, col := range columns {
		if col.Tally || col.Class == nil {
			continue
		}
		marker := placements[personID][col.Class.ID]
		if len(marker) == 0 {
			continue
		}
		placement, err := strconv.Atoi(marker[:1])
		if err != nil || placement < 1 || placement > 3 {
			continue
		}
		counts := out[col.Block]
		counts[placement]++
		out[col.Block] = counts
	}
	return out
}

func writeTallyTitleRow(b *bytes.Buffer, tallyLabel string, columns []tallyGridColumn) {
	row := []excelCell{{Value: "", StyleID: "subheader"}}
	for i := 0; i < len(columns); {
		col := columns[i]
		if col.Tally {
			row = append(row, excelCell{Value: tallyLabel, StyleID: "header"})
			i++
			continue
		}
		if isDesignOrSpecialDomain(classDomain(col.Meta)) {
			mergeAcross := 0
			for j := i + 1; j < len(columns); j++ {
				if columns[j].Tally || !isDesignOrSpecialDomain(classDomain(columns[j].Meta)) {
					break
				}
				mergeAcross++
			}
			row = append(row, excelCell{Value: "Design and Special Exhibits", StyleID: "subheader", MergeAcross: mergeAcross})
			i += mergeAcross + 1
			continue
		}
		row = append(row, excelCell{Value: "", StyleID: "subheader"})
		i++
	}
	writeExcelCellsRow(b, row)
}

func writeTallyClassHeaderRow(b *bytes.Buffer, columns []tallyGridColumn) {
	row := []excelCell{{Value: "Class No.", StyleID: "header"}}
	for _, col := range columns {
		if col.Tally {
			row = append(row, excelCell{Value: "#1s    #2s    #3s", StyleID: "header"})
			continue
		}
		row = append(row, excelCell{Value: classNumber(col.Meta), StyleID: "header"})
	}
	writeExcelCellsRow(b, row)
}

func writeWorkbookStart(b *bytes.Buffer) {
	b.WriteString(xml.Header)
	b.WriteString(`<?mso-application progid="Excel.Sheet"?>` + "\n")
	b.WriteString(`<Workbook xmlns="urn:schemas-microsoft-com:office:spreadsheet" xmlns:ss="urn:schemas-microsoft-com:office:spreadsheet" xmlns:x="urn:schemas-microsoft-com:office:excel">` + "\n")
	b.WriteString(`<Styles>`)
	b.WriteString(`<Style ss:ID="Default" ss:Name="Normal"><Alignment ss:Vertical="Center"/></Style>`)
	b.WriteString(`<Style ss:ID="title"><Font ss:Bold="1" ss:Size="14"/><Alignment ss:Horizontal="Center" ss:Vertical="Center"/></Style>`)
	b.WriteString(`<Style ss:ID="header"><Font ss:Bold="1"/><Interior ss:Color="#D8F3DC" ss:Pattern="Solid"/><Alignment ss:Horizontal="Center" ss:Vertical="Center" ss:WrapText="1"/><Borders><Border ss:Position="Bottom" ss:LineStyle="Continuous" ss:Weight="1"/></Borders></Style>`)
	b.WriteString(`<Style ss:ID="subheader"><Font ss:Bold="1"/><Interior ss:Color="#FDF8E1" ss:Pattern="Solid"/><Alignment ss:Horizontal="Center" ss:Vertical="Center" ss:WrapText="1"/></Style>`)
	b.WriteString(`<Style ss:ID="name"><Alignment ss:Vertical="Center"/><Borders><Border ss:Position="Bottom" ss:LineStyle="Continuous" ss:Weight="1" ss:Color="#E7E5E4"/></Borders></Style>`)
	b.WriteString(`<Style ss:ID="center"><Alignment ss:Horizontal="Center" ss:Vertical="Center"/><Borders><Border ss:Position="Bottom" ss:LineStyle="Continuous" ss:Weight="1" ss:Color="#E7E5E4"/></Borders></Style>`)
	b.WriteString(`<Style ss:ID="tally"><Font ss:Bold="1"/><Alignment ss:Horizontal="Center" ss:Vertical="Center"/><Interior ss:Color="#F5F5F4" ss:Pattern="Solid"/><Borders><Border ss:Position="Bottom" ss:LineStyle="Continuous" ss:Weight="1" ss:Color="#D6D3D1"/></Borders></Style>`)
	b.WriteString(`</Styles>` + "\n")
}

func writeWorkbookEnd(b *bytes.Buffer) {
	b.WriteString(`</Workbook>` + "\n")
}

type excelCell struct {
	Value       string
	StyleID     string
	MergeAcross int
}

func writeWorksheetStart(b *bytes.Buffer, name string) {
	b.WriteString(`<Worksheet ss:Name="`)
	xml.EscapeText(b, []byte(excelSheetName(name)))
	b.WriteString(`"><Table>` + "\n")
}

func writeWorksheetEnd(b *bytes.Buffer) {
	b.WriteString(`</Table></Worksheet>` + "\n")
}

func writeWorksheetEndWithOptions(b *bytes.Buffer, freezeTopLeft bool) {
	b.WriteString(`</Table>`)
	if freezeTopLeft {
		b.WriteString(`<x:WorksheetOptions><x:FreezePanes/><x:FrozenNoSplit/><x:SplitHorizontal>2</x:SplitHorizontal><x:TopRowBottomPane>2</x:TopRowBottomPane><x:SplitVertical>1</x:SplitVertical><x:LeftColumnRightPane>1</x:LeftColumnRightPane><x:ActivePane>0</x:ActivePane></x:WorksheetOptions>`)
	}
	b.WriteString(`</Worksheet>` + "\n")
}

func writeColumn(b *bytes.Buffer, width float64) {
	b.WriteString(`<Column ss:Width="`)
	b.WriteString(strconv.FormatFloat(width, 'f', -1, 64))
	b.WriteString(`"/>` + "\n")
}

func writeExcelCellsRow(b *bytes.Buffer, row []excelCell) {
	b.WriteString("<Row>")
	for _, cell := range row {
		b.WriteString("<Cell")
		if cell.StyleID != "" {
			b.WriteString(` ss:StyleID="`)
			xml.EscapeText(b, []byte(cell.StyleID))
			b.WriteString(`"`)
		}
		if cell.MergeAcross > 0 {
			b.WriteString(` ss:MergeAcross="`)
			b.WriteString(strconv.Itoa(cell.MergeAcross))
			b.WriteString(`"`)
		}
		b.WriteString(`><Data ss:Type="String">`)
		xml.EscapeText(b, []byte(excelSafeString(cell.Value)))
		b.WriteString("</Data></Cell>")
	}
	b.WriteString("</Row>\n")
}

func writeExcelSheet(b *bytes.Buffer, sheet exportSheet) {
	b.WriteString(`<Worksheet ss:Name="`)
	xml.EscapeText(b, []byte(excelSheetName(sheet.Name)))
	b.WriteString(`"><Table>` + "\n")
	writeExcelRow(b, sheet.Headers, true)
	for _, row := range sheet.Rows {
		writeExcelRow(b, row, false)
	}
	b.WriteString(`</Table></Worksheet>` + "\n")
}

func writeExcelRow(b *bytes.Buffer, row []string, header bool) {
	b.WriteString("<Row>")
	for _, cell := range row {
		style := ""
		if header {
			style = ` ss:StyleID="header"`
		}
		b.WriteString("<Cell" + style + `><Data ss:Type="String">`)
		xml.EscapeText(b, []byte(excelSafeString(cell)))
		b.WriteString("</Data></Cell>")
	}
	b.WriteString("</Row>\n")
}

func safeCSVRecord(row []string) []string {
	out := make([]string, len(row))
	for i, cell := range row {
		out[i] = excelSafeString(cell)
	}
	return out
}

func excelSafeString(s string) string {
	if s == "" {
		return ""
	}
	first := s[0]
	if first == '=' || first == '+' || first == '-' || first == '@' || first == '\t' || first == '\r' {
		return "'" + s
	}
	return s
}

func exportFilename(show *Show, name, ext string) string {
	base := slugify(show.Name)
	if base == "" {
		base = show.ID
	}
	return base + "-" + name + "." + ext
}

func excelSheetName(name string) string {
	replacer := strings.NewReplacer(":", " ", "\\", " ", "/", " ", "?", " ", "*", " ", "[", " ", "]", " ")
	name = strings.TrimSpace(replacer.Replace(name))
	if name == "" {
		return "Sheet"
	}
	if len(name) > 31 {
		return name[:31]
	}
	return name
}

func classNumber(cm classExportMeta) string {
	if cm.Class == nil {
		return ""
	}
	return cm.Class.ClassNumber
}

func classTitle(cm classExportMeta) string {
	if cm.Class == nil {
		return ""
	}
	return cm.Class.Title
}

func divisionTitle(cm classExportMeta) string {
	if cm.Division == nil {
		return ""
	}
	return cm.Division.Title
}

func divisionCode(division *Division) string {
	if division == nil {
		return ""
	}
	return division.Code
}

func divisionDomain(division *Division) string {
	if division == nil {
		return ""
	}
	return division.Domain
}

func sectionTitle(cm classExportMeta) string {
	if cm.Section == nil {
		return ""
	}
	return cm.Section.Title
}

func sectionCode(section *Section) string {
	if section == nil {
		return ""
	}
	return section.Code
}

func personField(person *Person, field string) string {
	if person == nil {
		return ""
	}
	switch field {
	case "first":
		return person.FirstName
	case "last":
		return person.LastName
	case "initials":
		return person.Initials
	case "email":
		return person.Email
	default:
		return ""
	}
}

func personName(person *Person) string {
	if person == nil {
		return ""
	}
	return strings.TrimSpace(person.FirstName + " " + person.LastName)
}

func placementText(placement int) string {
	switch placement {
	case 1:
		return "1st"
	case 2:
		return "2nd"
	case 3:
		return "3rd"
	case 0:
		return ""
	default:
		return strconv.Itoa(placement)
	}
}

func formatFloat(v float64) string {
	if v == 0 {
		return ""
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func formatCentsAmount(cents int) string {
	if cents == 0 {
		return ""
	}
	return strconv.FormatFloat(float64(cents)/100, 'f', 2, 64)
}

func intOrEmpty(v int) string {
	if v == 0 {
		return ""
	}
	return strconv.Itoa(v)
}

func classDomain(cm classExportMeta) string {
	if cm.Class == nil {
		return ""
	}
	return cm.Class.Domain
}

func isDesignOrSpecialDomain(domain string) bool {
	return domain == "design" || domain == "special"
}

func operatorPointsForPlacement(domain string, placement int) int {
	if placement < 1 || placement > 3 {
		return 0
	}
	if isDesignOrSpecialDomain(domain) {
		return map[int]int{1: 12, 2: 9, 3: 6}[placement]
	}
	return map[int]int{1: 4, 2: 3, 3: 2}[placement]
}

func formatCountPoints(count, multiplier int) string {
	if count == 0 {
		return ""
	}
	return fmt.Sprintf("%d x %d = %d", count, multiplier, count*multiplier)
}

func formatTallyCounts(counts [4]int) string {
	if counts[1] == 0 && counts[2] == 0 && counts[3] == 0 {
		return ""
	}
	return fmt.Sprintf("%d  %d  %d", counts[1], counts[2], counts[3])
}

func placementMarker(placement int) string {
	switch placement {
	case 1, 2, 3:
		return strconv.Itoa(placement)
	default:
		return "x"
	}
}

func personTallyName(person *Person) string {
	if person == nil {
		return ""
	}
	name := strings.TrimSpace(person.LastName + ", " + person.FirstName)
	if name == "," {
		return strings.TrimSpace(person.FirstName + " " + person.LastName)
	}
	return name
}

func classLabel(class *ShowClass) string {
	if class == nil {
		return ""
	}
	if class.ClassNumber == "" {
		return class.Title
	}
	return class.ClassNumber + ": " + class.Title
}

func compactShowLabel(show *Show) string {
	if show == nil {
		return ""
	}
	if show.Date != "" {
		if parsed, err := time.Parse("2006-01-02", show.Date); err == nil {
			return parsed.Format("Jan 2006")
		}
	}
	return show.Name
}

func classNumberLess(left, right string) bool {
	leftN, leftOK := leadingInt(left)
	rightN, rightOK := leadingInt(right)
	if leftOK && rightOK && leftN != rightN {
		return leftN < rightN
	}
	return strings.ToLower(left) < strings.ToLower(right)
}

func leadingInt(s string) (int, bool) {
	s = strings.TrimSpace(s)
	var digits strings.Builder
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		digits.WriteRune(r)
	}
	if digits.Len() == 0 {
		return 0, false
	}
	n, err := strconv.Atoi(digits.String())
	return n, err == nil
}

func maxInt(values ...int) int {
	max := 0
	for _, value := range values {
		if value > max {
			max = value
		}
	}
	return max
}
