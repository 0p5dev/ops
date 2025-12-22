package deployment

import (
	"context"
	"fmt"

	"github.com/0p5dev/ops/internal/config"
	"github.com/urfave/cli/v3"
)

// Deploy is the main entry point that routes to Create or Destroy based on flags
func Deploy(ctx context.Context, cmd *cli.Command) error {
	config := cmd.Metadata["config"].(config.Config)

	// Override config with command-line flags if provided
	if cmd.IsSet("min-instances") {
		config.MinInstances = cmd.Int("min-instances")
	}
	if cmd.IsSet("max-instances") {
		config.MaxInstances = cmd.Int("max-instances")
	}
	if cmd.IsSet("port") {
		config.Port = cmd.Int("port")
	}

	// Validate after flag overrides
	if config.MinInstances > config.MaxInstances {
		return fmt.Errorf("minInstances (%d) cannot be greater than maxInstances (%d)", config.MinInstances, config.MaxInstances)
	}

	// Update config in metadata for downstream functions
	cmd.Metadata["config"] = config

	// Check if destroy flag is set
	if cmd.Bool("destroy") {
		return Destroy(ctx, cmd)
	}

	return Create(ctx, cmd)
}
