// Package tui implements the `ms tui` subcommand: an interactive, amber
// CRT-themed search interface built with Bubble Tea v2.
package tui

import (
	"context"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/rubiojr/meliafts/internal/store"
	"github.com/rubiojr/meliafts/internal/themes"
	"github.com/urfave/cli/v3"
)

// defaultReloadInterval is the default auto-reload period.
const defaultReloadInterval = 30 * time.Second

// Command is the `ms tui` subcommand.
var Command = &cli.Command{
	Name:      "tui",
	Usage:     "Launch the interactive amber CRT search UI",
	ArgsUsage: "[query]",
	Description: `Open an interactive terminal UI to search the melia database.

The same Gmail-style query language as 'ms search' is supported. Type a query,
press Enter to run it, use the arrow keys to browse results and Enter to read a
message. Press Esc to step back and q (or Ctrl+C) to quit. An optional initial
query may be supplied on the command line.`,
	Flags: []cli.Flag{
		&cli.IntFlag{
			Name:    "limit",
			Aliases: []string{"n"},
			Value:   200,
			Usage:   "maximum number of results to load",
		},
		&cli.StringFlag{
			Name:    "theme",
			Aliases: []string{"t"},
			Value:   themes.Default,
			Usage:   "color theme (" + strings.Join(themes.Names(), ", ") + ")",
		},
		&cli.DurationFlag{
			Name:  "reload",
			Value: defaultReloadInterval,
			Usage: "auto-reload interval (e.g. 30s, 2m; 0 to disable)",
		},
	},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		th, err := newTheme(cmd.String("theme"))
		if err != nil {
			return err
		}

		st, err := store.Open(cmd.String("db"))
		if err != nil {
			return err
		}
		defer st.Close()

		initial := strings.Join(cmd.Args().Slice(), " ")
		m := newModel(st, cmd.Int("limit"), cmd.Duration("reload"), initial, th)
		p := tea.NewProgram(m, tea.WithContext(ctx))
		_, err = p.Run()
		return err
	},
}

// sessionState is the current screen of the UI.
type sessionState int

const (
	stateSearch sessionState = iota // editing the query
	stateList                       // browsing results
	stateDetail                     // reading a message
)

type model struct {
	store  *store.Store
	limit  int
	theme  theme
	reload time.Duration

	width  int
	height int
	ready  bool

	state sessionState

	input    textinput.Model
	viewport viewport.Model

	query   string // the query backing the current results (used by reload)
	results []store.Message
	cursor  int // selected index into results
	top     int // first visible result row (scroll offset)

	detail  *store.Message
	loading bool
	err     error
}

func newModel(st *store.Store, limit int, reload time.Duration, initialQuery string, th theme) *model {
	ti := textinput.New()
	ti.Prompt = "search › "
	ti.Placeholder = "subject:invoice unread: newer:7d"
	ti.SetStyles(th.input)
	ti.SetValue(initialQuery)
	ti.CharLimit = 512

	return &model{
		store:    st,
		limit:    limit,
		theme:    th,
		reload:   reload,
		state:    stateSearch,
		query:    initialQuery,
		input:    ti,
		viewport: viewport.New(),
	}
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(m.input.Focus(), m.runSearch(m.query, m.query != "", false), m.scheduleReload())
}

// --- messages & commands ---------------------------------------------------

type searchMsg struct {
	results []store.Message
	err     error
	advance bool // move focus to the list when results arrive
	keepPos bool // preserve the cursor/scroll position (used by reload)
}

type detailMsg struct {
	msg *store.Message
	err error
}

// reloadTickMsg is delivered on the auto-reload timer.
type reloadTickMsg struct{}

func (m *model) runSearch(q string, advance, keepPos bool) tea.Cmd {
	st, limit := m.store, m.limit
	return func() tea.Msg {
		res, err := st.Search(q, limit)
		return searchMsg{results: res, err: err, advance: advance, keepPos: keepPos}
	}
}

// scheduleReload arms the periodic auto-reload timer. It returns nil when
// auto-reload is disabled (interval <= 0), which stops the timer chain.
func (m *model) scheduleReload() tea.Cmd {
	if m.reload <= 0 {
		return nil
	}
	return tea.Tick(m.reload, func(time.Time) tea.Msg {
		return reloadTickMsg{}
	})
}

func (m *model) loadDetail(id string) tea.Cmd {
	st := m.store
	return func() tea.Msg {
		msg, err := st.Load(id)
		return detailMsg{msg: msg, err: err}
	}
}

