package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	ControllerBaseUrl string `json:"controllerBaseUrl"`
	SupabaseURL       string `json:"supabaseUrl"`
	SupabaseKey       string `json:"supabaseKey"`
}

func LoadConfig() Config {
	defaultConfig := Config{
		ControllerBaseUrl: "http://34.58.48.78",
		SupabaseURL:       os.Getenv("SUPABASE_URL"),
		SupabaseKey:       os.Getenv("SUPABASE_KEY"),
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

	// Override with env vars if they're set
	if supabaseURL := os.Getenv("SUPABASE_URL"); supabaseURL != "" {
		customConfig.SupabaseURL = supabaseURL
	}
	if supabaseKey := os.Getenv("SUPABASE_KEY"); supabaseKey != "" {
		customConfig.SupabaseKey = supabaseKey
	}

	return customConfig
}
