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
			{
				Name:    "force",
				Aliases: []string{"f"},
				Usage:   "Force reset ignoring rollback statement errors",
				Value:   "false",
			},
		},
	}
}

func (c *ResetCommand) Handle(ctx contracts.Context) error {
	verbose := ctx.Option("v") != "" && ctx.Option("v") != "false"
	forceFlag := ctx.Option("f") != "" && ctx.Option("f") != "false"
	if mgr, ok := c.Driver.(*Manager); ok {
		mgr.Verbose = verbose
		if forceFlag {
			mgr.Force = true
			if mgr.dbDriver != nil {
				mgr.dbDriver.SetForce(true)
			}
		}
	}
	return c.Driver.ResetMigrations()
}
