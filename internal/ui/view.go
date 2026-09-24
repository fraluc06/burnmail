package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"burnmail/internal/api"
)

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
