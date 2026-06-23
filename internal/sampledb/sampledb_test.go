package sampledb

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/rubiojr/meliafts/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func buildFixture(t *testing.T, opts Options) *store.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "melia.db")
	require.NoError(t, Build(context.Background(), path, opts))
	st, err := store.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	return st
}

func TestGenerateIsQueryable(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	st := buildFixture(t, Options{Seed: 1, Messages: 60, Now: now})

	all, err := st.Search("", 0, 0)
	require.NoError(t, err)
	assert.Len(t, all, 66, "6 curated + 60 random")

	cases := []struct {
		query string
		min   int
	}{
		{"subject:invoice", 1},
		{"body:kubernetes", 1},
		{"in:sent", 1},
		{"unread:", 1},
		{"flagged:", 1},
		{"from:bob", 1},
		{"in:spam", 1},
		{`subject:privacy`, 1},
	}
	for _, c := range cases {
		t.Run(c.query, func(t *testing.T) {
			res, err := st.Search(c.query, 0, 0)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(res), c.min, "query %q", c.query)
		})
	}

	// The fixture has folders populated with the right types.
	sent, err := st.Search("in:sent", 0, 0)
	require.NoError(t, err)
	for _, m := range sent {
		assert.Equal(t, accountEmail, m.FromAddress, "sent messages are from the account")
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	opts := Options{Seed: 7, Messages: 40, Now: now}

	first := buildFixture(t, opts)
	second := buildFixture(t, opts)

	a, err := first.Search("", 0, 0)
	require.NoError(t, err)
	b, err := second.Search("", 0, 0)
	require.NoError(t, err)

	require.Equal(t, len(a), len(b))
	for i := range a {
		assert.Equal(t, a[i].ID, b[i].ID)
		assert.Equal(t, a[i].Subject, b[i].Subject)
		assert.Equal(t, a[i].Date, b[i].Date)
	}
}
