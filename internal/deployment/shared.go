package deployment

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	"github.com/0p5dev/ops/internal/auth"
	"github.com/0p5dev/ops/internal/config"
	prompts "github.com/0p5dev/ops/internal/prompts"
	"github.com/0p5dev/ops/internal/ui"
	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"
)

type ProvisioningJobUpdate struct {
	Id          string  `json:"id"`
	ResourceId  string  `json:"resource_id"`
	Status      string  `json:"status"` // pending | succeeded | failed
	CreatedAt   string  `json:"created_at"`
	CompletedAt *string `json:"completed_at"`
	ServiceUrl  *string `json:"service_url"`
}

type TransmitImageResponse struct {
	Fqin string `json:"fqin"`
}

type CreateOrUpdateDeploymentResponseBody struct {
	JobId   string `json:"job_id"`
	Message string `json:"message"`
}

type deploymentAlreadyExistsError struct {
	deploymentName string
}

func (e *deploymentAlreadyExistsError) Error() string {
	return fmt.Sprintf("deployment '%s' already exists", e.deploymentName)
}

type deploymentNotFoundError struct {
	deploymentName string
}

func (e *deploymentNotFoundError) Error() string {
	return fmt.Sprintf("deployment '%s' not found", e.deploymentName)
}

type deploymentOperation func(ctx context.Context, deploymentName string, fqin string, token string, config config.Config, noWait bool) (serviceUrl string, err error)

// validateDeploymentName validates that a deployment name meets requirements
func validateDeploymentName(name string) error {
	if len(name) < 3 {
		return fmt.Errorf("deployment name must be at least 3 characters")
	}
	if len(name) > 32 {
		return fmt.Errorf("deployment name must be at most 32 characters")
	}

	matched, _ := regexp.MatchString("^[a-z][a-z0-9-]*[a-z0-9]$", name)
	if !matched {
		return fmt.Errorf("deployment name must start with a letter, contain only lowercase letters, numbers, and hyphens, and not end with a hyphen")
	}

	return nil
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

func handleCommonDeploymentHttpErrors(resp *http.Response) (handled bool, err error) {
	if (resp.StatusCode == http.StatusUnauthorized) || (resp.StatusCode == http.StatusForbidden) {
		resp.Body.Close()
		return true, fmt.Errorf("authentication failed: please log in again with 'ops login'")
	}

	if resp.StatusCode == http.StatusRequestTimeout {
		defer resp.Body.Close()
		var errResp map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		if msg, ok := errResp["error"]; ok {
			return true, fmt.Errorf("%s", msg)
		}
		return true, fmt.Errorf("deployment was cancelled")
	}

	return false, nil
}

func handleAcceptedDeploymentResponse(ctx context.Context, client *http.Client, resp *http.Response, controllerBaseURL string, token string, noWait bool) (serviceUrl string, err error) {
	defer resp.Body.Close()

	var respBody CreateOrUpdateDeploymentResponseBody
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve job_id, cannot watch deployment progress: %v", err)
	}

	if noWait {
		return "", nil
	}

	return watchProvisioningJob(ctx, client, controllerBaseURL, respBody.JobId, token)
}

func watchProvisioningJob(ctx context.Context, client *http.Client, controllerBaseURL string, jobID string, token string) (serviceUrl string, err error) {
	streamReq, err := http.NewRequestWithContext(
		ctx,
		"GET",
		fmt.Sprintf("%s/api/v1/provisioning-jobs/%s/status", controllerBaseURL, jobID),
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

func performDeployment(ctx context.Context, cmd *cli.Command, token string, config config.Config, deployOp deploymentOperation) error {
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

	// Create cancellable context and setup signal handling
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n\nReceived interrupt signal. Cancelling deployment...")
		cancel()
		// Don't exit here - let the error flow through naturally
		// This allows us to wait for the controller's cleanup response
	}()

	filename := fmt.Sprintf("%s.tgz", deploymentName)

	// Build container image with spinner
	err = ui.ShowSpinner("Building container image...", func() error {
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

	// Create/update deployment with spinner
	var serviceUrl string
	runDeploy := func(op deploymentOperation) error {
		return ui.ShowSpinner("Deploying...", func() error {
			var deployErr error
			serviceUrl, deployErr = op(ctx, deploymentName, fqin, token, config, cmd.Bool("no-wait"))
			return deployErr
		})
	}

	err = runDeploy(deployOp)
	if err != nil {
		var conflictErr *deploymentAlreadyExistsError
		var notFoundErr *deploymentNotFoundError

		if errors.As(err, &conflictErr) {
			fmt.Printf("A deployment named '%s' already exists.\n", deploymentName)
			shouldUpdate, promptErr := prompts.PromptConfirmation("Would you like to update that deployment instead?")
			if promptErr != nil {
				return fmt.Errorf("failed to confirm update choice: %w", promptErr)
			}
			if !shouldUpdate {
				return err
			}

			err = runDeploy(updateDeployment)
			if err != nil {
				if ctx.Err() != nil {
					return fmt.Errorf("deployment cancelled")
				}
				return fmt.Errorf("failed to deploy: %v", err)
			}
		} else if errors.As(err, &notFoundErr) {
			fmt.Printf("A deployment named '%s' does not exist.\n", deploymentName)
			shouldCreate, promptErr := prompts.PromptConfirmation("Would you like to create that deployment instead?")
			if promptErr != nil {
				return fmt.Errorf("failed to confirm create choice: %w", promptErr)
			}
			if !shouldCreate {
				return err
			}

			err = runDeploy(createDeployment)
			if err != nil {
				if ctx.Err() != nil {
					return fmt.Errorf("deployment cancelled")
				}
				return fmt.Errorf("failed to deploy: %v", err)
			}
		} else {
			// Check if it was cancelled
			if ctx.Err() != nil {
				return fmt.Errorf("deployment cancelled")
			}
			return fmt.Errorf("failed to deploy: %v", err)
		}
	}

	if cmd.Bool("no-wait") {
		fmt.Println("✓ Deployment pending. Run 'ops deployment list' or 'ops deployment describe " + deploymentName + "' to check status.")
	} else {
		fmt.Println("✓ Deployment successful! Your service is available at: ", serviceUrl)
	}

	return nil
}

// isAuthError checks if an error is authentication-related
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	returnstr := err.Error()
	return contains(returnstr, "authentication failed") || contains(returnstr, "unauthorized")
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || findSubstring(s, substr)))
}

// findSubstring finds a substring within a string
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// withAuthRetry executes a function with automatic retry on auth failure
func withAuthRetry(ctx context.Context, config config.Config, fn func(token string) error) error {
	for attempt := 0; attempt < 2; attempt++ {
		token, err := auth.GetBearerToken()
		if err != nil {
			if attempt == 0 {
				fmt.Println("Authentication required. Starting login...")
				if loginErr := auth.PerformLogin(ctx, config); loginErr != nil {
					return fmt.Errorf("failed to login: %w", loginErr)
				}
				continue
			}
			return err
		}

		err = fn(token)
		if err != nil && isAuthError(err) && attempt == 0 {
			fmt.Println("Authentication failed. Starting login...")
			if loginErr := auth.PerformLogin(ctx, config); loginErr != nil {
				return fmt.Errorf("failed to login: %w", loginErr)
			}
			continue
		}
		return err
	}

	return fmt.Errorf("operation failed after retry")
}

func outputJSON[T any](data T) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func outputYAML[T any](data T) error {
	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(2)
	defer encoder.Close()
	return encoder.Encode(data)
}
