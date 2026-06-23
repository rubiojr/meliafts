// Package util holds shared CLI configuration for the ms command: the
// application metadata and the global flags.
package util

import (
	"github.com/rubiojr/meliafts/internal/db"
	"github.com/urfave/cli/v3"
)

// AppName is the binary name.
const AppName = "ms"

// Version is the build version, overridable at link time with
// -ldflags "-X github.com/rubiojr/meliafts/cmd/ms/util.Version=...".
var Version = "dev"

// GlobalFlags returns the flags shared by every subcommand. They are persistent
// (the cli v3 default), so subcommands can read them with cmd.String etc.
func GlobalFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "db",
			Usage:   "path to the melia SQLite database",
			Value:   db.DefaultPath(),
			Sources: cli.EnvVars("MELIA_DB"),
		},
	}
}
