# Architecture: Actions

Developer notes for the actions subsystem — the engine that runs user scripts
when new messages appear. User-facing documentation lives in
[`docs/actions.md`](../actions.md).

## Goal

Give meliafts a small, safe automation surface ("mailbox hooks") without
compromising its core property: the melia database is only ever opened
**read-only**. Actions therefore *react* to new mail (notify, log, webhook,
hand-off) but can never mutate state, which also means they can't create
feedback loops.

## Pieces

```
pkg/action/               PUBLIC SDK — the action contract, shared by both sides
  action.go               Event/Message types, MELIAFTS_* names, Env()/JSON()
  handler.go              Read()/Handle() for actions written in Go
internal/actions/         the reusable engine (presentation-free)
  actions.go              Runner: discovery, payload, execution, scaffolding
  tracker.go              Tracker: the shared new-message detector
  poller.go               Poller: search → diff → fire, with baseline priming
cmd/ms/watch/             the `ms watch` command (thin CLI wrapper)
  watch.go
cmd/ms/tui/               `ms tui --actions` reuses Runner + Tracker on reload
```

Dependency direction (consistent with the rest of the tree):

```
cmd/ms/watch ┐
cmd/ms/tui   ┴→ internal/actions → internal/store → internal/query + internal/db
                       ↘
                        pkg/action  (public, standard-library only)
```

`pkg/action` is the **single source of truth for the wire contract** (the
`MELIAFTS_*` environment and the stdin JSON). The producer — `internal/actions`
— builds payloads with `action.Event.Env()` / `action.Event.JSON()`, and action
authors decode the same `action.Event` with `action.Read` / `action.Handle`. The
two can never drift because they share the definition. `pkg/action` depends only
on the standard library and on nothing internal, so it stays a safe public
import; the `store.Message → action.Message` conversion lives in
`internal/actions` (`toMessage`), keeping the SDK free of database types.

`internal/actions` is presentation-free (it knows the store and the filesystem,
nothing about the CLI or TUI), so the same `Runner` + `Tracker` back both the
headless `ms watch` command and the interactive `ms tui --actions` reload loop.

## Data flow

```
        ┌──────────────────────────── ms watch ───────────────────────────┐
        │                                                                  │
   every --interval                                                        │
        │                                                                  │
        ▼                                                                  │
  Poller.Tick ──▶ store.Search(query, limit, 0) ──▶ diff vs seen set       │
        │                                                │                 │
        │                                          new message ids         │
        │                                                ▼                 │
        │                                   Runner.Fire(event, msg)        │
        │                                                │                 │
        │                         for each executable in actions dir:      │
        │                         exec with env + JSON stdin + timeout     │
        └──────────────────────────────────────────────────────────────────┘
```

## New-message detection (`Tracker`)

Detection is factored into a tiny shared type so the headless `Poller` and the
TUI reload loop behave identically:

```go
type Tracker struct{ seen map[string]bool }

func (t *Tracker) Seen(msgs []store.Message)               // record a baseline, fire nothing
func (t *Tracker) Fresh(msgs []store.Message) []store.Message // ids not seen before (now recorded)
```

Detection is **id-based**, not time-based: a message is "new" the first time its
id is seen. That has two nice properties — a message being *modified* (e.g.
marked read) keeps its id and won't re-fire, and ids are never evicted within a
session, so scrolling/refetching can't cause a re-fire. The set is **in-memory
and session-scoped**; restarting re-baselines against the current mailbox.
Persisting a watermark is left to a follow-up (see Future work).

`Runner.FireNew(ctx, query, msgs)` is the firing counterpart: given the
newest-first messages a `Tracker` reported, it caps the batch to the most recent
`Max` (the overflow stays recorded, so a one-off bulk sync can't back up an
unbounded queue) and fires `Runner.Fire` oldest-first, so scripts see messages in
roughly chronological order.

### Poller (`ms watch`)

```go
type Poller struct {
    Store        *store.Store
    Runner       *Runner
    Query        string
    Limit        int
    FireExisting bool   // fire for pre-existing mail on the first poll

    tracker *Tracker
    primed  bool
}

func (p *Poller) Tick(ctx context.Context) (fired int, err error)
```

