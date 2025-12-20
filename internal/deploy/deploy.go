package deploy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"

	"github.com/0p5dev/ops/internal/auth"
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
	MinInstances   int    `json:"min_instances"`
	MaxInstances   int    `json:"max_instances"`
	Port           int    `json:"port"`
}

type CreateDeploymentResponseBody struct {
	ServiceUrl string `json:"service_url"`
}

func validateDeploymentName(name string) error {
	if len(name) < 3 {
		return fmt.Errorf("deployment name must be at least 3 characters")
	}
	if len(name) > 32 {
		return fmt.Errorf("deployment name must be at most 32 characters")
	}

	matched, _ := regexp.MatchString("^[a-zA-Z][a-zA-Z0-9-]*[a-zA-Z0-9]$", name)
	if !matched {
		return fmt.Errorf("deployment name must start with a letter, contain only letters, numbers, and hyphens, and not end with a hyphen")
	}

	return nil
}

func buildContainerImage(tag string) error {
	cmd := exec.Command("docker", "build", "-t", tag, ".")
	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func saveContainerImage(tag string, filename string) error {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("docker save %s | gzip > %s", tag, filename))
	err := cmd.Run()
	if err != nil {
		return err
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

func createDeployment(deploymentName string, fqin string, token string, config config.Config) (serviceUrl string, err error) {
	body := CreateDeploymentRequestBody{
		Name:           deploymentName,
		ContainerImage: fqin,
		MinInstances:   config.MinInstances,
		MaxInstances:   config.MaxInstances,
		Port:           config.Port,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/api/v1/deployments", config.ControllerBaseUrl), bytes.NewReader(bodyBytes))
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
	defer resp.Body.Close()

	if (resp.StatusCode == http.StatusUnauthorized) || (resp.StatusCode == http.StatusForbidden) {
		return "", fmt.Errorf("authentication failed: please log in again with 'ops login'")
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Wait a few minutes and try again.")
	}

	var respBody CreateDeploymentResponseBody
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		respBody.ServiceUrl = "Unknown (failed to decode response)"
	}

	return respBody.ServiceUrl, nil
}

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

	// Try deployment with auto-login on auth failure
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

		// Check if destroy flag is set
		if cmd.Bool("destroy") {
			confirmed, err := prompts.PromptConfirmation(fmt.Sprintf("Are you sure you want to destroy deployment '%s'", deploymentName))
			if err != nil {
				return fmt.Errorf("confirmation prompt failed: %w", err)
			}
			if !confirmed {
				fmt.Println("Deployment destruction cancelled")
				return nil
			}

			err = ui.ShowSpinner("Destroying deployment...", func() error {
				return destroyDeployment(deploymentName, token, config)
			})
			if err != nil && isAuthError(err) && attempt == 0 {
				fmt.Println("Authentication failed. Starting login...")
				if loginErr := auth.PerformLogin(ctx, config); loginErr != nil {
					return fmt.Errorf("failed to login: %w", loginErr)
				}
				continue
			}
			if err != nil {
				return fmt.Errorf("failed to destroy deployment: %v", err)
			}
			fmt.Printf("✓ Deployment '%s' destroyed successfully\n", deploymentName)
			return nil
		}

		confirmed, err := prompts.PromptConfirmation(fmt.Sprintf("Are you sure you want to create deployment '%s'", deploymentName))
		if err != nil {
			return fmt.Errorf("confirmation prompt failed: %w", err)
		}
		if !confirmed {
			fmt.Println("Deployment creation cancelled")
			return nil
		}

		err = performDeployment(ctx, deploymentName, token, config)
		if err != nil && isAuthError(err) && attempt == 0 {
			fmt.Println("Authentication failed. Starting login...")
			if loginErr := auth.PerformLogin(ctx, config); loginErr != nil {
				return fmt.Errorf("failed to login: %w", loginErr)
			}
			continue
		}
		return err
	}

	return fmt.Errorf("deployment failed after retry")
}

func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	returnstr := err.Error()
	return contains(returnstr, "authentication failed") || contains(returnstr, "unauthorized")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func performDeployment(ctx context.Context, deploymentName string, token string, config config.Config) error {
	filename := fmt.Sprintf("%s.tgz", deploymentName)

	// Build container image with spinner
	err := ui.ShowSpinner("Building container image...", func() error {
		return buildContainerImage(deploymentName)
	})
	if err != nil {
		return fmt.Errorf("failed to build container image: %v", err)
	}
	fmt.Println("✓ Container image built successfully")

	// Save container image with progress indicator
	stopProgress := ui.ShowProgress("Saving and compressing container image...")
	err = saveContainerImage(deploymentName, filename)
	stopProgress()
	if err != nil {
		return fmt.Errorf("failed to save container image: %v", err)
	}

	// Transmit image with progress indicator
	stopProgress = ui.ShowProgress("Uploading container image...")
	fqin, err := transmitCompressedImage(filename, token, config.ControllerBaseUrl)
	stopProgress()
	if err != nil {
		return fmt.Errorf("failed to transmit compressed image: %v", err)
	}

	// Clean up the compressed image file after successful transmission
	if err := os.Remove(filename); err != nil {
		// Log the error but don't fail the deployment
		fmt.Printf("Warning: failed to delete temporary file %s: %v\n", filename, err)
	}

	// Create deployment with spinner
	var serviceUrl string
	err = ui.ShowSpinner("Creating deployment...", func() error {
		var createErr error
		serviceUrl, createErr = createDeployment(deploymentName, fqin, token, config)
		return createErr
	})
	if err != nil {
		return fmt.Errorf("failed to create deployment: %v", err)
	}

	fmt.Println("Deployment successful! Your service is available at: ", serviceUrl)

	return nil
}
