package deployment

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/0p5dev/ops/internal/config"
	prompts "github.com/0p5dev/ops/internal/prompts"
	"github.com/urfave/cli/v3"
)

type DeploymentDetails struct {
	Name        string            `json:"name"`
	URL         string            `json:"url"`
	Image       string            `json:"image"`
	Status      string            `json:"status"`
	Location    string            `json:"location"`
	CreatedTime string            `json:"created_time"`
	UpdatedTime string            `json:"updated_time"`
	Scaling     DeploymentScaling `json:"scaling"`
	// Metrics     DeploymentMetrics `json:"metrics"`
}

type DeploymentScaling struct {
	MinInstances int32 `json:"min_instances"`
	MaxInstances int32 `json:"max_instances"`
}

type DeploymentMetrics struct {
	RequestsPerHour [24]int `json:"requests_per_hour"`
	CPUPerHour      [24]int `json:"cpu_per_hour"`
}

func getDeploymentDetails(deploymentName string, token string, config config.Config) (DeploymentDetails, error) {
	var details DeploymentDetails

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/deployments/%s", config.ControllerBaseUrl, deploymentName), nil)
	if err != nil {
		return details, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return details, err
	}
	defer resp.Body.Close()

	if (resp.StatusCode == http.StatusUnauthorized) || (resp.StatusCode == http.StatusForbidden) {
		return details, fmt.Errorf("authentication failed: please log in again with 'ops login'")
	}

	if resp.StatusCode != http.StatusOK {
		return details, fmt.Errorf("Status: %s", resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(&details); err != nil {
		return details, fmt.Errorf("failed to decode response: %w", err)
	}

	return details, nil
}

func Describe(ctx context.Context, cmd *cli.Command) error {
	config := cmd.Metadata["config"].(config.Config)
	outputFormat := cmd.String("output")

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

	// Fetch and display deployment details with authentication retry
	return withAuthRetry(ctx, config, func(token string) error {
		details, err := getDeploymentDetails(deploymentName, token, config)
		if err != nil {
			return err
		}

		// Display deployment details based on output format
		switch strings.ToLower(outputFormat) {
		case "json":
			return outputJSON(details)
		case "yaml":
			return outputYAML(details)
		case "table":
			fallthrough
		default:
			return outputTableDetails(details)
		}
	})
}

func outputTableDetails(details DeploymentDetails) error {
	fmt.Printf("%-20s: %s\n", "Name", details.Name)
	fmt.Printf("%-20s: %s\n", "URL", details.URL)
	fmt.Printf("%-20s: %s\n", "Image", details.Image)
	fmt.Printf("%-20s: %s\n", "Status", details.Status)
	fmt.Printf("%-20s: %s\n", "Created", details.CreatedTime)
	fmt.Printf("%-20s: %s\n", "Updated", details.UpdatedTime)
	fmt.Printf("%-20s: %d - %d\n", "Scaling", details.Scaling.MinInstances, details.Scaling.MaxInstances)
	return nil
}
