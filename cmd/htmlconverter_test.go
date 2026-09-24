package cmd

import (
	"strings"
	"testing"
)

func TestHTMLToText(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{
			name: "paragraph appears exactly once",
			html: "<p>Hello</p>",
			want: "Hello",
		},
		{
			name: "nested elements not duplicated",
			html: "<html><body><p>Hello</p></body></html>",
			want: "Hello",
		},
		{
			name: "link keeps text and href",
			html: `<a href="https://example.com">Example</a>`,
			want: "[Example](https://example.com)",
		},
		{
			name: "script content is skipped",
			html: "<p>hi</p><script>var x=1;</script>",
			want: "hi",
		},
		{
			name: "style content is skipped",
			html: "<style>body{color:red}</style><p>hi</p>",
			want: "hi",
		},
		{
			name: "bold emphasis",
			html: "<p>a <strong>b</strong> c</p>",
			want: "a *b* c",
		},
		{
			name: "heading decoration",
			html: "<h2>Title</h2>",
			want: "=== Title ===",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := htmlToText(tt.html)
			if got != tt.want {
				t.Errorf("htmlToText(%q) = %q, want %q", tt.html, got, tt.want)
			}
			if n := strings.Count(got, "\n\n\n"); n > 0 {
				t.Errorf("htmlToText(%q) produced runaway blank lines: %q", tt.html, got)
			}
		})
	}
}

func TestHTMLToTextTable(t *testing.T) {
	html := "<table><tr><th>Name</th><th>Qty</th></tr><tr><td>Apple</td><td>3</td></tr></table>"
	got := htmlToText(html)
	for _, want := range []string{"Name", "Qty", "Apple", "3"} {
		if n := strings.Count(got, want); n != 1 {
			t.Errorf("cell %q appears %d times in table output, want 1:\n%s", want, n, got)
		}
	}
}

func TestPadString(t *testing.T) {
	if got := padString("héllo", 3); got != "hél" {
		t.Errorf("padString(%q, 3) = %q, want %q", "héllo", got, "hél")
	}
	got := padString("日本", 5)
	if len([]rune(got)) != 5 {
		t.Errorf("padString(%q, 5) = %q, want 5 runes", "日本", got)
	}
}
