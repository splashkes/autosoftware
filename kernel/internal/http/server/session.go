package server

import (
	"context"
	"errors"
	"net/http"
)

type resolvedSessionKey struct{}

type ResolvedSession struct {
	SessionID   string
	PrincipalID string
	Status      string
}

type SessionResolver interface {
	ResolveSession(context.Context, string) (ResolvedSession, error)
}

func SessionFromContext(ctx context.Context) (ResolvedSession, bool) {
	session, ok := ctx.Value(resolvedSessionKey{}).(ResolvedSession)
	return session, ok
}

func SessionResolutionMiddleware(resolver SessionResolver, next http.Handler) http.Handler {
	if resolver == nil {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metadata := RequestMetadataFromContext(r.Context())
		if metadata.SessionID == "" {
			next.ServeHTTP(w, r)
			return
		}

		session, err := resolver.ResolveSession(r.Context(), metadata.SessionID)
		if err != nil && !errors.Is(err, context.Canceled) {
			next.ServeHTTP(w, r)
			return
		}
		if session.SessionID == "" {
			next.ServeHTTP(w, r)
			return
		}

		ctx := context.WithValue(r.Context(), resolvedSessionKey{}, session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
