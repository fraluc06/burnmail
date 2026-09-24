package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"

	"burnmail/internal/api"
	"burnmail/internal/storage"
)

type view int

const (
	listView view = iota
	detailView
	helpView
	confirmView
)

const (
	autoRefreshInterval = 10 * time.Second
	cacheFileName       = ".burnmail-cache.json"
	cacheExpiry         = 5 * time.Minute
)

type sortMode int

const (
	sortByDate sortMode = iota
	sortBySender
	sortBySubject
)

type messageCache struct {
	Messages  []api.Message `json:"messages"`
	Timestamp time.Time     `json:"timestamp"`
}

type model struct {
	table          table.Model
	viewport       viewport.Model
	searchInput    textinput.Model
	spinner        spinner.Model
	messages       []api.Message
	filteredMsgs   []api.Message
	messageDetails map[string]*api.MessageDetail
	currentView    view
	previousView   view
	selectedMsg    *api.MessageDetail
	width          int
	height         int
	client         *api.Client
	accountData    *storage.AccountData
	loading        bool
	err            error
	retryCount     int
	searchMode     bool
	statusMessage  string
	autoRefresh    bool
	sortBy         sortMode
	selectedItems  map[int]bool
	bulkMode       bool
	confirmAction  string
	confirmData    string
	lastUpdate     time.Time
	isDark         bool
}

type messagesLoadedMsg []api.Message
type messageDetailLoadedMsg *api.MessageDetail
type messageDeletedMsg struct{}
type bulkDeletedMsg struct{}
type attachmentDownloadedMsg struct {
	filename string
	err      error
}
type errMsg error
type tickMsg time.Time
type retryLoadMsg struct{}

var (
	baseStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240"))

	baseStyleFocused = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("#00D9FF"))

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B9D")).
			Bold(true).
			Padding(0, 1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Padding(1, 0, 0, 2)

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00D9FF")).
			Bold(true)

	separatorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555"))

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF87")).
			Padding(0, 0, 0, 2)

	keyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00D9FF")).
			Bold(true)

	descStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CCCCCC"))

	confirmBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FF6B9D")).
			Padding(1, 2).
			Width(50)

	searchBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#555555")).
			Padding(0, 1)

	searchBoxFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("#00D9FF")).
				Padding(0, 1)
)

