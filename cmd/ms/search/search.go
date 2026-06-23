// Package search implements the `ms search` subcommand, which runs a
// Gmail-style query against the melia database.
package search

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/rubiojr/meliafts/cmd/ms/util"
	"github.com/rubiojr/meliafts/internal/query"
	"github.com/rubiojr/meliafts/internal/store"
	"github.com/urfave/cli/v3"
)

// Command is the `ms search` subcommand.
var Command = &cli.Command{
	Name:      "search",
	Aliases:   []string{"s"},
	Usage:     "Search messages with a Gmail-style query",
	ArgsUsage: "<query>",
	Description: `Search the melia database using Gmail-style filters.

Field filters (full-text):
  subject:<text>      match the subject
  sender:<text>       match the sender name or address (alias: from:)
  recipient:<text>    match a recipient (alias: to:)
  snippet:<text>      match the message snippet
  body:<text>         match the message body

Boolean filters (true when present):
  unread:             only unread messages (read: for the inverse)
  flagged:            only flagged messages (alias: starred:)
  attachments:        only messages with attachments

Date filters (value is a relative duration or an absolute date):
  newer:<when>        received within the last <when> (aliases: after:, since:)
  older:<when>        received before the last <when> (aliases: before:, until:)

  <when> may be a relative duration using short or long units —
  7d / 7days, 2w / 2weeks, 1m / 1month, 1y / 1year, 12h / 12hours —
  or an absolute date such as 2024-01-31 or 2024/01/31.

Folder filter:
  in:<folder>         restrict to a folder (alias: folder:); <folder> is one of
                      inbox, sent, drafts, trash, spam (e.g. in:sent)

Any other word is searched across all text columns. Quote phrases with
double quotes, prefix-match with a trailing '*', and negate any term with a
leading '-'. Because '-' also starts a flag, place '--' before any query
that uses negation so it is passed through untouched.

Examples:
  ms search subject:invoice unread:
  ms search sender:bob flagged: attachments:
  ms search newer:7d body:kubernetes
  ms search after:2024-01-01 older:1month from:bob
  ms search -- subject:invoice -subject:draft`,
	Flags: []cli.Flag{
		&cli.IntFlag{
			Name:    "limit",
			Aliases: []string{"n"},
			Value:   50,
			Usage:   "maximum number of results (0 for no limit)",
		},
		&cli.BoolFlag{
			Name:  "sql",
			Usage: "print the generated SQL and arguments, then exit",
		},
		&cli.BoolFlag{
			Name:  "fts",
			Usage: "print the FTS5 MATCH expression, then exit",
		},
		&cli.BoolFlag{
			Name:  "json",
			Usage: "output results as JSON",
		},
	},
	Action: run,
}

func run(ctx context.Context, cmd *cli.Command) error {
	queryStr := strings.Join(cmd.Args().Slice(), " ")
	q, err := query.Parse(queryStr)
	if err != nil {
		return fmt.Errorf("invalid query: %w", err)
	}

	// --fts: show only the full-text MATCH expression.
	if cmd.Bool("fts") {
		expr, err := q.FTSMatch()
		if err != nil {
			return err
		}
		fmt.Println(expr)
		return nil
	}

	limit := cmd.Int("limit")

	// --sql: show the generated statement without touching the database.
	if cmd.Bool("sql") {
		compiled, err := q.Compile(query.Options{Limit: limit})
		if err != nil {
			return err
		}
		fmt.Println(compiled.SQL)
		fmt.Printf("-- args: %v\n", compiled.Args)
		return nil
	}

	st, err := store.Open(cmd.String("db"))
	if err != nil {
		return err
	}
	defer st.Close()

	if err := util.VerifySchema(cmd, st); err != nil {
		return err
	}

	results, err := st.Search(queryStr, limit, 0)
	if err != nil {
		return err
	}

	if cmd.Bool("json") {
		return writeJSON(results)
	}
	writeText(queryStr, results)
	return nil
}

func writeJSON(results []store.Message) error {
	if results == nil {
		results = []store.Message{}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(results)
}
