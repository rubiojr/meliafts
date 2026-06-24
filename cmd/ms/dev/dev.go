// Package dev holds developer and maintenance subcommands grouped under
// `ms dev`. They are not part of normal mail searching.
package dev

import "github.com/urfave/cli/v3"

// Command is the `ms dev` subcommand group.
var Command = &cli.Command{
	Name:  "dev",
	Usage: "Developer and maintenance tools",
	Commands: []*cli.Command{
		gendbCommand,
	},
}
