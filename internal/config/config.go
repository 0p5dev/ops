package config

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

type Config struct {
	ControllerBaseUrl string `json:"controllerBaseUrl"`
	SupabaseURL       string `json:"-"` // Not exposed in config file
	SupabaseKey       string `json:"-"` // Not exposed in config file
}

func LoadConfig() Config {
	defaultConfig := Config{
		ControllerBaseUrl: "http://34.58.48.78",
	}

	// Populate Supabase credentials for default config
	creds, err := getSupabaseCredentials(defaultConfig.ControllerBaseUrl)
	if err == nil {
		defaultConfig.SupabaseURL = creds.SupabaseUrl
		defaultConfig.SupabaseKey = creds.SupabaseAnonPublicKey
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return defaultConfig
	}

	configFile := filepath.Join(homeDir, ".config", "ops", "config.json")

	// Check if config file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return defaultConfig
	}

	// Read and parse config file
	configData, err := os.ReadFile(configFile)
	if err != nil {
		return defaultConfig
	}

	var customConfig Config
	if err := json.Unmarshal(configData, &customConfig); err != nil {
		return defaultConfig
	}

	// Get Supabase credentials from the controller
	creds, err = getSupabaseCredentials(customConfig.ControllerBaseUrl)
	if err == nil {
		customConfig.SupabaseURL = creds.SupabaseUrl
		customConfig.SupabaseKey = creds.SupabaseAnonPublicKey
	}

	// Allow env var override only for development/testing
	if supabaseURL := os.Getenv("SUPABASE_URL"); supabaseURL != "" {
		customConfig.SupabaseURL = supabaseURL
	}
	if supabaseKey := os.Getenv("SUPABASE_KEY"); supabaseKey != "" {
		customConfig.SupabaseKey = supabaseKey
	}

	return customConfig
}

type SupabaseCredentials struct {
	SupabaseUrl           string `json:"supabase_url"`
	SupabaseAnonPublicKey string `json:"supabase_anon_public_key"`
}

func getSupabaseCredentials(controllerBaseUrl string) (SupabaseCredentials, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/auth/supabase-credentials", controllerBaseUrl), nil)
	if err != nil {
		return SupabaseCredentials{}, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return SupabaseCredentials{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return SupabaseCredentials{}, fmt.Errorf("failed to get supabase credentials, status code: %d", resp.StatusCode)
	}

	var creds SupabaseCredentials
	if err := json.NewDecoder(resp.Body).Decode(&creds); err != nil {
		return SupabaseCredentials{}, err
	}

	return creds, nil
}
