package api

import (
	"net/http"
	"strings"

	"github.com/corbie79/miraeboy/internal/auth"
)

// requirePermission validates the Bearer token and checks that the user has
// at least minPerm on the package group named in the {group} path segment.
// Anonymous access is allowed when the group's anonymous_access meets minPerm.
// On success the GroupRecord is stored in the request context for handlers.
func (s *Server) requirePermission(minPerm auth.Permission, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupName := r.PathValue("group")

		if strings.ContainsAny(groupName, "/\\.") || groupName == "" {
			jsonError(w, http.StatusBadRequest, "invalid group name")
			return
		}

		// Load group (also verifies existence)
		grp, err := s.store.GetGroup(groupName)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if grp == nil {
			jsonError(w, http.StatusNotFound, "group not found: "+groupName)
			return
		}

		header := r.Header.Get("Authorization")

		if header == "" {
			// Unauthenticated: check anonymous access setting
			anonPerm := auth.Permission(grp.AnonymousAccess)
			if anonPerm == "" {
				anonPerm = auth.PermNone
			}
			if anonPerm.Satisfies(minPerm) {
				r = r.WithContext(contextWithGroup(r.Context(), grp))
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

		if !claims.GroupPermission(groupName).Satisfies(minPerm) {
			jsonError(w, http.StatusForbidden, "insufficient permission on group: "+groupName)
			return
		}

		ctx := contextWithClaims(r.Context(), claims)
		ctx = contextWithGroup(ctx, grp)
		next(w, r.WithContext(ctx))
	}
}

// requireGroupOwnerOrAdmin validates the token and requires that the user is
// either the global admin or has PermOwner on the group in the path.
// Used for group settings and member management endpoints.
func (s *Server) requireGroupOwnerOrAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := s.extractClaims(w, r, true)
		if !ok {
			return
		}

		groupName := r.PathValue("group")
		if !claims.Admin && !claims.GroupPermission(groupName).Satisfies(auth.PermOwner) {
			jsonError(w, http.StatusForbidden, "group owner or admin required")
			return
		}

		// Load group into context (may be needed by handler)
		grp, _ := s.store.GetGroup(groupName)
		ctx := contextWithClaims(r.Context(), claims)
		if grp != nil {
			ctx = contextWithGroup(ctx, grp)
		}
		next(w, r.WithContext(ctx))
	}
}

// adminOnly validates the token and requires the global admin flag.
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
		next(w, r.WithContext(contextWithClaims(r.Context(), claims)))
	}
}

// extractClaims parses the Bearer token from the Authorization header.
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
