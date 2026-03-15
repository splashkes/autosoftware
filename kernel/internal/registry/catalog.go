package registry

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"as/kernel/internal/realizations"
)

type Catalog struct {
	Summary CatalogSummary  `json:"summary"`
	Objects []CatalogObject `json:"objects"`
	Schemas []CatalogSchema `json:"schemas"`
}

type CatalogSummary struct {
	Realizations int `json:"realizations"`
	Contracts    int `json:"contracts"`
	Objects      int `json:"objects"`
	Schemas      int `json:"schemas"`
	Commands     int `json:"commands"`
	Projections  int `json:"projections"`
}

type CatalogObject struct {
	SeedID       string                     `json:"seed_id"`
	Kind         string                     `json:"kind"`
	Summary      string                     `json:"summary"`
	Capabilities []string                   `json:"capabilities"`
	SchemaRefs   []string                   `json:"schema_refs"`
	Realizations []CatalogObjectRealization `json:"realizations"`
	Commands     []CatalogCommand           `json:"commands"`
	Projections  []CatalogProjection        `json:"projections"`
}

type CatalogObjectRealization struct {
	Reference     string   `json:"reference"`
	SeedID        string   `json:"seed_id"`
	RealizationID string   `json:"realization_id"`
	ApproachID    string   `json:"approach_id,omitempty"`
	Summary       string   `json:"summary"`
	Status        string   `json:"status"`
	SurfaceKind   string   `json:"surface_kind"`
	ContractFile  string   `json:"contract_file"`
	SchemaRef     string   `json:"schema_ref"`
	Capabilities  []string `json:"capabilities"`
}

type CatalogCommand struct {
	Reference       string   `json:"reference"`
	SeedID          string   `json:"seed_id"`
	RealizationID   string   `json:"realization_id"`
	Name            string   `json:"name"`
	Summary         string   `json:"summary"`
	Path            string   `json:"path"`
	AuthModes       []string `json:"auth_modes"`
	Capabilities    []string `json:"capabilities"`
	Idempotency     string   `json:"idempotency"`
	InputSchemaRef  string   `json:"input_schema_ref"`
	ResultSchemaRef string   `json:"result_schema_ref"`
	Projection      string   `json:"projection"`
	Consistency     string   `json:"consistency"`
	ContractFile    string   `json:"contract_file"`
}

type CatalogProjection struct {
	Reference     string   `json:"reference"`
	SeedID        string   `json:"seed_id"`
	RealizationID string   `json:"realization_id"`
	Name          string   `json:"name"`
	Summary       string   `json:"summary"`
	Path          string   `json:"path"`
	Capabilities  []string `json:"capabilities"`
	Freshness     string   `json:"freshness"`
	ContractFile  string   `json:"contract_file"`
}

type CatalogSchema struct {
	Ref            string                    `json:"ref"`
	Path           string                    `json:"path"`
	Anchor         string                    `json:"anchor,omitempty"`
	ObjectUses     []CatalogSchemaObjectUse  `json:"object_uses"`
	CommandInputs  []CatalogSchemaCommandUse `json:"command_inputs"`
	CommandResults []CatalogSchemaCommandUse `json:"command_results"`
}

type CatalogSchemaObjectUse struct {
	Reference     string `json:"reference"`
	SeedID        string `json:"seed_id"`
	RealizationID string `json:"realization_id"`
	Kind          string `json:"kind"`
	Summary       string `json:"summary"`
	ContractFile  string `json:"contract_file"`
}

type CatalogSchemaCommandUse struct {
	Reference     string `json:"reference"`
	SeedID        string `json:"seed_id"`
	RealizationID string `json:"realization_id"`
	Name          string `json:"name"`
	Summary       string `json:"summary"`
	Path          string `json:"path"`
	ContractFile  string `json:"contract_file"`
}

type resolvedRef struct {
	Canonical string
	Path      string
	Anchor    string
}

