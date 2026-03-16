package main

import "testing"

func TestHTTPBaseURLFromAddr(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want string
	}{
		{name: "empty", addr: "", want: ""},
		{name: "host and port", addr: "127.0.0.1:8093", want: "http://127.0.0.1:8093"},
		{name: "bare port", addr: ":8093", want: "http://127.0.0.1:8093"},
		{name: "existing URL", addr: "https://registry.internal", want: "https://registry.internal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := httpBaseURLFromAddr(tt.addr); got != tt.want {
				t.Fatalf("httpBaseURLFromAddr(%q) = %q, want %q", tt.addr, got, tt.want)
			}
		})
	}
}

func TestCapabilityBaseURLPrefersExplicitURL(t *testing.T) {
	t.Setenv("AS_EXECD_REGISTRY_BASE_URL", "http://as-registryd.autosoftware.svc.cluster.local")
	t.Setenv("AS_REGISTRYD_ADDR", ":8093")

	got := capabilityBaseURL("AS_EXECD_REGISTRY_BASE_URL", "AS_REGISTRYD_ADDR", "127.0.0.1:8093")
	want := "http://as-registryd.autosoftware.svc.cluster.local"
	if got != want {
		t.Fatalf("capabilityBaseURL(explicit) = %q, want %q", got, want)
	}
}

func TestCapabilityBaseURLFallsBackToAddress(t *testing.T) {
	t.Setenv("AS_EXECD_REGISTRY_BASE_URL", "")
	t.Setenv("AS_REGISTRYD_ADDR", ":8093")

	got := capabilityBaseURL("AS_EXECD_REGISTRY_BASE_URL", "AS_REGISTRYD_ADDR", "127.0.0.1:8093")
	want := "http://127.0.0.1:8093"
	if got != want {
		t.Fatalf("capabilityBaseURL(addr) = %q, want %q", got, want)
	}
}
