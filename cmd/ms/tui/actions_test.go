package tui

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/rubiojr/meliafts/internal/actions"
	"github.com/rubiojr/meliafts/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// actionModel builds a windowed model with an action runner whose single script
// appends each fired message id to out.
func actionModel(t *testing.T) (*model, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("action scripts use /bin/sh")
	}
	dir := t.TempDir()
	out := filepath.Join(t.TempDir(), "fired.txt")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "10-log"),
		[]byte("#!/bin/sh\necho \"$MELIAFTS_ID\" >> "+out+"\n"), 0o755))

	m := newModel(nil, 50, defaultReloadInterval, "", testTheme)
	m.Update(tea.WindowSizeMsg{Width: 90, Height: 24})
	m.actions = &actions.Runner{Dir: dir, Timeout: time.Second}
	m.tracker = actions.NewTracker()
	return m, out
}

func TestTUIActionsFireOnReload(t *testing.T) {
	m, out := actionModel(t)

	// A fresh query / first load primes the baseline and fires nothing.
	m.results = []store.Message{{ID: "a"}, {ID: "b"}}
	assert.Nil(t, m.detectActions(false))

	// A reload that surfaces a new message fires for that message only.
	m.results = []store.Message{{ID: "c"}, {ID: "a"}, {ID: "b"}}
	cmd := m.detectActions(true)
	require.NotNil(t, cmd)

	msg := cmd()
	ran, ok := msg.(actionsRanMsg)
	require.True(t, ok)
	assert.Equal(t, 1, ran.fired)
	assert.NoError(t, ran.err)

	m.Update(ran)
	assert.Equal(t, 1, m.actionsFired)

	body, _ := os.ReadFile(out)
	assert.Equal(t, "c\n", string(body), "only the new id fired")

	// A reload with nothing new fires nothing.
	assert.Nil(t, m.detectActions(true))
}

func TestTUIActionsFireViaReloadPath(t *testing.T) {
	m, out := actionModel(t)
	m.query = "q"

	// First (fresh) result set primes the baseline through the real Update path.
	_, cmd := m.Update(searchMsg{results: []store.Message{{ID: "a"}}, limit: 50})
	drain(t, m, cmd)
	assert.Equal(t, 0, m.actionsFired)

	// A reload (keepPos) carrying a new message fires through onSearch.
	_, cmd = m.Update(searchMsg{
		results: []store.Message{{ID: "b"}, {ID: "a"}}, limit: 50, keepPos: true,
	})
	require.NotNil(t, cmd)
	drain(t, m, cmd)

	assert.Equal(t, 1, m.actionsFired)
	body, _ := os.ReadFile(out)
	assert.Equal(t, "b\n", string(body))
}

func TestTUIActionsReBaselineOnFreshQuery(t *testing.T) {
	m, out := actionModel(t)

	m.results = []store.Message{{ID: "a"}}
	assert.Nil(t, m.detectActions(false)) // prime

	// Switching query (reload == false) re-baselines: the "new" rows do not fire.
	m.results = []store.Message{{ID: "x"}, {ID: "y"}}
	assert.Nil(t, m.detectActions(false))

	// A subsequent reload of that query has nothing newer, so nothing fires.
	assert.Nil(t, m.detectActions(true))

	_, err := os.ReadFile(out)
	assert.True(t, os.IsNotExist(err), "no script should have run")
}

func TestTUIActionsDisabledByDefault(t *testing.T) {
	m := newModel(nil, 50, defaultReloadInterval, "", testTheme)
	m.results = []store.Message{{ID: "a"}}
	assert.Nil(t, m.detectActions(false))
	assert.Nil(t, m.detectActions(true))
	assert.Empty(t, m.actionsTag())
}

func TestActionsTag(t *testing.T) {
	m := newModel(nil, 50, defaultReloadInterval, "", testTheme)
	m.actions = &actions.Runner{} // enabled
	assert.Empty(t, m.actionsTag(), "no indicator before anything fires")

	m.actionsFired = 3
	assert.Equal(t, "3 fired", m.actionsTag())

	m.actionWarn = true
	assert.Equal(t, "3 fired (errors)", m.actionsTag())
}
