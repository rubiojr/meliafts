// Package util holds shared CLI configuration for the ms command: the
// application metadata and the global flags.
package util

import (
	"fmt"
	"os"

	"github.com/rubiojr/meliafts/internal/db"
	"github.com/rubiojr/meliafts/internal/store"
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
		&cli.BoolFlag{
			Name:  "force-schema-unsupported",
			Usage: "run even if the database schema version is not the supported one",
		},
	}
}

// VerifySchema checks the open database's melia schema version. When it drifts
// from the supported version it returns an error so the command bails — unless
// --force-schema-unsupported was given, in which case it warns on stderr and
// lets the command continue.
func VerifySchema(cmd *cli.Command, st *store.Store) error {
	err := st.CheckSchema()
	if err == nil {
		return nil
	}
	if cmd.Bool("force-schema-unsupported") {
		fmt.Fprintf(os.Stderr, "%s: warning: %v; continuing because --force-schema-unsupported was set\n", AppName, err)
		return nil
	}
	return fmt.Errorf("%w\npass --force-schema-unsupported to run against it anyway", err)
}
