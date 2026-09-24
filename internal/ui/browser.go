package ui

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"burnmail/internal/api"
)

const htmlFileCleanupDelay = 30 * time.Second

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
