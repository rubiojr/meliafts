package e2e

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/rubiojr/meliafts/internal/sampledb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// msBin is the path to the freshly built ms binary, set up in TestMain.
var msBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ms-e2e")
	if err != nil {
		panic(err)
	}
	msBin = filepath.Join(dir, "ms")

	build := exec.Command("go", "build", "-o", msBin, "./cmd/ms")
	build.Dir = ".." // repo root
	if out, err := build.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		panic("building ms failed: " + err.Error() + "\n" + string(out))
	}

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// fixtureDB builds a sample database and returns its path. Content is
// deterministic (seeded); dates are spread back from now so relative date
// filters like newer:7d match the curated recent messages.
func fixtureDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "melia.db")
	require.NoError(t, sampledb.Build(context.Background(), path, sampledb.Options{
		Seed: 1, Messages: 80, Now: time.Now(),
	}))
	return path
}

// runMS runs the ms binary and returns combined output and the exit code.
func runMS(t *testing.T, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(msBin, args...)
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("running ms %v: %v", args, err)
	}
	return string(out), code
}

func TestE2E(t *testing.T) {
	db := fixtureDB(t)
	ms := func(args ...string) (string, int) {
		return runMS(t, append([]string{"--db", db}, args...)...)
	}

	t.Run("search json returns hits", func(t *testing.T) {
		out, code := ms("search", "--json", "subject:invoice")
		require.Equal(t, 0, code, out)

		var msgs []map[string]any
		require.NoError(t, json.Unmarshal([]byte(out), &msgs))
		assert.NotEmpty(t, msgs)
	})

	t.Run("folder filter", func(t *testing.T) {
		out, code := ms("search", "--json", "in:sent")
		require.Equal(t, 0, code, out)
		var msgs []map[string]any
		require.NoError(t, json.Unmarshal([]byte(out), &msgs))
		assert.NotEmpty(t, msgs)
		for _, m := range msgs {
			assert.Equal(t, "you@example.com", m["from_address"], "sent items are from the account")
		}
	})

	t.Run("relative date filter", func(t *testing.T) {
		// The curated messages are dated within a few days of the fixture's
		// reference time, so newer:7d must return some of them.
		out, code := ms("search", "--json", "newer:7d")
		require.Equal(t, 0, code, out)
		var msgs []map[string]any
		require.NoError(t, json.Unmarshal([]byte(out), &msgs))
		assert.NotEmpty(t, msgs)
	})

	t.Run("sql flag combines folder and flag", func(t *testing.T) {
		out, code := ms("search", "--sql", "unread:", "in:sent")
		require.Equal(t, 0, code, out)
		assert.Contains(t, out, "m.is_read = ?")
		assert.Contains(t, out, "m.folder_id IN (SELECT id FROM folders WHERE type = ?)")
	})

	t.Run("no results", func(t *testing.T) {
		out, code := ms("search", "subject:zzzznotarealword")
		require.Equal(t, 0, code, out)
		assert.Contains(t, out, "No messages found")
	})

	t.Run("missing db errors", func(t *testing.T) {
		out, code := runMS(t, "--db", "/no/such/melia.db", "search", "hi")
		assert.Equal(t, 1, code)
		assert.Contains(t, out, "cannot open database")
	})
}
