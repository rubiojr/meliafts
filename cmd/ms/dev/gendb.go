package dev

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rubiojr/meliafts/internal/db"
	"github.com/rubiojr/meliafts/internal/profile"
	"github.com/rubiojr/meliafts/internal/sampledb"
	"github.com/urfave/cli/v3"
)

var gendbCommand = &cli.Command{
	Name:  "gendb",
	Usage: "Generate a sample melia database for demos and testing",
	Description: `Write a sample melia database populated with synthetic emails.

By default it generates random + curated demo messages. With --from-db it instead
profiles a real melia database (read-only, reading only aggregate structure —
never content) and reproduces that structure: message counts, folder layout,
date range, flag ratios and "All Mail" duplication. --profile reproduces a saved
profile JSON, and --save-profile writes the (content-free) structure profile of
the generated database.`,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Value: "melia.db", Usage: "output database path (replaced if it exists)"},
		&cli.IntFlag{Name: "count", Aliases: []string{"n"}, Value: 140, Usage: "number of random messages (ignored with --from-db/--profile)"},
		&cli.Int64Flag{Name: "seed", Value: 1, Usage: "random seed for reproducibility"},
		&cli.StringFlag{Name: "from-db", Usage: "profile this real melia database and reproduce its structure"},
		&cli.StringFlag{Name: "profile", Usage: "reproduce the structure from this profile JSON file"},
		&cli.StringFlag{Name: "save-profile", Usage: "write the generated database's (content-free) structure profile to this JSON file"},
	},
	Action: runGendb,
}

func runGendb(ctx context.Context, cmd *cli.Command) error {
	out := cmd.String("output")

	// Read and validate the inputs (--from-db / --profile) before touching the
	// destination, so an invalid source can never delete an existing output file.
	prof, err := loadProfile(cmd)
	if err != nil {
		return err
	}

	// Generate into a fresh temporary file next to the destination and only move
	// it into place once everything succeeds. A failed run therefore leaves any
	// existing output file untouched instead of destroying it. The temp file
	// lives in the destination directory so the final rename is atomic.
	tmp, err := buildToTemp(ctx, filepath.Dir(out), prof, cmd.Int64("seed"), cmd.Int("count"))
	if err != nil {
		return err
	}

	if err := maybeSaveProfile(cmd, tmp, prof); err != nil {
		removeTemp(tmp)
		return err
	}

	if err := os.Rename(tmp, out); err != nil {
		removeTemp(tmp)
		return fmt.Errorf("write %s: %w", out, err)
	}

	reportWritten(cmd, out, prof)
	return nil
}

// buildToTemp generates the database into a fresh temporary file inside dir and
// returns its path for the caller to rename into place. dir should normally be
// the destination's directory so the rename is atomic; tests can pass their own
// (e.g. t.TempDir()) to stay self-contained. On any error it cleans up the
// temporary file and returns the error.
func buildToTemp(ctx context.Context, dir string, prof *profile.Profile, seed int64, count int) (string, error) {
	f, err := os.CreateTemp(dir, ".ms-gendb-*.tmp")
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	f.Close()
	_ = os.Chmod(tmp, 0o644)

	if prof != nil {
		err = sampledb.BuildFromProfile(ctx, tmp, prof, sampledb.Options{Seed: seed})
	} else {
		err = sampledb.Build(ctx, tmp, sampledb.Options{Seed: seed, Messages: count, Now: time.Now()})
	}
	if err != nil {
		removeTemp(tmp)
		return "", err
	}
	return tmp, nil
}

// removeTemp deletes a partially written database and any SQLite sidecar files.
func removeTemp(path string) {
	for _, p := range []string{path, path + "-journal", path + "-wal", path + "-shm"} {
		_ = os.Remove(p)
	}
}

// reportWritten prints the success line, which differs for profile reproduction
// versus random generation.
func reportWritten(cmd *cli.Command, out string, prof *profile.Profile) {
	seed := cmd.Int64("seed")
	if prof != nil {
		fmt.Printf("wrote %s reproducing %d messages across %d folders (seed %d)\n",
			out, prof.Messages.Total, len(prof.Folders), seed)
		return
	}
	fmt.Printf("wrote %s (%d random + curated messages, seed %d)\n", out, cmd.Int("count"), seed)
}

// loadProfile returns the profile to reproduce, or nil for the random generator.
func loadProfile(cmd *cli.Command) (*profile.Profile, error) {
	switch {
	case cmd.String("from-db") != "":
		return profileFromDB(cmd.String("from-db"))
	case cmd.String("profile") != "":
		return readProfile(cmd.String("profile"))
	default:
		return nil, nil
	}
}

// maybeSaveProfile honours --save-profile in every mode. For --from-db/--profile
// it writes the input profile; for random generation (prof == nil) it profiles
// the freshly generated database at dbPath, so the flag is never silently
// ignored.
func maybeSaveProfile(cmd *cli.Command, dbPath string, prof *profile.Profile) error {
	path := cmd.String("save-profile")
	if path == "" {
		return nil
	}
	p := prof
	if p == nil {
		var err error
		if p, err = profileFromDB(dbPath); err != nil {
			return err
		}
	}
	return writeProfile(p, path)
}

func profileFromDB(path string) (*profile.Profile, error) {
	d, err := db.OpenReadOnly(path)
	if err != nil {
		return nil, err
	}
	defer d.Close()
	return profile.Collect(d)
}

func readProfile(path string) (*profile.Profile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p profile.Profile
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, fmt.Errorf("parse profile %s: %w", path, err)
	}
	return &p, nil
}

func writeProfile(p *profile.Profile, path string) error {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