Each `Tick` runs `store.Search(Query, Limit, 0)` — the same query layer the CLI
and TUI use, so actions are **scoped to the active query**. The first tick primes
(`tracker.Seen`, firing nothing) unless `FireExisting` is set; subsequent ticks
fire `Runner.FireNew(query, tracker.Fresh(results))`.

### TUI (`ms tui --actions`)

The model owns a `*Runner` and a `*Tracker` (both nil unless `--actions` is set
and the directory has an executable script). The hook point is `onSearch`, which
already distinguishes a **reload** from a fresh load via `searchMsg.keepPos`:

- `keepPos` (a reload — the auto-reload tick or `Ctrl+R`): `tracker.Fresh(results)`
  → if non-empty, return a `tea.Cmd` that calls `Runner.FireNew` **off the UI
  goroutine** and reports back an `actionsRanMsg`.
- a fresh query / quick filter / first load: `tracker.Seen(results)` only, so
  changing the view never replays mail.
- appended pages (`onPage`, endless scroll) are older rows → `tracker.Seen`.

Firing is never synchronous: the model returns a command, the scripts run in a
goroutine, and `actionsRanMsg{fired, err}` updates a small `N fired` status-bar
indicator (`actionsTag`). The TUI runner sets no `Logf` (stderr is unusable under
the alt-screen) — `ms watch -v` is the place to debug script output. Gating is an
explicit `--actions` flag, not mere directory presence.

## Execution model (`Runner`)

```go
type Runner struct {
    Dir     string                       // actions directory
    DBPath  string                       // injected into the event as MELIAFTS_DB
    Timeout time.Duration                // per-script timeout (0 = none)
    Max     int                          // cap on fires per batch (0 = unlimited)
    Filter  []string                     // allow-list of filename globs (empty = all)
    Verbose bool                         // log each event and script via Logf
    Logf    func(format string, args ...any)
}

func (r *Runner) Enabled() bool
func (r *Runner) Fire(ctx context.Context, ev action.Event) (ran int, err error)
```

`Fire` takes a public `action.Event`; the `Poller` builds it from a
`store.Message` via `toMessage`, and `Fire` injects the runner's `DBPath` before
encoding (the runner owns that config).

- **Discovery** (`scripts`): `os.ReadDir` the directory, keep regular files with
  any executable bit set, skipping directories, dotfiles, `*~` and `*.sample`.
  Results are sorted lexically. A missing directory is "no scripts", not an
  error.
- **Selection** (`selected`/`matches`): when `Filter` is non-empty, only scripts
  whose filename matches one of the `path.Match` globs are run. The filter is
  applied in `Fire`, not in `Enabled`/discovery, so it narrows what runs without
  changing whether the system is considered set up — a filter matching nothing
  simply fires nothing. `FireNew` counts a message as "fired" only when at least
  one script actually ran for it, so an all-excluding filter reports `0`.
- **Payload**: built by `pkg/action`, not hand-rolled here. `action.Event.Env()`
  produces the `MELIAFTS_*` slice (convenient for shell) and `action.Event.JSON()`
  the stdin object (convenient for jq/python or the Go SDK). `MELIAFTS_DB` lets a
  script reach back into the database for anything not in the light row (e.g. the
  body).
- **Process**: `exec.CommandContext` with a per-script `context.WithTimeout`,
  `cmd.Dir` set to the actions directory, `cmd.Env = os.Environ()+payload`. Output
  is captured with `CombinedOutput`; on failure the script name, error and output
  are logged via `Logf`. Scripts run **sequentially** in lexical order; one
  failing or timing-out script never stops the others. `cmd.WaitDelay`
  (`killGrace`) bounds shutdown: when the context cancels, killing the script's
  shell does not reap a grandchild that still holds the output pipe (e.g.
  `sh -c 'sleep 5'`), so without the delay `CombinedOutput` would block until the
  grandchild exits. `WaitDelay` closes the pipes shortly after, so a timeout is
  honoured promptly.
- **Exit codes** are advisory — the engine is a notifier, not a gate, so a
  non-zero exit is logged but changes no behaviour.
