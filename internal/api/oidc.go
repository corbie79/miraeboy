package api

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/corbie79/miraeboy/internal/auth"
	"github.com/corbie79/miraeboy/internal/config"
	"github.com/golang-jwt/jwt/v5"
)

// ─── Discovery + JWKS ────────────────────────────────────────────────────────

type discoveryDoc struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKsURI               string `json:"jwks_uri"`
}

// oidcProvider caches the discovery document, JWKS keys, and login state nonces.
type oidcProvider struct {
	mu          sync.RWMutex
	doc         *discoveryDoc
	keys        map[string]*rsa.PublicKey // kid → RSA public key
	lastRefresh time.Time

	stateMu sync.Mutex
	states  map[string]time.Time // state nonce → expiry
}

func newOIDCProvider() *oidcProvider {
	return &oidcProvider{
		keys:   make(map[string]*rsa.PublicKey),
		states: make(map[string]time.Time),
	}
}

// discover fetches (or returns cached) the OpenID Connect discovery document.
func (p *oidcProvider) discover(issuer string) (*discoveryDoc, error) {
	p.mu.RLock()
	if p.doc != nil && time.Since(p.lastRefresh) < 10*time.Minute {
		doc := p.doc
		p.mu.RUnlock()
		return doc, nil
	}
	p.mu.RUnlock()

	wellKnown := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	resp, err := http.Get(wellKnown) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}
	defer resp.Body.Close()

	var doc discoveryDoc
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("oidc discovery decode: %w", err)
	}

	keys, err := fetchJWKS(doc.JWKsURI)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	p.doc = &doc
	p.keys = keys
	p.lastRefresh = time.Now()
	p.mu.Unlock()

	return &doc, nil
}

// refreshKeys re-fetches the JWKS. Called when an unknown kid is encountered.
func (p *oidcProvider) refreshKeys(jwksURI string) error {
	keys, err := fetchJWKS(jwksURI)
	if err != nil {
		return err
	}
	p.mu.Lock()
	p.keys = keys
	p.mu.Unlock()
	return nil
}

func (p *oidcProvider) getKey(kid string) *rsa.PublicKey {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.keys[kid]
}

// fetchJWKS retrieves RSA public keys from a JWKS endpoint.
func fetchJWKS(uri string) (map[string]*rsa.PublicKey, error) {
	resp, err := http.Get(uri) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("jwks fetch: %w", err)
	}
	defer resp.Body.Close()

	var set struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return nil, fmt.Errorf("jwks decode: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey)
	for _, k := range set.Keys {
		if k.Kty != "RSA" || k.N == "" || k.E == "" {
			continue
		}
		nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			continue
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			continue
		}
		n := new(big.Int).SetBytes(nBytes)
		e := int(new(big.Int).SetBytes(eBytes).Int64())
		keys[k.Kid] = &rsa.PublicKey{N: n, E: e}
	}
	return keys, nil
}

// ─── State nonce ─────────────────────────────────────────────────────────────

func (p *oidcProvider) newState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	state := base64.RawURLEncoding.EncodeToString(b)

	p.stateMu.Lock()
	now := time.Now()
	for k, exp := range p.states {
		if now.After(exp) {
			delete(p.states, k)
		}
	}
	p.states[state] = now.Add(5 * time.Minute)
	p.stateMu.Unlock()
	return state
}

func (p *oidcProvider) consumeState(state string) bool {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	exp, ok := p.states[state]
	if !ok || time.Now().After(exp) {
		return false
	}
	delete(p.states, state)
	return true
}

// ─── Handlers ────────────────────────────────────────────────────────────────

// handleOIDCStatus returns whether OIDC is configured (used by web UI).
func (s *Server) handleOIDCStatus(w http.ResponseWriter, r *http.Request) {
	oidcCfg := s.cfg.Auth.OIDC
	if oidcCfg.Issuer == "" {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":   true,
		"login_url": "/api/auth/oidc/login",
	})
}

// handleOIDCLogin redirects the browser to the IdP authorization endpoint.
func (s *Server) handleOIDCLogin(w http.ResponseWriter, r *http.Request) {
	oidcCfg := s.cfg.Auth.OIDC
	if oidcCfg.Issuer == "" {
		jsonError(w, http.StatusNotFound, "OIDC not configured")
		return
	}

	doc, err := s.oidc.discover(oidcCfg.Issuer)
	if err != nil {
		log.Printf("OIDC discovery error: %v", err)
		jsonError(w, http.StatusServiceUnavailable, "OIDC provider unavailable")
		return
	}

	state := s.oidc.newState()

	q := url.Values{}
	q.Set("client_id", oidcCfg.ClientID)
	q.Set("redirect_uri", oidcCfg.RedirectURL)
	q.Set("response_type", "code")
	q.Set("scope", "openid email profile groups")
	q.Set("state", state)

	http.Redirect(w, r, doc.AuthorizationEndpoint+"?"+q.Encode(), http.StatusFound)
}

