package cmd

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"

	"burnmail/internal/api"
	"burnmail/internal/storage"
)

// loadAccount loads the stored account data, or returns an error if none exists.
func loadAccount() (*storage.AccountData, error) {
	accountData, err := storage.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load account data: %w", err)
	}
	if accountData == nil {
		return nil, errors.New("no account found, generate one first with 'burnmail g'")
	}
	return accountData, nil
}

// generateRandomString generates a random string of specified length
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	// Reject bytes at or above maxByte to avoid modulo bias: 256 is not a
	// multiple of len(charset), so plain b%len(charset) favors the first
	// 256%len(charset) characters.
	const maxByte = 256 - (256 % len(charset))

	result := make([]byte, 0, length)
	buf := make([]byte, length)
	for len(result) < length {
		if _, err := rand.Read(buf); err != nil {
			return ""
		}
		for _, b := range buf {
			if int(b) >= maxByte {
				continue
			}
			result = append(result, charset[int(b)%len(charset)])
			if len(result) == length {
				break
			}
		}
	}
	return string(result)
}

// fetchMessages prints progress and retries mail.tm rate-limit responses;
// shared by the plain-stdout commands (export). The interactive frontends
// live in internal/ui and fetch on their own.
func fetchMessages(client *api.Client) ([]api.Message, error) {
	fmt.Println(cyan("📬 Fetching messages..."))

	ctx, cancel := context.WithTimeout(context.Background(), api.RequestTimeout)
	defer cancel()

	messages, err := api.RetryWithBackoff(ctx, func() ([]api.Message, error) {
		return client.GetMessages()
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	return messages, nil
}
