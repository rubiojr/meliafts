# Melia Email Database CLI

A [Melia](https://melia.buxjr.com) read-only, email search CLI and TUI (very much a vibecoded WiP).

## Features

- **Read-only by design** — only ever opens the Melia SQLite database for reading, so it can never change, delete or send mail.
- **Gmail-style query language** `subject:`, `from:`/`to:`, `body:`, `newer:`/`older:`, `in:`, `unread:`, `flagged:`, `attachments:`, quoted `"phrases"`, `prefix*` and `-negation`.
- **Scriptable search** (`ms search`) with styled output, `--json`, and `--sql`/`--fts` to inspect the compiled query without touching the database.
- **Interactive TUI** (`ms tui`) with endless scrolling, auto-reload, quick filters (unread/sent), and readable rendering of HTML/marketing emails.
- **Themes** — `amber`, `green`, `ice`, `paper`, `synthwave` (`--theme`).
- **Actions** — run your own scripts when new mail arrives, git-hooks style: headless with `ms watch` or live in the TUI with `ms tui --actions`. See [docs/actions.md](docs/actions.md).
- **Single static binary**, pure Go with no cgo (via `modernc.org/sqlite`).

## Install

```
go install github.com/rubiojr/meliafts/cmd/ms@latest
```

## Run

```
ms --db ~/.config/melia/melia.db search subject:hello
ms search   subject:privacy isn't dead
1 message
───────────────────────────────────────

[U  ] 2024-03-04 18:50  EFFector List <editor@eff.org>
      Privacy Isn’t Dead. Far From It. | EFFector 36.3
      This is a friendly message from the Electronic Frontier Foundation. EFFector 36.3 Privacy Isn’t Dea…
```

MeliaFTS supports Gmail style filters:

```
ms search subject:invoice unread:
ms search sender:bob flagged: attachments:
ms search newer:7d body:kubernetes
ms search after:2024-01-01 older:1month from:bob
ms search -- subject:invoice -subject:draft
```

`ms search --help` for a quick overview.

## TUI

```
ms tui
```

![](/docs/tui.png)

_Note: data in the screenshot was randomly generated_

TUI supports themes (`--theme`), `ms tui --help` to list them.
