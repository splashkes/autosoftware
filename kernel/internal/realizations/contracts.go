package realizations

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var ErrInteractionContractMissing = errors.New("interaction contract missing")

type InteractionContract struct {
	ContractVersion string                  `yaml:"contract_version" json:"contract_version"`
	SurfaceKind     string                  `yaml:"surface_kind" json:"surface_kind"`
	SeedID          string                  `yaml:"seed_id" json:"seed_id"`
	RealizationID   string                  `yaml:"realization_id" json:"realization_id"`
	Summary         string                  `yaml:"summary" json:"summary"`
	Links           InteractionLinks        `yaml:"links" json:"links"`
	AuthModes       []string                `yaml:"auth_modes" json:"auth_modes"`
	Capabilities    []InteractionCapability `yaml:"capabilities" json:"capabilities"`
	DomainObjects   []InteractionObject     `yaml:"domain_objects" json:"domain_objects"`
	Commands        []InteractionCommand    `yaml:"commands" json:"commands"`
	Projections     []InteractionProjection `yaml:"projections" json:"projections"`
	Consistency     InteractionConsistency  `yaml:"consistency" json:"consistency"`
}

type InteractionLinks struct {
	SeedDesign        string `yaml:"seed_design" json:"seed_design"`
	SeedBrief         string `yaml:"seed_brief,omitempty" json:"seed_brief,omitempty"`
	SeedAcceptance    string `yaml:"seed_acceptance,omitempty" json:"seed_acceptance,omitempty"`
	RealizationReadme string `yaml:"realization_readme" json:"realization_readme"`
}

type InteractionCapability struct {
	Name    string `yaml:"name" json:"name"`
	Summary string `yaml:"summary" json:"summary"`
}

type InteractionObject struct {
	Kind         string   `yaml:"kind" json:"kind"`
	Summary      string   `yaml:"summary" json:"summary"`
	SchemaRef    string   `yaml:"schema_ref" json:"schema_ref"`
	Capabilities []string `yaml:"capabilities" json:"capabilities"`
}

type InteractionCommand struct {
	Name            string   `yaml:"name" json:"name"`
	Summary         string   `yaml:"summary" json:"summary"`
	Path            string   `yaml:"path" json:"path"`
	ObjectKinds     []string `yaml:"object_kinds" json:"object_kinds"`
	AuthModes       []string `yaml:"auth_modes" json:"auth_modes"`
	Idempotency     string   `yaml:"idempotency" json:"idempotency"`
	InputSchemaRef  string   `yaml:"input_schema_ref" json:"input_schema_ref"`
	ResultSchemaRef string   `yaml:"result_schema_ref" json:"result_schema_ref"`
	Capabilities    []string `yaml:"capabilities" json:"capabilities"`
	Projection      string   `yaml:"projection" json:"projection"`
	Consistency     string   `yaml:"consistency" json:"consistency"`
}

type InteractionProjection struct {
	Name         string   `yaml:"name" json:"name"`
	Summary      string   `yaml:"summary" json:"summary"`
	Path         string   `yaml:"path" json:"path"`
	ObjectKinds  []string `yaml:"object_kinds" json:"object_kinds"`
	AuthModes    []string `yaml:"auth_modes" json:"auth_modes"`
	Capabilities []string `yaml:"capabilities" json:"capabilities"`
	Freshness    string   `yaml:"freshness" json:"freshness"`
}

type InteractionConsistency struct {
	WriteVisibility     string `yaml:"write_visibility" json:"write_visibility"`
	ProjectionFreshness string `yaml:"projection_freshness" json:"projection_freshness"`
}

type LoadedInteractionContract struct {
	LocalRealization
	Contract     InteractionContract `json:"contract"`
	ContractFile string              `json:"contract_file"`
}

