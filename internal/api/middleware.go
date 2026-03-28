package api

import (
	"net/http"
	"strings"

	"github.com/corbie79/miraeboy/internal/auth"
)

type contextKey string

const claimsKey contextKey = "claims"

// auth wraps a HandlerFunc with Bearer token validation.
func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")

		if header == "" {
			if s.cfg.Auth.Anonymous {
				next(w, r)
				return
			}
			jsonError(w, http.StatusUnauthorized, "authorization required")
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			jsonError(w, http.StatusUnauthorized, "invalid authorization header")
			return
		}

		claims, err := auth.ValidateToken(s.cfg.Auth.JWTSecret, parts[1])
		if err != nil {
			jsonError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		// Store claims in context for downstream handlers.
		ctx := r.Context()
		ctx = contextWithClaims(ctx, claims)
		next(w, r.WithContext(ctx))
	}
}
