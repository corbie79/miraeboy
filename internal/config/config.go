package config

import (
	"log"
	"os"

	"github.com/corbie79/miraeboy/internal/auth"
	"gopkg.in/yaml.v3"
)

// UserContextEntry binds a user to a context with a specific permission level.
type UserContextEntry struct {
	Name       string `yaml:"name"`
	Permission string `yaml:"permission"` // "read", "readwrite", "admin"
}

// ContextDef defines a repository context declared in config.yaml.
type ContextDef struct {
	Name            string `yaml:"name"`
	Description     string `yaml:"description"`
	AnonymousAccess string `yaml:"anonymous_access"` // "read", "readwrite", "none"
}

// User represents an authenticated account.
type User struct {
	Username string             `yaml:"username"`
	Password string             `yaml:"password"`
	Admin    bool               `yaml:"admin"`
	Contexts []UserContextEntry `yaml:"contexts"`
}

type AuthConfig struct {
	Users     []User `yaml:"users"`
	JWTSecret string `yaml:"jwt_secret"`
	Anonymous bool   `yaml:"anonymous"` // global default anonymous read fallback
}

type ServerConfig struct {
	Address     string `yaml:"address"`
	StoragePath string `yaml:"storage_path"`
}

type Config struct {
	Server   ServerConfig `yaml:"server"`
	Auth     AuthConfig   `yaml:"auth"`
	Contexts []ContextDef `yaml:"contexts"`
}

func Load() *Config {
	cfg := &Config{}
	cfg.Server.Address = ":9300"
	cfg.Server.StoragePath = "./data"
	cfg.Auth.JWTSecret = "change-me-in-production"
	cfg.Auth.Anonymous = false

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

// FindContext returns the ContextDef with the given name, or nil.
func (c *Config) FindContext(name string) *ContextDef {
	for i := range c.Contexts {
		if c.Contexts[i].Name == name {
			return &c.Contexts[i]
		}
	}
	return nil
}

// BuildUserContextMap builds the context→permission map to embed in a JWT.
// Global admins receive {"*": "admin"}. Regular users get their explicit bindings.
func (c *Config) BuildUserContextMap(u *User) map[string]auth.Permission {
	m := make(map[string]auth.Permission)
	if u.Admin {
		m["*"] = auth.PermAdmin
		return m
	}
	for _, uc := range u.Contexts {
		m[uc.Name] = auth.Permission(uc.Permission)
	}
	return m
}

// AnonymousPermission returns the effective anonymous access level for a context.
// It checks the context's own anonymous_access setting, then falls back to the
// global auth.anonymous flag (which maps to PermRead when true).
func (c *Config) AnonymousPermission(contextName string) auth.Permission {
	def := c.FindContext(contextName)
	if def != nil && def.AnonymousAccess != "" {
		switch def.AnonymousAccess {
		case "read":
			return auth.PermRead
		case "readwrite":
			return auth.PermReadWrite
		case "admin":
			return auth.PermAdmin
		case "none":
			return auth.PermNone
		}
	}
	if c.Auth.Anonymous {
		return auth.PermRead
	}
	return auth.PermNone
}
