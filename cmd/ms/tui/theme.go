package tui

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
)

// palette describes a theme. The first eight roles form a monochrome base; the
// optional accent roles below let "colorful" themes paint individual UI
// elements — both result columns and the surrounding chrome (bars, rules,
// prompt) — with their own hues. An empty accent falls back to a base role, so
// a single-phosphor CRT theme only needs to set the base eight.
type palette struct {
	bg     string // screen background
	bgAlt  string // panel background
	fg     string // primary foreground text
	hi     string // bright highlight (selection, cursor)
	dim    string // dimmed text (read items)
	low    string // faint text (brackets)
	accent string // flagged marker
	danger string // errors

	// Optional result-row accents (fall back to a base role when empty).
	sender  string // from column                 (default: fg)
	subject string // subject column in the list  (default: fg)
	date    string // date column                 (default: dim)
	unread  string // unread marker               (default: hi)
	attach  string // attachment marker           (default: dim)
	label   string // detail meta labels          (default: dim)

	// Optional chrome accents.
	title    string // title-bar badge background  (default: fg)
	titleAlt string // title-bar rest background    (default: title)
	bar      string // status-bar background        (default: bgAlt)
	barText  string // status-bar text              (default: dim)
	rule     string // rules / dividers             (default: low)
	prompt   string // search prompt                (default: dim)
}

// defaultTheme is used when no --theme is supplied.
const defaultTheme = "amber"

// palettes holds every available theme, keyed by its flag name.
var palettes = map[string]palette{
	// Amber phosphor CRT: a single warm hue on near-black. Monochrome by design.
	"amber": {
		bg: "#160d00", bgAlt: "#2c1b04", fg: "#ffb000", hi: "#ffd982",
		dim: "#c08218", low: "#7a5410", accent: "#ff8a1e", danger: "#ff5436",
	},
	// Green phosphor terminal: classic single-hue P1 green on black.
	"green": {
		bg: "#02160a", bgAlt: "#06301a", fg: "#36ff7a", hi: "#c6ffd8",
		dim: "#1fb151", low: "#0f6a30", accent: "#b6ff00", danger: "#ff5c5c",
	},
	// Synthwave: a warm "orchid sunset" — deep plum night with an analogous
	// spread of orchid, magenta, rose, coral and gold. Deliberately free of any
	// blue/cyan so the chrome and the rows stay harmonious.
	"synthwave": {
		bg: "#241327", bgAlt: "#38203f", fg: "#f6e7f1", hi: "#fff4fb",
		dim: "#b884bb", low: "#6f476f", accent: "#ff4fa3", danger: "#ff5d6c",
		sender: "#c79bff", subject: "#ff8fc7", date: "#cf8fd6",
		unread: "#ffcf5c", attach: "#ff9e7a", label: "#c79bff",
		title: "#ffcf5c", titleAlt: "#ff4fa3",
		bar: "#c77be6", barText: "#1c0f20",
		rule: "#9a5fc4", prompt: "#ff8fc7",
	},
	// Ice: a cool but varied spread over deep navy. Sky/cyan chrome frame, steel
	// rules, glacial result columns.
	"ice": {
		bg: "#03121f", bgAlt: "#0a2942", fg: "#8fd6ff", hi: "#eaf7ff",
		dim: "#5aa6d6", low: "#2f6a8f", accent: "#66f0ff", danger: "#ff7a8a",
		sender: "#7ee8c8", subject: "#c9b3ff", date: "#9ad0ff",
		unread: "#eaf7ff", attach: "#a6f0c6", label: "#7ee8c8",
		title: "#66f0ff", titleAlt: "#8fd6ff",
		bar: "#7ee8c8", barText: "#03121f",
		rule: "#4f9fd0", prompt: "#7ee8c8",
	},
	// Paper: a warm light theme — fountain-pen inks on parchment. Dark sepia
	// text on cream, with green/burgundy/sienna/ochre ink accents and coffee
	// brown chrome bars carrying cream text.
	"paper": {
		bg: "#f3e9d2", bgAlt: "#e6d8ba", fg: "#4a3b28", hi: "#2a201a",
		dim: "#9a8a6a", low: "#b3a07e", accent: "#c0532e", danger: "#a52a2a",
		sender: "#356b4f", subject: "#8a3b52", label: "#356b4f", attach: "#b5871d",
		title: "#5b3a1f", titleAlt: "#7a5230",
		bar: "#6b4f2a", barText: "#f3e9d2",
		rule: "#c8b896", prompt: "#8a5a2a",
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

// firstColor returns the first non-empty hex value as a lipgloss color, so a
// theme can chain fallbacks (e.g. titleAlt -> title -> fg).
func firstColor(vals ...string) color.Color {
	for _, v := range vals {
		if v != "" {
			return lipgloss.Color(v)
		}
	}
	return lipgloss.Color("")
}

// theme holds every lipgloss style used by the UI, pre-built from a palette.
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
	p, ok := palettes[name]
	if !ok {
		return theme{}, fmt.Errorf("unknown theme %q (available: %s)", name, strings.Join(themeNames(), ", "))
	}
	return buildTheme(p), nil
}

func buildTheme(p palette) theme {
	c := lipgloss.Color
	bg := c(p.bg)
	fg, hi, dim, low := c(p.fg), c(p.hi), c(p.dim), c(p.low)
	accent, danger := c(p.accent), c(p.danger)

	// Chrome colors with fallbacks.
	titleBg := firstColor(p.title, p.fg)
	titleAltBg := firstColor(p.titleAlt, p.title, p.fg)
	barBg := firstColor(p.bar, p.bgAlt)
	barFg := firstColor(p.barText, p.dim)
	ruleC := firstColor(p.rule, p.low)

	// Result-row accents with fallbacks.
	senderC := firstColor(p.sender, p.fg)
	subjectC := firstColor(p.subject, p.fg)
	dateC := firstColor(p.date, p.dim)
	unreadC := firstColor(p.unread, p.hi)
	attachC := firstColor(p.attach, p.dim)
	labelC := firstColor(p.label, p.dim)

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
func buildInputStyles(p palette) textinput.Styles {
	c := lipgloss.Color
	s := textinput.DefaultDarkStyles()

	s.Focused.Prompt = lipgloss.NewStyle().Foreground(firstColor(p.prompt, p.dim)).Bold(true)
	s.Focused.Text = lipgloss.NewStyle().Foreground(c(p.hi))
	s.Focused.Placeholder = lipgloss.NewStyle().Foreground(c(p.low))
	s.Blurred.Prompt = lipgloss.NewStyle().Foreground(c(p.low)).Bold(true)
	s.Blurred.Text = lipgloss.NewStyle().Foreground(c(p.fg))
	s.Blurred.Placeholder = lipgloss.NewStyle().Foreground(c(p.low))
	s.Cursor.Color = c(p.hi)

	return s
}
