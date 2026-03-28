package api

import (
	"net/http"
	"strings"

	"github.com/corbie79/miraeboy/internal/auth"
)

// requirePermission validates the Bearer token and checks that the user has
// at least minPerm on the repository named in the {repository} path segment.
// Anonymous access is allowed when the repository's anonymous_access meets minPerm.
// On success the RepoRecord is stored in the request context for handlers.
func (s *Server) requirePermission(minPerm auth.Permission, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repoName := r.PathValue("repository")

		if strings.ContainsAny(repoName, "/\\.") || repoName == "" {
			jsonError(w, http.StatusBadRequest, "invalid repository name")
			return
		}

		// Load repository (also verifies existence)
		repo, err := s.store.GetRepo(repoName)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if repo == nil {
			jsonError(w, http.StatusNotFound, "repository not found: "+repoName)
			return
		}

		header := r.Header.Get("Authorization")

		if header == "" {
			// Unauthenticated: check anonymous access setting
			anonPerm := auth.Permission(repo.AnonymousAccess)
			if anonPerm == "" {
				anonPerm = auth.PermNone
			}
			if anonPerm.Satisfies(minPerm) {
				r = r.WithContext(contextWithRepo(r.Context(), repo))
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

		if !claims.GroupPermission(repoName).Satisfies(minPerm) {
			jsonError(w, http.StatusForbidden, "insufficient permission on repository: "+repoName)
			return
		}

		ctx := contextWithClaims(r.Context(), claims)
		ctx = contextWithRepo(ctx, repo)
		next(w, r.WithContext(ctx))
	}
}

// requireRepoOwnerOrAdmin validates the token and requires that the user is
// either the global admin or has PermOwner on the repository in the path.
func (s *Server) requireRepoOwnerOrAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := s.extractClaims(w, r, true)
		if !ok {
			return
		}

		repoName := r.PathValue("repository")
		if !claims.Admin && !claims.GroupPermission(repoName).Satisfies(auth.PermOwner) {
			jsonError(w, http.StatusForbidden, "repository owner or admin required")
			return
		}

		// Load repo into context (may be needed by handler)
		repo, _ := s.store.GetRepo(repoName)
		ctx := contextWithClaims(r.Context(), claims)
		if repo != nil {
			ctx = contextWithRepo(ctx, repo)
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

// replicaReadOnly blocks write requests (PUT, POST, DELETE, PATCH) when
// this node is running in replica mode. Replica nodes are read-only; all
// writes must go through the primary node.
func (s *Server) replicaReadOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.nodeRole == "replica" {
			switch r.Method {
			case http.MethodPut, http.MethodPost, http.MethodDelete, http.MethodPatch:
				jsonError(w, http.StatusServiceUnavailable,
					"this node is read-only (replica); send write requests to the primary node")
				return
			}
		}
		next(w, r)
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
