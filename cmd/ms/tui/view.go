package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/rubiojr/meliafts/internal/renderer"
	"github.com/rubiojr/meliafts/internal/store"
)

func (m *model) View() tea.View {
	var content string
	switch {
	case !m.ready:
		content = "loading…"
	case m.width < 50 || m.height < 8:
		content = "terminal too small"
	case m.state == stateDetail:
		content = m.viewDetail()
	default:
		content = m.viewBrowse()
	}

	v := tea.NewView(content)
	v.BackgroundColor = m.theme.bg
	v.AltScreen = true
	return v
}

// --- browse screen ---------------------------------------------------------

func (m *model) viewBrowse() string {
	lines := make([]string, 0, m.height)
	lines = append(lines, m.titleBar("mail search"))
	lines = append(lines, m.inputLine())
	lines = append(lines, m.rule())
	lines = append(lines, m.listLines(m.listHeight())...)
	lines = append(lines, m.statusBar(m.browseStatus()))
	return strings.Join(lines, "\n")
}

func (m *model) listLines(h int) []string {
	out := make([]string, 0, h)

	switch {
	case m.err != nil:
		out = append(out, " "+m.theme.err.Render("error: "+m.err.Error()))
	case m.loading && len(m.results) == 0:
		out = append(out, " "+m.theme.empty.Render("searching…"))
	case len(m.results) == 0:
		out = append(out, " "+m.theme.empty.Render("no messages match this query"))
	default:
		end := min(m.top+h, len(m.results))
		for i := m.top; i < end; i++ {
			out = append(out, m.listRow(i))
		}
	}

	for len(out) < h {
		out = append(out, "")
	}
	return out[:h]
}

func (m *model) listRow(i int) string {
	r := m.results[i]
	selected := i == m.cursor && m.state == stateList

	mark := "  "
	if selected {
		mark = m.theme.cursor.Render("▸ ")
	}

	flags := m.flagBadge(r)
	date := fmt.Sprintf("%-16s", formatDate(r.Date))
	from := fmt.Sprintf("%-18s", truncate(senderShort(r), 18))
	subjectW := max(4, m.width-46)
	subject := truncate(firstNonEmpty(r.Subject, "(no subject)"), subjectW)

	// Each column carries its own accent. Read messages collapse to a single
	// dim tone; the selected row is uniformly bright. Unread rows show the full
	// colorful spread (only "colorful" themes differ here — monochrome themes
	// map every accent back to the same hue).
	dateStyle, fromStyle, subjStyle := m.theme.date, m.theme.sender, m.theme.listSubject
	switch {
	case selected:
		fromStyle, subjStyle = m.theme.itemSel, m.theme.itemSel
	case r.IsRead:
		dateStyle, fromStyle, subjStyle = m.theme.itemDim, m.theme.itemDim, m.theme.itemDim
	}

	return mark + flags + " " +
		dateStyle.Render(date) + "  " +
		fromStyle.Render(from) + "  " +
		subjStyle.Render(subject)
}

func (m *model) flagBadge(r store.Message) string {
	u, f, a := " ", " ", " "
	if !r.IsRead {
		u = m.theme.flagUnread.Render("U")
	}
	if r.IsFlagged {
		f = m.theme.flagFlagged.Render("*")
	}
	if r.HasAttachments {
		a = m.theme.flagAttach.Render("@")
	}
	return m.theme.bracket.Render("[") + u + f + a + m.theme.bracket.Render("]")
}

func (m *model) browseStatus() string {
	if m.state == stateSearch {
		return "ENTER run · ↓/TAB browse · ^R reload · ESC quit"
	}
	pos := 0
	if len(m.results) > 0 {
		pos = m.cursor + 1
	}
	count := fmt.Sprintf("%d/%d", pos, len(m.results))
	if !m.loadedAll {
		count += "+" // more results available below
	}
	hint := "↑↓ move · ENTER open · u unread · s sent · ^R reload · / edit · q quit"
	if m.loadingMore {
		hint = "loading more… · " + hint
	}
	return count + " · " + hint
}

// --- detail screen ---------------------------------------------------------

func (m *model) viewDetail() string {
	lines := make([]string, 0, m.height)
	lines = append(lines, m.titleBar("message"))
	lines = append(lines, m.detailMeta()...)
	lines = append(lines, m.rule())
	lines = append(lines, strings.Split(m.viewport.View(), "\n")...)
	lines = append(lines, m.statusBar(m.detailStatus()))
	return strings.Join(lines, "\n")
}

func (m *model) detailMeta() []string {
	d := m.detail
	valueW := max(1, m.width-8)

	subject := m.theme.subject.Render(truncate(firstNonEmpty(d.Subject, "(no subject)"), max(1, m.width-1)))
	from := m.theme.metaLabel.Render("From  ") + m.theme.metaValue.Render(truncate(senderFull(d), valueW))
	date := m.theme.metaLabel.Render("Date  ") + m.theme.metaValue.Render(truncate(formatDate(d.Date), valueW))

	return []string{" " + subject, " " + from, " " + date}
}

func (m *model) detailStatus() string {
	pct := "top"
	switch {
	case m.viewport.AtBottom():
		pct = "end"
	case m.viewport.AtTop():
		pct = "top"
	default:
		pct = fmt.Sprintf("%d%%", int(m.viewport.ScrollPercent()*100))
	}
	return fmt.Sprintf("%s · n/p ↑↓ next·prev · PgUp/PgDn scroll · ESC back · q quit", pct)
}

// renderBody builds the scrollable message body. The renderer picks the best
// available source (plain text, then converted HTML, then the snippet) and
// returns clean unstyled text; here we only theme and width-wrap it.
func (m *model) renderBody(d *store.Message) string {
	text := renderer.Body(d.BodyText, d.BodyHTML, d.Snippet)
	if text == "" {
		return m.theme.empty.Render("(this message has no readable body)")
	}
	return m.theme.body.Width(max(10, m.width-1)).Render(text)
}

// --- chrome ----------------------------------------------------------------

func (m *model) titleBar(sub string) string {
	left := m.theme.title.Render(" MELIA ")
	avail := max(0, m.width-lipgloss.Width(left))
	subtitle := truncate(" · "+sub, avail)
	pad := avail - len([]rune(subtitle))
	if pad > 0 {
		subtitle += strings.Repeat(" ", pad)
	}
	return left + m.theme.titleSub.Render(subtitle)
}

func (m *model) inputLine() string {
	return " " + m.input.View()
}

func (m *model) rule() string {
	return m.theme.rule.Render(strings.Repeat("─", max(0, m.width)))
}

func (m *model) statusBar(s string) string {
	line := truncate(" "+s, m.width)
	if pad := m.width - len([]rune(line)); pad > 0 {
		line += strings.Repeat(" ", pad)
	}
	return m.theme.status.Render(line)
}

// --- helpers ---------------------------------------------------------------

func lipglossWidth(s string) int { return lipgloss.Width(s) }

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func senderShort(m store.Message) string {
	if m.FromName != "" {
		return m.FromName
	}
	return m.FromAddress
}

func senderFull(m *store.Message) string {
	switch {
	case m.FromName != "" && m.FromAddress != "":
		return fmt.Sprintf("%s <%s>", m.FromName, m.FromAddress)
	case m.FromName != "":
		return m.FromName
	default:
		return m.FromAddress
	}
}

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
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}
