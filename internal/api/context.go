package api

import (
	"context"

	"github.com/corbie79/miraeboy/internal/auth"
	"github.com/corbie79/miraeboy/internal/storage"
)

type contextKey string

const (
	claimsKey contextKey = "claims"
	groupKey  contextKey = "group"
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

// contextWithGroup stores the loaded GroupRecord so handlers can access
// group settings (conan_user, conan_channel, etc.) without re-querying storage.
func contextWithGroup(ctx context.Context, g *storage.GroupRecord) context.Context {
	return context.WithValue(ctx, groupKey, g)
}

// groupFromContext retrieves the GroupRecord stored by requirePermission.
func groupFromContext(ctx context.Context) *storage.GroupRecord {
	v := ctx.Value(groupKey)
	if v == nil {
		return nil
	}
	g, _ := v.(*storage.GroupRecord)
	return g
}
