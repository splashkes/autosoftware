package execution

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWaitForHealthyRejects404(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()

	if err := WaitForHealthy(ctx, addr); err == nil {
		t.Fatal("expected WaitForHealthy to reject a 404-only server")
	}
}

func TestWaitForHealthyAccepts200(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := WaitForHealthy(ctx, addr); err != nil {
		t.Fatalf("expected WaitForHealthy to accept a 200 server, got %v", err)
	}
}
