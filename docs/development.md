# Development

A practical guide to building, testing and generating fixtures for meliafts.
For the actions subsystem see [`actions.md`](actions.md) (users) and
[`architecture/actions.md`](architecture/actions.md) (internals).

Module: `github.com/rubiojr/meliafts` · Go 1.26.

## Building and running

```sh
go build ./...                 # build everything
go build -o /tmp/ms ./cmd/ms   # build the ms binary
go run ./cmd/ms search subject:invoice unread:
```

The database is always opened **read-only**. When no path is given it is looked
up in order — the Flatpak install
`~/.var/app/com.buxjr.melia/config/melia/melia.db` first, then the Snap install
(`~/snap/melia/current/.config/melia/melia.db`, then
`~/snap/melia/common/.config/melia/melia.db`), then a regular install at
`$XDG_CONFIG_HOME/melia/melia.db` (i.e. `~/.config/melia/melia.db`) — using the
first that exists and falling back to the Flatpak path. Override with `--db` or
`$MELIA_DB`. You don't need a real melia database to work on meliafts — generate
one (see below).

## Quality gates

Run these before finishing a change; CI/reviewers expect them clean:

```sh
gofmt -w .
go vet ./...
staticcheck ./...                 # honnef.co/go/tools/cmd/staticcheck
go run github.com/fzipp/gocyclo/cmd/gocyclo@latest -over 10 .   # complexity <= 10
go test ./...                     # includes the e2e package (builds ms; ~0.5s)
```

Conventions: keep cyclomatic complexity at or below 10 (split dispatchers and
helpers when needed), use `stretchr/testify` (`require` for fatal, `assert`
otherwise), and prefer table-driven tests with `t.TempDir()` for temp databases.
Never add write paths to the database access layer, and don't introduce cgo (we
use the pure-Go `modernc.org/sqlite`).

## Sample & test databases — `ms dev gendb`

`ms dev gendb` writes a sample melia database. It has two modes: a random
generator for demos, and a profiler that reproduces the structure of a real
mailbox without copying any private content.

| Flag | Default | Meaning |
|---|---|---|
| `--output`, `-o` | `melia.db` | output path (replaced if it exists) |
| `--count`, `-n` | `140` | number of random messages (ignored with `--from-db`/`--profile`) |
| `--seed` | `1` | random seed (generation is deterministic) |
| `--from-db` | _(none)_ | profile this real melia DB and reproduce its structure |
| `--profile` | _(none)_ | reproduce the structure from a saved profile JSON |
| `--save-profile` | _(none)_ | write the profile that was used to this JSON file |

### Random demo data

```sh
ms dev gendb -o /tmp/melia.db -n 200 --seed 7
ms --db /tmp/melia.db tui
```

Content is seed-determined and deterministic; only the message *dates* depend on
the current time (so relative filters like `newer:7d` match the curated recent
messages). Generated databases are stamped with the supported `schema_version`,
so they pass the drift check.

### Reproduce a real mailbox's shape

```sh
ms dev gendb --from-db ~/.var/app/com.buxjr.melia/config/melia/melia.db \
  -o fixture.db --save-profile profile.json
```

This opens your database **read-only**, extracts an aggregate-only profile
(`internal/profile`) and regenerates a database with synthetic content that
matches:

- the **folder layout** — one folder per real folder, with the same type and
  message/unread counts;
- the **total message count** and the **first/last date** range;
- the **flag ratios** — flagged, has-attachments and HTML vs. plain-text;
- the **"All Mail" duplication** — a Gmail/Proton archive folder whose rows mostly
  copy other messages (same Message-ID), which is what exercises the TUI's
  de-duplication and spam-hiding.

`--save-profile` writes the profile it used; `--profile profile.json` reproduces
from a saved one (no source database needed). The profile is plain JSON of
counts, ratios and date ranges — **no addresses, subjects or bodies** — so it is
safe to commit or share.

```jsonc
{
  "schema_version": 13,
  "accounts": 1,
  "folders": [
    { "type": "inbox", "messages": 4545, "unread": 380 },
    { "type": "archive", "messages": 5227, "unread": 12 },
    { "type": "spam", "messages": 67, "unread": 67 }
  ],
  "messages": { "total": 10510, "distinct_message_id": 5300,
                "first_date": "2019-03-01 …", "last_date": "2026-06-23 …",
                "with_html": 9000, "has_attachments": 300, "per_month": { "2026-06": 210 } }
}
```

### Regenerating the committed demo fixture

```sh
go run ./cmd/ms dev gendb -o testdata/melia.db
```

`testdata/melia.db` is only a demo target for `ms --db testdata/melia.db tui`;
the e2e tests build their own fixtures via `internal/sampledb` and don't depend
on it.

### Limitations

Reproduction is structural with synthetic content. It does **not** reproduce
exact body/subject lengths, the per-month volume curve, or recipient-count
distributions — the profile captures those (`subject_len`, `per_month`,
`recipients`), they just aren't fed into generation yet. The archive/"All Mail"
folder is stored as `custom`, because the embedded schema (v1.1.242) predates the
`archive` folder type.

## Schema version & drift detection

meliafts reads melia's `settings.schema_version` (falling back to
`PRAGMA user_version`) and refuses to run against a version it wasn't built for,
to avoid silently querying an incompatible schema.

- `internal/db.SupportedSchemaVersion` is the supported integer (currently `13`).
- `search`, `tui` and `watch` call `util.VerifySchema` after opening the store;
  on drift they bail with a clear message unless `--force-schema-unsupported` is
  passed (which warns and continues).
- `internal/sampledb` stamps `SupportedSchemaVersion` into generated databases.

When melia ships a new migration: bump `SupportedSchemaVersion`, refresh the
embedded dump under `db/schema/`, and re-run the gates.

## Testing notes

- **Query/store** tests run against real in-memory FTS5 databases (build
  `messages` + `messages_fts`, and `folders` for folder tests). The FTS table is
  external-content, so join on `rowid` and reference the table name
  `messages_fts` (not an alias) in `MATCH`.
- **TUI** tests need no terminal: construct the model, feed synthetic
  `tea.WindowSizeMsg` / `tea.KeyPressMsg`, and assert on `View().Content` with
  `ansi.Strip`. Never execute the `tea.Tick` reload command in a test (it blocks
  for the interval) — assert the handler returns a non-nil command instead.
- **End-to-end** (`e2e/`) builds `ms` once in `TestMain` and runs it against a
  `internal/sampledb` fixture. `go test ./...` runs it.
- **Profiles/fixtures**: `internal/profile` tests build a DB with the real schema
  via `db/meliadb` and raw inserts; `internal/sampledb` tests round-trip a profile
  (generate → profile → compare) and assert the de-duplication structure.
