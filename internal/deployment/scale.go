package deployment

import (
	"context"
	"fmt"

	"github.com/0p5dev/ops/internal/config"
	prompts "github.com/0p5dev/ops/internal/prompts"
	"github.com/0p5dev/ops/internal/ui"
	"github.com/urfave/cli/v3"
)

func Scale(ctx context.Context, cmd *cli.Command) error {
	config := cmd.Metadata["config"].(config.Config)

	// Get deployment name from argument or prompt
	var deploymentName string
	if cmd.Args().Len() > 1 {
		return fmt.Errorf("too many arguments: expected at most 1 deployment name, got %d", cmd.Args().Len())
	} else if cmd.Args().Len() == 1 {
		deploymentName = cmd.Args().First()
		// Validate the deployment name
		if err := validateDeploymentName(deploymentName); err != nil {
			return fmt.Errorf("invalid deployment name: %w", err)
		}
	} else {
		var err error
		deploymentName, err = prompts.PromptName("Deployment Name")
		if err != nil {
			return fmt.Errorf("failed to get deployment name: %w", err)
		}
	}

	// Get scaling parameters from flags or prompts
	var minInstances, maxInstances int32

	if cmd.IsSet("min-instances") {
		minInstances = cmd.Int32("min-instances")
	} else {
		val, err := prompts.PromptForInt("Minimum instances", 0, 10, 0)
		if err != nil {
			return fmt.Errorf("failed to get minimum instances: %w", err)
		}
		minInstances = int32(val)
	}

	if cmd.IsSet("max-instances") {
		maxInstances = cmd.Int32("max-instances")
	} else {
		val, err := prompts.PromptForInt("Maximum instances", int(minInstances), 10, int(minInstances))
		if err != nil {
			return fmt.Errorf("failed to get maximum instances: %w", err)
		}
		maxInstances = int32(val)
	}

	// Validate scaling parameters
	if minInstances < 0 || maxInstances < 0 {
		return fmt.Errorf("min-instances and max-instances must be non-negative")
	}
	if minInstances > maxInstances {
		return fmt.Errorf("min-instances cannot be greater than max-instances")
	}

	// Confirm scaling operation
	confirmed, err := prompts.PromptConfirmation(fmt.Sprintf("Are you sure you want to scale deployment '%s' to min=%d, max=%d instances", deploymentName, minInstances, maxInstances))
	if err != nil {
		return fmt.Errorf("confirmation prompt failed: %w", err)
	}
	if !confirmed {
		fmt.Println("Scaling operation cancelled")
		return nil
	}

	// Perform scaling with UI feedback
	return withAuthRetry(ctx, config, func(token string) error {
		return performScaling(ctx, deploymentName, minInstances, maxInstances, token, config)
	})
}

func performScaling(ctx context.Context, deploymentName string, minInstances, maxInstances int32, token string, config config.Config) error {
	var details DeploymentDetails
	var err error

	// Get deployment details with spinner
	err = ui.ShowSpinner("Retrieving deployment...", func() error {
		details, err = getDeploymentDetails(deploymentName, token, config)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to retrieve deployment details: %w", err)
	}
	fmt.Println("✓ Retrieved deployment details")

	// Update scaling configuration
	details.Scaling.MinInstances = minInstances
	details.Scaling.MaxInstances = maxInstances
	min := int(details.Scaling.MinInstances)
	max := int(details.Scaling.MaxInstances)

	// Scale deployment with spinner
	err = ui.ShowSpinner("Scaling deployment...", func() error {
		_, err = updateDeploymentWithParams(ctx, deploymentName, details.Image, token, config, &min, &max, nil, false)
		return err
	})

	if err != nil {
		return fmt.Errorf("scaling failed: %w", err)
	}

	// Success message with details
	fmt.Printf("✓ Deployment '%s' scaled successfully!\n", deploymentName)
	fmt.Printf("  • Min instances: %d\n", minInstances)
	fmt.Printf("  • Max instances: %d\n", maxInstances)
	fmt.Println()
	fmt.Println("The deployment will automatically adjust the number of instances based on demand.")

	return nil
}
