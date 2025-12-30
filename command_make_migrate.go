package migrate

import (
	"errors"

	"github.com/oarkflow/cli/contracts"
)

type MakeMigrationCommand struct {
	Driver IManager
}

func (c *MakeMigrationCommand) Signature() string {
	return "make:migration"
}

func (c *MakeMigrationCommand) Description() string {
	return "Creates a new migration file in the designated directory."
}

func (c *MakeMigrationCommand) Extend() contracts.Extend {
	return contracts.Extend{
		Flags: []contracts.Flag{
			{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Enable verbose output",
				Value:   "false",
			},
			{
				Name:    "raw",
				Aliases: []string{"r"},
				Usage:   "Create raw SQL migration file",
				Value:   "false",
			},
		},
	}
}

func (c *MakeMigrationCommand) Handle(ctx contracts.Context) error {
	name := ctx.Argument(0)
	if name == "" {
		return errors.New("migration name is required")
	}
	raw := ctx.Option("raw") == "true" || ctx.Option("raw") == "1"
	return c.Driver.CreateMigrationFile(name, raw)
}