func DiscoverContracts(repoRoot string) ([]LoadedInteractionContract, error) {
	entries, err := Discover(repoRoot)
	if err != nil {
		return nil, err
	}

	out := make([]LoadedInteractionContract, 0, len(entries))
	for _, entry := range entries {
		contract, err := LoadInteractionContract(repoRoot, entry)
		if err != nil {
			return nil, err
		}
		out = append(out, contract)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Reference < out[j].Reference
	})

	return out, nil
}

func LoadInteractionContract(repoRoot string, entry LocalRealization) (LoadedInteractionContract, error) {
	contractPath := filepath.Join(entry.RootDir, "interaction_contract.yaml")
	raw, err := os.ReadFile(contractPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return LoadedInteractionContract{}, fmt.Errorf("%w: %s", ErrInteractionContractMissing, contractPath)
		}
		return LoadedInteractionContract{}, fmt.Errorf("read %s: %w", contractPath, err)
	}

	var contract InteractionContract
	if err := yaml.Unmarshal(raw, &contract); err != nil {
		return LoadedInteractionContract{}, fmt.Errorf("parse %s: %w", contractPath, err)
	}
	if err := validateInteractionContract(repoRoot, entry, contractPath, &contract); err != nil {
		return LoadedInteractionContract{}, fmt.Errorf("validate %s: %w", contractPath, err)
	}

	return LoadedInteractionContract{
		LocalRealization: entry,
		Contract:         contract,
		ContractFile:     candidateRelativePath(repoRoot, contractPath),
	}, nil
}

func validateInteractionContract(repoRoot string, entry LocalRealization, contractPath string, contract *InteractionContract) error {
	if strings.TrimSpace(contract.ContractVersion) != "v1" {
		return fmt.Errorf("contract_version must be v1")
	}
	if _, ok := allowedSurfaceKinds[strings.TrimSpace(contract.SurfaceKind)]; !ok {
		return fmt.Errorf("surface_kind must be one of %s", formatAllowedKeys(allowedSurfaceKinds))
	}
	if strings.TrimSpace(contract.SeedID) != entry.SeedID {
		return fmt.Errorf("seed_id %q must match realization manifest %q", contract.SeedID, entry.SeedID)
	}
	if strings.TrimSpace(contract.RealizationID) != entry.RealizationID {
		return fmt.Errorf("realization_id %q must match realization manifest %q", contract.RealizationID, entry.RealizationID)
	}
	if strings.TrimSpace(contract.Summary) == "" {
		return fmt.Errorf("summary is required")
	}
	if strings.TrimSpace(contract.Links.SeedDesign) == "" {
		return fmt.Errorf("links.seed_design is required")
	}
	if err := validateContractRef(repoRoot, contractPath, contract.Links.SeedDesign); err != nil {
		return fmt.Errorf("links.seed_design: %w", err)
	}
	if strings.TrimSpace(contract.Links.RealizationReadme) == "" {
		return fmt.Errorf("links.realization_readme is required")
	}
	if err := validateContractRef(repoRoot, contractPath, contract.Links.RealizationReadme); err != nil {
		return fmt.Errorf("links.realization_readme: %w", err)
	}
	if strings.TrimSpace(contract.Links.SeedBrief) != "" {
		if err := validateContractRef(repoRoot, contractPath, contract.Links.SeedBrief); err != nil {
			return fmt.Errorf("links.seed_brief: %w", err)
		}
	}
	if strings.TrimSpace(contract.Links.SeedAcceptance) != "" {
		if err := validateContractRef(repoRoot, contractPath, contract.Links.SeedAcceptance); err != nil {
			return fmt.Errorf("links.seed_acceptance: %w", err)
		}
	}

	authModes, err := validateNamedList(contract.AuthModes, allowedAuthModes, "auth_modes")
	if err != nil {
		return err
	}

	capabilities, err := validateCapabilities(contract.Capabilities)
	if err != nil {
		return err
	}

	objectKinds, err := validateObjects(repoRoot, contractPath, contract.DomainObjects, capabilities)
	if err != nil {
		return err
	}

	if err := validateConsistencyDefaults(contract.Consistency); err != nil {
		return err
	}

	projections, err := validateProjections(entry.SeedID, contract.Projections, objectKinds, capabilities, authModes)
	if err != nil {
		return err
	}

	if contract.SurfaceKind == "interactive" && len(contract.Commands) == 0 {
		return fmt.Errorf("interactive realizations must declare at least one command")
	}

	if err := validateCommands(entry.SeedID, repoRoot, contractPath, contract.Commands, objectKinds, projections, capabilities, authModes); err != nil {
		return err
	}

	return nil
}

