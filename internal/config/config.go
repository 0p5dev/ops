package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	ControllerBaseUrl string `json:"controllerBaseUrl"`
	SupabaseURL       string `json:"-"` // Not exposed in config file
	SupabaseKey       string `json:"-"` // Not exposed in config file
}

func LoadConfig() Config {
	var SupabaseURL, SupabaseKey string

	defaultConfig := Config{
		ControllerBaseUrl: "http://34.58.48.78",
		SupabaseURL:       SupabaseURL,
		SupabaseKey:       SupabaseKey,
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

	customConfig.SupabaseURL = SupabaseURL
	customConfig.SupabaseKey = SupabaseKey

	// Allow env var override only for development/testing
	if supabaseURL := os.Getenv("SUPABASE_URL"); supabaseURL != "" {
		customConfig.SupabaseURL = supabaseURL
	}
	if supabaseKey := os.Getenv("SUPABASE_KEY"); supabaseKey != "" {
		customConfig.SupabaseKey = supabaseKey
	}

	return customConfig
}
