package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	prompts "github.com/0p5dev/ops/internal/prompts"
	"github.com/urfave/cli/v3"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

func Login(ctx context.Context, cmd *cli.Command) error {
	controllerBaseUrl := cmd.Metadata["controllerBaseUrl"].(string)
	// println("Controller Base URL:", controllerBaseUrl)

	username, err := prompts.PromptUsername("Enter your 0p5.dev username:")
	if err != nil {
		return err
	}

	password, err := prompts.PromptPassword("Enter your 0p5.dev password:")
	if err != nil {
		return err
	}

	loginData := LoginRequest{
		Username: username,
		Password: password,
	}
	jsonData, err := json.Marshal(loginData)
	if err != nil {
		return err
	}
	resp, err := http.Post(fmt.Sprintf("%s/api/v1/auth/login", controllerBaseUrl), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var loginResp LoginResponse
	err = json.Unmarshal(body, &loginResp)
	if err != nil {
		return err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	configDir := filepath.Join(homeDir, ".config", "ops")
	err = os.MkdirAll(configDir, 0755)
	if err != nil {
		return err
	}
	credentialsFile := filepath.Join(configDir, "credentials.json")
	tokenData := map[string]string{"token": loginResp.Token}
	tokenJSON, err := json.Marshal(tokenData)
	if err != nil {
		return err
	}
	err = os.WriteFile(credentialsFile, tokenJSON, 0600)
	if err != nil {
		return err
	}

	fmt.Println("Login successful. Credentials saved to", credentialsFile)

	return nil
}

func GetBearerToken() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	credentialsFile := filepath.Join(homeDir, ".config", "ops", "credentials.json")
	if _, err := os.Stat(credentialsFile); os.IsNotExist(err) {
		return "", fmt.Errorf("credentials not found, run 'ops login' to reauthenticate")
	}
	credentialsData, err := os.ReadFile(credentialsFile)
	if err != nil {
		return "", err
	}
	var loginResp LoginResponse
	err = json.Unmarshal(credentialsData, &loginResp)
	if err != nil {
		return "", err
	}
	return loginResp.Token, nil
}
