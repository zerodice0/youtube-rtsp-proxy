package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const addNewOption = "+ Add new favorite..."

// hasFzf checks if fzf is available in PATH
func hasFzf() bool {
	_, err := exec.LookPath("fzf")
	return err == nil
}

// selectWithFzf uses fzf for interactive selection
func selectWithFzf(items []string, prompt string) (string, error) {
	cmd := exec.Command("fzf", "--prompt", prompt+" ", "--height", "40%", "--reverse")
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}

	go func() {
		defer stdin.Close()
		for _, item := range items {
			fmt.Fprintln(stdin, item)
		}
	}()

	output, err := cmd.Output()
	if err != nil {
		// User cancelled (Ctrl+C or Esc)
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 130 {
			return "", nil
		}
		return "", nil // fzf returns error on cancel
	}

	return strings.TrimSpace(string(output)), nil
}

// selectWithNumber provides a numbered selection fallback
func selectWithNumber(items []string, prompt string) (string, error) {
	fmt.Println(prompt)
	fmt.Println()

	// Print numbered items
	for i, item := range items {
		if item == addNewOption {
			fmt.Println("  ─────────────────────────────")
			fmt.Printf("  a) %s\n", item)
		} else {
			fmt.Printf("  %d) %s\n", i+1, item)
		}
	}
	fmt.Println("  ─────────────────────────────")
	fmt.Println("  0) Cancel")
	fmt.Println()
	fmt.Print("Enter choice: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	input = strings.TrimSpace(input)

	// Handle 'a' for add option
	if strings.ToLower(input) == "a" {
		for _, item := range items {
			if item == addNewOption {
				return addNewOption, nil
			}
		}
	}

	// Handle cancel
	if input == "0" || input == "" {
		return "", nil
	}

	// Parse number
	num, err := strconv.Atoi(input)
	if err != nil || num < 1 || num > len(items) {
		return "", fmt.Errorf("invalid selection: %s", input)
	}

	selected := items[num-1]
	if selected == addNewOption {
		return "", fmt.Errorf("invalid selection")
	}

	return selected, nil
}

// SelectItem provides unified selection interface
// Uses fzf if available, otherwise falls back to numbered selection
func SelectItem(items []string, prompt string) (string, error) {
	if hasFzf() {
		return selectWithFzf(items, prompt)
	}
	return selectWithNumber(items, prompt)
}

// PromptInput prompts user for text input
func PromptInput(prompt string) (string, error) {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}
