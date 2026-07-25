package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"

	"burnmail/api"
	"burnmail/storage"
)

func generateEmail(_ *cobra.Command, _ []string) {
	if storage.Exists() {
		existingAccount, _ := storage.Load()
		if existingAccount != nil {
			fmt.Printf("%s Account already exists: %s\n", yellow("⚠"), cyan(existingAccount.Address))
			fmt.Printf("Use '%s' to delete it first.\n", yellow("burnmail d"))
			return
		}
	}

	fmt.Println(cyan("🔍 Fetching available domains..."))

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	client := api.GetClient()

	domains, err := retryWithBackoff(ctx, func() (interface{}, error) {
		return client.GetDomains()
	})
	if err != nil {
		fmt.Printf("%s Failed to get domains: %v\n", red("✗"), err)
		return
	}

	domainList := domains.([]api.Domain)
	if len(domainList) == 0 {
		fmt.Printf("%s No domains available\n", red("✗"))
		return
	}

	var selectedDomain string
	for _, d := range domainList {
		if d.IsActive {
			selectedDomain = d.Domain
			break
		}
	}

	if selectedDomain == "" {
		fmt.Printf("%s No active domains found\n", red("✗"))
		return
	}

	username := generateRandomString(8)
	address := username + "@" + selectedDomain
	password := generateRandomString(16)

	fmt.Println(cyan("📧 Creating email address..."))

	account, err := retryWithBackoff(ctx, func() (interface{}, error) {
		return client.CreateAccount(address, password)
	})
	if err != nil {
		fmt.Printf("%s Failed to create account: %v\n", red("✗"), err)
		return
	}

	token, err := retryWithBackoff(ctx, func() (interface{}, error) {
		return client.Login(address, password)
	})
	if err != nil {
		fmt.Printf("%s Failed to login: %v\n", red("✗"), err)
		return
	}

	accountData := &storage.AccountData{
		Address:   address,
		Password:  password,
		Token:     token.(string),
		AccountID: account.(*api.Account).ID,
		CreatedAt: time.Now().Format("02/01/2006, 15:04:05"),
	}

	if err := storage.Save(accountData); err != nil {
		fmt.Printf("%s Failed to save account: %v\n", red("✗"), err)
		return
	}

	if err := clipboard.WriteAll(address); err == nil {
		fmt.Printf("\n%s Email created and copied to clipboard!\n", green("✓"))
	} else {
		fmt.Printf("\n%s Email created!\n", green("✓"))
		fmt.Printf("%s Warning: Failed to copy to clipboard: %v\n", yellow("⚠"), err)
	}

	fmt.Printf("\n%s\n\n", green(address))
}

func deleteAccount(_ *cobra.Command, _ []string) {
	accountData := loadAccountOrExit()
	if accountData == nil {
		return
	}

	client := api.GetClient()
	client.SetToken(accountData.Token)

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	_, deleteErr := retryWithBackoff(ctx, func() (interface{}, error) {
		return nil, client.DeleteAccount(accountData.AccountID)
	})
	if deleteErr != nil {
		fmt.Printf("%s Failed to delete account from server: %v\n", yellow("⚠"), deleteErr)
	}

	if err := storage.Delete(); err != nil {
		fmt.Printf("%s Failed to delete local data: %v\n", red("✗"), err)
		return
	}

	fmt.Printf("%s Account deleted successfully\n", green("✓"))
}

func showAccount(_ *cobra.Command, _ []string) {
	accountData := loadAccountOrExit()
	if accountData == nil {
		return
	}

	fmt.Printf("\n%s: %s\n", cyan("Email"), accountData.Address)
	fmt.Printf("%s: %s\n\n", cyan("Created At"), accountData.CreatedAt)
}
