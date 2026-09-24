package cmd

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// htmlToText converts HTML to formatted plain text
// Preserves structure with:
// - Paragraph breaks
// - List formatting (• bullets)
// - Headings with visual separation
// - Link format [text](url)
// - Tables in text format
// - Bold/Strong emphasis with *text*
func htmlToText(htmlStr string) string {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return htmlStr // Fallback to raw HTML if parsing fails
	}

	converter := &htmlConverter{
		buf: &bytes.Buffer{},
	}
	converter.traverse(doc)
	return strings.TrimSpace(converter.buf.String())
}

type htmlConverter struct {
	buf         *bytes.Buffer
	lastNewline bool
	glued       bool // suppress the space before the next inline write
	inTable     bool
	tableRow    []string
	table       [][]string
}

func (c *htmlConverter) traverse(n *html.Node) {
	if n == nil {
		return
	}

	// Element children are traversed by handleElement itself: most cases
	// call traverseChildren, and the ones that don't (script, style, td...)
	// deliberately suppress or replace them. Re-traversing here would emit
	// every node twice.
	switch n.Type {
	case html.TextNode:
		c.addText(n.Data)
	case html.ElementNode:
		c.handleElement(n)
	case html.DocumentNode:
		c.traverseChildren(n)
	}
}

func (c *htmlConverter) handleElement(n *html.Node) {
	switch n.DataAtom {
	case atom.P:
		c.ensureNewline()
		c.traverseChildren(n)
		c.ensureNewline()

	case atom.Br:
		c.buf.WriteString("\n")
		c.lastNewline = true
		c.glued = false

	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		c.ensureNewline()
		c.writeOpen("=== ")
		c.traverseChildren(n)
		c.writeClose(" ===")
		c.buf.WriteString("\n")
		c.lastNewline = true

	case atom.Strong, atom.B:
		c.writeOpen("*")
		c.traverseChildren(n)
		c.writeClose("*")

	case atom.Em, atom.I:
		c.writeOpen("_")
		c.traverseChildren(n)
		c.writeClose("_")

	case atom.A:
		text := c.extractText(n)
		href := c.getAttr(n, "href")
		if href != "" {
			c.writeOpen("[")
			c.addText(text)
			// "](" glues both ways: no space after the link text and
			// none before the URL that follows.
			c.buf.WriteString("](")
			c.lastNewline = false
			c.glued = true
			c.addText(href)
			c.writeClose(")")
		} else {
			c.addText(text)
		}

	case atom.Ul, atom.Ol:
		c.ensureNewline()
		c.traverseChildren(n)
		c.ensureNewline()

	case atom.Li:
		c.ensureNewline()
		c.writeOpen("• ")
		c.traverseChildren(n)

	case atom.Table:
		c.ensureNewline()
		c.inTable = true
		c.table = [][]string{}
		c.tableRow = []string{}
		c.traverseChildren(n)
		c.inTable = false
		c.renderTable()
		c.ensureNewline()

	case atom.Tr:
		c.tableRow = []string{}
		c.traverseChildren(n)
		if len(c.tableRow) > 0 {
			c.table = append(c.table, c.tableRow)
		}

	case atom.Td, atom.Th:
		text := strings.TrimSpace(c.extractText(n))
		c.tableRow = append(c.tableRow, text)

	case atom.Blockquote:
		c.ensureNewline()
		c.writeOpen(">>> ")
		c.traverseChildren(n)
		c.ensureNewline()

	case atom.Hr:
		c.ensureNewline()
		c.buf.WriteString("─────────────────────\n")
		c.lastNewline = true

	case atom.Script, atom.Style, atom.Meta, atom.Title:
		// Skip these elements
		return

	default:
		c.traverseChildren(n)
	}
}

func (c *htmlConverter) traverseChildren(n *html.Node) {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		c.traverse(child)
	}
}

func (c *htmlConverter) addText(text string) {
	// Clean up whitespace
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	// Replace multiple spaces with single space
	text = strings.Join(strings.Fields(text), " ")

	c.separateIfNeeded()
	c.buf.WriteString(text)
	c.lastNewline = false
	c.glued = false
}

// separateIfNeeded inserts a single space between the pending content and the
// next inline write. It skips the space right after an opening marker (which
// glues to its content) and around existing spaces or line breaks.
func (c *htmlConverter) separateIfNeeded() {
	if c.glued {
		c.glued = false
		return
	}
	if c.buf.Len() == 0 || c.lastNewline {
		return
	}
	if b := c.buf.Bytes(); b[len(b)-1] == ' ' || b[len(b)-1] == '\n' {
		return
	}
	c.buf.WriteString(" ")
}

// writeOpen writes an inline marker that separates from preceding text but
// glues to the content that follows it.
func (c *htmlConverter) writeOpen(marker string) {
	c.separateIfNeeded()
	c.buf.WriteString(marker)
	c.lastNewline = false
	c.glued = true
}

// writeClose writes an inline marker that glues to the preceding content.
func (c *htmlConverter) writeClose(marker string) {
	c.buf.WriteString(marker)
	c.lastNewline = false
	c.glued = false
}

func (c *htmlConverter) ensureNewline() {
	if c.buf.Len() > 0 && !c.lastNewline {
		c.buf.WriteString("\n")
		c.lastNewline = true
	}
	c.glued = false
}

func (c *htmlConverter) extractText(n *html.Node) string {
	var buf bytes.Buffer
	c.extractTextHelper(n, &buf)
	return strings.TrimSpace(buf.String())
}

func (c *htmlConverter) extractTextHelper(n *html.Node, buf *bytes.Buffer) {
	if n == nil {
		return
	}

	if n.Type == html.TextNode {
		text := strings.TrimSpace(n.Data)
		if text != "" {
			text = strings.Join(strings.Fields(text), " ")
			buf.WriteString(text)
		}
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		c.extractTextHelper(child, buf)
	}
}

func (c *htmlConverter) getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func (c *htmlConverter) renderTable() {
	if len(c.table) == 0 {
		return
	}

	// Calculate column widths (in runes, not bytes, so multibyte
	// characters don't break alignment)
	colWidths := make([]int, 0)
	for _, row := range c.table {
		for i, cell := range row {
			w := len([]rune(cell))
			if i >= len(colWidths) {
				colWidths = append(colWidths, w)
			} else if w > colWidths[i] {
				colWidths[i] = w
			}
		}
	}

	// Ensure minimum width
	for i := range colWidths {
		if colWidths[i] < 10 {
			colWidths[i] = 10
		}
	}

	// Render table
	for i, row := range c.table {
		// Add separator before first row or after header
		if i == 0 || i == 1 {
			c.renderTableSeparator(colWidths)
		}

		// Render row
		c.buf.WriteString("│ ")
		for j, cell := range row {
			c.buf.WriteString(padString(cell, colWidths[j]))
			c.buf.WriteString(" │ ")
		}
		c.buf.WriteString("\n")
	}

	// Final separator
	c.renderTableSeparator(colWidths)
}

func (c *htmlConverter) renderTableSeparator(colWidths []int) {
	c.buf.WriteString("├")
	for i, width := range colWidths {
		c.buf.WriteString(strings.Repeat("─", width+2))
		if i < len(colWidths)-1 {
			c.buf.WriteString("┼")
		}
	}
	c.buf.WriteString("┤\n")
}

func padString(s string, width int) string {
	runes := []rune(s)
	if len(runes) >= width {
		return string(runes[:width])
	}
	return s + strings.Repeat(" ", width-len(runes))
}
