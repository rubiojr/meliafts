package actions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rubiojr/meliafts/internal/store"
	"github.com/rubiojr/meliafts/pkg/action"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireUnix(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("action scripts use /bin/sh")
	}
}

// writeScript writes name into dir with the given body, optionally executable.
func writeScript(t *testing.T, dir, name, body string, exec bool) {
	t.Helper()
	mode := os.FileMode(0o644)
	if exec {
		mode = 0o755
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), mode))
}

func TestRunnableDiscovery(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "10-run", "#!/bin/sh\n", true)
	writeScript(t, dir, "20-noexec", "#!/bin/sh\n", false)
	writeScript(t, dir, ".hidden", "#!/bin/sh\n", true)
	writeScript(t, dir, "30-backup~", "#!/bin/sh\n", true)
	writeScript(t, dir, "new-message.sample", "#!/bin/sh\n", true)
	require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir"), 0o755))

	r := &Runner{Dir: dir}
	got, err := r.scripts()
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "10-run", filepath.Base(got[0]))
	assert.True(t, r.Enabled())
}

func TestEnabledMissingDir(t *testing.T) {
	r := &Runner{Dir: filepath.Join(t.TempDir(), "does-not-exist")}
	assert.False(t, r.Enabled())
	got, err := r.scripts()
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestFirePayload(t *testing.T) {
	requireUnix(t)
	dir := t.TempDir()
	out := filepath.Join(t.TempDir(), "out.txt")
	writeScript(t, dir, "10-dump", "#!/bin/sh\n"+
		"{ echo \"id=$MELIAFTS_ID\"; echo \"event=$MELIAFTS_EVENT\";"+
		" echo \"subject=$MELIAFTS_SUBJECT\"; echo \"unread=$MELIAFTS_UNREAD\";"+
		" echo \"db=$MELIAFTS_DB\"; echo \"query=$MELIAFTS_QUERY\";"+
		" printf 'stdin='; cat; echo; } >> "+out+"\n", true)

	r := &Runner{Dir: dir, DBPath: "/tmp/melia.db", Timeout: 5 * time.Second}
	ran, err := r.Fire(context.Background(), action.Event{
		Name:  action.EventNew,
		Query: "unread:",
		Message: action.Message{
			ID: "msg-1", Subject: "Hello & welcome", IsRead: false,
			FromAddress: "a@b.com",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, ran)

	body, err := os.ReadFile(out)
	require.NoError(t, err)
	s := string(body)
	assert.Contains(t, s, "id=msg-1")
	assert.Contains(t, s, "event=new-message")
	assert.Contains(t, s, "subject=Hello & welcome")
	assert.Contains(t, s, "unread=1")
	assert.Contains(t, s, "db=/tmp/melia.db")
	assert.Contains(t, s, "query=unread:")
	assert.Contains(t, s, `"id":"msg-1"`)
	assert.Contains(t, s, `"from_address":"a@b.com"`)
}

func TestFireRunsAllAndReportsFailures(t *testing.T) {
	requireUnix(t)
	dir := t.TempDir()
	out := filepath.Join(t.TempDir(), "out.txt")
	writeScript(t, dir, "10-ok", "#!/bin/sh\necho ok >> "+out+"\n", true)
	writeScript(t, dir, "20-fail", "#!/bin/sh\nexit 3\n", true)

	var logs []string
	r := &Runner{Dir: dir, Logf: func(f string, a ...any) { logs = append(logs, f) }}
	ran, err := r.Fire(context.Background(), action.Event{Name: action.EventNew})

	assert.Equal(t, 2, ran)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "20-fail")
	assert.NotEmpty(t, logs, "a failure should be logged")

	body, _ := os.ReadFile(out)
	assert.Equal(t, "ok\n", string(body), "the first script still ran")
}

func TestFireVerboseLogs(t *testing.T) {
	requireUnix(t)
	dir := t.TempDir()
	writeScript(t, dir, "10-a", "#!/bin/sh\n", true)
	writeScript(t, dir, "20-b", "#!/bin/sh\n", true)

	var logs []string
	r := &Runner{Dir: dir, Verbose: true, Logf: func(f string, a ...any) {
		logs = append(logs, fmt.Sprintf(f, a...))
	}}
	_, err := r.Fire(context.Background(), action.Event{
		Name: action.EventNew, Message: action.Message{ID: "m1", Subject: "Hi there"},
	})
	require.NoError(t, err)

	joined := strings.Join(logs, "\n")
	assert.Contains(t, joined, `fire new-message m1 "Hi there"`)
	assert.Contains(t, joined, "run 10-a")
	assert.Contains(t, joined, "run 20-b")
}

func TestFireQuietByDefault(t *testing.T) {
	requireUnix(t)
	dir := t.TempDir()
	writeScript(t, dir, "10-a", "#!/bin/sh\n", true)

	var logs []string
	r := &Runner{Dir: dir, Logf: func(f string, a ...any) { logs = append(logs, f) }}
	_, err := r.Fire(context.Background(), action.Event{Name: action.EventNew, Message: action.Message{ID: "m1"}})
	require.NoError(t, err)
	assert.Empty(t, logs, "a successful non-verbose Fire should log nothing")
}

func TestFireTimeout(t *testing.T) {
	requireUnix(t)
	dir := t.TempDir()
	writeScript(t, dir, "10-sleep", "#!/bin/sh\nsleep 5\n", true)

	r := &Runner{Dir: dir, Timeout: 100 * time.Millisecond}
	start := time.Now()
	ran, err := r.Fire(context.Background(), action.Event{Name: action.EventNew})
	elapsed := time.Since(start)

	assert.Equal(t, 1, ran)
	require.Error(t, err)
	assert.Less(t, elapsed, 4*time.Second, "the slow script should be killed near the timeout")
}

func TestFireNewOrderAndCap(t *testing.T) {
	requireUnix(t)
	dir := t.TempDir()
	out := filepath.Join(t.TempDir(), "ids.txt")
	writeScript(t, dir, "10-log", "#!/bin/sh\necho \"$MELIAFTS_ID\" >> "+out+"\n", true)

	r := &Runner{Dir: dir, Max: 2}
	// Newest-first input, as the store returns it.
	fired, err := r.FireNew(context.Background(), "q",
		[]store.Message{{ID: "m5"}, {ID: "m4"}, {ID: "m3"}, {ID: "m2"}, {ID: "m1"}})
	require.NoError(t, err)
	assert.Equal(t, 2, fired)

	// The most recent two (m5, m4) fire, oldest-first.
	body, _ := os.ReadFile(out)
	assert.Equal(t, "m4\nm5\n", string(body))
}

func TestFireFilter(t *testing.T) {
	requireUnix(t)
	dir := t.TempDir()
	out := filepath.Join(t.TempDir(), "out.txt")
	writeScript(t, dir, "10-a", "#!/bin/sh\necho a >> "+out+"\n", true)
	writeScript(t, dir, "20-b", "#!/bin/sh\necho b >> "+out+"\n", true)

	fire := func(filter ...string) string {
		_ = os.Remove(out)
		r := &Runner{Dir: dir, Filter: filter}
		_, err := r.Fire(context.Background(), action.Event{Name: action.EventNew})
		require.NoError(t, err)
		body, _ := os.ReadFile(out)
		return string(body)
	}

	assert.Equal(t, "a\nb\n", fire(), "no filter runs all scripts")
	assert.Equal(t, "b\n", fire("20-b"), "exact filename allow-list")
	assert.Equal(t, "a\n", fire("10-*"), "glob match")
	assert.Equal(t, "a\nb\n", fire("10-a", "20-b"), "repeatable: union of patterns")
	assert.Equal(t, "", fire("nope"), "a filter matching nothing runs nothing")
}

func TestFireNewFilterExcludesAll(t *testing.T) {
	requireUnix(t)
	dir := t.TempDir()
	writeScript(t, dir, "10-a", "#!/bin/sh\n", true)

	r := &Runner{Dir: dir, Filter: []string{"nope"}}
	fired, err := r.FireNew(context.Background(), "q", []store.Message{{ID: "m1"}, {ID: "m2"}})
	require.NoError(t, err)
	assert.Equal(t, 0, fired, "no script ran, so no message counts as fired")
}

func TestScaffold(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "actions")
	path, err := Scaffold(dir)
	require.NoError(t, err)
	assert.Equal(t, "new-message.sample", filepath.Base(path))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Zero(t, info.Mode().Perm()&0o111, "the sample must not be executable")

	// A scaffolded directory is not yet "enabled" (only the sample is present).
	assert.False(t, (&Runner{Dir: dir}).Enabled())

	// Scaffolding again is a no-op and does not clobber an edited sample.
	require.NoError(t, os.WriteFile(path, []byte("custom"), 0o644))
	_, err = Scaffold(dir)
	require.NoError(t, err)
	body, _ := os.ReadFile(path)
	assert.Equal(t, "custom", string(body))
}

func TestDefaultDir(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/xdg/data")
	assert.Equal(t, filepath.Join("/xdg/data", "meliafts", "actions"), DefaultDir())

	t.Setenv("XDG_DATA_HOME", "")
	assert.True(t, strings.HasSuffix(DefaultDir(),
		filepath.Join(".local", "share", "meliafts", "actions")), "got %q", DefaultDir())
}
