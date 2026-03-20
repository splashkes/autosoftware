package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegistryHTTPClientAppendChangeSetUsesSnakeCaseJSON(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/runtime/registry/change-sets" {
			t.Fatalf("unexpected path %q", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client, err := newRegistryHTTPClient(server.URL, "")
	if err != nil {
		t.Fatalf("newRegistryHTTPClient: %v", err)
	}
	err = client.AppendChangeSet(context.Background(), registryAppendChangeSetInput{
		ChangeSetID:    "chg_123",
		Reference:      "0007-Flowershow/a-firstbloom",
		SeedID:         "0007-Flowershow",
		RealizationID:  "a-firstbloom",
		IdempotencyKey: "idem_123",
		AcceptedBy:     "flowershow-app",
		Metadata: map[string]any{
			"source": "test",
		},
		Rows: []registryAppendRowInput{
			{
				RowType:  "object.create",
				ObjectID: "org_123",
				Payload: map[string]any{
					"id":   "org_123",
					"name": "Uxbridge Horticultural Society",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("AppendChangeSet: %v", err)
	}

	if _, ok := captured["ChangeSetID"]; ok {
		t.Fatalf("request body used Go field names instead of snake_case: %#v", captured)
	}
	if got := captured["change_set_id"]; got != "chg_123" {
		t.Fatalf("change_set_id = %#v, want chg_123", got)
	}
	if got := captured["reference"]; got != "0007-Flowershow/a-firstbloom" {
		t.Fatalf("reference = %#v", got)
	}
	if got := captured["seed_id"]; got != "0007-Flowershow" {
		t.Fatalf("seed_id = %#v", got)
	}
	rows, ok := captured["rows"].([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("rows = %#v", captured["rows"])
	}
	row, ok := rows[0].(map[string]any)
	if !ok {
		t.Fatalf("row[0] = %#v", rows[0])
	}
	if got := row["row_type"]; got != "object.create" {
		t.Fatalf("row_type = %#v", got)
	}
	if got := row["object_id"]; got != "org_123" {
		t.Fatalf("object_id = %#v", got)
	}
}
