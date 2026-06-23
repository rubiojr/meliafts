package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func openTempDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "x.db"))
	require.NoError(t, err)
	d.SetMaxOpenConns(1) // keep PRAGMA state on one connection
	t.Cleanup(func() { d.Close() })
	return d
}

func TestSchemaVersionFromSettings(t *testing.T) {
	d := openTempDB(t)
	_, err := d.Exec(`CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL)`)
	require.NoError(t, err)
	_, err = d.Exec(`INSERT INTO settings VALUES ('schema_version', ?)`, SupportedSchemaVersion)
	require.NoError(t, err)

	v, ok := SchemaVersion(d)
	assert.True(t, ok)
	assert.Equal(t, SupportedSchemaVersion, v)
	assert.NoError(t, CheckSchema(d))
}

func TestSchemaVersionDrift(t *testing.T) {
	d := openTempDB(t)
	_, err := d.Exec(`CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL)`)
	require.NoError(t, err)
	_, err = d.Exec(`INSERT INTO settings VALUES ('schema_version', '999')`)
	require.NoError(t, err)

	err = CheckSchema(d)
	var se *SchemaError
	require.ErrorAs(t, err, &se)
	assert.True(t, se.Found)
	assert.Equal(t, 999, se.Got)
	assert.Contains(t, err.Error(), "unsupported melia schema version 999")
}

func TestSchemaVersionUserVersionFallback(t *testing.T) {
	d := openTempDB(t)
	// No settings table; the version lives in PRAGMA user_version instead.
	_, err := d.Exec("PRAGMA user_version = 13")
	require.NoError(t, err)

	v, ok := SchemaVersion(d)
	assert.True(t, ok)
	assert.Equal(t, 13, v)
}

func TestSchemaVersionMissing(t *testing.T) {
	d := openTempDB(t) // no settings, user_version defaults to 0

	v, ok := SchemaVersion(d)
	assert.False(t, ok)
	assert.Zero(t, v)

	err := CheckSchema(d)
	var se *SchemaError
	require.ErrorAs(t, err, &se)
	assert.False(t, se.Found)
	assert.Contains(t, err.Error(), "could not determine")
}
