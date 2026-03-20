package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func (s *postgresFlowershowStore) registryReference() string {
	return flowershowRegistryReference
}

func (s *postgresFlowershowStore) migrateLegacyClaimsToKernelRegistry(ctx context.Context) error {
	rows, err := s.registry.ListRows(ctx, registryListRowsInput{
		Reference:     s.registryReference(),
		SeedID:        s.seedID,
		RealizationID: s.realizationID,
		Limit:         1,
	})
	if err != nil {
		return fmt.Errorf("check kernel registry rows: %w", err)
	}
	if len(rows) > 0 {
		return nil
	}
	objects, claims, err := s.loadLegacyLocalClaims(ctx)
	if err != nil {
		return err
	}
	if len(claims) == 0 {
		return nil
	}
	appendInput := registryAppendChangeSetInput{
		ChangeSetID:    "flowershow-legacy-migration",
		Reference:      s.registryReference(),
		SeedID:         s.seedID,
		RealizationID:  s.realizationID,
		IdempotencyKey: "flowershow-legacy-migration",
		AcceptedBy:     "flowershow-legacy-migration",
		Metadata: map[string]any{
			"source": "as_flowershow_claims",
		},
	}
	seenObjects := make(map[string]struct{}, len(claims))
	for _, claim := range claims {
		if _, ok := seenObjects[claim.ObjectID]; ok {
			continue
		}
		object, ok := objects[claim.ObjectID]
		if !ok {
			continue
		}
		seenObjects[claim.ObjectID] = struct{}{}
		payload, err := structToMap(*object)
		if err != nil {
			return fmt.Errorf("marshal legacy object %s: %w", object.ID, err)
		}
		appendInput.Rows = append(appendInput.Rows, registryAppendRowInput{
			RowType:  "object.create",
			ObjectID: object.ID,
			Payload:  payload,
		})
	}
	for _, claim := range claims {
		payload, err := claimToMap(claim)
		if err != nil {
			return fmt.Errorf("marshal legacy claim %s: %w", claim.ID, err)
		}
		appendInput.Rows = append(appendInput.Rows, registryAppendRowInput{
			RowType:  "claim.create",
			ObjectID: claim.ObjectID,
			ClaimID:  claim.ID,
			Payload:  payload,
		})
	}
	if err := s.registry.AppendChangeSet(ctx, appendInput); err != nil {
		return fmt.Errorf("migrate legacy flowershow claims to kernel registry: %w", err)
	}
	return nil
}

func (s *postgresFlowershowStore) loadSnapshotFromKernelRegistry(ctx context.Context) (*memoryStore, error) {
	var (
		after int64
		all   []registryRowRecord
	)
	for {
		rows, err := s.registry.ListRows(ctx, registryListRowsInput{
			Reference:     s.registryReference(),
			SeedID:        s.seedID,
			RealizationID: s.realizationID,
			AfterRowID:    after,
			Limit:         5000,
		})
		if err != nil {
			return nil, fmt.Errorf("list kernel registry rows: %w", err)
		}
		if len(rows) == 0 {
			break
		}
		all = append(all, rows...)
		after = rows[len(rows)-1].RowID
	}
	objects := make(map[string]*FlowershowObject)
	claims := make([]FlowershowClaim, 0, len(all))
	for _, row := range all {
		switch strings.TrimSpace(row.RowType) {
		case "object.create":
			var object FlowershowObject
			if err := mapToStruct(row.Payload, &object); err != nil {
				return nil, fmt.Errorf("decode kernel registry object row %d: %w", row.RowID, err)
			}
			objects[object.ID] = &object
		case "claim.create":
			var claim FlowershowClaim
			if err := mapToStruct(row.Payload, &claim); err != nil {
				return nil, fmt.Errorf("decode kernel registry claim row %d: %w", row.RowID, err)
			}
			claims = append(claims, claim)
		}
	}
	if len(claims) == 0 {
		return newEmptyMemoryStore(), nil
	}
	replayed, err := replayFlowershowSnapshotFromClaims(objects, claims)
	if err != nil {
		return nil, fmt.Errorf("replay kernel registry claims: %w", err)
	}
	needsRebuild, err := s.projectionsNeedRebuild(ctx, replayed)
	if err != nil {
		return nil, fmt.Errorf("check projection rebuild necessity: %w", err)
	}
	if needsRebuild {
		if err := s.rebuildProjectionTablesFromSnapshot(ctx, replayed); err != nil {
			return nil, fmt.Errorf("rebuild projections from kernel registry: %w", err)
		}
	}
	return replayed, nil
}