func initialModel(accountData *storage.AccountData, client *api.Client) *model {
	columns := []table.Column{
		{Title: "✓", Width: 3},
		{Title: "📎", Width: 3},
		{Title: "From", Width: 20},
		{Title: "Subject", Width: 30},
		{Title: "Preview", Width: 20},
		{Title: "Date", Width: 12},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	vp := viewport.New(viewport.WithWidth(100), viewport.WithHeight(20))
	vp.Style = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		PaddingRight(2)

	ti := textinput.New()
	ti.Placeholder = "Search messages (sender, subject, content)..."
	ti.Prompt = " / "
	ti.SetWidth(80)
	ti.CharLimit = 100

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	cached := loadCache()
	var msgs []api.Message
	if cached != nil && time.Since(cached.Timestamp) < cacheExpiry {
		msgs = cached.Messages
	}

	m := &model{
		table:          t,
		viewport:       vp,
		searchInput:    ti,
		spinner:        sp,
		currentView:    listView,
		client:         client,
		accountData:    accountData,
		loading:        len(msgs) == 0,
		autoRefresh:    true,
		filteredMsgs:   msgs,
		messages:       msgs,
		messageDetails: make(map[string]*api.MessageDetail),
		selectedItems:  make(map[int]bool),
		sortBy:         sortByDate,
	}
	m.applyStyles()
	return m
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(
		loadMessages(m.client),
		tickCmd(),
		m.spinner.Tick,
		tea.RequestBackgroundColor,
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(autoRefreshInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

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

func (m *model) View() tea.View {
	var content string
	if m.loading {
		var lb strings.Builder
		lb.WriteString(titleStyle.Render(fmt.Sprintf("%s Loading...", m.spinner.View())))
		lb.WriteString("\n")
		content = lb.String()
	} else if m.err != nil {
		var eb strings.Builder
		eb.WriteString(titleStyle.Render("Error: "))
		eb.WriteString(m.err.Error())
		eb.WriteString("\n")
		content = eb.String()
	} else {
		var s strings.Builder

		if m.currentView != helpView {
			msgCount := len(m.messages)
			msgWord := "messages"
			if msgCount == 1 {
				msgWord = "message"
			}
			title := fmt.Sprintf("Burnmail - %s (%d %s)", m.accountData.Address, msgCount, msgWord)
			s.WriteString(titleStyle.Render(title))
			s.WriteString("\n")

			if m.statusMessage != "" {
				s.WriteString(statusStyle.Render(fmt.Sprintf("▸ %s", m.statusMessage)))
				s.WriteString("\n")
			}
			s.WriteString("\n")
		}

		switch m.currentView {
		case listView:
			var searchBox string
			var tableStyle lipgloss.Style

			if m.searchMode {
				searchBox = searchBoxFocusedStyle.Render(m.searchInput.View())
				tableStyle = baseStyle
			} else {
				searchBox = searchBoxStyle.Render(m.searchInput.View())
				tableStyle = baseStyleFocused
			}

			s.WriteString(searchBox)
			s.WriteString("\n\n")
			s.WriteString(tableStyle.Render(m.table.View()))
			s.WriteString("\n\n")

			sortNames := []string{"Date", "Sender", "Subject"}
			sortInfo := fmt.Sprintf("Sort: %s", sortNames[m.sortBy])
			s.WriteString(helpStyle.Render(sortInfo))
			s.WriteString(" ")
			var hpb strings.Builder
			hpb.WriteString("• Press ")
			hpb.WriteString(keyStyle.Render("?"))
			hpb.WriteString(" for help")
			s.WriteString(helpStyle.Render(hpb.String()))
			s.WriteString("\n")

			var hb strings.Builder
			hb.WriteString(keyStyle.Render("↑/↓"))
			hb.WriteString("/")
			hb.WriteString(keyStyle.Render("j/k"))
			hb.WriteString(":navigate ")
			hb.WriteString(keyStyle.Render("enter"))
			hb.WriteString(":open ")
			hb.WriteString(keyStyle.Render("s"))
			hb.WriteString(":sort ")
			hb.WriteString(keyStyle.Render("c"))
			hb.WriteString(":copy ")
			hb.WriteString(keyStyle.Render("v"))
			hb.WriteString(":bulk ")
			hb.WriteString(keyStyle.Render("r"))
			hb.WriteString(":refresh ")
			hb.WriteString(keyStyle.Render("/"))
			hb.WriteString(":search ")
			hb.WriteString(keyStyle.Render("a"))
			hb.WriteString(":auto:")
			if m.autoRefresh {
				hb.WriteString(keyStyle.Render("ON"))
			} else {
				hb.WriteString(keyStyle.Render("OFF"))
			}
			if m.bulkMode {
				hb.WriteString(" ")
				hb.WriteString(keyStyle.Render("space"))
				hb.WriteString(":select ")
				hb.WriteString(keyStyle.Render("d"))
				hb.WriteString(":delete")
			}
			hb.WriteString(" ")
			hb.WriteString(keyStyle.Render("q"))
			hb.WriteString(":quit")
			s.WriteString(helpStyle.Render(hb.String()))
		case helpView:
			s.WriteString(renderHelpScreen(m.width, m.height))
		case confirmView:
			s.WriteString(renderConfirmDialog(m.confirmData))
		default:
			s.WriteString(baseStyle.Render(m.viewport.View()))
			s.WriteString("\n")
			var dhb strings.Builder
			dhb.WriteString("↑/↓ • ")
			dhb.WriteString(keyStyle.Render("o"))
			dhb.WriteString(":browser • ")
			dhb.WriteString(keyStyle.Render("c"))
			dhb.WriteString(":copy • ")
			dhb.WriteString(keyStyle.Render("d"))
			dhb.WriteString(":delete • esc • ")
			dhb.WriteString(keyStyle.Render("?"))
			dhb.WriteString(":help")
			s.WriteString(helpStyle.Render(dhb.String()))
		}

		content = s.String()
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m *model) filterMessages() {
	if m.searchInput.Value() == "" {
		m.filteredMsgs = m.messages
		return
	}

	query := strings.ToLower(m.searchInput.Value())
	m.filteredMsgs = make([]api.Message, 0, len(m.messages))
	for _, msg := range m.messages {
		fromLower := strings.ToLower(msg.From.Address)
		subjectLower := strings.ToLower(msg.Subject)
		introLower := strings.ToLower(msg.Intro)

		if strings.Contains(fromLower, query) ||
			strings.Contains(subjectLower, query) ||
			strings.Contains(introLower, query) {
			m.filteredMsgs = append(m.filteredMsgs, msg)
		}
	}
}

func (m *model) updateColumnWidths(termWidth int) {
	var newCols []table.Column

	if termWidth < 80 {
		newCols = []table.Column{
			{Title: "✓", Width: 2},
			{Title: "From", Width: 15},
			{Title: "Subject", Width: max(20, termWidth-30)},
			{Title: "Date", Width: 10},
		}
	} else if termWidth < 120 {
		newCols = []table.Column{
			{Title: "✓", Width: 3},
			{Title: "📎", Width: 3},
			{Title: "From", Width: 20},
			{Title: "Subject", Width: max(25, termWidth-55)},
			{Title: "Preview", Width: 15},
			{Title: "Date", Width: 12},
		}
	} else {
		newCols = []table.Column{
			{Title: "✓", Width: 3},
			{Title: "📎", Width: 3},
			{Title: "From", Width: 25},
			{Title: "Subject", Width: max(30, termWidth-85)},
			{Title: "Preview", Width: 25},
			{Title: "Date", Width: 14},
		}
	}

	m.table.SetRows([]table.Row{})
	m.table.SetColumns(newCols)
	m.updateTableRows()
}

func (m *model) updateTableRows() {
	cols := m.table.Columns()
	fromWidth := 25
	subjectWidth := 35
	previewWidth := 25

	for _, col := range cols {
		switch col.Title {
		case "From":
			fromWidth = col.Width
		case "Subject":
			subjectWidth = col.Width
		case "Preview":
			previewWidth = col.Width
		}
	}

	rows := make([]table.Row, 0, len(m.filteredMsgs))
	for i, msg := range m.filteredMsgs {
		checkbox := " "
		if m.selectedItems[i] {
			checkbox = "✓"
		}

		attach := " "
		if msg.HasAttach {
			attach = "📎"
		}

		preview := truncate(msg.Intro, previewWidth)

		if len(cols) == 4 {
			rows = append(rows, table.Row{
				checkbox,
				truncate(msg.From.Address, fromWidth),
				truncate(msg.Subject, subjectWidth),
				msg.CreatedAt.Format("02/01 15:04"),
			})
		} else if len(cols) == 6 {
			rows = append(rows, table.Row{
				checkbox,
				attach,
				truncate(msg.From.Address, fromWidth),
				truncate(msg.Subject, subjectWidth),
				preview,
				msg.CreatedAt.Format("02/01 15:04"),
			})
		}
	}
	m.table.SetRows(rows)
}

func (m *model) sortMessages() {
	switch m.sortBy {
	case sortByDate:
		sort.Slice(m.filteredMsgs, func(i, j int) bool {
			return m.filteredMsgs[i].CreatedAt.After(m.filteredMsgs[j].CreatedAt)
		})
	case sortBySender:
		sort.Slice(m.filteredMsgs, func(i, j int) bool {
			return m.filteredMsgs[i].From.Address < m.filteredMsgs[j].From.Address
		})
	case sortBySubject:
		sort.Slice(m.filteredMsgs, func(i, j int) bool {
			return m.filteredMsgs[i].Subject < m.filteredMsgs[j].Subject
		})
	}
}

func (m *model) updateMessageSeen(id string, seen bool) {
	for i := range m.messages {
		if m.messages[i].ID == id {
			m.messages[i].Seen = seen
			break
		}
	}
	m.refreshTable()
}

func (m *model) refreshTable() {
	m.filterMessages()
	m.sortMessages()
	m.updateTableRows()
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}

	if max < 10 {
		return string(runes[:max])
	}

	const ellipsis = "..."
	words := strings.Fields(s)
	result := strings.Builder{}
	result.Grow(max)
	length := 0

	for _, word := range words {
		w := len([]rune(word))
		if length+w+1 > max-len(ellipsis) {
			break
		}
		if length > 0 {
			result.WriteByte(' ')
			length++
		}
		result.WriteString(word)
		length += w
	}

	if result.Len() == 0 {
		return string(runes[:max-len(ellipsis)]) + ellipsis
	}

	result.WriteString(ellipsis)
	return result.String()
}

func loadCache() *messageCache {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	cacheFile := filepath.Join(homeDir, cacheFileName)
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil
	}

	var cache messageCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil
	}

	return &cache
}

func saveCache(messages []api.Message) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}

	cache := messageCache{
		Messages:  messages,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(cache)
	if err != nil {
		return
	}

	cacheFile := filepath.Join(homeDir, cacheFileName)
	_ = os.WriteFile(cacheFile, data, 0600)
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

func (m *model) renderMessageDetail(msg *api.MessageDetail) string {
	var content strings.Builder
	content.WriteString(headerStyle.Render("From: "))
	content.WriteString(msg.From.Address)
	content.WriteString("\n")
	content.WriteString(headerStyle.Render("Subject: "))
	content.WriteString(msg.Subject)
	content.WriteString("\n")
	content.WriteString(headerStyle.Render("Date: "))
	content.WriteString(msg.CreatedAt.Format("02/01/2006 15:04:05"))
	content.WriteString("\n")
	content.WriteString(separatorStyle.Render(strings.Repeat("─", 80)))
	content.WriteString("\n\n")

	if msg.Text != "" {
		content.WriteString(msg.Text)
	} else if len(msg.HTML) > 0 {
		var htmlBuilder strings.Builder
		for _, h := range msg.HTML {
			htmlBuilder.WriteString(h)
		}
		text := htmlToText(htmlBuilder.String())
		content.WriteString(text)
		content.WriteString("\n\n")
		content.WriteString(separatorStyle.Render(strings.Repeat("─", 80)))
		content.WriteString("\n")
		content.WriteString(descStyle.Render("Press 'o' to open HTML in browser"))
	}

	if len(msg.Attachments) > 0 {
		m.writeAttachments(&content, msg.Attachments)
	}

	return content.String()
}

func (m *model) writeAttachments(content *strings.Builder, attachments []api.Attachment) {
	content.WriteString("\n\n")
	content.WriteString(separatorStyle.Render(strings.Repeat("─", 80)))
	content.WriteString("\n")
	content.WriteString(headerStyle.Render(fmt.Sprintf("📎 Attachments (%d)", len(attachments))))
	content.WriteString("\n\n")
	for i, att := range attachments {
		sizeKB := float64(att.Size) / 1024.0
		_, _ = fmt.Fprintf(content, "%d. %s (%s, %.1f KB)\n",
			i+1,
			keyStyle.Render(att.Filename),
			descStyle.Render(att.ContentType),
			sizeKB)
	}
	content.WriteString("\n")
	content.WriteString(descStyle.Render("Press '1-9' to download attachment, 'shift+a' to download all"))
}

func renderHelpScreen(_, _ int) string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("Burnmail - Help"))
	s.WriteString("\n\n")

	helpSections := []struct {
		title string
		items [][2]string
	}{
		{
			title: "General",
			items: [][2]string{
				{"?", "Show this help screen"},
				{"q", "Quit application (with confirmation)"},
				{"esc", "Go back / Cancel"},
				{"r", "Refresh messages"},
			},
		},
		{
			title: "List View",
			items: [][2]string{
				{"↑/↓ or j/k", "Navigate messages"},
				{"enter", "View selected message"},
				{"/", "Search messages"},
				{"s", "Cycle sort (Date → Sender → Subject)"},
				{"c", "Copy sender email to clipboard"},
				{"a", "Toggle auto-refresh (every 10s)"},
				{"v", "Toggle bulk selection mode"},
				{"space", "Select/deselect message (bulk mode)"},
				{"d", "Delete selected message(s)"},
			},
		},
		{
			title: "Detail View",
			items: [][2]string{
				{"↑/↓ or j/k", "Scroll message content"},
				{"o", "Open HTML content in browser"},
				{"c", "Copy message content to clipboard"},
				{"d", "Delete message"},
				{"1-9", "Download attachment by number"},
				{"shift+a", "Download all attachments"},
				{"esc", "Back to list"},
			},
		},
	}

	for _, section := range helpSections {
		var hb strings.Builder
		hb.WriteString("▸ ")
		hb.WriteString(section.title)
		s.WriteString(headerStyle.Render(hb.String()))
		s.WriteString("\n")
		for _, item := range section.items {
			s.WriteString("  ")
			s.WriteString(keyStyle.Render(fmt.Sprintf("%-10s", item[0])))
			s.WriteString(" ")
			s.WriteString(descStyle.Render(item[1]))
			s.WriteString("\n")
		}
		s.WriteString("\n")
	}

	s.WriteString(helpStyle.Render("Press '?' or 'esc' to close this help"))

	return s.String()
}

