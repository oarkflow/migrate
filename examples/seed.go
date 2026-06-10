package main

import (
	"fmt"
	"log"
	"os"

	"github.com/oarkflow/migrate"
)

func seedExample() {
	data, err := os.ReadFile("seed.bcl")
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}
	seed, err := migrate.ParseSeedBCL(data)
	if err != nil {
		log.Fatalf("Failed to parse seed file: %v", err)
	}
	queries, err := seed.ToSQL("postgres")
	if err != nil {
		log.Fatalf("Failed to generate seed SQL: %v", err)
	}
	for _, q := range queries {
		fmt.Println(q.SQL)
		// Here you could execute the SQL using your DB driver
	}
}