func (s *postgresFlowershowStore) appendClaimsToKernelRegistry(ctx context.Context, mem *memoryStore, start int) error {
	if start < 0 || start > len(mem.claims) {
		start = len(mem.claims)
	}
	if start == len(mem.claims) {
		return nil
	}
	changeSetID := "fchg-" + mem.claims[start].ID
	input := registryAppendChangeSetInput{
		ChangeSetID:    changeSetID,
		Reference:      s.registryReference(),
		SeedID:         s.seedID,
		RealizationID:  s.realizationID,
		IdempotencyKey: changeSetID,
		AcceptedBy:     "flowershow-app",
		Metadata: map[string]any{
			"source": "flowershow-app",
		},
	}
	seenObjects := make(map[string]struct{})
	for _, claim := range mem.claims[start:] {
		if _, ok := seenObjects[claim.ObjectID]; ok {
			continue
		}
		object, ok := mem.objects[claim.ObjectID]
		if !ok {
			continue
		}
		seenObjects[claim.ObjectID] = struct{}{}
		payload, err := structToMap(*object)
		if err != nil {
			return fmt.Errorf("marshal registry object %s: %w", object.ID, err)
		}
		input.Rows = append(input.Rows, registryAppendRowInput{
			RowType:  "object.create",
			ObjectID: object.ID,
			Payload:  payload,
		})
	}
	for _, claim := range mem.claims[start:] {
		payload, err := claimToMap(claim)
		if err != nil {
			return fmt.Errorf("marshal registry claim %s: %w", claim.ID, err)
		}
		input.Rows = append(input.Rows, registryAppendRowInput{
			RowType:  "claim.create",
			ObjectID: claim.ObjectID,
			ClaimID:  claim.ID,
			Payload:  payload,
		})
	}
	if err := s.registry.AppendChangeSet(ctx, input); err != nil {
		return fmt.Errorf("append kernel registry change set: %w", err)
	}
	return nil
}

func (s *postgresFlowershowStore) loadLegacyLocalClaims(ctx context.Context) (map[string]*FlowershowObject, []FlowershowClaim, error) {
	objects := make(map[string]*FlowershowObject)
	rows, err := s.pool.Query(ctx, `SELECT object_id, coalesce(object_type, ''), coalesce(slug, ''), coalesce(created_at, NOW()), coalesce(created_by, '') FROM as_flowershow_objects`)
	if err != nil {
		return nil, nil, fmt.Errorf("load legacy objects: %w", err)
	}
	for rows.Next() {
		var item FlowershowObject
		if err := rows.Scan(&item.ID, &item.ObjectType, &item.Slug, &item.CreatedAt, &item.CreatedBy); err != nil {
			rows.Close()
			return nil, nil, fmt.Errorf("scan legacy object: %w", err)
		}
		copy := item
		objects[item.ID] = &copy
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, nil, fmt.Errorf("iterate legacy objects: %w", err)
	}
	rows.Close()

	claimRows, err := s.pool.Query(ctx, `SELECT claim_id, coalesce(object_id, ''), coalesce(claim_seq, 0), coalesce(claim_type, ''), coalesce(accepted_at, NOW()), coalesce(accepted_by, ''), coalesce(supersedes_claim_id, ''), payload FROM as_flowershow_claims ORDER BY coalesce(claim_seq, 0) ASC`)
	if err != nil {
		return nil, nil, fmt.Errorf("load legacy claims: %w", err)
	}
	defer claimRows.Close()

	claims := make([]FlowershowClaim, 0)
	for claimRows.Next() {
		var item FlowershowClaim
		var payload []byte
		if err := claimRows.Scan(&item.ID, &item.ObjectID, &item.ClaimSeq, &item.ClaimType, &item.AcceptedAt, &item.AcceptedBy, &item.SupersedesClaimID, &payload); err != nil {
			return nil, nil, fmt.Errorf("scan legacy claim: %w", err)
		}
		if len(payload) > 0 {
			var decoded any
			if err := json.Unmarshal(payload, &decoded); err != nil {
				return nil, nil, fmt.Errorf("decode legacy claim payload: %w", err)
			}
			item.Payload = decoded
		}
		claims = append(claims, item)
	}
	if err := claimRows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate legacy claims: %w", err)
	}
	return objects, claims, nil
}

func structToMap(value any) (map[string]any, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func mapToStruct(payload map[string]any, target any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

func claimToMap(claim FlowershowClaim) (map[string]any, error) {
	return structToMap(claim)
}
