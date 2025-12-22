package prompts

import (
	"fmt"
	"regexp"

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
