package jsontransport

import (
	"errors"
	"net/http"
	"path/filepath"

	"as/kernel/internal/realizations"
)

type ContractsAPI struct {
	RepoRoot string
}

func NewContractsAPI(repoRoot string) *ContractsAPI {
	return &ContractsAPI{RepoRoot: repoRoot}
}

func (api *ContractsAPI) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/contracts", api.handleListContracts)
	mux.HandleFunc("GET /v1/contracts/{seed_id}/{realization_id}", api.handleGetContract)
}

func (api *ContractsAPI) handleListContracts(w http.ResponseWriter, r *http.Request) {
	contracts, err := realizations.DiscoverContracts(api.RepoRoot)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	items := make([]contractSummary, 0, len(contracts))
	for _, item := range contracts {
		items = append(items, summarizeContract(api.RepoRoot, item))
	}
	respondJSON(w, http.StatusOK, map[string]any{"contracts": items})
}

func (api *ContractsAPI) handleGetContract(w http.ResponseWriter, r *http.Request) {
	contracts, err := realizations.DiscoverContracts(api.RepoRoot)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	seedID := r.PathValue("seed_id")
	realizationID := r.PathValue("realization_id")
	for _, item := range contracts {
		if item.SeedID == seedID && item.RealizationID == realizationID {
			respondJSON(w, http.StatusOK, map[string]any{
				"reference":      item.Reference,
				"seed_id":        item.SeedID,
				"realization_id": item.RealizationID,
				"contract_file":  item.ContractFile,
				"manifest": map[string]any{
					"approach_id": item.ApproachID,
					"summary":     item.Summary,
					"status":      item.Status,
					"artifacts":   item.Artifacts,
					"readme":      item.Readme,
					"notes":       item.Notes,
				},
				"discovery": map[string]string{
					"self": itemSelfPath(item),
				},
				"contract": item.Contract,
			})
			return
		}
	}

	respondError(w, http.StatusNotFound, errors.New("contract not found"))
}

type contractSummary struct {
	Reference     string   `json:"reference"`
	SeedID        string   `json:"seed_id"`
	RealizationID string   `json:"realization_id"`
	SurfaceKind   string   `json:"surface_kind"`
	Summary       string   `json:"summary"`
	ContractFile  string   `json:"contract_file"`
	Commands      []string `json:"commands"`
	Projections   []string `json:"projections"`
	Self          string   `json:"self"`
}

func summarizeContract(repoRoot string, item realizations.LoadedInteractionContract) contractSummary {
	commands := make([]string, 0, len(item.Contract.Commands))
	for _, command := range item.Contract.Commands {
		commands = append(commands, command.Name)
	}

	projections := make([]string, 0, len(item.Contract.Projections))
	for _, projection := range item.Contract.Projections {
		projections = append(projections, projection.Name)
	}

	contractFile := item.ContractFile
	if contractFile == "" {
		contractFile = filepath.ToSlash(filepath.Join(item.RootDir, "interaction_contract.yaml"))
		if repoRoot != "" {
			if rel, err := filepath.Rel(repoRoot, filepath.Join(item.RootDir, "interaction_contract.yaml")); err == nil {
				contractFile = filepath.ToSlash(rel)
			}
		}
	}

	return contractSummary{
		Reference:     item.Reference,
		SeedID:        item.SeedID,
		RealizationID: item.RealizationID,
		SurfaceKind:   item.Contract.SurfaceKind,
		Summary:       item.Contract.Summary,
		ContractFile:  contractFile,
		Commands:      commands,
		Projections:   projections,
		Self:          itemSelfPath(item),
	}
}

func itemSelfPath(item realizations.LoadedInteractionContract) string {
	return "/v1/contracts/" + item.SeedID + "/" + item.RealizationID
}
