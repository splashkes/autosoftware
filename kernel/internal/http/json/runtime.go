package jsontransport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"as/kernel/internal/http/server"
	"as/kernel/internal/interactions"
)

type AppliedMigrationsReader func(context.Context) ([]string, error)

type RuntimeAPI struct {
	Service           *interactions.RuntimeService
	AppliedMigrations AppliedMigrationsReader
}

const maxJSONBodyBytes int64 = 1 << 20

func NewRuntimeAPI(service *interactions.RuntimeService, applied AppliedMigrationsReader) *RuntimeAPI {
	return &RuntimeAPI{
		Service:           service,
		AppliedMigrations: applied,
	}
}

func (api *RuntimeAPI) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", api.handleHealthz)
	mux.HandleFunc("GET /v1/runtime/health", api.handleRuntimeHealth)
	mux.HandleFunc("POST /v1/runtime/principals", api.handleCreatePrincipal)
	mux.HandleFunc("POST /v1/runtime/principal-identifiers", api.handleCreatePrincipalIdentifier)
	mux.HandleFunc("POST /v1/runtime/auth-identities", api.handleBindAuthIdentity)
	mux.HandleFunc("GET /v1/runtime/auth-identities/resolve", api.handleResolveAuthIdentity)
	mux.HandleFunc("POST /v1/runtime/sessions", api.handleCreateSession)
	mux.HandleFunc("GET /v1/runtime/sessions/{session_id}", api.handleGetSession)
	mux.HandleFunc("POST /v1/runtime/auth-challenges", api.handleCreateAuthChallenge)
	mux.HandleFunc("POST /v1/runtime/auth-challenges/{challenge_id}/consume", api.handleConsumeAuthChallenge)
	mux.HandleFunc("POST /v1/runtime/authority/bundles", api.handleUpsertAuthorityBundle)
	mux.HandleFunc("POST /v1/runtime/authority/grants", api.handleCreateAuthorityGrant)
	mux.HandleFunc("GET /v1/runtime/authority/ledger/principals/{principal_id}", api.handleListAuthorityLedgerByPrincipal)
	mux.HandleFunc("GET /v1/runtime/authority/effective/principals/{principal_id}", api.handleGetEffectiveAuthorityByPrincipal)
	mux.HandleFunc("POST /v1/runtime/handles", api.handleAssignHandle)
	mux.HandleFunc("GET /v1/runtime/handles/{namespace}/{handle}", api.handleResolveHandle)
	mux.HandleFunc("POST /v1/runtime/access-links", api.handleCreateAccessLink)
	mux.HandleFunc("POST /v1/runtime/access-links/consume", api.handleConsumeAccessLink)
	mux.HandleFunc("POST /v1/runtime/publications", api.handleUpsertPublication)
	mux.HandleFunc("POST /v1/runtime/state-transitions", api.handleRecordStateTransition)
	mux.HandleFunc("POST /v1/runtime/activity-events", api.handleRecordActivityEvent)
	mux.HandleFunc("POST /v1/runtime/jobs", api.handleEnqueueJob)
	mux.HandleFunc("GET /v1/runtime/jobs/{job_id}", api.handleGetJob)
	mux.HandleFunc("POST /v1/runtime/jobs/claim", api.handleClaimJobs)
	mux.HandleFunc("POST /v1/runtime/jobs/{job_id}/complete", api.handleCompleteJob)
	mux.HandleFunc("POST /v1/runtime/jobs/{job_id}/fail", api.handleFailJob)
	mux.HandleFunc("POST /v1/runtime/outbox-messages", api.handleEnqueueOutbox)
	mux.HandleFunc("POST /v1/runtime/threads", api.handleCreateThread)
	mux.HandleFunc("POST /v1/runtime/thread-participants", api.handleAddThreadParticipant)
	mux.HandleFunc("POST /v1/runtime/messages", api.handlePostMessage)
	mux.HandleFunc("GET /v1/runtime/threads/{thread_id}/messages", api.handleListMessages)
	mux.HandleFunc("POST /v1/runtime/search-documents", api.handleUpsertSearchDocument)
	mux.HandleFunc("GET /v1/runtime/search-documents", api.handleSearchDocuments)
	mux.HandleFunc("POST /v1/runtime/guard-decisions", api.handleRecordGuardDecision)
	mux.HandleFunc("POST /v1/runtime/risk-events", api.handleRecordRiskEvent)
}

