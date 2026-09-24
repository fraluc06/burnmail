package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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
