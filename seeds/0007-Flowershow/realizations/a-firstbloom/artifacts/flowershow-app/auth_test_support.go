package main

import (
	"encoding/json"
	"net/http"
)

func (a *app) handleTestSessionCreate(w http.ResponseWriter, r *http.Request) {
	if !a.allowTestAuth || !a.isServiceToken(r) {
		http.NotFound(w, r)
		return
	}
	var payload struct {
		User UserIdentity `json:"user"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if payload.User.CognitoSub == "" && payload.User.Email == "" {
		http.Error(w, "user identity required", http.StatusBadRequest)
		return
	}
	if err := a.setUserSession(w, r, payload.User); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"status": "ok",
		"user":   payload.User,
	})
}