func (api *RuntimeAPI) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (api *RuntimeAPI) handleRuntimeHealth(w http.ResponseWriter, r *http.Request) {
	if api.Service == nil {
		respondError(w, http.StatusServiceUnavailable, server.ServiceUnavailable("runtime service unavailable"))
		return
	}
	if err := api.Service.Ping(r.Context()); err != nil {
		respondError(w, http.StatusServiceUnavailable, server.ServiceUnavailable("runtime service unavailable"))
		return
	}

	payload := map[string]interface{}{
		"status":     "ok",
		"runtime_db": "connected",
	}
	if api.AppliedMigrations != nil {
		migrations, err := api.AppliedMigrations(r.Context())
		if err != nil {
			respondError(w, http.StatusInternalServerError, err)
			return
		}
		payload["applied_migrations"] = migrations
	}
	if session, ok := server.SessionFromContext(r.Context()); ok {
		payload["resolved_session"] = session
	}
	respondJSON(w, http.StatusOK, payload)
}

func (api *RuntimeAPI) handleCreatePrincipal(w http.ResponseWriter, r *http.Request) {
	var input interactions.CreatePrincipalInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.Service.CreatePrincipal(r.Context(), input)
	writeRuntimeResult(w, item, err)
}

func (api *RuntimeAPI) handleCreatePrincipalIdentifier(w http.ResponseWriter, r *http.Request) {
	var input interactions.CreatePrincipalIdentifierInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.Service.CreatePrincipalIdentifier(r.Context(), input)
	writeRuntimeResult(w, item, err)
}

func (api *RuntimeAPI) handleBindAuthIdentity(w http.ResponseWriter, r *http.Request) {
	var input interactions.BindAuthIdentityInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.Service.BindAuthIdentity(r.Context(), input)
	writeRuntimeResult(w, item, err)
}

func (api *RuntimeAPI) handleResolveAuthIdentity(w http.ResponseWriter, r *http.Request) {
	item, err := api.Service.ResolveAuthIdentity(
		r.Context(),
		r.URL.Query().Get("provider_id"),
		r.URL.Query().Get("provider_subject"),
	)
	writeRuntimeResult(w, item, err)
}

func (api *RuntimeAPI) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var input interactions.CreateSessionInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.Service.CreateSession(r.Context(), input)
	writeRuntimeResult(w, item, err)
}

func (api *RuntimeAPI) handleGetSession(w http.ResponseWriter, r *http.Request) {
	item, err := api.Service.GetSession(r.Context(), r.PathValue("session_id"))
	writeRuntimeResult(w, item, err)
}

func (api *RuntimeAPI) handleCreateAuthChallenge(w http.ResponseWriter, r *http.Request) {
	var input interactions.CreateAuthChallengeInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.Service.CreateAuthChallenge(r.Context(), input)
	writeRuntimeResult(w, item, err)
}

func (api *RuntimeAPI) handleConsumeAuthChallenge(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Verifier string `json:"verifier"`
	}
	if !decodeJSON(w, r, &payload) {
		return
	}
	item, err := api.Service.ConsumeAuthChallenge(r.Context(), r.PathValue("challenge_id"), payload.Verifier)
	writeRuntimeResult(w, item, err)
}

func (api *RuntimeAPI) handleUpsertAuthorityBundle(w http.ResponseWriter, r *http.Request) {
	var input interactions.UpsertAuthorityBundleInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.Service.UpsertAuthorityBundle(r.Context(), input)
	writeRuntimeResult(w, item, err)
}

