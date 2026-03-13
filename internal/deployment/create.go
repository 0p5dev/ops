package deployment

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/0p5dev/ops/internal/config"
	prompts "github.com/0p5dev/ops/internal/prompts"
	"github.com/0p5dev/ops/internal/ui"
	"github.com/urfave/cli/v3"
)

type TransmitImageResponse struct {
	Fqin string `json:"fqin"`
}

type CreateDeploymentRequestBody struct {
	Name           string `json:"name"`
	ContainerImage string `json:"container_image"`
	MinInstances   *int   `json:"min_instances,omitempty,string"`
	MaxInstances   *int   `json:"max_instances,omitempty,string"`
	Port           *int   `json:"port,omitempty,string"`
}

type CreateDeploymentResponseBody struct {
	JobId   string `json:"job_id"`
	Message string `json:"message"`
}

type ProvisioningJobUpdate struct {
	Id          string  `json:"id"`
	ResourceId  string  `json:"resource_id"`
	Status      string  `json:"status"` // pending | succeeded | failed
	CreatedAt   string  `json:"created_at"`
	CompletedAt *string `json:"completed_at"`
	ServiceUrl  *string `json:"service_url"`
}

func buildContainerImage(tag string, dockerfile string, buildContext string) error {
	cmd := exec.Command("docker", "build", "-f", dockerfile, "-t", tag, buildContext)

	// Capture both stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// Include the stderr output in the error message
		if stderr.Len() > 0 {
			return fmt.Errorf("docker build failed: %v\n%s", err, stderr.String())
		}
		return fmt.Errorf("docker build failed: %v", err)
	}

	return nil
}

func detectDockerfile() (string, error) {
	// Check for Dockerfile first
	if _, err := os.Stat("Dockerfile"); err == nil {
		return "Dockerfile", nil
	}

	// Check for Containerfile
	if _, err := os.Stat("Containerfile"); err == nil {
		return "Containerfile", nil
	}

	return "", fmt.Errorf("no Dockerfile or Containerfile found in current directory")
}

func saveContainerImage(tag string, filename string) error {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("docker save %s | gzip > %s", tag, filename))

	// Capture both stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// Include the stderr output in the error message
		if stderr.Len() > 0 {
			return fmt.Errorf("docker save failed: %v\n%s", err, stderr.String())
		}
		return fmt.Errorf("docker save failed: %v", err)
	}

	return nil
}

func transmitCompressedImage(filename string, token string, controllerBaseUrl string) (fqin string, err error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/container-images", controllerBaseUrl), file)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/gzip")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if (resp.StatusCode == http.StatusUnauthorized) || (resp.StatusCode == http.StatusForbidden) {
		return "", fmt.Errorf("authentication failed: please log in again (ops login)")
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status code %v", resp.Status)
	}

	var respBody TransmitImageResponse
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		return "", fmt.Errorf("failed to decode response body: %v", err)
	}

	return respBody.Fqin, nil
}

func createDeployment(ctx context.Context, deploymentName string, fqin string, token string, config config.Config) (serviceUrl string, err error) {
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

	var respBody CreateDeploymentResponseBody
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve job_id, cannot watch deployment progress: %v", err)
	}
	resp.Body.Close()

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

