package server

import (
	"context"

	"as/kernel/internal/interactions"
)

type runtimeSessionLookup interface {
	ResolveSession(context.Context, string) (interactions.ResolvedSession, error)
}

type RuntimeSessionResolver struct {
	Lookup runtimeSessionLookup
}

func (r RuntimeSessionResolver) ResolveSession(ctx context.Context, sessionID string) (ResolvedSession, error) {
	item, err := r.Lookup.ResolveSession(ctx, sessionID)
	if err != nil {
		return ResolvedSession{}, err
	}
	return ResolvedSession{
		SessionID:   item.SessionID,
		PrincipalID: item.PrincipalID,
		Status:      item.Status,
	}, nil
}
