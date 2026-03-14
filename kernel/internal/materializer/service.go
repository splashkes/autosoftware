package materializer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"as/kernel/internal/realizations"
)

var ErrReferenceNotFound = errors.New("realization reference not found")

type SourceFlags struct {
	Local  bool `json:"local"`
	Remote bool `json:"remote"`
}

type RealizationOption struct {
	Reference     string      `json:"reference"`
	SeedID        string      `json:"seed_id"`
	RealizationID string      `json:"realization_id"`
	ApproachID    string      `json:"approach_id,omitempty"`
	Summary       string      `json:"summary"`
	Status        string      `json:"status"`
	Sources       SourceFlags `json:"sources"`
}

type FilePreview struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Preview    string `json:"preview"`
	Truncated  bool   `json:"truncated"`
	Source     string `json:"source"`
	ByteLength int    `json:"byte_length"`
}

type LocalSource struct {
	Reference     string        `json:"reference"`
	SeedID        string        `json:"seed_id"`
	RealizationID string        `json:"realization_id"`
	ApproachID    string        `json:"approach_id,omitempty"`
	Summary       string        `json:"summary"`
	Status        string        `json:"status"`
	RootDir       string        `json:"root_dir"`
	Files         []FilePreview `json:"files"`
}

type RemoteSource struct {
	Reference     string                 `json:"reference"`
	SeedID        string                 `json:"seed_id,omitempty"`
	RealizationID string                 `json:"realization_id,omitempty"`
	ApproachID    string                 `json:"approach_id,omitempty"`
	Summary       string                 `json:"summary"`
	Status        string                 `json:"status"`
	RegistryURL   string                 `json:"registry_url"`
	FetchedAt     time.Time              `json:"fetched_at"`
	Metadata      map[string]string      `json:"metadata,omitempty"`
	Sources       SourceFlags            `json:"sources"`
	Files         []FilePreview          `json:"files,omitempty"`
	Warnings      []string               `json:"warnings,omitempty"`
	Raw           map[string]interface{} `json:"raw,omitempty"`
}

type Materialization struct {
	Reference     string        `json:"reference"`
	SeedID        string        `json:"seed_id,omitempty"`
	RealizationID string        `json:"realization_id,omitempty"`
	ApproachID    string        `json:"approach_id,omitempty"`
	Summary       string        `json:"summary"`
	Status        string        `json:"status"`
	GeneratedAt   time.Time     `json:"generated_at"`
	Sources       SourceFlags   `json:"sources"`
	Local         *LocalSource  `json:"local,omitempty"`
	Remote        *RemoteSource `json:"remote,omitempty"`
	Warnings      []string      `json:"warnings,omitempty"`
	PersistedPath string        `json:"persisted_path,omitempty"`
}

type Service struct {
	RepoRoot   string
	OutputRoot string
	Remote     *RemoteRegistryClient
	Now        func() time.Time
}

func NewService(repoRoot string, remote *RemoteRegistryClient) (*Service, error) {
	if repoRoot == "" {
		return nil, errors.New("repo root is required")
	}

	return &Service{
		RepoRoot:   repoRoot,
		OutputRoot: filepath.Join(repoRoot, "materialized", "realizations"),
		Remote:     remote,
		Now:        time.Now,
	}, nil
}

func (s *Service) ListRealizations(ctx context.Context) ([]RealizationOption, error) {
	localEntries, err := realizations.Discover(s.RepoRoot)
	if err != nil {
		return nil, err
	}

	merged := make(map[string]RealizationOption, len(localEntries))
	for _, entry := range localEntries {
		merged[entry.Reference] = RealizationOption{
			Reference:     entry.Reference,
			SeedID:        entry.SeedID,
			RealizationID: entry.RealizationID,
			ApproachID:    entry.ApproachID,
			Summary:       entry.Summary,
			Status:        entry.Status,
			Sources:       SourceFlags{Local: true},
		}
	}

	if s.Remote != nil {
		remoteEntries, remoteErr := s.Remote.ListRealizations(ctx)
		if remoteErr == nil {
			for _, entry := range remoteEntries {
				option := merged[entry.Reference]
				if option.Reference == "" {
					option = entry
				}
				if option.SeedID == "" {
					option.SeedID = entry.SeedID
				}
				if option.RealizationID == "" {
					option.RealizationID = entry.RealizationID
				}
				if option.ApproachID == "" {
					option.ApproachID = entry.ApproachID
				}
				if option.Summary == "" {
					option.Summary = entry.Summary
				}
				if option.Status == "" {
					option.Status = entry.Status
				}
				option.Reference = entry.Reference
				option.Sources.Remote = true
				merged[entry.Reference] = option
			}
		}
	}

	out := make([]RealizationOption, 0, len(merged))
	for _, option := range merged {
		if option.Reference == "" {
			continue
		}
		out = append(out, option)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].SeedID == out[j].SeedID {
			return out[i].RealizationID < out[j].RealizationID
		}
		return out[i].SeedID < out[j].SeedID
	})

	return out, nil
}