func validateCapabilities(items []InteractionCapability) (map[string]struct{}, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("capabilities must declare at least one kernel capability binding")
	}

	out := make(map[string]struct{}, len(items))
	for i, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			return nil, fmt.Errorf("capabilities[%d].name is required", i)
		}
		if _, ok := allowedCapabilities[name]; !ok {
			return nil, fmt.Errorf("capabilities[%d].name %q must be one of %s", i, name, formatAllowedKeys(allowedCapabilities))
		}
		if _, exists := out[name]; exists {
			return nil, fmt.Errorf("capabilities[%d].name %q is duplicated", i, name)
		}
		if strings.TrimSpace(item.Summary) == "" {
			return nil, fmt.Errorf("capabilities[%d].summary is required", i)
		}
		out[name] = struct{}{}
	}
	return out, nil
}

func validateObjects(repoRoot, contractPath string, items []InteractionObject, capabilities map[string]struct{}) (map[string]struct{}, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("domain_objects must declare at least one domain object")
	}

	out := make(map[string]struct{}, len(items))
	for i, item := range items {
		kind := strings.TrimSpace(item.Kind)
		if kind == "" {
			return nil, fmt.Errorf("domain_objects[%d].kind is required", i)
		}
		if _, exists := out[kind]; exists {
			return nil, fmt.Errorf("domain_objects[%d].kind %q is duplicated", i, kind)
		}
		if strings.TrimSpace(item.Summary) == "" {
			return nil, fmt.Errorf("domain_objects[%d].summary is required", i)
		}
		if strings.TrimSpace(item.SchemaRef) == "" {
			return nil, fmt.Errorf("domain_objects[%d].schema_ref is required", i)
		}
		if err := validateContractRef(repoRoot, contractPath, item.SchemaRef); err != nil {
			return nil, fmt.Errorf("domain_objects[%d].schema_ref: %w", i, err)
		}
		if err := validateSubset(fmt.Sprintf("domain_objects[%d].capabilities", i), item.Capabilities, capabilities); err != nil {
			return nil, err
		}
		out[kind] = struct{}{}
	}
	return out, nil
}

func validateConsistencyDefaults(item InteractionConsistency) error {
	if _, ok := allowedWriteVisibility[strings.TrimSpace(item.WriteVisibility)]; !ok {
		return fmt.Errorf("consistency.write_visibility must be one of %s", formatAllowedKeys(allowedWriteVisibility))
	}
	if _, ok := allowedProjectionFreshness[strings.TrimSpace(item.ProjectionFreshness)]; !ok {
		return fmt.Errorf("consistency.projection_freshness must be one of %s", formatAllowedKeys(allowedProjectionFreshness))
	}
	return nil
}

