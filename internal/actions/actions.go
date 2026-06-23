// Package actions runs user-provided "action" scripts when new mail arrives —
// mailbox hooks in the spirit of Git hooks. Because the melia database is only
// ever opened read-only, actions are purely reactive: they observe new messages
// and run external commands (notify, log, webhook, hand-off) but can never
// change, send or delete mail, and cannot create feedback loops.
//
// The package is presentation-free: it knows about the store and the filesystem,
// but nothing about the CLI or the TUI, so the same engine can back both the
// `ms watch` command and (later) the interactive reload loop.
package actions

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rubiojr/meliafts/internal/store"
	"github.com/rubiojr/meliafts/pkg/action"
)

// sampleName is the non-executable template dropped into a fresh actions
// directory. The .sample suffix means it is ignored by the runner.
const sampleName = "new-message.sample"

// Defaults for callers that do not expose every knob (e.g. the TUI).
const (
	DefaultMax     = 25               // most scripts fired per batch
	DefaultTimeout = 10 * time.Second // per-script timeout
)

// Runner discovers and executes the action scripts in a directory.
type Runner struct {
	Dir     string                           // directory holding the scripts
	DBPath  string                           // exported to scripts as MELIAFTS_DB
	Timeout time.Duration                    // per-script timeout (0 disables)
	Max     int                              // cap on fires per batch (0 = unlimited)
	Filter  []string                         // allow-list of script-name globs (empty = all)
	Verbose bool                             // log each event and script via Logf
	Logf    func(format string, args ...any) // optional log sink (may be nil)
}

// DefaultDir returns the default actions directory,
// $XDG_DATA_HOME/meliafts/actions, falling back to ~/.local/share/meliafts/actions.
func DefaultDir() string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "meliafts", "actions")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "meliafts", "actions")
}

// Enabled reports whether the directory holds at least one runnable script.
func (r *Runner) Enabled() bool {
	s, _ := r.scripts()
	return len(s) > 0
}

// scripts returns the runnable scripts in Dir, sorted lexically. A missing
// directory yields no scripts and no error.
func (r *Runner) scripts() ([]string, error) {
	entries, err := os.ReadDir(r.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if runnable(e) {
			out = append(out, filepath.Join(r.Dir, e.Name()))
		}
	}
	sort.Strings(out)
	return out, nil
}

// runnable reports whether a directory entry is an executable action script.
// It skips directories, dotfiles, editor backups and *.sample templates.
func runnable(e os.DirEntry) bool {
	name := e.Name()
	if e.IsDir() ||
		strings.HasPrefix(name, ".") ||
		strings.HasSuffix(name, "~") ||
		strings.HasSuffix(name, ".sample") {
		return false
	}
	info, err := e.Info()
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	return info.Mode().Perm()&0o111 != 0
}

// Fire runs every selected action script for one message, in lexical order. The
// runner's DBPath is injected into the event. Per-script failures are logged and
// joined into the returned error but never stop the remaining scripts. It returns
// the number of scripts executed. When Verbose is set it logs the event and each
// script as it runs.
func (r *Runner) Fire(ctx context.Context, ev action.Event) (int, error) {
	all, err := r.scripts()
	if err != nil {
		return 0, err
	}
	scripts := r.selected(all)
	if len(scripts) == 0 {
		return 0, nil
	}
	ev.DBPath = r.DBPath // the runner owns the database path
	if r.Verbose {
		r.logf("fire %s %s %q", ev.Name, ev.Message.ID, subjectOf(ev.Message))
	}

	env := append(os.Environ(), ev.Env()...)
	stdin := string(ev.JSON())

	var errs error
	for _, script := range scripts {
		if r.Verbose {
			r.logf("  run %s", filepath.Base(script))
		}
		if rerr := r.run(ctx, script, env, stdin); rerr != nil {
			errs = errors.Join(errs, rerr)
		}
	}
	return len(scripts), errs
}

// selected applies the Filter allow-list — glob patterns matched against each
// script's filename — to the discovered scripts. An empty filter keeps them all.
func (r *Runner) selected(scripts []string) []string {
	if len(r.Filter) == 0 {
		return scripts
	}
	var out []string
	for _, s := range scripts {
		if r.matches(filepath.Base(s)) {
			out = append(out, s)
		}
	}
	return out
}

