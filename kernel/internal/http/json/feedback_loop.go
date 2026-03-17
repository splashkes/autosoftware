package jsontransport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	feedbackloop "as/kernel/internal/feedback_loop"
	"as/kernel/internal/http/server"
)

type IncidentRecorder interface {
	RecordIncident(context.Context, feedbackloop.ClientIncident) error
}

type IncidentIngestHandler struct {
	Recorder     IncidentRecorder
	MaxBodyBytes int64
	Now          func() time.Time
}

func NewIncidentIngestHandler(recorder IncidentRecorder) *IncidentIngestHandler {
	return &IncidentIngestHandler{
		Recorder:     recorder,
		MaxBodyBytes: 64 << 10,
		Now:          time.Now,
	}
}

func (h *IncidentIngestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		server.WriteJSONError(w, r, http.StatusMethodNotAllowed, server.NewHTTPError(http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed"))
		return
	}

	if h.Recorder == nil {
		server.WriteJSONError(w, r, http.StatusServiceUnavailable, server.ServiceUnavailable("incident recorder unavailable"))
		return
	}

	bodyLimit := h.MaxBodyBytes
	if bodyLimit <= 0 {
		bodyLimit = 64 << 10
	}

	defer r.Body.Close()

	var incident feedbackloop.ClientIncident
	decoder := json.NewDecoder(io.LimitReader(r.Body, bodyLimit))
	if err := decoder.Decode(&incident); err != nil {
		server.WriteJSONError(w, r, http.StatusBadRequest, server.BadRequest("invalid incident payload"))
		return
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF && !errors.Is(err, io.EOF) {
		server.WriteJSONError(w, r, http.StatusBadRequest, server.BadRequest("incident payload must contain one JSON value"))
		return
	}

	if incident.ID == "" {
		incident.ID = feedbackloop.NewIncidentID()
	}
	if incident.CreatedAt.IsZero() {
		incident.CreatedAt = h.Now().UTC()
	}
	if incident.Kind == "" {
		incident.Kind = "client.event"
	}
	if incident.Severity == "" {
		incident.Severity = feedbackloop.SeverityError
	}
	if incident.Message == "" {
		incident.Message = incident.Kind
	}

	incident.Request = mergeRequestContext(
		incident.Request,
		server.RequestMetadataFromContext(r.Context()),
	)

	if err := h.Recorder.RecordIncident(r.Context(), incident); err != nil {
		server.WriteJSONError(w, r, http.StatusInternalServerError, server.Internal("failed to record incident"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":      "accepted",
		"incident_id": incident.ID,
	})
}

func mergeRequestContext(
	request feedbackloop.RequestContext,
	metadata server.RequestMetadata,
) feedbackloop.RequestContext {
	if request.RequestID == "" {
		request.RequestID = metadata.RequestID
	}
	if request.SessionID == "" {
		request.SessionID = metadata.SessionID
	}
	if request.SeedID == "" {
		request.SeedID = metadata.SeedID
	}
	if request.RealizationID == "" {
		request.RealizationID = metadata.RealizationID
	}
	if request.Route == "" {
		request.Route = metadata.Route
	}
	if request.Method == "" {
		request.Method = metadata.Method
	}
	if request.UserAgent == "" {
		request.UserAgent = metadata.UserAgent
	}

	return request
}
