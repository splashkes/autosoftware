package realizations

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ContextFile struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Preview    string `json:"preview"`
	Truncated  bool   `json:"truncated"`
	ByteLength int    `json:"byte_length"`
}

type GrowthContext struct {
	Reference       string        `json:"reference"`
	SeedID          string        `json:"seed_id"`
	SeedSummary     string        `json:"seed_summary,omitempty"`
	SeedStatus      string        `json:"seed_status,omitempty"`
	RealizationID   string        `json:"realization_id"`
	ApproachID      string        `json:"approach_id,omitempty"`
	Summary         string        `json:"summary"`
	Status          string        `json:"status"`
	SurfaceKind     string        `json:"surface_kind,omitempty"`
	ContractSummary string        `json:"contract_summary,omitempty"`
	Subdomain       string        `json:"subdomain,omitempty"`
	PathPrefix      string        `json:"path_prefix,omitempty"`
	Readiness       ReadinessInfo `json:"readiness"`
	SeedDocs        []ContextFile `json:"seed_docs"`
	ApproachDocs    []ContextFile `json:"approach_docs"`
	RealizationDocs []ContextFile `json:"realization_docs"`
	RuntimeDocs     []ContextFile `json:"runtime_docs"`
}

func LoadGrowthContext(repoRoot, reference string) (GrowthContext, error) {
	entries, err := Discover(repoRoot)
	if err != nil {
		return GrowthContext{}, err
	}

	entry, ok := ResolveByReference(entries, reference)
	if !ok {
		return GrowthContext{}, fmt.Errorf("realization reference not found: %s", reference)
	}

	meta, err := LoadRealizationMeta(repoRoot, entry)
	if err != nil {
		return GrowthContext{}, err
	}

	seedDir := seedDirForRealization(entry.RootDir)
	ctx := GrowthContext{
		Reference:       entry.Reference,
		SeedID:          entry.SeedID,
		SeedSummary:     meta.SeedSummary,
		SeedStatus:      meta.SeedStatus,
		RealizationID:   entry.RealizationID,
		ApproachID:      entry.ApproachID,
		Summary:         entry.Summary,
		Status:          entry.Status,
		SurfaceKind:     meta.SurfaceKind,
		ContractSummary: meta.ContractSummary,
		Subdomain:       entry.Subdomain,
		PathPrefix:      entry.PathPrefix,
		Readiness:       meta.Readiness,
	}

	ctx.SeedDocs = compactContextFiles([]ContextFile{
		previewContextFile(repoRoot, filepath.Join(seedDir, "README.md"), "seed_readme"),
		previewContextFile(repoRoot, filepath.Join(seedDir, "brief.md"), "seed_brief"),
		previewContextFile(repoRoot, filepath.Join(seedDir, "design.md"), "seed_design"),
		previewContextFile(repoRoot, filepath.Join(seedDir, "acceptance.md"), "seed_acceptance"),
		previewContextFile(repoRoot, filepath.Join(seedDir, "decision_log.md"), "seed_decision_log"),
		previewContextFile(repoRoot, filepath.Join(seedDir, "notes.md"), "seed_notes"),
		previewContextFile(repoRoot, filepath.Join(seedDir, "seed.yaml"), "seed_manifest"),
	})

	if strings.TrimSpace(entry.ApproachID) != "" {
		ctx.ApproachDocs = compactContextFiles([]ContextFile{
			previewContextFile(repoRoot, filepath.Join(seedDir, "approaches", entry.ApproachID+".yaml"), "approach_manifest"),
			previewContextFile(repoRoot, filepath.Join(seedDir, "approaches", "README.md"), "approach_readme"),
		})
	}

	realizationDir := entry.RootDir
	ctx.RealizationDocs = compactContextFiles([]ContextFile{
		previewContextFile(repoRoot, filepath.Join(realizationDir, "README.md"), "realization_readme"),
		previewContextFile(repoRoot, filepath.Join(realizationDir, "notes.md"), "realization_notes"),
		previewContextFile(repoRoot, filepath.Join(realizationDir, "realization.yaml"), "realization_manifest"),
		previewContextFile(repoRoot, filepath.Join(realizationDir, "interaction_contract.yaml"), "interaction_contract"),
	})

	if meta.RuntimeArtifact != "" {
		ctx.RuntimeDocs = append(ctx.RuntimeDocs, previewContextFile(repoRoot, filepath.Join(repoRoot, filepath.FromSlash(meta.RuntimeArtifact)), "runtime_manifest"))
	}
	for _, artifact := range entry.Artifacts {
		if strings.TrimSpace(artifact) == "" {
			continue
		}
		fullPath := filepath.Join(realizationDir, filepath.FromSlash(artifact))
		if !PathContained(repoRoot, fullPath) {
			continue
		}
		if filepath.Base(fullPath) == "runtime.yaml" || filepath.Base(fullPath) == "runtime.yml" {
			continue
		}
		ctx.RuntimeDocs = append(ctx.RuntimeDocs, previewContextFile(repoRoot, fullPath, "artifact"))
	}
	ctx.RuntimeDocs = compactContextFiles(ctx.RuntimeDocs)

	return ctx, nil
}

func previewContextFile(repoRoot, path, kind string) ContextFile {
	if !PathContained(repoRoot, path) {
		return ContextFile{}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ContextFile{}
	}

	preview := strings.TrimSpace(string(raw))
	truncated := false
	if len(preview) > 900 {
		preview = strings.TrimSpace(preview[:900]) + "\n..."
		truncated = true
	}

	return ContextFile{
		Path:       candidateRelativePath(repoRoot, path),
		Kind:       kind,
		Preview:    preview,
		Truncated:  truncated,
		ByteLength: len(raw),
	}
}

func compactContextFiles(files []ContextFile) []ContextFile {
	out := make([]ContextFile, 0, len(files))
	for _, file := range files {
		if strings.TrimSpace(file.Path) == "" {
			continue
		}
		out = append(out, file)
	}
	return out
}
