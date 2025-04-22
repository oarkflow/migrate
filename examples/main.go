package main

import (
	"github.com/oarkflow/squealx"

	"github.com/oarkflow/migrate/cmd"
)

func main() {
	dbConfig := squealx.Config{
		Host:     "localhost",
		Port:     5432,
		Driver:   "postgres",
		Username: "postgres",
		Password: "postgres",
		Database: "sujit",
	}
	config := cmd.Config{Config: dbConfig}
	err := cmd.Run(dbConfig.Driver, config)
	if err != nil {
		panic(err)
	}
}
