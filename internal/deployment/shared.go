package deployment

import (
	"context"
	"fmt"
	"regexp"

	"github.com/0p5dev/ops/internal/auth"
	"github.com/0p5dev/ops/internal/config"
)

// validateDeploymentName validates that a deployment name meets requirements
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
