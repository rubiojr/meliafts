package db

import (
	"database/sql"
	"os"
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

func TestCandidatePaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")

	got := candidatePaths()
	require.Len(t, got, 4)
	assert.Equal(t, filepath.Join(home, ".var/app/com.buxjr.melia/config/melia/melia.db"), got[0])
	assert.Equal(t, filepath.Join(home, "snap/melia/current/.config/melia/melia.db"), got[1])
	assert.Equal(t, filepath.Join(home, "snap/melia/common/.config/melia/melia.db"), got[2])
	assert.Equal(t, filepath.Join(home, ".config", "melia", "melia.db"), got[3])
}

func TestCandidatePathsHonorsXDGConfigHome(t *testing.T) {
	home := t.TempDir()
	cfg := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", cfg)

	got := candidatePaths()
	require.Len(t, got, 4)
	assert.Equal(t, filepath.Join(cfg, "melia", "melia.db"), got[3])
}

func TestFirstExistingOrPrimary(t *testing.T) {
	dir := t.TempDir()
	primary := filepath.Join(dir, "flatpak.db")
	secondary := filepath.Join(dir, "config.db")
	candidates := []string{primary, secondary}

	// Nothing exists yet: fall back to the primary (first) candidate.
	assert.Equal(t, primary, firstExistingOrPrimary(candidates))

	// Only the secondary exists: it is selected.
	require.NoError(t, os.WriteFile(secondary, []byte("x"), 0o644))
	assert.Equal(t, secondary, firstExistingOrPrimary(candidates))

	// The primary now exists too: it takes priority.
	require.NoError(t, os.WriteFile(primary, []byte("x"), 0o644))
	assert.Equal(t, primary, firstExistingOrPrimary(candidates))

	// An empty list yields no path.
	assert.Equal(t, "", firstExistingOrPrimary(nil))
}

func TestFirstExistingOrPrimarySkipsDirectories(t *testing.T) {
	dir := t.TempDir()
	primary := filepath.Join(dir, "flatpak.db")
	secondary := filepath.Join(dir, "config.db")
	require.NoError(t, os.Mkdir(primary, 0o755)) // a directory must not count
	require.NoError(t, os.WriteFile(secondary, []byte("x"), 0o644))

	assert.Equal(t, secondary, firstExistingOrPrimary([]string{primary, secondary}))
}

func TestDefaultPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")

	// With no database present, it falls back to the Flatpak location.
	flatpak := filepath.Join(home, ".var/app/com.buxjr.melia/config/melia/melia.db")
	assert.Equal(t, flatpak, DefaultPath())

	// A non-Flatpak database under ~/.config is picked up instead.
	cfgDB := filepath.Join(home, ".config", "melia", "melia.db")
	require.NoError(t, os.MkdirAll(filepath.Dir(cfgDB), 0o755))
	require.NoError(t, os.WriteFile(cfgDB, []byte("x"), 0o644))
	assert.Equal(t, cfgDB, DefaultPath())
}

func TestDefaultPathPrefersSnapOverConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")

	mkdb := func(p string) {
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte("x"), 0o644))
	}
	snapDB := filepath.Join(home, "snap/melia/current/.config/melia/melia.db")
	cfgDB := filepath.Join(home, ".config", "melia", "melia.db")
	mkdb(cfgDB)
	mkdb(snapDB)

	// Both present: the Snap location takes priority over the plain config one.
	assert.Equal(t, snapDB, DefaultPath())
}
