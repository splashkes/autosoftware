package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"as/kernel/internal/execution"
	"as/kernel/internal/realizations"
)

func main() {
	var (
		repoRoot     = flag.String("repo-root", "", "repository root")
		targetGOOS   = flag.String("goos", "linux", "target GOOS")
		targetGOARCH = flag.String("goarch", "amd64", "target GOARCH")
	)
	flag.Parse()

	root := strings.TrimSpace(*repoRoot)
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			log.Fatalf("resolve cwd: %v", err)
		}
		root = filepath.Clean(filepath.Join(cwd, ".."))
	}

	entries, err := realizations.Discover(root)
	if err != nil {
		log.Fatalf("discover realizations: %v", err)
	}

	built := 0
	reused := 0
	for _, entry := range entries {
		meta, err := realizations.LoadRealizationMeta(root, entry)
		if err != nil {
			log.Fatalf("load realization metadata for %s: %v", entry.Reference, err)
		}
		if meta.RuntimeManifest == nil {
			continue
		}
		manifest := *meta.RuntimeManifest
		if strings.TrimSpace(manifest.Runtime) != "go" || strings.TrimSpace(manifest.Run.Command) != "prebuilt" {
			continue
		}

		workingDir := filepath.Join(entry.RootDir, filepath.FromSlash(strings.TrimSpace(manifest.WorkingDirectory)))
		artifact, err := execution.BuildGoPrebuiltArtifact(root, entry, manifest, workingDir, strings.TrimSpace(*targetGOOS), strings.TrimSpace(*targetGOARCH))
		if err != nil {
			log.Fatalf("prebuild %s: %v", entry.Reference, err)
		}
		status := "reused"
		if artifact.Built {
			status = "built"
			built++
		} else {
			reused++
		}
		fmt.Printf("%s\t%s\t%s\n", status, entry.Reference, artifact.BinaryPath)
	}

	fmt.Printf("summary\tbuilt=%d\treused=%d\n", built, reused)
}
