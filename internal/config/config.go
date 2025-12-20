package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// These variables are set at build time via -ldflags
var (
	SupabaseURL string
	SupabaseKey string
)

type Config struct {
	ControllerBaseUrl string `yaml:"controllerBaseUrl"`
	SupabaseURL       string `yaml:"-"` // Not exposed in config file
	SupabaseKey       string `yaml:"-"` // Not exposed in config file
	MinInstances      int    `yaml:"minInstances"`
	MaxInstances      int    `yaml:"maxInstances"`
	Port              int    `yaml:"port"`
}

type ConfigFile struct {
	ControllerBaseUrl *string `yaml:"controllerBaseUrl"`
	MinInstances      *int    `yaml:"minInstances"`
	MaxInstances      *int    `yaml:"maxInstances"`
	Port              *int    `yaml:"port"`
}

func LoadConfig() (Config, error) {

	defaultConfig := Config{
		ControllerBaseUrl: "http://34.58.48.78",
		SupabaseURL:       SupabaseURL,
		SupabaseKey:       SupabaseKey,
		MinInstances:      0,
		MaxInstances:      1,
		Port:              8080,
	}

	// Start with default config
	finalConfig := defaultConfig

	// Check for global config file
	homeDir, err := os.UserHomeDir()
	if err == nil {
		globalConfigFile := filepath.Join(homeDir, ".config", "ops", "config.yaml")
		if _, err := os.Stat(globalConfigFile); err == nil {
			if configData, err := os.ReadFile(globalConfigFile); err == nil {
				var globalConfig ConfigFile
				if err := yaml.Unmarshal(configData, &globalConfig); err == nil {
					// Merge global config into final config
					mergeConfig(&finalConfig, globalConfig)
				}
			}
		}
	}

	// Check for local config file in current directory
	localConfigFile := "ops.config.yaml"
	if _, err := os.Stat(localConfigFile); err == nil {
		if configData, err := os.ReadFile(localConfigFile); err == nil {
			var localConfig ConfigFile
			if err := yaml.Unmarshal(configData, &localConfig); err == nil {
				// Merge local config into final config (takes precedence over global)
				mergeConfig(&finalConfig, localConfig)
			}
		}
	}

	// Always set Supabase credentials from build-time variables
	finalConfig.SupabaseURL = SupabaseURL
	finalConfig.SupabaseKey = SupabaseKey

	// Allow env var override only for development/testing
	if supabaseURL := os.Getenv("SUPABASE_URL"); supabaseURL != "" {
		finalConfig.SupabaseURL = supabaseURL
	}
	if supabaseKey := os.Getenv("SUPABASE_KEY"); supabaseKey != "" {
		finalConfig.SupabaseKey = supabaseKey
	}

	// Validate that MinInstances <= MaxInstances
	if finalConfig.MinInstances > finalConfig.MaxInstances {
		return Config{}, fmt.Errorf("configuration error: minInstances (%d) cannot be greater than maxInstances (%d)", finalConfig.MinInstances, finalConfig.MaxInstances)
	}

	return finalConfig, nil
}

// mergeConfig merges src config values into dst, only overwriting explicitly set values
func mergeConfig(dst *Config, src ConfigFile) {
	if src.ControllerBaseUrl != nil {
		dst.ControllerBaseUrl = *src.ControllerBaseUrl
	}
	if src.MinInstances != nil {
		dst.MinInstances = *src.MinInstances
	}
	if src.MaxInstances != nil {
		dst.MaxInstances = *src.MaxInstances
	}
	if src.Port != nil {
		dst.Port = *src.Port
	}
}