func LoadCatalog(repoRoot string) (Catalog, error) {
	contracts, err := realizations.DiscoverContracts(repoRoot)
	if err != nil {
		return Catalog{}, err
	}

	objectIndex := make(map[string]*CatalogObject)
	schemaIndex := make(map[string]*CatalogSchema)

	catalog := Catalog{
		Summary: CatalogSummary{
			Realizations: len(contracts),
			Contracts:    len(contracts),
		},
	}

	for _, item := range contracts {
		catalog.Summary.Commands += len(item.Contract.Commands)
		catalog.Summary.Projections += len(item.Contract.Projections)

		contractPath := filepath.Join(item.RootDir, "interaction_contract.yaml")
		contractFile := firstNonEmpty(item.ContractFile, candidateRelativePath(repoRoot, contractPath))

		for _, object := range item.Contract.DomainObjects {
			detail := ensureObject(objectIndex, item.SeedID, object.Kind)
			if detail.Summary == "" {
				detail.Summary = strings.TrimSpace(object.Summary)
			}
			addUnique(&detail.Capabilities, object.Capabilities...)

			schemaRef := resolveContractRef(repoRoot, contractPath, object.SchemaRef)
			if schemaRef.Canonical != "" {
				addUnique(&detail.SchemaRefs, schemaRef.Canonical)
				schema := ensureSchema(schemaIndex, schemaRef)
				schema.ObjectUses = append(schema.ObjectUses, CatalogSchemaObjectUse{
					Reference:     item.Reference,
					SeedID:        item.SeedID,
					RealizationID: item.RealizationID,
					Kind:          object.Kind,
					Summary:       strings.TrimSpace(object.Summary),
					ContractFile:  contractFile,
				})
			}

			detail.Realizations = append(detail.Realizations, CatalogObjectRealization{
				Reference:     item.Reference,
				SeedID:        item.SeedID,
				RealizationID: item.RealizationID,
				ApproachID:    item.ApproachID,
				Summary:       firstNonEmpty(strings.TrimSpace(item.Contract.Summary), strings.TrimSpace(item.Summary), item.RealizationID),
				Status:        strings.TrimSpace(item.Status),
				SurfaceKind:   strings.TrimSpace(item.Contract.SurfaceKind),
				ContractFile:  contractFile,
				SchemaRef:     schemaRef.Canonical,
				Capabilities:  append([]string(nil), object.Capabilities...),
			})
		}

		for _, command := range item.Contract.Commands {
			inputRef := resolveContractRef(repoRoot, contractPath, command.InputSchemaRef)
			resultRef := resolveContractRef(repoRoot, contractPath, command.ResultSchemaRef)

			for _, kind := range command.ObjectKinds {
				detail := ensureObject(objectIndex, item.SeedID, kind)
				detail.Commands = append(detail.Commands, CatalogCommand{
					Reference:       item.Reference,
					SeedID:          item.SeedID,
					RealizationID:   item.RealizationID,
					Name:            strings.TrimSpace(command.Name),
					Summary:         strings.TrimSpace(command.Summary),
					Path:            strings.TrimSpace(command.Path),
					AuthModes:       append([]string(nil), command.AuthModes...),
					Capabilities:    append([]string(nil), command.Capabilities...),
					Idempotency:     strings.TrimSpace(command.Idempotency),
					InputSchemaRef:  inputRef.Canonical,
					ResultSchemaRef: resultRef.Canonical,
					Projection:      strings.TrimSpace(command.Projection),
					Consistency:     strings.TrimSpace(command.Consistency),
					ContractFile:    contractFile,
				})
			}

			if inputRef.Canonical != "" {
				schema := ensureSchema(schemaIndex, inputRef)
				schema.CommandInputs = append(schema.CommandInputs, CatalogSchemaCommandUse{
					Reference:     item.Reference,
					SeedID:        item.SeedID,
					RealizationID: item.RealizationID,
					Name:          strings.TrimSpace(command.Name),
					Summary:       strings.TrimSpace(command.Summary),
					Path:          strings.TrimSpace(command.Path),
					ContractFile:  contractFile,
				})
			}
			if resultRef.Canonical != "" {
				schema := ensureSchema(schemaIndex, resultRef)
				schema.CommandResults = append(schema.CommandResults, CatalogSchemaCommandUse{
					Reference:     item.Reference,
					SeedID:        item.SeedID,
					RealizationID: item.RealizationID,
					Name:          strings.TrimSpace(command.Name),
					Summary:       strings.TrimSpace(command.Summary),
					Path:          strings.TrimSpace(command.Path),
					ContractFile:  contractFile,
				})
			}
		}

		for _, projection := range item.Contract.Projections {
			for _, kind := range projection.ObjectKinds {
				detail := ensureObject(objectIndex, item.SeedID, kind)
				detail.Projections = append(detail.Projections, CatalogProjection{
					Reference:     item.Reference,
					SeedID:        item.SeedID,
					RealizationID: item.RealizationID,
					Name:          strings.TrimSpace(projection.Name),
					Summary:       strings.TrimSpace(projection.Summary),
					Path:          strings.TrimSpace(projection.Path),
					Capabilities:  append([]string(nil), projection.Capabilities...),
					Freshness:     strings.TrimSpace(projection.Freshness),
					ContractFile:  contractFile,
				})
			}
		}
	}

	catalog.Objects = flattenObjects(objectIndex)
	catalog.Schemas = flattenSchemas(schemaIndex)
	catalog.Summary.Objects = len(catalog.Objects)
	catalog.Summary.Schemas = len(catalog.Schemas)
	return catalog, nil
}

