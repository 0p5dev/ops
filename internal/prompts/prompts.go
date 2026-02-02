package prompts

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/manifoldco/promptui"
)

func validateProjectName(input string) error {
	if len(input) < 3 {
		return fmt.Errorf("project name must be at least 3 characters")
	}
	if len(input) > 32 {
		return fmt.Errorf("project name must be at most 32 characters")
	}

	matched, _ := regexp.MatchString("^[a-zA-Z][a-zA-Z0-9-]*[a-zA-Z0-9]$", input)
	if !matched {
		return fmt.Errorf("project name must start with a letter, contain only letters, numbers, and hyphens, and not end with a hyphen")
	}

	return nil
}

func PromptName(label string) (string, error) {
	prompt := promptui.Prompt{
		Label:    label,
		Validate: validateProjectName,
	}

	result, err := prompt.Run()
	if err != nil {
		return "", fmt.Errorf("prompt failed %v", err)
	}
	return result, nil
}

func PromptConfirmation(message string) (bool, error) {
	prompt := promptui.Prompt{
		Label:     message,
		IsConfirm: true,
	}

	result, err := prompt.Run()
	if err != nil {
		// User declined or cancelled
		return false, nil
	}
	return result == "y" || result == "Y", nil
}

func PromptProviderSelection() (string, error) {
	prompt := promptui.Select{
		Label: "Select authentication provider",
		Items: []string{"GitHub", "Google"},
	}
	idx, _, err := prompt.Run()
	if err != nil {
		return "", fmt.Errorf("prompt failed %v", err)
	}
	// Return lowercase provider for use in URL
	if idx == 0 {
		return "github", nil
	} else if idx == 1 {
		return "google", nil
	}
	return "", fmt.Errorf("invalid provider selection")
}

// PromptForInt prompts the user for an integer value within a range
func PromptForInt(label string, min, max, defaultValue int) (int, error) {
	validate := func(input string) error {
		val, err := strconv.Atoi(input)
		if err != nil {
			return fmt.Errorf("please enter a valid number")
		}
		if val < min {
			return fmt.Errorf("value must be at least %d", min)
		}
		if val > max {
			return fmt.Errorf("value must be at most %d", max)
		}
		return nil
	}

	// Use 8080 as default for port, otherwise use min
	if label == "Port" {
		// Ensure the default is within the valid range
		if defaultValue < min {
			defaultValue = min
		}
		if defaultValue > max {
			defaultValue = max
		}
	}

	prompt := promptui.Prompt{
		Label:    fmt.Sprintf("%s (%d-%d)", label, min, max),
		Validate: validate,
		Default:  strconv.Itoa(defaultValue),
	}

	result, err := prompt.Run()
	if err != nil {
		return 0, fmt.Errorf("prompt failed: %v", err)
	}

	return strconv.Atoi(result)
}
