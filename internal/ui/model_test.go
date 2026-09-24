package ui

import (
	"errors"
	"testing"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"burnmail/internal/api"
)

func newTestModel() *model {
	m := &model{
		currentView:    listView,
		messageDetails: make(map[string]*api.MessageDetail),
		selectedItems:  make(map[int]bool),
	}
	return m
}

func key(s string) tea.KeyPressMsg {
	if len(s) == 1 {
		return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
	}
	return tea.KeyPressMsg{Code: tea.KeyEscape}
}

// Bug A: after errMsg exhausts retries, a successful reload must clear m.err,
// otherwise View() keeps rendering the error screen forever.
func TestErrClearedOnSuccess(t *testing.T) {
	m := newTestModel()
	m.retryCount = 2
	updated, _ := m.Update(errMsg(errors.New("boom")))
	m = updated.(*model)
	if m.err == nil {
		t.Fatal("expected err to be set after 3rd failure")
	}

	updated, _ = m.Update(messagesLoadedMsg([]api.Message{}))
	m = updated.(*model)
	if m.err != nil {
		t.Fatalf("BUG: err still set after successful reload: %v", m.err)
	}
}

// Bug B: errMsg retry must not block Update with time.Sleep.
func TestErrRetryDoesNotBlock(t *testing.T) {
	m := newTestModel()
	start := time.Now()
	updated, cmd := m.Update(errMsg(errors.New("boom")))
	elapsed := time.Since(start)
	m = updated.(*model)
	if elapsed > 100*time.Millisecond {
		t.Fatalf("BUG: Update blocked for %v", elapsed)
	}
	if cmd == nil {
		t.Fatal("expected a retry cmd")
	}
	if m.retryCount != 1 {
		t.Fatalf("expected retryCount=1, got %d", m.retryCount)
	}
}

// New keys: q opens quit confirmation, y quits, n cancels back.
func TestQuitConfirmFlow(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(key("q"))
	m = updated.(*model)
	if m.currentView != confirmView || m.confirmAction != "quit" {
		t.Fatalf("expected quit confirm, got view=%v action=%q", m.currentView, m.confirmAction)
	}

	_, cmd := m.Update(key("y"))
	if cmd == nil {
		t.Fatal("expected tea.Quit cmd on y")
	}
	if msg := cmd(); msg != (tea.QuitMsg{}) {
		t.Fatalf("expected QuitMsg, got %T", msg)
	}

	m = newTestModel()
	updated, _ = m.Update(key("q"))
	m = updated.(*model)
	updated, _ = m.Update(key("n"))
	m = updated.(*model)
	if m.currentView != listView {
		t.Fatalf("expected back to listView after n, got %v", m.currentView)
	}
}

// Regression for the tick explosion: messagesLoadedMsg must not schedule an
// extra tickCmd. The tick chain started in Init perpetuates itself via
// tickMsg, so scheduling another on every load doubled the number of active
// timers each refresh cycle, multiplying API requests.
func TestMessagesLoadedDoesNotScheduleExtraTick(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(messagesLoadedMsg(nil))
	if cmd != nil {
		t.Fatal("BUG: messagesLoadedMsg scheduled an extra tick cmd")
	}
}

// truncate must not split multibyte runes when returning a byte slice.
func TestTruncateMultibyte(t *testing.T) {
	tests := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{"short enough", "héllo", 10, "héllo"},
		{"fits in runes", "日本", 5, "日本"},
		{"max below 10 slices runes", "héllo", 2, "hé"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.s, tt.max)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.max, got, tt.want)
			}
		})
	}

	long := "日本語のテキストです。これは長い文章です。"
	if got := truncate(long, 12); !utf8.ValidString(got) {
		t.Errorf("truncate produced invalid UTF-8: %q", got)
	}
}

// New keys: ? toggles help, esc returns from detail to list.
func TestHelpAndEsc(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(key("?"))
	m = updated.(*model)
	if m.currentView != helpView || m.previousView != listView {
		t.Fatalf("expected helpView from listView, got view=%v prev=%v", m.currentView, m.previousView)
	}
	updated, _ = m.Update(key("esc"))
	m = updated.(*model)
	if m.currentView != listView {
		t.Fatalf("expected back to listView, got %v", m.currentView)
	}

	m = newTestModel()
	m.currentView = detailView
	m.selectedMsg = &api.MessageDetail{}
	updated, _ = m.Update(key("esc"))
	m = updated.(*model)
	if m.currentView != listView || m.selectedMsg != nil {
		t.Fatalf("expected esc to leave detailView, got view=%v msg=%v", m.currentView, m.selectedMsg)
	}
}
