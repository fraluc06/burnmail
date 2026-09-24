package ui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"

	"burnmail/internal/api"
)

func loadMessages(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		messages, err := client.GetMessages()
		if err != nil {
			return errMsg(err)
		}
		return messagesLoadedMsg(messages)
	}
}

func loadMessageDetail(client *api.Client, id string) tea.Cmd {
	return func() tea.Msg {
		message, err := client.GetMessage(id)
		if err != nil {
			return errMsg(err)
		}
		_ = client.MarkMessageAsRead(id)
		return messageDetailLoadedMsg(message)
	}
}

func deleteMessage(client *api.Client, id string) tea.Cmd {
	return func() tea.Msg {
		err := client.DeleteMessage(id)
		if err != nil {
			return errMsg(err)
		}
		return messageDeletedMsg{}
	}
}

func bulkDeleteMessages(client *api.Client, ids []string) tea.Cmd {
	return func() tea.Msg {
		// Sequential: one goroutine per id bypasses the client's rate
		// limiter and aborting mid-loop left the remaining deletions
		// running in the background, silently.
		for _, id := range ids {
			if err := client.DeleteMessage(id); err != nil {
				return errMsg(err)
			}
		}
		return bulkDeletedMsg{}
	}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 6
		footerHeight := 4
		availableHeight := msg.Height - headerHeight - footerHeight
		if availableHeight < 5 {
			availableHeight = 5
		}
		m.table.SetHeight(availableHeight)

		m.viewport.SetWidth(msg.Width - 4)
		m.viewport.SetHeight(availableHeight)

		searchWidth := msg.Width - 20
		if searchWidth < 20 {
			searchWidth = 20
		}
		m.searchInput.SetWidth(searchWidth)

		m.updateColumnWidths(msg.Width)

		return m, nil

	case messagesLoadedMsg:
		m.messages = msg
		m.loading = false
		m.err = nil
		m.retryCount = 0
		m.lastUpdate = time.Now()
		saveCache(m.messages)
		m.refreshTable()
		// Do not schedule a tick here: the tick chain started in Init is
		// self-perpetuating via tickMsg. Adding one per load would double the
		// number of active timers on every refresh, hammering the API.
		return m, nil

	case messageDetailLoadedMsg:
		m.selectedMsg = msg
		m.messageDetails[m.selectedMsg.ID] = m.selectedMsg
		m.currentView = detailView
		m.loading = false
		m.updateMessageSeen(m.selectedMsg.ID, true)
		m.viewport.SetContent(m.renderMessageDetail(m.selectedMsg))
		return m, nil

	case bulkDeletedMsg:
		deletedCount := len(m.selectedItems)
		m.statusMessage = fmt.Sprintf("%d messages deleted", deletedCount)

		selectedIDs := make(map[string]bool, deletedCount)
		for idx := range m.selectedItems {
			if idx < len(m.filteredMsgs) {
				selectedIDs[m.filteredMsgs[idx].ID] = true
			}
		}

		newMessages := make([]api.Message, 0, len(m.messages)-deletedCount)
		for _, msg := range m.messages {
			if !selectedIDs[msg.ID] {
				newMessages = append(newMessages, msg)
			}
		}
		m.messages = newMessages

		m.selectedItems = make(map[int]bool)
		m.bulkMode = false
		m.refreshTable()
		saveCache(m.messages)
		return m, nil

	case messageDeletedMsg:
		m.statusMessage = "Message deleted"
		m.currentView = listView
		if m.selectedMsg != nil {
			msgID := m.selectedMsg.ID
			m.selectedMsg = nil
			for i := range m.messages {
				if m.messages[i].ID == msgID {
					m.messages = append(m.messages[:i], m.messages[i+1:]...)
					break
				}
			}
			m.refreshTable()
			saveCache(m.messages)
		}
		return m, nil

	case attachmentDownloadedMsg:
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("Download failed (%s): %v", msg.filename, msg.err)
		} else {
			m.statusMessage = fmt.Sprintf("Downloaded %s", msg.filename)
		}
		return m, nil

	case tickMsg:
		if m.autoRefresh && m.currentView == listView && !m.loading {
			return m, tea.Batch(loadMessages(m.client), tickCmd())
		}
		return m, tickCmd()

	case errMsg:
		m.loading = false
		m.retryCount++

		if m.retryCount < 3 {
			m.statusMessage = fmt.Sprintf("Error (retry %d/3): %v", m.retryCount, msg)
			// Schedule the retry instead of sleeping: Update must never block the UI.
			delay := time.Second * time.Duration(m.retryCount)
			return m, tea.Tick(delay, func(time.Time) tea.Msg { return retryLoadMsg{} })
		}

		m.err = msg
		m.statusMessage = fmt.Sprintf("Error after 3 retries: %v", msg)
		return m, nil

	case retryLoadMsg:
		m.loading = true
		return m, loadMessages(m.client)

	case tea.BackgroundColorMsg:
		m.isDark = msg.IsDark()
		m.applyStyles()
		return m, nil

	case tea.KeyPressMsg:
		if m.currentView == confirmView {
			switch msg.String() {
			case "y", "Y":
				return m.executeConfirmedAction()
			case "n", "N", "esc", "q":
				m.currentView = m.previousView
				m.confirmAction = ""
				m.confirmData = ""
				return m, nil
			}
			return m, nil
		}

		if m.currentView == helpView {
			switch msg.String() {
			case "q", "esc", "?":
				m.currentView = m.previousView
				return m, nil
			}
			return m, nil
		}

		if m.searchMode {
			switch msg.String() {
			case "esc":
				m.searchMode = false
				m.searchInput.Blur()
				return m, nil
			case "enter":
				m.searchMode = false
				m.searchInput.Blur()
				m.refreshTable()
				m.statusMessage = "Refreshing..."
				return m, loadMessages(m.client)
			default:
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				return m, cmd
			}
		}

		switch msg.String() {
		case "/":
			if m.currentView == listView {
				m.searchMode = true
				m.searchInput.Focus()
				return m, nil
			}

		case "a":
			if m.currentView == listView {
				m.autoRefresh = !m.autoRefresh
				if m.autoRefresh {
					m.statusMessage = "Auto-refresh enabled"
				} else {
					m.statusMessage = "Auto-refresh disabled"
				}
				return m, nil
			}

		case "s":
			if m.currentView == listView {
				m.sortBy = (m.sortBy + 1) % 3
				sortNames := []string{"Date", "Sender", "Subject"}
				m.statusMessage = fmt.Sprintf("Sorted by: %s", sortNames[m.sortBy])
				m.sortMessages()
				m.updateTableRows()
				return m, nil
			}

		case "c":
			if m.currentView == listView && len(m.filteredMsgs) > 0 {
				selectedIdx := m.table.Cursor()
				if selectedIdx < len(m.filteredMsgs) {
					_ = clipboard.WriteAll(m.filteredMsgs[selectedIdx].From.Address)
					m.statusMessage = "Email copied to clipboard"
					return m, nil
				}
			} else if m.currentView == detailView && m.selectedMsg != nil {
				_ = clipboard.WriteAll(m.selectedMsg.Text)
				m.statusMessage = "Message copied to clipboard"
				return m, nil
			}

		case "v":
			if m.currentView == listView {
				m.bulkMode = !m.bulkMode
				if !m.bulkMode {
					m.selectedItems = make(map[int]bool)
					m.updateTableRows()
				}
				m.statusMessage = fmt.Sprintf("Bulk mode: %v", m.bulkMode)
				return m, nil
			}

		case "space":
			if m.currentView == listView && m.bulkMode {
				selectedIdx := m.table.Cursor()
				if m.selectedItems[selectedIdx] {
					delete(m.selectedItems, selectedIdx)
				} else {
					m.selectedItems[selectedIdx] = true
				}
				m.updateTableRows()
				return m, nil
			}

		case "d":
			if m.currentView == detailView && m.selectedMsg != nil {
				return m.showConfirm("delete_single", fmt.Sprintf("delete message '%s'", truncate(m.selectedMsg.Subject, 30)))
			} else if m.currentView == listView && m.bulkMode && len(m.selectedItems) > 0 {
				return m.showConfirm("delete_bulk", fmt.Sprintf("delete %d selected messages", len(m.selectedItems)))
			}

		case "enter":
			if m.currentView == listView && len(m.filteredMsgs) > 0 {
				selectedIdx := m.table.Cursor()
				if selectedIdx < len(m.filteredMsgs) {
					msgID := m.filteredMsgs[selectedIdx].ID
					if cached, ok := m.messageDetails[msgID]; ok {
						m.selectedMsg = cached
						m.currentView = detailView
						m.viewport.SetContent(m.renderMessageDetail(cached))
						return m, nil
					}
					m.loading = true
					return m, loadMessageDetail(m.client, msgID)
				}
			}

		case "o":
			if m.currentView == detailView && m.selectedMsg != nil {
				if len(m.selectedMsg.HTML) > 0 {
					openInBrowser(m.selectedMsg)
				}
			}

		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			if m.currentView == detailView && m.selectedMsg != nil && len(m.selectedMsg.Attachments) > 0 {
				idx := int(msg.String()[0] - '1')
				if idx < len(m.selectedMsg.Attachments) {
					att := m.selectedMsg.Attachments[idx]
					m.statusMessage = fmt.Sprintf("Downloading %s...", att.Filename)
					return m, downloadAttachmentCmd(m.client, m.selectedMsg.ID, att)
				}
			}

		case "A":
			if m.currentView == detailView && m.selectedMsg != nil && len(m.selectedMsg.Attachments) > 0 {
				atts := m.selectedMsg.Attachments
				cmds := make([]tea.Cmd, 0, len(atts))
				for _, att := range atts {
					cmds = append(cmds, downloadAttachmentCmd(m.client, m.selectedMsg.ID, att))
				}
				m.statusMessage = fmt.Sprintf("Downloading %d attachments...", len(atts))
				return m, tea.Batch(cmds...)
			}

		case "q", "Q":
			return m.showConfirm("quit", "quit Burnmail?")

		case "?":
			if m.currentView == listView || m.currentView == detailView {
				m.previousView = m.currentView
				m.currentView = helpView
				return m, nil
			}

		case "r", "R":
			if m.currentView == listView {
				m.loading = true
				m.statusMessage = "Refreshing..."
				return m, loadMessages(m.client)
			}

		case "esc":
			if m.currentView == detailView {
				m.currentView = listView
				m.selectedMsg = nil
				return m, nil
			}
		}
	}

	if m.currentView == listView {
		m.table, cmd = m.table.Update(msg)
	} else {
		m.viewport, cmd = m.viewport.Update(msg)
	}

	var spinnerCmd tea.Cmd
	m.spinner, spinnerCmd = m.spinner.Update(msg)

	return m, tea.Batch(cmd, spinnerCmd)
}

