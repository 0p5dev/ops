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

type UpdateDeploymentRequestBody struct {
	ContainerImage string `json:"container_image"`
	MinInstances   *int   `json:"min_instances,omitempty"`
	MaxInstances   *int   `json:"max_instances,omitempty"`
	Port           *int   `json:"port,omitempty"`
}

func updateDeployment(ctx context.Context, deploymentName string, fqin string, token string, config config.Config, noWait bool) (serviceUrl string, err error) {
	min := config.MinInstances
	max := config.MaxInstances
	port := config.Port

	return updateDeploymentWithParams(ctx, deploymentName, fqin, token, config, &min, &max, &port, noWait)
}

func updateDeploymentWithParams(ctx context.Context, deploymentName string, containerImage string, token string, config config.Config, minInstances *int, maxInstances *int, port *int, noWait bool) (serviceUrl string, err error) {
	body := UpdateDeploymentRequestBody{
		ContainerImage: containerImage,
		MinInstances:   minInstances,
		MaxInstances:   maxInstances,
		Port:           port,
	}

	return executeUpdateDeploymentRequest(ctx, deploymentName, body, token, config, noWait)
}

func executeUpdateDeploymentRequest(ctx context.Context, deploymentName string, body UpdateDeploymentRequestBody, token string, config config.Config, noWait bool) (serviceUrl string, err error) {

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "PATCH", fmt.Sprintf("%s/api/v1/deployments/%s", config.ControllerBaseUrl, deploymentName), bytes.NewReader(bodyBytes))
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

	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return "", &deploymentNotFoundError{deploymentName: deploymentName}
	}

	if resp.StatusCode != http.StatusAccepted {
		resp.Body.Close()
		return "", fmt.Errorf("An unknown error occurred.")
	}

	return handleAcceptedDeploymentResponse(ctx, client, resp, config.ControllerBaseUrl, token, noWait)
}

// Update handles the updating of an existing deployment
func Update(ctx context.Context, cmd *cli.Command) error {
	config := cmd.Metadata["config"].(config.Config)

	// Perform creation with auth retry
	return withAuthRetry(ctx, config, func(token string) error {
		return performDeployment(ctx, cmd, token, config, updateDeployment)
	})
}