func GetObject(catalog Catalog, seedID, kind string) (CatalogObject, bool) {
	seedID = strings.TrimSpace(seedID)
	kind = strings.TrimSpace(kind)
	for _, object := range catalog.Objects {
		if object.SeedID == seedID && object.Kind == kind {
			return object, true
		}
	}
	return CatalogObject{}, false
}

func GetSchema(catalog Catalog, ref string) (CatalogSchema, bool) {
	ref = strings.TrimSpace(ref)
	for _, schema := range catalog.Schemas {
		if schema.Ref == ref {
			return schema, true
		}
	}
	return CatalogSchema{}, false
}

func ensureObject(index map[string]*CatalogObject, seedID, kind string) *CatalogObject {
	key := seedID + "\x00" + kind
	if item, ok := index[key]; ok {
		return item
	}
	item := &CatalogObject{
		SeedID: strings.TrimSpace(seedID),
		Kind:   strings.TrimSpace(kind),
	}
	index[key] = item
	return item
}

func ensureSchema(index map[string]*CatalogSchema, ref resolvedRef) *CatalogSchema {
	if item, ok := index[ref.Canonical]; ok {
		return item
	}
	item := &CatalogSchema{
		Ref:    ref.Canonical,
		Path:   ref.Path,
		Anchor: ref.Anchor,
	}
	index[ref.Canonical] = item
	return item
}

func flattenObjects(index map[string]*CatalogObject) []CatalogObject {
	out := make([]CatalogObject, 0, len(index))
	for _, item := range index {
		sort.Strings(item.Capabilities)
		sort.Strings(item.SchemaRefs)
		sort.Slice(item.Realizations, func(i, j int) bool {
			return item.Realizations[i].Reference < item.Realizations[j].Reference
		})
		sort.Slice(item.Commands, func(i, j int) bool {
			if item.Commands[i].Reference == item.Commands[j].Reference {
				return item.Commands[i].Name < item.Commands[j].Name
			}
			return item.Commands[i].Reference < item.Commands[j].Reference
		})
		sort.Slice(item.Projections, func(i, j int) bool {
			if item.Projections[i].Reference == item.Projections[j].Reference {
				return item.Projections[i].Name < item.Projections[j].Name
			}
			return item.Projections[i].Reference < item.Projections[j].Reference
		})
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SeedID == out[j].SeedID {
			return out[i].Kind < out[j].Kind
		}
		return out[i].SeedID < out[j].SeedID
	})
	return out
}

func flattenSchemas(index map[string]*CatalogSchema) []CatalogSchema {
	out := make([]CatalogSchema, 0, len(index))
	for _, item := range index {
		sort.Slice(item.ObjectUses, func(i, j int) bool {
			if item.ObjectUses[i].Reference == item.ObjectUses[j].Reference {
				return item.ObjectUses[i].Kind < item.ObjectUses[j].Kind
			}
			return item.ObjectUses[i].Reference < item.ObjectUses[j].Reference
		})
		sort.Slice(item.CommandInputs, func(i, j int) bool {
			if item.CommandInputs[i].Reference == item.CommandInputs[j].Reference {
				return item.CommandInputs[i].Name < item.CommandInputs[j].Name
			}
			return item.CommandInputs[i].Reference < item.CommandInputs[j].Reference
		})
		sort.Slice(item.CommandResults, func(i, j int) bool {
			if item.CommandResults[i].Reference == item.CommandResults[j].Reference {
				return item.CommandResults[i].Name < item.CommandResults[j].Name
			}
			return item.CommandResults[i].Reference < item.CommandResults[j].Reference
		})
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Ref < out[j].Ref
	})
	return out
}

