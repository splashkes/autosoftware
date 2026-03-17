package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"as/kernel/internal/interactions"
)

type RateLimitEnforcer interface {
	EnforceRateLimit(context.Context, interactions.EnforceRateLimitInput) (interactions.RateLimitDecision, error)
}

type RateLimitOptions struct {
	Enabled             bool
	Window              time.Duration
	BlockDuration       time.Duration
	AnonymousReadLimit  int64
	AnonymousWriteLimit int64
	SessionReadLimit    int64
	SessionWriteLimit   int64
	InternalLimit       int64
	WorkerLimit         int64
	FeedbackLimit       int64
}

type routeRateLimitPolicy struct {
	Namespace string
	Action    string
	Limit     int64
	Window    time.Duration
	Block     time.Duration
}

func RateLimitMiddleware(enforcer RateLimitEnforcer, options RateLimitOptions, next http.Handler) http.Handler {
	if enforcer == nil || !options.Enabled {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		policy, ok := rateLimitPolicyForRequest(r, options)
		if !ok || policy.Limit <= 0 {
			next.ServeHTTP(w, r)
			return
		}

		requestMeta := RequestMetadataFromContext(r.Context())
		subjectKey, authState, principalID := rateLimitSubjectKey(r)
		decision, err := enforcer.EnforceRateLimit(r.Context(), interactions.EnforceRateLimitInput{
			Namespace:     policy.Namespace,
			SubjectKey:    subjectKey,
			Action:        policy.Action,
			Limit:         policy.Limit,
			Window:        policy.Window,
			BlockDuration: policy.Block,
			RequestID:     requestMeta.RequestID,
			SessionID:     requestMeta.SessionID,
			PrincipalID:   principalID,
			Metadata: map[string]any{
				"auth_state": authState,
				"method":     r.Method,
				"path":       r.URL.Path,
			},
		})
		if err == nil {
			next.ServeHTTP(w, r)
			return
		}
		if !errors.Is(err, interactions.ErrRateLimited) {
			next.ServeHTTP(w, r)
			return
		}

		WriteJSONError(w, r, http.StatusTooManyRequests, RateLimited(
			rateLimitMessage(policy.Namespace, authState),
			decision.RetryAfter,
			map[string]any{
				"auth_state": authState,
				"limit":      decision.Limit,
				"namespace":  decision.Namespace,
			},
		))
	})
}

