package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const tokenTTL = 24 * time.Hour

// Permission represents the access level for a package group.
type Permission string

const (
	PermNone   Permission = "none"
	PermRead   Permission = "read"
	PermWrite  Permission = "write"  // upload + download
	PermDelete Permission = "delete" // upload + download + delete packages
	PermOwner  Permission = "owner"  // delete + manage group settings & members
)

var permOrder = map[Permission]int{
	PermNone:   0,
	PermRead:   1,
	PermWrite:  2,
	PermDelete: 3,
	PermOwner:  4,
}

// Satisfies returns true if p meets or exceeds the required minimum permission.
func (p Permission) Satisfies(required Permission) bool {
	return permOrder[p] >= permOrder[required]
}

// Claims is the JWT payload.
type Claims struct {
	Username string                `json:"username"`
	Admin    bool                  `json:"admin"`
	Groups   map[string]Permission `json:"groups"` // {"conan-local":"write"} or {"*":"owner"}
	jwt.RegisteredClaims
}

// GroupPermission returns the effective permission for a package group.
// Global admins always get PermOwner. Wildcard "*" is used as fallback.
func (c *Claims) GroupPermission(group string) Permission {
	if c.Admin {
		return PermOwner
	}
	if c.Groups == nil {
		return PermNone
	}
	if p, ok := c.Groups[group]; ok {
		return p
	}
	if p, ok := c.Groups["*"]; ok {
		return p
	}
	return PermNone
}

// IssueToken generates a signed JWT embedding group permissions.
func IssueToken(secret, username string, admin bool, groups map[string]Permission) (string, error) {
	if groups == nil {
		groups = map[string]Permission{}
	}
	claims := Claims{
		Username: username,
		Admin:    admin,
		Groups:   groups,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateToken parses and validates a JWT, returning the claims.
func ValidateToken(secret, tokenStr string) (*Claims, error) {
	return ValidateTokenWithLeeway(secret, tokenStr, 0)
}

// ValidateTokenWithLeeway validates a JWT but allows tokens that expired within
// the given leeway window. Used by the refresh endpoint to accept recently-expired tokens.
func ValidateTokenWithLeeway(secret, tokenStr string, leeway time.Duration) (*Claims, error) {
	opts := []jwt.ParserOption{}
	if leeway > 0 {
		opts = append(opts, jwt.WithLeeway(leeway))
	}
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	}, opts...)
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}
