package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	ControllerBaseUrl string `json:"controllerBaseUrl"`
}

func LoadConfig() Config {
	defaultConfig := Config{
		ControllerBaseUrl: "http://34.58.48.78",
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

	return customConfig
}
