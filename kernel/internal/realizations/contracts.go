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
	DomainRelations []InteractionRelation   `yaml:"domain_relations,omitempty" json:"domain_relations,omitempty"`
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
	Kind         string                `yaml:"kind" json:"kind"`
	Summary      string                `yaml:"summary" json:"summary"`
	SchemaRef    string                `yaml:"schema_ref" json:"schema_ref"`
	Capabilities []string              `yaml:"capabilities" json:"capabilities"`
	DataLayout   InteractionDataLayout `yaml:"data_layout,omitempty" json:"data_layout,omitempty"`
}

type InteractionRelation struct {
	Kind         string                 `yaml:"kind" json:"kind"`
	Summary      string                 `yaml:"summary" json:"summary"`
	FromKinds    []string               `yaml:"from_kinds" json:"from_kinds"`
	ToKinds      []string               `yaml:"to_kinds" json:"to_kinds"`
	Cardinality  string                 `yaml:"cardinality" json:"cardinality"`
	Visibility   string                 `yaml:"visibility" json:"visibility"`
	SchemaRef    string                 `yaml:"schema_ref" json:"schema_ref"`
	Capabilities []string               `yaml:"capabilities" json:"capabilities"`
	Attributes   []InteractionDataField `yaml:"attributes,omitempty" json:"attributes,omitempty"`
}

type InteractionDataLayout struct {
	SharedMetadata InteractionDataSection `yaml:"shared_metadata,omitempty" json:"shared_metadata,omitempty"`
	PublicPayload  InteractionDataSection `yaml:"public_payload,omitempty" json:"public_payload,omitempty"`
	PrivatePayload InteractionDataSection `yaml:"private_payload,omitempty" json:"private_payload,omitempty"`
	RuntimeOnly    InteractionDataSection `yaml:"runtime_only,omitempty" json:"runtime_only,omitempty"`
}

type InteractionDataSection struct {
	Summary string                 `yaml:"summary,omitempty" json:"summary,omitempty"`
	Fields  []InteractionDataField `yaml:"fields,omitempty" json:"fields,omitempty"`
}

type InteractionDataField struct {
	Name    string `yaml:"name" json:"name"`
	Type    string `yaml:"type,omitempty" json:"type,omitempty"`
	Summary string `yaml:"summary,omitempty" json:"summary,omitempty"`
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
	Name         string                `yaml:"name" json:"name"`
	Summary      string                `yaml:"summary" json:"summary"`
	Path         string                `yaml:"path" json:"path"`
	ObjectKinds  []string              `yaml:"object_kinds" json:"object_kinds"`
	AuthModes    []string              `yaml:"auth_modes" json:"auth_modes"`
	Capabilities []string              `yaml:"capabilities" json:"capabilities"`
	Freshness    string                `yaml:"freshness" json:"freshness"`
	DataViews    []InteractionDataView `yaml:"data_views,omitempty" json:"data_views,omitempty"`
}

type InteractionDataView struct {
	AuthModes []string `yaml:"auth_modes" json:"auth_modes"`
	Sections  []string `yaml:"sections" json:"sections"`
	Summary   string   `yaml:"summary,omitempty" json:"summary,omitempty"`
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
	if err := validateRelations(repoRoot, contractPath, contract.DomainRelations, objectKinds, capabilities); err != nil {
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
		if err := validateDataLayout(fmt.Sprintf("domain_objects[%d].data_layout", i), item.DataLayout); err != nil {
			return nil, err
		}
		out[kind] = struct{}{}
	}
	return out, nil
}

func validateRelations(repoRoot, contractPath string, items []InteractionRelation, objectKinds, capabilities map[string]struct{}) error {
	seen := make(map[string]struct{}, len(items))
	for i, item := range items {
		kind := strings.TrimSpace(item.Kind)
		if kind == "" {
			return fmt.Errorf("domain_relations[%d].kind is required", i)
		}
		if _, exists := seen[kind]; exists {
			return fmt.Errorf("domain_relations[%d].kind %q is duplicated", i, kind)
		}
		if strings.TrimSpace(item.Summary) == "" {
			return fmt.Errorf("domain_relations[%d].summary is required", i)
		}
		if err := validateSubset(fmt.Sprintf("domain_relations[%d].from_kinds", i), item.FromKinds, objectKinds); err != nil {
			return err
		}
		if err := validateSubset(fmt.Sprintf("domain_relations[%d].to_kinds", i), item.ToKinds, objectKinds); err != nil {
			return err
		}
		if _, ok := allowedRelationCardinality[strings.TrimSpace(item.Cardinality)]; !ok {
			return fmt.Errorf("domain_relations[%d].cardinality must be one of %s", i, formatAllowedKeys(allowedRelationCardinality))
		}
		if _, ok := allowedRelationVisibility[strings.TrimSpace(item.Visibility)]; !ok {
			return fmt.Errorf("domain_relations[%d].visibility must be one of %s", i, formatAllowedKeys(allowedRelationVisibility))
		}
		if strings.TrimSpace(item.SchemaRef) == "" {
			return fmt.Errorf("domain_relations[%d].schema_ref is required", i)
		}
		if err := validateContractRef(repoRoot, contractPath, item.SchemaRef); err != nil {
			return fmt.Errorf("domain_relations[%d].schema_ref: %w", i, err)
		}
		if err := validateSubset(fmt.Sprintf("domain_relations[%d].capabilities", i), item.Capabilities, capabilities); err != nil {
			return err
		}
		if err := validateFieldList(fmt.Sprintf("domain_relations[%d].attributes", i), item.Attributes); err != nil {
			return err
		}
		seen[kind] = struct{}{}
	}
	return nil
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
		if err := validateDataViews(fmt.Sprintf("projections[%d].data_views", i), item.DataViews, item.AuthModes, authModes); err != nil {
			return nil, err
		}
		out[name] = struct{}{}
		paths[path] = struct{}{}
	}
	return out, nil
}

