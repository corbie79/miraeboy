package api

import (
	"net/http"
	"strings"

	"github.com/corbie79/miraeboy/internal/auth"
)

// auth validates the Bearer token. For routes with no context (global routes).
func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := s.extractClaims(w, r, false)
		if !ok {
			return
		}
		if claims != nil {
			r = r.WithContext(contextWithClaims(r.Context(), claims))
		}
		next(w, r)
	}
}

// requirePermission validates the Bearer token AND checks that the user has
// at least minPerm on the context named in the {context} path segment.
// It also handles anonymous access based on the context's configuration.
func (s *Server) requirePermission(minPerm auth.Permission, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		contextName := r.PathValue("context")

		// Validate context name to prevent path traversal
		if strings.ContainsAny(contextName, "/\\..") || contextName == "" {
			jsonError(w, http.StatusBadRequest, "invalid context name")
			return
		}

		// Check context exists (config-defined or dynamically created)
		if !s.contextExists(contextName) {
			jsonError(w, http.StatusNotFound, "context not found: "+contextName)
			return
		}

		header := r.Header.Get("Authorization")

		if header == "" {
			// No token: check anonymous access
			anonPerm := s.cfg.AnonymousPermission(contextName)
			if anonPerm.Satisfies(minPerm) {
				next(w, r)
				return
			}
			jsonError(w, http.StatusUnauthorized, "authorization required")
			return
		}

		claims, ok := s.extractClaims(w, r, true)
		if !ok {
			return
		}

		if !claims.PermissionFor(contextName).Satisfies(minPerm) {
			jsonError(w, http.StatusForbidden, "insufficient permission on context: "+contextName)
			return
		}

		r = r.WithContext(contextWithClaims(r.Context(), claims))
		next(w, r)
	}
}

// adminOnly validates the Bearer token and requires global admin role.
func (s *Server) adminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := s.extractClaims(w, r, true)
		if !ok {
			return
		}
		if !claims.Admin {
			jsonError(w, http.StatusForbidden, "admin required")
			return
		}
		r = r.WithContext(contextWithClaims(r.Context(), claims))
		next(w, r)
	}
}

// extractClaims parses the Bearer token from the Authorization header.
// If required is true, it writes an error response and returns false when the token is missing/invalid.
// If required is false, a missing token returns (nil, true) — caller handles anonymous.
func (s *Server) extractClaims(w http.ResponseWriter, r *http.Request, required bool) (*auth.Claims, bool) {
	header := r.Header.Get("Authorization")
	if header == "" {
		if required {
			jsonError(w, http.StatusUnauthorized, "authorization required")
			return nil, false
		}
		return nil, true
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		jsonError(w, http.StatusUnauthorized, "invalid authorization header")
		return nil, false
	}

	claims, err := auth.ValidateToken(s.cfg.Auth.JWTSecret, parts[1])
	if err != nil {
		jsonError(w, http.StatusUnauthorized, "invalid or expired token")
		return nil, false
	}

	return claims, true
}

// contextExists checks both config-defined and dynamic contexts.
func (s *Server) contextExists(name string) bool {
	// Check config-defined contexts
	if s.cfg.FindContext(name) != nil {
		return true
	}
	// Check dynamically created contexts
	return s.store.ContextExists(name)
}
