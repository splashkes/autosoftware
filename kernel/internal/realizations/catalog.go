package realizations

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var ErrRepoRootNotFound = errors.New("repo root not found")

type Manifest struct {
	RealizationID string   `yaml:"realization_id" json:"realization_id"`
	SeedID        string   `yaml:"seed_id" json:"seed_id"`
	ApproachID    string   `yaml:"approach_id" json:"approach_id"`
	Summary       string   `yaml:"summary" json:"summary"`
	Status        string   `yaml:"status" json:"status"`
	Artifacts     []string `yaml:"artifacts" json:"artifacts"`
}

type LocalRealization struct {
	Manifest
	Reference string `json:"reference"`
	RootDir   string `json:"root_dir"`
	Readme    string `json:"readme,omitempty"`
	Notes     string `json:"notes,omitempty"`
}

func FindRepoRoot(start string) (string, error) {
	if start == "" {
		start = "."
	}

	current, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolve repo root: %w", err)
	}

	for {
		if isRepoRoot(current) {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", ErrRepoRootNotFound
		}
		current = parent
	}
}

func Discover(repoRoot string) ([]LocalRealization, error) {
	var out []LocalRealization

	searchRoots := []string{
		filepath.Join(repoRoot, "genesis", "realizations"),
		filepath.Join(repoRoot, "seeds"),
	}

	for _, root := range searchRoots {
		if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || d.Name() != "realization.yaml" {
				return nil
			}

			entry, loadErr := loadLocalRealization(repoRoot, path)
			if loadErr != nil {
				return loadErr
			}

			out = append(out, entry)
			return nil
		}); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].SeedID == out[j].SeedID {
			return out[i].RealizationID < out[j].RealizationID
		}
		return out[i].SeedID < out[j].SeedID
	})

	return out, nil
}

func NormalizeReference(reference string) string {
	return strings.Trim(strings.TrimSpace(reference), "/")
}

func BuildReference(seedID, realizationID string) string {
	seedID = strings.TrimSpace(seedID)
	realizationID = strings.TrimSpace(realizationID)

	switch {
	case seedID == "":
		return realizationID
	case realizationID == "":
		return seedID
	default:
		return seedID + "/" + realizationID
	}
}

func SplitReference(reference string) (seedID, realizationID string) {
	reference = NormalizeReference(reference)
	if reference == "" {
		return "", ""
	}

	parts := strings.Split(reference, "/")
	if len(parts) == 1 {
		return "", parts[0]
	}

	return strings.Join(parts[:len(parts)-1], "/"), parts[len(parts)-1]
}

func ResolveByReference(entries []LocalRealization, reference string) (LocalRealization, bool) {
	reference = NormalizeReference(reference)
	if reference == "" {
		return LocalRealization{}, false
	}

	for _, entry := range entries {
		if entry.Reference == reference {
			return entry, true
		}
	}

	var match LocalRealization
	matchCount := 0
	for _, entry := range entries {
		if entry.RealizationID == reference {
			match = entry
			matchCount++
		}
	}

	return match, matchCount == 1
}

func loadLocalRealization(repoRoot, manifestPath string) (LocalRealization, error) {
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return LocalRealization{}, fmt.Errorf("read %s: %w", manifestPath, err)
	}

	var manifest Manifest
	if err := yaml.Unmarshal(raw, &manifest); err != nil {
		return LocalRealization{}, fmt.Errorf("parse %s: %w", manifestPath, err)
	}

	rootDir := filepath.Dir(manifestPath)
	entry := LocalRealization{
		Manifest:  manifest,
		Reference: NormalizeReference(BuildReference(manifest.SeedID, manifest.RealizationID)),
		RootDir:   rootDir,
		Readme:    candidateRelativePath(repoRoot, filepath.Join(rootDir, "README.md")),
		Notes:     candidateRelativePath(repoRoot, filepath.Join(rootDir, "notes.md")),
	}

	return entry, nil
}

func candidateRelativePath(repoRoot, path string) string {
	if _, err := os.Stat(path); err != nil {
		return ""
	}

	rel, err := filepath.Rel(repoRoot, path)
	if err != nil {
		return path
	}

	return filepath.ToSlash(rel)
}

// PathContained reports whether target is contained within root after resolving
// both to absolute paths. It returns false if either path cannot be resolved.
func PathContained(root, target string) bool {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	return absTarget == absRoot || strings.HasPrefix(absTarget, absRoot+string(filepath.Separator))
}

func isRepoRoot(path string) bool {
	required := []string{"genesis", "kernel", "seeds"}
	for _, name := range required {
		info, err := os.Stat(filepath.Join(path, name))
		if err != nil || !info.IsDir() {
			return false
		}
	}

	return true
}
