package execution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"as/kernel/internal/realizations"
)

const DefaultHealthWaitTimeout = 3 * time.Minute

type CapabilityURLs struct {
	RegistryBaseURL    string
	PublicAPIBaseURL   string
	InternalAPIBaseURL string
	RuntimeDatabaseURL string
}

type LocalSpec struct {
	ExecutionID       string
	Reference         string
	SeedID            string
	RealizationID     string
	RouteSubdomain    string
	RoutePathPrefix   string
	PreviewPathPrefix string
	UpstreamAddr      string
	WorkingDir        string
	Command           string
	Args              []string
	Environment       []string
}

type LocalProcess struct {
	executionID string
	cmd         *exec.Cmd
	logFile     string
}

type RunningProcess struct {
	ExecutionID string
	PID         int
	LogFile     string
}

type LocalExecutor struct {
	mu        sync.Mutex
	processes map[string]LocalProcess
	onExit    func(string, error)
}

func NewLocalExecutor(onExit func(string, error)) *LocalExecutor {
	return &LocalExecutor{processes: make(map[string]LocalProcess), onExit: onExit}
}

func BuildLocalSpec(repoRoot, reference, executionID string, urls CapabilityURLs) (LocalSpec, error) {
	entries, err := realizations.Discover(repoRoot)
	if err != nil {
		return LocalSpec{}, err
	}
	entry, ok := realizations.ResolveByReference(entries, reference)
	if !ok {
		return LocalSpec{}, fmt.Errorf("realization reference not found: %s", reference)
	}
	meta, err := realizations.LoadRealizationMeta(repoRoot, entry)
	if err != nil {
		return LocalSpec{}, err
	}
	if meta.RuntimeManifest == nil {
		return LocalSpec{}, errors.New("runtime manifest is required")
	}
	if err := realizations.ValidateLocalRuntimeManifest(repoRoot, entry.RootDir, *meta.RuntimeManifest); err != nil {
		return LocalSpec{}, err
	}

	upstreamAddr, err := reserveLoopbackAddress()
	if err != nil {
		return LocalSpec{}, err
	}
	workingDir := filepath.Join(entry.RootDir, filepath.FromSlash(strings.TrimSpace(meta.RuntimeManifest.WorkingDirectory)))
	envMap := make(map[string]string, len(meta.RuntimeManifest.Environment)+8)
	for key, value := range meta.RuntimeManifest.Environment {
		envMap[strings.TrimSpace(key)] = value
	}
	envMap["AS_ADDR"] = upstreamAddr
	envMap["AS_SEED_ID"] = entry.SeedID
	envMap["AS_REALIZATION_ID"] = entry.RealizationID
	envMap["AS_EXECUTION_ID"] = executionID
	if strings.TrimSpace(urls.RegistryBaseURL) != "" {
		envMap["AS_REGISTRY_URL"] = strings.TrimSpace(urls.RegistryBaseURL)
	}
	if strings.TrimSpace(urls.PublicAPIBaseURL) != "" {
		envMap["AS_PUBLIC_API_URL"] = strings.TrimSpace(urls.PublicAPIBaseURL)
	}
	if strings.TrimSpace(urls.InternalAPIBaseURL) != "" {
		envMap["AS_INTERNAL_API_URL"] = strings.TrimSpace(urls.InternalAPIBaseURL)
	}
	if strings.TrimSpace(urls.RuntimeDatabaseURL) != "" {
		envMap["AS_RUNTIME_DATABASE_URL"] = strings.TrimSpace(urls.RuntimeDatabaseURL)
	}
	command := strings.TrimSpace(meta.RuntimeManifest.Run.Command)
	args := append([]string(nil), meta.RuntimeManifest.Run.Args...)
	builtCommand, builtArgs, err := prepareLocalLaunchCommand(repoRoot, entry, *meta.RuntimeManifest, workingDir)
	if err != nil {
		return LocalSpec{}, err
	}
	if strings.TrimSpace(builtCommand) != "" {
		command = builtCommand
		args = builtArgs
	}

	return LocalSpec{
		ExecutionID:       executionID,
		Reference:         entry.Reference,
		SeedID:            entry.SeedID,
		RealizationID:     entry.RealizationID,
		RouteSubdomain:    strings.TrimSpace(entry.Subdomain),
		RoutePathPrefix:   strings.TrimSpace(entry.PathPrefix),
		PreviewPathPrefix: PreviewPath(executionID),
		UpstreamAddr:      upstreamAddr,
		WorkingDir:        workingDir,
		Command:           command,
		Args:              args,
		Environment:       buildProcessEnvironment(envMap),
	}, nil
}

func PreviewPath(executionID string) string {
	return "/__runs/" + strings.TrimSpace(executionID) + "/"
}

func (e *LocalExecutor) Launch(_ context.Context, spec LocalSpec) (string, error) {
	if strings.TrimSpace(spec.ExecutionID) == "" {
		return "", errors.New("execution_id is required")
	}
	if strings.TrimSpace(spec.WorkingDir) == "" || strings.TrimSpace(spec.Command) == "" {
		return "", errors.New("working_dir and command are required")
	}

	logDir := filepath.Join(os.TempDir(), "as-executions", spec.ExecutionID)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", err
	}
	logPath := filepath.Join(logDir, "process.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return "", err
	}

	cmd := exec.Command(spec.Command, spec.Args...)
	cmd.Dir = spec.WorkingDir
	cmd.Env = spec.Environment
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return "", err
	}

	e.mu.Lock()
	e.processes[spec.ExecutionID] = LocalProcess{executionID: spec.ExecutionID, cmd: cmd, logFile: logPath}
	e.mu.Unlock()

	go func() {
		err := cmd.Wait()
		_ = logFile.Close()
		e.mu.Lock()
		delete(e.processes, spec.ExecutionID)
		e.mu.Unlock()
		if e.onExit != nil {
			e.onExit(spec.ExecutionID, err)
		}
	}()

	return logPath, nil
}

