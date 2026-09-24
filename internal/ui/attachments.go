package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"burnmail/internal/api"
)

func getDownloadsDir() string {
	var downloadsDir string

	switch runtime.GOOS {
	case "windows":
		userProfile := os.Getenv("USERPROFILE")
		if userProfile == "" {
			var upb strings.Builder
			upb.WriteString(os.Getenv("HOMEDRIVE"))
			upb.WriteString(os.Getenv("HOMEPATH"))
			userProfile = upb.String()
		}
		downloadsDir = filepath.Join(userProfile, "Downloads")

	case "darwin", "linux":
		xdgDownload := os.Getenv("XDG_DOWNLOAD_DIR")
		if xdgDownload != "" {
			downloadsDir = xdgDownload
		} else {
			home := os.Getenv("HOME")
			downloadsDir = filepath.Join(home, "Downloads")
		}

	default:
		downloadsDir, _ = os.Getwd()
	}

	if _, err := os.Stat(downloadsDir); os.IsNotExist(err) {
		downloadsDir, _ = os.Getwd()
	}

	return downloadsDir
}

// safeFilename keeps a remote-provided attachment name inside the downloads
// directory: only its base name is trusted, never a path.
func safeFilename(name string) string {
	base := filepath.Base(name)
	if base == "." || base == ".." {
		return "attachment"
	}
	return base
}

func downloadAttachment(client *api.Client, messageID string, att api.Attachment) error {
	data, err := client.DownloadAttachment(messageID, att.ID)
	if err != nil {
		return err
	}

	filename := safeFilename(att.Filename)

	downloadsDir := getDownloadsDir()
	filePath := filepath.Join(downloadsDir, filename)
	counter := 1
	baseName := strings.TrimSuffix(filename, filepath.Ext(filename))
	ext := filepath.Ext(filename)

	for {
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			break
		}
		filePath = filepath.Join(downloadsDir, fmt.Sprintf("%s_%d%s", baseName, counter, ext))
		counter++
	}

	return os.WriteFile(filePath, data, 0644)
}