func (s *Service) Materialize(ctx context.Context, reference string) (Materialization, error) {
	reference = realizations.NormalizeReference(reference)
	if reference == "" {
		return Materialization{}, ErrReferenceNotFound
	}

	result := Materialization{
		Reference:   reference,
		GeneratedAt: s.now().UTC(),
	}
	result.SeedID, result.RealizationID = realizations.SplitReference(reference)

	localEntries, err := realizations.Discover(s.RepoRoot)
	if err != nil {
		return Materialization{}, err
	}

	if local, ok := realizations.ResolveByReference(localEntries, reference); ok {
		source, loadErr := s.localSource(local)
		if loadErr != nil {
			result.Warnings = append(result.Warnings, loadErr.Error())
		} else {
			result.Local = &source
			result.Sources.Local = true
			adoptLocalSummary(&result, source)
		}
	}

	if s.Remote != nil {
		remote, remoteErr := s.Remote.GetMaterialization(ctx, reference)
		if remoteErr != nil {
			result.Warnings = append(result.Warnings, remoteErr.Error())
		} else if remote.Reference != "" {
			remote.RegistryURL = s.Remote.BaseURL
			result.Remote = &remote
			result.Sources.Remote = true
			adoptRemoteSummary(&result, remote)
		}
	}

	if !result.Sources.Local && !result.Sources.Remote {
		return Materialization{}, ErrReferenceNotFound
	}

	path, err := s.persist(result)
	if err != nil {
		return Materialization{}, err
	}
	result.PersistedPath = path

	return result, nil
}

func (s *Service) localSource(entry realizations.LocalRealization) (LocalSource, error) {
	source := LocalSource{
		Reference:     entry.Reference,
		SeedID:        entry.SeedID,
		RealizationID: entry.RealizationID,
		ApproachID:    entry.ApproachID,
		Summary:       entry.Summary,
		Status:        entry.Status,
		RootDir:       relativeOrOriginal(s.RepoRoot, entry.RootDir),
	}

	source.Files = append(source.Files, collectDocPreview(s.RepoRoot, entry.Readme, "readme"))
	source.Files = append(source.Files, collectDocPreview(s.RepoRoot, entry.Notes, "notes"))

	for _, artifact := range entry.Artifacts {
		artifactPath := filepath.Join(entry.RootDir, filepath.FromSlash(artifact))
		preview, err := readPreview(s.RepoRoot, artifactPath, "artifact", "repo")
		if err != nil {
			return LocalSource{}, err
		}
		source.Files = append(source.Files, preview)
	}

	source.Files = compactFiles(source.Files)
	return source, nil
}

