package deploy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"

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
		return "", fmt.Errorf("status code %v", resp.Status)
	}

	var respBody CreateDeploymentResponseBody
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		return "", fmt.Errorf("failed to decode response body: %v", err)
	}

	return respBody.ServiceUrl, nil
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

	token, err := auth.GetBearerToken()
	if err != nil {
		return err
	}

	deploymentName, err := prompts.PromptName("Deployment Name")
	if err != nil {
		return fmt.Errorf("failed to get deployment name: %w", err)
	}
	filename := fmt.Sprintf("%s.tgz", deploymentName)

	// Build container image with spinner
	err = ui.ShowSpinner("Building container image...", func() error {
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
