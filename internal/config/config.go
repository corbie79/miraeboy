package config

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type User struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Admin    bool   `yaml:"admin"`
}

type AuthConfig struct {
	Users     []User `yaml:"users"`
	JWTSecret string `yaml:"jwt_secret"`
	Anonymous bool   `yaml:"anonymous"` // true = 인증 없이 읽기 허용
}

type ServerConfig struct {
	Address     string `yaml:"address"`
	StoragePath string `yaml:"storage_path"`
}

type Config struct {
	Server ServerConfig `yaml:"server"`
	Auth   AuthConfig   `yaml:"auth"`
}

func Load() *Config {
	cfg := &Config{}
	// 기본값
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

func (c *Config) FindUser(username, password string) *User {
	for i := range c.Auth.Users {
		u := &c.Auth.Users[i]
		if u.Username == username && u.Password == password {
			return u
		}
	}
	return nil
}
