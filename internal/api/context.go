package api

import (
	"context"

	"github.com/corbie79/miraeboy/internal/auth"
)

func contextWithClaims(ctx context.Context, claims *auth.Claims) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
}

// claimsFromContext retrieves the JWT claims stored by the auth middleware.
// Returns nil if no claims are present (anonymous access).
func claimsFromContext(ctx context.Context) *auth.Claims {
	v := ctx.Value(claimsKey)
	if v == nil {
		return nil
	}
	c, _ := v.(*auth.Claims)
	return c
}
