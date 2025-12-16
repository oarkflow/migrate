package main

import (
	"embed"
	"log"

	"github.com/oarkflow/migrate"
)

//go:embed ../migrations/* ../migrations/seeds/* ../migrations/templates/*
var assets embed.FS

func mai2n() {
	// Create a manager that uses embedded migrations/seeds/templates
	mgr := migrate.NewManager(migrate.WithEmbeddedFiles(assets))

	// Run as normal (this will use embedded files for listing/reading)
	mgr.Run()
	log.Println("Manager started with embedded assets")
}
