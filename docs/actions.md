# Actions

Actions are drop-in scripts that meliafts runs when **new messages** appear —
think Git hooks, but for your mailbox. Use them to fire a desktop notification,
post to a webhook, append to a log, or kick off any command you like.

Because meliafts only ever opens the melia database **read-only**, actions are
purely *reactive*: they can't change, delete, or send mail, and they can't get
stuck in a loop. They observe and react.

## Quick start

```sh
# 1. Create the actions directory and a template (does nothing on its own):
ms watch
# → wrote a template to ~/.local/share/meliafts/actions/new-message.sample

# 2. Turn the template into a real, executable action:
cd ~/.local/share/meliafts/actions
cp new-message.sample 10-notify
chmod +x 10-notify

# 3. Watch your inbox and run actions as mail arrives:
ms watch
```

Run it against a query to scope which mail you care about:

```sh
ms watch unread: from:boss        # only new unread mail from the boss
ms watch in:inbox -from:noreply    # new inbox mail that isn't from a noreply
```

## The actions directory

```
~/.local/share/meliafts/actions/
  new-message.sample      # template, NOT executable → never runs
  10-notify               # executable → runs
  20-webhook              # executable → runs after 10-notify
```

- Location: `$XDG_DATA_HOME/meliafts/actions` (defaults to
  `~/.local/share/meliafts/actions`). Override with `--actions-dir`.
- Only files with the **executable bit set** run. `chmod +x` to enable,
  `chmod -x` to disable — no config to edit.
- Scripts run in **lexical order**, so prefix them (`10-`, `20-`) to control
  sequence. A common convention is one script per task.
- Ignored: directories, dotfiles, editor backups (`*~`) and `*.sample` files.

## Selecting which actions run

By default every executable in the directory fires. To run only some of them,
pass `--actions-filter` — an **allow-list** of glob patterns matched against the
script filename. It is repeatable, and a script runs if it matches any pattern:

```sh
ms watch --actions-filter 10-notify              # exactly one script
ms watch --actions-filter '2*' --actions-filter 10-notify   # several
ms tui --actions --actions-filter '*-notify'     # same flag in the TUI
```