func (api *RuntimeAPI) handleCreateAuthorityGrant(w http.ResponseWriter, r *http.Request) {
	var input interactions.CreateAuthorityGrantInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.Service.CreateAuthorityGrant(r.Context(), input)
	writeRuntimeResult(w, item, err)
}

func (api *RuntimeAPI) handleListAuthorityLedgerByPrincipal(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := api.Service.ListAuthorityLedgerByPrincipal(r.Context(), r.PathValue("principal_id"), limit)
	writeRuntimeResult(w, items, err)
}

func (api *RuntimeAPI) handleGetEffectiveAuthorityByPrincipal(w http.ResponseWriter, r *http.Request) {
	item, err := api.Service.GetEffectiveAuthorityByPrincipal(r.Context(), r.PathValue("principal_id"))
	writeRuntimeResult(w, item, err)
}

func (api *RuntimeAPI) handleAssignHandle(w http.ResponseWriter, r *http.Request) {
	var input interactions.AssignHandleInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.Service.AssignHandle(r.Context(), input)
	writeRuntimeResult(w, item, err)
}

func (api *RuntimeAPI) handleResolveHandle(w http.ResponseWriter, r *http.Request) {
	item, err := api.Service.ResolveHandle(r.Context(), r.PathValue("namespace"), r.PathValue("handle"))
	writeRuntimeResult(w, item, err)
}

func (api *RuntimeAPI) handleCreateAccessLink(w http.ResponseWriter, r *http.Request) {
	var input interactions.CreateAccessLinkInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.Service.CreateAccessLink(r.Context(), input)
	writeRuntimeResult(w, item, err)
}

func (api *RuntimeAPI) handleConsumeAccessLink(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Token string `json:"token"`
	}
	if !decodeJSON(w, r, &payload) {
		return
	}
	item, err := api.Service.ConsumeAccessLink(r.Context(), payload.Token)
	writeRuntimeResult(w, item, err)
}

func (api *RuntimeAPI) handleUpsertPublication(w http.ResponseWriter, r *http.Request) {
	var input interactions.UpsertPublicationInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.Service.UpsertPublication(r.Context(), input)
	writeRuntimeResult(w, item, err)
}

func (api *RuntimeAPI) handleRecordStateTransition(w http.ResponseWriter, r *http.Request) {
	var input interactions.RecordStateTransitionInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.Service.RecordStateTransition(r.Context(), input)
	writeRuntimeResult(w, item, err)
}

func (api *RuntimeAPI) handleRecordActivityEvent(w http.ResponseWriter, r *http.Request) {
	var input interactions.RecordActivityEventInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.Service.RecordActivityEvent(r.Context(), input)
	writeRuntimeResult(w, item, err)
}

func (api *RuntimeAPI) handleEnqueueJob(w http.ResponseWriter, r *http.Request) {
	var input interactions.EnqueueJobInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.Service.EnqueueJob(r.Context(), input)
	writeRuntimeResult(w, item, err)
}

func (api *RuntimeAPI) handleGetJob(w http.ResponseWriter, r *http.Request) {
	item, err := api.Service.GetJob(r.Context(), r.PathValue("job_id"))
	writeRuntimeResult(w, item, err)
}

func (api *RuntimeAPI) handleClaimJobs(w http.ResponseWriter, r *http.Request) {
	var input interactions.JobClaimInput
	if !decodeJSON(w, r, &input) {
		return
	}
	items, err := api.Service.ClaimJobs(r.Context(), input)
	writeRuntimeResult(w, items, err)
}

func (api *RuntimeAPI) handleCompleteJob(w http.ResponseWriter, r *http.Request) {
	var input interactions.JobCompleteInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.Service.CompleteJob(r.Context(), r.PathValue("job_id"), input)
	writeRuntimeResult(w, item, err)
}

