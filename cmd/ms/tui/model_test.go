package tui

import (
	"database/sql"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rubiojr/meliafts/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// testTheme is the default theme used across model tests.
var testTheme = mustTheme(defaultTheme)

func mustTheme(name string) theme {
	th, err := newTheme(name)
	if err != nil {
		panic(err)
	}
	return th
}

func TestThemes(t *testing.T) {
	names := themeNames()
	assert.Subset(t, names, []string{"amber", "green", "synthwave", "ice"})

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
}

func TestModelSearchFlow(t *testing.T) {
	st := newTestStore(t)
	m := newModel(st, 50, "", testTheme)

	// Window size and initial (empty) search loading all messages.
	m.Update(tea.WindowSizeMsg{Width: 90, Height: 24})
	runCmd(t, m, m.runSearch("", false))
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
	m := newModel(st, 50, "", testTheme)
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
	m := newModel(nil, 50, "", testTheme)
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
