package sampledb

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/rubiojr/meliafts/internal/db"
	"github.com/rubiojr/meliafts/internal/profile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func profileOf(t *testing.T, path string) *profile.Profile {
	t.Helper()
	d, err := db.OpenReadOnly(path)
	require.NoError(t, err)
	defer d.Close()
	p, err := profile.Collect(d)
	require.NoError(t, err)
	return p
}

func foldersByType(p *profile.Profile) map[string]int {
	out := map[string]int{}
	for _, f := range p.Folders {
		out[f.Type] += f.Messages
	}
	return out
}

func TestBuildFromProfileRoundTrip(t *testing.T) {
	ctx := context.Background()

	src := filepath.Join(t.TempDir(), "src.db")
	require.NoError(t, Build(ctx, src, Options{Seed: 1, Messages: 40, Now: time.Now()}))
	want := profileOf(t, src)

	out := filepath.Join(t.TempDir(), "out.db")
	require.NoError(t, BuildFromProfile(ctx, out, want, Options{Seed: 2}))
	got := profileOf(t, out)

	assert.Equal(t, want.Messages.Total, got.Messages.Total, "total reproduced exactly")
	assert.Equal(t, foldersByType(want), foldersByType(got), "per-type folder sizes reproduced")
	assert.Equal(t, 13, got.SchemaVersion, "generated DB is stamped with the supported schema version")
}

func TestBuildFromProfileDeduplicates(t *testing.T) {
	ctx := context.Background()
	p := &profile.Profile{
		Accounts: 1,
		Folders: []profile.Folder{
			{Type: "inbox", Messages: 12, Unread: 4},
			{Type: "spam", Messages: 3, Unread: 3},
			{Type: "archive", Messages: 12, Unread: 1}, // "All Mail" — duplicates the rest
		},
		Messages: profile.Messages{
			Total: 27, FirstDate: "2024-01-01 00:00:00", LastDate: "2024-12-31 00:00:00",
			Flagged: 2, HasAttachments: 3, WithHTML: 6,
		},
	}

	out := filepath.Join(t.TempDir(), "out.db")
	require.NoError(t, BuildFromProfile(ctx, out, p, Options{Seed: 1}))
	got := profileOf(t, out)

	assert.Equal(t, 27, got.Messages.Total, "total row count reproduced")
	assert.Less(t, got.Messages.DistinctMessageID, 27, "All Mail copies share a Message-ID (dedup structure)")

	// The folder layout is reproduced (archive maps to a schema-valid custom type).
	bt := foldersByType(got)
	assert.Equal(t, 12, bt["inbox"])
	assert.Equal(t, 3, bt["spam"])
	assert.Equal(t, 12, bt["custom"], "archive/All Mail stored as custom under the embedded schema")
}

func TestBuildFromProfileDateRange(t *testing.T) {
	ctx := context.Background()
	p := &profile.Profile{
		Accounts: 1,
		Folders:  []profile.Folder{{Type: "inbox", Messages: 30}},
		Messages: profile.Messages{Total: 30, FirstDate: "2020-06-01 00:00:00", LastDate: "2020-06-30 23:59:59"},
	}
	out := filepath.Join(t.TempDir(), "out.db")
	require.NoError(t, BuildFromProfile(ctx, out, p, Options{Seed: 1}))
	got := profileOf(t, out)

	assert.GreaterOrEqual(t, got.Messages.FirstDate, "2020-06-01")
	assert.LessOrEqual(t, got.Messages.LastDate, "2020-07-01")
}
