package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"

	"burnmail/api"
	"burnmail/storage"
)

func generateEmail(_ *cobra.Command, _ []string) error {
	if storage.Exists() {
		existingAccount, _ := storage.Load()
		if existingAccount != nil {
			fmt.Printf("%s Account already exists: %s\n", yellow("⚠"), cyan(existingAccount.Address))
			fmt.Printf("Use '%s' to delete it first.\n", yellow("burnmail d"))
			return nil
		}
	}

	fmt.Println(cyan("🔍 Fetching available domains..."))

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	client := api.GetClient()

	domains, err := retryWithBackoff(ctx, func() ([]api.Domain, error) {
		return client.GetDomains()
	})
	if err != nil {
		return fmt.Errorf("failed to get domains: %w", err)
	}

	if len(domains) == 0 {
		return errors.New("no domains available")
	}

	var selectedDomain string
	for _, d := range domains {
		if d.IsActive {
			selectedDomain = d.Domain
			break
		}
	}

	if selectedDomain == "" {
		return errors.New("no active domains found")
	}

	username := generateRandomString(8)
	address := username + "@" + selectedDomain
	password := generateRandomString(16)

	fmt.Println(cyan("📧 Creating email address..."))

	account, err := retryWithBackoff(ctx, func() (*api.Account, error) {
		return client.CreateAccount(address, password)
	})
	if err != nil {
		return fmt.Errorf("failed to create account: %w", err)
	}

	token, err := retryWithBackoff(ctx, func() (string, error) {
		return client.Login(address, password)
	})
	if err != nil {
		return fmt.Errorf("failed to login: %w", err)
	}

	accountData := &storage.AccountData{
		Address:   address,
		Password:  password,
		Token:     token,
		AccountID: account.ID,
		CreatedAt: time.Now().Format("02/01/2006, 15:04:05"),
	}

	if err := storage.Save(accountData); err != nil {
		return fmt.Errorf("failed to save account: %w", err)
	}

	if err := clipboard.WriteAll(address); err == nil {
		fmt.Printf("\n%s Email created and copied to clipboard!\n", green("✓"))
	} else {
		fmt.Printf("\n%s Email created!\n", green("✓"))
		fmt.Printf("%s Warning: Failed to copy to clipboard: %v\n", yellow("⚠"), err)
	}

	fmt.Printf("\n%s\n\n", green(address))
	return nil
}

func deleteAccount(_ *cobra.Command, _ []string) error {
	accountData, err := loadAccount()
	if err != nil {
		return err
	}

	client := api.GetClient()
	client.SetToken(accountData.Token)

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	_, deleteErr := retryWithBackoff(ctx, func() (struct{}, error) {
		return struct{}{}, client.DeleteAccount(accountData.AccountID)
	})
	if deleteErr != nil {
		fmt.Printf("%s Failed to delete account from server: %v\n", yellow("⚠"), deleteErr)
	}

	if err := storage.Delete(); err != nil {
		return fmt.Errorf("failed to delete local data: %w", err)
	}

	fmt.Printf("%s Account deleted successfully\n", green("✓"))
	return nil
}

func showAccount(_ *cobra.Command, _ []string) error {
	accountData, err := loadAccount()
	if err != nil {
		return err
	}

	fmt.Printf("\n%s: %s\n", cyan("Email"), accountData.Address)
	fmt.Printf("%s: %s\n\n", cyan("Created At"), accountData.CreatedAt)
	return nil
}
