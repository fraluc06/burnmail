package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"burnmail/internal/ui"
)

func viewMessagesTUI(_ *cobra.Command, _ []string) error {
	accountData, err := loadAccount()
	if err != nil {
		return err
	}

	if err := ui.RunInbox(accountData); err != nil {
		return fmt.Errorf("tui error: %w", err)
	}
	return nil
}

func viewMessages(_ *cobra.Command, _ []string) error {
	accountData, err := loadAccount()
	if err != nil {
		return err
	}
	return ui.RunClassic(accountData)
}
