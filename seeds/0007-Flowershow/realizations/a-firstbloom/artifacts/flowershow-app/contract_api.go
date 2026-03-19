package main

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

type localContractSummaryDoc struct {
	SurfaceKind   string `yaml:"surface_kind"`
	SeedID        string `yaml:"seed_id"`
	RealizationID string `yaml:"realization_id"`
	Summary       string `yaml:"summary"`
	Commands      []struct {
		Name string `yaml:"name"`
	} `yaml:"commands"`
	Projections []struct {
		Name string `yaml:"name"`
	} `yaml:"projections"`
}

func interactionContractPath() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("could not resolve contract path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "interaction_contract.yaml")), nil
}

func loadInteractionContractDocument() (map[string]any, localContractSummaryDoc, error) {
	path, err := interactionContractPath()
	if err != nil {
		return nil, localContractSummaryDoc{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, localContractSummaryDoc{}, err
	}

	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, localContractSummaryDoc{}, err
	}
	var summary localContractSummaryDoc
	if err := yaml.Unmarshal(raw, &summary); err != nil {
		return nil, localContractSummaryDoc{}, err
	}
	return doc, summary, nil
}

func (a *app) handleContractsList(w http.ResponseWriter, r *http.Request) {
	doc, summary, err := loadInteractionContractDocument()
	if err != nil {
		a.writeAPIError(w, r, http.StatusInternalServerError, "contract_unavailable", "Contract metadata is unavailable.", "Check that interaction_contract.yaml is present beside the realization manifest.", nil)
		return
	}

	commands := make([]string, 0, len(summary.Commands))
	for _, item := range summary.Commands {
		commands = append(commands, item.Name)
	}
	projections := make([]string, 0, len(summary.Projections))
	for _, item := range summary.Projections {
		projections = append(projections, item.Name)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"contracts": []map[string]any{{
			"reference":      summary.SeedID + "/" + summary.RealizationID,
			"seed_id":        summary.SeedID,
			"realization_id": summary.RealizationID,
			"surface_kind":   summary.SurfaceKind,
			"summary":        summary.Summary,
			"commands":       commands,
			"projections":    projections,
			"self":           flowershowContractSelf,
			"contract":       doc,
		}},
	})
}

func (a *app) handleContractDetail(w http.ResponseWriter, r *http.Request) {
	seedID := r.PathValue("seed_id")
	realizationID := r.PathValue("realization_id")
	if seedID != "0007-Flowershow" || realizationID != "a-firstbloom" {
		a.writeAPIError(w, r, http.StatusNotFound, "contract_not_found", "Contract not found.", "Inspect GET /v1/contracts for the realized seed and available contract references.", nil)
		return
	}

	doc, summary, err := loadInteractionContractDocument()
	if err != nil {
		a.writeAPIError(w, r, http.StatusInternalServerError, "contract_unavailable", "Contract metadata is unavailable.", "Check that interaction_contract.yaml is present beside the realization manifest.", nil)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"reference":      summary.SeedID + "/" + summary.RealizationID,
		"seed_id":        summary.SeedID,
		"realization_id": summary.RealizationID,
		"contract_file":  "interaction_contract.yaml",
		"discovery": map[string]string{
			"self": flowershowContractSelf,
		},
		"contract": doc,
	})
}
