package deployment

import (
	"context"
	"fmt"
	"net/http"

	"github.com/0p5dev/ops/internal/config"
	prompts "github.com/0p5dev/ops/internal/prompts"
	"github.com/0p5dev/ops/internal/ui"
	"github.com/urfave/cli/v3"
)

func destroyDeployment(deploymentName string, token string, config config.Config) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/deployments/%s", config.ControllerBaseUrl, deploymentName), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if (resp.StatusCode == http.StatusUnauthorized) || (resp.StatusCode == http.StatusForbidden) {
		return fmt.Errorf("authentication failed: please log in again with 'ops login'")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Status: %s", resp.Status)
	}

	return nil
}

// Destroy handles the destruction of an existing deployment
func Destroy(ctx context.Context, cmd *cli.Command) error {
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

	// Confirm destruction
	confirmed, err := prompts.PromptConfirmation(fmt.Sprintf("Are you sure you want to destroy deployment '%s'", deploymentName))
	if err != nil {
		return fmt.Errorf("confirmation prompt failed: %w", err)
	}
	if !confirmed {
		fmt.Println("Deployment destruction cancelled")
		return nil
	}

	// Perform destruction with auth retry
	err = withAuthRetry(ctx, config, func(token string) error {
		return ui.ShowSpinner("Destroying deployment...", func() error {
			return destroyDeployment(deploymentName, token, config)
		})
	})
	if err != nil {
		return fmt.Errorf("failed to destroy deployment: %v", err)
	}

	fmt.Printf("✓ Deployment '%s' destroyed successfully\n", deploymentName)
	return nil
}
