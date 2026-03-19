package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const flowershowContractSelf = "/v1/contracts/0007-Flowershow/a-firstbloom"

type apiErrorEnvelope struct {
	Error apiErrorBody `json:"error"`
}

type apiErrorBody struct {
	Code        string          `json:"code"`
	Message     string          `json:"message"`
	Hint        string          `json:"hint,omitempty"`
	RequestID   string          `json:"request_id"`
	AuthMode    string          `json:"auth_mode,omitempty"`
	ContractRef string          `json:"contract_ref,omitempty"`
	FieldErrors []apiFieldError `json:"field_errors,omitempty"`
}

type apiFieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (a *app) requestAuthMode(r *http.Request) string {
	switch {
	case a.isServiceToken(r):
		return "service_token"
	case func() bool {
		_, ok := a.currentAgentToken(r)
		return ok
	}():
		return "agent_token"
	case func() bool {
		_, ok := a.currentUser(r)
		return ok
	}():
		return "session"
	default:
		return "anonymous"
	}
}

func requestID(r *http.Request) string {
	for _, key := range []string{"X-Request-ID", "X-Request-Id"} {
		if value := strings.TrimSpace(r.Header.Get(key)); value != "" {
			return value
		}
	}
	return fmt.Sprintf("req-%d", time.Now().UTC().UnixNano())
}

func (a *app) writeAPIError(w http.ResponseWriter, r *http.Request, status int, code, message, hint string, fieldErrors []apiFieldError) {
	authMode := a.requestAuthMode(r)
	body := apiErrorBody{
		Code:        code,
		Message:     message,
		RequestID:   requestID(r),
		AuthMode:    authMode,
		ContractRef: flowershowContractSelf,
	}
	if authMode != "anonymous" {
		body.Hint = hint
		body.FieldErrors = fieldErrors
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Request-ID", body.RequestID)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiErrorEnvelope{Error: body})
}
