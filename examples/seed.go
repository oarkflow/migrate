package main

import (
	"fmt"
	"log"
	"os"
	
	"github.com/oarkflow/bcl"
	
	"github.com/oarkflow/migrate"
)

func main() {
	data, err := os.ReadFile("seed.bcl")
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}
	var cfg migrate.SeedConfig
	if _, err := bcl.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("Failed to unmarshal migration file: %v", err)
	}
	for _, c := range cfg.Seeds {
		fmt.Println(c.ToSQL("mysql"))
	}
}