func resolveContractRef(repoRoot, contractPath, ref string) resolvedRef {
	raw := strings.TrimSpace(ref)
	if raw == "" {
		return resolvedRef{}
	}

	target, anchor, _ := strings.Cut(raw, "#")
	target = strings.TrimSpace(target)
	anchor = strings.TrimSpace(anchor)

	absTarget := contractPath
	if target != "" {
		if filepath.IsAbs(target) {
			absTarget = filepath.Clean(target)
		} else {
			absTarget = filepath.Join(filepath.Dir(contractPath), filepath.FromSlash(target))
		}
	}

	rel, err := filepath.Rel(repoRoot, absTarget)
	if err != nil {
		rel = absTarget
	}
	rel = filepath.ToSlash(rel)

	canonical := rel
	if anchor != "" {
		canonical += "#" + anchor
	}

	return resolvedRef{
		Canonical: canonical,
		Path:      rel,
		Anchor:    anchor,
	}
}

func candidateRelativePath(repoRoot, path string) string {
	rel, err := filepath.Rel(repoRoot, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func addUnique(dest *[]string, values ...string) {
	seen := make(map[string]struct{}, len(*dest))
	for _, item := range *dest {
		seen[item] = struct{}{}
	}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		*dest = append(*dest, trimmed)
		seen[trimmed] = struct{}{}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func FilterObjects(items []CatalogObject, seedID, schemaRef, query string) []CatalogObject {
	seedID = strings.TrimSpace(seedID)
	schemaRef = strings.TrimSpace(schemaRef)
	query = strings.TrimSpace(query)

	var out []CatalogObject
	for _, item := range items {
		if seedID != "" && item.SeedID != seedID {
			continue
		}
		if schemaRef != "" && !contains(item.SchemaRefs, schemaRef) {
			continue
		}
		if query != "" && !matchesObjectQuery(item, query) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func FilterSchemas(items []CatalogSchema, seedID, query string) []CatalogSchema {
	seedID = strings.TrimSpace(seedID)
	query = strings.TrimSpace(query)

	var out []CatalogSchema
	for _, item := range items {
		if seedID != "" && !schemaHasSeed(item, seedID) {
			continue
		}
		if query != "" && !matchesSchemaQuery(item, query) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func matchesObjectQuery(item CatalogObject, query string) bool {
	for _, candidate := range []string{item.SeedID, item.Kind, item.Summary} {
		if containsFold(candidate, query) {
			return true
		}
	}
	for _, value := range item.SchemaRefs {
		if containsFold(value, query) {
			return true
		}
	}
	for _, value := range item.Capabilities {
		if containsFold(value, query) {
			return true
		}
	}
	return false
}

func matchesSchemaQuery(item CatalogSchema, query string) bool {
	for _, candidate := range []string{item.Ref, item.Path, item.Anchor} {
		if containsFold(candidate, query) {
			return true
		}
	}
	for _, use := range item.ObjectUses {
		for _, candidate := range []string{use.SeedID, use.Kind, use.Summary} {
			if containsFold(candidate, query) {
				return true
			}
		}
	}
	for _, use := range append(append([]CatalogSchemaCommandUse(nil), item.CommandInputs...), item.CommandResults...) {
		for _, candidate := range []string{use.SeedID, use.Name, use.Path, use.Summary} {
			if containsFold(candidate, query) {
				return true
			}
		}
	}
	return false
}

func schemaHasSeed(item CatalogSchema, seedID string) bool {
	for _, use := range item.ObjectUses {
		if use.SeedID == seedID {
			return true
		}
	}
	for _, use := range item.CommandInputs {
		if use.SeedID == seedID {
			return true
		}
	}
	for _, use := range item.CommandResults {
		if use.SeedID == seedID {
			return true
		}
	}
	return false
}

func containsFold(text, query string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(query))
}

func (c CatalogObject) String() string {
	return fmt.Sprintf("%s/%s", c.SeedID, c.Kind)
}
