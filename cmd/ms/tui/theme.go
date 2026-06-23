package tui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
	"github.com/rubiojr/meliafts/internal/themes"
)

// firstColor returns the first non-empty hex value as a lipgloss color, so a
// theme can chain fallbacks (e.g. TitleAlt -> Title -> Fg).
func firstColor(vals ...string) color.Color {
	for _, v := range vals {
		if v != "" {
			return lipgloss.Color(v)
		}
	}
	return lipgloss.Color("")
}

// theme holds every lipgloss style used by the UI, built from a themes.Palette.
type theme struct {
	bg color.Color

	title    lipgloss.Style
	titleSub lipgloss.Style
	status   lipgloss.Style
	rule     lipgloss.Style

	cursor  lipgloss.Style
	itemDim lipgloss.Style
	itemSel lipgloss.Style

	sender      lipgloss.Style
	listSubject lipgloss.Style
	date        lipgloss.Style

	flagUnread  lipgloss.Style
	flagFlagged lipgloss.Style
	flagAttach  lipgloss.Style
	bracket     lipgloss.Style

	metaLabel lipgloss.Style
	metaValue lipgloss.Style
	subject   lipgloss.Style
	body      lipgloss.Style

	empty lipgloss.Style
	err   lipgloss.Style

	input textinput.Styles
}

// newTheme resolves a theme by name and builds its styles.
func newTheme(name string) (theme, error) {
	p, ok := themes.Get(name)
	if !ok {
		return theme{}, fmt.Errorf("unknown theme %q (available: %s)", name, strings.Join(themes.Names(), ", "))
	}
	return buildTheme(p), nil
}

func buildTheme(p themes.Palette) theme {
	c := lipgloss.Color
	bg := c(p.Bg)
	fg, hi, dim, low := c(p.Fg), c(p.Hi), c(p.Dim), c(p.Low)
	accent, danger := c(p.Accent), c(p.Danger)

	// Chrome colors with fallbacks.
	titleBg := firstColor(p.Title, p.Fg)
	titleAltBg := firstColor(p.TitleAlt, p.Title, p.Fg)
	barBg := firstColor(p.Bar, p.BgAlt)
	barFg := firstColor(p.BarText, p.Dim)
	ruleC := firstColor(p.Rule, p.Low)

	// Result-row accents with fallbacks.
	senderC := firstColor(p.Sender, p.Fg)
	subjectC := firstColor(p.Subject, p.Fg)
	dateC := firstColor(p.Date, p.Dim)
	unreadC := firstColor(p.Unread, p.Hi)
	attachC := firstColor(p.Attach, p.Dim)
	labelC := firstColor(p.Label, p.Dim)

	style := lipgloss.NewStyle

	return theme{
		bg: bg,

		title:    style().Bold(true).Foreground(bg).Background(titleBg),
		titleSub: style().Foreground(bg).Background(titleAltBg),
		status:   style().Foreground(barFg).Background(barBg),
		rule:     style().Foreground(ruleC),

		cursor:  style().Foreground(hi).Bold(true),
		itemDim: style().Foreground(dim),
		itemSel: style().Foreground(hi).Bold(true),

		sender:      style().Foreground(senderC),
		listSubject: style().Foreground(subjectC),
		date:        style().Foreground(dateC),

		flagUnread:  style().Foreground(unreadC).Bold(true),
		flagFlagged: style().Foreground(accent).Bold(true),
		flagAttach:  style().Foreground(attachC),
		bracket:     style().Foreground(low),

		metaLabel: style().Foreground(labelC).Bold(true),
		metaValue: style().Foreground(fg),
		subject:   style().Foreground(hi).Bold(true),
		body:      style().Foreground(fg),

		empty: style().Foreground(dim).Italic(true),
		err:   style().Foreground(danger).Bold(true),

		input: buildInputStyles(p),
	}
}

// buildInputStyles themes the search text input.
func buildInputStyles(p themes.Palette) textinput.Styles {
	c := lipgloss.Color
	s := textinput.DefaultDarkStyles()

	s.Focused.Prompt = lipgloss.NewStyle().Foreground(firstColor(p.Prompt, p.Dim)).Bold(true)
	s.Focused.Text = lipgloss.NewStyle().Foreground(c(p.Hi))
	s.Focused.Placeholder = lipgloss.NewStyle().Foreground(c(p.Low))
	s.Blurred.Prompt = lipgloss.NewStyle().Foreground(c(p.Low)).Bold(true)
	s.Blurred.Text = lipgloss.NewStyle().Foreground(c(p.Fg))
	s.Blurred.Placeholder = lipgloss.NewStyle().Foreground(c(p.Low))
	s.Cursor.Color = c(p.Hi)

	return s
}
