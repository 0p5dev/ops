package deployment

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/0p5dev/ops/internal/config"
	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"
)

type Deployment struct {
	Id             string    `json:"id" yaml:"id"`
	Name           string    `json:"name" yaml:"name"`
	Url            string    `json:"url" yaml:"url"`
	ContainerImage string    `json:"container_image" yaml:"containerImage"`
	UserEmail      string    `json:"user_email" yaml:"userEmail"`
	MinInstances   int       `json:"min_instances" yaml:"minInstances"`
	MaxInstances   int       `json:"max_instances" yaml:"maxInstances"`
	CreatedAt      time.Time `json:"created_at" yaml:"createdAt"`
	UpdatedAt      time.Time `json:"updated_at" yaml:"updatedAt"`
}

type PaginatedDeploymentsResponse struct {
	Deployments []Deployment `json:"deployments"`
	Count       int          `json:"count"`
	Page        int          `json:"page"`
	Limit       int          `json:"limit"`
	TotalPages  int          `json:"total_pages"`
}

// List retrieves and displays all deployments
func List(ctx context.Context, cmd *cli.Command) error {
	config := cmd.Metadata["config"].(config.Config)
	outputFormat := cmd.String("output")

	return withAuthRetry(ctx, config, func(token string) error {
		req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/deployments", config.ControllerBaseUrl), nil)
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
			return fmt.Errorf("failed to list deployments: %s", resp.Status)
		}

		var deploymentsResponse PaginatedDeploymentsResponse
		if err := json.NewDecoder(resp.Body).Decode(&deploymentsResponse); err != nil {
			return fmt.Errorf("failed to decode response: %v", err)
		}

		deployments := deploymentsResponse.Deployments

		if len(deployments) == 0 {
			fmt.Println("No deployments found")
			return nil
		}

		// Output based on format
		switch strings.ToLower(outputFormat) {
		case "json":
			return outputJSON(deployments)
		case "yaml":
			return outputYAML(deployments)
		case "table":
			fallthrough
		default:
			return outputTable(deployments)
		}
	})
}

func outputTable(deployments []Deployment) error {
	if len(deployments) == 0 {
		fmt.Println("No deployments found")
		return nil
	}

	// Print header
	fmt.Printf("%-30s %-45s %-15s %-15s %-20s\n", "NAME", "URL", "MIN INSTANCES", "MAX INSTANCES", "CREATED AT")
	fmt.Println(strings.Repeat("-", 125))

	// Print rows
	for _, d := range deployments {
		name := d.Name
		if len(name) > 30 {
			name = name[:27] + "..."
		}
		url := d.Url
		if len(url) > 45 {
			url = url[:42] + "..."
		}
		createdAt := d.CreatedAt.Format("2006-01-02 15:04:05")
		fmt.Printf("%-30s %-45s %-15d %-15d %-20s\n", name, url, d.MinInstances, d.MaxInstances, createdAt)
	}

	fmt.Printf("\nTotal: %d deployment(s)\n", len(deployments))
	return nil
}

func outputJSON(deployments []Deployment) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(deployments)
}

func outputYAML(deployments []Deployment) error {
	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(2)
	defer encoder.Close()
	return encoder.Encode(deployments)
}
