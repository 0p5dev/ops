package ui

import (
	"fmt"
	"time"

	"github.com/manifoldco/promptui"
)

// ShowSpinner displays a simple spinner with a message
func ShowSpinner(message string, fn func() error) error {
	done := make(chan bool)
	var err error

	// Start spinner in a goroutine
	go func() {
		spinner := []string{"|", "/", "-", "\\"}
		i := 0
		for {
			select {
			case <-done:
				return
			default:
				fmt.Printf("\r\033[K%s %s", spinner[i%len(spinner)], message)
				i++
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	// Execute the function
	err = fn()

	// Stop spinner
	done <- true
	fmt.Printf("\r") // Clear the line

	return err
}

// ShowProgress shows a simple progress indicator
func ShowProgress(message string) func() {
	done := make(chan bool)

	go func() {
		spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		for {
			select {
			case <-done:
				fmt.Printf("\r✓ %s\n", message)
				return
			default:
				fmt.Printf("\r%s %s", spinner[i%len(spinner)], message)
				i++
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()

	return func() {
		done <- true
		time.Sleep(100 * time.Millisecond) // Give time for the goroutine to finish
	}
}

// Confirm shows a confirmation prompt using promptui
func Confirm(message string) (bool, error) {
	prompt := promptui.Prompt{
		Label:     message,
		IsConfirm: true,
	}

	result, err := prompt.Run()
	if err != nil {
		return false, err
	}

	return result == "y" || result == "Y", nil
}
