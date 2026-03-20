package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const flowershowRegistryReference = "0007-Flowershow/a-firstbloom"

type registryBoundary interface {
	AppendChangeSet(context.Context, registryAppendChangeSetInput) error
	ListRows(context.Context, registryListRowsInput) ([]registryRowRecord, error)
	UpsertAuthorityBundle(context.Context, authorityBundleUpsertInput) error
	CreateAuthorityGrant(context.Context, authorityGrantCreateInput) error
	MaterializeAuthorityState(context.Context) error
}

type registryHTTPClient struct {
	baseURL       string
	internalToken string
	httpClient    *http.Client
}

type registryAppendChangeSetInput struct {
	ChangeSetID    string
	Reference      string
	SeedID         string
	RealizationID  string
	IdempotencyKey string
	AcceptedBy     string
	Metadata       map[string]any
	Rows           []registryAppendRowInput
}

type registryAppendRowInput struct {
	RowType  string         `json:"row_type"`
	ObjectID string         `json:"object_id,omitempty"`
	ClaimID  string         `json:"claim_id,omitempty"`
	Payload  map[string]any `json:"payload,omitempty"`
}

type registryRowRecord struct {
	RowID         int64          `json:"row_id"`
	ChangeSetID   string         `json:"change_set_id"`
	Reference     string         `json:"reference"`
	SeedID        string         `json:"seed_id"`
	RealizationID string         `json:"realization_id"`
	RowOrder      int            `json:"row_order"`
	RowType       string         `json:"row_type"`
	ObjectID      string         `json:"object_id"`
	ClaimID       string         `json:"claim_id"`
	Payload       map[string]any `json:"payload"`
	AcceptedAt    time.Time      `json:"accepted_at"`
}

type registryListRowsInput struct {
	Reference     string
	SeedID        string
	RealizationID string
	AfterRowID    int64
	Limit         int
}

type authorityBundleUpsertInput struct {
	BundleID     string         `json:"bundle_id"`
	DisplayName  string         `json:"display_name,omitempty"`
	Capabilities []string       `json:"capabilities"`
	Status       string         `json:"status,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type authorityGrantCreateInput struct {
	GrantID            string         `json:"grant_id,omitempty"`
	GrantorPrincipalID string         `json:"grantor_principal_id,omitempty"`
	GranteePrincipalID string         `json:"grantee_principal_id,omitempty"`
	BundleID           string         `json:"bundle_id,omitempty"`
	ScopeKind          string         `json:"scope_kind,omitempty"`
	ScopeID            string         `json:"scope_id,omitempty"`
	DelegationMode     string         `json:"delegation_mode,omitempty"`
	Basis              string         `json:"basis,omitempty"`
	Status             string         `json:"status,omitempty"`
	Reason             string         `json:"reason,omitempty"`
	EvidenceRefs       []string       `json:"evidence_refs,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`
}

func newRegistryHTTPClient(baseURL, internalToken string) (*registryHTTPClient, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil, fmt.Errorf("kernel registry boundary URL is required")
	}
	return &registryHTTPClient{
		baseURL:       strings.TrimRight(baseURL, "/"),
		internalToken: strings.TrimSpace(internalToken),
		httpClient:    &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func (c *registryHTTPClient) AppendChangeSet(ctx context.Context, input registryAppendChangeSetInput) error {
	body, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("marshal registry change set: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/runtime/registry/change-sets", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create registry append request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.internalToken != "" {
		req.Header.Set("X-AS-Internal-Token", c.internalToken)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("append registry change set: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("append registry change set: status %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	return nil
}

func (c *registryHTTPClient) ListRows(ctx context.Context, input registryListRowsInput) ([]registryRowRecord, error) {
	values := url.Values{}
	if strings.TrimSpace(input.Reference) != "" {
		values.Set("reference", strings.TrimSpace(input.Reference))
	}
	if strings.TrimSpace(input.SeedID) != "" {
		values.Set("seed_id", strings.TrimSpace(input.SeedID))
	}
	if strings.TrimSpace(input.RealizationID) != "" {
		values.Set("realization_id", strings.TrimSpace(input.RealizationID))
	}
	if input.AfterRowID > 0 {
		values.Set("after", fmt.Sprintf("%d", input.AfterRowID))
	}
	if input.Limit > 0 {
		values.Set("limit", fmt.Sprintf("%d", input.Limit))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/runtime/registry/rows?"+values.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("create registry rows request: %w", err)
	}
	if c.internalToken != "" {
		req.Header.Set("X-AS-Internal-Token", c.internalToken)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list registry rows: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("list registry rows: status %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	var rows []registryRowRecord
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, fmt.Errorf("decode registry rows: %w", err)
	}
	return rows, nil
}

func (c *registryHTTPClient) UpsertAuthorityBundle(ctx context.Context, input authorityBundleUpsertInput) error {
	return c.postJSON(ctx, "/v1/runtime/authority/bundles", input)
}

func (c *registryHTTPClient) CreateAuthorityGrant(ctx context.Context, input authorityGrantCreateInput) error {
	return c.postJSON(ctx, "/v1/runtime/authority/grants", input)
}

func (c *registryHTTPClient) MaterializeAuthorityState(ctx context.Context) error {
	return c.postJSON(ctx, "/v1/runtime/authority/materialize", map[string]any{})
}

func (c *registryHTTPClient) postJSON(ctx context.Context, path string, input any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create %s request: %w", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.internalToken != "" {
		req.Header.Set("X-AS-Internal-Token", c.internalToken)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("post %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("post %s: status %d: %s", path, resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	return nil
}
