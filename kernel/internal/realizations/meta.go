package realizations

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type SeedManifest struct {
	SeedID  string `yaml:"seed_id" json:"seed_id"`
	Version int    `yaml:"version" json:"version"`
	Summary string `yaml:"summary" json:"summary"`
	Status  string `yaml:"status" json:"status"`
}

type RuntimeManifest struct {
	Kind             string             `yaml:"kind" json:"kind"`
	Version          int                `yaml:"version" json:"version"`
	Runtime          string             `yaml:"runtime" json:"runtime"`
	Entrypoint       string             `yaml:"entrypoint" json:"entrypoint"`
	WorkingDirectory string             `yaml:"working_directory" json:"working_directory"`
	Run              RuntimeManifestRun `yaml:"run" json:"run"`
	Environment      map[string]string  `yaml:"environment" json:"environment"`
	Notes            []string           `yaml:"notes" json:"notes"`
}

type RuntimeManifestRun struct {
	Command string   `yaml:"command" json:"command"`
	Args    []string `yaml:"args" json:"args"`
}

type ReadinessInfo struct {
	Stage        string `json:"stage"`
	Label        string `json:"label"`
	Summary      string `json:"summary"`
	HasContract  bool   `json:"has_contract"`
	HasRuntime   bool   `json:"has_runtime"`
	CanInspect   bool   `json:"can_inspect"`
	CanGrow      bool   `json:"can_grow"`
	CanRun       bool   `json:"can_run"`
	SurfaceKind  string `json:"surface_kind,omitempty"`
	ContractFile string `json:"contract_file,omitempty"`
	RuntimeFile  string `json:"runtime_file,omitempty"`
}

type RealizationMeta struct {
	SeedSummary     string           `json:"seed_summary,omitempty"`
	SeedStatus      string           `json:"seed_status,omitempty"`
	ContractSummary string           `json:"contract_summary,omitempty"`
	ContractFile    string           `json:"contract_file,omitempty"`
	SurfaceKind     string           `json:"surface_kind,omitempty"`
	RuntimeArtifact string           `json:"runtime_artifact,omitempty"`
	RuntimeManifest *RuntimeManifest `json:"runtime_manifest,omitempty"`
	Readiness       ReadinessInfo    `json:"readiness"`
}

func LoadRealizationMeta(repoRoot string, entry LocalRealization) (RealizationMeta, error) {
	meta := RealizationMeta{
		Readiness: ReadinessInfo{
			Stage:      "designed",
			Label:      "Designed",
			Summary:    "Seed docs exist, but this realization is not yet normalized for growth.",
			CanInspect: true,
			CanGrow:    true,
		},
	}

	seedDir := seedDirForRealization(entry.RootDir)
	if seed, err := loadSeedManifest(seedDir); err == nil {
		meta.SeedSummary = strings.TrimSpace(seed.Summary)
		meta.SeedStatus = strings.TrimSpace(seed.Status)
	}

	if contract, err := LoadInteractionContract(repoRoot, entry); err == nil {
		meta.ContractSummary = strings.TrimSpace(contract.Contract.Summary)
		meta.ContractFile = contract.ContractFile
		meta.SurfaceKind = strings.TrimSpace(contract.Contract.SurfaceKind)
		meta.Readiness.HasContract = true
		meta.Readiness.SurfaceKind = meta.SurfaceKind
		meta.Readiness.ContractFile = meta.ContractFile
	}

	if runtimeArtifact := findRuntimeArtifact(entry); runtimeArtifact != "" {
		runtimePath := filepath.Join(entry.RootDir, filepath.FromSlash(runtimeArtifact))
		runtimeManifest, err := loadRuntimeManifest(runtimePath)
		if err != nil {
			return RealizationMeta{}, err
		}
		meta.RuntimeArtifact = candidateRelativePath(repoRoot, runtimePath)
		meta.RuntimeManifest = &runtimeManifest
		meta.Readiness.HasRuntime = true
		meta.Readiness.RuntimeFile = meta.RuntimeArtifact
	}

	meta.Readiness = classifyReadiness(entry.Status, meta.Readiness.HasContract, meta.Readiness.HasRuntime, meta.SurfaceKind, meta.ContractFile, meta.RuntimeArtifact)
	return meta, nil
}

func seedDirForRealization(rootDir string) string {
	return filepath.Dir(filepath.Dir(rootDir))
}

func loadSeedManifest(seedDir string) (SeedManifest, error) {
	raw, err := os.ReadFile(filepath.Join(seedDir, "seed.yaml"))
	if err != nil {
		return SeedManifest{}, err
	}

	var manifest SeedManifest
	if err := yaml.Unmarshal(raw, &manifest); err != nil {
		return SeedManifest{}, fmt.Errorf("parse %s: %w", filepath.Join(seedDir, "seed.yaml"), err)
	}
	return manifest, nil
}

func loadRuntimeManifest(path string) (RuntimeManifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return RuntimeManifest{}, fmt.Errorf("read %s: %w", path, err)
	}

	var manifest RuntimeManifest
	if err := yaml.Unmarshal(raw, &manifest); err != nil {
		return RuntimeManifest{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return manifest, nil
}

func findRuntimeArtifact(entry LocalRealization) string {
	for _, artifact := range entry.Artifacts {
		candidate := strings.TrimSpace(artifact)
		if candidate == "" {
			continue
		}
		base := strings.ToLower(filepath.Base(candidate))
		if base == "runtime.yaml" || base == "runtime.yml" {
			return candidate
		}
	}
	return ""
}

func classifyReadiness(status string, hasContract, hasRuntime bool, surfaceKind, contractFile, runtimeFile string) ReadinessInfo {
	info := ReadinessInfo{
		HasContract:  hasContract,
		HasRuntime:   hasRuntime,
		CanInspect:   true,
		CanGrow:      true,
		CanRun:       hasRuntime,
		SurfaceKind:  strings.TrimSpace(surfaceKind),
		ContractFile: strings.TrimSpace(contractFile),
		RuntimeFile:  strings.TrimSpace(runtimeFile),
	}

	lifecycle := strings.TrimSpace(status)
	switch {
	case info.SurfaceKind == "bootstrap_only":
		info.Stage = "bootstrap"
		info.Label = "Bootstrap"
		info.Summary = "Bootstrap-only realization for inspecting or replaying foundational system state."
	case hasRuntime && lifecycle == "accepted":
		info.Stage = "accepted"
		info.Label = "Accepted"
		info.Summary = "Runnable realization with accepted status."
	case hasRuntime:
		info.Stage = "runnable"
		info.Label = "Runnable"
		info.Summary = "Runnable artifact exists. Inspect, grow, validate, or run this realization."
	case hasContract:
		info.Stage = "defined"
		info.Label = "Defined"
		info.Summary = "Docs and interaction contract are in place. This realization is ready to grow but not yet runnable."
	default:
		info.Stage = "designed"
		info.Label = "Designed"
		info.Summary = "Seed docs exist, but this realization is not yet normalized for growth."
	}

	return info
}
