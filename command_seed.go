package migrate

import (
	"os"
	"path/filepath"
	"strings"
	
	"github.com/oarkflow/cli/contracts"
)

type ValidateCommand struct {
	Driver IManager
}

func (c *ValidateCommand) Signature() string {
	return "migration:validate"
}

func (c *ValidateCommand) Description() string {
	return "Validates the migration history against migration files."
}

func (c *ValidateCommand) Extend() contracts.Extend {
	return contracts.Extend{
		Flags: []contracts.Flag{
			{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Enable verbose output",
				Value:   "false",
			},
		},
	}
}

func (c *ValidateCommand) Handle(ctx contracts.Context) error {
	return c.Driver.ValidateMigrations()
}

type SeedCommand struct {
	Driver IManager
}

func (c *SeedCommand) Signature() string {
	return "db:seed"
}

func (c *SeedCommand) Description() string {
	return "Run database seeds from a seed file."
}

func (c *SeedCommand) Extend() contracts.Extend {
	return contracts.Extend{
		Flags: []contracts.Flag{
			{
				Name:    "file",
				Aliases: []string{"f"},
				Usage:   "Seed file to run",
				Value:   "",
			},
			{
				Name:    "truncate",
				Aliases: []string{"t"},
				Usage:   "Truncate tables before seeding",
				Value:   "false",
			},
		},
	}
}

func (c *SeedCommand) Handle(ctx contracts.Context) error {
	var files []string
	seedFile := ctx.Option("file")
	truncateOption := ctx.Option("truncate")
	truncate := truncateOption == "true" || truncateOption == "1"
	if seedFile != "" {
		files = append(files, seedFile)
	} else {
		osFiles, _ := os.ReadDir(c.Driver.SeedDir())
		for _, file := range osFiles {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".bcl") {
				files = append(files, filepath.Join(c.Driver.SeedDir(), file.Name()))
			}
		}
	}
	if len(files) == 0 {
		logger.Printf("No seed files found in %s", c.Driver.SeedDir())
		return nil
	}
	return c.Driver.RunSeeds(truncate, files...)
}
