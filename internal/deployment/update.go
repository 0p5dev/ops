package deployment

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/0p5dev/ops/internal/config"
	"github.com/urfave/cli/v3"
)

type UpdateDeploymentRequestBody struct {
	ContainerImage string `json:"container_image"`
	MinInstances   *int   `json:"min_instances,omitempty,string"`
	MaxInstances   *int   `json:"max_instances,omitempty,string"`
	Port           *int   `json:"port,omitempty,string"`
}

func updateDeployment(ctx context.Context, deploymentName string, fqin string, token string, config config.Config, noWait bool) (serviceUrl string, err error) {
	min := config.MinInstances
	max := config.MaxInstances
	port := config.Port

	body := UpdateDeploymentRequestBody{
		ContainerImage: fqin,
		MinInstances:   &min,
		MaxInstances:   &max,
		Port:           &port,
	}

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

	if (resp.StatusCode == http.StatusUnauthorized) || (resp.StatusCode == http.StatusForbidden) {
		return "", fmt.Errorf("authentication failed: please log in again with 'ops login'")
	}

	// Handle 408 Request Timeout (cancellation)
	if resp.StatusCode == http.StatusRequestTimeout {
		var errResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errResp)
		if msg, ok := errResp["error"]; ok {
			return "", fmt.Errorf("%s", msg)
		}
		return "", fmt.Errorf("deployment was cancelled")
	}

	if resp.StatusCode != http.StatusAccepted {
		return "", fmt.Errorf("Wait a few minutes and try again.")
	}

	var respBody CreateOrUpdateDeploymentResponseBody
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve job_id, cannot watch deployment progress: %v", err)
	}
	resp.Body.Close()

	if noWait {
		return "", nil
	}

	streamReq, err := http.NewRequestWithContext(
		ctx,
		"GET",
		fmt.Sprintf("%s/api/v1/provisioning-jobs/%s/status", config.ControllerBaseUrl, respBody.JobId),
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create provisioning stream request: %v", err)
	}
	streamReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	streamReq.Header.Set("Accept", "text/event-stream")

	streamResp, err := client.Do(streamReq)
	if err != nil {
		return "", fmt.Errorf("failed to connect to provisioning stream: %v", err)
	}
	defer streamResp.Body.Close()

	if (streamResp.StatusCode == http.StatusUnauthorized) || (streamResp.StatusCode == http.StatusForbidden) {
		return "", fmt.Errorf("authentication failed: please log in again with 'ops login'")
	}

	if streamResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to stream provisioning job: status code %v", streamResp.Status)
	}

	scanner := bufio.NewScanner(streamResp.Body)
	var eventType string
	var dataLines []string

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			if eventType == "message" {
				data := strings.Join(dataLines, "\n")
				fmt.Println(data)

				var msgBody ProvisioningJobUpdate
				if err := json.Unmarshal([]byte(data), &msgBody); err == nil && msgBody.ServiceUrl != nil {
					return *msgBody.ServiceUrl, nil
				}

				return data, nil
			}

			eventType = ""
			dataLines = nil
			continue
		}

		if after, ok := strings.CutPrefix(line, "event:"); ok {
			eventType = strings.TrimSpace(after)
			continue
		}

		if after, ok := strings.CutPrefix(line, "data:"); ok {
			dataLines = append(dataLines, strings.TrimSpace(after))
		}
	}

	if err := scanner.Err(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("deployment was cancelled")
		}
		return "", fmt.Errorf("error reading provisioning stream: %v", err)
	}

	if ctx.Err() != nil {
		return "", fmt.Errorf("deployment was cancelled")
	}

	return "", fmt.Errorf("provisioning stream closed before message event")
}

// Update handles the updating of an existing deployment
func Update(ctx context.Context, cmd *cli.Command) error {
	config := cmd.Metadata["config"].(config.Config)

	// Perform creation with auth retry
	return withAuthRetry(ctx, config, func(token string) error {
		return performDeployment(ctx, cmd, token, config, updateDeployment)
	})
}