func (s *Service) persist(materialization Materialization) (string, error) {
	if err := os.MkdirAll(filepath.Join(s.OutputRoot, materialization.SeedID, materialization.RealizationID), 0o755); err != nil {
		return "", fmt.Errorf("create materialized directory: %w", err)
	}

	target := filepath.Join(s.OutputRoot, materialization.SeedID, materialization.RealizationID, "materialization.json")
	payload, err := json.MarshalIndent(materialization, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal materialization: %w", err)
	}

	payload = append(payload, '\n')
	if err := os.WriteFile(target, payload, 0o644); err != nil {
		return "", fmt.Errorf("write materialization: %w", err)
	}

	return relativeOrOriginal(s.RepoRoot, target), nil
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

type RemoteRegistryClient struct {
	BaseURL string
	Client  *http.Client
}

func (c *RemoteRegistryClient) ListRealizations(ctx context.Context) ([]RealizationOption, error) {
	if c == nil || strings.TrimSpace(c.BaseURL) == "" {
		return nil, errors.New("remote registry is not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.BaseURL, "/")+"/v1/realizations", nil)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Realizations []RealizationOption `json:"realizations"`
	}

	if err := c.doJSON(req, &payload); err != nil {
		return nil, fmt.Errorf("load remote realizations: %w", err)
	}

	for i := range payload.Realizations {
		payload.Realizations[i].Sources.Remote = true
		payload.Realizations[i].Reference = realizations.NormalizeReference(payload.Realizations[i].Reference)
	}

	return payload.Realizations, nil
}

func (c *RemoteRegistryClient) GetMaterialization(ctx context.Context, reference string) (RemoteSource, error) {
	if c == nil || strings.TrimSpace(c.BaseURL) == "" {
		return RemoteSource{}, errors.New("remote registry is not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.BaseURL, "/")+"/v1/materializations?reference="+url.QueryEscape(reference), nil)
	if err != nil {
		return RemoteSource{}, err
	}

	var payload Materialization
	if err := c.doJSON(req, &payload); err != nil {
		return RemoteSource{}, fmt.Errorf("load remote materialization: %w", err)
	}

	return RemoteSource{
		Reference:     payload.Reference,
		SeedID:        payload.SeedID,
		RealizationID: payload.RealizationID,
		ApproachID:    payload.ApproachID,
		Summary:       payload.Summary,
		Status:        payload.Status,
		FetchedAt:     sTime(payload.GeneratedAt),
		Sources:       payload.Sources,
		Files:         collectRemoteFiles(payload),
		Warnings:      payload.Warnings,
	}, nil
}

func (c *RemoteRegistryClient) doJSON(req *http.Request, out interface{}) error {
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: 4 * time.Second}
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return ErrReferenceNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

func collectDocPreview(repoRoot, relativePath, kind string) FilePreview {
	if relativePath == "" {
		return FilePreview{}
	}

	preview, err := readPreview(repoRoot, filepath.Join(repoRoot, filepath.FromSlash(relativePath)), kind, "repo")
	if err != nil {
		return FilePreview{
			Path:    relativePath,
			Kind:    kind,
			Preview: err.Error(),
			Source:  "repo",
		}
	}

	return preview
}

func readPreview(repoRoot, path, kind, source string) (FilePreview, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return FilePreview{}, fmt.Errorf("read %s: %w", path, err)
	}

	preview := string(raw)
	truncated := false
	if len(preview) > 700 {
		preview = strings.TrimSpace(preview[:700]) + "\n..."
		truncated = true
	}

	return FilePreview{
		Path:       relativeOrOriginal(repoRoot, path),
		Kind:       kind,
		Preview:    strings.TrimSpace(preview),
		Truncated:  truncated,
		Source:     source,
		ByteLength: len(raw),
	}, nil
}

func collectRemoteFiles(payload Materialization) []FilePreview {
	var files []FilePreview
	if payload.Local != nil {
		for _, file := range payload.Local.Files {
			file.Source = "remote"
			files = append(files, file)
		}
	}
	if payload.Remote != nil {
		for _, file := range payload.Remote.Files {
			file.Source = "remote"
			files = append(files, file)
		}
	}

	return compactFiles(files)
}

func compactFiles(files []FilePreview) []FilePreview {
	out := files[:0]
	for _, file := range files {
		if file.Path == "" && file.Preview == "" {
			continue
		}
		out = append(out, file)
	}

	return out
}

func adoptLocalSummary(result *Materialization, source LocalSource) {
	if result.SeedID == "" {
		result.SeedID = source.SeedID
	}
	if result.RealizationID == "" {
		result.RealizationID = source.RealizationID
	}
	if result.ApproachID == "" {
		result.ApproachID = source.ApproachID
	}
	if result.Summary == "" {
		result.Summary = source.Summary
	}
	if result.Status == "" {
		result.Status = source.Status
	}
}

func adoptRemoteSummary(result *Materialization, source RemoteSource) {
	if result.SeedID == "" {
		result.SeedID = source.SeedID
	}
	if result.RealizationID == "" {
		result.RealizationID = source.RealizationID
	}
	if result.ApproachID == "" {
		result.ApproachID = source.ApproachID
	}
	if result.Summary == "" {
		result.Summary = source.Summary
	}
	if result.Status == "" {
		result.Status = source.Status
	}
}

func relativeOrOriginal(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}

	return filepath.ToSlash(rel)
}

func sTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value
}
