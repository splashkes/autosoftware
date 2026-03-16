package realizations

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var envKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

var reservedRuntimeEnv = map[string]struct{}{
	"AS_ADDR":                 {},
	"AS_REGISTRY_URL":         {},
	"AS_PUBLIC_API_URL":       {},
	"AS_RUNTIME_DATABASE_URL": {},
	"AS_RUNTIME_AUTO_MIGRATE": {},
	"AS_INTERNAL_API_URL":     {},
	"AS_INTERNAL_API_TOKEN":   {},
	"AS_EXECUTION_ID":         {},
	"AS_SEED_ID":              {},
	"AS_REALIZATION_ID":       {},
}

func ValidateLocalRuntimeManifest(repoRoot, realizationRoot string, manifest RuntimeManifest) error {
	runtimeFamily := strings.TrimSpace(manifest.Runtime)
	command := strings.TrimSpace(manifest.Run.Command)
	if runtimeFamily == "" || command == "" {
		return fmt.Errorf("runtime and run.command are required")
	}

	switch runtimeFamily {
	case "go":
		if command != "go" {
			return fmt.Errorf("go runtime must use command go")
		}
	case "node":
		if command != "node" {
			return fmt.Errorf("node runtime must use command node")
		}
	case "python":
		if command != "python" && command != "python3" {
			return fmt.Errorf("python runtime must use command python or python3")
		}
	default:
		return fmt.Errorf("runtime %q is not allowed for local launch", runtimeFamily)
	}

	if err := requireContainedPath(repoRoot, realizationRoot, manifest.WorkingDirectory, "working_directory"); err != nil {
		return err
	}
	if err := requireContainedPath(repoRoot, realizationRoot, manifest.Entrypoint, "entrypoint"); err != nil {
		return err
	}
	for key := range manifest.Environment {
		if !envKeyPattern.MatchString(strings.TrimSpace(key)) {
			return fmt.Errorf("environment key %q is invalid", key)
		}
		if _, blocked := reservedRuntimeEnv[strings.TrimSpace(key)]; blocked {
			return fmt.Errorf("environment key %q is reserved", key)
		}
	}
	return nil
}

func CanLaunchLocally(repoRoot, realizationRoot string, manifest *RuntimeManifest) bool {
	if manifest == nil {
		return false
	}
	return ValidateLocalRuntimeManifest(repoRoot, realizationRoot, *manifest) == nil
}

func requireContainedPath(repoRoot, realizationRoot, candidate, field string) error {
	trimmed := strings.TrimSpace(candidate)
	if trimmed == "" {
		return fmt.Errorf("%s is required", field)
	}
	target := filepath.Join(realizationRoot, filepath.FromSlash(trimmed))
	if !PathContained(repoRoot, target) {
		return fmt.Errorf("%s must stay within the repo root", field)
	}
	if !PathContained(realizationRoot, target) {
		return fmt.Errorf("%s must stay within the realization root", field)
	}
	return nil
}
