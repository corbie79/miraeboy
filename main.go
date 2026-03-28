package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/corbie79/miraeboy/internal/api"
	"github.com/corbie79/miraeboy/internal/config"
	"github.com/corbie79/miraeboy/internal/storage"
)

//go:embed web/dist
var webDist embed.FS

func main() {
	cfg := config.Load()

	var store *storage.Storage
	var err error

	if cfg.Server.S3.Endpoint != "" {
		// Use S3-compatible backend
		s3Cfg := storage.S3Config{
			Endpoint:        cfg.Server.S3.Endpoint,
			Bucket:          cfg.Server.S3.Bucket,
			AccessKeyID:     cfg.Server.S3.AccessKeyID,
			SecretAccessKey: cfg.Server.S3.SecretAccessKey,
			UseSSL:          cfg.Server.S3.UseSSL,
			Region:          cfg.Server.S3.Region,
		}
		b, s3Err := storage.NewS3Backend(s3Cfg)
		if s3Err != nil {
			log.Fatalf("Failed to initialize S3 storage: %v", s3Err)
		}
		store = storage.NewWithBackend(b)
		log.Printf("Using S3 backend: %s/%s", cfg.Server.S3.Endpoint, cfg.Server.S3.Bucket)
	} else {
		store, err = storage.New(cfg.Server.StoragePath)
		if err != nil {
			log.Fatalf("Failed to initialize storage: %v", err)
		}
		log.Printf("Using filesystem backend: %s", cfg.Server.StoragePath)
	}

	// Strip the "web/dist" prefix so the FS root is the dist directory
	webFS, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		log.Fatalf("Failed to prepare web assets: %v", err)
	}

	role := cfg.Server.NodeRole
	if role == "" {
		role = "primary"
	}
	log.Printf("Node role: %s", role)

	server := api.NewServer(cfg, store, webFS)

	log.Printf("Conan2 server listening on %s", cfg.Server.Address)
	if err := server.Run(cfg.Server.Address); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
