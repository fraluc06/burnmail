package cmd

import (
	"errors"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"burnmail/api"
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
