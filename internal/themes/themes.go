// Package themes holds the color palettes for the ms TUI. Each theme lives in
// its own file and registers itself via init(), so adding a theme is just a
// matter of dropping in a new file. The palettes are pure data; the TUI turns a
// Palette into concrete lipgloss styles.
package themes

import "sort"

// Default is the theme used when none is requested.
const Default = "amber"

// Palette describes a theme as a set of hex colors. The first eight roles form
// a monochrome base; the optional roles let "colorful" themes paint individual
// elements with their own hues. An empty optional role falls back to a base
// role (handled by the consumer), so a single-phosphor CRT theme only needs the
// base eight.
type Palette struct {
	Name string

	// Base roles.
	Bg     string // screen background
	BgAlt  string // panel background
	Fg     string // primary foreground text
	Hi     string // bright highlight (selection, cursor)
	Dim    string // dimmed text (read items)
	Low    string // faint text (brackets)
	Accent string // flagged marker
	Danger string // errors

	// Optional result-row accents.
	Sender  string // from column                (default: Fg)
	Subject string // subject column in the list (default: Fg)
	Date    string // date column                (default: Dim)
	Unread  string // unread marker              (default: Hi)
	Attach  string // attachment marker          (default: Dim)
	Label   string // detail meta labels         (default: Dim)

	// Optional chrome accents.
	Title    string // title-bar badge background (default: Fg)
	TitleAlt string // title-bar rest background   (default: Title)
	Bar      string // status-bar background       (default: BgAlt)
	BarText  string // status-bar text             (default: Dim)
	Rule     string // rules / dividers            (default: Low)
	Prompt   string // search prompt               (default: Dim)
}

// registry holds every theme keyed by name. Themes self-register from init().
var registry = map[string]Palette{}

// register adds a palette to the registry. It is called from each theme's
// init(); a duplicate name panics to catch mistakes at startup.
func register(p Palette) {
	if _, dup := registry[p.Name]; dup {
		panic("themes: duplicate theme name " + p.Name)
	}
	registry[p.Name] = p
}

// Get returns the palette for name and whether it exists.
func Get(name string) (Palette, bool) {
	p, ok := registry[name]
	return p, ok
}

// Names returns the registered theme names in stable, sorted order.
func Names() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