func (e *LocalExecutor) Stop(executionID string) error {
	e.mu.Lock()
	process, ok := e.processes[strings.TrimSpace(executionID)]
	e.mu.Unlock()
	if !ok || process.cmd.Process == nil {
		return errors.New("execution process not found")
	}
	return process.cmd.Process.Kill()
}

func (e *LocalExecutor) RunningProcesses() []RunningProcess {
	e.mu.Lock()
	defer e.mu.Unlock()

	out := make([]RunningProcess, 0, len(e.processes))
	for executionID, process := range e.processes {
		if process.cmd == nil || process.cmd.Process == nil {
			continue
		}
		out = append(out, RunningProcess{
			ExecutionID: executionID,
			PID:         process.cmd.Process.Pid,
			LogFile:     process.logFile,
		})
	}
	return out
}

func WaitForHealthy(ctx context.Context, upstreamAddr string) error {
	client := &http.Client{Timeout: 400 * time.Millisecond}
	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultHealthWaitTimeout)
		defer cancel()
	}
	if deadline.IsZero() {
		deadline = time.Now().Add(DefaultHealthWaitTimeout)
	}

	candidates := []string{"http://" + upstreamAddr + "/healthz", "http://" + upstreamAddr + "/"}
	for {
		for _, candidate := range candidates {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate, nil)
			if err != nil {
				continue
			}
			resp, err := client.Do(req)
			if err == nil {
				_ = resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 400 {
					return nil
				}
			}
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func prepareLocalLaunchCommand(repoRoot string, entry realizations.LocalRealization, manifest realizations.RuntimeManifest, workingDir string) (string, []string, error) {
	command := strings.TrimSpace(manifest.Run.Command)
	args := append([]string(nil), manifest.Run.Args...)
	if strings.TrimSpace(manifest.Runtime) != "go" || command != "go" || !isGoRunDot(args) {
		return command, args, nil
	}
	if _, err := os.Stat(filepath.Join(workingDir, "go.mod")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return command, args, nil
		}
		return "", nil, err
	}

	binaryPath, err := ensureGoLaunchBinary(repoRoot, entry, manifest, workingDir)
	if err != nil {
		return "", nil, err
	}
	return binaryPath, nil, nil
}

func isGoRunDot(args []string) bool {
	if len(args) != 2 {
		return false
	}
	return strings.TrimSpace(args[0]) == "run" && strings.TrimSpace(args[1]) == "."
}

func ensureGoLaunchBinary(repoRoot string, entry realizations.LocalRealization, manifest realizations.RuntimeManifest, workingDir string) (string, error) {
	buildDir := filepath.Join(
		repoRoot,
		"materialized",
		"realizations",
		entry.SeedID,
		entry.RealizationID,
		"runtime",
		"local",
		runtime.GOOS+"-"+runtime.GOARCH,
	)
	if !realizations.PathContained(repoRoot, buildDir) {
		return "", fmt.Errorf("launch build directory escapes repo root")
	}
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return "", fmt.Errorf("create launch build directory: %w", err)
	}

	target := filepath.Join(buildDir, goLaunchBinaryName(manifest, workingDir))
	sourceModTime, err := newestRegularFileModTime(workingDir)
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(target); err == nil && !info.ModTime().Before(sourceModTime) {
		return target, nil
	}

	tmpTarget := target + ".tmp"
	_ = os.Remove(tmpTarget)
	cmd := exec.Command("go", "build", "-o", tmpTarget, ".")
	cmd.Dir = workingDir
	cmd.Env = buildProcessEnvironment(map[string]string{})
	output, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed == "" {
			return "", fmt.Errorf("build local launch binary: %w", err)
		}
		return "", fmt.Errorf("build local launch binary: %w: %s", err, trimmed)
	}
	if err := os.Chmod(tmpTarget, 0o755); err != nil {
		_ = os.Remove(tmpTarget)
		return "", fmt.Errorf("chmod launch binary: %w", err)
	}
	if err := os.Rename(tmpTarget, target); err != nil {
		_ = os.Remove(tmpTarget)
		return "", fmt.Errorf("finalize launch binary: %w", err)
	}
	return target, nil
}

func goLaunchBinaryName(manifest realizations.RuntimeManifest, workingDir string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(manifest.Entrypoint) + "\n" + strings.TrimSpace(workingDir)))
	return "launch-" + hex.EncodeToString(sum[:6])
}

func newestRegularFileModTime(root string) (time.Time, error) {
	var newest time.Time
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == "node_modules" {
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
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
		return nil
	})
	if err != nil {
		return time.Time{}, fmt.Errorf("scan launch sources: %w", err)
	}
	return newest, nil
}

func reserveLoopbackAddress() (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr, nil
}

func buildProcessEnvironment(overrides map[string]string) []string {
	base := map[string]string{}
	for _, pair := range os.Environ() {
		key, value, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}
		if keepHostEnv(key) {
			base[key] = value
		}
	}
	for key, value := range overrides {
		base[key] = value
	}
	out := make([]string, 0, len(base))
	for key, value := range base {
		out = append(out, key+"="+value)
	}
	return out
}

func keepHostEnv(key string) bool {
	switch key {
	case "PATH", "HOME", "USER", "TMPDIR", "SHELL", "LANG", "TERM", "GOENV", "GOCACHE", "GOMODCACHE", "GOPATH", "XDG_CACHE_HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "CGO_ENABLED":
		return true
	default:
		return strings.HasPrefix(key, "LC_")
	}
}
