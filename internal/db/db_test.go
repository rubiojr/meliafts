package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestOpenReadOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "melia.db")

	// Create a writable database with a row.
	w, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	_, err = w.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)`)
	require.NoError(t, err)
	_, err = w.Exec(`INSERT INTO t (v) VALUES ('hello')`)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	// Open read-only and read the row back.
	ro, err := OpenReadOnly(path)
	require.NoError(t, err)
	t.Cleanup(func() { ro.Close() })

	var v string
	require.NoError(t, ro.QueryRow(`SELECT v FROM t WHERE id = 1`).Scan(&v))
	assert.Equal(t, "hello", v)

	// Writes must be rejected.
	_, err = ro.Exec(`INSERT INTO t (v) VALUES ('nope')`)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "readonly")
}

func TestOpenReadOnlyMissing(t *testing.T) {
	_, err := OpenReadOnly(filepath.Join(t.TempDir(), "does-not-exist.db"))
	require.Error(t, err)
	assert.ErrorContains(t, err, "cannot open database")
}

func TestDefaultPath(t *testing.T) {
	p := DefaultPath()
	require.NotEmpty(t, p)
	assert.Contains(t, p, filepath.FromSlash(".var/app/com.buxjr.melia/config/melia/melia.db"))
}
