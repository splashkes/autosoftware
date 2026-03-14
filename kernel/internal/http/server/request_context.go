package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

const (
	HeaderRequestID     = "X-AS-Request-ID"
	HeaderSessionID     = "X-AS-Session-ID"
	HeaderSeedID        = "X-AS-Seed-ID"
	HeaderRealizationID = "X-AS-Realization-ID"

	DefaultSessionCookieName = "__Host-as_session"
	LegacySessionCookieName  = "as_session_id"
)

type requestMetadataKey struct{}

type RequestMetadataOptions struct {
	TrustRequestIDHeader  bool
	TrustSessionHeader    bool
	TrustSelectionHeaders bool
	SessionCookieNames    []string
}

type RequestMetadata struct {
	RequestID     string
	SessionID     string
	SeedID        string
	RealizationID string
	Route         string
	Method        string
	UserAgent     string
}

func WithRequestMetadata(ctx context.Context, metadata RequestMetadata) context.Context {
	return context.WithValue(ctx, requestMetadataKey{}, metadata)
}

func WithRoute(ctx context.Context, route string) context.Context {
	metadata := RequestMetadataFromContext(ctx)
	metadata.Route = route
	return WithRequestMetadata(ctx, metadata)
}

func RequestMetadataFromContext(ctx context.Context) RequestMetadata {
	metadata, ok := ctx.Value(requestMetadataKey{}).(RequestMetadata)
	if !ok {
		return RequestMetadata{}
	}

	return metadata
}

func CorrelationMiddleware(next http.Handler) http.Handler {
	return CorrelationMiddlewareWithOptions(RequestMetadataOptions{}, next)
}

func CorrelationMiddlewareWithOptions(options RequestMetadataOptions, next http.Handler) http.Handler {
	cookieNames := options.SessionCookieNames
	if len(cookieNames) == 0 {
		cookieNames = []string{DefaultSessionCookieName, LegacySessionCookieName}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := newOpaqueID("req")
		if options.TrustRequestIDHeader {
			requestID = firstNonEmpty(sanitizedOpaqueHeader(r, HeaderRequestID), requestID)
		}

		sessionID := sessionFromCookie(r, cookieNames...)
		if options.TrustSessionHeader {
			sessionID = firstNonEmpty(sanitizedOpaqueHeader(r, HeaderSessionID), sessionID)
		}

		metadata := RequestMetadata{
			RequestID: requestID,
			SessionID: sessionID,
			Route:     r.URL.Path,
			Method:    r.Method,
			UserAgent: r.UserAgent(),
		}
		if options.TrustSelectionHeaders {
			metadata.SeedID = sanitizedOpaqueHeader(r, HeaderSeedID)
			metadata.RealizationID = sanitizedOpaqueHeader(r, HeaderRealizationID)
		}

		w.Header().Set(HeaderRequestID, metadata.RequestID)
		if metadata.RealizationID != "" {
			w.Header().Set(HeaderRealizationID, metadata.RealizationID)
		}

		next.ServeHTTP(w, r.WithContext(WithRequestMetadata(r.Context(), metadata)))
	})
}

func trimmedHeader(r *http.Request, key string) string {
	return strings.TrimSpace(r.Header.Get(key))
}

var opaqueHeaderPattern = regexp.MustCompile(`^[A-Za-z0-9._:/-]{1,200}$`)

func sanitizedOpaqueHeader(r *http.Request, key string) string {
	value := trimmedHeader(r, key)
	if value == "" || !opaqueHeaderPattern.MatchString(value) {
		return ""
	}

	return value
}

func sessionFromCookie(r *http.Request, cookieNames ...string) string {
	for _, name := range cookieNames {
		if strings.TrimSpace(name) == "" {
			continue
		}

		cookie, err := r.Cookie(name)
		if err != nil {
			continue
		}

		value := strings.TrimSpace(cookie.Value)
		if opaqueHeaderPattern.MatchString(value) {
			return value
		}
	}

	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}

func newOpaqueID(prefix string) string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic(err)
	}

	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(raw[:]))
}