func renderConfirmDialog(description string) string {
	var boxContent strings.Builder
	boxContent.WriteString(titleStyle.Render("⚠ Confirmation Required"))
	boxContent.WriteString("\n\n")
	var descb strings.Builder
	descb.WriteString("Are you sure you want to ")
	descb.WriteString(description)
	descb.WriteString("?")
	boxContent.WriteString(descStyle.Render(descb.String()))
	boxContent.WriteString("\n\n")
	boxContent.WriteString(keyStyle.Render("Y"))
	boxContent.WriteString(descStyle.Render(" - Yes, proceed"))
	boxContent.WriteString("\n")
	boxContent.WriteString(keyStyle.Render("N"))
	boxContent.WriteString(descStyle.Render(" - No, cancel"))

	var s strings.Builder
	s.WriteString("\n\n")
	s.WriteString(confirmBoxStyle.Render(boxContent.String()))

	return s.String()
}

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

// downloadAttachmentCmd runs downloadAttachment off the UI goroutine and
// reports the result back through the message loop, so failures surface in
// the status line instead of being silently dropped.
func downloadAttachmentCmd(client *api.Client, messageID string, att api.Attachment) tea.Cmd {
	return func() tea.Msg {
		err := downloadAttachment(client, messageID, att)
		return attachmentDownloadedMsg{filename: att.Filename, err: err}
	}
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

func (m *model) applyStyles() {
	tableStyles := table.DefaultStyles()
	tableStyles.Header = tableStyles.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	tableStyles.Selected = tableStyles.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	m.table.SetStyles(tableStyles)

	tiStyles := textinput.DefaultStyles(m.isDark)
	tiStyles.Focused.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("#00D9FF")).Bold(true)
	tiStyles.Focused.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	tiStyles.Focused.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))
	tiStyles.Blurred.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("#00D9FF")).Bold(true)
	tiStyles.Blurred.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	tiStyles.Blurred.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))
	m.searchInput.SetStyles(tiStyles)
}

func runTUI(accountData *storage.AccountData, client *api.Client) error {
	client.SetToken(accountData.Token)

	p := tea.NewProgram(
		initialModel(accountData, client),
	)

	_, err := p.Run()
	return err
}
