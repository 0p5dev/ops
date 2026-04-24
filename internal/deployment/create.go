package deployment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/0p5dev/ops/internal/config"
	"github.com/urfave/cli/v3"
)

type CreateDeploymentRequestBody struct {
	Name           string `json:"name"`
	ContainerImage string `json:"container_image"`
	MinInstances   *int   `json:"min_instances,omitempty,string"`
	MaxInstances   *int   `json:"max_instances,omitempty,string"`
	Port           *int   `json:"port,omitempty,string"`
}

func createDeployment(ctx context.Context, deploymentName string, fqin string, token string, config config.Config, noWait bool) (serviceUrl string, err error) {
	min := config.MinInstances
	max := config.MaxInstances
	port := config.Port

	body := CreateDeploymentRequestBody{
		Name:           deploymentName,
		ContainerImage: fqin,
		MinInstances:   &min,
		MaxInstances:   &max,
		Port:           &port,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/api/v1/deployments", config.ControllerBaseUrl), bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	if handled, commonErr := handleCommonDeploymentHttpErrors(resp); handled {
		return "", commonErr
	}

	if resp.StatusCode == http.StatusConflict {
		if closeErr := resp.Body.Close(); closeErr != nil {
			return "", fmt.Errorf("failed to close response body: %w", closeErr)
		}
		return "", &deploymentAlreadyExistsError{deploymentName: deploymentName}
	}

	if resp.StatusCode != http.StatusAccepted {
		if closeErr := resp.Body.Close(); closeErr != nil {
			return "", fmt.Errorf("failed to close response body: %w", closeErr)
		}
		return "", fmt.Errorf("an unknown error occurred")
	}

	return handleAcceptedDeploymentResponse(ctx, client, resp, config.ControllerBaseUrl, token, noWait)
}

// Create handles the creation of a new deployment
func Create(ctx context.Context, cmd *cli.Command) error {
	config := cmd.Metadata["config"].(config.Config)

	// Perform creation with auth retry
	return withAuthRetry(ctx, config, func(token string) error {
		return performDeployment(ctx, cmd, token, config, createDeployment)
	})
}
