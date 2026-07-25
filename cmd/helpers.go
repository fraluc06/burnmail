package cmd

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"burnmail/api"
	"burnmail/storage"
)

// loadAccountOrExit loads account data or exits with error message
func loadAccountOrExit() *storage.AccountData {
	accountData, err := storage.Load()
	if err != nil || accountData == nil {
		fmt.Printf("%s No account found. Generate one first with '%s'\n", red("✗"), yellow("burnmail g"))
		return nil
	}
	return accountData
}

// generateRandomString generates a random string of specified length
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	for i, b := range bytes {
		bytes[i] = charset[b%byte(len(charset))]
	}
	return string(bytes)
}

// openInBrowser opens HTML content in the default browser
func openInBrowser(message *api.MessageDetail) {
	tmpFile, err := os.CreateTemp("", "burnmail-*.html")
	if err != nil {
		return
	}
	tmpFilePath := tmpFile.Name()

	var htmlBuilder strings.Builder
	for _, h := range message.HTML {
		htmlBuilder.WriteString(h)
	}

	if _, err := tmpFile.WriteString(htmlBuilder.String()); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFilePath)
		return
	}
	_ = tmpFile.Close()

	go func() {
		time.Sleep(htmlFileCleanupDelay)
		_ = os.Remove(tmpFilePath)
	}()

	var execCmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		execCmd = exec.Command("open", tmpFilePath)
	case "linux":
		execCmd = exec.Command("xdg-open", tmpFilePath)
	case "windows":
		execCmd = exec.Command("cmd", "/c", "start", tmpFilePath)
	default:
		_ = os.Remove(tmpFilePath)
		return
	}

	if err := execCmd.Start(); err != nil {
		_ = os.Remove(tmpFilePath)
	}
}

// retryWithBackoff retries a function with exponential backoff
func retryWithBackoff(ctx context.Context, fn func() (any, error)) (any, error) {
	for attempt := 0; attempt < retryMaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		result, err := fn()
		if err == nil {
			return result, nil
		}

		if attempt == retryMaxAttempts-1 {
			return nil, err
		}

		if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "rate limit") {
			delay := min(time.Duration(math.Pow(2, float64(attempt)))*retryBaseDelay, retryMaxDelay)

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		} else {
			return nil, err
		}
	}

	return nil, fmt.Errorf("max retries exceeded")
}
