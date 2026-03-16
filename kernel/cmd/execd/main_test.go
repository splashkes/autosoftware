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
