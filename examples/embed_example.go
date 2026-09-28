package main

import (
	"context"
	"embed"
	"log"

	"github.com/oarkflow/migrate"
)

//go:embed migrations/* migrations/seeds/*
var assets embed.FS

func mai2n() {
	// Create a manager that uses embedded migrations/seeds/templates
	ctx := context.Background()
	mgr := migrate.NewManager(migrate.WithContext(ctx), migrate.WithEmbeddedFiles(assets))

	// Run as normal (this will use embedded files for listing/reading)
	mgr.Run(ctx)
	log.Println("Manager started with embedded assets")
}