func validateDataLayout(field string, layout InteractionDataLayout) error {
	if !layout.hasContent() {
		return nil
	}
	if err := validateDataSection(field+".shared_metadata", layout.SharedMetadata); err != nil {
		return err
	}
	if err := validateOptionalDataSection(field+".public_payload", layout.PublicPayload); err != nil {
		return err
	}
	if err := validateOptionalDataSection(field+".private_payload", layout.PrivatePayload); err != nil {
		return err
	}
	if err := validateOptionalDataSection(field+".runtime_only", layout.RuntimeOnly); err != nil {
		return err
	}
	return nil
}

func validateOptionalDataSection(field string, section InteractionDataSection) error {
	if !section.hasContent() {
		return nil
	}
	return validateDataSection(field, section)
}

func validateDataSection(field string, section InteractionDataSection) error {
	if strings.TrimSpace(section.Summary) == "" {
		return fmt.Errorf("%s.summary is required", field)
	}
	if len(section.Fields) == 0 {
		return fmt.Errorf("%s.fields must declare at least one field", field)
	}
	return validateFieldList(field+".fields", section.Fields)
}

func validateFieldList(field string, items []InteractionDataField) error {
	seen := map[string]struct{}{}
	for i, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			return fmt.Errorf("%s[%d].name is required", field, i)
		}
		if _, exists := seen[name]; exists {
			return fmt.Errorf("%s[%d].name %q is duplicated", field, i, name)
		}
		if strings.TrimSpace(item.Type) == "" {
			return fmt.Errorf("%s[%d].type is required", field, i)
		}
		if strings.TrimSpace(item.Summary) == "" {
			return fmt.Errorf("%s[%d].summary is required", field, i)
		}
		seen[name] = struct{}{}
	}
	return nil
}

func validateDataViews(field string, views []InteractionDataView, projectionModes []string, authModes map[string]struct{}) error {
	if len(views) == 0 {
		return nil
	}
	projectionAuthModes, err := validateNamedList(projectionModes, authModes, field+".projection_auth_modes")
	if err != nil {
		return err
	}
	covered := map[string]struct{}{}
	for i, view := range views {
		if err := validateSubset(fmt.Sprintf("%s[%d].auth_modes", field, i), view.AuthModes, projectionAuthModes); err != nil {
			return err
		}
		if err := validateSubset(fmt.Sprintf("%s[%d].sections", field, i), view.Sections, allowedDataLayoutSections); err != nil {
			return err
		}
		if strings.TrimSpace(view.Summary) == "" {
			return fmt.Errorf("%s[%d].summary is required", field, i)
		}
		for _, mode := range view.AuthModes {
			mode = strings.TrimSpace(mode)
			if _, exists := covered[mode]; exists {
				return fmt.Errorf("%s[%d].auth_modes %q is already covered by another data view", field, i, mode)
			}
			covered[mode] = struct{}{}
		}
	}
	for _, mode := range projectionModes {
		mode = strings.TrimSpace(mode)
		if _, ok := covered[mode]; !ok {
			return fmt.Errorf("%s must cover projection auth mode %q", field, mode)
		}
	}
	return nil
}

func (layout InteractionDataLayout) hasContent() bool {
	return layout.SharedMetadata.hasContent() || layout.PublicPayload.hasContent() || layout.PrivatePayload.hasContent() || layout.RuntimeOnly.hasContent()
}

func (section InteractionDataSection) hasContent() bool {
	return strings.TrimSpace(section.Summary) != "" || len(section.Fields) > 0
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

var allowedDataLayoutSections = map[string]struct{}{
	"private_payload": {},
	"public_payload":  {},
	"runtime_only":    {},
	"shared_metadata": {},
}

var allowedRelationCardinality = map[string]struct{}{
	"many_to_many": {},
	"many_to_one":  {},
	"one_to_many":  {},
	"one_to_one":   {},
}

var allowedRelationVisibility = map[string]struct{}{
	"mixed":        {},
	"private":      {},
	"public":       {},
	"runtime_only": {},
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
