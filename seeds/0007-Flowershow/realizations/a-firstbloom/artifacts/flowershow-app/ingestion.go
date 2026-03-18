package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type ingestionImportRequest struct {
	SourceDocument SourceDocument   `json:"source_document"`
	Citations      []SourceCitation `json:"citations"`
}

func (a *app) importIngestion(input ingestionImportRequest, defaultShowID string) (*SourceDocument, []*SourceCitation, error) {
	if strings.TrimSpace(input.SourceDocument.Title) == "" {
		return nil, nil, errors.New("source_document.title is required")
	}
	doc := input.SourceDocument
	if strings.TrimSpace(doc.ShowID) == "" {
		doc.ShowID = strings.TrimSpace(defaultShowID)
	}
	createdDoc, err := a.store.createSourceDocument(doc)
	if err != nil {
		return nil, nil, err
	}
	createdCitations := make([]*SourceCitation, 0, len(input.Citations))
	for _, citation := range input.Citations {
		if strings.TrimSpace(citation.SourceDocumentID) == "" {
			citation.SourceDocumentID = createdDoc.ID
		}
		created, err := a.store.createSourceCitation(citation)
		if err != nil {
			return createdDoc, createdCitations, err
		}
		createdCitations = append(createdCitations, created)
	}
	return createdDoc, createdCitations, nil
}

func (a *app) handleAdminIngestionImport(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	var input ingestionImportRequest
	if err := json.Unmarshal([]byte(r.FormValue("payload")), &input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	showID := strings.TrimSpace(r.FormValue("show_id"))
	if _, _, err := a.importIngestion(input, showID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if showID != "" {
		a.publishAdminSections(showID, "governance", "schedule")
		a.respondAdminSectionOrRedirect(w, r, showID, "governance")
		return
	}
	redirect(w, r)
}