// --- update ----------------------------------------------------------------

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ready = true
		m.applyLayout()
		return m, nil
	case searchMsg:
		return m.onSearch(msg)
	case detailMsg:
		return m.onDetail(msg)
	case reloadTickMsg:
		// Silently refresh the current results and re-arm the timer.
		return m, tea.Batch(m.runSearch(m.query, false, true), m.scheduleReload())
	case tea.KeyPressMsg:
		return m.onKey(msg)
	}

	// Forward other messages (cursor blink, etc.) to the active component.
	var cmd tea.Cmd
	switch m.state {
	case stateSearch:
		m.input, cmd = m.input.Update(msg)
	case stateDetail:
		m.viewport, cmd = m.viewport.Update(msg)
	}
	return m, cmd
}

func (m *model) onSearch(msg searchMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	m.err = msg.err
	if msg.err != nil {
		m.results = nil
		return m, nil
	}
	m.results = msg.results
	if msg.keepPos {
		m.cursor = clamp(m.cursor, 0, max(0, len(m.results)-1))
		m.syncScroll()
	} else {
		m.cursor, m.top = 0, 0
	}
	if msg.advance && len(m.results) > 0 {
		m.state = stateList
		m.input.Blur()
	}
	return m, nil
}

func (m *model) onDetail(msg detailMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.err = msg.err
		return m, nil
	}
	m.err = nil
	m.detail = msg.msg
	m.state = stateDetail
	m.viewport.SetContent(m.renderBody(msg.msg))
	m.viewport.GotoTop()
	return m, nil
}

func (m *model) onKey(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "ctrl+r":
		// Reload the active query, keeping the cursor where it is.
		m.loading = true
		return m, m.runSearch(m.query, false, true)
	}
	switch m.state {
	case stateSearch:
		return m.keySearch(k)
	case stateList:
		return m.keyList(k)
	case stateDetail:
		return m.keyDetail(k)
	}
	return m, nil
}

func (m *model) keySearch(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "enter":
		m.loading = true
		m.query = m.input.Value()
		return m, m.runSearch(m.query, true, false)
	case "down", "tab":
		if len(m.results) > 0 {
			m.state = stateList
			m.input.Blur()
		}
		return m, nil
	case "esc":
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(k)
	return m, cmd
}

func (m *model) keyList(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "q":
		return m, tea.Quit
	case "esc", "/", "i":
		m.state = stateSearch
		return m, m.input.Focus()
	case "enter":
		if len(m.results) > 0 {
			m.loading = true
			return m, m.loadDetail(m.results[m.cursor].ID)
		}
		return m, nil
	default:
		m.navigateList(k.String())
		return m, nil
	}
}

// navigateList moves the list cursor in response to a navigation key.
func (m *model) navigateList(s string) {
	switch s {
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case "pgup", "ctrl+u":
		m.moveCursor(-m.listHeight())
	case "pgdown", "ctrl+d":
		m.moveCursor(m.listHeight())
	case "home", "g":
		m.moveTo(0)
	case "end", "G":
		m.moveTo(len(m.results) - 1)
	}
}

func (m *model) keyDetail(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "q":
		return m, tea.Quit
	case "esc", "backspace", "left", "h":
		m.state = stateList
		m.detail = nil
		return m, nil
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(k)
	return m, cmd
}

// --- navigation & layout ---------------------------------------------------

func (m *model) moveCursor(delta int) { m.moveTo(m.cursor + delta) }

func (m *model) moveTo(idx int) {
	if len(m.results) == 0 {
		return
	}
	m.cursor = clamp(idx, 0, len(m.results)-1)
	m.syncScroll()
}

func (m *model) syncScroll() {
	h := m.listHeight()
	if m.cursor < m.top {
		m.top = m.cursor
	}
	if m.cursor >= m.top+h {
		m.top = m.cursor - h + 1
	}
	if m.top < 0 {
		m.top = 0
	}
}

func (m *model) listHeight() int       { return max(1, m.height-4) }
func (m *model) detailBodyHeight() int { return max(1, m.height-6) }

func (m *model) applyLayout() {
	promptW := lipglossWidth(m.input.Prompt)
	m.input.SetWidth(max(10, m.width-promptW-3))

	m.viewport.SetWidth(m.width)
	m.viewport.SetHeight(m.detailBodyHeight())
	if m.detail != nil {
		m.viewport.SetContent(m.renderBody(m.detail))
	}
	m.syncScroll()
}
