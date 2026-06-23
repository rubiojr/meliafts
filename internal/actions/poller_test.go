package actions

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	meliadb "github.com/rubiojr/meliafts/db"
	"github.com/rubiojr/meliafts/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// watchDB builds a melia database and returns its path plus an open writable
// handle, so a test can insert new messages between polls to simulate mail
// arriving. The store side is opened separately, read-only.
func watchDB(t *testing.T) (string, *sql.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "melia.db")
	w, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	require.NoError(t, meliadb.Apply(context.Background(), w))
	_, err = w.Exec(`INSERT INTO accounts
		(id,name,email,type,incoming_host,incoming_port,incoming_security,
		 outgoing_host,outgoing_port,outgoing_security,username,account_is_default)
		VALUES ('a','me','me@x','imap','h',993,'ssl','h',465,'ssl','me@x',1)`)
	require.NoError(t, err)
	_, err = w.Exec(`INSERT INTO folders (id,account_id,name,path,type)
		VALUES ('f-inbox','a','Inbox','INBOX','inbox')`)
	require.NoError(t, err)
	t.Cleanup(func() { w.Close() })
	return path, w
}

// insertMsg adds a message dated 2026-01-NN so later ids sort newer.
func insertMsg(t *testing.T, w *sql.DB, id, subject string, day int) {
	t.Helper()
	_, err := w.Exec(`INSERT INTO messages
		(id,account_id,folder_id,message_id,from_address,from_name,to_addresses,
		 subject,snippet,date,is_read)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		id, "a", "f-inbox", "<"+id+"@x>", "sender@x", "Sender",
		`[{"address":"me@x"}]`, subject, "snippet", fmt.Sprintf("2026-01-%02d 10:00:00", day), 0)
	require.NoError(t, err)
}

// logRunner returns a Runner whose single script appends MELIAFTS_ID to a file,
// and the path of that file.
func logRunner(t *testing.T, max int) (*Runner, string) {
	t.Helper()
	requireUnix(t)
	dir := t.TempDir()
	out := filepath.Join(t.TempDir(), "fired.txt")
	writeScript(t, dir, "10-log", "#!/bin/sh\necho \"$MELIAFTS_ID\" >> "+out+"\n", true)
	return &Runner{Dir: dir, Max: max}, out
}

func firedIDs(t *testing.T, out string) []string {
	t.Helper()
	body, err := os.ReadFile(out)
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)
	var ids []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(body)), "\n") {
		if line != "" {
			ids = append(ids, line)
		}
	}
	return ids
}

func openStore(t *testing.T, path string) *store.Store {
	t.Helper()
	st, err := store.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	return st
}

func TestPollerPrimesThenFires(t *testing.T) {
	path, w := watchDB(t)
	insertMsg(t, w, "m01", "one", 1)
	insertMsg(t, w, "m02", "two", 2)

	runner, out := logRunner(t, 0)
	p := &Poller{Store: openStore(t, path), Runner: runner, Limit: 100}

	// First tick primes the baseline silently.
	fired, err := p.Tick(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, fired)
	assert.Empty(t, firedIDs(t, out))

	// New mail arrives; the next tick fires for it only.
	insertMsg(t, w, "m03", "three", 3)
	fired, err = p.Tick(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, fired)
	assert.Equal(t, []string{"m03"}, firedIDs(t, out))

	// A tick with nothing new fires nothing.
	fired, err = p.Tick(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, fired)
}

func TestPollerFireExisting(t *testing.T) {
	path, w := watchDB(t)
	insertMsg(t, w, "m01", "one", 1)
	insertMsg(t, w, "m02", "two", 2)

	runner, out := logRunner(t, 0)
	p := &Poller{Store: openStore(t, path), Runner: runner, Limit: 100, FireExisting: true}

	fired, err := p.Tick(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, fired)
	// Fired oldest-first (results arrive newest-first and are reversed).
	assert.Equal(t, []string{"m01", "m02"}, firedIDs(t, out))
}

func TestPollerCap(t *testing.T) {
	path, w := watchDB(t)
	for i := 1; i <= 5; i++ {
		insertMsg(t, w, fmt.Sprintf("m%02d", i), "subject", i)
	}

	runner, out := logRunner(t, 2) // cap at 2 per batch
	p := &Poller{Store: openStore(t, path), Runner: runner, Limit: 100, FireExisting: true}

	fired, err := p.Tick(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, fired)
	// The cap keeps the most recent messages.
	assert.Equal(t, []string{"m04", "m05"}, firedIDs(t, out))
}

func TestPollerQueryScope(t *testing.T) {
	path, w := watchDB(t)
	insertMsg(t, w, "m01", "your invoice is ready", 1)
	insertMsg(t, w, "m02", "lunch on friday", 2)

	runner, out := logRunner(t, 0)
	p := &Poller{Store: openStore(t, path), Runner: runner, Query: "subject:invoice", Limit: 100, FireExisting: true}

	fired, err := p.Tick(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, fired)
	assert.Equal(t, []string{"m01"}, firedIDs(t, out))
}
