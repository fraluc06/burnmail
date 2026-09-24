package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"burnmail/api"
)

type ExportData struct {
	Account    *exportAccount  `json:"account"`
	Messages   []MessageExport `json:"messages"`
	ExportedAt string          `json:"exportedAt"`
}

// exportAccount mirrors the account fields safe to write to disk. The stored
// password and token are deliberately excluded so the export file can be
// shared or archived without leaking credentials.
type exportAccount struct {
	Address   string `json:"address"`
	AccountID string `json:"accountId"`
	CreatedAt string `json:"createdAt"`
}

type MessageExport struct {
	*api.MessageDetail
	IsIncluded bool `json:"isIncluded"`
}

func exportData(_ *cobra.Command, _ []string) error {
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
		fmt.Printf("\n%s No messages to export. Your inbox is empty.\n", yellow("📭"))
		return nil
	}

	fmt.Printf("%s Found %d messages. Fetching details...\n", cyan("📖"), len(messages))

	exportedMessages := make([]MessageExport, 0, len(messages))
	for i, msg := range messages {
		fmt.Printf("\r%s Fetching message %d/%d...", cyan("⏳"), i+1, len(messages))

		fullMessage, err := client.GetMessage(msg.ID)
		if err != nil {
			fmt.Printf("\n%s Failed to fetch message %s: %v\n", yellow("⚠"), msg.ID, err)
			continue
		}

		exportedMessages = append(exportedMessages, MessageExport{
			MessageDetail: fullMessage,
			IsIncluded:    true,
		})
	}
	fmt.Println() // New line after progress

	exportDataStruct := ExportData{
		Account: &exportAccount{
			Address:   accountData.Address,
			AccountID: accountData.AccountID,
			CreatedAt: accountData.CreatedAt,
		},
		Messages:   exportedMessages,
		ExportedAt: time.Now().Format("02/01/2006, 15:04:05"),
	}

	// Create filename with email address and timestamp
	filename := fmt.Sprintf("burnmail_export_%s_%d.json", strings.ReplaceAll(accountData.Address, "@", "_"), time.Now().Unix())

	jsonData, err := json.MarshalIndent(exportDataStruct, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal export data: %w", err)
	}

	if err := os.WriteFile(filename, jsonData, 0600); err != nil {
		return fmt.Errorf("failed to write export file: %w", err)
	}

	absPath, _ := os.Getwd()
	fullPath := filepath.Join(absPath, filename)

	fmt.Printf("\n%s Export completed successfully!\n", green("✓"))
	fmt.Printf("%s File: %s\n", cyan("💾"), filename)
	fmt.Printf("%s Messages exported: %d\n", cyan("📧"), len(exportedMessages))
	fmt.Printf("%s Full path: %s\n\n", cyan("📍"), fullPath)
	return nil
}