func validateProjections(seedID string, items []InteractionProjection, objectKinds, capabilities, authModes map[string]struct{}) (map[string]struct{}, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("projections must declare at least one projection")
	}

	out := make(map[string]struct{}, len(items))
	paths := make(map[string]struct{}, len(items))
	prefix := "/v1/projections/" + seedID + "/"
	for i, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			return nil, fmt.Errorf("projections[%d].name is required", i)
		}
		if _, exists := out[name]; exists {
			return nil, fmt.Errorf("projections[%d].name %q is duplicated", i, name)
		}
		if strings.TrimSpace(item.Summary) == "" {
			return nil, fmt.Errorf("projections[%d].summary is required", i)
		}
		path := strings.TrimSpace(item.Path)
		if path == "" {
			return nil, fmt.Errorf("projections[%d].path is required", i)
		}
		if !strings.HasPrefix(path, prefix) {
			return nil, fmt.Errorf("projections[%d].path %q must start with %q", i, path, prefix)
		}
		if _, exists := paths[path]; exists {
			return nil, fmt.Errorf("projections[%d].path %q is duplicated", i, path)
		}
		if err := validateSubset(fmt.Sprintf("projections[%d].object_kinds", i), item.ObjectKinds, objectKinds); err != nil {
			return nil, err
		}
		if err := validateSubset(fmt.Sprintf("projections[%d].auth_modes", i), item.AuthModes, authModes); err != nil {
			return nil, err
		}
		if err := validateSubset(fmt.Sprintf("projections[%d].capabilities", i), item.Capabilities, capabilities); err != nil {
			return nil, err
		}
		if _, ok := allowedProjectionFreshness[strings.TrimSpace(item.Freshness)]; !ok {
			return nil, fmt.Errorf("projections[%d].freshness must be one of %s", i, formatAllowedKeys(allowedProjectionFreshness))
		}
		out[name] = struct{}{}
		paths[path] = struct{}{}
	}
	return out, nil
}

func validateCommands(seedID, repoRoot, contractPath string, items []InteractionCommand, objectKinds, projections, capabilities, authModes map[string]struct{}) error {
	names := make(map[string]struct{}, len(items))
	paths := make(map[string]struct{}, len(items))
	prefix := "/v1/commands/" + seedID + "/"
	for i, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			return fmt.Errorf("commands[%d].name is required", i)
		}
		if _, exists := names[name]; exists {
			return fmt.Errorf("commands[%d].name %q is duplicated", i, name)
		}
		if strings.TrimSpace(item.Summary) == "" {
			return fmt.Errorf("commands[%d].summary is required", i)
		}
		path := strings.TrimSpace(item.Path)
		if path == "" {
			return fmt.Errorf("commands[%d].path is required", i)
		}
		if !strings.HasPrefix(path, prefix) {
			return fmt.Errorf("commands[%d].path %q must start with %q", i, path, prefix)
		}
		if _, exists := paths[path]; exists {
			return fmt.Errorf("commands[%d].path %q is duplicated", i, path)
		}
		if err := validateSubset(fmt.Sprintf("commands[%d].object_kinds", i), item.ObjectKinds, objectKinds); err != nil {
			return err
		}
		if err := validateSubset(fmt.Sprintf("commands[%d].auth_modes", i), item.AuthModes, authModes); err != nil {
			return err
		}
		if _, ok := allowedIdempotency[strings.TrimSpace(item.Idempotency)]; !ok {
			return fmt.Errorf("commands[%d].idempotency must be one of %s", i, formatAllowedKeys(allowedIdempotency))
		}
		if strings.TrimSpace(item.InputSchemaRef) == "" {
			return fmt.Errorf("commands[%d].input_schema_ref is required", i)
		}
		if err := validateContractRef(repoRoot, contractPath, item.InputSchemaRef); err != nil {
			return fmt.Errorf("commands[%d].input_schema_ref: %w", i, err)
		}
		if strings.TrimSpace(item.ResultSchemaRef) == "" {
			return fmt.Errorf("commands[%d].result_schema_ref is required", i)
		}
		if err := validateContractRef(repoRoot, contractPath, item.ResultSchemaRef); err != nil {
			return fmt.Errorf("commands[%d].result_schema_ref: %w", i, err)
		}
		if err := validateSubset(fmt.Sprintf("commands[%d].capabilities", i), item.Capabilities, capabilities); err != nil {
			return err
		}
		if _, ok := projections[strings.TrimSpace(item.Projection)]; !ok {
			return fmt.Errorf("commands[%d].projection %q must reference a declared projection", i, item.Projection)
		}
		if _, ok := allowedWriteVisibility[strings.TrimSpace(item.Consistency)]; !ok {
			return fmt.Errorf("commands[%d].consistency must be one of %s", i, formatAllowedKeys(allowedWriteVisibility))
		}
		names[name] = struct{}{}
		paths[path] = struct{}{}
	}
	return nil
}

