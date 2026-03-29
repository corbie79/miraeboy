package config

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

// InitialMember seeds a repository's member list at startup.
type InitialMember struct {
	Username   string `yaml:"username"`
	Permission string `yaml:"permission"` // "read", "write", "delete", "owner"
}

// RepoDef defines a Conan repository declared in config.yaml.
// Used only for initial seeding — once stored on disk, the API manages everything.
type RepoDef struct {
	Name              string          `yaml:"name"`
	Description       string          `yaml:"description"`
	Owner             string          `yaml:"owner"`
	AllowedNamespaces []string        `yaml:"allowed_namespaces"` // enforced @namespace on upload (empty = any)
	AllowedChannels   []string        `yaml:"allowed_channels"`   // enforced channel on upload (empty = any)
	AnonymousAccess   string          `yaml:"anonymous_access"`   // "read", "write", "none"
	Members           []InitialMember `yaml:"members"`
}

// User represents a server account.
type User struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Admin    bool   `yaml:"admin"`
}

// OIDCGroupMapping maps a single OIDC group to a repository permission.
// Use repository: "*" to grant the permission on all repositories.
type OIDCGroupMapping struct {
	Group      string `yaml:"group"`
	Repository string `yaml:"repository"`
	Permission string `yaml:"permission"` // "read", "write", "delete", "owner"
}

// OIDCConfig holds OpenID Connect SSO settings.
// Leave Issuer empty to disable OIDC (local username/password is always available as fallback).
type OIDCConfig struct {
	Issuer       string             `yaml:"issuer"`        // e.g. https://keycloak.example.com/realms/company
	ClientID     string             `yaml:"client_id"`
	ClientSecret string             `yaml:"client_secret"`
	RedirectURL  string             `yaml:"redirect_url"`  // e.g. http://miraeboy.example.com/api/auth/oidc/callback
	GroupsClaim  string             `yaml:"groups_claim"`  // claim name for groups array (default: "groups")
	AdminGroups  []string           `yaml:"admin_groups"`  // any of these → admin=true
	GroupMappings []OIDCGroupMapping `yaml:"group_mappings"`
}

type AuthConfig struct {
	Users     []User     `yaml:"users"`
	JWTSecret string     `yaml:"jwt_secret"`
	OIDC      OIDCConfig `yaml:"oidc"`
}

// S3Config holds S3-compatible object storage settings.
// Leave Endpoint empty to disable S3 and fall back to local filesystem.
type S3Config struct {
	Endpoint        string `yaml:"endpoint"`
	Bucket          string `yaml:"bucket"`
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
	UseSSL          bool   `yaml:"use_ssl"`
	Region          string `yaml:"region"`
}

type ServerConfig struct {
	Address           string   `yaml:"address"`
	StoragePath       string   `yaml:"storage_path"`
	NodeRole          string   `yaml:"node_role"`          // "primary" (default) or "replica"
	ArtifactoryCompat bool     `yaml:"artifactory_compat"` // also register /artifactory/api/conan/... routes
	S3                S3Config `yaml:"s3"`
	GitWorkspace      string   `yaml:"git_workspace"` // base dir for per-repo git clones (default: ./git-workspace)
}

// BuildConfig holds the integrated build server settings.
// Set agent_key to enable the build system. Leave empty to disable.
type BuildConfig struct {
	AgentKey     string `yaml:"agent_key"`     // shared key for miraeboy-agent authentication
	ArtifactsDir string `yaml:"artifacts_dir"` // where to store built binaries (default: ./artifacts)
}

type Config struct {
	Server       ServerConfig `yaml:"server"`
	Auth         AuthConfig   `yaml:"auth"`
	Build        BuildConfig  `yaml:"build"`
	Repositories []RepoDef    `yaml:"repositories"`
}

func Load() *Config {
	cfg := &Config{}
	cfg.Server.Address = ":9300"
	cfg.Server.StoragePath = "./data"
	cfg.Auth.JWTSecret = "change-me-in-production"

	data, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Printf("config.yaml not found, using defaults")
		return cfg
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		log.Fatalf("Failed to parse config.yaml: %v", err)
	}
	return cfg
}

// FindUser returns the matching user or nil if credentials don't match.
func (c *Config) FindUser(username, password string) *User {
	for i := range c.Auth.Users {
		u := &c.Auth.Users[i]
		if u.Username == username && u.Password == password {
			return u
		}
	}
	return nil
}
