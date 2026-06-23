package tui

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
)

// palette is a small set of hex colors that fully describes a theme. Every
// lipgloss style in the UI is derived from these eight roles.
type palette struct {
	bg     string // screen background
	bgAlt  string // panel / status-bar background
	fg     string // primary foreground text
	hi     string // bright highlight (selection, unread, cursor)
	dim    string // dimmed text (read items, metadata labels)
	low    string // faint text (rules, brackets)
	accent string // hot accent (flagged marker)
	danger string // errors
}

// defaultTheme is used when no --theme is supplied.
const defaultTheme = "amber"

// palettes holds every available theme, keyed by its flag name.
var palettes = map[string]palette{
	// Amber phosphor CRT: warm orange on near-black.
	"amber": {
		bg: "#160d00", bgAlt: "#2c1b04", fg: "#ffb000", hi: "#ffd982",
		dim: "#c08218", low: "#7a5410", accent: "#ff8a1e", danger: "#ff5436",
	},
	// Green phosphor terminal: classic P1 green on black.
	"green": {
		bg: "#02160a", bgAlt: "#06301a", fg: "#36ff7a", hi: "#c6ffd8",
		dim: "#1fb151", low: "#0f6a30", accent: "#b6ff00", danger: "#ff5c5c",
	},
	// Synthwave: neon pink and cyan over deep indigo.
	"synthwave": {
		bg: "#1a0b2e", bgAlt: "#2d1650", fg: "#ff7edb", hi: "#fdfdff",
		dim: "#c264d6", low: "#6d4a99", accent: "#36f9f6", danger: "#fe4450",
	},
	// Ice: glacial cyan and sky blue over deep navy.
	"ice": {
		bg: "#03121f", bgAlt: "#0a2942", fg: "#8fd6ff", hi: "#eaf7ff",
		dim: "#5aa6d6", low: "#2f6a8f", accent: "#66f0ff", danger: "#ff7a8a",
	},
}

// themeNames returns the available theme names in stable, sorted order.
func themeNames() []string {
	names := make([]string, 0, len(palettes))
	for n := range palettes {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// theme holds every lipgloss style used by the UI, pre-built from a palette.
type theme struct {
	bg color.Color

	title    lipgloss.Style
	titleSub lipgloss.Style
	status   lipgloss.Style
	rule     lipgloss.Style

	cursor  lipgloss.Style
	item    lipgloss.Style
	itemDim lipgloss.Style
	itemSel lipgloss.Style

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
	p, ok := palettes[name]
	if !ok {
		return theme{}, fmt.Errorf("unknown theme %q (available: %s)", name, strings.Join(themeNames(), ", "))
	}
	return buildTheme(p), nil
}

func buildTheme(p palette) theme {
	c := lipgloss.Color
	bg, bgAlt := c(p.bg), c(p.bgAlt)
	fg, hi, dim, low := c(p.fg), c(p.hi), c(p.dim), c(p.low)
	accent, danger := c(p.accent), c(p.danger)

	style := lipgloss.NewStyle

	return theme{
		bg: bg,

		title:    style().Bold(true).Foreground(bg).Background(fg),
		titleSub: style().Foreground(bg).Background(fg),
		status:   style().Foreground(dim).Background(bgAlt),
		rule:     style().Foreground(low),

		cursor:  style().Foreground(hi).Bold(true),
		item:    style().Foreground(fg),
		itemDim: style().Foreground(dim),
		itemSel: style().Foreground(hi).Bold(true),

		flagUnread:  style().Foreground(hi).Bold(true),
		flagFlagged: style().Foreground(accent).Bold(true),
		flagAttach:  style().Foreground(dim),
		bracket:     style().Foreground(low),

		metaLabel: style().Foreground(dim).Bold(true),
		metaValue: style().Foreground(fg),
		subject:   style().Foreground(hi).Bold(true),
		body:      style().Foreground(fg),

		empty: style().Foreground(dim).Italic(true),
		err:   style().Foreground(danger).Bold(true),

		input: buildInputStyles(p),
	}
}

// buildInputStyles themes the search text input.
func buildInputStyles(p palette) textinput.Styles {
	c := lipgloss.Color
	s := textinput.DefaultDarkStyles()

	s.Focused.Prompt = lipgloss.NewStyle().Foreground(c(p.dim)).Bold(true)
	s.Focused.Text = lipgloss.NewStyle().Foreground(c(p.hi))
	s.Focused.Placeholder = lipgloss.NewStyle().Foreground(c(p.low))
	s.Blurred.Prompt = lipgloss.NewStyle().Foreground(c(p.low)).Bold(true)
	s.Blurred.Text = lipgloss.NewStyle().Foreground(c(p.fg))
	s.Blurred.Placeholder = lipgloss.NewStyle().Foreground(c(p.low))
	s.Cursor.Color = c(p.hi)

	return s
}
