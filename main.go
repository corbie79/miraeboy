package main

import (
	"log"

	"github.com/corbie79/miraeboy/internal/api"
	"github.com/corbie79/miraeboy/internal/config"
	"github.com/corbie79/miraeboy/internal/storage"
)

func main() {
	cfg := config.Load()

	store, err := storage.New(cfg.Server.StoragePath)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	server := api.NewServer(cfg, store)

	log.Printf("Conan2 server listening on %s", cfg.Server.Address)
	if err := server.Run(cfg.Server.Address); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
