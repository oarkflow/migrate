package migrate

import (
	"fmt"
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
			{
				Name:    "include-raw",
				Aliases: []string{"r"},
				Usage:   "Include raw .sql seed files",
				Value:   "false",
			},
			{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Enable verbose output for seeding",
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
	includeRawOption := ctx.Option("include-raw")
	includeRaw := includeRawOption == "true" || includeRawOption == "1"
	verboseOption := ctx.Option("verbose")
	verbose := verboseOption == "true" || verboseOption == "1"
	if verbose {
		if mgr, ok := c.Driver.(*Manager); ok {
			mgr.Verbose = true
		}
	}
	if seedFile != "" {
		ext := strings.ToLower(filepath.Ext(seedFile))
		if ext == ".sql" && !includeRaw {
			return fmt.Errorf("raw seed file specified but --include-raw not set: %s", seedFile)
		}
		files = append(files, seedFile)
	} else {
		osFiles, _ := os.ReadDir(c.Driver.SeedDir())
		for _, file := range osFiles {
			if file.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(file.Name()))
			switch ext {
			case ".bcl":
				files = append(files, filepath.Join(c.Driver.SeedDir(), file.Name()))
			case ".sql":
				if includeRaw {
					files = append(files, filepath.Join(c.Driver.SeedDir(), file.Name()))
				}
			}
		}
	}
	if len(files) == 0 {
		logger.Printf("No seed files found in %s", c.Driver.SeedDir())
		return nil
	}
	return c.Driver.RunSeeds(truncate, includeRaw, files...)
}
