// Package watch implements the `ms watch` subcommand: it polls the melia
// database for new messages matching a query and runs user action scripts for
// them. See docs/actions.md (user) and docs/architecture/actions.md (developer).
package watch

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/rubiojr/meliafts/cmd/ms/util"
	"github.com/rubiojr/meliafts/internal/actions"
	"github.com/rubiojr/meliafts/internal/store"
	"github.com/urfave/cli/v3"
)

// Command is the `ms watch` subcommand.
var Command = &cli.Command{
	Name:      "watch",
	Usage:     "Watch for new messages and run action scripts",
	ArgsUsage: "[query]",
	Description: `Poll the melia database for new messages and run action scripts.

Each time a new message matches the (optional) query, every executable script in
the actions directory (default ~/.local/share/meliafts/actions) is run with the
message supplied as MELIAFTS_* environment variables and as JSON on stdin.

The first poll silently records what already matches so old mail is not replayed;
pass --fire-existing to opt out. Run 'ms watch' once with no scripts to scaffold
a template you can edit and chmod +x. See docs/actions.md for the full guide.`,
	Flags: []cli.Flag{
		&cli.DurationFlag{Name: "interval", Aliases: []string{"i"}, Value: 30 * time.Second, Usage: "poll interval (e.g. 30s, 2m)"},
		&cli.IntFlag{Name: "limit", Aliases: []string{"n"}, Value: 100, Usage: "messages scanned per poll"},
		&cli.StringFlag{Name: "actions-dir", Value: actions.DefaultDir(), Usage: "directory of action scripts"},
		&cli.IntFlag{Name: "actions-max", Value: actions.DefaultMax, Usage: "most scripts fired per poll"},
		&cli.DurationFlag{Name: "timeout", Value: actions.DefaultTimeout, Usage: "per-script timeout"},
		&cli.StringSliceFlag{Name: "actions-filter", Usage: "only run scripts whose filename matches this glob (repeatable; allow-list)"},
		&cli.BoolFlag{Name: "fire-existing", Usage: "also fire for mail already present on the first poll"},
		&cli.BoolFlag{Name: "once", Usage: "poll a single time and exit"},
		&cli.BoolFlag{Name: "verbose", Aliases: []string{"v"}, Usage: "log each event and the scripts it runs to stderr"},
	},
	Action: run,
}

func run(ctx context.Context, cmd *cli.Command) error {
	dir := cmd.String("actions-dir")
	runner := &actions.Runner{
		Dir:     dir,
		DBPath:  cmd.String("db"),
		Timeout: cmd.Duration("timeout"),
		Max:     cmd.Int("actions-max"),
		Filter:  cmd.StringSlice("actions-filter"),
		Verbose: cmd.Bool("verbose"),
		Logf:    logf,
	}

	if !runner.Enabled() {
		return scaffold(dir)
	}

	st, err := store.Open(cmd.String("db"))
	if err != nil {
		return err
	}
	defer st.Close()

	if err := util.VerifySchema(cmd, st); err != nil {
		return err
	}

	poller := &actions.Poller{
		Store:        st,
		Runner:       runner,
		Query:        strings.Join(cmd.Args().Slice(), " "),
		Limit:        cmd.Int("limit"),
		FireExisting: cmd.Bool("fire-existing"),
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cmd.Bool("once") {
		_, err := poller.Tick(ctx)
		return err
	}
	return loop(ctx, poller, cmd.Duration("interval"))
}

// loop polls until the context is cancelled. Transient poll errors are logged
// and do not stop the watcher.
func loop(ctx context.Context, p *actions.Poller, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if _, err := p.Tick(ctx); err != nil {
			if ctx.Err() != nil {
				return nil // cancelled during the poll
			}
			logf("poll error: %v", err)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// scaffold writes the sample script and tells the user how to enable it.
func scaffold(dir string) error {
	path, err := actions.Scaffold(dir)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "ms watch: no executable action scripts in %s\n", dir)
	fmt.Fprintf(os.Stderr, "wrote a template to %s\n", path)
	fmt.Fprintln(os.Stderr, "edit a copy without the .sample suffix and `chmod +x` it to enable.")
	return nil
}

func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "ms watch: "+format+"\n", args...)
}
