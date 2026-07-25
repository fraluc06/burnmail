package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"burnmail/api"
)

func viewMessages(_ *cobra.Command, _ []string) {
	accountData := loadAccountOrExit()
	if accountData == nil {
		return
	}

	client := api.GetClient()
	client.SetToken(accountData.Token)

	messages, success := fetchMessages(client)
	if !success {
		return
	}

	if len(messages) == 0 {
		fmt.Printf("\n%s No messages yet. Your inbox is empty.\n", yellow("📭"))
		return
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
		return
	}

	selectedMessage := messages[idx]

	fmt.Println(cyan("\n📖 Loading message..."))
	fullMessage, err := client.GetMessage(selectedMessage.ID)
	if err != nil {
		fmt.Printf("%s Failed to get message: %v\n", red("✗"), err)
		return
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
}

func viewMessagesTUI(_ *cobra.Command, _ []string) {
	accountData := loadAccountOrExit()
	if accountData == nil {
		return
	}

	client := api.GetClient()

	if err := runTUI(accountData, client); err != nil {
		fmt.Printf("%s TUI error: %v\n", red("✗"), err)
	}
}

func fetchMessages(client *api.Client) ([]api.Message, bool) {
	fmt.Println(cyan("📬 Fetching messages..."))

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	result, err := retryWithBackoff(ctx, func() (interface{}, error) {
		return client.GetMessages()
	})
	if err != nil {
		fmt.Printf("%s Failed to get messages: %v\n", red("✗"), err)
		return nil, false
	}

	messages := result.([]api.Message)

	if len(messages) == 0 {
		return messages, true
	}

	return messages, true
}