func (m *model) showConfirm(action, description string) (tea.Model, tea.Cmd) {
	m.previousView = m.currentView
	m.currentView = confirmView
	m.confirmAction = action
	m.confirmData = description
	return m, nil
}

func (m *model) executeConfirmedAction() (tea.Model, tea.Cmd) {
	m.currentView = m.previousView
	action := m.confirmAction
	m.confirmAction = ""

	switch action {
	case "quit":
		return m, tea.Quit

	case "delete_single":
		if m.selectedMsg != nil {
			m.loading = true
			m.statusMessage = "Deleting message..."
			return m, deleteMessage(m.client, m.selectedMsg.ID)
		}

	case "delete_bulk":
		var ids []string
		for idx := range m.selectedItems {
			if idx < len(m.filteredMsgs) {
				ids = append(ids, m.filteredMsgs[idx].ID)
			}
		}
		m.loading = true
		m.statusMessage = fmt.Sprintf("Deleting %d messages...", len(ids))
		return m, bulkDeleteMessages(m.client, ids)
	}

	return m, nil
}

// downloadAttachmentCmd runs downloadAttachment off the UI goroutine and
// reports the result back through the message loop, so failures surface in
// the status line instead of being silently dropped.
func downloadAttachmentCmd(client *api.Client, messageID string, att api.Attachment) tea.Cmd {
	return func() tea.Msg {
		err := downloadAttachment(client, messageID, att)
		return attachmentDownloadedMsg{filename: att.Filename, err: err}
	}
}
