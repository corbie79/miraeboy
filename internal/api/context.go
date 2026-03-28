package api

import (
	"context"

	"github.com/corbie79/miraeboy/internal/auth"
	"github.com/corbie79/miraeboy/internal/storage"
)

type contextKey string

const (
	claimsKey contextKey = "claims"
	repoKey   contextKey = "repo"
)

func contextWithClaims(ctx context.Context, claims *auth.Claims) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
}

func claimsFromContext(ctx context.Context) *auth.Claims {
	v := ctx.Value(claimsKey)
	if v == nil {
		return nil
	}
	c, _ := v.(*auth.Claims)
	return c
}

// contextWithRepo stores the loaded RepoRecord so handlers can access
// repository settings (allowed_namespaces, allowed_channels, etc.) without re-querying storage.
func contextWithRepo(ctx context.Context, r *storage.RepoRecord) context.Context {
	return context.WithValue(ctx, repoKey, r)
}

// repoFromContext retrieves the RepoRecord stored by requirePermission.
func repoFromContext(ctx context.Context) *storage.RepoRecord {
	v := ctx.Value(repoKey)
	if v == nil {
		return nil
	}
	r, _ := v.(*storage.RepoRecord)
	return r
}