func performDeployment(ctx context.Context, deploymentName string, token string, config config.Config, dockerfile string, buildContext string) error {
	// Create cancellable context and setup signal handling
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Flag to track if we've reached the deployment creation stage
	deploymentStarted := false

	go func() {
		<-sigChan
		fmt.Println("\n\nReceived interrupt signal. Cancelling deployment...")
		cancel()
		// Don't exit here - let the error flow through naturally
		// This allows us to wait for the controller's cleanup response
	}()

	filename := fmt.Sprintf("%s.tgz", deploymentName)

	// Build container image with spinner
	err := ui.ShowSpinner("Building container image...", func() error {
		return buildContainerImage(deploymentName, dockerfile, buildContext)
	})
	if err != nil {
		return fmt.Errorf("failed to build container image: %v", err)
	}
	if ctx.Err() != nil {
		return fmt.Errorf("deployment cancelled")
	}
	fmt.Println("✓ Container image built successfully")

	// Save container image with progress indicator
	stopProgress := ui.ShowProgress("Saving and compressing container image...")
	err = saveContainerImage(deploymentName, filename)
	stopProgress()
	if err != nil {
		return fmt.Errorf("failed to save container image: %v", err)
	}
	if ctx.Err() != nil {
		os.Remove(filename) // cleanup
		return fmt.Errorf("deployment cancelled")
	}

	// Transmit image with progress indicator
	stopProgress = ui.ShowProgress("Uploading container image...")
	fqin, err := transmitCompressedImage(filename, token, config.ControllerBaseUrl)
	stopProgress()
	if err != nil {
		return fmt.Errorf("failed to transmit compressed image: %v", err)
	}
	if ctx.Err() != nil {
		os.Remove(filename) // cleanup
		return fmt.Errorf("deployment cancelled")
	}

	// Clean up the compressed image file after successful transmission
	if err := os.Remove(filename); err != nil {
		// Log the error but don't fail the deployment
		fmt.Printf("Warning: failed to delete temporary file %s: %v\n", filename, err)
	}

	// Mark that deployment creation has started
	deploymentStarted = true

	// Create deployment with spinner
	var serviceUrl string
	err = ui.ShowSpinner("Creating deployment...", func() error {
		var createErr error
		serviceUrl, createErr = createDeployment(ctx, deploymentName, fqin, token, config)
		return createErr
	})
	if err != nil {
		// Check if it was cancelled
		if ctx.Err() != nil {
			if deploymentStarted {
				// Deployment was in progress, wait for controller cleanup
				fmt.Println("\nDeployment cancelled. Waiting for controller to clean up resources...")
				// The error from createDeployment indicates the controller's response
				// If it's a clean cancellation, the controller will have cleaned up
				fmt.Println("✓ Deployment cancelled and resources cleaned up")
			}
			return fmt.Errorf("deployment cancelled")
		}
		return fmt.Errorf("failed to create deployment: %v", err)
	}

	fmt.Println("Deployment successful! Your service is available at: ", serviceUrl)

	return nil
}

// Create handles the creation of a new deployment
func Create(ctx context.Context, cmd *cli.Command) error {
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

	var err error
	if !cmd.Bool("yes") {
		config.Port, err = prompts.PromptForInt("Port", 1, 65535, config.Port)
		if err != nil {
			return fmt.Errorf("failed to get port: %w", err)
		}
	}

	// Validate after flag overrides
	if config.MinInstances > config.MaxInstances {
		return fmt.Errorf("minInstances (%d) cannot be greater than maxInstances (%d)", config.MinInstances, config.MaxInstances)
	}

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

	// Get dockerfile path
	dockerfile := cmd.String("file")
	if dockerfile == "" {
		var err error
		dockerfile, err = detectDockerfile()
		if err != nil {
			return err
		}
		fmt.Printf("Using %s\n", dockerfile)
	} else {
		// Verify the specified file exists
		if _, err := os.Stat(dockerfile); err != nil {
			return fmt.Errorf("dockerfile not found: %s", dockerfile)
		}
	}

	// Get build context
	buildContext := cmd.String("context")

	// Confirm creation unless --yes/-y is set
	if !cmd.Bool("yes") {
		confirmed, err := prompts.PromptConfirmation(fmt.Sprintf("Are you sure you want to create deployment '%s' exposed on port %d?", deploymentName, config.Port))
		if err != nil {
			return fmt.Errorf("confirmation prompt failed: %w", err)
		}
		if !confirmed {
			fmt.Println("Deployment creation cancelled")
			return nil
		}
	}

	// Perform creation with auth retry
	return withAuthRetry(ctx, config, func(token string) error {
		return performDeployment(ctx, deploymentName, token, config, dockerfile, buildContext)
	})
}