// handleOIDCCallback handles the authorization code callback from the IdP.
func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	oidcCfg := s.cfg.Auth.OIDC

	// Validate CSRF state
	if !s.oidc.consumeState(r.URL.Query().Get("state")) {
		http.Redirect(w, r, "/#/login?error=invalid_state", http.StatusFound)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		errMsg := r.URL.Query().Get("error_description")
		if errMsg == "" {
			errMsg = r.URL.Query().Get("error")
		}
		http.Redirect(w, r, "/#/login?error="+url.QueryEscape(errMsg), http.StatusFound)
		return
	}

	doc, err := s.oidc.discover(oidcCfg.Issuer)
	if err != nil {
		http.Redirect(w, r, "/#/login?error=provider_unavailable", http.StatusFound)
		return
	}

	// Exchange code → tokens
	tokenResp, err := exchangeCode(doc.TokenEndpoint, code, oidcCfg)
	if err != nil {
		log.Printf("OIDC token exchange error: %v", err)
		http.Redirect(w, r, "/#/login?error=token_exchange_failed", http.StatusFound)
		return
	}

	// Validate and parse ID token
	idClaims, err := s.validateIDToken(tokenResp.IDToken, doc, oidcCfg)
	if err != nil {
		log.Printf("OIDC ID token validation error: %v", err)
		http.Redirect(w, r, "/#/login?error=invalid_id_token", http.StatusFound)
		return
	}

	// Extract username
	username := stringClaim(idClaims, "preferred_username")
	if username == "" {
		username = stringClaim(idClaims, "email")
	}
	if username == "" {
		username = stringClaim(idClaims, "sub")
	}

	// Extract groups from configured claim (default: "groups")
	groupsClaim := oidcCfg.GroupsClaim
	if groupsClaim == "" {
		groupsClaim = "groups"
	}
	userGroups := stringSliceClaim(idClaims, groupsClaim)

	// Map OIDC groups → admin flag + repo permissions
	isAdmin, perms := mapOIDCGroups(userGroups, oidcCfg)

	// Issue internal JWT
	internalToken, err := auth.IssueToken(s.cfg.Auth.JWTSecret, username, isAdmin, perms)
	if err != nil {
		log.Printf("OIDC token issue error: %v", err)
		http.Redirect(w, r, "/#/login?error=internal_error", http.StatusFound)
		return
	}

	log.Printf("OIDC login: user=%s admin=%v groups=%v", username, isAdmin, userGroups)

	// Redirect web UI with the internal token in the URL fragment
	http.Redirect(w, r, "/#/auth/callback?token="+internalToken, http.StatusFound)
}

// ─── ID token validation ─────────────────────────────────────────────────────

func (s *Server) validateIDToken(idToken string, doc *discoveryDoc, cfg config.OIDCConfig) (jwt.MapClaims, error) {
	token, err := jwt.Parse(idToken, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		kid, _ := t.Header["kid"].(string)
		key := s.oidc.getKey(kid)
		if key == nil {
			// Unknown kid → try refreshing JWKS once
			if err := s.oidc.refreshKeys(doc.JWKsURI); err == nil {
				key = s.oidc.getKey(kid)
			}
		}
		if key == nil {
			return nil, fmt.Errorf("unknown signing key: %q", kid)
		}
		return key, nil
	}, jwt.WithValidMethods([]string{"RS256", "RS384", "RS512"}))

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Verify issuer
	iss, _ := claims["iss"].(string)
	if strings.TrimRight(iss, "/") != strings.TrimRight(cfg.Issuer, "/") {
		return nil, fmt.Errorf("issuer mismatch: got %q", iss)
	}

	// Verify audience (aud may be string or []string)
	if !audienceContains(claims["aud"], cfg.ClientID) {
		// Keycloak sometimes puts client_id in azp instead of aud
		azp, _ := claims["azp"].(string)
		if azp != cfg.ClientID {
			return nil, fmt.Errorf("audience mismatch")
		}
	}

	return claims, nil
}

// ─── Group mapping ────────────────────────────────────────────────────────────

// mapOIDCGroups resolves OIDC group membership into an admin flag and repo permissions.
func mapOIDCGroups(userGroups []string, cfg config.OIDCConfig) (isAdmin bool, perms map[string]auth.Permission) {
	perms = make(map[string]auth.Permission)

	groupSet := make(map[string]bool, len(userGroups))
	for _, g := range userGroups {
		groupSet[g] = true
	}

	// Admin check
	for _, ag := range cfg.AdminGroups {
		if groupSet[ag] {
			isAdmin = true
			break
		}
	}

	// Repo permission mappings — keep highest permission per repository
	for _, m := range cfg.GroupMappings {
		if !groupSet[m.Group] {
			continue
		}
		perm := auth.Permission(m.Permission)
		existing := perms[m.Repository]
		if existing == "" || perm.Satisfies(existing) {
			perms[m.Repository] = perm
		}
	}

	return isAdmin, perms
}

// ─── Token exchange ───────────────────────────────────────────────────────────

type oidcTokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

func exchangeCode(tokenEndpoint, code string, cfg config.OIDCConfig) (*oidcTokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", cfg.RedirectURL)
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", cfg.ClientSecret)

	resp, err := http.PostForm(tokenEndpoint, form) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token endpoint %d: %s", resp.StatusCode, body)
	}

	var tr oidcTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return nil, fmt.Errorf("token decode: %w", err)
	}
	if tr.IDToken == "" {
		return nil, fmt.Errorf("no id_token in response (check IdP client scope configuration)")
	}
	return &tr, nil
}

// ─── Claim helpers ────────────────────────────────────────────────────────────

func stringClaim(claims jwt.MapClaims, key string) string {
	v, _ := claims[key].(string)
	return v
}

func stringSliceClaim(claims jwt.MapClaims, key string) []string {
	raw, ok := claims[key]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case string:
		// comma-separated fallback
		return strings.Split(v, ",")
	}
	return nil
}

func audienceContains(aud any, clientID string) bool {
	switch v := aud.(type) {
	case string:
		return v == clientID
	case []any:
		for _, a := range v {
			if s, ok := a.(string); ok && s == clientID {
				return true
			}
		}
	case []string:
		for _, s := range v {
			if s == clientID {
				return true
			}
		}
	}
	return false
}
