package sampledb

import (
	"context"
	"encoding/json"
	"os"
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

// TestBuildFromProfileRatios checks that two statistical properties are
// reproduced from explicit profile inputs: how many messages carry a full
// body_text (with_text — melia loads bodies lazily) and how many are
// cross-folder duplicates (total - distinct_message_id).
func TestBuildFromProfileRatios(t *testing.T) {
	ctx := context.Background()
	p := &profile.Profile{
		Accounts: 1,
		Folders: []profile.Folder{
			{Type: "inbox", Messages: 400},
			{Type: "archive", Messages: 400}, // All Mail
		},
		Messages: profile.Messages{
			Total:             800,
			DistinctMessageID: 760, // 40 duplicates ⇒ 5%
			WithText:          80,  // 10% have a stored body
			FirstDate:         "2024-01-01 00:00:00",
			LastDate:          "2024-12-31 00:00:00",
		},
	}
	out := filepath.Join(t.TempDir(), "out.db")
	require.NoError(t, BuildFromProfile(ctx, out, p, Options{Seed: 1}))
	got := profileOf(t, out)

	assert.Equal(t, 800, got.Messages.Total)
	assert.InDelta(t, ratioOf(80, 800), ratioOf(got.Messages.WithText, 800), 0.04, "with_text ratio honored")
	assert.InDelta(t, ratioOf(40, 800), ratioOf(800-got.Messages.DistinctMessageID, 800), 0.04, "dedup ratio honored")
	assert.Greater(t, got.Messages.Snippet.Avg, 0.0, "snippets present even where body_text is empty")
}

// TestBuildFromRealProfile reproduces a profile captured from a real melia
// database (testdata/profile.json: ~10.5k messages, 14 folders including an
// archive/"All Mail" and several empty null-type folders). It guards things the
// small synthetic profiles above cannot:
//
//   - Scale regression: generating thousands of rows in one transaction makes
//     SQLite spill a statement journal. With TMPDIR unset that open used to fail
//     with SQLITE_CANTOPEN; opening the database with temp_store=memory
//     (openForWrite) fixes it. We unset TMPDIR here so the test would fail
//     without that fix.
//   - Structural fidelity at a realistic shape: folder layout (incl. empty and
//     null-type folders), total count, archive→custom mapping.
//   - Statistical fidelity: the with_text (mostly-empty bodies, snippet only)
//     and distinct_message_id (sparse cross-folder dedup) ratios are reproduced,
//     not just "present".
func TestBuildFromRealProfile(t *testing.T) {
	t.Setenv("TMPDIR", "") // simulate a minimal env; see the CANTOPEN note above

	b, err := os.ReadFile("testdata/profile.json")
	require.NoError(t, err)
	var p profile.Profile
	require.NoError(t, json.Unmarshal(b, &p))

	out := filepath.Join(t.TempDir(), "real.db")
	require.NoError(t, BuildFromProfile(context.Background(), out, &p, Options{Seed: 1}))
	got := profileOf(t, out)

	assert.Equal(t, p.Messages.Total, got.Messages.Total, "total reproduced exactly")
	assert.Equal(t, len(p.Folders), len(got.Folders), "every folder reproduced, incl. empty and null-type")
	assert.Equal(t, 13, got.SchemaVersion)

	want, have := foldersByType(&p), foldersByType(got)
	assert.Equal(t, want["inbox"], have["inbox"], "inbox size preserved")
	assert.Equal(t, want["archive"], have["custom"], "archive/All Mail reproduced as custom")
	assert.Equal(t, want["(null)"], have["(null)"], "null-type folders preserved")

	// with_text: real melia stores the body lazily, so almost every message has
	// only a snippet. The reproduced ratio tracks the profile (~0.7%), nowhere
	// near the old 100%, and snippets are present even where body_text is empty.
	assert.InDelta(t, ratioOf(p.Messages.WithText, p.Messages.Total),
		ratioOf(got.Messages.WithText, got.Messages.Total), 0.01, "with_text ratio reproduced")
	assert.Greater(t, got.Messages.Snippet.Avg, 0.0, "snippets present despite empty bodies")

	// distinct_message_id: only the All Mail copies share a Message-ID, so the
	// duplicate ratio is small (~4%), not the old hardcoded ~45%.
	wantDup := ratioOf(p.Messages.Total-p.Messages.DistinctMessageID, p.Messages.Total)
	gotDup := ratioOf(got.Messages.Total-got.Messages.DistinctMessageID, got.Messages.Total)
	assert.InDelta(t, wantDup, gotDup, 0.015, "duplicate ratio reproduced")
}

// ratioOf returns n/total as a float, or 0 when total is 0.
func ratioOf(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total)
}