func rateLimitPolicyForRequest(r *http.Request, options RateLimitOptions) (routeRateLimitPolicy, bool) {
	path := strings.TrimSpace(r.URL.Path)
	authenticated := AuthStateFromContext(r.Context()) == "session"
	readLimit := options.AnonymousReadLimit
	writeLimit := options.AnonymousWriteLimit
	if authenticated {
		readLimit = options.SessionReadLimit
		writeLimit = options.SessionWriteLimit
	}

	switch {
	case path == "/healthz", path == "/v1/runtime/health":
		return routeRateLimitPolicy{}, false
	case strings.HasPrefix(path, "/feedback/incidents"):
		return routeRateLimitPolicy{
			Namespace: "feedback.incidents",
			Action:    "feedback.record",
			Limit:     options.FeedbackLimit,
			Window:    options.Window,
			Block:     options.BlockDuration,
		}, true
	case strings.HasPrefix(path, "/boot/commands/"):
		return routeRateLimitPolicy{
			Namespace: "boot.commands",
			Action:    path,
			Limit:     writeLimit,
			Window:    options.Window,
			Block:     options.BlockDuration,
		}, true
	case strings.HasPrefix(path, "/boot/projections/"):
		return routeRateLimitPolicy{
			Namespace: "boot.projections",
			Action:    path,
			Limit:     readLimit,
			Window:    options.Window,
			Block:     options.BlockDuration,
		}, true
	case strings.HasPrefix(path, "/v1/runtime/jobs/claim"),
		strings.HasSuffix(path, "/complete") && strings.Contains(path, "/v1/runtime/jobs/"),
		strings.HasSuffix(path, "/fail") && strings.Contains(path, "/v1/runtime/jobs/"):
		return routeRateLimitPolicy{
			Namespace: "internal.runtime.worker",
			Action:    path,
			Limit:     options.WorkerLimit,
			Window:    options.Window,
			Block:     options.BlockDuration,
		}, true
	case strings.HasPrefix(path, "/v1/runtime/"):
		return routeRateLimitPolicy{
			Namespace: "internal.runtime",
			Action:    path,
			Limit:     options.InternalLimit,
			Window:    options.Window,
			Block:     options.BlockDuration,
		}, true
	case strings.HasPrefix(path, "/v1/commands/"):
		return routeRateLimitPolicy{
			Namespace: "api.commands",
			Action:    path,
			Limit:     writeLimit,
			Window:    options.Window,
			Block:     options.BlockDuration,
		}, true
	case strings.HasPrefix(path, "/v1/projections/"):
		return routeRateLimitPolicy{
			Namespace: "api.projections",
			Action:    path,
			Limit:     readLimit,
			Window:    options.Window,
			Block:     options.BlockDuration,
		}, true
	case strings.HasPrefix(path, "/v1/contracts"):
		return routeRateLimitPolicy{
			Namespace: "api.contracts",
			Action:    path,
			Limit:     readLimit,
			Window:    options.Window,
			Block:     options.BlockDuration,
		}, true
	case strings.HasPrefix(path, "/v1/registry/"):
		return routeRateLimitPolicy{
			Namespace: "api.registry",
			Action:    path,
			Limit:     readLimit,
			Window:    options.Window,
			Block:     options.BlockDuration,
		}, true
	default:
		return routeRateLimitPolicy{}, false
	}
}

func rateLimitSubjectKey(r *http.Request) (subjectKey, authState, principalID string) {
	requestMeta := RequestMetadataFromContext(r.Context())
	if session, ok := SessionFromContext(r.Context()); ok && strings.TrimSpace(session.SessionID) != "" {
		if strings.TrimSpace(session.PrincipalID) != "" {
			return "principal:" + strings.TrimSpace(session.PrincipalID), "session", strings.TrimSpace(session.PrincipalID)
		}
		return "session:" + strings.TrimSpace(session.SessionID), "session", ""
	}
	if strings.TrimSpace(requestMeta.SessionID) != "" {
		return "session:" + strings.TrimSpace(requestMeta.SessionID), "anonymous", ""
	}
	if ip := requestClientIP(r); ip != "" {
		return "ip:" + ip, "anonymous", ""
	}
	return "anonymous:global", "anonymous", ""
}

func requestClientIP(r *http.Request) string {
	for _, candidate := range []string{
		strings.TrimSpace(r.Header.Get("CF-Connecting-IP")),
		strings.TrimSpace(r.Header.Get("True-Client-IP")),
		firstForwardedFor(r.Header.Get("X-Forwarded-For")),
		strings.TrimSpace(r.Header.Get("X-Real-IP")),
		remoteHost(r.RemoteAddr),
	} {
		if ip := normalizedIP(candidate); ip != "" {
			return ip
		}
	}
	return ""
}

func firstForwardedFor(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	parts := strings.Split(value, ",")
	return strings.TrimSpace(parts[0])
}

func remoteHost(value string) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(value))
	if err != nil {
		return strings.TrimSpace(value)
	}
	return host
}

func normalizedIP(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	ip := net.ParseIP(value)
	if ip == nil {
		return ""
	}
	return ip.String()
}

func rateLimitMessage(namespace, authState string) string {
	switch {
	case authState == "session":
		return "Too many requests for this signed-in session. Try again shortly."
	case namespace == "feedback.incidents":
		return "Too many incident reports were sent from this browser. Try again shortly."
	default:
		return "Too many requests from this client. Try again shortly."
	}
}
