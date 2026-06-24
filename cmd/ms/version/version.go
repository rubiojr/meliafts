// Package version implements the `ms version` subcommand, printing the build
// version together with the VCS revision recorded by the Go toolchain.
package version

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/rubiojr/meliafts/cmd/ms/util"
	"github.com/urfave/cli/v3"
)

// Command is the `ms version` subcommand.
var Command = &cli.Command{
	Name:   "version",
	Usage:  "Print the ms version",
	Action: run,
}

func run(_ context.Context, _ *cli.Command) error {
	fmt.Printf("%s %s\n", util.AppName, String())
	return nil
}

// String returns the release version, augmented with the git revision and a
// "dirty" marker when the binary was built from a modified working tree. The
// VCS details come from the embedded Go build info, so they are only present for
// builds made inside a checkout (e.g. `go build`/`go install`); plain `go run`
// and `-buildvcs=false` builds fall back to the bare version.
func String() string {
	rev, dirty := vcsInfo()
	return format(util.Version, rev, dirty)
}

// format renders the version string from its parts. revision is empty when no
// VCS data is available, in which case only the bare version is returned.
func format(version, revision string, dirty bool) string {
	switch {
	case revision == "":
		return version
	case dirty:
		return fmt.Sprintf("%s (%s, dirty)", version, revision)
	default:
		return fmt.Sprintf("%s (%s)", version, revision)
	}
}

// vcsInfo returns the short git revision and whether the working tree was dirty
// at build time, read from the embedded build info. revision is empty when no
// VCS data was recorded.
func vcsInfo() (revision string, dirty bool) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", false
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if len(revision) > 12 {
		revision = revision[:12]
	}
	return revision, dirty
}
