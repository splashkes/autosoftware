package execution

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"as/kernel/internal/realizations"
)

const goPrebuiltMetadataFile = "current.json"

const (
	DOKSRuntimeGOOS   = "linux"
	DOKSRuntimeGOARCH = "amd64"
)

type GoPrebuiltArtifact struct {
	BinaryPath   string
	MetadataPath string
	Fingerprint  string
	Built        bool
}

type goPrebuiltMetadata struct {
	Runtime          string    `json:"runtime"`
	GOOS             string    `json:"goos"`
	GOARCH           string    `json:"goarch"`
	Fingerprint      string    `json:"fingerprint"`
	Binary           string    `json:"binary"`
	Entrypoint       string    `json:"entrypoint,omitempty"`
	WorkingDirectory string    `json:"working_directory,omitempty"`
	Toolchain        string    `json:"toolchain,omitempty"`
	BuiltAt          time.Time `json:"built_at"`
}

func BuildGoPrebuiltArtifact(repoRoot string, entry realizations.LocalRealization, manifest realizations.RuntimeManifest, workingDir, targetGOOS, targetGOARCH string) (GoPrebuiltArtifact, error) {
	if strings.TrimSpace(manifest.Runtime) != "go" {
		return GoPrebuiltArtifact{}, fmt.Errorf("prebuilt artifact build only supports go runtimes")
	}
	if strings.TrimSpace(manifest.Run.Command) != "prebuilt" {
		return GoPrebuiltArtifact{}, fmt.Errorf("go runtime %s must use run.command prebuilt", entry.Reference)
	}

	toolchain, err := goToolchainVersion()
	if err != nil {
		return GoPrebuiltArtifact{}, err
	}
	fingerprint, err := computeGoPrebuiltFingerprint(workingDir, manifest, targetGOOS, targetGOARCH, toolchain)
	if err != nil {
		return GoPrebuiltArtifact{}, err
	}
	buildDir, err := prebuiltArtifactDir(repoRoot, entry, targetGOOS, targetGOARCH)
	if err != nil {
		return GoPrebuiltArtifact{}, err
	}
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return GoPrebuiltArtifact{}, fmt.Errorf("create prebuilt artifact directory: %w", err)
	}

	binaryName := "launch-" + fingerprint
	target := filepath.Join(buildDir, binaryName)
	metadataPath := filepath.Join(buildDir, goPrebuiltMetadataFile)
	metadata := goPrebuiltMetadata{
		Runtime:          "go",
		GOOS:             targetGOOS,
		GOARCH:           targetGOARCH,
		Fingerprint:      fingerprint,
		Binary:           binaryName,
		Entrypoint:       strings.TrimSpace(manifest.Entrypoint),
		WorkingDirectory: strings.TrimSpace(manifest.WorkingDirectory),
		Toolchain:        toolchain,
		BuiltAt:          time.Now().UTC(),
	}

	if info, err := os.Stat(target); err == nil && info.Mode().IsRegular() {
		if err := writeGoPrebuiltMetadata(metadataPath, metadata); err != nil {
			return GoPrebuiltArtifact{}, err
		}
		return GoPrebuiltArtifact{
			BinaryPath:   target,
			MetadataPath: metadataPath,
			Fingerprint:  fingerprint,
			Built:        false,
		}, nil
	}

	tmpTarget := target + ".tmp"
	_ = os.Remove(tmpTarget)
	cmd := exec.Command("go", "build", "-o", tmpTarget, ".")
	cmd.Dir = workingDir
	cmd.Env = buildProcessEnvironment(map[string]string{
		"GOOS":        targetGOOS,
		"GOARCH":      targetGOARCH,
		"CGO_ENABLED": "0",
	})
	output, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed == "" {
			return GoPrebuiltArtifact{}, fmt.Errorf("build prebuilt artifact for %s: %w", entry.Reference, err)
		}
		return GoPrebuiltArtifact{}, fmt.Errorf("build prebuilt artifact for %s: %w: %s", entry.Reference, err, trimmed)
	}
	if err := os.Chmod(tmpTarget, 0o755); err != nil {
		_ = os.Remove(tmpTarget)
		return GoPrebuiltArtifact{}, fmt.Errorf("chmod prebuilt artifact: %w", err)
	}
	if err := os.Rename(tmpTarget, target); err != nil {
		_ = os.Remove(tmpTarget)
		return GoPrebuiltArtifact{}, fmt.Errorf("finalize prebuilt artifact: %w", err)
	}
	if err := writeGoPrebuiltMetadata(metadataPath, metadata); err != nil {
		return GoPrebuiltArtifact{}, err
	}
	return GoPrebuiltArtifact{
		BinaryPath:   target,
		MetadataPath: metadataPath,
		Fingerprint:  fingerprint,
		Built:        true,
	}, nil
}

