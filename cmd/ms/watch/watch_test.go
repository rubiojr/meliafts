package watch

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// runWatch invokes the watch command in-process with the given arguments
// (excluding the command name).
func runWatch(t *testing.T, args ...string) error {
	t.Helper()
	return Command.Run(context.Background(), append([]string{"watch"}, args...))
}

// TestWatchInvalidQueryFailsBeforeScaffolding asserts that an invalid query is
// reported (non-zero exit) even when the actions directory has no runnable
// scripts. Previously watch scaffolded a template and exited 0 without ever
// validating the query.
func TestWatchInvalidQueryFailsBeforeScaffolding(t *testing.T) {
	dir := t.TempDir()

	err := runWatch(t, "--actions-dir", dir, "in:bogusfolder")
	require.Error(t, err, "an invalid query must be reported")

	entries, rerr := os.ReadDir(dir)
	require.NoError(t, rerr)
	require.Empty(t, entries, "an invalid query must not scaffold a template")
}

// TestWatchValidQueryWithoutScriptsScaffolds ensures the scaffold-on-empty
// behaviour still works for a valid query (here, no query at all).
func TestWatchValidQueryWithoutScriptsScaffolds(t *testing.T) {
	dir := t.TempDir()

	err := runWatch(t, "--actions-dir", dir)
	require.NoError(t, err)

	entries, rerr := os.ReadDir(dir)
	require.NoError(t, rerr)
	require.NotEmpty(t, entries, "a valid query with no scripts should scaffold a template")
}
