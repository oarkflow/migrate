package migrate

import (
	"github.com/oarkflow/cli/contracts"
)

type ResetCommand struct {
	Driver IManager
}

func (c *ResetCommand) Signature() string {
	return "migration:reset"
}

func (c *ResetCommand) Description() string {
	return "Resets migrations by rolling back and reapplying all migrations."
}

func (c *ResetCommand) Extend() contracts.Extend {
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

func (c *ResetCommand) Handle(ctx contracts.Context) error {
	return c.Driver.ResetMigrations()
}