func ResolveGoPrebuiltArtifact(repoRoot string, entry realizations.LocalRealization, targetGOOS, targetGOARCH string) (string, error) {
	buildDir, err := prebuiltArtifactDir(repoRoot, entry, targetGOOS, targetGOARCH)
	if err != nil {
		return "", err
	}
	metadataPath := filepath.Join(buildDir, goPrebuiltMetadataFile)
	raw, err := os.ReadFile(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("missing prebuilt artifact metadata for %s at %s; build realizations during CI/materialization before launch", entry.Reference, metadataPath)
		}
		return "", fmt.Errorf("read prebuilt artifact metadata: %w", err)
	}

	var metadata goPrebuiltMetadata
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return "", fmt.Errorf("parse prebuilt artifact metadata: %w", err)
	}
	if strings.TrimSpace(metadata.Runtime) != "go" {
		return "", fmt.Errorf("prebuilt artifact metadata for %s has unsupported runtime %q", entry.Reference, metadata.Runtime)
	}
	if strings.TrimSpace(metadata.GOOS) != targetGOOS || strings.TrimSpace(metadata.GOARCH) != targetGOARCH {
		return "", fmt.Errorf("prebuilt artifact metadata for %s targets %s-%s, not %s-%s", entry.Reference, metadata.GOOS, metadata.GOARCH, targetGOOS, targetGOARCH)
	}
	binaryName := strings.TrimSpace(metadata.Binary)
	if binaryName == "" {
		return "", fmt.Errorf("prebuilt artifact metadata for %s does not name a binary", entry.Reference)
	}
	binaryPath := filepath.Join(buildDir, filepath.FromSlash(binaryName))
	if !realizations.PathContained(repoRoot, binaryPath) || !realizations.PathContained(buildDir, binaryPath) {
		return "", fmt.Errorf("prebuilt artifact path escapes allowed directories for %s", entry.Reference)
	}
	info, err := os.Stat(binaryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("prebuilt artifact binary missing for %s at %s", entry.Reference, binaryPath)
		}
		return "", fmt.Errorf("stat prebuilt artifact binary: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("prebuilt artifact binary for %s is not a regular file", entry.Reference)
	}
	return binaryPath, nil
}

func computeGoPrebuiltFingerprint(workingDir string, manifest realizations.RuntimeManifest, targetGOOS, targetGOARCH, toolchain string) (string, error) {
	files, err := fingerprintFiles(workingDir)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	writeFingerprintLine(hash, "runtime", strings.TrimSpace(manifest.Runtime))
	writeFingerprintLine(hash, "entrypoint", strings.TrimSpace(manifest.Entrypoint))
	writeFingerprintLine(hash, "working_directory", strings.TrimSpace(manifest.WorkingDirectory))
	writeFingerprintLine(hash, "target", targetGOOS+"/"+targetGOARCH)
	writeFingerprintLine(hash, "toolchain", strings.TrimSpace(toolchain))

	for _, file := range files {
		relative, err := filepath.Rel(workingDir, file)
		if err != nil {
			return "", fmt.Errorf("compute fingerprint relative path: %w", err)
		}
		writeFingerprintLine(hash, "file", filepath.ToSlash(relative))
		if err := appendFileToHash(hash, file); err != nil {
			return "", err
		}
	}

	sum := hash.Sum(nil)
	return hex.EncodeToString(sum[:12]), nil
}

func fingerprintFiles(root string) ([]string, error) {
	files := []string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch filepath.Base(path) {
			case ".git", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan prebuilt inputs: %w", err)
	}
	sort.Strings(files)
	return files, nil
}

func writeFingerprintLine(w io.Writer, key, value string) {
	_, _ = io.WriteString(w, key+"="+value+"\n")
}

func appendFileToHash(w io.Writer, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open prebuilt input %s: %w", path, err)
	}
	defer file.Close()

	if _, err := io.Copy(w, file); err != nil {
		return fmt.Errorf("hash prebuilt input %s: %w", path, err)
	}
	_, _ = io.WriteString(w, "\n")
	return nil
}

func prebuiltArtifactDir(repoRoot string, entry realizations.LocalRealization, targetGOOS, targetGOARCH string) (string, error) {
	buildDir := filepath.Join(
		repoRoot,
		"materialized",
		"realizations",
		entry.SeedID,
		entry.RealizationID,
		"runtime",
		"prebuilt",
		targetGOOS+"-"+targetGOARCH,
	)
	if !realizations.PathContained(repoRoot, buildDir) {
		return "", fmt.Errorf("prebuilt artifact directory escapes repo root")
	}
	return buildDir, nil
}

func goToolchainVersion() (string, error) {
	output, err := exec.Command("go", "env", "GOVERSION").CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed == "" {
			return "", fmt.Errorf("resolve go toolchain version: %w", err)
		}
		return "", fmt.Errorf("resolve go toolchain version: %w: %s", err, trimmed)
	}
	version := strings.TrimSpace(string(output))
	if version == "" {
		return "", fmt.Errorf("resolve go toolchain version: empty result")
	}
	return version, nil
}

func writeGoPrebuiltMetadata(path string, metadata goPrebuiltMetadata) error {
	raw, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("encode prebuilt artifact metadata: %w", err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write prebuilt artifact metadata: %w", err)
	}
	return nil
}
