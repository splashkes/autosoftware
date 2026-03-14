package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
)

const (
	HeaderRequestID     = "X-AS-Request-ID"
	HeaderSessionID     = "X-AS-Session-ID"
	HeaderSeedID        = "X-AS-Seed-ID"
	HeaderRealizationID = "X-AS-Realization-ID"
)

type requestMetadataKey struct{}

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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metadata := RequestMetadata{
			RequestID:     firstNonEmpty(trimmedHeader(r, HeaderRequestID), newOpaqueID("req")),
			SessionID:     firstNonEmpty(trimmedHeader(r, HeaderSessionID), sessionFromCookie(r)),
			SeedID:        trimmedHeader(r, HeaderSeedID),
			RealizationID: trimmedHeader(r, HeaderRealizationID),
			Route:         r.URL.Path,
			Method:        r.Method,
			UserAgent:     r.UserAgent(),
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

func sessionFromCookie(r *http.Request) string {
	cookie, err := r.Cookie("as_session_id")
	if err != nil {
		return ""
	}

	return strings.TrimSpace(cookie.Value)
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
