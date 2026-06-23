package tui

import (
	"database/sql"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rubiojr/meliafts/internal/store"
	"github.com/rubiojr/meliafts/internal/themes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// testTheme is the default theme used across model tests.
var testTheme = mustTheme(themes.Default)

func mustTheme(name string) theme {
	th, err := newTheme(name)
	if err != nil {
		panic(err)
	}
	return th
}

func TestThemes(t *testing.T) {
	names := themes.Names()
	assert.Subset(t, names, []string{"amber", "green", "synthwave", "ice", "paper"})

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			th, err := newTheme(name)
			require.NoError(t, err)
			assert.NotNil(t, th.bg)
			// A styled string should be produced for each theme.
			assert.NotEmpty(t, th.subject.Render("x"))
		})
	}

	_, err := newTheme("does-not-exist")
	assert.ErrorContains(t, err, "unknown theme")
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "melia.db")

	w, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	stmts := []string{
		`CREATE TABLE messages (
			id TEXT PRIMARY KEY, subject TEXT, from_name TEXT, from_address TEXT,
			to_addresses TEXT, snippet TEXT, body_text TEXT, body_html TEXT,
			date DATETIME NOT NULL, is_read INTEGER DEFAULT 0, is_flagged INTEGER DEFAULT 0,
			has_attachments INTEGER DEFAULT 0)`,
		`CREATE VIRTUAL TABLE messages_fts USING fts5(
			subject, from_name, from_address, to_text, snippet, body_text,
			content=messages, content_rowid=rowid)`,
		`INSERT INTO messages (id, subject, from_name, from_address, snippet, body_text, date, is_read, is_flagged, has_attachments) VALUES
			('m1','Invoice 2024','Bob Smith','bob@acme.com','your invoice','The full invoice body text.','2024-01-15 09:30:00',0,1,1),
			('m2','Meeting notes','Carol','carol@work.com','standup','Agenda items here.','2024-02-20 14:00:00',1,0,0)`,
		`INSERT INTO messages_fts(rowid, subject, from_name, from_address, to_text, snippet, body_text)
			SELECT rowid, subject, from_name, from_address, '', snippet, body_text FROM messages`,
	}
	for _, s := range stmts {
		_, err := w.Exec(s)
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())

	st, err := store.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	return st
}

func key(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEsc}
	case "ctrl+r":
		return tea.KeyPressMsg{Mod: tea.ModCtrl, Code: 'r'}
	default:
		return tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
	}
}

// runCmd executes a tea.Cmd and feeds the resulting message back into the model.
func runCmd(t *testing.T, m *model, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	if msg := cmd(); msg != nil {
		m.Update(msg)
	}
}

func TestKeyStringMapping(t *testing.T) {
	// Guard our handler's key matching against the actual String() values.
	assert.Equal(t, "enter", key("enter").String())
	assert.Equal(t, "down", key("down").String())
	assert.Equal(t, "esc", key("esc").String())
	assert.Equal(t, "/", key("/").String())
	assert.Equal(t, "q", key("q").String())
	assert.Equal(t, "ctrl+r", key("ctrl+r").String())
}

func TestReloadKeepsPosition(t *testing.T) {
	st := newTestStore(t)
	m := newModel(st, 50, defaultReloadInterval, "", testTheme)
	m.Update(tea.WindowSizeMsg{Width: 90, Height: 24})
	runCmd(t, m, m.runSearch("", false, false))
	require.Len(t, m.results, 2)

	// Browse the list and select the second message.
	m.state = stateList
	m.moveTo(1)
	require.Equal(t, 1, m.cursor)

	// ctrl+r reloads while preserving the cursor and the list state.
	_, cmd := m.Update(key("ctrl+r"))
	require.NotNil(t, cmd)
	runCmd(t, m, cmd)
	assert.Equal(t, stateList, m.state)
	assert.Equal(t, 1, m.cursor)
	assert.Len(t, m.results, 2)
}

func TestReloadUsesActiveQuery(t *testing.T) {
	st := newTestStore(t)
	m := newModel(st, 50, defaultReloadInterval, "", testTheme)
	m.Update(tea.WindowSizeMsg{Width: 90, Height: 24})
	runCmd(t, m, m.runSearch("", false, false)) // active query is ""
	require.Len(t, m.results, 2)

	m.state = stateList
	// Edit the input without pressing Enter: the active query must not change.
	m.input.SetValue("subject:invoice")

	_, cmd := m.Update(key("ctrl+r"))
	runCmd(t, m, cmd)
	// Reload re-ran the active query (""), not the half-typed input.
	assert.Len(t, m.results, 2)
}

