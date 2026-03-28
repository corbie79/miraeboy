package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const tokenTTL = 24 * time.Hour

// Permission represents the access level for a context.
type Permission string

const (
	PermRead      Permission = "read"
	PermReadWrite Permission = "readwrite"
	PermAdmin     Permission = "admin"
	PermNone      Permission = "none"
)

// Satisfies returns true if p meets or exceeds the required minimum permission.
func (p Permission) Satisfies(required Permission) bool {
	order := map[Permission]int{
		PermNone:      0,
		PermRead:      1,
		PermReadWrite: 2,
		PermAdmin:     3,
	}
	return order[p] >= order[required]
}

type Claims struct {
	Username string                `json:"username"`
	Admin    bool                  `json:"admin"`
	Contexts map[string]Permission `json:"contexts"` // {"conan-local":"readwrite"}, or {"*":"admin"}
	jwt.RegisteredClaims
}

// PermissionFor returns the effective permission for the given context.
// Global admins always get PermAdmin. Wildcard "*" is checked as a fallback.
func (c *Claims) PermissionFor(ctx string) Permission {
	if c.Admin {
		return PermAdmin
	}
	if c.Contexts == nil {
		return PermNone
	}
	if p, ok := c.Contexts[ctx]; ok {
		return p
	}
	if p, ok := c.Contexts["*"]; ok {
		return p
	}
	return PermNone
}

// IssueToken generates a signed JWT for the given user with context permissions.
func IssueToken(secret, username string, admin bool, contexts map[string]Permission) (string, error) {
	if contexts == nil {
		contexts = map[string]Permission{}
	}
	claims := Claims{
		Username: username,
		Admin:    admin,
		Contexts: contexts,
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
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}
