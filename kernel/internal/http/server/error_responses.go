package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"as/kernel/internal/interactions"
)

type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code              string         `json:"code"`
	Message           string         `json:"message"`
	RequestID         string         `json:"request_id,omitempty"`
	RetryAfterSeconds int            `json:"retry_after_seconds,omitempty"`
	Details           map[string]any `json:"details,omitempty"`
}

type HTTPError struct {
	status     int
	code       string
	message    string
	details    map[string]any
	retryAfter time.Duration
	cause      error
}

func (e *HTTPError) Error() string {
	if e == nil {
		return ""
	}
	if e.cause != nil {
		return e.cause.Error()
	}
	return e.message
}

func (e *HTTPError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func NewHTTPError(status int, code, message string) *HTTPError {
	return &HTTPError{
		status:  status,
		code:    strings.TrimSpace(code),
		message: strings.TrimSpace(message),
	}
}

func NewHTTPErrorWithCause(status int, code, message string, cause error) *HTTPError {
	item := NewHTTPError(status, code, message)
	item.cause = cause
	return item
}

func BadRequest(message string) error {
	return NewHTTPError(http.StatusBadRequest, "bad_request", message)
}

func Unauthorized(message string) error {
	return NewHTTPError(http.StatusUnauthorized, "unauthorized", message)
}

func Forbidden(message string) error {
	return NewHTTPError(http.StatusForbidden, "forbidden", message)
}

func NotFound(message string) error {
	return NewHTTPError(http.StatusNotFound, "not_found", message)
}

func Conflict(message string) error {
	return NewHTTPError(http.StatusConflict, "conflict", message)
}

func RateLimited(message string, retryAfter time.Duration, details map[string]any) error {
	item := NewHTTPError(http.StatusTooManyRequests, "rate_limited", message)
	item.retryAfter = retryAfter
	item.details = cloneDetails(details)
	return item
}

func ServiceUnavailable(message string) error {
	return NewHTTPError(http.StatusServiceUnavailable, "service_unavailable", message)
}

func Internal(message string) error {
	return NewHTTPError(http.StatusInternalServerError, "internal", message)
}

func AuthStateFromContext(ctx context.Context) string {
	session, ok := SessionFromContext(ctx)
	if ok && strings.TrimSpace(session.SessionID) != "" {
		return "session"
	}
	return "anonymous"
}

func WriteJSONError(w http.ResponseWriter, r *http.Request, status int, err error) {
	spec := buildErrorBody(status, err)
	if spec.RequestID == "" {
		spec.RequestID = strings.TrimSpace(w.Header().Get(HeaderRequestID))
	}
	if r != nil {
		if spec.RequestID == "" {
			spec.RequestID = RequestMetadataFromContext(r.Context()).RequestID
		}
		if shouldIncludeAuthState(spec.Code) {
			spec.Details = withAuthState(spec.Details, AuthStateFromContext(r.Context()))
		}
	}

	if spec.RetryAfterSeconds > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(spec.RetryAfterSeconds))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resolveHTTPStatus(status, err))
	_ = json.NewEncoder(w).Encode(ErrorEnvelope{Error: spec})
	log.Printf("http error %d: %v", resolveHTTPStatus(status, err), err)
}

func WriteTextError(w http.ResponseWriter, r *http.Request, status int, err error) {
	spec := buildErrorBody(status, err)
	http.Error(w, spec.Message, resolveHTTPStatus(status, err))
	log.Printf("http error %d: %v", resolveHTTPStatus(status, err), err)
}

func buildErrorBody(status int, err error) ErrorBody {
	resolvedStatus := resolveHTTPStatus(status, err)

	if err != nil {
		var httpErr *HTTPError
		if errors.As(err, &httpErr) {
			return bodyFromHTTPError(httpErr)
		}

		var rateLimitErr *interactions.RateLimitError
		if errors.As(err, &rateLimitErr) {
			return ErrorBody{
				Code:              "rate_limited",
				Message:           firstNonEmpty(strings.TrimSpace(rateLimitErr.Message), "Too many requests. Try again shortly."),
				RetryAfterSeconds: durationSeconds(rateLimitErr.RetryAfter),
				Details: withNonEmptyDetails(map[string]any{
					"namespace":   strings.TrimSpace(rateLimitErr.Namespace),
					"subject_key": strings.TrimSpace(rateLimitErr.SubjectKey),
				}),
			}
		}
	}

	return ErrorBody{
		Code:    statusCodeName(resolvedStatus),
		Message: safeErrorMessage(resolvedStatus, err),
	}
}