func TestScheduleReloadToggle(t *testing.T) {
	st := newTestStore(t)

	on := newModel(st, 50, defaultReloadInterval, "", testTheme)
	assert.NotNil(t, on.scheduleReload())

	off := newModel(st, 50, 0, "", testTheme)
	assert.Nil(t, off.scheduleReload(), "a reload interval of 0 disables the timer")
}

func TestAutoReloadTickReArms(t *testing.T) {
	st := newTestStore(t)
	m := newModel(st, 50, defaultReloadInterval, "", testTheme)
	m.Update(tea.WindowSizeMsg{Width: 90, Height: 24})
	runCmd(t, m, m.runSearch("", false, false))
	m.state = stateList
	m.moveTo(1)

	// A tick returns a command (reload + re-armed timer) and leaves the view as
	// it was. The returned cmd is not executed here to avoid blocking on the
	// 30s timer.
	_, cmd := m.Update(reloadTickMsg{})
	require.NotNil(t, cmd)
	assert.Equal(t, stateList, m.state)
	assert.Equal(t, 1, m.cursor)
}

func TestModelSearchFlow(t *testing.T) {
	st := newTestStore(t)
	m := newModel(st, 50, defaultReloadInterval, "", testTheme)

	// Window size and initial (empty) search loading all messages.
	m.Update(tea.WindowSizeMsg{Width: 90, Height: 24})
	runCmd(t, m, m.runSearch("", false, false))
	require.Len(t, m.results, 2)
	assert.Equal(t, stateSearch, m.state)

	// Type a query and run it: focus should advance to the list.
	m.input.SetValue("subject:invoice")
	_, cmd := m.Update(key("enter"))
	runCmd(t, m, cmd)
	require.Equal(t, stateList, m.state)
	require.Len(t, m.results, 1)
	assert.Equal(t, "m1", m.results[0].ID)

	// Open the selected message.
	_, cmd = m.Update(key("enter"))
	runCmd(t, m, cmd)
	require.Equal(t, stateDetail, m.state)
	require.NotNil(t, m.detail)
	assert.Equal(t, "Invoice 2024", m.detail.Subject)

	// The detail view shows the body.
	plain := ansi.Strip(m.View().Content)
	assert.Contains(t, plain, "Invoice 2024")
	assert.Contains(t, plain, "The full invoice body text.")
	assert.Contains(t, plain, "bob@acme.com")

	// Esc returns to the list.
	m.Update(key("esc"))
	assert.Equal(t, stateList, m.state)

	// Esc again returns to search.
	m.Update(key("esc"))
	assert.Equal(t, stateSearch, m.state)
}

func TestModelInvalidQuery(t *testing.T) {
	st := newTestStore(t)
	m := newModel(st, 50, defaultReloadInterval, "", testTheme)
	m.Update(tea.WindowSizeMsg{Width: 90, Height: 24})

	m.input.SetValue("bogus:field")
	_, cmd := m.Update(key("enter"))
	runCmd(t, m, cmd)

	// Stays in search with an error, which is surfaced in the view.
	assert.Equal(t, stateSearch, m.state)
	require.Error(t, m.err)
	assert.Contains(t, ansi.Strip(m.View().Content), "error")
}

func TestModelScrolling(t *testing.T) {
	m := newModel(nil, 50, defaultReloadInterval, "", testTheme)
	m.Update(tea.WindowSizeMsg{Width: 90, Height: 24}) // listHeight = 24-4 = 20
	m.results = make([]store.Message, 100)
	for i := range m.results {
		m.results[i].ID = string(rune('a' + i%26))
	}
	m.state = stateList

	m.moveTo(0)
	assert.Equal(t, 0, m.top)

	// Moving past the bottom of the viewport scrolls the window.
	m.moveTo(25)
	assert.Equal(t, 25, m.cursor)
	assert.Equal(t, 25-m.listHeight()+1, m.top)

	// Clamping at the ends.
	m.moveCursor(1000)
	assert.Equal(t, 99, m.cursor)
	m.moveCursor(-1000)
	assert.Equal(t, 0, m.cursor)
	assert.Equal(t, 0, m.top)
}

func TestHTMLToText(t *testing.T) {
	in := `<html><head><style>.x{color:red}</style></head>
		<body><p>Hello <b>world</b></p><br>line two<script>alert(1)</script>
		<div>caf&eacute; &amp; tea</div></body></html>`
	out := htmlToText(in)

	assert.Contains(t, out, "Hello world")
	assert.Contains(t, out, "line two")
	assert.Contains(t, out, "café & tea")
	assert.NotContains(t, out, "alert(1)")
	assert.NotContains(t, out, "color:red")
	assert.NotContains(t, out, "<")
}