// matches reports whether name matches any Filter pattern. A pattern with no
// wildcards is an exact filename match.
func (r *Runner) matches(name string) bool {
	for _, pat := range r.Filter {
		if ok, _ := path.Match(pat, name); ok {
			return true
		}
	}
	return false
}

// killGrace bounds how long the runner waits, after a script's process exits or
// the context is cancelled, for its I/O pipes to close. Without it a killed
// script whose grandchild keeps the pipe open (e.g. `sh -c 'sleep 5'`) would
// block the runner until the grandchild exits.
const killGrace = 2 * time.Second

// run executes a single script with the payload, enforcing the timeout.
func (r *Runner) run(ctx context.Context, path string, env []string, stdin string) error {
	if r.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.Timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, path)
	cmd.Dir = r.Dir
	cmd.Env = env
	cmd.Stdin = strings.NewReader(stdin)
	cmd.WaitDelay = killGrace

	if out, err := cmd.CombinedOutput(); err != nil {
		name := filepath.Base(path)
		r.logf("action %s failed: %v%s", name, err, indent(out))
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

// FireNew fires the new-message action for each message and returns how many
// produced at least one script run, plus any joined errors. The input is
// expected newest-first, as the store returns it: the batch is capped to the
// most recent Max and fired oldest-first so scripts receive messages in roughly
// chronological order.
func (r *Runner) FireNew(ctx context.Context, query string, msgs []store.Message) (int, error) {
	if r.Max > 0 && len(msgs) > r.Max {
		r.logf("actions: %d new messages, firing the most recent %d", len(msgs), r.Max)
		msgs = msgs[:r.Max]
	}
	var errs error
	fired := 0
	for i := len(msgs) - 1; i >= 0; i-- {
		ev := action.Event{Name: action.EventNew, Query: query, Message: toMessage(msgs[i])}
		ran, err := r.Fire(ctx, ev)
		if err != nil {
			errs = errors.Join(errs, err)
		}
		if ran > 0 {
			fired++
		}
	}
	return fired, errs
}

// toMessage converts a store row into the public action message payload.
func toMessage(m store.Message) action.Message {
	return action.Message{
		ID:             m.ID,
		Date:           m.Date,
		IsRead:         m.IsRead,
		IsFlagged:      m.IsFlagged,
		HasAttachments: m.HasAttachments,
		FromName:       m.FromName,
		FromAddress:    m.FromAddress,
		Subject:        m.Subject,
		Snippet:        m.Snippet,
		ToAddresses:    m.ToAddresses,
		BodyText:       m.BodyText,
		BodyHTML:       m.BodyHTML,
	}
}

func (r *Runner) logf(format string, args ...any) {
	if r.Logf != nil {
		r.Logf(format, args...)
	}
}

// Scaffold creates dir (if needed) and writes the non-executable sample script
// unless it already exists. It returns the path of the sample.
func Scaffold(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, sampleName)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.WriteFile(path, []byte(sampleScript), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// subjectOf returns a short, single-line subject for verbose logging.
func subjectOf(m action.Message) string {
	s := strings.TrimSpace(m.Subject)
	if s == "" {
		return "(no subject)"
	}
	if r := []rune(s); len(r) > 72 {
		return string(r[:71]) + "…"
	}
	return s
}

// indent prefixes captured script output with a newline and tab, or returns ""
// for empty output, so log lines stay readable.
func indent(out []byte) string {
	s := strings.TrimRight(string(out), "\n")
	if s == "" {
		return ""
	}
	return "\n\t" + strings.ReplaceAll(s, "\n", "\n\t")
}

// sampleScript is the template written by Scaffold.
const sampleScript = `#!/bin/sh
#
# meliafts action script (sample).
#
# Copy or rename this file to drop the .sample suffix, then 'chmod +x' it to
# enable. Every executable file in this directory runs once per new message,
# in lexical order (prefix with 10-, 20-, ... to control the order).
#
# Each message arrives as MELIAFTS_* environment variables and as a JSON object
# on stdin. MELIAFTS_DB points at the read-only database for deeper queries.
#
# This sample shows a desktop notification for new unread mail.

[ "$MELIAFTS_UNREAD" = "1" ] || exit 0

command -v notify-send >/dev/null 2>&1 || exit 0
notify-send "New mail: ${MELIAFTS_FROM_NAME:-$MELIAFTS_FROM_ADDRESS}" "$MELIAFTS_SUBJECT"
`
