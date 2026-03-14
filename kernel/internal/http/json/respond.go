package jsontransport

import (
	"encoding/json"
	"log"
	"net/http"
)

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, err error) {
	log.Printf("http error %d: %v", status, err)
	respondJSON(w, status, map[string]string{"error": safeErrorMessage(status, err)})
}

func safeErrorMessage(status int, err error) string {
	switch {
	case status == http.StatusBadRequest:
		return err.Error()
	case status == http.StatusNotFound:
		return "not found"
	default:
		return "internal error"
	}
}
