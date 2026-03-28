package config

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

// InitialMember seeds a group's member list at startup.
type InitialMember struct {
	Username   string `yaml:"username"`
	Permission string `yaml:"permission"` // "read", "write", "delete", "owner"
}

// GroupDef defines a package group declared in config.yaml.
// Used only for initial seeding — once stored on disk, the API manages everything.
type GroupDef struct {
	Name            string          `yaml:"name"`
	Description     string          `yaml:"description"`
	Owner           string          `yaml:"owner"`
	ConanUser       string          `yaml:"conan_user"`       // enforced @user on upload ("" = any)
	ConanChannel    string          `yaml:"conan_channel"`    // enforced @channel on upload ("" = any)
	AnonymousAccess string          `yaml:"anonymous_access"` // "read", "write", "none"
	Members         []InitialMember `yaml:"members"`
}

// User represents a server account.
type User struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Admin    bool   `yaml:"admin"`
}

type AuthConfig struct {
	Users     []User `yaml:"users"`
	JWTSecret string `yaml:"jwt_secret"`
}

type ServerConfig struct {
	Address     string `yaml:"address"`
	StoragePath string `yaml:"storage_path"`
}

type Config struct {
	Server ServerConfig `yaml:"server"`
	Auth   AuthConfig   `yaml:"auth"`
	Groups []GroupDef   `yaml:"groups"`
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