func bodyFromHTTPError(err *HTTPError) ErrorBody {
	if err == nil {
		return ErrorBody{Code: "internal", Message: "internal error"}
	}
	return ErrorBody{
		Code:              firstNonEmpty(strings.TrimSpace(err.code), statusCodeName(err.status)),
		Message:           firstNonEmpty(strings.TrimSpace(err.message), safeErrorMessage(err.status, err)),
		RetryAfterSeconds: durationSeconds(err.retryAfter),
		Details:           cloneDetails(err.details),
	}
}

func resolveHTTPStatus(status int, err error) int {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) && httpErr != nil && httpErr.status > 0 {
		return httpErr.status
	}

	switch {
	case errors.Is(err, interactions.ErrRateLimited):
		return http.StatusTooManyRequests
	case errors.Is(err, interactions.ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, interactions.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, interactions.ErrConflict):
		return http.StatusConflict
	case errors.Is(err, interactions.ErrNotFound):
		return http.StatusNotFound
	case status > 0:
		return status
	default:
		return http.StatusInternalServerError
	}
}

func safeErrorMessage(status int, err error) string {
	if err != nil {
		var httpErr *HTTPError
		if errors.As(err, &httpErr) && httpErr != nil && strings.TrimSpace(httpErr.message) != "" {
			return strings.TrimSpace(httpErr.message)
		}
		var rateLimitErr *interactions.RateLimitError
		if errors.As(err, &rateLimitErr) && strings.TrimSpace(rateLimitErr.Message) != "" {
			return strings.TrimSpace(rateLimitErr.Message)
		}
	}

	switch status {
	case http.StatusBadRequest:
		if safe := userVisibleBadRequest(err); safe != "" {
			return safe
		}
		return "invalid request"
	case http.StatusUnauthorized:
		return "authentication required"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not found"
	case http.StatusConflict:
		return "request conflicts with existing state"
	case http.StatusTooManyRequests:
		return "Too many requests. Try again shortly."
	case http.StatusServiceUnavailable:
		return "service unavailable"
	default:
		return "internal error"
	}
}

func userVisibleBadRequest(err error) string {
	if err == nil {
		return ""
	}

	message := strings.TrimSpace(err.Error())
	switch {
	case message == "":
		return ""
	case len(message) > 200:
		return ""
	case strings.Contains(message, "\n"):
		return ""
	case strings.Contains(message, "/"):
		return ""
	case strings.Contains(message, "\\"):
		return ""
	case strings.Contains(message, ": "):
		return ""
	case strings.Contains(strings.ToLower(message), "select "):
		return ""
	case strings.Contains(strings.ToLower(message), "insert "):
		return ""
	case strings.Contains(strings.ToLower(message), "update "):
		return ""
	case strings.Contains(strings.ToLower(message), "delete "):
		return ""
	default:
		return message
	}
}

func statusCodeName(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusMethodNotAllowed:
		return "method_not_allowed"
	case http.StatusConflict:
		return "conflict"
	case http.StatusRequestEntityTooLarge:
		return "payload_too_large"
	case http.StatusTooManyRequests:
		return "rate_limited"
	case http.StatusServiceUnavailable:
		return "service_unavailable"
	default:
		return "internal"
	}
}

func shouldIncludeAuthState(code string) bool {
	switch strings.TrimSpace(code) {
	case "unauthorized", "forbidden", "rate_limited":
		return true
	default:
		return false
	}
}

func withAuthState(details map[string]any, authState string) map[string]any {
	if strings.TrimSpace(authState) == "" {
		return details
	}
	out := cloneDetails(details)
	if out == nil {
		out = make(map[string]any, 1)
	}
	if _, exists := out["auth_state"]; !exists {
		out["auth_state"] = authState
	}
	return out
}

func durationSeconds(value time.Duration) int {
	if value <= 0 {
		return 0
	}
	seconds := int((value + time.Second - 1) / time.Second)
	if seconds < 1 {
		return 1
	}
	return seconds
}

func cloneDetails(details map[string]any) map[string]any {
	if len(details) == 0 {
		return nil
	}
	out := make(map[string]any, len(details))
	for key, value := range details {
		out[key] = value
	}
	return out
}

func withNonEmptyDetails(details map[string]any) map[string]any {
	if len(details) == 0 {
		return nil
	}
	out := make(map[string]any, len(details))
	for key, value := range details {
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				out[key] = typed
			}
		case nil:
		default:
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func PrefersJSONErrors(r *http.Request) bool {
	if r == nil {
		return false
	}

	path := strings.TrimSpace(r.URL.Path)
	switch {
	case strings.HasPrefix(path, "/v1/"),
		strings.HasPrefix(path, "/boot/"),
		strings.HasPrefix(path, "/feedback/"):
		return true
	}

	accept := strings.ToLower(strings.TrimSpace(r.Header.Get("Accept")))
	return strings.Contains(accept, "application/json")
}