- **Verbose logging** lives in the runner, not the command: when `Verbose` is
  set, `Fire` logs the event (`fire new-message <id> "<subject>"`) and each
  script as it starts (`  run <name>`) through `Logf`. Keeping it here means
  `ms watch` and `ms watch --once` (and any future caller) get identical output
  with no extra wiring; the `--verbose` flag just sets `Runner.Verbose`.

### Scaffolding

`Scaffold(dir)` creates the directory and writes a non-executable
`new-message.sample` template (embedded as a string constant). `ms watch` calls
it when no executable scripts are found, so a first run leaves the user with a
ready-to-edit example and clear guidance, then exits without looping.

`DefaultDir()` resolves `$XDG_DATA_HOME/meliafts/actions`, falling back to
`~/.local/share/meliafts/actions`.

## `ms watch`

A thin urfave/cli v3 command (self-registering `var Command`, added to the root
in `cmd/ms/main.go`). It:

1. Builds a `Runner` from the flags (`--actions-dir`, `--timeout`,
   `--actions-max`, a stderr `Logf`).
2. If the runner isn't `Enabled()`, scaffolds the sample and exits with guidance.
3. Opens the store read-only via the global `--db`.
4. Builds a `Poller` (`--limit`, `--fire-existing`) for the query argument.
5. `--once` → a single `Tick`; otherwise loops on a `time.Ticker(--interval)`.

Signals: the command wraps the context with `signal.NotifyContext` so Ctrl-C /
SIGTERM cancel the loop and any in-flight script (the timeout context derives
from it) and it exits cleanly.

## Security model

- **Opt-in by the executable bit.** Nothing runs until the user `chmod +x`es a
  script; the shipped template is non-executable and `*.sample` is skipped
  regardless.
- **Read-only DB.** Actions cannot change the mailbox, so there is no write
  amplification or trigger loop; the blast radius is whatever the user's own
  script does.
- **Bounded.** Per-script timeouts and the per-batch `Max` cap stop a hung or
  fork-happy script from wedging or flooding the watcher.
- These are arbitrary executables running with the user's permissions — the same
  trust model as a shell rc file. The docs say so plainly.

## Testing

- **`Runner`** is tested directly with tiny `#!/bin/sh` scripts in a `t.TempDir`:
  discovery rules (exec bit, dotfiles, `*.sample`), the env+stdin payload (a
  script dumps `MELIAFTS_*` and stdin to a file the test reads back), and the
  timeout (a sleeping script is killed and reported).
- **`Poller`** is tested against a real SQLite melia DB built with the schema. A
  second, **writable** handle inserts a new message *between* ticks to simulate
  mail arriving, proving the prime-then-fire semantics, `FireExisting`, and the
  `Max` cap. No timers are involved, so the tests are fast and deterministic.
- **`Tracker`** has direct unit tests (baseline vs. fresh, order preservation),
  and the **TUI** wiring is tested without a terminal: a model with a runner whose
  script logs fired ids is driven through `onSearch` (a `keepPos` reload fires for
  the new id; a fresh load only re-baselines), executing the real script.
- **`pkg/action`** tests the contract both ways: `Env()`/`JSON()` output, and a
  producer↔consumer round trip where `decode` reads back an event from the very
  `Env()`/`JSON()` a host would emit (plus the stdin-empty env fallback). An
  external module with a local `replace` building a real action against the SDK
  is the manual end-to-end check.

## Future work

- **Persistence.** A watermark (recent ids or last-fired timestamp) under
  `$XDG_STATE_HOME/meliafts/` so restarts, `ms watch` and the TUI don't
  re-baseline a mailbox they already saw.
- **More events.** `reload` (summary), `open` (message read). The flat directory
  carries `MELIAFTS_EVENT`; per-event subdirectories are a possible structure if
  this grows.
- **Richer payload.** Expose the folder type (needs the light Search row to
  carry it) and optionally the rendered body via `internal/renderer`.
- **Parallelism.** Bounded concurrent execution if sequential firing proves too
  slow for large batches.
- **Process-group kill.** A timed-out script's grandchildren are currently
  orphaned (bounded by `WaitDelay`, not killed). Running each script in its own
  process group and signalling the group on timeout would reap them, at the cost
  of unix-only `SysProcAttr` code.