func (api *RuntimeAPI) handleFailJob(w http.ResponseWriter, r *http.Request) {
	var input interactions.JobFailInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.Service.FailJob(r.Context(), r.PathValue("job_id"), input)
	writeRuntimeResult(w, item, err)
}

func (api *RuntimeAPI) handleEnqueueOutbox(w http.ResponseWriter, r *http.Request) {
	var input interactions.EnqueueOutboxMessageInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.Service.EnqueueOutboxMessage(r.Context(), input)
	writeRuntimeResult(w, item, err)
}

func (api *RuntimeAPI) handleCreateThread(w http.ResponseWriter, r *http.Request) {
	var input interactions.CreateThreadInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.Service.CreateThread(r.Context(), input)
	writeRuntimeResult(w, item, err)
}

func (api *RuntimeAPI) handleAddThreadParticipant(w http.ResponseWriter, r *http.Request) {
	var input interactions.AddThreadParticipantInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.Service.AddThreadParticipant(r.Context(), input)
	writeRuntimeResult(w, item, err)
}

func (api *RuntimeAPI) handlePostMessage(w http.ResponseWriter, r *http.Request) {
	var input interactions.PostMessageInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.Service.PostMessage(r.Context(), input)
	writeRuntimeResult(w, item, err)
}

func (api *RuntimeAPI) handleListMessages(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := api.Service.ListMessages(r.Context(), r.PathValue("thread_id"), limit)
	writeRuntimeResult(w, items, err)
}

func (api *RuntimeAPI) handleUpsertSearchDocument(w http.ResponseWriter, r *http.Request) {
	var input interactions.UpsertSearchDocumentInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.Service.UpsertSearchDocument(r.Context(), input)
	writeRuntimeResult(w, item, err)
}

func (api *RuntimeAPI) handleSearchDocuments(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := api.Service.SearchDocuments(r.Context(), interactions.SearchDocumentsInput{
		Scope: r.URL.Query().Get("scope"),
		Query: r.URL.Query().Get("q"),
		Limit: limit,
	})
	writeRuntimeResult(w, items, err)
}

func (api *RuntimeAPI) handleRecordGuardDecision(w http.ResponseWriter, r *http.Request) {
	var input interactions.RecordGuardDecisionInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.Service.RecordGuardDecision(r.Context(), input)
	writeRuntimeResult(w, item, err)
}

func (api *RuntimeAPI) handleRecordRiskEvent(w http.ResponseWriter, r *http.Request) {
	var input interactions.RecordRiskEventInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.Service.RecordRiskEvent(r.Context(), input)
	writeRuntimeResult(w, item, err)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target interface{}) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			respondError(w, http.StatusRequestEntityTooLarge, server.NewHTTPError(http.StatusRequestEntityTooLarge, "payload_too_large", "request body is too large"))
			return false
		}
		respondError(w, http.StatusBadRequest, err)
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != nil && !errors.Is(err, io.EOF) {
		respondError(w, http.StatusBadRequest, errors.New("request body must contain one JSON object"))
		return false
	}
	return true
}

func writeRuntimeResult(w http.ResponseWriter, payload interface{}, err error) {
	switch {
	case err == nil:
		respondJSON(w, http.StatusOK, payload)
	case errors.Is(err, interactions.ErrNotFound):
		respondError(w, http.StatusNotFound, err)
	case errors.Is(err, interactions.ErrConflict):
		respondError(w, http.StatusConflict, err)
	case errors.Is(err, interactions.ErrUnauthorized):
		respondError(w, http.StatusUnauthorized, err)
	case errors.Is(err, interactions.ErrForbidden):
		respondError(w, http.StatusForbidden, err)
	case errors.Is(err, interactions.ErrRateLimited):
		respondError(w, http.StatusTooManyRequests, err)
	case strings.Contains(strings.ToLower(err.Error()), "runtime database is not configured"):
		respondError(w, http.StatusServiceUnavailable, err)
	default:
		respondError(w, http.StatusBadRequest, err)
	}
}
