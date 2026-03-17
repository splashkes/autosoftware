package jsontransport

import (
	"encoding/json"
	"net/http"

	"as/kernel/internal/http/server"
)

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, err error) {
	server.WriteJSONError(w, nil, status, err)
}
