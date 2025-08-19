package migrate

import (
	"fmt"
	"strconv"

	"github.com/oarkflow/cli/contracts"
)

type RollbackCommand struct {
	Driver IManager
}

func (c *RollbackCommand) Signature() string {
	return "migration:rollback"
}

func (c *RollbackCommand) Description() string {
	return "Rolls back migrations. Optionally specify --step=<n>."
}

func (c *RollbackCommand) Extend() contracts.Extend {
	return contracts.Extend{
		Flags: []contracts.Flag{
			{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Enable verbose output",
				Value:   "false",
			},
			{
				Name:    "step",
				Aliases: []string{"s"},
				Usage:   "Number of migrations to rollback (default: 1)",
				Value:   "1",
			},
		},
	}
}

func (c *RollbackCommand) Handle(ctx contracts.Context) error {
	verbose := ctx.Option("v") != "" && ctx.Option("v") != "false"
	if mgr, ok := c.Driver.(*Manager); ok {
		mgr.Verbose = verbose
	}
	stepStr := ctx.Option("step")
	step := 1
	if stepStr != "" {
		var err error
		step, err = strconv.Atoi(stepStr)
		if err != nil {
			return fmt.Errorf("invalid step value: %w", err)
		}
	}
	return c.Driver.RollbackMigration(step)
}
