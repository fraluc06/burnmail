package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"burnmail/internal/api"
)

func viewMessages(_ *cobra.Command, _ []string) error {
	accountData, err := loadAccount()
	if err != nil {
		return err
	}

	client := api.GetClient()
	client.SetToken(accountData.Token)

	messages, err := fetchMessages(client)
	if err != nil {
		return err
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

func viewMessagesTUI(_ *cobra.Command, _ []string) error {
	accountData, err := loadAccount()
	if err != nil {
		return err
	}

	client := api.GetClient()

	if err := runTUI(accountData, client); err != nil {
		return fmt.Errorf("tui error: %w", err)
	}
	return nil
}

func fetchMessages(client *api.Client) ([]api.Message, error) {
	fmt.Println(cyan("📬 Fetching messages..."))

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	messages, err := retryWithBackoff(ctx, func() ([]api.Message, error) {
		return client.GetMessages()
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	return messages, nil
}
