package cmd

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGetDownloadsDirXDG(t *testing.T) {
	switch runtime.GOOS {
	case "darwin", "linux":
		dir := t.TempDir()
		t.Setenv("XDG_DOWNLOAD_DIR", dir)
		if got := getDownloadsDir(); got != dir {
			t.Errorf("getDownloadsDir() = %q, want %q", got, dir)
		}
	default:
		t.Skip("XDG_DOWNLOAD_DIR handling is unix-only")
	}
}

func TestSafeFilename(t *testing.T) {
	downloadsDir := t.TempDir()

	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{"plain", "report.pdf", "report.pdf"},
		{"unix traversal", "../../etc/passwd", "passwd"},
		{"absolute path", "/etc/shadow", "shadow"},
		{"dot dot", "..", "attachment"},
		{"dot", ".", "attachment"},
		{"empty", "", "attachment"},
	}
	if runtime.GOOS == "windows" {
		// Windows treats backslash as a separator too.
		tests = append(tests, struct {
			name     string
			input    string
			contains string
		}{"windows traversal", `..\..\..\Windows\System32\evil.exe`, "evil.exe"})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := safeFilename(tt.input)
			if got != tt.contains {
				t.Errorf("safeFilename(%q) = %q, want %q", tt.input, got, tt.contains)
			}
			full := filepath.Join(downloadsDir, got)
			if !strings.HasPrefix(full, downloadsDir+string(filepath.Separator)) {
				t.Errorf("safeFilename(%q): joined path %q escapes downloads dir", tt.input, full)
			}
		})
	}
}
