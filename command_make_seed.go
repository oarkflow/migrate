package migrate

import (
	"errors"
	
	"github.com/oarkflow/cli/contracts"
)

type MakeSeedCommand struct {
	Driver IManager
}

func (c *MakeSeedCommand) Signature() string {
	return "make:seed"
}

func (c *MakeSeedCommand) Description() string {
	return "Creates a new seed file for table in the designated directory."
}

func (c *MakeSeedCommand) Extend() contracts.Extend {
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

func (c *MakeSeedCommand) Handle(ctx contracts.Context) error {
	name := ctx.Argument(0)
	if name == "" {
		return errors.New("seed name is required")
	}
	return c.Driver.CreateSeedFile(name)
}
