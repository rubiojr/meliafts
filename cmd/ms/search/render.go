package search

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/rubiojr/meliafts/internal/store"
)

// Styles for the search output. lipgloss (via termenv) automatically downgrades
// to plain text when stdout is not a terminal or NO_COLOR is set, so piped or
// redirected output stays clean.
var (
	badgeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("231")).
			Background(lipgloss.Color("63")).
			Padding(0, 1)

	queryStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	emptyQueryStyle = lipgloss.NewStyle().Faint(true).Italic(true)
	countStyle      = lipgloss.NewStyle().Faint(true)
	ruleStyle       = lipgloss.NewStyle().Faint(true)

	dateStyle      = lipgloss.NewStyle().Faint(true)
	senderStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))
	subjectStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("254"))
	noSubjectStyle = lipgloss.NewStyle().Faint(true).Italic(true)
	snippetStyle   = lipgloss.NewStyle().Faint(true)
	emptyState     = lipgloss.NewStyle().Faint(true).Italic(true)

	bracketStyle = lipgloss.NewStyle().Faint(true)
	unreadStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	flaggedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	attachStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("141"))
)

const bodyIndent = "      " // aligns with the text after the "[xxx] " flag column

// writeText renders the search results as a styled, human-readable report led by
// a header summarising the query and result count. Output is written through
// lipgloss.Writer, which downgrades or strips ANSI styling when stdout is not a
// terminal or NO_COLOR is set.
func writeText(queryStr string, results []store.Message) {
	var b strings.Builder

	b.WriteString(renderHeader(queryStr, len(results)))
	b.WriteString("\n\n")

	if len(results) == 0 {
		b.WriteString(emptyState.Render("No messages found."))
	} else {
		blocks := make([]string, 0, len(results))
		for _, r := range results {
			blocks = append(blocks, renderMessage(r))
		}
		b.WriteString(strings.Join(blocks, "\n\n"))
	}

	fmt.Fprintln(lipgloss.Writer, b.String())
}

// renderHeader builds the main header: a badge, the query (or "all messages"),
// the result count and an underline rule sized to the header width.
func renderHeader(queryStr string, n int) string {
	q := emptyQueryStyle.Render("all messages")
	if strings.TrimSpace(queryStr) != "" {
		q = queryStyle.Render(queryStr)
	}

	title := lipgloss.JoinHorizontal(lipgloss.Center, badgeStyle.Render("ms search"), "  ", q)
	count := countStyle.Render(plural(n))

	width := lipgloss.Width(title)
	if w := lipgloss.Width(count); w > width {
		width = w
	}
	rule := ruleStyle.Render(strings.Repeat("─", width))

	return strings.Join([]string{title, count, rule}, "\n")
}

// renderMessage renders a single result as a styled header line plus an indented
// subject and snippet.
func renderMessage(r store.Message) string {
	header := fmt.Sprintf("%s %s  %s",
		renderFlags(r),
		dateStyle.Render(fmt.Sprintf("%-16s", formatDate(r.Date))),
		senderStyle.Render(sender(r)),
	)

	subject := subjectStyle.Render(r.Subject)
	if r.Subject == "" {
		subject = noSubjectStyle.Render("(no subject)")
	}

	lines := []string{header, bodyIndent + subject}
	if r.Snippet != "" {
		lines = append(lines, bodyIndent+snippetStyle.Render(truncate(r.Snippet, 100)))
	}
	return strings.Join(lines, "\n")
}

// renderFlags renders the read/flagged/attachment state as a fixed three-column
// badge: 'U' unread (green), '*' flagged (orange), '@' has-attachment (purple);
// a space marks an unset bit so columns stay aligned.
func renderFlags(r store.Message) string {
	unread := " "
	if !r.IsRead {
		unread = unreadStyle.Render("U")
	}
	flagged := " "
	if r.IsFlagged {
		flagged = flaggedStyle.Render("*")
	}
	attach := " "
	if r.HasAttachments {
		attach = attachStyle.Render("@")
	}
	return bracketStyle.Render("[") + unread + flagged + attach + bracketStyle.Render("]")
}

func sender(r store.Message) string {
	switch {
	case r.FromName != "" && r.FromAddress != "":
		return fmt.Sprintf("%s <%s>", r.FromName, r.FromAddress)
	case r.FromName != "":
		return r.FromName
	default:
		return r.FromAddress
	}
}

func plural(n int) string {
	if n == 1 {
		return "1 message"
	}
	return fmt.Sprintf("%d messages", n)
}

// formatDate normalises the textual datetime forms returned for the DATETIME
// column into a compact "2006-01-02 15:04" display. The original string is
// returned (trimmed) if it cannot be parsed.
func formatDate(s string) string {
	if s == "" {
		return ""
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.Format("2006-01-02 15:04")
		}
	}
	return truncate(s, 16)
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return string(r[:max])
	}
	return string(r[:max-1]) + "\u2026"
}
