package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/manifoldco/promptui"

	"burnmail/internal/api"
	"burnmail/internal/storage"
)

// Classic-view print helpers (the TUI renders through lipgloss styles in
// styles.go instead; fatih/color only ever touches plain stdout).
var (
	cyan   = color.New(color.FgCyan).SprintFunc()
	yellow = color.New(color.FgYellow).SprintFunc()
)

// RunClassic lists the inbox through the promptui selector and prints the
// selected message as plain text.
func RunClassic(accountData *storage.AccountData) error {
	client := api.GetClient()
	client.SetToken(accountData.Token)

	ctx, cancel := context.WithTimeout(context.Background(), api.RequestTimeout)
	defer cancel()

	fmt.Println(cyan("📬 Fetching messages..."))
	messages, err := api.RetryWithBackoff(ctx, func() ([]api.Message, error) {
		return client.GetMessages()
	})
	if err != nil {
		return fmt.Errorf("failed to get messages: %w", err)
	}

	if len(messages) == 0 {
		fmt.Printf("\n%s No messages yet. Your inbox is empty.\n", yellow("📭"))
		return nil
	}

	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "▸ {{ .Subject | cyan }} - from {{ .From.Address | yellow }}",
		Inactive: "  {{ .Subject | cyan }} - from {{ .From.Address | yellow }}",
		Selected: "{{ .Subject | green }}",
	}

	prompt := promptui.Select{
		Label:     "Select an email",
		Items:     messages,
		Templates: templates,
		Size:      10,
	}

	idx, _, err := prompt.Run()
	if err != nil {
		// User aborted the prompt (Ctrl+C / Esc): not a failure.
		return nil
	}

	selectedMessage := messages[idx]

	fmt.Println(cyan("\n📖 Loading message..."))
	fullMessage, err := client.GetMessage(selectedMessage.ID)
	if err != nil {
		return fmt.Errorf("failed to get message: %w", err)
	}

	fmt.Printf("\n%s\n", strings.Repeat("─", 60))
	fmt.Printf("%s: %s\n", cyan("From"), fullMessage.From.Address)
	fmt.Printf("%s: %s\n", cyan("Subject"), fullMessage.Subject)
	fmt.Printf("%s: %s\n", cyan("Date"), fullMessage.CreatedAt.Format("02/01/2006 15:04:05"))
	fmt.Printf("%s\n\n", strings.Repeat("─", 60))

	if fullMessage.Text != "" {
		fmt.Println(fullMessage.Text)
	} else if len(fullMessage.HTML) > 0 {
		fmt.Println(cyan("\n[HTML content - opening in browser...]"))
		openInBrowser(fullMessage)
	}

	fmt.Println()
	return nil
}
