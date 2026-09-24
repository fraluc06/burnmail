package cmd

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

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
func retryWithBackoff[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	var zero T
	for attempt := 0; attempt < retryMaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		default:
		}

		result, err := fn()
		if err == nil {
			return result, nil
		}

		if attempt == retryMaxAttempts-1 {
			return zero, err
		}

		if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "rate limit") {
			delay := min(time.Duration(math.Pow(2, float64(attempt)))*retryBaseDelay, retryMaxDelay)

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return zero, ctx.Err()
			}
		} else {
			return zero, err
		}
	}

	return zero, errors.New("max retries exceeded")
}
