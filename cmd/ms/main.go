// Command ms searches the melia mail database using a Gmail-style query
// language compiled to SQLite FTS5.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/rubiojr/meliafts/cmd/ms/search"
	"github.com/rubiojr/meliafts/cmd/ms/tui"
	"github.com/rubiojr/meliafts/cmd/ms/util"
	"github.com/rubiojr/meliafts/cmd/ms/watch"
	"github.com/urfave/cli/v3"
)

func main() {
	cmd := &cli.Command{
		Name:    util.AppName,
		Usage:   "Search the melia mail database",
		Version: util.Version,
		Flags:   util.GlobalFlags(),
		Commands: []*cli.Command{
			search.Command,
			tui.Command,
			watch.Command,
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", util.AppName, err)
		os.Exit(1)
	}
}