A pattern with no wildcards is an exact filename match; `*` and `?` work as in
shell globs (quote them so your shell doesn't expand them first). A filter that
matches nothing simply runs nothing. This is handy for testing a single action,
or for running different subsets from `ms watch` and `ms tui` against the same
directory.

## What a script receives

Every script is run **once per new message**. The message is provided two ways,
so use whichever suits your language:

### Environment variables

| Variable | Meaning |
|---|---|
| `MELIAFTS_EVENT` | the event name (currently always `new-message`) |
| `MELIAFTS_ID` | message id |
| `MELIAFTS_DATE` | message date, RFC 3339 (e.g. `2026-06-20T15:05:03Z`) |
| `MELIAFTS_SUBJECT` | subject |
| `MELIAFTS_FROM_NAME` | sender display name (may be empty) |
| `MELIAFTS_FROM_ADDRESS` | sender email address |
| `MELIAFTS_SNIPPET` | short preview of the body |
| `MELIAFTS_UNREAD` | `1` if unread, else `0` |
| `MELIAFTS_FLAGGED` | `1` if flagged/starred, else `0` |
| `MELIAFTS_HAS_ATTACHMENTS` | `1` if it has attachments, else `0` |
| `MELIAFTS_QUERY` | the query `ms watch` is running |
| `MELIAFTS_DB` | path to the melia database (read-only) |

### JSON on stdin

The full message record is also piped to the script's standard input as a JSON
object (the same shape as `ms search --json`), e.g.:

```json
{"id":"msg-00042","date":"2026-06-20T15:05:03Z","is_read":false,
 "is_flagged":false,"has_attachments":false,"from_name":"Amazon",
 "from_address":"pickup-point@amazon.es","subject":"Paquete listo para recogida",
 "snippet":"El paquete está listo…"}
```

The body is **not** included by default (to keep watching cheap). If you need
it, query `MELIAFTS_DB` yourself — for example with the `ms` binary or `sqlite3`.

### Exit code & timeout

- The exit code is advisory: a non-zero exit is logged but never blocks meliafts
  or the other scripts.
- Each script is given `--timeout` (default 10s) to finish; if it overruns it is
  killed. Keep actions quick, or have them hand off to a background job.

## `ms watch`

```
ms watch [query]
```

| Flag | Default | Meaning |
|---|---|---|
| `--interval`, `-i` | `30s` | how often to poll for new mail |
| `--limit`, `-n` | `100` | how many matching messages to scan each poll |
| `--actions-dir` | `~/.local/share/meliafts/actions` | where the scripts live |
| `--actions-max` | `25` | most scripts fired in a single poll (prevents a flood) |
| `--actions-filter` | _(none)_ | only run scripts matching this glob; repeatable (allow-list) |
| `--timeout` | `10s` | per-script timeout |
| `--fire-existing` | off | also fire for mail already present on the first poll |
| `--once` | off | poll a single time and exit |
| `--verbose`, `-v` | off | log each event and the scripts it runs to stderr |

`ms watch` also honours the global `--db` flag (and `$MELIA_DB`).

By default the **first poll is silent**: it records what's already in your
mailbox as the baseline so you aren't flooded with notifications for old mail.
From then on, only genuinely new messages trigger actions. Pass `--fire-existing`
to opt out of that baseline (handy with `--once` to run an action over the
current matches):

```sh
ms watch --once --fire-existing subject:invoice
```

## In the TUI

The interactive UI can fire the same actions while you browse. Pass `--actions`
to `ms tui`:

```sh
ms tui --actions unread:
```

Whenever the list **reloads** — on the auto-reload timer (`--reload`, default
30s) or when you press `Ctrl+R` — any message that is new since the last reload
triggers your scripts, exactly as `ms watch` does. The same directory, scripts
and contract apply, and the TUI takes the same knobs: `--actions-dir`,
`--actions-max`, `--timeout` and the repeatable `--actions-filter`. Loading the first
results or changing the query only re-establishes the baseline, so switching
views never replays mail. A small `N fired` indicator appears in the status bar
once actions have run.

`ms watch` and `ms tui --actions` are complementary: the watcher is for running
headless in the background, the TUI for acting on new mail while you read.

With `--verbose`, each event and the scripts it triggers are logged to stderr —
handy while writing or debugging an action:

```
$ ms watch --verbose unread:
ms watch: fire new-message msg-00120 "Go 1.x is out — here's what's new"
ms watch:   run 10-notify
ms watch:   run 20-webhook
```

## Examples

Desktop notification for new unread mail (the shipped sample):

```sh
#!/bin/sh
[ "$MELIAFTS_UNREAD" = "1" ] || exit 0
command -v notify-send >/dev/null 2>&1 || exit 0
notify-send "New mail: ${MELIAFTS_FROM_NAME:-$MELIAFTS_FROM_ADDRESS}" "$MELIAFTS_SUBJECT"
```

Post to a webhook using the JSON on stdin:

```sh
#!/bin/sh
jq -c '{text: "📧 \(.subject) — \(.from_address)"}' \
  | curl -sS -X POST -H 'content-type: application/json' -d @- "$SLACK_WEBHOOK_URL"
```

Append a line to a log:

```sh
#!/bin/sh
printf '%s\t%s\t%s\n' "$MELIAFTS_DATE" "$MELIAFTS_FROM_ADDRESS" "$MELIAFTS_SUBJECT" \
  >> "$HOME/mail.log"
```

## Writing an action in Go

Any language works — the contract is just environment variables and a JSON line
on stdin. For Go there's a tiny SDK, `github.com/rubiojr/meliafts/pkg/action`,
that parses both for you and hands your code a typed `Event`:

```go
package main

import "github.com/rubiojr/meliafts/pkg/action"

func main() {
    action.Handle(func(ev action.Event) error {
        if ev.Message.IsRead {
            return nil // only notify about unread mail
        }
        return notify(ev.Message.FromAddress, ev.Message.Subject)
    })
}
```

`action.Handle` reads the event, runs your function once per message, and exits
non-zero (logged by `ms watch`) if it returns an error. `ev.Message` has typed
fields (`ID`, `Subject`, `FromAddress`, `IsRead`, `HasAttachments`, …) and
`ev.Query` / `ev.DBPath` give you the watch query and the database path. Build it
and drop the binary in the actions directory like any other script:

```sh
go build -o ~/.local/share/meliafts/actions/10-notify .
chmod +x ~/.local/share/meliafts/actions/10-notify
```

If you'd rather control the flow yourself, `action.Read()` returns the same
`Event` without the exit-on-error behaviour.

## Running it in the background

`ms watch` runs in the foreground and exits cleanly on Ctrl-C. To keep it
running, use your service manager. A minimal systemd user service:

```ini
# ~/.config/systemd/user/meliafts-watch.service
[Unit]
Description=meliafts mail watcher

[Service]
ExecStart=%h/go/bin/ms watch unread:
Restart=on-failure

[Install]
WantedBy=default.target
```

```sh
systemctl --user enable --now meliafts-watch.service
```

> Note: `ms watch` keeps its "already seen" list in memory only, so each restart
> re-baselines against your current mailbox (it won't replay old mail, but it
> also won't notify about messages that arrived while it was down).

## Safety

Actions run **arbitrary executables you place in the directory**, with your
permissions. Treat the actions directory like `~/.bashrc`:

- Nothing runs until you set the executable bit, and the shipped template is
  intentionally non-executable.
- Keep the directory writable only by you.
- meliafts never writes to your mailbox, so an action can't corrupt or loop back
  into your mail — the worst a buggy script can do is whatever *it* does.

## Troubleshooting

- **Nothing runs.** Check the executable bit (`ls -l`), that the file isn't named
  `*.sample`, and run with `--verbose`. `ms watch` with no executable scripts
  prints where it expects them.
- **It fired for my whole inbox.** You probably passed `--fire-existing`. Without
  it, the first poll is a silent baseline.
- **A script misbehaves.** Failures and timeouts are logged to stderr with the
  script name; run `ms watch --verbose` to see activity.