func validateNamedList(items []string, allowed map[string]struct{}, field string) (map[string]struct{}, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("%s must declare at least one value", field)
	}

	out := make(map[string]struct{}, len(items))
	for i, item := range items {
		value := strings.TrimSpace(item)
		if value == "" {
			return nil, fmt.Errorf("%s[%d] is required", field, i)
		}
		if _, ok := allowed[value]; !ok {
			return nil, fmt.Errorf("%s[%d] %q must be one of %s", field, i, value, formatAllowedKeys(allowed))
		}
		if _, exists := out[value]; exists {
			return nil, fmt.Errorf("%s[%d] %q is duplicated", field, i, value)
		}
		out[value] = struct{}{}
	}
	return out, nil
}

func validateSubset(field string, items []string, allowed map[string]struct{}) error {
	if len(items) == 0 {
		return fmt.Errorf("%s must declare at least one value", field)
	}

	seen := make(map[string]struct{}, len(items))
	for i, item := range items {
		value := strings.TrimSpace(item)
		if value == "" {
			return fmt.Errorf("%s[%d] is required", field, i)
		}
		if _, ok := allowed[value]; !ok {
			return fmt.Errorf("%s[%d] %q must reference a declared value", field, i, value)
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("%s[%d] %q is duplicated", field, i, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validateContractRef(repoRoot, contractPath, ref string) error {
	path := strings.TrimSpace(strings.Split(ref, "#")[0])
	if path == "" {
		return fmt.Errorf("reference must include a file path")
	}

	target := path
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(contractPath), filepath.FromSlash(target))
	}
	if !PathContained(repoRoot, target) {
		return fmt.Errorf("reference %q escapes repository root", ref)
	}
	if _, err := os.Stat(target); err != nil {
		return fmt.Errorf("resolve %q: %w", ref, err)
	}
	return nil
}

func formatAllowedKeys(items map[string]struct{}) string {
	values := make([]string, 0, len(items))
	for key := range items {
		values = append(values, key)
	}
	sort.Strings(values)
	return strings.Join(values, ", ")
}

var allowedSurfaceKinds = map[string]struct{}{
	"bootstrap_only": {},
	"interactive":    {},
	"read_only":      {},
}

var allowedAuthModes = map[string]struct{}{
	"access_link":   {},
	"anonymous":     {},
	"service_token": {},
	"session":       {},
}

var allowedCapabilities = map[string]struct{}{
	"access_links":             {},
	"activity_events":          {},
	"assignments":              {},
	"auth_challenges":          {},
	"consent":                  {},
	"guard_decisions":          {},
	"handles":                  {},
	"jobs":                     {},
	"memberships":              {},
	"messages":                 {},
	"notification_preferences": {},
	"outbox":                   {},
	"principal_identifiers":    {},
	"principals":               {},
	"publications":             {},
	"risk_events":              {},
	"search_documents":         {},
	"search_facets":            {},
	"sessions":                 {},
	"state_transitions":        {},
	"subscriptions":            {},
	"threads":                  {},
	"uploads":                  {},
}

var allowedIdempotency = map[string]struct{}{
	"none":        {},
	"recommended": {},
	"required":    {},
}

var allowedWriteVisibility = map[string]struct{}{
	"async_job":        {},
	"eventual":         {},
	"read_your_writes": {},
}

var allowedProjectionFreshness = map[string]struct{}{
	"direct":       {},
	"eventual":     {},
	"materialized": {},
}
