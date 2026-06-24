package dev

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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
profile JSON, and --save-profile writes the (content-free) profile it used.`,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Value: "melia.db", Usage: "output database path (replaced if it exists)"},
		&cli.IntFlag{Name: "count", Aliases: []string{"n"}, Value: 140, Usage: "number of random messages (ignored with --from-db/--profile)"},
		&cli.Int64Flag{Name: "seed", Value: 1, Usage: "random seed for reproducibility"},
		&cli.StringFlag{Name: "from-db", Usage: "profile this real melia database and reproduce its structure"},
		&cli.StringFlag{Name: "profile", Usage: "reproduce the structure from this profile JSON file"},
		&cli.StringFlag{Name: "save-profile", Usage: "write the profile used to this JSON file"},
	},
	Action: runGendb,
}

func runGendb(ctx context.Context, cmd *cli.Command) error {
	out := cmd.String("output")
	if err := os.Remove(out); err != nil && !os.IsNotExist(err) {
		return err
	}

	prof, err := loadProfile(cmd)
	if err != nil {
		return err
	}
	if prof != nil && cmd.String("save-profile") != "" {
		if err := writeProfile(prof, cmd.String("save-profile")); err != nil {
			return err
		}
	}

	seed := cmd.Int64("seed")
	if prof != nil {
		if err := sampledb.BuildFromProfile(ctx, out, prof, sampledb.Options{Seed: seed}); err != nil {
			return err
		}
		fmt.Printf("wrote %s reproducing %d messages across %d folders (seed %d)\n",
			out, prof.Messages.Total, len(prof.Folders), seed)
		return nil
	}

	count := cmd.Int("count")
	if err := sampledb.Build(ctx, out, sampledb.Options{Seed: seed, Messages: count, Now: time.Now()}); err != nil {
		return err
	}
	fmt.Printf("wrote %s (%d random + curated messages, seed %d)\n", out, count, seed)
	return nil
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
