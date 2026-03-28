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

	store, err := storage.New(cfg.Server.StoragePath)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// Strip the "web/dist" prefix so the FS root is the dist directory
	webFS, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		log.Fatalf("Failed to prepare web assets: %v", err)
	}

	server := api.NewServer(cfg, store, webFS)

	log.Printf("Conan2 server listening on %s", cfg.Server.Address)
	if err := server.Run(cfg.Server.Address); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
