package dev

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// runGendbArgs invokes the gendb command in-process with the given arguments
// (excluding the command name).
func runGendbArgs(t *testing.T, args ...string) error {
	t.Helper()
	return gendbCommand.Run(context.Background(), append([]string{"gendb"}, args...))
}

// TestGendbFailedRunPreservesOutput asserts that a gendb run which fails after
// the output path already exists leaves that file byte-for-byte intact. It
// previously deleted the output before reading its inputs, so a bad --profile
// silently destroyed an existing database.
func TestGendbFailedRunPreservesOutput(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "melia.db")

	require.NoError(t, runGendbArgs(t, "-o", out, "-n", "5"))
	before, err := os.ReadFile(out)
	require.NoError(t, err)
	require.NotEmpty(t, before)

	bad := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(bad, []byte("not valid json"), 0o644))

	err = runGendbArgs(t, "-o", out, "--profile", bad)
	require.Error(t, err, "malformed profile should fail")

	after, err := os.ReadFile(out)
	require.NoError(t, err, "existing output must survive a failed run")
	require.Equal(t, before, after, "existing output must be unchanged after a failed run")
}

// TestGendbFromDBNonexistentPreservesOutput covers the other failure path: an
// unreadable --from-db source must not take the existing output with it.
func TestGendbFromDBNonexistentPreservesOutput(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "melia.db")

	require.NoError(t, runGendbArgs(t, "-o", out, "-n", "5"))
	before, err := os.ReadFile(out)
	require.NoError(t, err)

	err = runGendbArgs(t, "-o", out, "--from-db", filepath.Join(dir, "nope.db"))
	require.Error(t, err)

	after, err := os.ReadFile(out)
	require.NoError(t, err, "existing output must survive a failed --from-db run")
	require.Equal(t, before, after)
}

// TestGendbFromDBSameAsOutput guards the worst manifestation: pointing
// --from-db and --output at the same path must not delete the database.
func TestGendbFromDBSameAsOutput(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "melia.db")

	require.NoError(t, runGendbArgs(t, "-o", out, "-n", "5"))

	err := runGendbArgs(t, "--from-db", out, "-o", out)
	require.NoError(t, err, "reproducing a db in place should succeed")

	info, err := os.Stat(out)
	require.NoError(t, err, "the source/output database must still exist")
	require.NotZero(t, info.Size())
}

// TestBuildToTempUsesGivenDir verifies buildToTemp is self-contained: it creates
// the temporary database inside the directory it is given (so a test can keep
// everything under t.TempDir()) and cleans it up via removeTemp.
func TestBuildToTempUsesGivenDir(t *testing.T) {
	dir := t.TempDir()

	tmp, err := buildToTemp(context.Background(), dir, nil, 1, 5)
	require.NoError(t, err)
	require.Equal(t, dir, filepath.Dir(tmp), "temp file must be created in the given dir")

	info, err := os.Stat(tmp)
	require.NoError(t, err)
	require.NotZero(t, info.Size(), "generated database must not be empty")

	removeTemp(tmp)
	_, err = os.Stat(tmp)
	require.ErrorIs(t, err, os.ErrNotExist, "removeTemp must delete the temp file")
}
